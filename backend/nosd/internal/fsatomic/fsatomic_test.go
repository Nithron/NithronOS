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
	// A reader must not delete it: it may be a concurrent writer's file.
	if _, err := os.Stat(path + ".tmp"); err != nil {
		t.Fatalf("a reader removed the temp file: %v", err)
	}
}

// Regression: LoadJSON deleted path+".tmp", the file a concurrent SaveJSON
// was about to rename into place, so that save failed and was lost (pool
// transactions stuck in "running" while the UI polled them).
func TestSavesSurviveConcurrentReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	stop := make(chan struct{})
	readers := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			defer func() { readers <- struct{}{} }()
			var v map[string]int
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = LoadJSON(path, &v)
				}
			}
		}()
	}
	const n = 300
	for i := 1; i <= n; i++ {
		if err := SaveJSON(path, map[string]int{"n": i}, 0o600); err != nil {
			close(stop)
			t.Fatalf("save %d failed while readers ran: %v", i, err)
		}
	}
	close(stop)
	for i := 0; i < 4; i++ {
		<-readers
	}
	var got map[string]int
	if _, err := LoadJSON(path, &got); err != nil || got["n"] != n {
		t.Fatalf("last save lost: %v %v", got, err)
	}
	if left, _ := filepath.Glob(path + ".tmp*"); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}
