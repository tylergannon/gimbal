package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRouteAnalysis(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	vetConfig := filepath.Join(dir, "vet.cfg")
	if err := os.WriteFile(vetConfig, []byte(`{"ImportPath":"example.com/p","GoFiles":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ordinaryConfig := filepath.Join(dir, "settings.cfg")
	if err := os.WriteFile(ordinaryConfig, []byte("ordinary settings"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want []string
		ok   bool
	}{
		{name: "standalone", args: []string{"lint", "./..."}, want: []string{"./..."}, ok: true},
		{name: "vet version", args: []string{"-V=full"}, want: []string{"-V=full"}, ok: true},
		{name: "vet flags", args: []string{"-flags"}, want: []string{"-flags"}, ok: true},
		{name: "vet unit", args: []string{"-json", vetConfig}, want: []string{"-json", vetConfig}, ok: true},
		{name: "run prompt help with cfg argument", args: []string{"run-prompt", "-h", ordinaryConfig}},
		{name: "run prompt wins over real vet config", args: []string{"run-prompt", "-h", vetConfig}},
		{name: "guided work wins over vet config", args: []string{"work", "-plan", vetConfig}},
		{name: "lfg wins over vet config", args: []string{"lfg", "-file", vetConfig}},
		{name: "planning wins over vet config", args: []string{"plan", "-file", vetConfig}},
		{name: "index wins over vet config", args: []string{"index", "-from", vetConfig}},
		{name: "sprint wins over vet config", args: []string{"sprint", "-plan", vetConfig}},
		{name: "server help with cfg socket", args: []string{"-h", "-uds", ordinaryConfig}},
		{name: "server wins over real vet config", args: []string{"-h", "-uds", vetConfig}},
		{name: "ordinary server", args: []string{"-no-web"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := routeAnalysis(test.args)
			if ok != test.ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("routeAnalysis(%q) = (%q, %v), want (%q, %v)", test.args, got, ok, test.want, test.ok)
			}
		})
	}
}
