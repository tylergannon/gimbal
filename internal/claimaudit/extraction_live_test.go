package claimaudit

import (
	"context"
	"os"
	"testing"
	"time"

	jev "github.com/kazz187/jev-sdk-go"
)

// These labeled cases check the distinction that live indexes exposed: a
// navigation description is not a source-data assertion, but a factual premise
// inside a route still needs coverage. This small regression does not establish
// exhaustive judge accuracy. Run with GIMBAL_LIVE=1 and TYPESAFE_API_KEY set.
func TestLiveExtractionSemantics(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 and TYPESAFE_API_KEY for live Jev")
	}
	client, err := jev.New(jev.WithModel("jev-1.13.0"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, block, claim, want string }{
		{"layout", "The index directs authors through clear, high-level choices to the appropriate topic index, annotated leaf, or exact source passage in the underlying evidence cache. Detailed evidence and citations are maintained within those destination files.", "", "complete"},
		{"directory", "The repository contains five topical research packages. Each directory contains a topic INDEX.md; originals are in sources/.", "", "complete"},
		{"question", "For body-limit questions, see [capacity](topic-001/INDEX.md).", "", "complete"},
		{"covered", "The v2 request body limit is 12 KiB.", "The v2 request body limit is 12 KiB.", "complete"},
		{"routing-premise", "For the v2 request body limit of 12 KiB, see [capacity](topic-001/INDEX.md).", "", "omitted_or_distorted"},
		{"missing-qualifier", "The v2 request body limit is 12 KiB only when compression is enabled.", "The v2 request body limit is 12 KiB.", "omitted_or_distorted"},
		{"negation", "The service does not deduplicate messages.", "The service deduplicates messages.", "omitted_or_distorted"},
		{"partial-list", "The v2 request body limit is 12 KiB. Delivery is at least once.", "The v2 request body limit is 12 KiB.", "omitted_or_distorted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var claims []Claim
			if c.claim != "" {
				claims = []Claim{{ID: "c1", Text: c.claim, Kind: "fact"}}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			batch := client.Batch(map[string]any{"block": Block{ID: "b1", Text: c.block}, "claims": claims})
			handle := batch.Add("verdict", jev.OneOf[string](extractionQuestion, "complete", "omitted_or_distorted", "uncertain"))
			if _, err := batch.Run(ctx); err != nil {
				t.Fatal(err)
			}
			answer, err := handle.Get()
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("want=%s label=%s confidence=%.3f distribution=%v", c.want, answer.Value, answer.Confidence, answer.Probs)
			if answer.Value != c.want {
				t.Errorf("label %s, want %s", answer.Value, c.want)
			}
		})
	}
}
