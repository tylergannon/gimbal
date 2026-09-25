// Command gimbal runs Gimbal's project runtime and web application, and
// lists and runs the workflows built into it.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tylergannon/gimbal/internal/gimballint"
	"github.com/tylergannon/gimbal/opencode"
	"github.com/tylergannon/gimbal/web"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	if analysisArgs, ok := routeAnalysis(os.Args[1:]); ok {
		os.Args = append([]string{os.Args[0]}, analysisArgs...)
		singlechecker.Main(gimballint.Analyzer)
		return
	}
	if code := executeCLI(os.Args[1:], os.Stdout, os.Stderr, os.Getenv, defaultArtifactUploaders()); code != 0 {
		os.Exit(code)
	}
}

func executeCLI(args []string, stdout, stderr io.Writer, getenv func(string) string, uploaders artifactUploaderConstructors) int {
	if err := runWithUploaders(args, stdout, stderr, getenv, uploaders); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintln(stderr, "gimbal:", err)
		return 1
	}
	return 0
}

// routeAnalysis recognizes the standalone lint command and the three calls in
// the go vet tool protocol before the ordinary Gimbal CLI parses its flags.
func routeAnalysis(args []string) ([]string, bool) {
	if len(args) > 0 && args[0] == "lint" {
		return args[1:], true
	}
	if len(args) == 1 && (args[0] == "-flags" || args[0] == "-V=full") {
		return args, true
	}
	if isOrdinaryCLI(args) {
		return nil, false
	}
	if len(args) > 0 && strings.HasSuffix(args[len(args)-1], ".cfg") && isVetConfig(args[len(args)-1]) {
		return args, true
	}
	return nil, false
}

func isOrdinaryCLI(args []string) bool {
	if len(args) > 0 {
		switch args[0] {
		case "run-prompt", "run", "runs", "watch", "steer", "count-tokens", "opencode", "upload-artifact":
			return true
		}
	}
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "-port", "--port", "-uds", "--uds", "-no-web", "--no-web":
			return true
		}
	}
	return false
}

func isVetConfig(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var config struct {
		ImportPath string
		GoFiles    []string
	}
	return json.Unmarshal(data, &config) == nil && config.ImportPath != "" && config.GoFiles != nil
}

// run is the ordinary Gimbal command line. lint never reaches it, since main
// routes that command to the analyzer first.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string) error {
	return runWithUploaders(args, stdout, stderr, getenv, defaultArtifactUploaders())
}

func runWithUploaders(args []string, stdout, stderr io.Writer, getenv func(string) string, uploaders artifactUploaderConstructors) error {
	root := newRootCommand(stdout, stderr, getenv)
	root.AddCommand(newUploadArtifactCommand(stdout, getenv, uploaders))
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func newRootCommand(stdout, stderr io.Writer, getenv func(string) string) *cobra.Command {
	var server serverFlags
	root := &cobra.Command{
		Use:   "gimbal",
		Short: "Gimbal's project runtime and web application",
		Long: `Without a subcommand, gimbal serves the current repository on the web
application and waits. Repeat --project to admit multiple repositories to one
instance. Each repository keeps its runs and conversations under .gimbal.
An active project has one instance owner. A second instance refuses that
project, including a path alias; it can admit the project after the owner exits.
Separate instances can host different projects with distinct instance
directories and web endpoints.
gimbal lint [packages] checks workflow authoring rules, standalone or as a go
vet tool.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return serve(cmd.Context(), server)
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	server.bind(root.Flags())
	root.AddCommand(newRunCommand(), &cobra.Command{
		Use:                "run-prompt [flags] PROMPT",
		Short:              "Run one prompt in a separate headless runtime and print the answer",
		DisableFlagParsing: true,
		RunE:               func(_ *cobra.Command, args []string) error { return runPrompt(args, stdout, stderr, getenv) },
	})
	root.AddCommand(newRunsCommand(), newWatchCommand(), newSteerCommand(), newCountTokensCommand(), newOpenCodeCommand(stdout, getenv))
	return root
}

func newOpenCodeCommand(stdout io.Writer, getenv func(string) string) *cobra.Command {
	command := &cobra.Command{
		Use:   "opencode",
		Short: "Manage Gimbal's shared OpenCode server",
		Long: `Manage the shared OpenCode server used by opencode/MODEL and
opencode/PROVIDER/MODEL selections. State defaults to ~/.gimbal/opencode;
GIMBAL_OPENCODE_DIR changes that default. Raw adapter captures are written
under the selected state directory's captures/ subdirectory.`,
		Args: cobra.NoArgs,
	}
	defaultStateDir := strings.TrimSpace(getenv("GIMBAL_OPENCODE_DIR"))
	if defaultStateDir == "" && strings.TrimSpace(getenv("HOME")) != "" {
		defaultStateDir = filepath.Join(getenv("HOME"), ".gimbal", "opencode")
	}

	var startStateDir string
	start := &cobra.Command{
		Use:     "start",
		Short:   "Start the shared OpenCode server if it is not already running",
		Example: "  gimbal opencode start\n  gimbal opencode start --state-dir .gimbal/opencode",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, err := opencode.StartServer(cmd.Context(), startStateDir)
			if err != nil {
				return err
			}
			if info.Started {
				_, err = fmt.Fprintf(stdout, "OpenCode started at %s (pid %d)\n", info.URL, info.PID)
			} else {
				_, err = fmt.Fprintf(stdout, "OpenCode already running at %s (pid %d)\n", info.URL, info.PID)
			}
			return err
		},
	}
	start.Flags().StringVar(&startStateDir, "state-dir", defaultStateDir, "shared runtime state directory")

	var stopStateDir string
	stop := &cobra.Command{
		Use:     "stop",
		Short:   "Stop the shared OpenCode server and interrupt its active work",
		Example: "  gimbal opencode stop\n  gimbal opencode stop --state-dir .gimbal/opencode",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stopped, err := opencode.StopServer(cmd.Context(), stopStateDir)
			if err != nil {
				return err
			}
			if stopped {
				_, err = fmt.Fprintln(stdout, "OpenCode stopped")
			} else {
				_, err = fmt.Fprintln(stdout, "OpenCode is not running")
			}
			return err
		},
	}
	stop.Flags().StringVar(&stopStateDir, "state-dir", defaultStateDir, "shared runtime state directory")
	command.AddCommand(start, stop)
	return command
}

// serverFlags shape the web application of every command that runs one.
type serverFlags struct {
	port        int
	uds         string
	noWeb       bool
	projects    []string
	instanceDir string
}

func (f *serverFlags) bind(fs *pflag.FlagSet) {
	fs.IntVar(&f.port, "port", 8080, "loopback TCP port for the web application")
	fs.StringVar(&f.uds, "uds", "", "Unix-domain socket for the web application instead of TCP")
	fs.BoolVar(&f.noWeb, "no-web", false, "run without the web application")
	fs.StringArrayVar(&f.projects, "project", nil, "repository directory to admit (repeat for multiple projects; default: current directory)")
	fs.StringVar(&f.instanceDir, "instance-dir", ".gimbal", "state directory for this persistent instance (default .gimbal)")
}

func (f *serverFlags) options() []web.Option {
	switch {
	case f.noWeb:
		return []web.Option{web.WithNoWeb()}
	case f.uds != "":
		return []web.Option{web.WithUDS(f.uds)}
	default:
		return []web.Option{web.WithPort(f.port)}
	}
}

// serve runs one instance over the selected repositories until interrupted.
func serve(ctx context.Context, server serverFlags) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	options := server.options()
	projects := server.projects
	if len(projects) == 0 {
		projects = []string{"."}
	}
	instance, err := web.NewInstance(ctx, server.instanceDir, projects, options...)
	if err != nil {
		return err
	}
	<-ctx.Done()
	instance.Wait()
	return nil
}
