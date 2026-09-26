package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestResolveConfigValueLiteralsTemplatesAndEscapes(t *testing.T) {
	t.Setenv("TEST_CONFIG_LEFT", "left")
	t.Setenv("TEST_CONFIG_RIGHT", "right")

	cases := map[string]string{
		"literal-key":                            "literal-key",
		"$TEST_CONFIG_LEFT":                      "left",
		"${TEST_CONFIG_LEFT}_$TEST_CONFIG_RIGHT": "left_right",
		"$$TEST_CONFIG_LEFT":                     "$TEST_CONFIG_LEFT",
		"$!literal-$TEST_CONFIG_RIGHT":           "!literal-right",
	}
	for input, want := range cases {
		got, ok := ResolveConfigValue(input, nil)
		if !ok || got != want {
			t.Errorf("ResolveConfigValue(%q) = %q, %v; want %q", input, got, ok, want)
		}
	}
}

func TestResolveConfigValueScopedEnvPrecedesProcessEnv(t *testing.T) {
	t.Setenv("TEST_CONFIG_SCOPED", "process")
	got, ok := ResolveConfigValue("$TEST_CONFIG_SCOPED", map[string]string{"TEST_CONFIG_SCOPED": "credential"})
	if !ok || got != "credential" {
		t.Fatalf("got %q, %v; want credential", got, ok)
	}
}

func TestResolveConfigValueCommandsTrimOutput(t *testing.T) {
	cases := map[string]string{
		"!echo '  spaced-key  '":           "spaced-key",
		"!printf 'line1\\nline2'":          "line1\nline2",
		"!echo 'hello world' | tr ' ' '-'": "hello-world",
	}
	for input, want := range cases {
		ClearConfigValueCache()
		got, ok := ResolveConfigValue(input, nil)
		if !ok || got != want {
			t.Errorf("ResolveConfigValue(%q) = %q, %v; want %q", input, got, ok, want)
		}
	}
}

func TestResolveConfigValueCommandFailures(t *testing.T) {
	ClearConfigValueCache()
	for _, command := range []string{"!exit 1", "!nonexistent-command-12345", "!printf ''"} {
		if got, ok := ResolveConfigValue(command, nil); ok {
			t.Errorf("ResolveConfigValue(%q) = %q, true; want no value", command, got)
		}
	}
}

func TestResolveConfigValueCachesCommands(t *testing.T) {
	ClearConfigValueCache()
	counterFile := filepath.Join(t.TempDir(), "counter")
	if err := os.WriteFile(counterFile, []byte("0"), 0o644); err != nil {
		t.Fatal(err)
	}
	success := commandThatIncrements(counterFile, false)
	for i := range 2 {
		if got, ok := ResolveConfigValue(success, nil); !ok || got != "value" {
			t.Fatalf("call %d: got %q, %v", i, got, ok)
		}
	}
	if got := readCounter(t, counterFile); got != 1 {
		t.Fatalf("counter = %d; want 1 (cached)", got)
	}
	ClearConfigValueCache()
	if got, ok := ResolveConfigValue(success, nil); !ok || got != "value" {
		t.Fatalf("after clear: got %q, %v", got, ok)
	}
	if got := readCounter(t, counterFile); got != 2 {
		t.Fatalf("counter = %d; want 2 (cache cleared)", got)
	}

	failure := commandThatIncrements(counterFile, true)
	for range 2 {
		if _, ok := ResolveConfigValue(failure, nil); ok {
			t.Fatal("failure command unexpectedly resolved")
		}
	}
	if got := readCounter(t, counterFile); got != 3 {
		t.Fatalf("counter = %d; want 3 (failure cached)", got)
	}
}

func TestResolveConfigValueDoesNotCacheEnv(t *testing.T) {
	t.Setenv("TEST_CONFIG_DYNAMIC", "first")
	if got, _ := ResolveConfigValue("$TEST_CONFIG_DYNAMIC", nil); got != "first" {
		t.Fatalf("got %q; want first", got)
	}
	t.Setenv("TEST_CONFIG_DYNAMIC", "second")
	if got, _ := ResolveConfigValue("$TEST_CONFIG_DYNAMIC", nil); got != "second" {
		t.Fatalf("got %q; want second", got)
	}
}

func TestResolveConfigValueUncachedRunsEveryCall(t *testing.T) {
	counterFile := filepath.Join(t.TempDir(), "uncached-counter")
	if err := os.WriteFile(counterFile, []byte("0"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := commandThatIncrements(counterFile, false)
	for i := range 2 {
		if got, ok := ResolveConfigValueUncached(command, nil); !ok || got != "value" {
			t.Fatalf("call %d: got %q, %v", i, got, ok)
		}
	}
	if got := readCounter(t, counterFile); got != 2 {
		t.Fatalf("counter = %d; want 2", got)
	}
}

func commandThatIncrements(counterFile string, fail bool) string {
	exit := ""
	if fail {
		exit = "; exit 1"
	}
	return "!sh -c 'count=$(cat \"" + counterFile + "\"); echo $((count + 1)) > \"" + counterFile + "\"; echo value" + exit + "'"
}

func readCounter(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("counter %q is not an integer", data)
	}
	return value
}
