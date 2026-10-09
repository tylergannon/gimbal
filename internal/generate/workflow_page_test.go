package generate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowInvocationPreservesAuthoredContinuations(t *testing.T) {
	d := &inspectionData{}
	g, info, err := extractInspected("../workflows/pyramidsummary", "PyramidSummary", "pyramid-summary", nil, true, d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := pageGuide(inspectGraph(g, "PyramidSummary", d), info, g.Roles(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--goal", "--semantic-index", "--largest-document", "--output-dir"} {
		if !strings.Contains(p.Guide.Invocation, flag) {
			t.Fatalf("authored invocation lost %s: %s", flag, p.Guide.Invocation)
		}
	}
	if !strings.Contains(p.Guide.Invocation, "\\\n") {
		t.Fatal("authored shell continuation was lost")
	}
}

func TestWorkflowInvocationQuotesStringPlaceholders(t *testing.T) {
	p, err := pageGuide(sourcePage{Name: "review"}, entryInfo{fields: []field{{name: "Goal", flag: "goal", kind: "string"}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Guide.Invocation, `--goal "<goal>"`) {
		t.Fatalf("placeholder would be shell redirection: %s", p.Guide.Invocation)
	}
}

func TestWorkflowPageSharesCommandMetadataAndChangesWithGuide(t *testing.T) {
	d := &inspectionData{}
	g, info, err := extractInspected("../workflows/validateproduct", "ValidateProduct", "validate-product", nil, true, d)
	if err != nil {
		t.Fatal(err)
	}
	page, err := pageGuide(inspectGraph(g, "ValidateProduct", d), info, g.Roles(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if page.Module != "" || page.Guide.Summary != info.summary || page.Guide.Prose != info.long {
		t.Fatal("shipped page must use command help without builder paths")
	}
	if len(page.Guide.Parameters) != 1 || page.Guide.Parameters[0].Flag != "suite-file" || !page.Guide.Parameters[0].Required || page.Guide.Parameters[0].Description != info.fields[0].doc {
		t.Fatal("page input differs from command parameter")
	}
	if len(page.Guide.Roles) != len(g.Roles()) || page.Guide.Roles[0].Default == "" || page.Guide.Snapshot == "" {
		t.Fatal("roles/defaults/snapshot missing")
	}
	original := page.Guide.Snapshot
	again, err := pageGuide(inspectGraph(g, "ValidateProduct", d), info, g.Roles(), nil)
	if err != nil || again.Guide.Snapshot != original {
		t.Fatal("snapshot is not deterministic")
	}
	info.long += "\nNew guide paragraph."
	changed, err := pageGuide(inspectGraph(g, "ValidateProduct", d), info, g.Roles(), nil)
	if err != nil || changed.Guide.Snapshot == original {
		t.Fatal("guide edit must change the accepted snapshot")
	}
}

func TestPageJSONContainsGuideAndProjectionTogether(t *testing.T) {
	file := filepath.Join(t.TempDir(), "review.page.json")
	if err := Source("../workflows/review", "Review", "review", file, ""); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var p sourcePage
	if err := json.Unmarshal(bytes, &p); err != nil {
		t.Fatal(err)
	}
	if p.Guide == nil || p.Guide.Summary == "" || len(p.Body) == 0 || p.Module != "" {
		t.Fatal("descriptor must contain usable guide and source projection")
	}
}
