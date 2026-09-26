package shell

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func resultText(result model.AgentToolResult) string {
	var b strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(model.TextContent); ok {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func runTool(t *testing.T, tool model.AgentTool, params map[string]any) (model.AgentToolResult, error) {
	t.Helper()
	return tool.Execute(context.Background(), "test-call", params, nil)
}

func TestBashExecutesSimpleCommands(t *testing.T) {
	result, err := runTool(t, CreateBashTool(t.TempDir(), nil), map[string]any{"command": "echo 'test output'"})
	if err != nil {
		t.Fatal(err)
	}
	if got := resultText(result); !strings.Contains(got, "test output") {
		t.Fatalf("output = %q, want it to contain %q", got, "test output")
	}
	if result.Details != nil {
		t.Fatalf("details = %#v, want nil", result.Details)
	}
}

func TestBashHandlesCommandErrors(t *testing.T) {
	_, err := runTool(t, CreateBashTool(t.TempDir(), nil), map[string]any{"command": "exit 1"})
	if err == nil || !strings.Contains(err.Error(), "code 1") {
		t.Fatalf("err = %v, want it to mention code 1", err)
	}
}

func TestBashMapsSignalKilledCommands(t *testing.T) {
	ops := CreateLocalBashOperations("")
	for _, tc := range []struct {
		signal   string
		exitCode int
	}{
		{"KILL", 137},
		{"TERM", 143},
	} {
		code, err := ops.Exec(context.Background(), "kill -"+tc.signal+" $$", t.TempDir(), BashExecOptions{OnData: func([]byte) {}})
		if err != nil {
			t.Fatalf("signal %s: %v", tc.signal, err)
		}
		if code == nil || *code != tc.exitCode {
			t.Fatalf("signal %s: exit code = %v, want %d", tc.signal, code, tc.exitCode)
		}
	}
}

func TestBashNullExitCodeThroughOperations(t *testing.T) {
	ops := BashOperations{Exec: func(_ context.Context, _ string, _ string, options BashExecOptions) (*int, error) {
		options.OnData([]byte("partial\n"))
		return nil, nil
	}}
	tool := CreateBashTool(t.TempDir(), &BashToolOptions{Operations: &ops})
	_, err := runTool(t, tool, map[string]any{"command": "remote"})
	if err == nil || !strings.Contains(err.Error(), "Command terminated without an exit code") {
		t.Fatalf("err = %v, want the null-exit-code error", err)
	}
}

func TestBashRespectsTimeout(t *testing.T) {
	_, err := runTool(t, CreateBashTool(t.TempDir(), nil), map[string]any{
		"command": "sleep 5",
		"timeout": 0.05,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timed out") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if !strings.Contains(err.Error(), "Command timed out after 0.05 seconds") {
		t.Fatalf("err = %v, want the timeout seconds", err)
	}
}

func TestBashTruncatedErrorKeepsFullOutputPath(t *testing.T) {
	for _, tc := range []struct {
		execErr string
		want    string
	}{
		{"timeout:5", "Command timed out after 5 seconds"},
		{"aborted", "Command aborted"},
	} {
		ops := BashOperations{Exec: func(_ context.Context, _ string, _ string, options BashExecOptions) (*int, error) {
			for i := 1; i <= 3000; i++ {
				options.OnData([]byte(strconv.Itoa(i) + "\n"))
			}
			if tc.execErr == "aborted" {
				return nil, ErrShellAborted
			}
			return nil, &ShellTimeoutError{Seconds: 5}
		}}
		tool := CreateBashTool(t.TempDir(), &BashToolOptions{Operations: &ops})
		_, err := runTool(t, tool, map[string]any{"command": "chatty-fail"})
		if err == nil {
			t.Fatalf("%s: expected an error", tc.execErr)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want %q", tc.execErr, err, tc.want)
		}
		if !strings.Contains(err.Error(), "Full output: ") {
			t.Fatalf("%s: err = %v, want a full output path", tc.execErr, err)
		}
		if strings.Contains(err.Error(), "Full output: undefined") {
			t.Fatalf("%s: err = %v, has an undefined path", tc.execErr, err)
		}
		path := fullOutputPathFromError(err.Error())
		if path == "" {
			t.Fatalf("%s: no path parsed from %q", tc.execErr, err.Error())
		}
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("%s: full output path %s: %v", tc.execErr, path, statErr)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(data), "2998\n2999\n3000") {
			t.Fatalf("%s: full output missing the tail", tc.execErr)
		}
	}
}

func fullOutputPathFromError(message string) string {
	const marker = "Full output: "
	_, after, ok := strings.Cut(message, marker)
	if !ok {
		return ""
	}
	rest := after
	if end := strings.IndexAny(rest, "]\n"); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(rest)
}

func TestBashMissingCwd(t *testing.T) {
	missing := t.TempDir() + "/does-not-exist"
	_, err := runTool(t, CreateBashTool(missing, nil), map[string]any{"command": "true"})
	want := "Working directory does not exist: " + missing + "\nCannot execute bash commands."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestBashCustomShellPath(t *testing.T) {
	// A custom local shell path that does not exist fails resolution.
	ops := CreateLocalBashOperations("/custom/bash")
	_, err := ops.Exec(context.Background(), "echo test", t.TempDir(), BashExecOptions{OnData: func([]byte) {}})
	if err == nil || err.Error() != "Custom shell path not found: /custom/bash" {
		t.Fatalf("err = %v, want the custom-shell-not-found error", err)
	}

	// A tool with explicit operations must not use the local resolver.
	used := false
	ops = BashOperations{Exec: func(context.Context, string, string, BashExecOptions) (*int, error) {
		used = true
		zero := 0
		return &zero, nil
	}}
	tool := CreateBashTool(t.TempDir(), &BashToolOptions{Operations: &ops, ShellPath: "/custom/bash"})
	if _, err := runTool(t, tool, map[string]any{"command": "echo test"}); err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("custom operations were not used")
	}
}

func TestBashStdinCommandTransport(t *testing.T) {
	tool := CreateShellTool(t.TempDir(), ShellToolConfig{
		Name:           "testshell",
		Label:          "testshell",
		ShellName:      "testshell",
		TempFilePrefix: "pi-testshell",
		ResolveShell: func() (ShellConfig, error) {
			return ShellConfig{Shell: "/bin/sh", Args: []string{"-s"}, CommandTransport: shellTransportStdin}, nil
		},
	}, nil)
	result, err := runTool(t, tool, map[string]any{"command": "echo from-stdin"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(resultText(result)); got != "from-stdin" {
		t.Fatalf("output = %q, want %q", got, "from-stdin")
	}
}

func TestLegacyWSLBashConfig(t *testing.T) {
	if !IsLegacyWSLBashPath(`C:\Windows\System32\bash.exe`) {
		t.Fatal("expected legacy WSL bash path to be recognized")
	}
	if IsLegacyWSLBashPath("/bin/bash") {
		t.Fatal("did not expect /bin/bash to be recognized as legacy WSL")
	}
	config := bashShellConfig(`C:\Windows\System32\bash.exe`)
	if config.CommandTransport != shellTransportStdin || config.Args[0] != "-s" {
		t.Fatalf("config = %#v, want stdin transport with -s", config)
	}
}

func TestBashCommandPrefix(t *testing.T) {
	tool := CreateBashTool(t.TempDir(), &BashToolOptions{CommandPrefix: "export TEST_VAR=hello"})
	result, err := runTool(t, tool, map[string]any{"command": "echo $TEST_VAR"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(resultText(result)); got != "hello" {
		t.Fatalf("output = %q, want %q", got, "hello")
	}

	tool = CreateBashTool(t.TempDir(), &BashToolOptions{CommandPrefix: "echo prefix-output"})
	result, err = runTool(t, tool, map[string]any{"command": "echo command-output"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(resultText(result)); got != "prefix-output\ncommand-output" {
		t.Fatalf("output = %q, want prefix and command lines", got)
	}
}

func TestBashCoalescesStreamingUpdates(t *testing.T) {
	ops := BashOperations{Exec: func(_ context.Context, _ string, _ string, options BashExecOptions) (*int, error) {
		for i := range 5000 {
			options.OnData([]byte("line " + strconv.Itoa(i) + "\n"))
		}
		zero := 0
		return &zero, nil
	}}
	tool := CreateBashTool(t.TempDir(), &BashToolOptions{Operations: &ops})
	updates := 0
	result, err := tool.Execute(context.Background(), "test-call", map[string]any{"command": "chatty"}, func(model.AgentToolResult) {
		updates++
	})
	if err != nil {
		t.Fatal(err)
	}
	if updates >= 25 {
		t.Fatalf("updates = %d, want fewer than 25", updates)
	}
	if got := resultText(result); !strings.Contains(got, "line 4999") {
		t.Fatalf("output does not contain the last line: %q", got)
	}
}

func TestBashTrailingNewlineLineCount(t *testing.T) {
	ops := BashOperations{Exec: func(_ context.Context, _ string, _ string, options BashExecOptions) (*int, error) {
		for i := 1; i <= 4000; i++ {
			options.OnData([]byte("line-" + pad4(i) + "\n"))
		}
		zero := 0
		return &zero, nil
	}}
	tool := CreateBashTool(t.TempDir(), &BashToolOptions{Operations: &ops})
	result, err := runTool(t, tool, map[string]any{"command": "many-lines"})
	if err != nil {
		t.Fatal(err)
	}
	output := resultText(result)
	details, ok := result.Details.(*BashToolDetails)
	if !ok || details.Truncation == nil {
		t.Fatalf("details = %#v, want truncation details", result.Details)
	}
	if details.Truncation.TotalLines != 4000 || details.Truncation.OutputLines != 2000 {
		t.Fatalf("truncation = %#v, want total 4000 and output 2000", details.Truncation)
	}
	if !strings.Contains(output, "line-2001") || !strings.Contains(output, "line-4000") {
		t.Fatalf("output = %q, want the last 2000 lines", output)
	}
	if !strings.Contains(output, "[Showing lines 2001-4000 of 4000. Full output: ") {
		t.Fatalf("output = %q, want the lines notice", output)
	}
	if strings.Contains(output, "4001") {
		t.Fatalf("output = %q, must not contain 4001", output)
	}
}

func TestBashSplitUTF8(t *testing.T) {
	euro := []byte("€\n")
	ops := BashOperations{Exec: func(_ context.Context, _ string, _ string, options BashExecOptions) (*int, error) {
		options.OnData(euro[:1])
		options.OnData(euro[1:])
		zero := 0
		return &zero, nil
	}}
	tool := CreateBashTool(t.TempDir(), &BashToolOptions{Operations: &ops})
	result, err := runTool(t, tool, map[string]any{"command": "split-utf8"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(resultText(result)); got != "€" {
		t.Fatalf("output = %q, want €", got)
	}
}

func TestBashLocalOperationsUsesEnv(t *testing.T) {
	ops := CreateLocalBashOperations("")
	code, err := ops.Exec(context.Background(), "echo $TEST_LOCAL_BASH_OPS", t.TempDir(), BashExecOptions{
		OnData: func([]byte) {},
		Env:    append(os.Environ(), "TEST_LOCAL_BASH_OPS=from-local-ops"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if code == nil || *code != 0 {
		t.Fatalf("exit code = %v, want 0", code)
	}
}

func TestBashPersistsFullOutputOnLineTruncation(t *testing.T) {
	result, err := runTool(t, CreateBashTool(t.TempDir(), nil), map[string]any{"command": "seq 3000"})
	if err != nil {
		t.Fatal(err)
	}
	details, ok := result.Details.(*BashToolDetails)
	if !ok || details.Truncation == nil {
		t.Fatalf("details = %#v, want truncation details", result.Details)
	}
	if !details.Truncation.Truncated || details.Truncation.TruncatedBy == nil || *details.Truncation.TruncatedBy != "lines" {
		t.Fatalf("truncation = %#v, want truncated by lines", details.Truncation)
	}
	if details.FullOutputPath == "" {
		t.Fatal("full output path is empty")
	}
	data, err := os.ReadFile(details.FullOutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "1\n2\n3") || !strings.Contains(string(data), "2998\n2999\n3000") {
		t.Fatal("full output does not contain the beginning and end")
	}
}

func pad4(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}
