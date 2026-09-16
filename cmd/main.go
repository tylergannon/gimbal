// Command gimble runs Gimble's project runtime and web application.
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
	"strings"

	"github.com/tylergannon/gimble/internal/gimblelint"
	"github.com/tylergannon/gimble/web"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	if analysisArgs, ok := routeAnalysis(os.Args[1:]); ok {
		os.Args = append([]string{os.Args[0]}, analysisArgs...)
		singlechecker.Main(gimblelint.Analyzer)
		return
	}
	if err := run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		_, _ = fmt.Fprintln(os.Stderr, "gimble:", err)
		os.Exit(1)
	}
}

// routeAnalysis recognizes the standalone lint command and the three calls in
// the go vet tool protocol before the ordinary Gimble CLI parses its flags.
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
	if len(args) > 0 && (args[0] == "run-prompt" || args[0] == "graph") {
		return true
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

func run(args []string, stdout, stderr io.Writer, getenv func(string) string) error {
	if len(args) > 0 && args[0] == "run-prompt" {
		return runPrompt(args[1:], stdout, stderr, getenv)
	}
	if len(args) > 0 && args[0] == "graph" {
		return runGraph(args[1:], stderr)
	}
	return runServer(args, stderr)
}

func runServer(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("gimble", flag.ContinueOnError)
	flags.SetOutput(stderr)
	port := flags.Int("port", 8080, "loopback TCP port for the web application")
	uds := flags.String("uds", "", "Unix-domain socket for the web application instead of TCP")
	noWeb := flags.Bool("no-web", false, "run without the web application")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var options []web.Option
	switch {
	case *noWeb:
		options = append(options, web.WithNoWeb())
	case *uds != "":
		options = append(options, web.WithUDS(*uds))
	default:
		options = append(options, web.WithPort(*port))
	}
	if _, err := web.NewRuntime(ctx, ".gimble", options...); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
