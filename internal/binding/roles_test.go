package binding

import (
	"strings"
	"testing"
)

func TestRolesShareABindingAndNameTheMissingFlag(t *testing.T) {
	models, err := Roles("gpt-5.6-luna", map[string]string{"coder": "", "planner": "", "reviewer": "claude-haiku-4-5-20251001"})
	if err != nil {
		t.Fatal(err)
	}
	if models["coder"].Adapter != models["planner"].Adapter {
		t.Error("two roles on the fallback model got two harnesses")
	}
	if models["reviewer"].Adapter == models["coder"].Adapter {
		t.Error("a role on another model shares the fallback's harness")
	}
	if _, err := Roles("", map[string]string{"coder": "gpt-5.6-luna", "planner": ""}); err == nil || !strings.Contains(err.Error(), "give --planner or --model") {
		t.Errorf("err = %v, want the missing role named", err)
	}
}
