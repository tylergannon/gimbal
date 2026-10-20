package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/generate"
)

func TestGenerationAdmitsOnlyRecordedEffectsThroughHelpers(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, body, helper string
		reject             bool
	}{
		{"same-package-helper", `return local()};func local()error{return nil`, "", false},
		{"same-package-effect", `return local()};func local()error{_,err:=os.ReadFile("mutable");return err`, "", true},
		{"direct-file", `_,_=os.ReadFile("mutable");return nil`, "", true},
		{"helper-file", `return helper.Read()`, `package helper;import "os";func Read()error{_,err:=os.ReadFile("mutable");return err}`, true},
		{"unknown-opaque", `_ = os.Getenv("INPUT");return nil`, "", true},
		{"defer", `defer os.Getpid();return nil`, "", true},
		{"cancel", `return ctx.Err()`, "", true},
		{"recorded-file", `_,_,_,err:=gimbal.RunCommand(ctx,"read",env.WorkDir,"cat","input");return err`, "", false},
		{"pure-helper", `return helper.Read()`, `package helper;import "fmt";func Read()error{_ = fmt.Sprintf("%d",2);return nil}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("go.mod", "module example.com/admission\n\ngo 1.26.0\nrequire github.com/tylergannon/gimbal v0.0.0\nreplace github.com/tylergannon/gimbal => "+root+"\n")
			imports := `"context";"github.com/tylergannon/gimbal";"os"`
			if test.helper != "" {
				imports += `;"example.com/admission/helper"`
				write("helper/helper.go", test.helper)
			}
			// Keep imports used independently of the behavior under admission.
			write("flow.go", "package flow\nimport("+imports+")\nvar _=os.ErrNotExist\nfunc Flow(ctx context.Context,env gimbal.Env)error{"+test.body+"}\n")
			t.Setenv("GOFLAGS", "-mod=mod")
			_, err := generate.Extract(dir, "Flow", "flow")
			if test.reject {
				if err == nil || !strings.Contains(err.Error(), "GIMBAL111-LIBRARY-EFFECTS") {
					t.Fatalf("expected effect rejection, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
