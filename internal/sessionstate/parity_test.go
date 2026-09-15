package sessionstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const webFixtures = "../../web/src/lib/sessionstate/fixtures"

func loadFixture(t *testing.T, name string) *Obj {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		data, err = os.ReadFile(filepath.Join(webFixtures, name))
	}
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	value, err := DecodeValue(data)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	fixture := objOf(value)
	if fixture == nil {
		t.Fatalf("fixture %s is not an object", name)
	}
	return fixture
}

func seedState(fixture *Obj) *Obj {
	seed := objOf(fixture.Get("seed"))
	info, family := NewObj(), NewObj()
	for _, entry := range arrOf(seed.Get("info")) {
		row := objOf(entry)
		info.Set(str(row.Get("id")), row.Clone())
		if !truthy(row.Get("parentID")) {
			family.Set(str(row.Get("id")), []any{str(row.Get("id"))})
		}
	}
	state := NewObj()
	state.Set("info", info)
	state.Set("family", family)
	state.Set("active", NewObj())
	state.Set("message", cloneValue(coalesce(seed.Get("messages"), NewObj())))
	state.Set("pending", cloneValue(coalesce(seed.Get("pending"), NewObj())))
	state.Set("permission", NewObj())
	state.Set("form", NewObj())
	return state
}

func decodeState(t *testing.T, state *Obj) ProjectionState {
	t.Helper()
	encoded, err := state.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded ProjectionState
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func splitLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func canonical(t *testing.T, text string) any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
