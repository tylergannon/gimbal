package main

import (
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// Appgen reads application source at runtime. Compiling those same packages into
// the generator makes an observed edit rebuild the observer before it can run.
func TestGeneratorBuildInputsExcludeObservedApplication(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-json", ".")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	foundGenerator := false
	for {
		var pkg struct {
			ImportPath string
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(pkg.ImportPath, "github.com/tylergannon/gimbal/internal/workflows/") {
			t.Errorf("generator compiles authored workflow %s", pkg.ImportPath)
		}
		switch pkg.ImportPath {
		case "github.com/tylergannon/gimbal", "github.com/tylergannon/gimbal/internal/observation":
			t.Errorf("generator compiles observed application package %s", pkg.ImportPath)
		case "github.com/tylergannon/gimbal/internal/generate":
			foundGenerator = true
		}
	}
	if !foundGenerator {
		t.Fatal("dependency query did not inspect the actual generator")
	}
}
