package apps

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeHost struct{ ops []AppOp }

func (f *fakeHost) AppOp(_ context.Context, op AppOp) (string, error) {
	f.ops = append(f.ops, op)
	if op.Op == "prepare" {
		// The agent creates the folders; here nosd may write config.
		if err := os.MkdirAll(filepath.Join(testAppsRoot, op.AppID, "config"), 0o750); err != nil {
			return "", err
		}
	}
	if op.Op == "group" {
		return "998\n", nil
	}
	if op.Op == "snapshot" {
		return "/srv/apps/.snapshots/" + op.AppID + "/20261126-120000-" + op.Name, nil
	}
	return "", nil
}

func (f *fakeHost) names() string {
	var out []string
	for _, o := range f.ops {
		out = append(out, o.Op)
	}
	return strings.Join(out, ",")
}

var testAppsRoot string

// Installing goes through nos-agent for every privileged step; nosd only
// writes the compose and env files.
func TestInstallUsesTheAgent(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	builtin := filepath.Join(filepath.Dir(here), "..", "..", "..", "..", "usr", "share", "nos", "apps")
	tmp := t.TempDir()
	testAppsRoot = filepath.Join(tmp, "srv-apps")

	catalog := NewCatalogManager(builtin, filepath.Join(tmp, "cache.json"), filepath.Join(tmp, "sources"))
	state, err := NewStateStore(filepath.Join(tmp, "apps.json"))
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{}
	lm := NewLifecycleManager(catalog, state, NewTemplateRenderer(builtin), testAppsRoot, host, nil)

	// A user-sent NOS_APP_GID is replaced by the agent's answer.
	err = lm.InstallApp(context.Background(), InstallRequest{ID: "whoami", Params: map[string]interface{}{"PUID": "1001", AppGIDParam: "0"}}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if got := host.names(); got != "prepare,group,snapshot,start,proxy" {
		t.Fatalf("agent ops: %s", got)
	}
	if prep := host.ops[0]; prep.UID != 1001 || prep.GID != 1000 {
		t.Fatalf("data owner: %+v", prep)
	}
	if proxy := host.ops[4]; proxy.Port != 8080 {
		t.Fatalf("proxy port (host port, not the container name): %+v", proxy)
	}
	if _, err := os.Stat(filepath.Join(testAppsRoot, "whoami", "config", "docker-compose.yml")); err != nil {
		t.Fatalf("compose file: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(testAppsRoot, "whoami", "config", ".env"))
	if err != nil || !strings.Contains(string(env), AppGIDParam+"=998\n") {
		t.Fatalf("env file must carry the share group id: %v\n%s", err, env)
	}
	app, err := state.GetApp("whoami")
	if err != nil || len(app.Snapshots) != 1 || app.Snapshots[0].ID != "20261126-120000-post-install" {
		t.Fatalf("state: %v %+v", err, app)
	}

	host.ops = nil
	if err := lm.DeleteApp(context.Background(), "whoami", false, "admin"); err != nil {
		t.Fatal(err)
	}
	if got := host.names(); got != "stop,disable,unproxy,remove" || host.ops[3].KeepData {
		t.Fatalf("delete ops: %s %+v", got, host.ops)
	}
	if _, err := state.GetApp("whoami"); err == nil {
		t.Fatal("app still in state")
	}
}

func TestRendererUsesCatalogResources(t *testing.T) {
	tr := NewTemplateRenderer(t.TempDir())
	compose := "services:\n  app:\n    image: x\n  db:\n    image: y\n    deploy:\n      resources:\n        limits:\n          memory: 64m\n"
	out := tr.applySecurityDefaults(compose, false, ResourceLimits{CPULimit: "4.0", MemoryLimit: "4096m"})
	if !strings.Contains(out, "cpus: \"4.0\"") || !strings.Contains(out, "memory: 4096m") {
		t.Fatalf("catalog limits ignored:\n%s", out)
	}
	if !strings.Contains(out, "memory: 64m") {
		t.Fatalf("a template's own deploy block must stay:\n%s", out)
	}
	if out := tr.applySecurityDefaults("services:\n  app:\n    image: x\n", false, ResourceLimits{}); !strings.Contains(out, "memory: 1024M") {
		t.Fatalf("default limits:\n%s", out)
	}
}

// The nAI template joins the group nOS hands it, so it can read the shares
// opened to it.
func TestNAITemplateGetsTheShareGroup(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	builtin := filepath.Join(filepath.Dir(here), "..", "..", "..", "..", "usr", "share", "nos", "apps")
	catalog := NewCatalogManager(builtin, filepath.Join(t.TempDir(), "cache.json"), filepath.Join(t.TempDir(), "sources"))
	entry, err := catalog.GetEntry("nai")
	if err != nil {
		t.Skipf("no nai entry in the built-in catalog: %v", err)
	}
	out, err := NewTemplateRenderer(builtin).RenderComposeFile(entry, map[string]interface{}{AppGIDParam: "977"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `- "977"`) {
		t.Fatalf("group_add does not carry NOS_APP_GID:\n%s", out)
	}
}

// A snapshot taken on request goes through the agent and is recorded.
func TestSnapshotAppRecordsIt(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	builtin := filepath.Join(filepath.Dir(here), "..", "..", "..", "..", "usr", "share", "nos", "apps")
	tmp := t.TempDir()
	testAppsRoot = filepath.Join(tmp, "srv-apps")
	catalog := NewCatalogManager(builtin, filepath.Join(tmp, "cache.json"), filepath.Join(tmp, "sources"))
	state, _ := NewStateStore(filepath.Join(tmp, "apps.json"))
	host := &fakeHost{}
	lm := NewLifecycleManager(catalog, state, NewTemplateRenderer(builtin), testAppsRoot, host, nil)
	if _, err := lm.SnapshotApp(context.Background(), "whoami", "admin"); err == nil {
		t.Fatal("snapshot of an app that isn't installed")
	}
	if err := lm.InstallApp(context.Background(), InstallRequest{ID: "whoami", Params: map[string]interface{}{}}, "admin"); err != nil {
		t.Fatal(err)
	}
	snap, err := lm.SnapshotApp(context.Background(), "whoami", "admin")
	if err != nil || snap.Name != "manual" || snap.ID != "20261126-120000-manual" {
		t.Fatalf("snapshot: %+v %v", snap, err)
	}
	app, _ := state.GetApp("whoami")
	if len(app.Snapshots) != 2 || app.Snapshots[1].ID != snap.ID {
		t.Fatalf("recorded: %+v", app.Snapshots)
	}
}
