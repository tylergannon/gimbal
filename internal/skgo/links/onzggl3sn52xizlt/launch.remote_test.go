package routes

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
)

func TestLaunchInputOptionalMeaningAndAddressableErrors(t *testing.T) {
	d := gimbal.WorkflowDescription{Fields: []gimbal.WorkflowField{{Name: "goal", Kind: "string"}, {Name: "enabled", Kind: "bool", Optional: true}, {Name: "count", Kind: "int", Optional: true}, {Name: "label", Kind: "string", Optional: true, Nullable: true}}, InputSchema: json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string"},"enabled":{"type":"boolean"},"count":{"type":"integer"},"label":{"type":["string","null"]}},"required":["goal"],"additionalProperties":false}`)}
	fields := []LaunchField{{Name: "goal", Value: "ship"}, {Name: "enabled", Value: "false", Include: true}, {Name: "count", Value: "0", Include: true}, {Name: "label", Value: "", Include: true}}
	raw, err := launchInput(d, fields)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"count":0,"enabled":false,"goal":"ship","label":""}` {
		t.Fatalf("explicit values=%s", raw)
	}
	fields[1].Include = false
	fields[2].Include = false
	fields[3].Value = "null"
	raw, err = launchInput(d, fields)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"goal":"ship","label":null}` {
		t.Fatalf("omission/null=%s", raw)
	}
	fields[1].Include = true
	fields[1].Value = "yes"
	if _, err = launchInput(d, fields); err == nil || !strings.Contains(err.Error(), "inputs[1].value") {
		t.Fatalf("address error=%v", err)
	}
	fields[1].Name = "unknown"
	if _, err = launchInput(d, fields); err == nil || !strings.Contains(err.Error(), "inputs[1].value") {
		t.Fatalf("unknown error=%v", err)
	}
	fields[1].Name = "goal"
	if _, err = launchInput(d, fields); err == nil || !strings.Contains(err.Error(), "inputs[1].value") {
		t.Fatalf("duplicate error=%v", err)
	}
}
