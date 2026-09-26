package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// BashToolSnippet is the prompt snippet bash contributes, pi's
// bashToolSystemPromptContribution.snippet.
const BashToolSnippet = "Execute bash commands (ls, grep, find, etc.)"

// BashToolGuidelines is the prompt guideline bash contributes.
var BashToolGuidelines = []string{
	"You can inspect PI_* environment variables for current model and session details.",
}

// BashToolDetails is the structured details the bash tool returns. Either field
// is absent when it does not apply.
type BashToolDetails struct {
	Truncation     *files.Result `json:"truncation,omitempty"`
	FullOutputPath string        `json:"fullOutputPath,omitempty"`
}

// BashSpawnContext is the command, working directory and environment handed to
// a spawn hook before execution (pi BashSpawnContext).
type BashSpawnContext struct {
	Command string
	Cwd     string
	Env     []string
}

// BashSpawnHook may rewrite the command, cwd or environment before spawning.
type BashSpawnHook func(BashSpawnContext) BashSpawnContext

// BashToolOptions configures CreateBashTool. The zero value uses the local
// shell, exposes session environment variables when a SessionEnv is supplied,
// and applies no command prefix.
type BashToolOptions struct {
	// Operations overrides process execution. Nil uses the local shell.
	Operations *BashOperations
	// CommandPrefix is prepended, on its own line, to every command.
	CommandPrefix string
	// ShellPath is an explicit shell binary used by the local operations.
	ShellPath string
	// ExposeSessionEnvironment controls whether SessionEnv values are added as
	// PI_* environment variables. Nil means true.
	ExposeSessionEnvironment *bool
	// SpawnHook adjusts the command, cwd or environment before execution.
	SpawnHook BashSpawnHook
	// SessionEnv supplies current PI_* session metadata. It is only consulted
	// when session environment exposure is enabled.
	SessionEnv func() map[string]string
}

// ShellToolConfig is the per-tool configuration pi factors out of its shared
// createShellToolDefinition.
type ShellToolConfig struct {
	Name             string
	Label            string
	ShellName        string
	TempFilePrefix   string
	ResolveShell     func() (ShellConfig, error)
	PromptSnippet    string
	PromptGuidelines []string
}

// CreateBashTool builds pi's bash tool rooted at cwd.
func CreateBashTool(cwd string, options *BashToolOptions) model.AgentTool {
	return CreateShellTool(cwd, ShellToolConfig{
		Name:             "bash",
		Label:            "bash",
		ShellName:        "bash",
		TempFilePrefix:   "pi-bash",
		PromptSnippet:    BashToolSnippet,
		PromptGuidelines: BashToolGuidelines,
	}, options)
}

// CreateShellTool builds a shell tool from a config, pi's
// createShellToolDefinition. The bash tool is the only built-in instance in
// this port; the config seam keeps the shape for a caller that wants another
// shell.
func CreateShellTool(cwd string, config ShellToolConfig, options *BashToolOptions) model.AgentTool {
	if options == nil {
		options = &BashToolOptions{}
	}
	if config.ResolveShell == nil {
		config.ResolveShell = func() (ShellConfig, error) { return GetShellConfig(options.ShellPath) }
	}
	ops := localShellOperations(config.ShellName, config.ResolveShell)
	if options.Operations != nil {
		ops = *options.Operations
	}

	exposeSessionEnvironment := options.ExposeSessionEnvironment == nil || *options.ExposeSessionEnvironment
	commandPrefix := options.CommandPrefix
	spawnHook := options.SpawnHook
	sessionEnv := options.SessionEnv

	return model.AgentTool{
		Name:  config.Name,
		Label: config.Label,
		Description: fmt.Sprintf(
			"Execute a %s command in the current working directory. Returns stdout and stderr. Output is truncated to last %d lines or %dKB (whichever is hit first). If truncated, full output is saved to a temp file. Optionally provide a timeout in seconds.",
			config.ShellName, files.DefaultMaxLines, files.DefaultMaxBytes/1024,
		),
		Parameters: model.Object(
			model.Prop("command", &model.Schema{Type: "string", Description: "Shell command to execute"}),
			model.Opt("timeout", &model.Schema{Type: "number", Description: "Timeout in seconds (optional, no default timeout)"}),
		),
		ConstrainedSampling: &model.ConstrainedSamplingConfig{
			Type:   model.ConstrainedSamplingJSONSchema,
			Strict: model.ConstrainedSamplingPrefer,
		},
		Execute: func(ctx context.Context, _ string, params map[string]any, onUpdate model.ToolUpdateFunc) (model.AgentToolResult, error) {
			command := argString(params, "command")
			if commandPrefix != "" {
				command = commandPrefix + "\n" + command
			}

			timeout, hasTimeout := argFloat(params, "timeout")
			if hasTimeout && timeout <= 0 {
				return model.AgentToolResult{}, errors.New("Invalid timeout: must be a finite number of seconds") //nolint:staticcheck // byte-exact pi error string
			}

			envSession := func() map[string]string { return nil }
			if exposeSessionEnvironment {
				envSession = sessionEnv
			}
			spawnContext := BashSpawnContext{
				Command: command,
				Cwd:     cwd,
				Env:     bashCommandEnv(envSession),
			}
			if spawnHook != nil {
				spawnContext = spawnHook(spawnContext)
			}

			updater := newBashUpdater(onUpdate, NewOutputAccumulator(OutputAccumulatorOptions{
				TempFilePrefix: config.TempFilePrefix,
			}))
			if onUpdate != nil {
				// pi emits an initial empty update before spawning.
				onUpdate(model.AgentToolResult{Content: model.ContentList{}})
			}

			execOptions := BashExecOptions{
				OnData: updater.write,
				Env:    spawnContext.Env,
			}
			if hasTimeout {
				execOptions.TimeoutSeconds = timeout
			}
			exitCode, execErr := ops.Exec(ctx, spawnContext.Command, spawnContext.Cwd, execOptions)

			snapshot := updater.finish()
			formatOutput := func(emptyText string) (string, *BashToolDetails) {
				text := snapshot.Content
				if text == "" {
					text = emptyText
				}
				var details *BashToolDetails
				truncation := snapshot.Truncation
				if truncation.Truncated {
					details = &BashToolDetails{FullOutputPath: snapshot.FullOutputPath}
					details.Truncation = &truncation
					startLine := truncation.TotalLines - truncation.OutputLines + 1
					endLine := truncation.TotalLines
					switch {
					case truncation.LastLinePartial:
						lastLineSize := files.FormatSize(updater.lastLineBytes())
						text += fmt.Sprintf("\n\n[Showing last %s of line %d (line is %s). Full output: %s]",
							files.FormatSize(truncation.OutputBytes), endLine, lastLineSize, snapshot.FullOutputPath)
					case truncation.TruncatedBy != nil && *truncation.TruncatedBy == "lines":
						text += fmt.Sprintf("\n\n[Showing lines %d-%d of %d. Full output: %s]",
							startLine, endLine, truncation.TotalLines, snapshot.FullOutputPath)
					default:
						text += fmt.Sprintf("\n\n[Showing lines %d-%d of %d (%s limit). Full output: %s]",
							startLine, endLine, truncation.TotalLines, files.FormatSize(files.DefaultMaxBytes), snapshot.FullOutputPath)
					}
				}
				return text, details
			}
			appendStatus := func(text, status string) string {
				if text == "" {
					return status
				}
				return text + "\n\n" + status
			}

			if execErr != nil {
				if errors.Is(execErr, ErrShellAborted) {
					text, _ := formatOutput("")
					return model.AgentToolResult{}, errors.New(appendStatus(text, "Command aborted"))
				}
				if timeoutErr, ok := errors.AsType[*ShellTimeoutError](execErr); ok {
					text, _ := formatOutput("")
					return model.AgentToolResult{}, fmt.Errorf("%s", appendStatus(text,
						fmt.Sprintf("Command timed out after %s seconds", formatSeconds(timeoutErr.Seconds))))
				}
				return model.AgentToolResult{}, execErr
			}
			if exitCode == nil {
				text, _ := formatOutput("(no output)")
				return model.AgentToolResult{}, errors.New(appendStatus(text, "Command terminated without an exit code"))
			}
			if *exitCode != 0 {
				text, _ := formatOutput("(no output)")
				return model.AgentToolResult{}, fmt.Errorf("%s", appendStatus(text, fmt.Sprintf("Command exited with code %d", *exitCode)))
			}
			text, details := formatOutput("(no output)")
			result := model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: text}}}
			if details != nil {
				result.Details = details
			}
			return result, nil
		},
	}
}

// bashUpdateThrottle is pi's BASH_UPDATE_THROTTLE_MS.
const bashUpdateThrottle = 100 * time.Millisecond

// bashUpdater throttles partial onUpdate emits (leading and trailing edge) and
// serializes accumulator access across the exec pipe goroutines.
type bashUpdater struct {
	mu       sync.Mutex
	onUpdate model.ToolUpdateFunc
	acc      *OutputAccumulator
	// accepting guards against output callbacks that arrive after the
	// operation resolved (pi#5208).
	accepting  bool
	dirty      bool
	lastUpdate time.Time
	timer      *time.Timer
}

func newBashUpdater(onUpdate model.ToolUpdateFunc, acc *OutputAccumulator) *bashUpdater {
	return &bashUpdater{onUpdate: onUpdate, acc: acc, accepting: true}
}

func (u *bashUpdater) write(p []byte) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.accepting {
		return
	}
	u.acc.Append(p)
	u.scheduleLocked()
}

// finish flushes the trailing-edge update, finalizes the accumulator and
// returns the final snapshot.
func (u *bashUpdater) finish() OutputSnapshot {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.accepting = false
	u.acc.Finish()
	u.clearTimerLocked()
	u.emitLocked()
	snapshot := u.acc.Snapshot(true)
	_ = u.acc.CloseTempFile()
	return snapshot
}

func (u *bashUpdater) lastLineBytes() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.acc.LastLineBytes()
}

func (u *bashUpdater) emitLocked() {
	if u.onUpdate == nil || !u.dirty {
		return
	}
	u.dirty = false
	u.lastUpdate = time.Now()
	snapshot := u.acc.Snapshot(true)
	var details *BashToolDetails
	if snapshot.Truncation.Truncated || snapshot.FullOutputPath != "" {
		details = &BashToolDetails{FullOutputPath: snapshot.FullOutputPath}
		if snapshot.Truncation.Truncated {
			truncation := snapshot.Truncation
			details.Truncation = &truncation
		}
	}
	u.onUpdate(model.AgentToolResult{
		Content: model.ContentList{model.TextContent{Text: snapshot.Content}},
		Details: details,
	})
}

func (u *bashUpdater) scheduleLocked() {
	if u.onUpdate == nil {
		return
	}
	u.dirty = true
	delay := bashUpdateThrottle - time.Since(u.lastUpdate)
	if delay <= 0 {
		u.clearTimerLocked()
		u.emitLocked()
		return
	}
	if u.timer == nil {
		u.timer = time.AfterFunc(delay, func() {
			u.mu.Lock()
			defer u.mu.Unlock()
			u.timer = nil
			u.emitLocked()
		})
	}
}

func (u *bashUpdater) clearTimerLocked() {
	if u.timer != nil {
		u.timer.Stop()
		u.timer = nil
	}
}

func argString(params map[string]any, key string) string {
	if value, ok := params[key].(string); ok {
		return value
	}
	return ""
}

func argFloat(params map[string]any, key string) (float64, bool) {
	switch value := params[key].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// formatSeconds renders a timeout the way pi's `timeout:${timeout}` does: 0.5
// stays "0.5" and 2 stays "2".
func formatSeconds(seconds float64) string {
	return strconv.FormatFloat(seconds, 'g', -1, 64)
}
