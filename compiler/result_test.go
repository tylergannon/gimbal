package compiler_test

import (
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/compiler"
	"golang.org/x/tools/go/packages"
)

func TestCheckSplitResponse(t *testing.T) {
	pkgs, err := packages.Load(&packages.Config{Dir: "../internal/experiments/instrumented/resulttypes", Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps}, ".")
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("load: %v", err)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		t.Fatal(pkg.Errors)
	}
	for _, tc := range []struct{ name, want string }{
		{"Result", ""},
		{"Numeric", "validator that guarantees Go decoder range"},
		{"CollisionResult", "union discriminator collides case-insensitively"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := pkg.Types.Scope().Lookup(tc.name)
			if obj == nil {
				t.Fatalf("missing %s", tc.name)
			}
			err := compiler.CheckSplitResponse(pkg, obj.Type(), obj.Pos())
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "plain.go:") {
				t.Fatalf("diagnostic: %v", err)
			}
		})
	}
}
