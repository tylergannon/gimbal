package temporalgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResultContractDiagnostics(t *testing.T) {
	root := moduleCopy(t)
	dir := filepath.Join(root, "internal/experiments/instrumented/resulttypes")
	path := filepath.Join(dir, "plain.go")
	original := string(read(t, path))
	output := filepath.Join(dir, "..", "results_temporal_gen.go")
	for _, tc := range []struct{ name, typ, extra, want string }{
		{"discriminator_collision", "CollisionResult", "", "union discriminator collides case-insensitively"},
		{"promoted_schema", "Derived", "type Derived struct { Result; Receipt bool `json:\"receipt\"` }", "embedded Generate result fields are unsupported"},
		{"numeric", "Numeric", "", "validator that guarantees Go decoder range"},
		{"root_decode", "Result", `func (*Result) UnmarshalJSON([]byte) error { return nil }`, "unsupported implicit json method: UnmarshalJSON"},
		{"variant_decode", "Result", `func (*Success) UnmarshalJSON([]byte) error { return nil }`, "unsupported implicit json method: UnmarshalJSON"},
		{"variant_text", "Result", `func (*Rejected) UnmarshalText([]byte) error { return nil }`, "unsupported implicit json method: UnmarshalText"},
		{"arbitrary_validator", "Opaque", `type Opaque struct{ Value string };func (Opaque) Schema() json.RawMessage{return nil};func(Opaque)ValidateJSON([]byte)error{return nil}`, "requires Polytype-generated Schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := original[:strings.Index(original, "func Results(")]
			source = strings.Replace(source, `"fmt"`, `"encoding/json"`, 1)
			if tc.name != "arbitrary_validator" {
				source = strings.Replace(source, `"encoding/json"`, "", 1)
			}
			source += `func Results(ctx context.Context,env gimbal.Env)error{
 session:=gimbal.NewSession(ctx,"coder",env.WorkDir)
 _,err:=session.Generate[` + tc.typ + `](ctx,"Return a result.")
 return err
 }
 ` + tc.extra
			write(t, path, []byte(source))
			err := emitSource(dir, "Results", "results", output)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "plain.go:") {
				t.Fatalf("diagnostic=%v want %s", err, tc.want)
			}
			if !strings.Contains(string(read(t, output)), "temporalGenerationFailed") {
				t.Fatal("rejected result retained stale target")
			}
		})
	}
	write(t, path, []byte(original))
	if err := emitSource(dir, "Results", "results", output); err != nil {
		t.Fatal(err)
	}
	generated := string(read(t, output))
	if !strings.Contains(generated, "session.GenerateResponse[resulttypes.Result]") || !strings.Contains(generated, "gimbal.ConsumeResponse[resulttypes.Result]") || strings.Contains(generated, "operationResult[resulttypes.Result]") {
		t.Fatal("generated result did not cross as bytes")
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	if err := emitSource(dir, "Results", "results", output); err != nil {
		t.Fatal(err)
	}
	if string(read(t, output)) != generated {
		t.Fatal("deletion/regeneration changed the result target")
	}
	command(t, root, "", "test", "./internal/experiments/instrumented", "-run", "^TestPolytypeResponseCorrespondence$", "-count=1")
}
