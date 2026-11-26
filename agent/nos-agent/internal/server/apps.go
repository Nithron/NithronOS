package server

// POST /v1/apps/op runs one privileged step of an app's lifecycle for nosd,
// which runs as the unprivileged nos user and used to try these itself
// (systemctl, chown, the snapshot script, writing Caddy snippets) and fail on
// installed systems. Only fixed operations on validated app IDs exist; the
// Caddy snippet is rendered here from a port number. (This replaces
// /v1/systemd/install-app, which wrote any unit file nosd sent and started
// it as root, and /v1/app/compose-up|down, which nothing used.)

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reAppID      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	reSnapName   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	reSnapStamp  = regexp.MustCompile(`^\d{8}-\d{6}(-[a-z0-9-]{1,32})?$`)
	appsRootDir  = "/srv/apps"
	appsLibDir   = "/usr/lib/nos/apps"
	caddyAppsDir = "/etc/caddy/Caddyfile.d"
	caddyfile    = "/etc/caddy/Caddyfile"
)

// AppOpRequest is one operation.
type AppOpRequest struct {
	Op       string `json:"op"` // prepare, group, start, stop, restart, disable, pull, snapshot, rollback, remove, proxy, unproxy
	AppID    string `json:"app_id"`
	UID      int    `json:"uid,omitempty"`       // prepare: owner of the data folder
	GID      int    `json:"gid,omitempty"`       // prepare
	Name     string `json:"name,omitempty"`      // snapshot
	Snapshot string `json:"snapshot,omitempty"`  // rollback
	KeepData bool   `json:"keep_data,omitempty"` // remove
	Port     int    `json:"port,omitempty"`      // proxy: the host port the app publishes
}

// appHost runs commands and touches files; tests replace it.
type appHost interface {
	Run(name string, args ...string) ([]byte, error)
	MkdirOwned(path string, mode os.FileMode, uid, gid int) error
	WriteFile(path string, data []byte) error
	Remove(path string) error
	ReadFile(path string) ([]byte, error)
	ListDir(path string) []string
}

type realAppHost struct{}

func (realAppHost) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (realAppHost) MkdirOwned(path string, mode os.FileMode, uid, gid int) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	return os.Lchown(path, uid, gid)
}

func (realAppHost) WriteFile(path string, data []byte) error {
	return realSharesHost{}.WriteFile(path, data)
}
func (realAppHost) Remove(path string) error             { return os.RemoveAll(path) }
func (realAppHost) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (realAppHost) ListDir(path string) []string {
	entries, _ := os.ReadDir(path)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, filepath.Join(path, e.Name()))
	}
	return out
}

var newAppHost = func() appHost { return realAppHost{} }

// nosIDs returns the nos user's uid and gid (nosd writes the app's compose
// file into its config folder).
var nosIDs = func() (int, int, error) {
	out, err := exec.Command("id", "-u", "nos").Output()
	if err != nil {
		return 0, 0, fmt.Errorf("user nos: %w", err)
	}
	uid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	out, err = exec.Command("id", "-g", "nos").Output()
	if err != nil {
		return 0, 0, fmt.Errorf("user nos: %w", err)
	}
	gid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return uid, gid, nil
}

func handleAppOp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req AppOpRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !reAppID.MatchString(req.AppID) {
		writeErr(w, http.StatusBadRequest, "invalid app id")
		return
	}
	out, code, err := runAppOp(newAppHost(), req)
	if err != nil {
		writeErr(w, code, err.Error())
		return
	}
	logAuthPriv("apps." + req.Op + " " + req.AppID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}

func runAppOp(h appHost, req AppOpRequest) (string, int, error) {
	unit := "nos-app@" + req.AppID + ".service"
	appDir := filepath.Join(appsRootDir, req.AppID)
	script := func(args ...string) (string, int, error) {
		out, err := h.Run(filepath.Join(appsLibDir, "nos-app-snapshot.sh"), args...)
		if err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("%s: %s", args[0], strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), 0, nil
	}
	systemctl := func(args ...string) (string, int, error) {
		out, err := h.Run("systemctl", args...)
		if err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("systemctl %s: %s", args[0], strings.TrimSpace(string(out)))
		}
		return "", 0, nil
	}

	switch req.Op {
	case "prepare":
		// config: nosd writes the compose and env files; data: the app's
		// container user owns it (PUID/PGID, 1000 by default).
		if req.UID < 1 || req.GID < 1 || req.UID > 65533 || req.GID > 65533 {
			return "", http.StatusBadRequest, fmt.Errorf("uid and gid must be 1-65533")
		}
		nosUID, nosGID, err := nosIDs()
		if err != nil {
			return "", http.StatusInternalServerError, err
		}
		if err := h.MkdirOwned(appDir, 0o755, 0, 0); err != nil {
			return "", http.StatusInternalServerError, err
		}
		if err := h.MkdirOwned(filepath.Join(appDir, "config"), 0o750, nosUID, nosGID); err != nil {
			return "", http.StatusInternalServerError, err
		}
		if _, code, err := script("ensure-subvolume", req.AppID); err != nil {
			return "", code, err
		}
		if out, err := h.Run("chown", fmt.Sprintf("%d:%d", req.UID, req.GID), filepath.Join(appDir, "data")); err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("chown data: %s", strings.TrimSpace(string(out)))
		}
		return "", 0, nil
	case "group":
		// The app's share-reader group; the output is its GID.
		gid, err := ensureAppGroup(h.Run, req.AppID)
		if err != nil {
			return "", http.StatusInternalServerError, err
		}
		return gid, 0, nil
	case "start", "stop", "restart":
		if req.Op == "start" {
			if _, code, err := systemctl("enable", unit); err != nil {
				return "", code, err
			}
		}
		return systemctl(req.Op, unit)
	case "disable":
		return systemctl("disable", "--now", unit)
	case "pull":
		out, err := h.Run("docker", "compose", "--project-directory", appDir,
			"-f", filepath.Join(appDir, "config", "docker-compose.yml"),
			"--env-file", filepath.Join(appDir, "config", ".env"),
			"--project-name", "nos-app-"+req.AppID, "pull")
		if err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("pull: %s", strings.TrimSpace(string(out)))
		}
		return "", 0, nil
	case "snapshot":
		if !reSnapName.MatchString(req.Name) {
			return "", http.StatusBadRequest, fmt.Errorf("invalid snapshot name")
		}
		return script("snapshot-pre", req.AppID, req.Name)
	case "rollback":
		if !reSnapStamp.MatchString(req.Snapshot) {
			return "", http.StatusBadRequest, fmt.Errorf("invalid snapshot")
		}
		return script("rollback", req.AppID, req.Snapshot)
	case "remove":
		if !req.KeepData {
			// Btrfs subvolumes (the data folder, read-only snapshots) are
			// deleted with btrfs; a plain removal fails on them.
			snapDir := filepath.Join(appsRootDir, ".snapshots", req.AppID)
			for _, p := range append(h.ListDir(snapDir), filepath.Join(appDir, "data")) {
				_, _ = h.Run("btrfs", "subvolume", "delete", p)
			}
			_ = h.Remove(snapDir)
			if err := h.Remove(appDir); err != nil {
				return "", http.StatusInternalServerError, err
			}
			return "", 0, nil
		}
		return "", 0, h.Remove(filepath.Join(appDir, "config"))
	case "proxy":
		if req.Port < 1 || req.Port > 65535 {
			return "", http.StatusBadRequest, fmt.Errorf("invalid port")
		}
		if err := h.WriteFile(filepath.Join(caddyAppsDir, "app-"+req.AppID+".caddy"), []byte(appCaddySnippet(req.AppID, req.Port))); err != nil {
			return "", http.StatusInternalServerError, err
		}
		return reloadCaddy(h)
	case "unproxy":
		if err := h.Remove(filepath.Join(caddyAppsDir, "app-"+req.AppID+".caddy")); err != nil {
			return "", http.StatusInternalServerError, err
		}
		return reloadCaddy(h)
	}
	return "", http.StatusBadRequest, fmt.Errorf("unknown op %q", req.Op)
}

// appCaddySnippet proxies /apps/<id>/ to the port the app publishes on the
// host. (It used to proxy to the container's name, which Caddy on the host
// can't resolve.)
func appCaddySnippet(appID string, port int) string {
	return fmt.Sprintf(`# Managed by nOS (nos-agent): app %[1]s
handle_path /apps/%[1]s/* {
	reverse_proxy 127.0.0.1:%[2]d {
		flush_interval -1
	}
}
`, appID, port)
}

// caddyAdmin puts Caddy's admin API on a socket only root and caddy reach.
const caddyAdmin = "admin unix//var/lib/caddy/admin.sock"

// caddyAppsImport is what the site block needs to load the app snippets.
const caddyAppsImport = "import " + "/etc/caddy/Caddyfile.d/app-*.caddy"

// reloadCaddy makes sure the Caddyfile loads the app snippets, then reloads
// Caddy. The Caddyfile shipped with "admin off", which makes `systemctl
// reload caddy` fail; the admin API now listens on a local socket instead.
func reloadCaddy(h appHost) (string, int, error) {
	if b, err := h.ReadFile(caddyfile); err == nil {
		updated := ensureCaddyAppsImport(string(b))
		if updated != string(b) {
			if err := h.WriteFile(caddyfile, []byte(updated)); err != nil {
				return "", http.StatusInternalServerError, err
			}
		}
	}
	if out, err := h.Run("caddy", "validate", "--adapter", "caddyfile", "--config", caddyfile); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("caddy validate: %s", strings.TrimSpace(string(out)))
	}
	if out, err := h.Run("systemctl", "reload-or-restart", "caddy"); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("reload caddy: %s", strings.TrimSpace(string(out)))
	}
	return "", 0, nil
}

// ensureCaddyAppsImport adds the app snippet import to the :443 site block
// and turns "admin off" into an admin socket in Caddy's state directory
// (only root and caddy can reach it), leaving everything else alone.
func ensureCaddyAppsImport(conf string) string {
	conf = strings.Replace(conf, "\n  admin off\n", "\n  "+caddyAdmin+"\n", 1)
	if strings.Contains(conf, caddyAppsImport) {
		return conf
	}
	i := strings.Index(conf, ":443 {")
	if i < 0 {
		return conf
	}
	j := i + len(":443 {")
	return conf[:j] + "\n  # Apps installed by nOS\n  " + caddyAppsImport + "\n" + conf[j:]
}
