package fsatomic

import (
	"os"
	"path/filepath"
	"testing"
)

type dummy struct {
	A string `json:"a"`
	B int    `json:"b"`
}

func FuzzLoadJSON_Truncated(f *testing.F) {
	dir := f.TempDir()
	path := filepath.Join(dir, "state.json")
	// seed with valid
	_ = SaveJSON(path, dummy{A: "x", B: 1}, 0o600)
	f.Add([]byte("{"))
	f.Add([]byte("{\n\"a\":"))
	f.Fuzz(func(t *testing.T, partial []byte) {
		// Write partial to .tmp and simulate crash
		_ = os.WriteFile(path+".tmp", partial, 0o600)
		var out dummy
		_, _ = LoadJSON(path, &out)
		// Clean up .tmp for next iteration
		_ = os.Remove(path + ".tmp")
	})
}
