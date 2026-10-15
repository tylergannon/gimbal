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
