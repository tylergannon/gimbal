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
	if len(p.Diagnostics) != 1 || call.Detail.ContextKnown || len(call.Context) != 0 || !strings.Contains(call.Note, "not known") {
		t.Fatalf("unknown context was claimed known: %+v, gaps=%v", call, p.Diagnostics)
	}
}

func TestInspectionSingleCaseSwitches(t *testing.T) {
	d := &inspectionData{}
	g, _, err := extractInspected("testdata/inspection", "Switches", "switches", nil, false, d)
	if err != nil {
		t.Fatal(err)
	}
	p := inspectGraph(g, "Switches", d)
	if len(p.Body) != 4 || p.Body[1].BranchLabels[0] != `mode == "review"` || p.Body[2].BranchLabels[0] != `mode == "stop"` || p.Body[2].Control.Actions[0] != "return nil" {
		t.Fatalf("switches lost predicates or actions: %+v", p.Body)
	}
	if p.Body[0].Context == nil {
		t.Fatal("known empty context serialized as unknown")
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

func TestInspectionPreservesServiceControlFlow(t *testing.T) {
	file, _ := filepath.Abs("testdata/inspection/fixture.go")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	modified := string(source) + `
func ServiceFlow(ctx context.Context, start bool) error {
	if !start { return nil }
	if start { _ = gimbal.Service(ctx, "product", ".", "serve") }
	return nil
}
`
	overlay := map[string][]byte{file: []byte(modified)}
	d := &inspectionData{}
	g, _, err := extractInspected("testdata/inspection", "ServiceFlow", "services", overlay, false, d)
	if err != nil {
		t.Fatal(err)
	}
	p := inspectGraph(g, "ServiceFlow", d)
	if len(p.Body) != 2 {
		t.Fatalf("body=%+v", p.Body)
	}
	guard := p.Body[0]
	if guard.Control == nil || !strings.Contains(guard.Control.Code, "if !start") || len(guard.Control.Actions) != 1 || guard.Control.Actions[0] != "return nil" {
		t.Fatalf("guard lost exact source: %+v", guard.Control)
	}
	condition := p.Body[1]
	if condition.Kind != "Condition" || condition.BranchLabels[0] != "start" || len(condition.Children[0]) != 1 {
		t.Fatalf("service condition=%+v", condition)
	}
	service := condition.Children[0][0]
	if service.Kind != "Service" || service.Label != "product" || service.Detail.Expression != `gimbal.Service(ctx, "product", ".", "serve")` {
		t.Fatalf("service=%+v", service)
	}
	normal, _, err := extractInspected("testdata/inspection", "ServiceFlow", "services", overlay, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(normal.Services) != 1 || len(normal.Body) != 1 {
		t.Fatalf("normal graph changed: %+v", normal)
	}
}

func TestDiagramCommentOwnership(t *testing.T) {
	file, _ := filepath.Abs("testdata/inspection/fixture.go")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source = append(source, []byte(`
func Comments(ctx context.Context, flag bool) error {
	// Product brief
	// A nested description.
	// Another line.
	gimbal.Set(ctx, "brief", "value")
	// Detached prose

	gimbal.Set(ctx, "detached", true)
	gimbal.Set(ctx, "trailing", true) // Not the next title
	gimbal.Set(ctx, "after-trailing", true)
	// Start the reviewer
	s := gimbal.NewSession(ctx, "reviewer", ".")
	// Review the product
	// Read the supplied context.
	_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
	// Optional review
	// Only review when requested.
	if flag {
		gimbal.Set(ctx, "unannotated-child", true)
		// Child review
		_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
	}
	// Start the product
	// The ordinary error guard does not become a separate diagram node.
	if err := gimbal.Service(ctx, "product", ".", "serve"); err != nil { return err }
	// When the service starts
	if err := gimbal.Service(ctx, "second-product", ".", "serve"); err == nil {
		gimbal.Set(ctx, "service-started", true)
	}
	// Parallel review
	group := gimbal.Group(ctx, "parallel")
	// First reviewer
	// Works independently.
	group.Go("left", func(ctx context.Context) error {
		_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
		return nil
	})
	// Repeat review
	for i := 0; i < 2; i++ {
		_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
	}
	// Switch mode
	switch flag {
	case true:
		gimbal.Set(ctx, "switch-child", true)
	}
	return group.Wait()
}
`)...)
	d := &inspectionData{}
	g, _, err := extractInspected("testdata/inspection", "Comments", "comments", map[string][]byte{file: source}, false, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", g.Diagnostics)
	}
	p := inspectGraph(g, "Comments", d)
	byTitle := map[string]*viewNode{}
	var visit func([]*viewNode)
	visit = func(nodes []*viewNode) {
		for _, n := range nodes {
			if n.Title != "" {
				if byTitle[n.Title] != nil {
					t.Errorf("duplicate annotation %q", n.Title)
				}
				byTitle[n.Title] = n
			}
			for _, child := range n.Children {
				visit(child)
			}
		}
	}
	visit(p.Body)
	want := map[string]string{"Product brief": "Set", "Start the reviewer": "NewSession", "Review the product": "Generate", "Optional review": "Condition", "Child review": "Generate", "Start the product": "Service", "Parallel review": "Group", "When the service starts": "Condition", "First reviewer": "Go", "Repeat review": "Repeat", "Switch mode": "Condition"}
	if len(byTitle) != len(want) {
		t.Fatalf("titles=%v", byTitle)
	}
	for title, kind := range want {
		n := byTitle[title]
		if n == nil || n.Kind != kind {
			t.Errorf("%q owner=%+v, want %s", title, n, kind)
		}
	}
	if n := byTitle["Product brief"]; n.Description != "A nested description.\nAnother line." || n.Label != "brief" || n.Context[0].Key != "brief" {
		t.Fatalf("comment changed semantic data: %+v", n)
	}
	if n := byTitle["First reviewer"]; n.Children[0][0].Title != "" || n.Description != "Works independently." {
		t.Fatalf("branch ownership: %+v", n)
	}
}

func TestInspectionJSONBranchAnnotations(t *testing.T) {
	output := filepath.Join(t.TempDir(), "routing.json")
	if err := Source("testdata/inspection", "Routing", "routing", output, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var page sourcePage
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatal(err)
	}
	if page.Name != "routing" || page.Entry != "Routing" || len(page.Diagnostics) != 0 {
		t.Fatalf("unexpected inspection: %+v", page)
	}
	var conditions []*viewNode
	for _, node := range page.Body {
		if node.Kind == "Condition" {
			conditions = append(conditions, node)
		}
	}
	if len(conditions) != 2 {
		t.Fatalf("conditions=%d", len(conditions))
	}
	chain, choice := conditions[0], conditions[1]
	if chain.Control.Kind != "if" || len(chain.Branches) != 3 || len(chain.Children) != 3 {
		t.Fatalf("if chain=%+v", chain)
	}
	if chain.Branches[0].Label != "Command provided" || chain.Branches[0].Code != `start != ""` || strings.Contains(chain.Description, "When true:") {
		t.Fatalf("first branch metadata=%+v", chain.Branches[0])
	}
	if chain.Branches[1].Title != "Existing product supplied?" || chain.Branches[1].Label != "Product location provided" || chain.Branches[1].Code != `existing != ""` {
		t.Fatalf("else-if metadata=%+v", chain.Branches[1])
	}
	if !chain.Branches[2].Default || chain.Branches[2].Label != "" || chain.Branches[2].Title != "" {
		t.Fatalf("else invented caption=%+v", chain.Branches[2])
	}
	if choice.Control.Kind != "switch" || len(choice.Branches) != 3 || choice.Branches[0].Title != "Visual review" || choice.Branches[1].Title != "Functional review" || !choice.Branches[2].Default {
		t.Fatalf("switch metadata=%+v", choice.Branches)
	}
	if choice.Children[0][0].Title != "Review each screen" || choice.Children[1][0].Title != "Independent checks" {
		t.Fatal("case caption replaced its first operation title")
	}
	if len(chain.Children[0]) != 2 || chain.Children[0][1].Kind != "Repeat" || chain.Children[1][0].Kind != "Scope" || chain.Children[2][0].Kind != "Scope" {
		t.Fatalf("branch nesting not retained: %+v", chain.Children)
	}
	if chain.Children[0][1].Title != "Check the started product" || chain.Children[1][0].Title != "Inspect the existing product" || chain.Children[2][0].Title != "Review the setup instructions" {
		t.Fatal("container comments not retained")
	}
}
