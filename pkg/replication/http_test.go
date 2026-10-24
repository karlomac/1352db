package replication

import (
	"context"
	"fmt"
	"testing"

	"github.com/stonefish/pdb-go/internal/mockcouch"
	"github.com/stonefish/pdb-go/pkg/adapter"
)

func TestReplicationOverHTTP(t *testing.T) {
	srcSrv := mockcouch.StartServer()
	defer srcSrv.Close()
	tgtSrv := mockcouch.StartServer()
	defer tgtSrv.Close()

	src := adapter.NewHTTPAdapter(srcSrv.URL, "testdb")
	tgt := adapter.NewHTTPAdapter(tgtSrv.URL, "testdb")

	for i := 0; i < 4; i++ {
		d := adapter.Document{ID: fmt.Sprintf("doc%d", i), Extra: map[string]any{"n": i}}
		if _, err := src.Put(context.Background(), d); err != nil {
			t.Fatal(err)
		}
	}

	r := New(src, tgt, Options{BatchSize: 2})
	res := r.Run(context.Background())
	if !res.OK {
		t.Fatalf("replication failed: %v", res.Errors)
	}
	if res.DocsWritten != 4 {
		t.Fatalf("expected 4 written, got %d", res.DocsWritten)
	}
	for i := 0; i < 4; i++ {
		if _, err := tgt.Get(context.Background(), fmt.Sprintf("doc%d", i), ""); err != nil {
			t.Fatalf("missing doc%d: %v", i, err)
		}
	}
}
