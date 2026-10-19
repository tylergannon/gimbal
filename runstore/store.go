// Package runstore defines the portable identity and recording boundary for a
// workflow run. Opening a run acquires exclusive writer ownership; it does not
// resume execution. Implementations must serialize concurrent handle methods.
package runstore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/oklog/ulid/v2"
)

type RunID string
type NodeID string
type RecordID string

func NewRunID() RunID        { return RunID(ulid.Make().String()) }
func NewNodeID() NodeID      { return NodeID(ulid.Make().String()) }
func NewRecordID() RecordID  { return RecordID(ulid.Make().String()) }
func ValidID(id string) bool { _, err := ulid.ParseStrict(id); return err == nil }

var (
	ErrExists     = errors.New("runstore: run already exists")
	ErrBusy       = errors.New("runstore: writer is busy")
	ErrIncomplete = errors.New("runstore: incomplete initialization")
	ErrNotFound   = errors.New("runstore: not found")
	ErrConflict   = errors.New("runstore: conflicting record")
	ErrCorrupt    = errors.New("runstore: corrupt data")
	ErrClosed     = errors.New("runstore: handle closed")
)

type Kind string

const (
	Scope    Kind = "scope"
	Session  Kind = "session"
	Action   Kind = "action"
	Service  Kind = "service"
	Question Kind = "question"
)

type Header struct {
	Version   int       `json:"version"`
	RunID     RunID     `json:"run_id"`
	Workflow  string    `json:"workflow"`
	Root      NodeID    `json:"root"`
	CreatedAt time.Time `json:"created_at"`
}

type Slot struct {
	Parent  NodeID `json:"parent,omitempty"`
	Kind    Kind   `json:"kind"`
	Name    string `json:"name"`
	Ordinal int    `json:"ordinal"`
}

type NodeSpec struct {
	Slot
	DisplayKey string          `json:"display_key"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

type Node struct {
	ID NodeID `json:"id"`
	NodeSpec
}

type Fact struct {
	Version int             `json:"version"`
	ID      RecordID        `json:"id"`
	Node    NodeID          `json:"node"`
	Scope   NodeID          `json:"scope,omitempty"`
	Session NodeID          `json:"session,omitempty"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type Record struct {
	Sequence uint64 `json:"sequence"`
	Fact
}

type Durability uint8

const (
	Buffered  Durability = iota // Complete OS write; not a durability acknowledgment.
	Committed                   // Synced through the returned sequence.
)

type BlobRef struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Store interface {
	Create(context.Context, Header) (Run, error)
	Open(context.Context, RunID) (Run, error)
}

type Run interface {
	Header(context.Context) (Header, error)
	EnsureNode(context.Context, NodeSpec) (Node, error)
	Node(context.Context, Slot) (Node, error)
	Append(context.Context, []Fact, Durability) ([]Record, error)
	Flush(context.Context) error
	// Read returns committed records after cursor, bounded by limit.
	Read(context.Context, uint64, int) ([]Record, error)
	PutBlob(context.Context, io.Reader) (BlobRef, error)
	OpenBlob(context.Context, BlobRef) (io.ReadCloser, error)
	Close() error
}
