package server

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"nithronos/backend/nosd/internal/config"
)

var redactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password\s*[:=]\s*)([^\s"']+)`),
	regexp.MustCompile(`(?i)(secret\s*[:=]\s*)([^\s"']+)`),
	regexp.MustCompile(`(?i)(token\s*[:=]\s*)([^\s"']+)`),
	regexp.MustCompile(`(?i)(key\s*=\s*)([^\s"']+)`),
}

func redactLine(line string) string {
	s := line
	for _, re := range redactionPatterns {
		s = re.ReplaceAllString(s, `$1REDACTED`)
	}
	return s
}

func writeTarFile(tw *tar.Writer, name string, r io.Reader) error {
	// Read all into buffer? For streams, we can read to memory to set size; but safer to stream without size by buffering.
	// We'll read into a temporary buffer since sizes are expected to be modest for text outputs.
	buf := bufio.NewScanner(r)
	var b strings.Builder
	for buf.Scan() {
		b.WriteString(redactLine(buf.Text()))
		b.WriteByte('\n')
	}
	data := []byte(b.String())
	hdr := &tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), ModTime: time.Now()}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeTarFileIfExists(tw *tar.Writer, hostPath, name string) {
	f, err := os.Open(hostPath)
	if err != nil {
		return
	}
	defer f.Close()
	_ = writeTarFile(tw, name, f)
}

func writeCmdOutput(tw *tar.Writer, name string, cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	out, _ := c.CombinedOutput()
	_ = writeTarFile(tw, name, strings.NewReader(string(out)))
}

func handleSupportBundle(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename=nos-support-bundle.tar.gz")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		tw := tar.NewWriter(gz)
		defer tw.Close()

		// Journals (last 2000 lines)
		writeCmdOutput(tw, "logs/journal_nosd.txt", "journalctl", "-u", "nosd", "-n", "2000")
		writeCmdOutput(tw, "logs/journal_nos_agent.txt", "journalctl", "-u", "nos-agent", "-n", "2000")
		writeTarFileIfExists(tw, "/var/log/caddy/access.log", "logs/caddy_access.log")
		writeTarFileIfExists(tw, "/var/log/caddy/error.log", "logs/caddy_error.log")

		// System info
		writeCmdOutput(tw, "system/uname.txt", "uname", "-a")
		writeTarFileIfExists(tw, "/etc/os-release", "system/os-release")

		// Firewall rules
		writeCmdOutput(tw, "network/nft_ruleset.txt", "nft", "list", "ruleset")

		// Storage
		writeCmdOutput(tw, "storage/lsblk.json", "lsblk", "-J", "-O")
		writeCmdOutput(tw, "storage/blkid.txt", "blkid")
		writeCmdOutput(tw, "storage/btrfs_show.txt", "btrfs", "filesystem", "show")
		// usage for common mount roots if present
		for _, m := range []string{"/", "/srv", "/mnt", "/pool", "/data"} {
			if fi, err := os.Stat(m); err == nil && fi.IsDir() {
				name := strings.TrimPrefix(m, "/")
				writeCmdOutput(tw, "storage/usage_"+strings.ReplaceAll(name, "/", "_")+".txt", "btrfs", "fi", "usage", m)
			}
		}

		// Config files (redacted): /etc/nos/*.yaml, pools.json, schedules.yaml; fstab/crypttab
		nosDir := filepath.Join(cfg.EtcDir, "nos")
		if matches, _ := filepath.Glob(filepath.Join(nosDir, "*.yaml")); len(matches) > 0 {
			for _, p := range matches {
				name := filepath.Base(p)
				writeTarFileIfExists(tw, filepath.Join(nosDir, name), filepath.Join("configs/nos", name))
			}
		}
		writeTarFileIfExists(tw, filepath.Join(nosDir, "pools.json"), "configs/nos/pools.json")
		writeTarFileIfExists(tw, filepath.Join(cfg.EtcDir, "fstab"), "system/fstab")
		writeTarFileIfExists(tw, filepath.Join(cfg.EtcDir, "crypttab"), "system/crypttab")

		// SMART snapshots
		if matches, _ := filepath.Glob(filepath.Join("/var/lib/nos/health/smart", "*.json")); len(matches) > 0 {
			for _, p := range matches {
				name := filepath.Base(p)
				writeTarFileIfExists(tw, p, filepath.Join("health/smart", name))
			}
		}

		// Pool transactions (last N=10)
		txDir := filepath.Join("/var/lib/nos", "pools", "tx")
		if entries, err := os.ReadDir(txDir); err == nil {
			files := make([]string, 0, len(entries))
			for _, e := range entries {
				files = append(files, e.Name())
			}
			start := 0
			if len(files) > 10 {
				start = len(files) - 10
			}
			for _, name := range files[start:] {
				writeTarFileIfExists(tw, filepath.Join(txDir, name), filepath.Join("pools/tx", name))
			}
		}
	}
}
