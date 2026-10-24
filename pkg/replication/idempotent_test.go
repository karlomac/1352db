package replication

import (
	"context"
	"fmt"
	"testing"

	"github.com/stonefish/pdb-go/pkg/adapter"
)

func TestReplicationIdempotent(t *testing.T) {
	src := adapter.NewMemoryAdapter()
	tgt := adapter.NewMemoryAdapter()
	for i := 0; i < 3; i++ {
		src.Put(context.Background(), adapter.Document{ID: fmt.Sprintf("d%d", i)})
	}
	r := New(src, tgt, Options{BatchSize: 10})
	r.Run(context.Background())
	// second run should write 0
	r2 := New(src, tgt, Options{BatchSize: 10})
	res := r2.Run(context.Background())
	if res.DocsWritten != 0 {
		t.Fatalf("expected 0 written on second run, got %d", res.DocsWritten)
	}
}
