package shell

import (
	"context"
	"strings"
	"testing"
)

func TestSanitizeBinaryOutput(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello\tworld\n", "hello\tworld\n"},
		{"a\rb", "a\rb"},
		{"a\x00b", "ab"},
		{"a\x1bb", "ab"},
		{"a\uFFF9b", "ab"},
		{"a\uFFFBb", "ab"},
	}
	for _, tc := range tests {
		if got := SanitizeBinaryOutput(tc.in); got != tc.want {
			t.Fatalf("SanitizeBinaryOutput(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripAnsi(t *testing.T) {
	if got := StripAnsi("\x1b[31mred\x1b[0m"); got != "red" {
		t.Fatalf("StripAnsi = %q, want %q", got, "red")
	}
	if got := StripAnsi("plain"); got != "plain" {
		t.Fatalf("StripAnsi = %q, want %q", got, "plain")
	}
}

func TestExecuteBashSanitization(t *testing.T) {
	result, err := ExecuteBashWithOperations(context.Background(),
		`printf '\033[31mred\033[0m\r\n'`, t.TempDir(), CreateLocalBashOperations(""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", result.ExitCode)
	}
	if result.Output != "red\n" {
		t.Fatalf("output = %q, want %q", result.Output, "red\n")
	}
}

func TestGetShellConfigCustomMissing(t *testing.T) {
	_, err := GetShellConfig("/custom/bash")
	if err == nil || err.Error() != "Custom shell path not found: /custom/bash" {
		t.Fatalf("err = %v, want the custom-shell-not-found error", err)
	}
}

func TestGetShellConfigDefault(t *testing.T) {
	config, err := GetShellConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if config.Shell == "" || len(config.Args) == 0 {
		t.Fatalf("config = %#v, want a resolved shell", config)
	}
	if !strings.Contains(config.Shell, "bash") && config.Shell != "sh" {
		t.Fatalf("config.Shell = %q, want bash or sh", config.Shell)
	}
}
