package main

import (
	"bytes"
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
		{name: "CLI long version", args: []string{"--version"}},
		{name: "CLI short version", args: []string{"-v"}},
		{name: "vet flags", args: []string{"-flags"}, want: []string{"-flags"}, ok: true},
		{name: "vet unit", args: []string{"-json", vetConfig}, want: []string{"-json", vetConfig}, ok: true},
		{name: "run prompt help with cfg argument", args: []string{"run-prompt", "-h", ordinaryConfig}},
		{name: "run prompt wins over real vet config", args: []string{"run-prompt", "-h", vetConfig}},
		{name: "upload artifact wins over real vet config", args: []string{"upload-artifact", "--provider", "r2", vetConfig}},
		{name: "server help with cfg socket", args: []string{"-h", "-uds", ordinaryConfig}},
		{name: "server wins over real vet config", args: []string{"-h", "-uds", vetConfig}},
		{name: "ordinary server", args: []string{"-no-web"}},
		{name: "run a workflow", args: []string{"run", "review", "--goal", "check the code"}},
		{name: "manage opencode", args: []string{"opencode", "start"}},
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

func TestCLIReportsBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "v1.2.3", "abc1234", "2026-10-12T17:00:00Z"
	t.Cleanup(func() { version, commit, date = oldVersion, oldCommit, oldDate })

	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runWithUploaders([]string{flag}, &stdout, &stderr, func(string) string { return "" }, artifactUploaderConstructors{})
			if err != nil {
				t.Fatalf("run version command: %v (stderr: %q)", err, stderr.String())
			}
			const want = "gimbal version v1.2.3 (commit abc1234, built 2026-10-12T17:00:00Z)\n"
			if got := stdout.String(); got != want {
				t.Fatalf("version output = %q, want %q", got, want)
			}
		})
	}
}
