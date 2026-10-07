package generate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/workflow"
)

func inspectionFixture(t *testing.T, overlay map[string][]byte) sourcePage {
	t.Helper()
	d := &inspectionData{}
	g, _, err := extractInspected("testdata/inspection", "Inspect", "context-fixture", overlay, false, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Diagnostics) != 0 {
		t.Fatalf("unexpected analysis gaps: %v", g.Diagnostics)
	}
	return inspectGraph(g, "Inspect", d)
}
func callsIn(nodes []*viewNode) []*viewNode {
	var found []*viewNode
	for _, n := range nodes {
		if n.Kind == "Generate" {
			found = append(found, n)
		}
		for _, body := range n.Children {
			found = append(found, callsIn(body)...)
		}
	}
	return found
}
func contextKey(t *testing.T, n *viewNode, key string) contextField {
	t.Helper()
	i := slices.IndexFunc(n.Context, func(f contextField) bool { return f.Key == key })
	if i < 0 {
		t.Fatalf("%s lacks %q: %+v", n.Label, key, n.Context)
	}
	return n.Context[i]
}
func lacks(t *testing.T, n *viewNode, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if slices.ContainsFunc(n.Context, func(f contextField) bool { return f.Key == key }) {
			t.Errorf("%s in %s leaks key %s", n.Label, n.Scope, key)
		}
	}
}
func TestInspectionContextAtEachSourceCall(t *testing.T) {
	p := inspectionFixture(t, nil)
	calls := callsIn(p.Body)
	if len(calls) != 5 {
		t.Fatalf("calls=%d, want all five including computed prompts", len(calls))
	}
	first, left, right, after, dynamic := calls[0], calls[1], calls[2], calls[3], calls[4]
	if first.Prompt != "Read the product context.\nDescribe its nested shape." || !first.Detail.PromptKnown {
		t.Fatalf("constant prompt=%+v", first)
	}
	lacks(t, first, "later", "optional", "sibling")
	if left.Detail.PromptKnown || left.Detail.PromptExpression != "prompt" {
		t.Fatalf("computed prompt lost: %+v", left.Detail)
	}
	shared := contextKey(t, left, "shared")
	if shared.Shape.Type != "int" || shared.Scope != "root/parallel/left" || len(shared.Shadowed) != 1 {
		t.Fatalf("child shadow=%+v", shared)
	}
	optional := contextKey(t, left, "optional")
	if !optional.Optional || optional.When != "conditional" {
		t.Fatalf("conditional write=%+v", optional)
	}
	lacks(t, left, "sibling")
	if contextKey(t, right, "shared").Shape.Type != "string" {
		t.Fatal("left shadow leaks into sibling")
	}
	lacks(t, right, "optional")
	lacks(t, after, "optional", "sibling")
	if contextKey(t, after, "shared").Scope != "root" {
		t.Fatal("group join merged child shadow")
	}
	contextKey(t, after, "later")
	if dynamic.Detail.PromptExpression != `prompt + "!"` {
		t.Errorf("prompt expr=%q", dynamic.Detail.PromptExpression)
	}
	if !contextKey(t, dynamic, "key").Dynamic {
		t.Fatal("dynamic key omitted or treated as concrete")
	}
	brief := contextKey(t, first, "brief")
	if brief.Shape.Kind != "object" || len(brief.Shape.Fields) != 2 {
		t.Fatalf("nested fields=%+v", brief.Shape)
	}
	product, next := brief.Shape.Fields[0], brief.Shape.Fields[1]
	if product.Name != "product" || product.Shape.Fields[0].Name != "name" {
		t.Fatalf("JSON tags lost: %+v", product)
	}
	guides := product.Shape.Fields[1]
	if guides.Name != "guides" || !guides.Optional || guides.Shape.Element.Fields[0].Name != "path" {
		t.Fatalf("nested array shape=%+v", guides)
	}
	if next.Shape.Element.Kind != "reference" {
		t.Fatalf("recursive type should remain a reference: %+v", next)
	}
}

func TestInspectionRegeneratesFromChangedSource(t *testing.T) {
	file, err := filepath.Abs("testdata/inspection/fixture.go")
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(string(original), "Read the product context.", "Changed prompt.", 1)
	modified = strings.Replace(modified, `"later", true`, `"renamed", 99`, 1)
	modified = strings.Replace(modified, `json:"path"`, `json:"location"`, 1)
	p := inspectionFixture(t, map[string][]byte{file: []byte(modified)})
	calls := callsIn(p.Body)
	if !strings.HasPrefix(calls[0].Prompt, "Changed prompt.") {
		t.Fatal("prompt did not regenerate")
	}
	lacks(t, calls[0], "renamed")
	if contextKey(t, calls[3], "renamed").Shape.Type != "int" {
		t.Fatal("changed key/type did not regenerate")
	}
	if contextKey(t, calls[0], "brief").Shape.Fields[0].Shape.Fields[1].Shape.Element.Fields[0].Name != "location" {
		t.Fatal("nested field did not regenerate")
	}
}

func TestHTMLSourcePathAndSafeEmbeddedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "viewer.html")
	if err := Source("testdata/inspection", "Inspect", `</script><script>bad()</script>`, path, ""); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	start := strings.Index(html, `<script id="source-data" type="application/json">`)
	if start < 0 {
		t.Fatal("missing generated data")
	}
	start += len(`<script id="source-data" type="application/json">`)
	end := strings.Index(html[start:], "</script>")
	var p sourcePage
	if err := json.Unmarshal([]byte(html[start:start+end]), &p); err != nil {
		t.Fatal(err)
	}
	if p.Name != `</script><script>bad()</script>` {
		t.Fatal("name not preserved safely")
	}
	if len(callsIn(p.Body)) != 5 {
		t.Fatal("standalone source path lost calls")
	}
	if strings.Contains(html, `<script>bad()</script>`) {
		t.Fatal("source data breaks out of JSON script element")
	}
}

func TestBranchAlternativesAndExitingWrites(t *testing.T) {
	ctx := []contextField{{Key: "inherited", Write: "root", Scope: "root"}}
	ops := []workflow.Operation{
		workflow.Condition{Branches: []workflow.Branch{
			{Case: "bad", Exits: true, Body: []workflow.Operation{workflow.Set{Key: "exiting"}}},
			{Case: "flag", Body: []workflow.Operation{workflow.Set{Key: "choice"}}},
		}},
		workflow.AgentCall{Session: "reader"},
	}
	b := pageBuilder{details: &inspectionData{Calls: map[workflow.Source][]callInspection{}}, cursors: map[workflow.Source]int{}}
	nodes, _ := b.body(ops, ctx, "root", "")
	call := nodes[1]
	lacks(t, call, "exiting")
	if !contextKey(t, call, "choice").Optional {
		t.Fatal("conditional context claimed unconditional")
	}
}

func TestUnknownContextDoesNotAssertLexicalScope(t *testing.T) {
	file, _ := filepath.Abs("testdata/inspection/fixture.go")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(string(source), "ctx, initialPrompt)", "context.Background(), initialPrompt)", 1)
	d := &inspectionData{}
	g, _, err := extractInspected("testdata/inspection", "Inspect", "unknown-context", map[string][]byte{file: []byte(modified)}, false, d)
	if err != nil {
		t.Fatal(err)
	}
	p := inspectGraph(g, "Inspect", d)
	call := callsIn(p.Body)[0]
	if len(p.Diagnostics) != 1 || call.Detail.ContextKnown || len(call.Context) != 0 || !strings.Contains(call.Note, "unresolved") {
		t.Fatalf("unknown context was claimed known: %+v, gaps=%v", call, p.Diagnostics)
	}
}

func TestCustomSerializationRemainsGoShape(t *testing.T) {
	file, _ := filepath.Abs("testdata/inspection/fixture.go")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	modified := string(source) + "\nfunc (Context) MarshalJSON() ([]byte,error) {panic(\"must never execute during inspection\")}\n"
	p := inspectionFixture(t, map[string][]byte{file: []byte(modified)})
	shape := contextKey(t, callsIn(p.Body)[0], "brief").Shape
	if !shape.GoOnly || !strings.Contains(shape.Note, "MarshalJSON") || shape.Fields[0].Name != "Product" || len(shape.Fields) != 3 {
		t.Fatalf("custom serialized fields incorrectly asserted: %+v", shape)
	}
}
