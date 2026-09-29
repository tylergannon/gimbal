package claimaudit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jev "github.com/kazz187/jev-sdk-go"
)

func TestExtractionFindingShowsBlockAndExistingClaims(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	block := Block{ID: "b1", File: "INDEX.md", Line: 1, Text: "The bulletin is dated March 3. The notices conflict."}
	if err := os.WriteFile(filepath.Join(dir, block.File), []byte(block.Text+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	claims := []Claim{{ID: "c1", Text: "The bulletin is dated March 3.", Occurrences: []Occurrence{{BlockID: "b1", Text: "The bulletin is dated March 3."}}}}
	findings := map[string]Judgment{"extract:b1": {Task: "extract:b1", Label: "omitted_or_distorted"}}
	if err := mark(dir, Inventory{Blocks: []Block{block}}, claims, nil, nil, findings, nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, StateDir, "AUDIT.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Block INDEX.md:1:", "The notices conflict.", "Extracted: c1: The bulletin is dated March 3."} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("audit omits %q: %s", want, data)
		}
	}
}

func TestAuditMarksEachTableAndParagraphOccurrenceStably(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := "| Feature | Status |\n| --- | --- |\n| Alpha | Enabled. |\n| Beta | Disabled. |\n\nAlpha is enabled. Beta is disabled.\n"
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "original.md"), []byte("Alpha is enabled.\nBeta is disabled.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Blocks) != 2 {
		t.Fatalf("blocks=%d, want table and paragraph", len(inv.Blocks))
	}
	claims := []Claim{
		{ID: "c1", Text: "Alpha is enabled.", Kind: "fact", Occurrences: []Occurrence{{BlockID: inv.Blocks[0].ID, Text: "Enabled."}, {BlockID: inv.Blocks[1].ID, Text: "Alpha is enabled."}}, References: []Reference{{Path: "sources/original.md", StartLine: 1, EndLine: 1}}},
		{ID: "c2", Text: "Beta is disabled.", Kind: "fact", Occurrences: []Occurrence{{BlockID: inv.Blocks[0].ID, Text: "Disabled."}, {BlockID: inv.Blocks[1].ID, Text: "Beta is disabled."}}, References: []Reference{{Path: "sources/original.md", StartLine: 2, EndLine: 2}}},
	}
	if err := writeClaims(dir, claims); err != nil {
		t.Fatal(err)
	}
	client, err := jev.New(jev.WithProvider(jev.ProviderFunc(func(_ context.Context, req *jev.Request) (*jev.Response, error) {
		answers := map[string]jev.RawAnswer{}
		for name := range req.Questions {
			label := "supports"
			switch name {
			case "verdict":
				state := req.State.(map[string]any)
				if _, ok := state["block"]; ok {
					label = "complete"
				} else if _, ok := state["left_claim"]; ok {
					label = "contradiction"
				}
			case "c1/c2":
				label = "contradiction"
			default:
				label = "compatible"
			}
			answers[name] = jev.RawAnswer{Type: jev.KindChoice, Choice: &label, Probabilities: map[string]float64{label: 1}}
		}
		return &jev.Response{Model: model, Answers: answers}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	first, err := Audit(context.Background(), dir, client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.Metrics.PairFindings != 1 {
		t.Fatalf("pair findings=%d, want 1", first.Metrics.PairFindings)
	}
	marked, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"Enabled.", "Disabled.", "Alpha is enabled.", "Beta is disabled."} {
		if !strings.Contains(string(marked), phrase+inlineAuditStart) {
			t.Fatalf("missing adjacent marker for %q: %s", phrase, marked)
		}
	}
	if strings.Count(string(marked), "disputed by") != 4 || !strings.Contains(string(marked), "Enabled."+inlineAuditStart+" ") {
		t.Fatalf("not all endpoints marked at both occurrences: %s", marked)
	}
	if !strings.Contains(string(marked), "| Alpha | Enabled."+inlineAuditStart) || !strings.Contains(string(marked), inlineAuditEnd+" |") {
		t.Fatalf("table marker escaped its cell: %s", marked)
	}
	if markFree(string(marked)) != original {
		t.Fatalf("marker stripping changed factual text: %q", markFree(string(marked)))
	}
	prepared, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Extract) != 0 {
		t.Fatalf("markers triggered extraction: %v", prepared.Extract)
	}
	second, err := Audit(context.Background(), dir, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != first.Revision {
		t.Fatalf("revision changed after marker-only pass: %s to %s", first.Revision, second.Revision)
	}
	markedAgain, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(markedAgain) != string(marked) {
		t.Fatalf("repeat audit changed markers:\nfirst: %s\nagain: %s", marked, markedAgain)
	}
	blocks := map[string]Block{}
	for _, block := range prepared.Blocks {
		blocks[block.ID] = block
	}
	for i := range claims {
		claims[i].Disposition = "retain-unresolved"
		claims[i].DispositionDigest = dispositionDigest(claims[i], blocks, prepared.Sources)
	}
	if err := writeClaims(dir, claims); err != nil {
		t.Fatal(err)
	}
	accepted, err := Audit(context.Background(), dir, client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !accepted.AuthoringAllowed {
		t.Fatalf("dispositioned findings still block authoring: %+v", accepted)
	}
	if err := VerifyCurrent(dir); err != nil {
		t.Fatalf("fresh marked revision failed verification: %v", err)
	}
	markedAgain, err = os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(markFree(string(markedAgain))), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCurrent(dir); err == nil || !strings.Contains(err.Error(), "markers changed") {
		t.Fatalf("removed audit markers passed final gate: %v", err)
	}
}

func TestMarkKeepsSourceLinkOutsideAuditLink(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := "| Metric | Evidence |\n| --- | --- |\n| Limit | [The limit is 12 KiB.](sources/a.md) |\n\n[The limit is 12 KiB.](sources/a.md)\n"
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "a.md"), []byte("The limit is 12 KiB.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	claim := Claim{ID: "limit", Text: "The limit is 12 KiB.", Kind: "fact", Occurrences: []Occurrence{
		{BlockID: inv.Blocks[0].ID, Text: "The limit is 12 KiB."},
		{BlockID: inv.Blocks[1].ID, Text: "The limit is 12 KiB."},
	}}
	if err := mark(dir, inv, []Claim{claim}, map[string]bool{"limit": true}, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	marked, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(marked), "](sources/a.md)"+inlineAuditStart) != 2 {
		t.Fatalf("audit link broke original source links: %s", marked)
	}
	if markFree(string(marked)) != original {
		t.Fatalf("marker removal changed source links: %s", markFree(string(marked)))
	}
	prepared, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Extract) != 0 {
		t.Fatalf("markers changed extraction inventory: %v", prepared.Extract)
	}
}

func TestMarkMultilineTableOccurrence(t *testing.T) {
	dir := t.TempDir()
	original := "| Limit | Evidence |\n| --- | --- |\n| 12 KiB | [source](sources/a.md) |\n"
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	claim := Claim{ID: "table", Text: "The limit is 12 KiB.", Occurrences: []Occurrence{{BlockID: inv.Blocks[0].ID, Text: inv.Blocks[0].Text}}}
	if err := mark(dir, inv, []Claim{claim}, map[string]bool{"table": true}, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	marked, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(marked), inlineAuditStart) || !strings.Contains(string(marked), inlineAuditEnd+" |") {
		t.Fatalf("full-table occurrence was not marked inside the final cell: %s", marked)
	}
	if markFree(string(marked)) != original {
		t.Fatalf("full-table marker changed original text: %s", markFree(string(marked)))
	}
	prepared, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Extract) != 0 {
		t.Fatalf("full-table marker changed extraction inventory: %v", prepared.Extract)
	}
}
