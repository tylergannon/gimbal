package skgo

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/skgo/generate"
)

// `go generate` and `skgo native build` both run `skgo generate`. The native
// build reads its Swift selection from native/skgo-native.json; the directive
// in config.go spells it out. When the two lists differ, each pass deletes the
// other's native checks from skgo_gen.go.
func TestNativeSelection(t *testing.T) {
	raw, err := os.ReadFile("../../native/skgo-native.json")
	if err != nil {
		t.Fatal(err)
	}
	var native struct {
		SwiftRemotes []string `json:"swiftRemotes"`
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	var directive []string
	for line := range strings.Lines(string(source)) {
		if strings.HasPrefix(line, "//go:generate go run ../generate/appgen generate ") {
			directive = strings.Fields(line)
		}
	}
	if directive == nil {
		t.Fatal("config.go has no appgen generate directive")
	}
	var remotes []string
	for i, field := range directive {
		if field == "--swift-remote" && i+1 < len(directive) {
			remotes = append(remotes, directive[i+1])
		}
	}
	if !slices.Equal(remotes, native.SwiftRemotes) {
		t.Fatalf("config.go selects %q for Swift; native/skgo-native.json selects %q", remotes, native.SwiftRemotes)
	}
	if !slices.Contains(directive, "--hook-package") {
		t.Fatal("config.go does not pass --hook-package, which skgo native build always passes")
	}
}

// The embedded generator keeps its selection readable by SKGO's native/dev
// entry points, which discover hook and locals configuration from this file.
func TestEmbeddedGeneratorDeclaration(t *testing.T) {
	cfg, err := generate.DeclaredConfig(generate.Config{Out: "."})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HookPackage != "github.com/tylergannon/gimbal/internal/serverhooks" || cfg.LocalsPackage != "github.com/tylergannon/gimbal/internal/app" {
		t.Fatalf("embedded generation lost native/dev selections: %#v", cfg)
	}
}
