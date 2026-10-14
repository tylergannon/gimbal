package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestInspectionBatchSharesLoadAndPreservesIndependentErrors(t *testing.T) {
	root := cacheFixture(t)
	second := filepath.Join(root, "second")
	if err := os.Mkdir(second, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "flow.go"), []byte(strings.ReplaceAll(cacheFixtureSource, `"first"`, `"second"`)), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	original := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = original })
	loadSourceTypes = func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		calls++
		return packages.Load(cfg, patterns...)
	}
	selected := []InspectionRequest{{Dir: root, Entry: "Flow", Name: "one"}, {Dir: second, Entry: "Flow", Name: "two"}}
	results := InspectBatch(root, selected, nil)
	if calls != 1 {
		t.Fatalf("batch typed loads=%d; want one", calls)
	}
	for i, literal := range []string{"first", "second"} {
		if results[i].Error != "" || !strings.Contains(string(results[i].Page), literal) {
			t.Fatalf("result %d: %+v", i, results[i])
		}
	}
	if err := os.WriteFile(filepath.Join(second, "flow.go"), []byte("package fixture\nfunc broken( {"), 0600); err != nil {
		t.Fatal(err)
	}
	results = InspectBatch(root, selected, nil)
	if results[0].Error != "" || !strings.Contains(string(results[0].Page), "first") {
		t.Fatalf("healthy workflow lost: %+v", results[0])
	}
	if !strings.Contains(results[1].Error, "expected") || len(results[1].Page) != 0 {
		t.Fatalf("broken workflow diagnostic: %+v", results[1])
	}
}
