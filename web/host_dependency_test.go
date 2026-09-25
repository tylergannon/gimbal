package web

import (
	"testing"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/workflows/implementation"
	"github.com/tylergannon/gimbal/internal/workflows/pyramidsummary"
	"github.com/tylergannon/gimbal/internal/workflows/researchdocument"
	"github.com/tylergannon/gimbal/internal/workflows/review"
	"github.com/tylergannon/gimbal/internal/workflows/validateproduct"
)

// This package assembles the generated routes. A workflow importing web would
// cycle when a concrete route and the host need that workflow's body.
func TestWorkflowBodiesCanJoinWebAssembly(t *testing.T) {
	bodies := []any{
		review.Review,
		implementation.Implement,
		researchdocument.ResearchDocument,
		pyramidsummary.PyramidSummary,
		validateproduct.ValidateProduct,
	}
	if len(bodies) != 5 {
		t.Fatal("missing workflow")
	}
	run := (*host.Project).Run
	_ = run
}
