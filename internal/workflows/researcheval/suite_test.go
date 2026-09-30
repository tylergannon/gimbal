package researcheval

import "testing"

func TestBundledSuiteFitsReaderBudget(t *testing.T) {
	dir := t.TempDir()
	if err := materializeSuite(dir); err != nil {
		t.Fatal(err)
	}
	s, err := loadSuite(dir)
	if err != nil {
		t.Fatal(err)
	}
	development, holdout := 0, 0
	for _, c := range s.Cases {
		switch c.Split {
		case "development":
			development++
		case "holdout":
			holdout++
		}
		sources, err := sourceTexts(dir, c)
		if err != nil {
			t.Fatal(err)
		}
		unknown := false
		for _, q := range c.Queries {
			if !q.Answerable {
				unknown = true
				continue
			}
			needed := map[string]bool{}
			bytes := 0
			for _, e := range q.Evidence {
				if !needed[e.Source] {
					needed[e.Source] = true
					bytes += len(sources[e.Source])
				}
			}
			// Gold must be reachable through a route without exceeding the
			// reader's six-read and 18 KB limits.
			if len(needed) > 3 || bytes > 12000 {
				t.Errorf("%s/%s needs %d source reads and %d bytes before index navigation", c.ID, q.ID, len(needed), bytes)
			}
		}
		if !unknown {
			t.Errorf("%s needs an unanswerable question", c.ID)
		}
	}
	if development < 1 || holdout < 1 {
		t.Fatalf("suite needs development and holdout: development=%d holdout=%d", development, holdout)
	}
}
