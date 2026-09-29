package claimaudit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jev "github.com/kazz187/jev-sdk-go"
)

func TestAuditMarksBothEndsAndResumesAnswers(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "topic-001", "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"INDEX.md":                      "Alpha is enabled.\n\nAlpha is disabled.\n\nBeta is unrelated.\n",
		"topic-001/sources/original.md": "# Version A\nAlpha is enabled.\nAlpha is disabled.\nBeta is unrelated.\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Blocks) != 3 {
		t.Fatalf("blocks=%d", len(inv.Blocks))
	}
	claims := []Claim{}
	for i, b := range inv.Blocks {
		claims = append(claims, Claim{ID: []string{"c1", "c2", "c3"}[i], Text: b.Text, Kind: "fact", Occurrences: []Occurrence{{BlockID: b.ID, Text: b.Text}}, References: []Reference{{Path: "topic-001/sources/original.md", StartLine: i + 2, EndLine: i + 2}}})
	}
	if err := writeClaims(dir, claims); err != nil {
		t.Fatal(err)
	}
	requests := 0
	client, err := jev.New(jev.WithProvider(jev.ProviderFunc(func(_ context.Context, req *jev.Request) (*jev.Response, error) {
		requests++
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
		return &jev.Response{Model: model, Answers: answers, Usage: jev.Usage{InputTokens: 10}}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	first, err := Audit(context.Background(), dir, client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Complete || first.AuthoringAllowed || first.Metrics.PairsAnswered != 3 || first.Metrics.PairFindings != 1 {
		t.Fatalf("completion=%+v", first)
	}
	data, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "disputed by") != 2 {
		t.Fatalf("missing both markers: %s", data)
	}
	if strings.Contains(string(data), "c3 source-supported") {
		t.Fatal("unrelated supported claim adds noise to the index")
	}
	report, err := os.ReadFile(statePath(dir, "AUDIT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "pair:c1/c2: contradiction") || strings.Contains(string(report), "pair:c1/c3: compatible") {
		t.Fatalf("summary must retain findings without dumping successful pair checks: %s", report)
	}
	judgments, err := readJudgments(dir)
	if err != nil || len(judgments) < 6 {
		t.Fatalf("raw judgments lost: count=%d error=%v", len(judgments), err)
	}
	before := requests
	second, err := Audit(context.Background(), dir, client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if requests != before || second.Metrics.PairsAnswered != 3 {
		t.Fatalf("resume requests=%d before=%d summary=%+v", requests, before, second)
	}
	var candidates []ReviewCandidate
	candidateData, err := os.ReadFile(statePath(dir, "review-candidates.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(candidateData, &candidates); err != nil {
		t.Fatal(err)
	}
	var pair ReviewCandidate
	for _, candidate := range candidates {
		if candidate.Kind == "pair" {
			pair = candidate
		}
	}
	if pair.ID != "c1/c2" {
		t.Fatalf("pair candidate=%+v", pair)
	}
	review := ReviewDecision{Kind: "pair", ID: pair.ID, Digest: pair.Digest, Decision: "reviewed-compatible", Rationale: "Independent editor inspected both original spans and determined their scopes differ."}
	line, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath(dir, "reviews.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewed, err := Audit(context.Background(), dir, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reviewed.AuthoringAllowed {
		t.Fatalf("valid reviewed decision did not clear false pair: %+v", reviewed)
	}
	marked, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(marked), "disputed by") {
		t.Fatalf("reviewed pair remained marked: %s", marked)
	}
	sourcePath := filepath.Join(dir, "topic-001/sources/original.md")
	if err := os.WriteFile(sourcePath, []byte("# Version A\nAlpha is enabled.\nAlpha is disabled.\nBeta is unrelated.\nNew exception.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	stale, err := Audit(context.Background(), dir, client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if stale.AuthoringAllowed {
		t.Fatalf("stale review survived changed source: %+v", stale)
	}
}

func TestSnapshotChangeBlocksCompletion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("Fact one.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("Fact one.\nNew fact.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(dir, inv); err == nil {
		t.Fatal("changed index passed snapshot")
	}
}

func TestEveryCrossBatchPairIsAsked(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "topic-001", "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for i := 1; i <= 6; i++ {
		lines = append(lines, "Claim "+string(rune('0'+i))+" is recorded.")
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(strings.Join(lines, "\n\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "topic-001/sources/original.md"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	var claims []Claim
	for i, b := range inv.Blocks {
		claims = append(claims, Claim{ID: string(rune('A' + i)), Text: b.Text, Kind: "fact", Occurrences: []Occurrence{{BlockID: b.ID, Text: b.Text}}, References: []Reference{{Path: "topic-001/sources/original.md", StartLine: i + 1, EndLine: i + 1, Quote: b.Text}}})
	}
	if err := writeClaims(dir, claims); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var seenMu sync.Mutex
	var active, peak atomic.Int32
	client, err := jev.New(jev.WithProvider(jev.ProviderFunc(func(_ context.Context, req *jev.Request) (*jev.Response, error) {
		pairRequest := false
		for name := range req.Questions {
			if strings.Contains(name, "/") {
				pairRequest = true
				break
			}
		}
		if pairRequest {
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			time.Sleep(180 * time.Millisecond)
			defer active.Add(-1)
		}
		answers := map[string]jev.RawAnswer{}
		for name := range req.Questions {
			label := "supports"
			switch name {
			case "verdict":
				if _, ok := req.State.(map[string]any)["block"]; ok {
					label = "complete"
				}
			default:
				label = "compatible"
				seenMu.Lock()
				seen[name] = true
				seenMu.Unlock()
			}
			answers[name] = jev.RawAnswer{Type: jev.KindChoice, Choice: &label, Probabilities: map[string]float64{label: 1}}
		}
		return &jev.Response{Model: model, Answers: answers}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Audit(context.Background(), dir, client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AuthoringAllowed || result.Metrics.PairsAnswered != 15 || len(seen) != 15 {
		t.Fatalf("coverage=%+v pair IDs=%v", result, seen)
	}
	if peak.Load() < 2 || peak.Load() > 8 {
		t.Fatalf("pair request concurrency=%d, want 2..8", peak.Load())
	}
	if result.Metrics.WholeIndexPacketBytes == 0 || result.Metrics.WholeIndexO200kTokens == 0 || result.Metrics.WholeIndexJevTested {
		t.Fatalf("whole-index fit diagnostic=%+v", result.Metrics)
	}
	if !seen["A/F"] || !seen["E/F"] {
		t.Fatalf("missing boundary pairs: %v", seen)
	}
}

func TestTransientResumeKeepsCompletedExtraction(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "topic-001", "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("A supported fact.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "topic-001/sources/original.md"), []byte("A supported fact.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeClaims(dir, []Claim{{ID: "one", Text: "A supported fact.", Kind: "fact", Occurrences: []Occurrence{{BlockID: inv.Blocks[0].ID, Text: "A supported fact."}}, References: []Reference{{Path: "topic-001/sources/original.md", StartLine: 1, EndLine: 1}}}}); err != nil {
		t.Fatal(err)
	}
	extractions, sources := 0, 0
	client, err := jev.New(jev.WithProvider(jev.ProviderFunc(func(_ context.Context, req *jev.Request) (*jev.Response, error) {
		state := req.State.(map[string]any)
		label := "supports"
		if _, ok := state["block"]; ok {
			extractions++
			label = "complete"
		} else {
			sources++
			if sources == 1 {
				return nil, jev.ErrRateLimit
			}
		}
		return &jev.Response{Model: model, Answers: map[string]jev.RawAnswer{"verdict": {Type: jev.KindChoice, Choice: &label, Probabilities: map[string]float64{label: 1}}}}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	first, err := Audit(context.Background(), dir, client, 0)
	var transient *TransientError
	if !errors.As(err, &transient) || first.Complete {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := Audit(context.Background(), dir, client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Complete || !second.AuthoringAllowed || extractions != 1 || sources != 2 {
		t.Fatalf("resume=%+v extraction=%d source=%d", second, extractions, sources)
	}
}

func TestSourceDispositionRequiresCurrentDigest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("An uncited assertion.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := Claim{ID: "one", Text: "An uncited assertion.", Kind: "fact", Occurrences: []Occurrence{{BlockID: inv.Blocks[0].ID, Text: "An uncited assertion."}}}
	if err := writeClaims(dir, []Claim{c}); err != nil {
		t.Fatal(err)
	}
	client, err := jev.New(jev.WithProvider(jev.ProviderFunc(func(_ context.Context, _ *jev.Request) (*jev.Response, error) {
		label := "complete"
		return &jev.Response{Model: model, Answers: map[string]jev.RawAnswer{"verdict": {Type: jev.KindChoice, Choice: &label, Probabilities: map[string]float64{label: 1}}}}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	first, err := Audit(context.Background(), dir, client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.AuthoringAllowed || len(first.RepairRequired) != 1 {
		t.Fatalf("uncited claim passed: %+v", first)
	}
	c.Disposition = "exclude-from-factual-use"
	c.DispositionDigest = "stale"
	if err := writeClaims(dir, []Claim{c}); err != nil {
		t.Fatal(err)
	}
	stale, err := Audit(context.Background(), dir, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stale.AuthoringAllowed {
		t.Fatalf("stale disposition passed: %+v", stale)
	}
	var candidates map[string]string
	data, err := os.ReadFile(statePath(dir, "disposition-candidates.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &candidates); err != nil {
		t.Fatal(err)
	}
	c.DispositionDigest = candidates[c.ID]
	if err := writeClaims(dir, []Claim{c}); err != nil {
		t.Fatal(err)
	}
	valid, err := Audit(context.Background(), dir, client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !valid.AuthoringAllowed || len(valid.Unresolved) != 1 {
		t.Fatalf("valid exclusion rejected: %+v", valid)
	}
}

func TestSharedOriginalsAreSourcesNotIndexBlocks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources", "manual"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "manual", "INDEX.md"), []byte("Original fact.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("[Original](sources/manual/INDEX.md)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Sources) != 1 || len(inv.Files) != 1 {
		t.Fatalf("sources=%d indexfiles=%d", len(inv.Sources), len(inv.Files))
	}
	if _, err := readReference(dir, Reference{Path: "sources/manual/INDEX.md", StartLine: 1, EndLine: 1, Quote: "Original fact."}); err != nil {
		t.Fatal(err)
	}
}
