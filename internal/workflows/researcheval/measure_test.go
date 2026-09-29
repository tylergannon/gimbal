package researcheval

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/observation"
)

func TestBundledSuiteEvidence(t *testing.T) {
	dir := t.TempDir()
	if err := materializeSuite(dir); err != nil {
		t.Fatal(err)
	}
	s, err := loadSuite(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Cases) < 3 {
		t.Fatalf("cases=%d", len(s.Cases))
	}
	for _, c := range s.Cases {
		if len(c.Facts) < 5 || len(c.Queries) < 3 {
			t.Fatalf("insufficient case %s", c.ID)
		}
	}
}

func TestEvidenceRecallIndependentOfWrongAnswer(t *testing.T) {
	q := question{Answerable: true, AnswerTerms: [][]string{{"12 KiB"}}, Evidence: []evidence{{Source: "s.md", Quote: "The limit is 12 KiB."}}}
	originals := map[string]string{"s.md": "The limit is 12 KiB.\n"}
	read := map[string]string{"topic/sources/s.md": originals["s.md"]}
	answer := ReaderStep{Answer: "The limit is 32 KiB.", Citations: []ReaderCitation{{Path: "topic/sources/s.md", Quote: "The limit is 12 KiB."}}}
	found, passed := exactAnswer(q, answer, read, originals)
	if found != 1 || passed {
		t.Fatalf("found=%d passed=%v", found, passed)
	}
	answer.Answer = "The limit is 12 KiB."
	if _, passed := exactAnswer(q, answer, read, originals); !passed {
		t.Fatal("correct grounded answer failed")
	}
	read["topic/sources/s.md"] = "An index paraphrase says: The limit is 12 KiB.\n"
	if _, passed := exactAnswer(q, answer, read, originals); passed {
		t.Fatal("summary accepted as original")
	}
}

func TestUnreadOrFabricatedCitationsFail(t *testing.T) {
	q := question{Answerable: true, AnswerTerms: [][]string{{"12"}}, Evidence: []evidence{{Source: "s", Quote: "12"}}}
	for _, answer := range []ReaderStep{{Answer: "12", Citations: []ReaderCitation{{Path: "s", Quote: "12"}}}, {Answer: "12", Citations: []ReaderCitation{{Path: "other", Quote: "12"}}}} {
		if _, ok := exactAnswer(q, answer, map[string]string{}, map[string]string{"s": "12"}); ok {
			t.Fatal("unread evidence passed")
		}
	}
	if _, ok := exactAnswer(question{}, ReaderStep{Abstain: true}, nil, nil); !ok {
		t.Fatal("unknown abstention failed")
	}
	if _, ok := exactAnswer(question{}, ReaderStep{Answer: "invented answer"}, nil, nil); ok {
		t.Fatal("unknown hallucination passed")
	}
}

func TestQualityCannotWinByOmittingFacts(t *testing.T) {
	c := researchCase{Facts: []fact{{ID: "a"}, {ID: "b"}}, Queries: []question{{ID: "q"}}}
	good := Quality{Facts: []FactGrade{{ID: "a", DocumentCovered: true, IndexCovered: true}, {ID: "b", DocumentCovered: true, IndexCovered: true}}, DocumentAssertions: 2, IndexAssertions: 2}
	queries := []queryScore{{ID: "q", Success: true}}
	if score, err := scoreQuality(c, good, queries); err != nil || !score.Passed {
		t.Fatalf("%+v %v", score, err)
	}
	missing := good
	missing.Facts = missing.Facts[:1]
	if _, err := scoreQuality(c, missing, queries); err == nil {
		t.Fatal("omission accepted")
	}
	bad := good
	bad.UnsupportedIndexClaims = []string{"made-up limit"}
	if s, err := scoreQuality(c, bad, queries); err != nil || s.Passed {
		t.Fatalf("unsupported claim won %+v %v", s, err)
	}
	if s, err := scoreQuality(c, good, nil); err == nil && s.Passed {
		t.Fatalf("unfinished retrieval won %+v %v", s, err)
	}
}

func TestSelectionIncludesFailuresAndUnknownCost(t *testing.T) {
	candidates := []candidate{{ID: "cheap"}, {ID: "reliable"}, {ID: "unknown"}}
	s := suite{Cases: []researchCase{{ID: "dev", Split: "development"}, {ID: "hold", Split: "holdout"}}}
	pass := score{Passed: true}
	trials := []trialResult{{Candidate: candidates[0], Case: "dev", Split: "development", Quality: pass, CostKnown: true, CostUSD: .1}, {Candidate: candidates[0], Case: "dev", Split: "development", Error: "failed"}, {Candidate: candidates[1], Case: "dev", Split: "development", Quality: pass, CostKnown: true, CostUSD: .5, AuditRepairPasses: 1}, {Candidate: candidates[2], Case: "dev", Split: "development", Quality: pass, CostKnown: false, CostUSD: 0}}
	trials = append(trials, trials[2])
	best, ok := bestCandidate(candidates, trials, s)
	if !ok || best.ID != "reliable" {
		t.Fatalf("selected %+v %v", best, ok)
	}
	trials[2].AuditRepairPasses = 2
	if _, ok := bestCandidate(candidates, trials, s); ok {
		t.Fatal("two-repair candidate eligible")
	}
}

func TestReaderContextWithholdsGoldAndOtherCases(t *testing.T) {
	data := gimbal.ScopeData{Values: []gimbal.ScopeValue{{Key: "question", Value: "visible question"}, {Key: "read passages", Value: "visible passage"}, {Key: "assessment gold", Value: "SECRET ANSWER"}, {Key: "candidate allowlist", Value: "SECRET MODEL"}}}
	var rendered bytes.Buffer
	if err := template.Must(template.New("reader").Parse(readerContext)).Execute(&rendered, data); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(rendered.Bytes(), []byte("SECRET")) || !bytes.Contains(rendered.Bytes(), []byte("visible question")) {
		t.Fatal(rendered.String())
	}
}

func TestAuditAccountingSumsPassesWithoutChargingCachedJudgments(t *testing.T) {
	corpus := t.TempDir()
	dir := filepath.Join(corpus, ".semantic-index", "history")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	first := auditSummary{Complete: true, CoverageComplete: true, AuthoringAllowed: true}
	first.Metrics.InputTokens = 100
	first.Metrics.PairFindings = 1
	second := auditSummary{Complete: true, CoverageComplete: true, AuthoringAllowed: true}
	second.Metrics.InputTokens = 20
	second.Metrics.RepairPass = 1
	gap := auditSummary{Complete: true, CoverageComplete: true, AuthoringAllowed: true}
	for name, a := range map[string]auditSummary{"initial-pass-00.json": first, "initial-pass-01.json": second, "gap-1-pass-00.json": gap} {
		if err := writeJSON(filepath.Join(dir, name), a); err != nil {
			t.Fatal(err)
		}
	}
	clean, one, repairs, tokens := auditHistory(corpus)
	if clean || !one || repairs != 1 || tokens != 120 {
		t.Fatalf("%v %v %d %d", clean, one, repairs, tokens)
	}
}

func TestToolUseAndMissingUsageAreNotFreeSuccess(t *testing.T) {
	if !containsTool(map[string]any{"parts": []any{map[string]any{"type": "tool", "name": "read"}}}) {
		t.Fatal("tool hidden")
	}
	if containsTool(map[string]any{"type": "text", "text": "tool"}) {
		t.Fatal("plain text called a tool")
	}
	if _, known := researchCost(observation.RunSnapshot{}); known {
		t.Fatal("absent usage priced as free")
	}
}

func TestCandidatePriceDoesNotMakeUnknownUsageFree(t *testing.T) {
	tokens := observation.Tokens{Input: 1_000_000, CacheRead: 1_000_000, Output: 500_000, Reasoning: 500_000}
	for model, want := range map[string]float64{"deepseek-4.1-flash": 1.506, "glm-5.3-flash": 0.68} {
		cost, known := candidatePrice(model, tokens)
		if !known || cost < want-0.000001 || cost > want+0.000001 {
			t.Fatalf("%s: cost=%v known=%v", model, cost, known)
		}
	}
	if _, known := candidatePrice("unknown", tokens); known {
		t.Fatal("unknown model priced")
	}
	tokens.CacheWrite = 1
	if _, known := candidatePrice("glm-5.3-flash", tokens); known {
		t.Fatal("unspecified cache-write price treated as free")
	}
}

func TestSelectionNeedsRepeatsAndReportsFailures(t *testing.T) {
	c := candidate{ID: "one"}
	s := suite{Cases: []researchCase{{ID: "a", Split: "development"}, {ID: "b", Split: "development"}}}
	a := trialResult{Candidate: c, Case: "a", Split: "development", Quality: score{Passed: true}, CostKnown: true, InitialAuditClean: true}
	b := a
	b.Case = "b"
	trials := []trialResult{a, b}
	if _, ok := bestCandidate([]candidate{c}, trials, s); ok {
		t.Fatal("single observations selected")
	}
	trials = append(trials, a, b)
	if _, ok := bestCandidate([]candidate{c}, trials, s); !ok {
		t.Fatal("repeated complete cases not eligible")
	}
	failed := a
	failed.Quality.Passed = false
	failed.InitialAuditClean = false
	failed.Error = "provider failure"
	trials = append(trials, failed)
	summary := summarizeCandidates([]candidate{c}, trials)[0]
	if summary.Trials != 5 || summary.Passed != 4 || summary.PassRate != 0.8 || summary.InitialAuditCleanRate != 0.8 {
		t.Fatalf("%+v", summary)
	}
	if _, ok := bestCandidate([]candidate{c}, trials, s); ok {
		t.Fatal("failed repeat ignored")
	}
}

func TestReaderAllowsAbsoluteLinksOnlyInsideCorpus(t *testing.T) {
	root := t.TempDir()
	if _, err := readerPath(root, filepath.Join(root, "topic", "INDEX.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := readerPath(root, filepath.Join(root, "..", "elsewhere")); err == nil {
		t.Fatal("outside absolute accepted")
	}
	if _, err := readerPath(root, "../elsewhere"); err == nil {
		t.Fatal("outside relative accepted")
	}
}

func TestAbsoluteAndRelativeCitationsShareEvidence(t *testing.T) {
	root := t.TempDir()
	answer := ReaderStep{Answer: "12", Citations: []ReaderCitation{{Path: filepath.Join(root, "sources", "s.md"), Quote: "12"}}}
	answer.Citations[0].Path += "#L1"
	normalizeCitations(root, &answer)
	q := question{Answerable: true, AnswerTerms: [][]string{{"12"}}, Evidence: []evidence{{Source: "s.md", Quote: "12"}}}
	if _, ok := exactAnswer(q, answer, map[string]string{"sources/s.md": "12"}, map[string]string{"s.md": "12"}); !ok {
		t.Fatal("absolute citation rejected")
	}
}
