// Package checkpointer implements durable replication checkpoints stored in
// _local docs on source and target, mirroring pouchdb-checkpointer.
package checkpointer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stonefish/pdb-go/pkg/adapter"
	"github.com/stonefish/pdb-go/pkg/collate"
)

const checkpointVersion = 1
const replicator = "pdb"
const checkpointHistorySize = 5

type CheckpointDoc struct {
	ID         string         `json:"-"`
	SessionID  string         `json:"session_id"`
	LastSeq    string         `json:"last_seq"`
	Version    int            `json:"version"`
	Replicator string         `json:"replicator"`
	History    []HistoryEntry `json:"history"`
	Rev        string         `json:"-"`
}

type HistoryEntry struct {
	LastSeq   string `json:"last_seq"`
	SessionID string `json:"session_id"`
}

type Checkpointer struct {
	Src    adapter.Adapter
	Target adapter.Adapter
	ID     string
	Opts   Opts
}

type Opts struct {
	WriteSource bool
	WriteTarget bool
}

func New(src, target adapter.Adapter, id string, opts Opts) *Checkpointer {
	if !opts.WriteSource && !opts.WriteTarget {
		// default both
	}
	if opts.WriteSource == false && opts.WriteTarget == false {
		// leave as-is; caller may set false intentionally for "none"
	}
	return &Checkpointer{Src: src, Target: target, ID: id, Opts: opts}
}

func DefaultOpts() Opts { return Opts{WriteSource: true, WriteTarget: true} }

func (c *Checkpointer) docID() string { return "_local/" + c.ID }

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "not_found") || strings.Contains(s, "404")
}

func (c *Checkpointer) getDoc(db adapter.Adapter) (CheckpointDoc, error) {
	d, err := db.Get(context.Background(), c.docID(), "")
	if err != nil {
		if isNotFound(err) {
			return CheckpointDoc{}, fmt.Errorf("not_found")
		}
		return CheckpointDoc{}, err
	}
	// unmarshal from adapter.Document Extra? _local docs store fields in Extra since schema unknown.
	// Reconstruct via JSON round-trip of Extra.
	b, _ := json.Marshal(d.Extra)
	var cd CheckpointDoc
	_ = json.Unmarshal(b, &cd)
	cd.ID = d.ID
	if cd.LastSeq == "" {
		// maybe stored top-level? Extra includes last_seq
	}
	if d.Rev != "" {
		cd.Rev = d.Rev
	}
	// also handle case where doc has _id
	if cd.ID == "" {
		cd.ID = d.ID
	}
	return cd, nil
}

func (c *Checkpointer) putDoc(db adapter.Adapter, doc CheckpointDoc) error {
	// build adapter.Document with Extra fields
	extra := map[string]any{}
	b, _ := json.Marshal(doc)
	json.Unmarshal(b, &extra)
	// remove _id? keep
	adoc := adapter.Document{ID: c.docID(), Rev: doc.Rev, Extra: extra}
	// ensure _id in extra
	adoc.Extra["_id"] = c.docID()
	_, err := db.Put(context.Background(), adoc)
	if err != nil {
		// conflict retry
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "conflict") {
			return c.putDoc(db, doc)
		}
		return err
	}
	return nil
}

func (c *Checkpointer) WriteCheckpoint(checkpoint, session string) error {
	// update target before source
	if c.Opts.WriteTarget {
		if err := c.updateCheckpoint(c.Target, checkpoint, session); err != nil {
			return err
		}
	}
	if c.Opts.WriteSource {
		if err := c.updateCheckpoint(c.Src, checkpoint, session); err != nil {
			// forbidden => disable source writes
			if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "forbidden") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
				c.Opts.WriteSource = false
				return nil
			}
			return err
		}
	}
	return nil
}

func (c *Checkpointer) updateCheckpoint(db adapter.Adapter, checkpoint, session string) error {
	doc, err := c.getDoc(db)
	if err != nil && !strings.Contains(err.Error(), "not_found") {
		return err
	}
	if strings.Contains(err.Error(), "not_found") {
		doc = CheckpointDoc{
			ID:         c.docID(),
			SessionID:  session,
			LastSeq:    checkpoint,
			Version:    checkpointVersion,
			Replicator: replicator,
			History:    []HistoryEntry{},
		}
	}
	if doc.LastSeq == checkpoint {
		return nil
	}
	// filter out current session
	var hist []HistoryEntry
	for _, h := range doc.History {
		if h.SessionID != session {
			hist = append(hist, h)
		}
	}
	hist = append([]HistoryEntry{{LastSeq: checkpoint, SessionID: session}}, hist...)
	if len(hist) > checkpointHistorySize {
		hist = hist[:checkpointHistorySize]
	}
	doc.History = hist
	doc.SessionID = session
	doc.LastSeq = checkpoint
	doc.Version = checkpointVersion
	doc.Replicator = replicator
	return c.putDoc(db, doc)
}

func (c *Checkpointer) GetCheckpoint() (string, error) {
	if !c.Opts.WriteSource && !c.Opts.WriteTarget {
		return "0", nil
	}
	if c.Opts.WriteSource && !c.Opts.WriteTarget {
		doc, err := c.getDoc(c.Src)
		if err != nil {
			if strings.Contains(err.Error(), "not_found") {
				return "0", nil
			}
			return "0", err
		}
		if doc.LastSeq == "" {
			return "0", nil
		}
		return doc.LastSeq, nil
	}
	// target present (possibly source too)
	targetDoc, err := c.getDoc(c.Target)
	if err != nil {
		if strings.Contains(err.Error(), "not_found") {
			return "0", nil
		}
		return "0", err
	}
	if c.Opts.WriteTarget && !c.Opts.WriteSource {
		if targetDoc.LastSeq == "" {
			return "0", nil
		}
		return targetDoc.LastSeq, nil
	}
	// both
	sourceDoc, err := c.getDoc(c.Src)
	if err != nil {
		if strings.Contains(err.Error(), "not_found") {
			if targetDoc.LastSeq != "" {
				// write source checkpoint 0
				sd := CheckpointDoc{ID: c.docID(), LastSeq: "0", Version: checkpointVersion, Replicator: replicator}
				c.putDoc(c.Src, sd)
			}
			return "0", nil
		}
		return "0", err
	}
	if sourceDoc.Version != targetDoc.Version {
		return "0", nil
	}
	return compareHistory(sourceDoc, targetDoc), nil
}

func compareHistory(srcDoc, tgtDoc CheckpointDoc) string {
	if srcDoc.SessionID == tgtDoc.SessionID {
		return srcDoc.LastSeq
	}
	return compareHistoryLogs(srcDoc.History, tgtDoc.History)
}

func compareHistoryLogs(srcHist, tgtHist []HistoryEntry) string {
	if len(srcHist) == 0 || len(tgtHist) == 0 {
		return "0"
	}
	s := srcHist[0]
	// rest
	srcRest := srcHist[1:]
	// target first
	t := tgtHist[0]
	tgtRest := tgtHist[1:]

	if hasSession(s.SessionID, tgtHist) {
		return s.LastSeq
	}
	if hasSession(t.SessionID, srcRest) {
		return t.LastSeq
	}
	return compareHistoryLogs(srcRest, tgtRest)
}

func hasSession(id string, hist []HistoryEntry) bool {
	if id == "" {
		return false
	}
	for _, h := range hist {
		if h.SessionID == id {
			return true
		}
	}
	return false
}

// collate import kept for future seq comparison
var _ = collate.Compare
