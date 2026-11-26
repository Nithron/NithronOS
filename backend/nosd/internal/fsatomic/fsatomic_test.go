package fsatomic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConcurrentSaveJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// Run N concurrent SaveJSON calls; last write wins
	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := WithLock(path, func() error {
				return SaveJSON(path, map[string]int{"i": i}, 0)
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("save error: %v", err)
		}
	}
	// Wait briefly for file to appear (Windows FS latency)
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Validate JSON
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var v map[string]int
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("json: %v", err)
	}
}

func TestLoadIgnoresTmp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// write good file
	if err := SaveJSON(path, map[string]string{"a": "b"}, 0o600); err != nil {
		t.Fatal(err)
	}
	// create crash artifact
	if err := os.WriteFile(path+".tmp", []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	ok, err := LoadJSON(path, &got)
	if err != nil || !ok {
		t.Fatalf("load: %v ok=%v", err, ok)
	}
	if got["a"] != "b" {
		t.Fatalf("want b, got %v", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp should be removed, err=%v", err)
	}
}
