package generate

import (
	"strings"
	"testing"
)

func TestGeneratedCLIUsesSharedJSONInvocation(t *testing.T) {
	graph, info, err := extract("testdata/fixture", "Fixture", "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	text := clientCommandSource("fixture", "Fixture", "fixture", info, graph)
	for _, want := range []string{"invocation.Request", "web.Start(cmd.Context(), instanceDir, request)", "--input", "Changed(\"lead\")", "pi/diffusion/model-id"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated CLI lacks %q", want)
		}
	}
	for _, forbidden := range []string{"FormClient", "internal/skgo", "routes.Start", "formInput"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("generated CLI retains %q", forbidden)
		}
	}
}
