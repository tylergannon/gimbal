package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/modelalias"
)

func TestResolveModelUsesCentralCatalogWithoutInstanceOrSecrets(t *testing.T) {
	for _, spec := range []string{"luna", "terra", "fable", "pro:low"} {
		args := []string{"resolve-model", spec}
		if needsSecrets(args) || !isOrdinaryCLI(args) {
			t.Fatalf("resolver must be a standalone, secret-free CLI: %v", args)
		}
		var out bytes.Buffer
		if err := run(args, &out, &bytes.Buffer{}, os.Getenv); err != nil {
			t.Fatal(err)
		}
		var got modelalias.ResolvedSelection
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		name, effort, explicit := strings.Cut(spec, ":")
		want, err := modelalias.Resolve(modelalias.Selection{Name: name, Effort: effort, EffortPresent: explicit})
		if err != nil || got != want {
			t.Fatalf("%s resolved %+v, want %+v (%v)", spec, got, want, err)
		}
	}
}
