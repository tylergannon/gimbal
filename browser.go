package gimbal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// browserCloseTimeout bounds stopping a browser's recording and closing it,
// on a ctx independent of the scope's own cancellation.
const browserCloseTimeout = 30 * time.Second

// browserOpenScript writes the handle's wrapper, opens the browser with its
// idle shutdown disabled, and starts recording when a video path is given.
// $1 wrapper path, $2 wrapper text, $3 session, $4 video or "".
const browserOpenScript = `set -e
printf '%s' "$2" > "$1"
chmod +x "$1"
playwright-cli -s="$3" open about:blank --idle-timeout=0
[ -z "$4" ] || playwright-cli -s="$3" video-start "$4" --cursor
`

// browserCloseScript finalizes the recording before closing, since close
// while recording writes no file, and always attempts close.
// $1 session, $2 video or "".
const browserCloseScript = `rc=0
if [ -n "$2" ]; then playwright-cli -s="$1" video-stop || rc=1; test -s "$2" || { echo "video missing or empty: $2" >&2; rc=1; }; fi
playwright-cli -s="$1" close || rc=1
exit $rc
`

// browserWrapperScript is the command an agent is given. It refuses a session
// flag and the lifecycle subcommands, which belong to the workflow, then runs
// the rest in the browser's workdir on its fixed session.
const browserWrapperScript = `#!/bin/sh
for arg in "$@"; do
  case $arg in
    -s|-s=*|--session|--session=*) echo "browser: the session is fixed" >&2; exit 2 ;;
  esac
done
for arg in "$@"; do
  case $arg in
    -*) ;;
    open|attach|detach|close|close-all|kill-all|delete-data|video-start|video-stop|install|install-browser)
      echo "browser: $arg belongs to the workflow, not the agent" >&2; exit 2 ;;
    *) break ;;
  esac
done
cd %s && exec playwright-cli -s=%s "$@"
`

// browserAccessText is what WithBrowser appends to an agent's prompt.
const browserAccessText = "You have a browser that is already open for you. Drive it only with the shell-quoted command %[1]s, " +
	"for example `%[1]s goto '<url>'`, `%[1]s snapshot`, `%[1]s click <ref>`, `%[1]s screenshot --filename=%[2]s`. " +
	"Save screenshots in %[3]s. The workflow opens, records and closes this browser; the command refuses open, close, video and session commands. " +
	"Do not call playwright-cli directly."

// NewBrowser opens a browser named name in the ctx's environment and adopts
// it into the ctx's scope. workdir is the absolute directory every browser
// command runs in, and where its screenshots go. video is an absolute .webm
// path to record into, or "" for none. The browser is released when its
// scope ends. There is no Close.
//
// When the scope ends, before its services stop and its sessions close, the
// recording is stopped and checked to be non-empty and the browser is closed;
// a failure there enters the scope's error. A browser that fails to open is
// closed again at once, and both failures are returned. name is the browser's
// constant name for the graph and run record.
func NewBrowser(ctx context.Context, name, workdir, video string) (*Browser, error) {
	owner, err := current(ctx)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(workdir) {
		return nil, fmt.Errorf("gimbal: browser %q: workdir %q is not absolute", name, workdir)
	}
	if video != "" && (!filepath.IsAbs(video) || filepath.Ext(video) != ".webm") {
		return nil, fmt.Errorf("gimbal: browser %q: video %q is not an absolute .webm path", name, video)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gimbal: browser %q: %w", name, err)
	}
	environment, _ := ctx.Value(environmentKey{}).(string)
	b := &Browser{
		name:        name,
		workdir:     filepath.Clean(workdir),
		video:       video,
		environment: environment,
		owner:       owner,
		createCtx:   context.WithoutCancel(ctx),
	}
	if err := owner.adoptBrowser(b); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(runDir(ctx) + "/" + b.id))
	b.session = hex.EncodeToString(sum[:12])
	b.wrapper = filepath.Join(b.workdir, ".gimbal-browser-"+b.session)
	wrapperText := fmt.Sprintf(browserWrapperScript, browserShellQuote(b.workdir), b.session)

	_, ended, openErr, recordErr := runCommand(ctx, owner, name, b.workdir, "zsh", []string{"-c", browserOpenScript, "zsh", b.wrapper, wrapperText, b.session, video})
	if openErr == nil && ended.ExitCode != 0 {
		openErr = fmt.Errorf("exit %d: %s", ended.ExitCode, strings.TrimSpace(ended.Stderr))
	}
	if openErr = errors.Join(openErr, recordErr); openErr != nil {
		openErr = fmt.Errorf("gimbal: browser %s: open: %w", b.id, openErr)
		return nil, errors.Join(openErr, b.close())
	}
	return b, nil
}

// Browser is a live browser session owned by the scope that created it.
type Browser struct {
	id, name, workdir, video string          // id = scope.next(name)
	session                  string          // first 12 bytes of SHA256(runDir(ctx)+"/"+id), as 24 hex characters
	wrapper                  string          // workdir + "/.gimbal-browser-" + session: one per handle
	environment              string          // environment name from ctx at creation ("" = host)
	owner                    *scope          // the scope that created it and releases it
	createCtx                context.Context // context.WithoutCancel(ctx), used for close

	mu       sync.Mutex
	released bool
}

// WithBrowser gives this turn's agent working access to b: the prompt it is
// sent ends with instructions for driving b through a command that runs on
// b's session only. Access is per agent. A supervisor gets it only through
// its own options to WithSupervisor; otherwise it sees the worker's
// instructions only as part of the task it reviews. The turn fails before
// any model is called when b is released, when the ctx's scope is not b's
// scope or one inside it, or when the session is in a different environment
// than b.
func WithBrowser(b *Browser) AgentOption {
	return func(o *options) { o.browsers = append(o.browsers, b) }
}

// close stops b's recording, closes it, and marks it released. It runs once;
// a later call returns nil. A failure names the browser, and is not a
// confirmed release.
func (b *Browser) close() error {
	b.mu.Lock()
	if b.released {
		b.mu.Unlock()
		return nil
	}
	b.released = true
	b.mu.Unlock()
	ctx, cancel := context.WithTimeout(b.createCtx, browserCloseTimeout)
	defer cancel()
	_, ended, err, recordErr := runCommand(ctx, b.owner, b.name, b.workdir, "zsh", []string{"-c", browserCloseScript, "zsh", b.session, b.video})
	if err == nil && ended.ExitCode != 0 {
		err = fmt.Errorf("exit %d: %s", ended.ExitCode, strings.TrimSpace(ended.Stderr))
	}
	if err = errors.Join(err, recordErr); err != nil {
		return fmt.Errorf("gimbal: browser %s (session %s): close: %w", b.id, b.session, err)
	}
	return nil
}

// browserAccess appends each browser's access block to prompt, after checking
// that the agent may use it here.
func browserAccess(ctx context.Context, s *Session, prompt string, browsers []*Browser) (string, error) {
	if len(browsers) == 0 {
		return prompt, nil
	}
	here, err := current(ctx)
	if err != nil {
		return "", err
	}
	for _, b := range browsers {
		if b == nil {
			return "", fmt.Errorf("gimbal: %s: WithBrowser was given a nil browser", s.id)
		}
		if err := b.usableFrom(here, s); err != nil {
			return "", fmt.Errorf("gimbal: %s: %w", s.id, err)
		}
		quoted := browserShellQuote(b.wrapper)
		screenshot := browserShellQuote(filepath.Join(b.workdir, "<name>.png"))
		prompt += "\n\n" + fmt.Sprintf(browserAccessText, quoted, screenshot, browserShellQuote(b.workdir))
	}
	return prompt, nil
}

// usableFrom reports why s, in the scope here, cannot drive b.
func (b *Browser) usableFrom(here *scope, s *Session) error {
	b.mu.Lock()
	released := b.released
	b.mu.Unlock()
	b.owner.mu.Lock()
	ended := b.owner.ended
	b.owner.mu.Unlock()
	switch {
	case released || ended:
		return fmt.Errorf("browser %s was used after its scope ended", b.id)
	case !within(here, b.owner):
		return fmt.Errorf("browser %s belongs to scope %q, which scope %q is not inside", b.id, b.owner.key, here.key)
	case b.environment != s.environment:
		return fmt.Errorf("browser %s is in environment %q, but the session is in environment %q", b.id, b.environment, s.environment)
	}
	return nil
}

// within reports whether s is owner or a scope inside it.
func within(s, owner *scope) bool {
	for ; s != nil; s = s.parent {
		if s == owner {
			return true
		}
	}
	return false
}

// browserShellQuote quotes value as one POSIX shell word.
func browserShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
