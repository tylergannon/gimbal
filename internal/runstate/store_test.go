package runstate

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tylergannon/gimbal/runstore"
	"testing"
)

func TestRestartIngressSharesWriterAndRetainsInputIdentity(t *testing.T) {
	ctx := context.Background()
	project := t.TempDir()
	id := runstore.NewRunID()
	records, err := ForProject(project).Create(ctx, runstore.Header{RunID: id, Workflow: "test"})
	if err != nil {
		t.Fatal(err)
	}
	header, _ := records.Header(ctx)
	_, err = records.EnsureNode(ctx, runstore.NodeSpec{Slot: runstore.Slot{Parent: header.Root, Kind: runstore.Scope, Name: "child", Ordinal: 1}, DisplayKey: "child.1"})
	if err != nil {
		t.Fatal(err)
	}
	message := `{"id":"same-input","message":"hello"}`
	if err := SubmitInteractive(ctx, project, string(id), "child.1/dialogue", message); !errors.Is(err, runstore.ErrBusy) {
		t.Fatalf("busy writer acknowledged: %v", err)
	}
	_ = records.Close()
	if err := SubmitInteractive(ctx, project, string(id), "child.1/dialogue", message); err != nil {
		t.Fatal(err)
	}
	if err := SubmitInteractive(ctx, project, string(id), "child.1/dialogue", message); err != nil {
		t.Fatal(err)
	}
	if err := SubmitInteractive(ctx, project, string(id), "child.1/dialogue", `{"id":"same-input","message":"different"}`); !errors.Is(err, runstore.ErrConflict) {
		t.Fatalf("conflicting retry %v", err)
	}
	if err := SubmitInteractive(ctx, project, string(id), "child.1/dialogue", `{"id":"second","message":"next"}`); err != nil {
		t.Fatal(err)
	}
	records, err = ForProject(project).Open(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := records.Read(ctx, 0, 4096)
	if err != nil {
		t.Fatal(err)
	}
	var inputs []InteractionInput
	var sequence uint64
	for _, record := range batch {
		if record.Kind == "interaction-input" {
			var input InteractionInput
			_ = json.Unmarshal(record.Payload, &input)
			inputs = append(inputs, input)
			if record.Sequence <= sequence {
				t.Fatal("acceptance order ambiguous")
			}
			sequence = record.Sequence
		}
	}
	if len(inputs) != 2 || inputs[0].ID != "same-input" || inputs[1].ID != "second" || inputs[0].Address != "child.1/dialogue" {
		t.Fatalf("inbox %+v", inputs)
	}
	payload := json.RawMessage(`{"scope":"child.1","event":{"kind":"scope_ended"}}`)
	_, err = records.Append(ctx, []runstore.Fact{{Version: 1, ID: runstore.NewRecordID(), Node: header.Root, Kind: "lifecycle", Payload: payload}}, runstore.Committed)
	if err != nil {
		t.Fatal(err)
	}
	_ = records.Close()
	if err := SubmitInteractive(ctx, project, string(id), "child.1/dialogue", `{"id":"after-end","message":"late"}`); err == nil {
		t.Fatal("ended owner accepted new input")
	}
}
