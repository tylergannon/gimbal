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
	for _, confirmed := range []bool{false, true} {
		name := "uncertain-review"
		if confirmed {
			name = "confirmed-contradiction"
		}
		t.Run(name, func(t *testing.T) { auditPairReview(t, confirmed) })
	}
}

func auditPairReview(t *testing.T, confirmed bool) {
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
					label = "uncertain"
					if confirmed {
						label = "contradiction"
					}
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
	if !first.Complete || first.AuthoringAllowed == confirmed || first.Metrics.PairsAnswered != 3 || first.Metrics.PairFindings != 1 {
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
	if first.Metrics.InputTokens == 0 || first.Metrics.RequestCount == 0 {
		t.Fatalf("fixture incurred no Jev usage: %+v", first.Metrics)
	}
	if second.Metrics.InputTokens != first.Metrics.InputTokens || second.Metrics.RequestCount != first.Metrics.RequestCount {
		t.Fatalf("cached completed audit lost incurred usage: first=%+v second=%+v", first.Metrics, second.Metrics)
	}
	begun, err := Begin(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if begun.Metrics.InputTokens != first.Metrics.InputTokens || begun.Metrics.RequestCount != first.Metrics.RequestCount {
		t.Fatalf("Begin lost completed pass usage: %+v", begun)
	}
	replayed, err := Audit(context.Background(), dir, client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if requests != before || replayed.Metrics.InputTokens != first.Metrics.InputTokens || replayed.Metrics.RequestCount != first.Metrics.RequestCount {
		t.Fatalf("completed Begin/Audit replay reset or double counted usage: requests=%d before=%d first=%+v replayed=%+v", requests, before, first.Metrics, replayed.Metrics)
	}
	next, err := Begin(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if next.Metrics.InputTokens != 0 || next.Metrics.RequestCount != 0 {
		t.Fatalf("new repair pass retained prior usage: %+v", next)
	}
	next, err = Audit(context.Background(), dir, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if requests != before || next.Metrics.InputTokens != 0 || next.Metrics.RequestCount != 0 {
		t.Fatalf("new cached repair pass incurred phantom usage: requests=%d before=%d completion=%+v", requests, before, next)
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
	if confirmed {
		if pair.ID != "" {
			t.Fatalf("confirmed contradiction offered editor review: %+v", pair)
		}
		left, _ := claimEvidence(dir, claims[0])
		right, _ := claimEvidence(dir, claims[1])
		pair = ReviewCandidate{Kind: "pair", ID: "c1/c2", Digest: pairReviewDigest(claims[0], claims[1], left, right)}
	} else if pair.ID != "c1/c2" {
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
	if !confirmed {
		index, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(strings.ReplaceAll(string(index), "Beta is unrelated.", "Beta is still unrelated.")), 0644); err != nil {
			t.Fatal(err)
		}
		updated, err := Prepare(dir)
		if err != nil {
			t.Fatal(err)
		}
		claims[2].Text = "Beta is still unrelated."
		claims[2].Occurrences = []Occurrence{{BlockID: updated.Blocks[2].ID, Text: claims[2].Text}}
		if err := writeClaims(dir, claims); err != nil {
			t.Fatal(err)
		}
	}
	reviewed, err := Audit(context.Background(), dir, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed {
		if reviewed.AuthoringAllowed || len(reviewed.Unresolved) != 1 || len(reviewed.RepairRequired) != 1 {
			t.Fatalf("editor erased confirmed contradiction: %+v", reviewed)
		}
		marked, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
		if err != nil || strings.Count(string(marked), "disputed by") != 2 {
			t.Fatalf("confirmed contradiction lost markers: %s %v", marked, err)
		}
		for i := range 2 {
			claims[i].Disposition = "retain-unresolved"
			claims[i].DispositionDigest = dispositionDigest(claims[i], nil, inv.Sources)
		}
		if err := writeClaims(dir, claims); err != nil {
			t.Fatal(err)
		}
		retained, err := Audit(context.Background(), dir, client, 2)
		if err != nil || !retained.AuthoringAllowed || len(retained.Unresolved) != 1 {
			t.Fatalf("retained contradiction blocked authoring or disappeared: %+v %v", retained, err)
		}
		marked, err = os.ReadFile(filepath.Join(dir, "INDEX.md"))
		if err != nil || strings.Count(string(marked), "disputed by") != 2 {
			t.Fatalf("retained contradiction lost markers: %s %v", marked, err)
		}
		return
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
	if len(stale.Unresolved) != 1 {
		t.Fatalf("stale review survived changed source: %+v", stale)
	}
}

func TestUnsupportedInferenceBlocksAuthoring(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("Consumers must deduplicate messages.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "delivery.md"), []byte("Consumers may receive duplicates.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	claim := Claim{ID: "inference", Text: "Consumers must deduplicate messages.", Kind: "inference", Occurrences: []Occurrence{{BlockID: inv.Blocks[0].ID, Text: inv.Blocks[0].Text}}, References: []Reference{{Path: "sources/delivery.md", StartLine: 1, EndLine: 1}}}
	if err := writeClaims(dir, []Claim{claim}); err != nil {
		t.Fatal(err)
	}
	client, err := jev.New(jev.WithProvider(jev.ProviderFunc(func(_ context.Context, req *jev.Request) (*jev.Response, error) {
		answers := map[string]jev.RawAnswer{}
		for name := range req.Questions {
			label := "not-established"
			if state, ok := req.State.(map[string]any); ok {
				if _, isBlock := state["block"]; isBlock {
					label = "complete"
				}
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
	if result.AuthoringAllowed || result.Metrics.SourceFindings != 1 {
		t.Fatalf("unsupported inference escaped source gate: %+v", result)
	}
	claim.Disposition = "exclude-from-factual-use"
	claim.DispositionDigest = dispositionDigest(claim, nil, inv.Sources)
	if err := writeClaims(dir, []Claim{claim}); err != nil {
		t.Fatal(err)
	}
	result, err = Audit(context.Background(), dir, client, 1)
	if err != nil || result.AuthoringAllowed {
		t.Fatalf("disposition excused unsupported inference: %+v %v", result, err)
	}
	claim.Kind = "recommendation"
	claim.DispositionDigest = dispositionDigest(claim, nil, inv.Sources)
	if err := writeClaims(dir, []Claim{claim}); err != nil {
		t.Fatal(err)
	}
	result, err = Audit(context.Background(), dir, client, 2)
	if err != nil || result.AuthoringAllowed {
		t.Fatalf("recommendation label excused unsupported assertion: %+v %v", result, err)
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

func TestUncitedClaimCannotBeDispositioned(t *testing.T) {
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
	if candidates[c.ID] != "" {
		t.Fatal("uncited claim offered a disposition")
	}
	c.DispositionDigest = dispositionDigest(c, nil, inv.Sources)
	if err := writeClaims(dir, []Claim{c}); err != nil {
		t.Fatal(err)
	}
	valid, err := Audit(context.Background(), dir, client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if valid.AuthoringAllowed || len(valid.RepairRequired) != 1 {
		t.Fatalf("uncited claim was excused by disposition: %+v", valid)
	}
}

func TestIndependentSourceReviewClearsFalseAlarm(t *testing.T) {
	for _, verdict := range []string{"uncertain", "source-contradicts", "aggregate-contradicts"} {
		t.Run(verdict, func(t *testing.T) { auditSourceReview(t, verdict) })
	}
}

func auditSourceReview(t *testing.T, verdict string) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"INDEX.md": "The limit is 12 KiB.\n", "sources/limit.md": "# Limit\nThe limit is 12 KiB.\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := Claim{ID: "limit", Text: "The limit is 12 KiB.", Kind: "fact", Occurrences: []Occurrence{{BlockID: inv.Blocks[0].ID, Text: "The limit is 12 KiB."}}, References: []Reference{{Path: "sources/limit.md", StartLine: 2, EndLine: 2, Quote: "The limit is 12 KiB."}}}
	if verdict == "aggregate-contradicts" {
		c.References = append(c.References, c.References[0])
	}
	if err := writeClaims(dir, []Claim{c}); err != nil {
		t.Fatal(err)
	}
	client, err := jev.New(jev.WithProvider(jev.ProviderFunc(func(_ context.Context, req *jev.Request) (*jev.Response, error) {
		label := "uncertain"
		state, _ := req.State.(map[string]any)
		if verdict == "source-contradicts" || (verdict == "aggregate-contradicts" && state["reference"] == nil) {
			label = "contradicts"
		}
		if verdict == "aggregate-contradicts" && state["reference"] != nil {
			label = "supports"
		}
		if state, ok := req.State.(map[string]any); ok && state["block"] != nil {
			label = "complete"
		}
		return &jev.Response{Model: model, Answers: map[string]jev.RawAnswer{"verdict": {Type: jev.KindChoice, Choice: &label, Probabilities: map[string]float64{label: 1}}}}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	first, err := Audit(context.Background(), dir, client, 0)
	if err != nil || first.AuthoringAllowed {
		t.Fatalf("false alarm passed without review: %+v %v", first, err)
	}
	var candidates []ReviewCandidate
	data, err := os.ReadFile(statePath(dir, "review-candidates.json"))
	if err != nil || json.Unmarshal(data, &candidates) != nil {
		t.Fatalf("candidates: %s %v", data, err)
	}
	if verdict != "uncertain" {
		if len(candidates) != 0 {
			t.Fatalf("contradicted source offered editor override: %+v", candidates)
		}
		// Even a correctly hashed, affirmative editorial override cannot clear it.
		review := ReviewDecision{Kind: "source", ID: c.ID, Digest: dispositionDigest(c, nil, inv.Sources), Decision: "reviewed-source-supported", Rationale: "Editor claims this is supported."}
		line, err := json.Marshal(review)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(statePath(dir, "reviews.jsonl"), append(line, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		c.Disposition = "retain-unresolved"
		c.DispositionDigest = dispositionDigest(c, nil, inv.Sources)
		if err := writeClaims(dir, []Claim{c}); err != nil {
			t.Fatal(err)
		}
		second, err := Audit(context.Background(), dir, client, 1)
		if err != nil || second.AuthoringAllowed || second.Metrics.SourceFindings != 1 || len(second.Unresolved) != 1 || len(second.RepairRequired) != 1 {
			t.Fatalf("editor/disposition cleared contradicted source: %+v %v", second, err)
		}
		marked, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
		if err != nil || !strings.Contains(string(marked), "source-unresolved") {
			t.Fatalf("source finding lost marker: %s %v", marked, err)
		}
		return
	}
	if len(candidates) != 1 || candidates[0].Kind != "source" {
		t.Fatalf("source review unavailable: %+v", candidates)
	}
	review := ReviewDecision{Kind: "source", ID: c.ID, Digest: candidates[0].Digest, Decision: "reviewed-source-supported", Rationale: "sources/limit.md line 2 states the full limit and unit."}
	line, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath(dir, "reviews.jsonl"), append(line, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := Audit(context.Background(), dir, client, 1)
	if err != nil || !second.AuthoringAllowed {
		t.Fatalf("supported claim not cleared by independent review: %+v %v", second, err)
	}
}

func TestDispositionSurvivesUnrelatedBlockRepair(t *testing.T) {
	c := Claim{ID: "one", Text: "The notice says 45 seconds.", Scope: "v2 notice", Kind: "fact", Occurrences: []Occurrence{{BlockID: "block", Text: "The notice says 45 seconds."}}, References: []Reference{{Path: "sources/notice.md", StartLine: 2, EndLine: 2, Quote: "45 seconds"}}}
	sources := map[string]string{"sources/notice.md": "A heading\nThe notice says 45 seconds.\n"}
	first := dispositionDigest(c, map[string]Block{"block": {Digest: "before"}}, sources)
	if first != dispositionDigest(c, map[string]Block{"block": {Digest: "after unrelated edit"}}, sources) {
		t.Fatal("unrelated block text invalidated a current disposition")
	}
	c.Text = "The default is 45 seconds."
	if first == dispositionDigest(c, nil, sources) {
		t.Fatal("changed claim retained disposition")
	}
	c.Text = "The notice says 45 seconds."
	sources["sources/notice.md"] = "A heading\nThe notice says 60 seconds.\n"
	if first == dispositionDigest(c, nil, sources) {
		t.Fatal("changed original retained disposition")
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

func TestPairReviewDigestTracksEvidenceAndAnchors(t *testing.T) {
	a := Claim{ID: "a", Text: "45 seconds", Occurrences: []Occurrence{{BlockID: "block", Text: "45 seconds"}}}
	b := Claim{ID: "b", Text: "60 seconds", Occurrences: []Occurrence{{BlockID: "other", Text: "60 seconds"}}}
	left, right := []string{"Notice: 45 seconds"}, []string{"Runbook: 60 seconds"}
	original := pairReviewDigest(a, b, left, right)
	a.Disposition, a.DispositionDigest = "retain-unresolved", "disposition"
	if original != pairReviewDigest(a, b, left, right) {
		t.Fatal("disposition invalidated review")
	}
	a.Occurrences[0].Text = "changed anchor"
	if original == pairReviewDigest(a, b, left, right) {
		t.Fatal("changed anchor retained review")
	}
	a.Occurrences[0].Text = "45 seconds"
	b.Text = "90 seconds"
	if original == pairReviewDigest(a, b, left, right) {
		t.Fatal("changed claim retained review")
	}
	b.Text = "60 seconds"
	right[0] = "Runbook: 90 seconds"
	if original == pairReviewDigest(a, b, left, right) {
		t.Fatal("changed original retained review")
	}
}
