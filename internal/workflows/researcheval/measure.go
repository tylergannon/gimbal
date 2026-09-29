package researcheval

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed suite
var bundledSuite embed.FS

type evidence struct {
	Source string `json:"source"`
	Quote  string `json:"quote"`
}

type fact struct {
	ID        string     `json:"id"`
	Statement string     `json:"statement"`
	Evidence  []evidence `json:"evidence"`
}

type question struct {
	ID          string     `json:"id"`
	Question    string     `json:"question"`
	Answerable  bool       `json:"answerable"`
	AnswerTerms [][]string `json:"answer_terms"`
	Evidence    []evidence `json:"evidence"`
}

type researchCase struct {
	ID      string     `json:"id"`
	Split   string     `json:"split"`
	Goal    string     `json:"goal"`
	Sources string     `json:"sources"`
	Facts   []fact     `json:"facts"`
	Queries []question `json:"queries"`
}

type suite struct {
	Name  string         `json:"name"`
	Cases []researchCase `json:"cases"`
}

// FactGrade binds the assessor's semantic judgment to one fixed gold fact.
type FactGrade struct {
	ID              string `json:"id"`
	DocumentCovered bool   `json:"document_covered"`
	IndexCovered    bool   `json:"index_covered"`
	Reason          string `json:"reason"`
}

// Quality is an independent judgment, not the operational Jev verdict.
type Quality struct {
	Facts                     []FactGrade `json:"facts"`
	DocumentAssertions        int         `json:"document_assertions"`
	IndexAssertions           int         `json:"index_assertions"`
	UnsupportedDocumentClaims []string    `json:"unsupported_document_claims"`
	UnsupportedIndexClaims    []string    `json:"unsupported_index_claims"`
	HiddenDisagreements       []string    `json:"hidden_disagreements"`
	Evidence                  string      `json:"evidence"`
}

// ReaderStep requests explicit files or returns an answer using already read evidence.
type ReaderStep struct {
	Paths     []string         `json:"paths"`
	Answer    string           `json:"answer"`
	Abstain   bool             `json:"abstain"`
	Citations []ReaderCitation `json:"citations"`
}

type ReaderCitation struct {
	Path  string `json:"path"`
	Quote string `json:"quote"`
}

// AnswerGrade checks meaning independently of literal answer-term coverage.
type AnswerGrade struct {
	Correct  bool   `json:"correct"`
	Grounded bool   `json:"grounded"`
	Reason   string `json:"reason"`
}

type queryScore struct {
	ID               string `json:"id"`
	Success          bool   `json:"success"`
	ExpectedEvidence int    `json:"expected_evidence"`
	FoundEvidence    int    `json:"found_evidence"`
	Reads            int    `json:"reads"`
	Bytes            int    `json:"bytes"`
	Answer           string `json:"answer"`
	Error            string `json:"error,omitempty"`
}

type score struct {
	Facts               int          `json:"facts"`
	DocumentFacts       int          `json:"document_facts"`
	IndexFacts          int          `json:"index_facts"`
	DocumentAssertions  int          `json:"document_assertions"`
	IndexAssertions     int          `json:"index_assertions"`
	UnsupportedDocument int          `json:"unsupported_document"`
	UnsupportedIndex    int          `json:"unsupported_index"`
	HiddenDisagreements int          `json:"hidden_disagreements"`
	Queries             []queryScore `json:"queries"`
	Passed              bool         `json:"passed"`
}

func scoreQuality(c researchCase, quality Quality, queries []queryScore) (score, error) {
	s := score{Facts: len(c.Facts), DocumentAssertions: quality.DocumentAssertions,
		IndexAssertions: quality.IndexAssertions, UnsupportedDocument: len(quality.UnsupportedDocumentClaims),
		UnsupportedIndex: len(quality.UnsupportedIndexClaims), HiddenDisagreements: len(quality.HiddenDisagreements), Queries: queries}
	want := map[string]bool{}
	for _, f := range c.Facts {
		want[f.ID] = true
	}
	seen := map[string]bool{}
	for _, f := range quality.Facts {
		if !want[f.ID] || seen[f.ID] {
			return s, fmt.Errorf("invalid or duplicate fact grade %q", f.ID)
		}
		seen[f.ID] = true
		if f.DocumentCovered {
			s.DocumentFacts++
		}
		if f.IndexCovered {
			s.IndexFacts++
		}
	}
	if len(seen) != len(want) || quality.DocumentAssertions < 1 || quality.IndexAssertions < 1 {
		return s, fmt.Errorf("assessment omitted facts or assessed no assertions")
	}
	if s.UnsupportedDocument > s.DocumentAssertions || s.UnsupportedIndex > s.IndexAssertions {
		return s, fmt.Errorf("unsupported count exceeds assessed assertion count")
	}
	s.Passed = s.DocumentFacts == s.Facts && s.IndexFacts == s.Facts && s.UnsupportedDocument == 0 && s.UnsupportedIndex == 0 && s.HiddenDisagreements == 0 && len(queries) == len(c.Queries)
	queryIDs := map[string]bool{}
	for _, q := range queries {
		if queryIDs[q.ID] {
			return s, fmt.Errorf("duplicate query grade %q", q.ID)
		}
		queryIDs[q.ID] = true
		s.Passed = s.Passed && q.Success
	}
	for _, q := range c.Queries {
		if !queryIDs[q.ID] {
			return s, fmt.Errorf("missing query grade %q", q.ID)
		}
	}
	return s, nil
}

func exactAnswer(q question, answer ReaderStep, read map[string]string, originals map[string]string) (int, bool) {
	if !q.Answerable {
		return 0, answer.Abstain && len(answer.Citations) == 0
	}
	answerValid := !answer.Abstain && strings.TrimSpace(answer.Answer) != ""
	text := strings.ToLower(answer.Answer)
	for _, variants := range q.AnswerTerms {
		matched := false
		for _, term := range variants {
			matched = matched || strings.Contains(text, strings.ToLower(term))
		}
		if !matched {
			answerValid = false
		}
	}
	found := 0
	for _, want := range q.Evidence {
		matched := false
		for _, got := range answer.Citations {
			body, wasRead := read[got.Path]
			if !wasRead || got.Quote == "" || !strings.Contains(body, got.Quote) {
				continue
			}
			// Generated research may copy originals into several topic folders.
			// Match the actual original bytes, not a basename or an index summary.
			if body == originals[want.Source] && strings.Contains(got.Quote, want.Quote) {
				matched = true
			}
		}
		if matched {
			found++
		}
	}
	for _, got := range answer.Citations {
		body, ok := read[got.Path]
		if !ok || got.Quote == "" || !strings.Contains(body, got.Quote) {
			return found, false
		}
		original := false
		for _, source := range originals {
			original = original || source == body
		}
		if !original {
			return found, false
		}
	}
	return found, answerValid && found == len(q.Evidence)
}

func loadSuite(root string) (suite, error) {
	var s suite
	data, err := os.ReadFile(filepath.Join(root, "suite.json"))
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if len(s.Cases) < 2 {
		return s, fmt.Errorf("suite needs development and holdout cases")
	}
	ids, splits := map[string]bool{}, map[string]bool{}
	for _, c := range s.Cases {
		if c.ID == "" || ids[c.ID] || (c.Split != "development" && c.Split != "holdout") || len(c.Facts) == 0 || len(c.Queries) == 0 {
			return s, fmt.Errorf("invalid case %q", c.ID)
		}
		ids[c.ID], splits[c.Split] = true, true
		sources, err := sourceTexts(root, c)
		if err != nil {
			return s, err
		}
		factIDs, queryIDs := map[string]bool{}, map[string]bool{}
		var evidence []evidence
		for _, f := range c.Facts {
			if f.ID == "" || factIDs[f.ID] || len(f.Evidence) == 0 {
				return s, fmt.Errorf("invalid fact %q", f.ID)
			}
			factIDs[f.ID] = true
			evidence = append(evidence, f.Evidence...)
		}
		for _, q := range c.Queries {
			if q.ID == "" || queryIDs[q.ID] || (q.Answerable && (len(q.AnswerTerms) == 0 || len(q.Evidence) == 0)) {
				return s, fmt.Errorf("invalid query %q", q.ID)
			}
			queryIDs[q.ID] = true
			evidence = append(evidence, q.Evidence...)
		}
		for _, e := range evidence {
			if e.Quote == "" || !strings.Contains(sources[e.Source], e.Quote) {
				return s, fmt.Errorf("case %s: invalid gold evidence %s", c.ID, e.Source)
			}
		}
	}
	if !splits["development"] || !splits["holdout"] {
		return s, fmt.Errorf("suite needs both splits")
	}
	return s, nil
}

func sourceTexts(root string, c researchCase) (map[string]string, error) {
	dir, err := within(root, c.Sources)
	if err != nil {
		return nil, err
	}
	texts := map[string]string{}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source symlink %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		texts[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	return texts, err
}

func within(root, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute path forbidden: %s", name)
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %s", name)
	}
	return path, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
