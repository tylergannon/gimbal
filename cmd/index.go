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
	"slices"
	"strings"

	"github.com/tylergannon/gimble/internal/workflows/semanticindex"
	"github.com/tylergannon/gimble/web"
)

func runIndex(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gimble index", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var in semanticindex.Input
	flags.StringVar(&in.Source, "from", "", "local source cache to read")
	flags.StringVar(&in.Output, "to", "", "semantic index directory, outside the source cache")
	flags.StringVar(&in.Model, "model", "gpt-5.6-luna", "Codex model for index readers and routing")
	flags.StringVar(&in.Mode, "mode", "auto", "auto, build, update, or audit")
	flags.BoolVar(&in.DryRun, "dry-run", false, "preview without model calls or changing the index")
	repo := flags.String("repo", ".", "planning project to link to the completed index")
	noWeb := flags.Bool("no-web", false, "omit the live web server")
	port := flags.Int("port", 0, "web server port; 0 chooses a free port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("use -from and -to to name the source cache and index")
	}
	if in.Source == "" || in.Output == "" {
		return errors.New("index requires -from and -to")
	}
	switch in.Mode {
	case "auto", "build", "update", "audit":
	default:
		return errors.New("index mode must be auto, build, update, or audit")
	}
	var err error
	in.Source, err = filepath.Abs(in.Source)
	if err != nil {
		return err
	}
	in.Source, err = filepath.EvalSymlinks(in.Source)
	if err != nil {
		return err
	}
	info, err := os.Stat(in.Source)
	if err != nil || !info.IsDir() {
		return errors.New("index source must be an existing directory")
	}
	in.Output, err = futurePath(in.Output)
	if err != nil {
		return err
	}
	if pathWithin(in.Source, in.Output) || pathWithin(in.Output, in.Source) {
		return errors.New("source cache and index must not overlap")
	}
	project, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return err
	}
	info, err = os.Stat(project)
	if err != nil || !info.IsDir() {
		return errors.New("planning project must be an existing directory")
	}
	// Put runtime data alongside the index, never in its read-only source cache.
	if err := os.MkdirAll(filepath.Dir(in.Output), 0o755); err != nil {
		return err
	}
	records, err := os.MkdirTemp(filepath.Dir(in.Output), ".gimble-index-")
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(records, "request.json"), data, 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Index run records:", records)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	option := web.WithPort(*port)
	if *noWeb || in.DryRun {
		option = web.WithNoWeb()
	}
	runtime, err := web.NewRuntime(ctx, records, option)
	if err != nil {
		return err
	}
	var entrypoint string
	if err := runtime.Run(ctx, "semantic index", func(ctx context.Context) error {
		var err error
		entrypoint, err = semanticindex.Build(ctx, in, stdout)
		return err
	}); err != nil {
		return err
	}
	if in.DryRun || in.Mode == "audit" {
		return nil
	}
	_, _ = fmt.Fprintln(stdout, "Semantic index:", entrypoint)
	config := filepath.Join(project, ".gimble", "semantic-index.json")
	config, err = futurePath(config)
	if err != nil {
		return err
	}
	if pathWithin(in.Source, config) || pathWithin(in.Output, config) {
		_, _ = fmt.Fprintln(stdout, "Use gimble plan -semantic-index", entrypoint, "-token-cache", in.Source, "to use this index without writing into its source or output.")
		return nil
	}
	data, err = json.MarshalIndent(struct {
		Source     string `json:"source"`
		Entrypoint string `json:"entrypoint"`
	}{in.Source, entrypoint}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(config, data, 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Sprint Plan will discover this index through", config)
	return nil
}

// Resolve the existing prefix, including symlinks, before comparing future paths.
func futurePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		_, err := os.Lstat(abs)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(abs)
			if err != nil {
				return "", err
			}
			for _, s := range slices.Backward(suffix) {
				resolved = filepath.Join(resolved, s)
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		suffix = append(suffix, filepath.Base(abs))
		abs = filepath.Dir(abs)
	}
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
