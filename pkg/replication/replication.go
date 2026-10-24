// Package replication implements a cdb-compatible pull replication engine.
package replication

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stonefish/pdb-go/pkg/adapter"
	"github.com/stonefish/pdb-go/pkg/checkpointer"
)

// Options configure a replication.
type Options struct {
	Continuous   bool
	BatchSize    int
	BatchesLimit int
	Style        string
	DocIDs       []string
	Selector     string
	Filter       string
	Heartbeat    int
	Timeout      int
	Retry        bool
	Checkpoint   string // "both"|"source"|"target"|"none"
	Since        string
}

// Result is the replication result.
type Result struct {
	OK               bool     `json:"ok"`
	StartTime        string   `json:"start_time"`
	EndTime          string   `json:"end_time"`
	DocsRead         int      `json:"docs_read"`
	DocsWritten      int      `json:"docs_written"`
	DocWriteFailures int      `json:"doc_write_failures"`
	Errors           []string `json:"errors,omitempty"`
	LastSeq          string   `json:"last_seq"`
	Status           string   `json:"status"`
}

// Replicator runs a single replication from src to target.
type Replicator struct {
	Src    adapter.Adapter
	Target adapter.Adapter
	Opts   Options
	Result Result
}

func New(src, target adapter.Adapter, opts Options) *Replicator {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	if opts.BatchesLimit <= 0 {
		opts.BatchesLimit = 10
	}
	if opts.Style == "" {
		opts.Style = "all_docs"
	}
	if opts.Checkpoint == "" {
		opts.Checkpoint = "both"
	}
	return &Replicator{Src: src, Target: target, Opts: opts}
}

// Run executes one replication round (non-continuous). Returns result.
func (r *Replicator) Run(ctx context.Context) Result {
	r.Result = Result{OK: true, StartTime: time.Now().UTC().Format(time.RFC3339), Status: "running"}
	session := newSessionID()

	// checkpointer setup
	cpOpts := checkpointer.DefaultOpts()
	switch r.Opts.Checkpoint {
	case "none":
		cpOpts = checkpointer.Opts{WriteSource: false, WriteTarget: false}
	case "source":
		cpOpts = checkpointer.Opts{WriteSource: true, WriteTarget: false}
	case "target":
		cpOpts = checkpointer.Opts{WriteSource: false, WriteTarget: true}
	}
	srcInfo, _ := r.Src.Info(ctx)
	tgtInfo, _ := r.Target.Info(ctx)
	replID := "repl-" + srcInfo.DBName + "-" + tgtInfo.DBName
	cp := checkpointer.New(r.Src, r.Target, replID, cpOpts)

	lastSeq := r.Opts.Since
	if lastSeq == "" {
		cpSeq, err := cp.GetCheckpoint()
		if err == nil && cpSeq != "" {
			lastSeq = cpSeq
		} else {
			lastSeq = "0"
		}
	} else {
		// write checkpoint at requested since
		if r.Opts.Checkpoint != "none" {
			cp.WriteCheckpoint(lastSeq, session)
		}
	}
	r.Result.LastSeq = lastSeq

	// Read changes from source since lastSeq in batches
	for {
		if ctx.Err() != nil {
			r.Result.OK = false
			r.Result.Status = "cancelled"
			r.Result.EndTime = time.Now().UTC().Format(time.RFC3339)
			return r.Result
		}
		changesOpts := adapter.ChangesOpts{
			Since:       lastSeq,
			Limit:       r.Opts.BatchSize,
			BatchSize:   r.Opts.BatchSize,
			Style:       r.Opts.Style,
			IncludeDocs: r.Opts.Filter != "",
			ReturnDocs:  true,
		}
		ch, err := r.Src.Changes(ctx, changesOpts)
		if err != nil {
			r.Result.OK = false
			r.Result.Errors = append(r.Result.Errors, err.Error())
			r.Result.Status = "error"
			r.Result.EndTime = time.Now().UTC().Format(time.RFC3339)
			return r.Result
		}
		var batch []adapter.Change
		for c := range ch {
			batch = append(batch, c)
		}
		if len(batch) == 0 {
			break
		}
		// revsDiff against target
		diff := map[string][]string{}
		for _, c := range batch {
			revs := []string{}
			for _, chg := range c.Changes {
				revs = append(revs, chg.Rev)
			}
			diff[c.ID] = revs
		}
		diffs, err := r.Target.RevsDiff(ctx, diff)
		if err != nil {
			r.Result.OK = false
			r.Result.Errors = append(r.Result.Errors, err.Error())
			r.Result.Status = "error"
			r.Result.EndTime = time.Now().UTC().Format(time.RFC3339)
			return r.Result
		}
		// Build bulk_get requests
		var reqs []adapter.BulkGetRequest
		for id, rd := range diffs {
			for _, rev := range rd.Missing {
				reqs = append(reqs, adapter.BulkGetRequest{ID: id, Rev: rev})
			}
		}
		// fetch docs from source via BulkGet
		var docs []adapter.Document
		if len(reqs) > 0 {
			bg, err := r.Src.BulkGet(ctx, reqs)
			if err != nil {
				r.Result.OK = false
				r.Result.Errors = append(r.Result.Errors, err.Error())
				r.Result.Status = "error"
				r.Result.EndTime = time.Now().UTC().Format(time.RFC3339)
				return r.Result
			}
			for _, res := range bg.Results {
				for _, d := range res.Docs {
					if d.OK != nil {
						docs = append(docs, *d.OK)
						r.Result.DocsRead++
					}
				}
			}
		}
		// write via target.bulkDocs new_edits:false
		if len(docs) > 0 {
			results, err := r.Target.BulkDocs(ctx, docs, false)
			if err != nil {
				r.Result.OK = false
				r.Result.Errors = append(r.Result.Errors, err.Error())
				r.Result.Status = "error"
				r.Result.EndTime = time.Now().UTC().Format(time.RFC3339)
				return r.Result
			}
			for _, res := range results {
				if res.Error != "" {
					r.Result.DocWriteFailures++
					r.Result.Errors = append(r.Result.Errors, res.Error+": "+res.Message)
					// denied handling
					if res.Error == "unauthorized" || res.Error == "forbidden" {
						// emit denied (no-op here)
					}
				} else {
					r.Result.DocsWritten++
				}
			}
		}
		// update lastSeq from last change
		if len(batch) > 0 {
			lastSeq = batch[len(batch)-1].Seq
		}
		r.Result.LastSeq = lastSeq
		if r.Opts.Checkpoint != "none" {
			cp.WriteCheckpoint(lastSeq, session)
		}

		// if fewer than batch size, done
		if len(batch) < r.Opts.BatchSize {
			break
		}
	}

	r.Result.Status = "complete"
	r.Result.EndTime = time.Now().UTC().Format(time.RFC3339)
	return r.Result
}

// JSON helper
func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

var _ = fmt.Sprintf

func newSessionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
