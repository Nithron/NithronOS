package hash

import (
	"bytes"
	"hash/adler32"
	"math/rand"
	"testing"
)

// The rolling window gives adler32.Checksum's value at every offset (the
// server hashes its blocks with adler32.Checksum).
func TestAdlerWindowMatchesChecksum(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	data := make([]byte, 20000)
	rng.Read(data)
	for i := range data[:3000] {
		data[i] = 0xff // long runs of large bytes stress the modular arithmetic
	}
	const n = 4096
	w := newAdlerWindow(data[:n])
	for pos := 0; pos+n <= len(data); pos++ {
		if got, want := w.sum(), adler32.Checksum(data[pos:pos+n]); got != want {
			t.Fatalf("offset %d: %08x, want %08x", pos, got, want)
		}
		if pos+n < len(data) {
			w.roll(data[pos], data[pos+n])
		}
	}
}

// An edit in the middle keeps the blocks around it as copies, a block that
// repeats is reused, and applying the plan to the old file gives the new one.
func TestDeltaPlanRebuildsTheFile(t *testing.T) {
	const bs = 1024
	rng := rand.New(rand.NewSource(2))
	old := make([]byte, 8*bs)
	rng.Read(old)
	remote, err := ComputeBlockHashesFromReader(bytes.NewReader(old), int64(len(old)), bs)
	if err != nil {
		t.Fatal(err)
	}
	// New file: block 0, 100 inserted bytes, blocks 1..7, and block 0 again.
	ins := make([]byte, 100)
	rng.Read(ins)
	newData := append(append(append([]byte{}, old[:bs]...), ins...), old[bs:]...)
	newData = append(newData, old[:bs]...)

	plan, err := ComputeDeltaPlanFromReader(bytes.NewReader(newData), int64(len(newData)), remote, bs)
	if err != nil {
		t.Fatal(err)
	}
	var rebuilt []byte
	for _, op := range plan.Operations {
		if op.Type == "copy" {
			rebuilt = append(rebuilt, old[op.Offset:op.Offset+int64(op.Size)]...)
		} else {
			rebuilt = append(rebuilt, op.Data...)
		}
	}
	if !bytes.Equal(rebuilt, newData) {
		t.Fatal("the plan doesn't rebuild the new file")
	}
	if plan.SavedBytes != 9*bs {
		t.Fatalf("saved %d bytes, want %d (8 blocks, block 0 twice)", plan.SavedBytes, 9*bs)
	}
}
