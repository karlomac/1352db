package replication

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stonefish/pdb-go/pkg/adapter"
)

func TestSQLiteReplication(t *testing.T) {
	os.Remove("/tmp/s.db")
	os.Remove("/tmp/t.db")
	src, _ := adapter.NewSQLiteAdapter("/tmp/s.db", "src")
	defer src.Close()
	tgt, _ := adapter.NewSQLiteAdapter("/tmp/t.db", "tgt")
	defer tgt.Close()
	for i := 0; i < 5; i++ {
		src.Put(context.Background(), adapter.Document{ID: fmt.Sprintf("d%d", i), Extra: map[string]any{"n": i}})
	}
	r := New(src, tgt, Options{BatchSize: 2})
	res := r.Run(context.Background())
	if !res.OK || res.DocsWritten != 5 {
		t.Fatalf("res=%+v", res)
	}
	// idempotent
	r2 := New(src, tgt, Options{BatchSize: 2})
	res2 := r2.Run(context.Background())
	if res2.DocsWritten != 0 {
		t.Fatalf("expected 0, got %d", res2.DocsWritten)
	}
}
