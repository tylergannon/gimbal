package runview

import (
	"github.com/tylergannon/gimbal/internal/observation"
	"testing"
)

func TestSmallNonzeroCostNeverLooksFree(t *testing.T) {
	for _, cost := range []float64{.009, .0001, .000000001} {
		snapshot := observation.RunSnapshot{Totals: observation.Totals{Scopes: map[string]observation.Total{"": {ByModel: map[string]observation.Usage{"unknown-model": {StatedCost: cost}}}}}}
		if got := formatCost(snapshot); got != "<$0.01" {
			t.Errorf("cost %g renders %q", cost, got)
		}
	}
}

func TestEndedRunDoesNotAskForAnUnanswerableQuestion(t *testing.T) {
	snapshot := observation.RunSnapshot{Run: observation.RunRow{Status: observation.StatusCancelled}, Interviews: map[string]observation.InterviewRow{"question": {Status: observation.InterviewStatusPending, Question: "Which color?"}}}
	if got := pendingInterviews(snapshot); len(got) != 0 {
		t.Fatalf("ended run has waiting questions: %+v", got)
	}
	if got := summarizeRun(snapshot, pendingInterviews(snapshot)); got != "Cancelled" {
		t.Fatalf("ended summary = %q", got)
	}
}
