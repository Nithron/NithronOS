package hash

import (
	"bytes"
	"math/rand"
	"testing"
)

// 100 bytes overwritten in the middle of a file of 9 blocks and a short
// one: one block is new data, the rest (the short last block too) are copies.
func TestDeltaPlanOverwrite(t *testing.T) {
	const bs = 64 << 10
	rng := rand.New(rand.NewSource(3))
	old := make([]byte, 9*bs+bs/3) // and a short last block
	rng.Read(old)
	remote, err := ComputeBlockHashesFromReader(bytes.NewReader(old), int64(len(old)), bs)
	if err != nil {
		t.Fatal(err)
	}
	edited := append([]byte{}, old...)
	rng.Read(edited[5*bs : 5*bs+100])
	plan, err := ComputeDeltaPlanFromReader(bytes.NewReader(edited), int64(len(edited)), remote, bs)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SavedBytes != 8*bs+bs/3 {
		var kinds []string
		for _, op := range plan.Operations {
			kinds = append(kinds, op.Type)
		}
		t.Fatalf("saved %d bytes, want %d; ops %v", plan.SavedBytes, 8*bs+bs/3, kinds)
	}
	if !bytes.Equal(applyPlan(old, plan), edited) {
		t.Fatal("the plan doesn't rebuild the edited file")
	}
}

func applyPlan(old []byte, plan *DeltaPlan) []byte {
	var out []byte
	for _, op := range plan.Operations {
		if op.Type == "copy" {
			out = append(out, old[op.Offset:op.Offset+int64(op.Size)]...)
		} else {
			out = append(out, op.Data...)
		}
	}
	return out
}
