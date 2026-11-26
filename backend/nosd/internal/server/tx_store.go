package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
	"nithronos/backend/nosd/internal/pools"
)

func txDir() string {
	base := os.Getenv("NOS_STATE_DIR")
	if base == "" {
		base = "/var/lib/nos"
	}
	return filepath.Join(base, "pools", "tx")
}

func txPath(id string) string    { return filepath.Join(txDir(), id+".json") }
func txLogPath(id string) string { return filepath.Join(txDir(), id+".log") }

func saveTx(t pools.Tx) error {
	_ = os.MkdirAll(txDir(), 0o755)
	return fsatomic.SaveJSON(txPath(t.ID), t, 0o600)
}

func appendTxLog(id string, level, stepID, msg string) {
	_ = os.MkdirAll(txDir(), 0o755)
	f, err := os.OpenFile(txLogPath(id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	rec := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339), "level": level, "stepId": stepID, "msg": msg}
	b, _ := json.Marshal(rec)
	fmt.Fprintln(f, string(b))
}

func readLogTail(id string, cursor, max int) (lines []string, next int) {
	f, err := os.Open(txLogPath(id))
	if err != nil {
		return []string{}, cursor
	}
	defer f.Close()
	r := bufio.NewScanner(f)
	idx := 0
	for r.Scan() {
		if idx >= cursor {
			lines = append(lines, r.Text())
			if len(lines) >= max {
				break
			}
		}
		idx++
	}
	return lines, idx
}
