package generate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalDefinitionGeneratedFromOrdinaryGo(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	mod = []byte(strings.Replace(string(mod), "module github.com/tylergannon/gimbal", "module example.com/authored", 1) + fmt.Sprintf("\nrequire github.com/tylergannon/gimbal v0.0.0\nreplace github.com/tylergannon/gimbal => %s\n", root))
	if err = os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0644); err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "go.sum"), sums, 0644); err != nil {
		t.Fatal(err)
	}
	source := `package authored
import("context";"github.com/tylergannon/gimbal";"github.com/tylergannon/polytype")
type Params struct {Goal string ` + "`json:\"goal_text\"`" + `; Flag polytype.Optional[bool];Count polytype.Optional[int]}
type Result struct {Summary string ` + "`json:\"summary\"`" + `}
var received Params
func Flow(ctx context.Context,env gimbal.Env,p Params)error {received=p;if false {_ = gimbal.Scope(ctx,"remote",func(ctx context.Context)error{_,err:=gimbal.FulfillService(ctx,"http",env.WorkDir,"serve",8080,"consumer");return err});s:=gimbal.NewSession(ctx,gimbal.RoleCodeReview,env.WorkDir);_,err:=s.Generate[Result](ctx,"review");return err};return nil}
`
	if err = os.WriteFile(filepath.Join(dir, "flow.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if err = Definition(dir, "Flow", "external", map[string]string{"code-review": "gpt-5.6-luna"}); err != nil {
		t.Fatal(err)
	}
	checks := `package authored
import("context";"testing";"github.com/tylergannon/gimbal")
func TestPreparedInput(t *testing.T){d:=FlowDefinition();r:=&gimbal.Registry{};if err:=r.RegisterWorkflow(d);err!=nil{t.Fatal(err)};for _,raw:=range []string{"{\"goal_text\":\"x\"}","{\"goal_text\":\"x\",\"flag\":false,\"count\":0}"}{fn,err:=d.Prepare([]byte(raw));if err!=nil{t.Fatal(err)};if err=fn(context.Background(),gimbal.Env{});err!=nil{t.Fatal(err)};if received.Goal!="x"{t.Fatal(received)}};if !received.Flag.Present || !received.Count.Present || received.Flag.Value || received.Count.Value!=0{t.Fatal(received)};if _,err:=d.Prepare([]byte("{\"goal_text\":\"x\",\"flag\":null}"));err==nil{t.Fatal("accepted null")};if len((Result{}).Schema())==0{t.Fatal("no typed result schema")}}
`
	if err = os.WriteFile(filepath.Join(dir, "flow_test.go"), []byte(checks), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated external package: %v\n%s", err, out)
	}
	// Changing the authored input/output needs only regenerating this module.
	source = strings.Replace(source, "Goal string", "Label string", 1)
	source = strings.Replace(source, "Summary string", "Answer string", 1)
	if err = os.WriteFile(filepath.Join(dir, "flow.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(dir, "flow_test.go")); err != nil {
		t.Fatal(err)
	}
	if err = Definition(dir, "Flow", "external", map[string]string{"code-review": "gpt-5.6-luna"}); err != nil {
		t.Fatalf("regenerate changed project: %v", err)
	}
	cmd = exec.Command("go", "build", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("rebuilt external package: %v\n%s", err, out)
	}
}
