package invocation

import (
	"encoding/json"
	"testing"

	"github.com/tylergannon/polytype"
)

func TestJSONValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"goal":" "}`, `null`, `{"goal":17}`, `{"goal":"x","typo":true}`, `{"goal":"x"} {}`} {
		if _, _, err := Prepare(Request{Workflow: "review", Input: json.RawMessage(raw)}); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if _, _, err := Prepare(Request{Workflow: "review", Input: json.RawMessage(`{"goal":"review"}`)}); err != nil {
		t.Fatal(err)
	}
	var input struct {
		Flag  polytype.Optional[bool] `json:"flag,omitzero"`
		Count polytype.Optional[int]  `json:"count,omitzero"`
	}
	if err := json.Unmarshal([]byte(`{}`), &input); err != nil {
		t.Fatal(err)
	}
	if input.Flag.Present || input.Count.Present {
		t.Fatal("omitted fields became present")
	}
	if err := json.Unmarshal([]byte(`{"flag":false,"count":0}`), &input); err != nil {
		t.Fatal(err)
	}
	if !input.Flag.Present || !input.Count.Present || input.Flag.Value || input.Count.Value != 0 {
		t.Fatalf("explicit zero lost: %+v", input)
	}
}
