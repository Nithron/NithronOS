// Command route-dump prints the nosd API: its routes (default) or an OpenAPI
// document generated from them (-format openapi, see docs/api/openapi.json).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/server"
)

func main() {
	format := flag.String("format", "routes", "routes or openapi")
	release := flag.String("version", "", "info.version of the OpenAPI document (default: the build's version)")
	flag.Parse()

	cleanup, err := isolateState()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer cleanup()
	r := server.NewRouter(config.FromEnv()).(*chi.Mux)
	routes := server.ListRoutes(r)
	var out any = routes
	switch *format {
	case "routes":
	case "openapi":
		doc := server.OpenAPIDocument(routes)
		if *release != "" {
			doc["info"].(map[string]any)["version"] = *release
		}
		out = doc
	default:
		fmt.Fprintf(os.Stderr, "unknown -format %q\n", *format)
		os.Exit(2)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	cleanup()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

// isolateState points every state file at a temporary directory, so all
// services start (and register their routes) wherever this runs: the output
// must not depend on the machine.
func isolateState() (func(), error) {
	dir, err := os.MkdirTemp("", "nos-route-dump-")
	if err != nil {
		return nil, err
	}
	env := map[string]string{
		"NOS_ETC_DIR":            filepath.Join(dir, "etc"),
		"NOS_USERS_PATH":         filepath.Join(dir, "etc", "users.json"),
		"NOS_SECRET_PATH":        filepath.Join(dir, "etc", "secret.key"),
		"NOS_SESSIONS_PATH":      filepath.Join(dir, "lib", "sessions.json"),
		"NOS_RL_PATH":            filepath.Join(dir, "lib", "ratelimit.json"),
		"NOS_FIRSTBOOT_PATH":     filepath.Join(dir, "lib", "firstboot.json"),
		"NOS_SHARES_PATH":        filepath.Join(dir, "etc", "shares.json"),
		"NOS_APPS_STATE":         filepath.Join(dir, "etc", "apps.json"),
		"NOS_APPS_DATA_DIR":      filepath.Join(dir, "lib", "apps"),
		"NOS_APPS_INSTALL_DIR":   filepath.Join(dir, "srv", "apps"),
		"NOS_DISABLE_APP_EVENTS": "1",
	}
	for k, v := range env {
		if err := os.Setenv(k, v); err != nil {
			return nil, err
		}
	}
	return func() { _ = os.RemoveAll(dir) }, nil
}
