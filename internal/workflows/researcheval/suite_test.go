package researcheval

import "testing"

func TestBundledSuiteNeedsSelectiveRetrieval(t *testing.T) {
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
		// Six reads include the root index. Even spending all six on sources
		// must not reveal the whole corpus, as the original five-file suite did.
		if len(sources) <= 6 {
			t.Errorf("%s has %d sources: the six-read budget permits corpus dumping", c.ID, len(sources))
		}
		factSources := map[string]bool{}
		for _, f := range c.Facts {
			for _, e := range f.Evidence {
				factSources[e.Source] = true
			}
		}
		if len(factSources) < 5 {
			t.Errorf("%s concentrates its gold facts in only %d sources", c.ID, len(factSources))
		}
		crossSource, unknown := false, false
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
			crossSource = crossSource || len(needed) > 1
			// Leave at least two reads and 6 KB for an index route, plus the
			// remaining fourth turn for the answer. Gold is reachable without
			// requiring an unrestricted scan, even on a multi-source question.
			if len(needed) > 3 || bytes > 12000 {
				t.Errorf("%s/%s needs %d source reads and %d bytes before index navigation", c.ID, q.ID, len(needed), bytes)
			}
		}
		if !crossSource || !unknown {
			t.Errorf("%s needs both a cross-source question and an unanswerable question", c.ID)
		}
	}
	if development < 2 || holdout < 1 {
		t.Fatalf("suite needs two development cases and a holdout: development=%d holdout=%d", development, holdout)
	}
}
