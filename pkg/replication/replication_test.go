package replication

import (
	"context"
	"fmt"
	"testing"

	"github.com/stonefish/pdb-go/pkg/adapter"
)

func TestReplicationRoundtrip(t *testing.T) {
	src := adapter.NewMemoryAdapter()
	tgt := adapter.NewMemoryAdapter()

	// seed source
	for i := 0; i < 5; i++ {
		doc := adapter.Document{ID: fmt.Sprintf("doc%d", i), Extra: map[string]any{"n": i}}
		if _, err := src.Put(context.Background(), doc); err != nil {
			t.Fatal(err)
		}
	}

	r := New(src, tgt, Options{BatchSize: 2})
	res := r.Run(context.Background())
	if !res.OK {
		t.Fatalf("replication failed: %v", res.Errors)
	}
	if res.DocsRead != 5 || res.DocsWritten != 5 {
		t.Fatalf("expected 5 read/written, got read=%d written=%d", res.DocsRead, res.DocsWritten)
	}
	// verify target has docs
	info, _ := tgt.Info(context.Background())
	t.Logf("target seq=%s", info.Seq)
	for i := 0; i < 5; i++ {
		d, err := tgt.Get(context.Background(), fmt.Sprintf("doc%d", i), "")
		if err != nil {
			t.Fatalf("missing doc%d: %v", i, err)
		}
		if d.ID != fmt.Sprintf("doc%d", i) {
			t.Fatalf("bad id %s", d.ID)
		}
	}
}
