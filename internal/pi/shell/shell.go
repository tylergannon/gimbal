package shell

import (
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// ShellConfig is the resolved shell and the transport used to hand it a command.
// CommandTransport is "argv" (the command is appended to Args) or "stdin" (the
// command is written to the process's standard input), pi's ShellConfig.
type ShellConfig struct {
	Shell            string
	Args             []string
	CommandTransport string
}

const (
	shellTransportArgv  = "argv"
	shellTransportStdin = "stdin"
)

// legacyWslBashRE matches the legacy Windows-bundled WSL launcher
// (C:\Windows\System32\bash.exe, or the WoW64 sysnative redirect) after path
// normalization. It mishandles `-c "<cmd>"` quoting, so commands go via stdin.
var legacyWslBashRE = regexp.MustCompile(`^[a-z]:\\windows\\(?:system32|sysnative)\\bash\.exe$`)

// IsLegacyWSLBashPath reports whether path is the legacy WSL bash launcher.
func IsLegacyWSLBashPath(path string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(path, "/", `\`))
	return legacyWslBashRE.MatchString(normalized)
}

// bashShellConfig ports pi's getBashShellConfig: legacy WSL bash takes the
// command on stdin (`bash -s`); every other bash takes `bash -c <command>`.
func bashShellConfig(shell string) ShellConfig {
	if IsLegacyWSLBashPath(shell) {
		return ShellConfig{Shell: shell, Args: []string{"-s"}, CommandTransport: shellTransportStdin}
	}
	return ShellConfig{Shell: shell, Args: []string{"-c"}, CommandTransport: shellTransportArgv}
}

// GetShellConfig resolves the shell used to run a command, pi's getShellConfig
// (utils/shell.ts). Resolution order is: an explicit customShellPath, then
// /bin/bash, then bash on PATH, then a fallback to sh. A customShellPath that
// does not exist is an error.
//
// The Windows branches of pi's resolver (Git Bash in known locations, bash.exe
// on PATH) are intentionally absent: this port targets macOS/Linux.
func GetShellConfig(customShellPath string) (ShellConfig, error) {
	if customShellPath != "" {
		if fileExists(customShellPath) {
			return bashShellConfig(customShellPath), nil
		}
		return ShellConfig{}, &ShellPathNotFoundError{Path: customShellPath}
	}

	if fileExists("/bin/bash") {
		return bashShellConfig("/bin/bash"), nil
	}
	if path, err := exec.LookPath("bash"); err == nil && path != "" {
		return bashShellConfig(path), nil
	}
	return ShellConfig{Shell: "sh", Args: []string{"-c"}, CommandTransport: shellTransportArgv}, nil
}

// ShellPathNotFoundError mirrors pi's `Custom shell path not found: <path>`.
type ShellPathNotFoundError struct{ Path string }

func (e *ShellPathNotFoundError) Error() string {
	return "Custom shell path not found: " + e.Path
}

// fileExists reports whether path exists. It is a var so a test can substitute
// the check without touching the filesystem, matching pi's existsSync seam.
var fileExists = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetShellEnv returns the child environment pi builds in getShellEnv: the
// ambient environment with PATH carrying the pi bin directory first.
//
// pi prepends getBinDir() (join(getAgentDir(), "bin")); that directory is owned
// by the config module, which this package does not depend on, so only the
// deduplicating PATH handling is reproduced here. Callers that need the bin
// directory on PATH (there are none in the headless port) should add it before
// spawning.
func GetShellEnv() []string {
	return os.Environ()
}

// SanitizeBinaryOutput drops the characters pi's sanitizeBinaryOutput drops:
// control characters other than tab, newline and carriage return, and the
// Unicode format characters U+FFF9..U+FFFB. Invalid UTF-8 becomes U+FFFD, the
// same replacement pi's non-fatal TextDecoder produces.
func SanitizeBinaryOutput(s string) string {
	if utf8.ValidString(s) && !strings.ContainsFunc(s, needsSanitizing) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
			s = s[1:]
			continue
		}
		if allowedRune(r) {
			b.WriteString(s[:size])
		}
		s = s[size:]
	}
	return b.String()
}

func needsSanitizing(r rune) bool { return !allowedRune(r) && r != utf8.RuneError }

func allowedRune(r rune) bool {
	if r == '\t' || r == '\n' || r == '\r' {
		return true
	}
	if r <= 0x1f {
		return false
	}
	if r >= 0xfff9 && r <= 0xfffb {
		return false
	}
	return true
}

// ansiPattern is pi's ansi-regex / strip-ansi combined pattern. Valid string
// terminators are BEL, ESC-backslash or 0x9C; OSC sequences run to the first
// terminator, and CSI sequences are ESC/C1 followed by optional intermediates
// and parameters and a final byte.
var ansiPattern = regexp.MustCompile(
	`(?:\x1B\][\s\S]*?(?:\x07|\x1B\\|\x9C)|[\x1B\x9B][\[\]()#;?]*(?:\d{1,4}(?:[;:]\d{0,4})*)?[\dA-PR-TZcf-nq-uy=><~])`,
)

// StripAnsi removes ANSI escape sequences, porting strip-ansi as used by the
// bash executor.
func StripAnsi(s string) string {
	if strings.IndexByte(s, 0x1b) < 0 && strings.IndexByte(s, 0x9b) < 0 {
		return s
	}
	return ansiPattern.ReplaceAllString(s, "")
}

// The PI_* variables resolveSpawnContext clears from the inherited environment
// and then restores from the session.
var (
	piSessionEnvKeys = []string{
		"PI_SESSION_ID",
		"PI_SESSION_FILE",
		"PI_PROVIDER",
		"PI_MODEL",
		"PI_REASONING_LEVEL",
	}
	trackedMu       sync.Mutex
	trackedChildren = map[int]struct{}{}
)

// TrackDetachedChildPID records a child pid so it can be killed if the parent
// receives a shutdown signal (utils/shell.ts trackDetachedChildPid).
func TrackDetachedChildPID(pid int) {
	if pid <= 0 {
		return
	}
	trackedMu.Lock()
	trackedChildren[pid] = struct{}{}
	trackedMu.Unlock()
}

// UntrackDetachedChildPID drops a tracked pid once the child has been reaped.
func UntrackDetachedChildPID(pid int) {
	trackedMu.Lock()
	delete(trackedChildren, pid)
	trackedMu.Unlock()
}

// KillTrackedDetachedChildren kills every tracked detached child and clears the
// set (utils/shell.ts killTrackedDetachedChildren).
func KillTrackedDetachedChildren() {
	trackedMu.Lock()
	pids := make([]int, 0, len(trackedChildren))
	for pid := range trackedChildren {
		pids = append(pids, pid)
	}
	trackedChildren = map[int]struct{}{}
	trackedMu.Unlock()

	for _, pid := range pids {
		KillProcessTree(pid)
	}
}

// bashCommandEnv builds the child environment, pi's resolveSpawnContext: the
// inherited environment with the PI_* keys removed, then the supplied session
// values re-added in a stable order.
func bashCommandEnv(sessionEnv func() map[string]string) []string {
	environ := os.Environ()
	env := make([]string, 0, len(environ)+len(piSessionEnvKeys))
	for _, kv := range environ {
		if name, _, ok := strings.Cut(kv, "="); ok && slices.Contains(piSessionEnvKeys, name) {
			continue
		}
		env = append(env, kv)
	}
	if sessionEnv == nil {
		return env
	}
	provided := sessionEnv()
	for _, key := range piSessionEnvKeys {
		if value, ok := provided[key]; ok && value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}
