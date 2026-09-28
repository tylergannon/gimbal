package gimbal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/runlog"
)

// fakePlaywrightScript stands in for playwright-cli on the host. It logs
// "<physical cwd>|<args>" for every call, fails the subcommands listed in
// FAKE_PLAYWRIGHT_FAIL, and on video-stop writes the recording that
// video-start named unless FAKE_PLAYWRIGHT_NOVIDEO is set.
const fakePlaywrightScript = `#!/bin/sh
printf '%s|%s\n' "$(pwd -P)" "$*" >> "$FAKE_PLAYWRIGHT_LOG"
session=
case $1 in -s=*) session=${1#-s=}; shift ;; esac
state="$FAKE_PLAYWRIGHT_STATE/$session"
case "$FAKE_PLAYWRIGHT_FAIL" in *" $1 "*) echo "$1 refused" >&2; exit 1 ;; esac
case $1 in
  video-start) printf '%s' "$2" > "$state.video" ;;
  video-stop) [ -n "$FAKE_PLAYWRIGHT_NOVIDEO" ] || printf 'webm' > "$(cat "$state.video")" ;;
esac
exit 0
`

// fakePlaywright puts fakePlaywrightScript first on PATH, with zsh's startup
// files isolated so they cannot put a real playwright-cli ahead of it, and
// returns its log. It fails the test unless zsh resolves the fake.
func fakePlaywright(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not installed")
	}
	bin := t.TempDir()
	fake := filepath.Join(bin, "playwright-cli")
	if err := os.WriteFile(fake, []byte(fakePlaywrightScript), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZDOTDIR", t.TempDir())
	t.Setenv("FAKE_PLAYWRIGHT_LOG", log)
	t.Setenv("FAKE_PLAYWRIGHT_STATE", t.TempDir())
	t.Setenv("FAKE_PLAYWRIGHT_FAIL", "")
	t.Setenv("FAKE_PLAYWRIGHT_NOVIDEO", "")
	resolved, err := exec.Command("zsh", "-c", "command -v playwright-cli").Output()
	if err != nil || strings.TrimSpace(string(resolved)) != fake {
		t.Fatalf("zsh resolves playwright-cli to %q (%v), not the fake %s", resolved, err, fake)
	}
	return log
}

func playwrightCalls(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// browserWorkdir is a workdir whose name needs shell quoting.
func browserWorkdir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "it's a work dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func physical(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestBrowserOpensDrivesAndClosesOnTheHost(t *testing.T) {
	log := fakePlaywright(t)
	workdir := browserWorkdir(t)
	video := filepath.Join(workdir, "run video.webm")
	var b *Browser
	var dir string
	err := runTest(t, nil, func(ctx context.Context) error {
		dir = runDir(ctx)
		var err error
		b, err = NewBrowser(ctx, "browser", workdir, video)
		if err != nil {
			return err
		}
		// The browser outlives the call that opened it: nothing has closed it.
		if calls := playwrightCalls(t, log); len(calls) != 2 {
			t.Errorf("calls while open = %q, want open and video-start only", calls)
		}
		info, err := os.Stat(b.wrapper)
		if err != nil || info.Mode()&0o111 == 0 {
			t.Fatalf("wrapper %s: %v, mode %v", b.wrapper, err, info)
		}
		elsewhere := t.TempDir()
		for _, args := range [][]string{
			{"open", "about:blank"}, {"close"}, {"close-all"}, {"kill-all"}, {"video-start", video}, {"video-stop"},
			{"-s=other", "snapshot"}, {"-s", "other", "snapshot"}, {"--session=other", "snapshot"}, {"--raw", "close"},
		} {
			before := len(playwrightCalls(t, log))
			cmd := exec.Command(b.wrapper, args...)
			cmd.Dir = elsewhere
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.HasPrefix(string(out), "browser: ") {
				t.Errorf("wrapper %q = %v, %q; want exit 2 with a browser: message", args, err, out)
			}
			if after := len(playwrightCalls(t, log)); after != before {
				t.Errorf("wrapper %q reached playwright-cli", args)
			}
		}
		for _, args := range [][]string{{"snapshot"}, {"--raw", "eval", "close"}} {
			cmd := exec.Command(b.wrapper, args...)
			cmd.Dir = elsewhere
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("wrapper %q = %v, %q", args, err, out)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	cwd, session := physical(t, workdir), b.session
	want := []string{
		cwd + "|-s=" + session + " open about:blank --idle-timeout=0",
		cwd + "|-s=" + session + " video-start " + video + " --cursor",
		cwd + "|-s=" + session + " snapshot",
		cwd + "|-s=" + session + " --raw eval close",
		cwd + "|-s=" + session + " video-stop",
		cwd + "|-s=" + session + " close",
	}
	if calls := playwrightCalls(t, log); !slices.Equal(calls, want) {
		t.Errorf("playwright-cli calls:\n got %q\nwant %q", calls, want)
	}
	if data, err := os.ReadFile(video); err != nil || len(data) == 0 {
		t.Errorf("video %s = %q, %v; want the finished recording", video, data, err)
	}

	var starts []CommandStarted
	if err := runlog.Read[LifecycleRecord](t.Context(), dir, func(record LifecycleRecord) error {
		if started, ok := record.Event.(CommandStarted); ok {
			starts = append(starts, started)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(starts) != 2 || starts[0].Args[1] != browserOpenScript || starts[1].Args[1] != browserCloseScript {
		t.Fatalf("recorded commands = %+v, want the open then the close", starts)
	}
	for _, started := range starts {
		if started.Name != "browser" || started.Command != "zsh" || started.Workdir != workdir {
			t.Errorf("recorded command = %+v, want browser's zsh in %s", started, workdir)
		}
	}
}

func TestBrowserSessionsAreDistinctPerHandleAndPerRun(t *testing.T) {
	log := fakePlaywright(t)
	workdir := browserWorkdir(t)
	project := t.TempDir()
	open := func(names ...string) []*Browser {
		var browsers []*Browser
		err := Run(Project(t.Context(), project), "identity", nil, func(ctx context.Context) error {
			return Scope(ctx, "evaluate", func(ctx context.Context) error {
				for _, name := range names {
					b, err := NewBrowser(ctx, name, workdir, "")
					if err != nil {
						return err
					}
					browsers = append(browsers, b)
				}
				return nil
			})
		})
		if err != nil {
			t.Fatal(err)
		}
		return browsers
	}
	first := open("browser", "browser")
	second := open("browser")
	all := append(slices.Clone(first), second...)
	sessions, wrappers := map[string]bool{}, map[string]bool{}
	for _, b := range all {
		if len(b.session) != 24 || strings.Trim(b.session, "0123456789abcdef") != "" {
			t.Errorf("session %q is not 24 hex characters", b.session)
		}
		if _, err := os.Stat(b.wrapper); err != nil {
			t.Error(err)
		}
		sessions[b.session], wrappers[b.wrapper] = true, true
	}
	if len(sessions) != 3 || len(wrappers) != 3 {
		t.Errorf("sessions %v, wrappers %v; want three of each", sessions, wrappers)
	}
	if first[0].id == first[1].id {
		t.Errorf("two handles in one scope share id %q", first[0].id)
	}
	if first[0].id != second[0].id {
		t.Errorf("ids %q and %q; the runs should share the scope path and differ only by run", first[0].id, second[0].id)
	}
	for _, b := range all {
		closes := 0
		for _, call := range playwrightCalls(t, log) {
			if strings.HasSuffix(call, "|-s="+b.session+" close") {
				closes++
			}
		}
		if closes != 1 {
			t.Errorf("session %s closed %d times, want once", b.session, closes)
		}
	}
}

func TestBrowserThatFailsToOpenIsClosedAgain(t *testing.T) {
	for _, tc := range []struct {
		fail string
		want []string
	}{
		{" open ", []string{"open refused"}},
		{" video-start close ", []string{"video-start refused", "close refused"}},
	} {
		t.Run(strings.TrimSpace(tc.fail), func(t *testing.T) {
			log := fakePlaywright(t)
			t.Setenv("FAKE_PLAYWRIGHT_FAIL", tc.fail)
			workdir := browserWorkdir(t)
			var b *Browser
			err := runTest(t, nil, func(ctx context.Context) error {
				var err error
				b, err = NewBrowser(ctx, "browser", workdir, filepath.Join(workdir, "v.webm"))
				return err
			})
			if b != nil || err == nil || !strings.Contains(err.Error(), "gimbal: browser browser.1: open: ") {
				t.Fatalf("NewBrowser = %v, %v; want no browser and the open failure", b, err)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
			calls := playwrightCalls(t, log)
			closes := 0
			for _, call := range calls {
				if strings.HasSuffix(call, " close") {
					closes++
				}
			}
			if closes != 1 {
				t.Errorf("calls %q; want exactly one close, not repeated when the scope ends", calls)
			}
		})
	}
}

func TestBrowserWithAnEmptyRecordingFailsItsScope(t *testing.T) {
	fakePlaywright(t)
	t.Setenv("FAKE_PLAYWRIGHT_NOVIDEO", "1")
	workdir := browserWorkdir(t)
	video := filepath.Join(workdir, "v.webm")
	err := runTest(t, nil, func(ctx context.Context) error {
		_, err := NewBrowser(ctx, "browser", workdir, video)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "video missing or empty: "+video) || !strings.Contains(err.Error(), "browser browser.1") {
		t.Fatalf("run error = %v, want the missing recording named for its browser", err)
	}
}

func TestNewBrowserRefusesRelativePaths(t *testing.T) {
	log := fakePlaywright(t)
	workdir := browserWorkdir(t)
	err := runTest(t, nil, func(ctx context.Context) error {
		for _, tc := range [][2]string{
			{"relative", ""},
			{workdir, "relative.webm"},
			{workdir, filepath.Join(workdir, "video.mp4")},
		} {
			if b, err := NewBrowser(ctx, "browser", tc[0], tc[1]); b != nil || err == nil {
				t.Errorf("NewBrowser(%q, %q) = %v, %v; want an error", tc[0], tc[1], b, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls := playwrightCalls(t, log); len(calls) != 0 {
		t.Errorf("calls %q; a refused browser runs nothing", calls)
	}
}

// browserTestEnvironment runs a browser's open and close and a service in
// place of a container, logging each in order. Its services block until
// stopped.
type browserTestEnvironment struct {
	adapter HarnessAdapter

	mu        sync.Mutex
	log       []string
	commands  []ExecutionCommand
	closeCode int
	// closeErr and closeDeadline are the close ctx's state when it started.
	closeErr      error
	closeDeadline time.Time
}

func (e *browserTestEnvironment) Harness(WorkflowRole) HarnessAdapter { return e.adapter }

func (e *browserTestEnvironment) record(entry string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.log = append(e.log, entry)
}

func (e *browserTestEnvironment) entries() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.log)
}

func (e *browserTestEnvironment) Start(ctx context.Context, command ExecutionCommand, _, stderr io.Writer) (ExecutionProcess, error) {
	e.mu.Lock()
	e.commands = append(e.commands, command)
	e.mu.Unlock()
	switch command.Args[1] {
	case browserOpenScript:
		e.record("open")
		return &browserTestProcess{workdir: command.Workdir}, nil
	case browserCloseScript:
		e.record("close")
		e.mu.Lock()
		e.closeErr = ctx.Err()
		e.closeDeadline, _ = ctx.Deadline()
		code := e.closeCode
		e.mu.Unlock()
		if code != 0 {
			_, _ = io.WriteString(stderr, "close refused")
		}
		return &browserTestProcess{workdir: command.Workdir, code: code}, nil
	}
	e.record("start " + command.Args[1])
	return &browserTestProcess{workdir: command.Workdir, service: command.Args[1], env: e, stopped: make(chan struct{})}, nil
}

type browserTestProcess struct {
	workdir string
	code    int
	service string // "" for a command, which exits at once
	env     *browserTestEnvironment
	once    sync.Once
	stopped chan struct{}
}

func (p *browserTestProcess) Workdir() string { return p.workdir }
func (p *browserTestProcess) Wait() (int, error) {
	if p.service != "" {
		<-p.stopped
	}
	return p.code, nil
}
func (p *browserTestProcess) Stop() error {
	if p.service != "" {
		p.once.Do(func() { p.env.record("stop " + p.service); close(p.stopped) })
	}
	return nil
}

type browserTestBackend struct{ env *browserTestEnvironment }

func (b browserTestBackend) Resolve(context.Context, string) (ExecutionEnvironment, error) {
	return b.env, nil
}
func (browserTestBackend) Close() error { return nil }

func TestBrowserClosesInItsEnvironmentBeforeServicesAndSessions(t *testing.T) {
	var prompts []string
	f := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		prompts = append(prompts, prompt)
		return "ok", nil
	}}
	env := &browserTestEnvironment{adapter: f, closeCode: 1}
	f.onClose = func(context.Context, string) { env.record("close session") }
	workdir := "/remote/it's a work dir"
	var b *Browser
	err := Run(Project(t.Context(), t.TempDir()), "environment", bind(f, "m", "tester"), func(ctx context.Context) error {
		ctx = InEnvironment(ctx, "docker")
		return Scope(ctx, "evaluate", func(ctx context.Context) error {
			if err := Service(ctx, "app", workdir, "serve"); err != nil {
				return err
			}
			var err error
			if b, err = NewBrowser(ctx, "browser", workdir, "/remote/v.webm"); err != nil {
				return err
			}
			tester := NewSession(ctx, "tester", workdir)
			_, err = tester.Generate[Text](ctx, "test the app", WithBrowser(b))
			return err
		})
	}, WithExecution(browserTestBackend{env}))
	if err == nil || !strings.Contains(err.Error(), "gimbal: browser evaluate.1/browser.1 (session "+b.session+"): close: exit 1: close refused") {
		t.Fatalf("run error = %v, want the close failure named for its browser", err)
	}
	want := []string{"start serve", "open", "close", "stop serve", "close session"}
	if got := env.entries(); !slices.Equal(got, want) {
		t.Errorf("order %q, want %q", got, want)
	}
	for _, command := range env.commands {
		if command.Workdir != workdir || command.Command != "zsh" {
			t.Errorf("command %+v, want zsh in %s", command, workdir)
		}
	}
	if open := env.commands[1]; open.Args[3] != b.wrapper || open.Args[5] != b.session || open.Args[6] != "/remote/v.webm" {
		t.Errorf("open args %q", open.Args)
	}
	if len(prompts) != 1 || !strings.HasPrefix(prompts[0], "test the app\n\n") || !strings.Contains(prompts[0], browserShellQuote(b.wrapper)) {
		t.Errorf("prompts %q, want the task and this browser's access", prompts)
	}
}

func TestBrowserClosesAfterTheRunIsCancelled(t *testing.T) {
	env := &browserTestEnvironment{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_ = Run(Project(ctx, t.TempDir()), "cancelled", nil, func(ctx context.Context) error {
		ctx = InEnvironment(ctx, "docker")
		if _, err := NewBrowser(ctx, "browser", "/remote", ""); err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	}, WithExecution(browserTestBackend{env}))
	if got := env.entries(); !slices.Equal(got, []string{"open", "close"}) {
		t.Fatalf("order %q, want the browser closed", got)
	}
	if env.closeErr != nil {
		t.Errorf("close started on a done ctx: %v", env.closeErr)
	}
	if env.closeDeadline.IsZero() || time.Until(env.closeDeadline) > browserCloseTimeout {
		t.Errorf("close deadline %v; want one at most %s away", env.closeDeadline, browserCloseTimeout)
	}
}

func TestWithBrowserRefusesABrowserItCannotUse(t *testing.T) {
	calls := 0
	f := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		calls++
		return "ok", nil
	}}
	env := &browserTestEnvironment{adapter: f}
	var errs []error
	err := Run(Project(t.Context(), t.TempDir()), "refusals", bind(f, "m", "tester", "host"), func(ctx context.Context) error {
		remote := InEnvironment(ctx, "docker")
		tester := NewSession(remote, "tester", "/remote")
		onHost := NewSession(ctx, "host", "/remote")
		ask := func(ctx context.Context, s *Session, b *Browser) {
			_, err := s.Generate[Text](ctx, "use it", WithBrowser(b))
			errs = append(errs, err)
		}
		ask(remote, tester, nil)
		var ended *Browser
		if err := Scope(remote, "inner", func(inner context.Context) error {
			b, err := NewBrowser(inner, "browser", "/remote", "")
			if err != nil {
				return err
			}
			ended = b
			ask(remote, tester, b)
			ask(inner, onHost, b)
			return Scope(remote, "sibling", func(sibling context.Context) error {
				ask(sibling, tester, b)
				return nil
			})
		}); err != nil {
			return err
		}
		ask(remote, tester, ended)
		return nil
	}, WithExecution(browserTestBackend{env}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"gimbal: tester.1: WithBrowser was given a nil browser",
		`gimbal: tester.1: browser inner.1/browser.1 belongs to scope "inner.1", which scope "" is not inside`,
		`gimbal: host.1: browser inner.1/browser.1 is in environment "docker", but the session is in environment ""`,
		`gimbal: tester.1: browser inner.1/browser.1 belongs to scope "inner.1", which scope "sibling.1" is not inside`,
		"gimbal: tester.1: browser inner.1/browser.1 was used after its scope ended",
	}
	if len(errs) != len(want) {
		t.Fatalf("errors %v, want %d", errs, len(want))
	}
	for i := range want {
		if errs[i] == nil || errs[i].Error() != want[i] {
			t.Errorf("error %d = %v, want %s", i, errs[i], want[i])
		}
	}
	if calls != 0 {
		t.Errorf("%d model calls; a refused browser reaches no model", calls)
	}
}

func TestWithBrowserGrantsAccessPerAgent(t *testing.T) {
	// Each native session's prompts, and which agent it is, told by its first
	// prompt: the worker's task or a supervisor's instruction.
	var mu sync.Mutex
	prompts := map[string][]string{}
	agent := map[string]string{}
	looked := func() bool {
		mu.Lock()
		defer mu.Unlock()
		var granted, plain bool
		for native, role := range agent {
			granted = granted || role == "granted" && len(prompts[native]) > 0
			plain = plain || role == "plain" && len(prompts[native]) > 0
		}
		return granted && plain
	}
	f := &fake{}
	f.answer = func(_ context.Context, session, prompt string, _ json.RawMessage, emit func(AgentEvent) error) (string, error) {
		mu.Lock()
		if agent[session] == "" {
			switch {
			case strings.HasPrefix(prompt, "drive the app"):
				agent[session] = "worker"
			case strings.Contains(prompt, "watch with the browser"):
				agent[session] = "granted"
			case strings.Contains(prompt, "watch without it"):
				agent[session] = "plain"
			}
		}
		prompts[session] = append(prompts[session], prompt)
		role := agent[session]
		mu.Unlock()
		if role == "worker" {
			_ = emit(fakeAgentEvent("session.tool.called", session, "message-1", map[string]any{"id": "1", "input": map[string]any{"command": "snapshot"}, "executed": true}))
			for i := 0; i < 400 && !looked(); i++ {
				time.Sleep(5 * time.Millisecond)
			}
			return "done", nil
		}
		return `{"objections": []}`, nil
	}
	env := &browserTestEnvironment{adapter: f}
	var b *Browser
	var dir, worker string
	err := Run(Project(t.Context(), t.TempDir()), "grants", bind(f, "m", "tester", "watch"), func(ctx context.Context) error {
		dir = runDir(ctx)
		ctx = InEnvironment(ctx, "docker")
		var err error
		if b, err = NewBrowser(ctx, "browser", "/remote", ""); err != nil {
			return err
		}
		tester := NewSession(ctx, "tester", "/remote")
		worker = tester.id
		granted := NewSession(ctx, "watch", "/remote")
		plain := NewSession(ctx, "watch", "/remote")
		_, err = tester.Generate[Text](ctx, "drive the app", WithBrowser(b),
			WithSupervisor(granted, "watch with the browser", WithInterval(10*time.Millisecond), WithBrowser(b)),
			WithSupervisor(plain, "watch without it", WithInterval(10*time.Millisecond)))
		return err
	}, WithExecution(browserTestBackend{env}))
	if err != nil {
		t.Fatal(err)
	}
	if !looked() {
		t.Fatal("both supervisors did not look")
	}
	command := browserShellQuote(b.wrapper)
	access := "\n\nYou have a browser that is already open for you. Drive it only with the shell-quoted command " + command
	const accessEnd = "Do not call playwright-cli directly."
	seen := map[string]int{}
	for native, role := range agent {
		seen[role]++
		for _, prompt := range prompts[native] {
			switch role {
			case "worker":
				if !strings.HasPrefix(prompt, "drive the app"+access) || !strings.HasSuffix(prompt, accessEnd) {
					t.Errorf("worker prompt %q, want the task then its access", prompt)
				}
			case "granted":
				if !strings.HasSuffix(prompt, accessEnd) {
					t.Errorf("granted supervisor's look %q does not end with its own access", prompt)
				}
			case "plain":
				if strings.HasSuffix(prompt, accessEnd) {
					t.Errorf("ungranted supervisor's look ends with access: %q", prompt)
				}
			}
		}
	}
	if seen["worker"] != 1 || seen["granted"] != 1 || seen["plain"] != 1 {
		t.Fatalf("agents %v, want one worker and two supervisors", agent)
	}
	for native, role := range agent {
		if role == "plain" && !strings.Contains(prompts[native][0], command) {
			t.Errorf("ungranted supervisor's first look %q does not quote the worker's task and its access", prompts[native][0])
		}
	}
	var started []TurnStarted
	if err := runlog.Read[LifecycleRecord](t.Context(), dir, func(record LifecycleRecord) error {
		if turn, ok := record.Event.(TurnStarted); ok && record.Session.Value == worker {
			started = append(started, turn)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(started) != 1 || started[0].Prompt != "drive the app" {
		t.Errorf("worker TurnStarted = %+v, want the workflow's prompt", started)
	}
	enqueued := false
	transcripts, _ := filepath.Glob(filepath.Join(dir, "sessions", "*.jsonl"))
	for _, path := range transcripts {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := bufio.NewScanner(file)
		lines.Buffer(nil, 1<<20)
		for lines.Scan() {
			line := lines.Text()
			enqueued = enqueued || strings.Contains(line, "session.inbox.enqueued") && strings.Contains(line, `"drive the app\n\nYou have a browser`)
		}
		_ = file.Close()
	}
	if !enqueued {
		t.Errorf("no transcript in %v records the prompt sent with its access", transcripts)
	}
}

func TestBrowserOpenStopsIfWrapperCannotBeWritten(t *testing.T) {
	log := fakePlaywright(t)
	// A directory cannot be overwritten by printf, even when tests run as root.
	cmd := exec.Command("zsh", "-c", browserOpenScript, "zsh", t.TempDir(), "wrapper", "session", "")
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("open succeeded without writing its wrapper: %s", output)
	}
	if calls := playwrightCalls(t, log); len(calls) != 0 {
		t.Fatalf("browser started after wrapper write failed: %q", calls)
	}
}
