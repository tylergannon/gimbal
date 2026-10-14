package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStockRegistrySeparatesRuntimeFromProjection(t *testing.T) {
	root := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module github.com/tylergannon/gimbal\n\ngo 1.27\n")
	write("registry.go", "package gimbal\ntype Registry struct { Calls []string; Fail bool }\n")
	write("internal/invocation/adapters.go", `//go:build !jsonschema

package invocation
import (
 "errors"
 "github.com/tylergannon/gimbal"
)
func record(r *gimbal.Registry, name string) error {
 r.Calls=append(r.Calls,name)
 if r.Fail && name=="review" {return errors.New("literal registration failure")}
 return nil
}
func registerServiceCheck(r *gimbal.Registry) error {return record(r,"service-check")}
func registerSprintPlan(r *gimbal.Registry) error {return record(r,"sprint-plan")}
func registerReview(r *gimbal.Registry) error {return record(r,"review")}
func registerImplement(r *gimbal.Registry) error {return record(r,"implement")}
func registerResearchDocument(r *gimbal.Registry) error {return record(r,"research-document")}
func registerResearchEval(r *gimbal.Registry) error {return record(r,"research-eval")}
func registerPyramidSummary(r *gimbal.Registry) error {return record(r,"pyramid-summary")}
func registerValidateProduct(r *gimbal.Registry) error {return record(r,"validate-product")}
`)
	write("internal/invocation/runtime_test.go", `//go:build !jsonschema

package invocation
import (
 "strings"
 "testing"
 "github.com/tylergannon/gimbal"
)
func TestRuntime(t *testing.T) {
 r:=new(gimbal.Registry)
 if err:=Register(r);err!=nil {t.Fatal(err)}
 if got:=strings.Join(r.Calls,",");got!="service-check,sprint-plan,review,implement,research-document,research-eval,pyramid-summary,validate-product" {t.Fatal(got)}
 r=&gimbal.Registry{Fail:true}
 err:=Register(r)
 if err==nil || err.Error()!="literal registration failure" || strings.Join(r.Calls,",")!="service-check,sprint-plan,review" {t.Fatalf("registration failure lost: %v %+v",err,r)}
}
`)
	write("internal/invocation/projection_test.go", `//go:build jsonschema

package invocation
import (
 "testing"
 "github.com/tylergannon/gimbal"
)
var _ func(*gimbal.Registry) error = Register
func TestProjection(t *testing.T) {
 defer func(){if got:=recover();got!="invocation.Register is unavailable during jsonschema analysis" {t.Fatalf("projection executed runtime registration: %v",got)}}()
 _=Register(new(gimbal.Registry))
}
`)
	if err := writeStockRegistry(root); err != nil {
		t.Fatal(err)
	}
	for _, tags := range []string{"", "jsonschema"} {
		t.Run("tags="+tags, func(t *testing.T) {
			cmd := exec.Command("go", "test", "-count=1", "-tags="+tags, "./internal/invocation")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("generated registry: %v\n%s", err, output)
			}
		})
	}
}
