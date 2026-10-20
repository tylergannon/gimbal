// Package runstate resolves a project's recording store on every application
// process, including detached runners and post-exit cleanup owners.
package runstate

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type storeKey struct{}

func ForProject(project string) runstore.Store {
	return directory.Store{Root: filepath.Join(project, "runs")}
}
func WithStore(ctx context.Context, store runstore.Store) context.Context {
	return context.WithValue(ctx, storeKey{}, store)
}
func Override(ctx context.Context) runstore.Store {
	s, _ := ctx.Value(storeKey{}).(runstore.Store)
	return s
}
func Resolve(ctx context.Context, project string) runstore.Store {
	if s := Override(ctx); s != nil {
		return s
	}
	return ForProject(project)
}

// Recognized detects projection format, including an initializing directory.
// It never opens the authoritative store or acquires a writer lock.
func Recognized(runDir string) bool {
	if _, e := os.Stat(filepath.Join(runDir, "recording-format")); e == nil {
		return true
	}
	_, e := os.Stat(filepath.Join(runDir, "state"))
	return e == nil
}

// InteractionInput is the journal's durable wire representation. Sequence in
// runstore, rather than a timestamp, orders admission and delivery updates.
type InteractionInput struct {
	Configure   bool   `json:"configure"`
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	Stop        bool   `json:"stop"`
	ID          string `json:"id"`
	Address     string `json:"address"`
	Message     string `json:"message"`
	Finish      bool   `json:"finish"`
	Disposition string `json:"disposition"`
}

// SubmitInteractive admits input while execution is absent. It uses exactly
// the same exclusive writer as execution; ErrBusy is not an acknowledgement.
func SubmitInteractive(ctx context.Context, project, id, address, request string) error {
	var input InteractionInput
	if err := json.Unmarshal([]byte(request), &input); err != nil {
		return err
	}
	if input.ID == "" || address == "" || path.Clean(address) != address || strings.HasPrefix(address, "/") || strings.Contains(address, "..") {
		return fmt.Errorf("invalid interaction identity")
	}
	input.Address = address
	input.Disposition = "pending"
	records, err := ForProject(project).Open(ctx, runstore.RunID(id))
	if err != nil {
		return err
	}
	defer func() { _ = records.Close() }()
	header, err := records.Header(ctx)
	if err != nil {
		return err
	}
	scope := path.Dir(address)
	if scope == "." {
		scope = ""
	}
	known := scope == ""
	ended := false
	var cursor uint64
	for {
		batch, err := records.Read(ctx, cursor, 4096)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, record := range batch {
			cursor = record.Sequence
			switch record.Kind {
			case "node-created":
				var node runstore.Node
				if json.Unmarshal(record.Payload, &node) == nil && node.Kind == runstore.Scope && node.DisplayKey == scope {
					known = true
				}
			case "lifecycle":
				var lifecycle struct {
					Scope string `json:"scope"`
					Event struct {
						Kind string `json:"kind"`
					} `json:"event"`
				}
				if json.Unmarshal(record.Payload, &lifecycle) == nil && (lifecycle.Event.Kind == "run_ended" || (lifecycle.Event.Kind == "scope_ended" && lifecycle.Scope == scope)) {
					ended = true
				}
			case "interaction-input":
				var old InteractionInput
				if json.Unmarshal(record.Payload, &old) == nil && old.ID == input.ID {
					old.Disposition = "pending"
					if old != input {
						return runstore.ErrConflict
					}
					return nil
				}
			}
		}
	}
	if !known || ended {
		return fmt.Errorf("interaction owner is unavailable")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	_, err = records.Append(ctx, []runstore.Fact{{Version: 1, ID: runstore.NewRecordID(), Node: header.Root, Kind: "interaction-input", Payload: raw}}, runstore.Committed)
	return err
}
