package generate

import (
	"strings"
	"testing"
)

func TestGeneratedCLIUsesTypedFormClient(t *testing.T) {
	graph, info, err := extract("testdata/fixture", "Fixture", "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	text := clientCommandSource("fixture", "Fixture", "fixture", info, graph)
	for _, want := range []string{"client.Client{FormClient: formClient}", ".StartFixture(cmd.Context(), formInput)", "RoleLead:", "Changed(\"lead\")"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated CLI lacks %q", want)
		}
	}
	for _, forbidden := range []string{"WorkflowEntry", "WithWorkflows", "control/submit", "web.Submit"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("generated CLI retains %q", forbidden)
		}
	}
}
