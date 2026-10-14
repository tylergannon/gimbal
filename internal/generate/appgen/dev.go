package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/internal/generate"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/polytype/codegen"
	"github.com/tylergannon/skgo/dev"
	skgo "github.com/tylergannon/skgo/generate"
)

func development(args []string) error {
	flags := flag.NewFlagSet("dev", flag.ContinueOnError)
	port := flags.String("port", envDefault("GIMBAL_DEV_PORT", "8080"), "public application port")
	vitePort := flags.String("vite-port", envDefault("GIMBAL_DEV_VITE_PORT", "5173"), "Vite port")
	instance := flags.String("instance-dir", envDefault("GIMBAL_INSTANCE_DIR", ".gimbal"), "application catalog directory")
	noProject := flags.Bool("no-project", false, "start with no implicitly admitted project")
	debug := flags.Bool("debug", false, "retain Go debugger information in development builds")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, err := developmentGeneration(root)
	if err != nil {
		return err
	}
	restartFiles, err := generatorFiles(root)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Workflow codecs are part of the compiled generator model. Preserve the
	// ordinary startup regeneration; subsequent TS work belongs to its Worker.
	initial := exec.CommandContext(ctx, "go", "tool", "polytype", "--typescript", "../web/src/lib/workflow")
	initial.Dir = filepath.Join(root, "workflow")
	if err := regenerateCompiledWorkflow(filepath.Join(root, "workflow/polytype_gen.go"), func() error {
		if output, err := initial.CombinedOutput(); err != nil {
			return fmt.Errorf("workflow generation: %w\n%s", err, output)
		}
		return nil
	}); err != nil {
		return err
	}
	var workflow codegen.Worker
	defer func() { _ = workflow.Close() }()
	var observation codegen.Worker
	defer func() { _ = observation.Close() }()
	var worker skgo.Worker
	defer func() { _ = worker.Close() }()
	project := func(ctx context.Context) ([]byte, error) {
		started := time.Now()
		last := started
		var timings []string
		mark := func(name string) {
			now := time.Now()
			timings = append(timings, name+"="+now.Sub(last).Round(time.Millisecond).String())
			last = now
		}
		defer func() {
			if os.Getenv("GIMBAL_DEV_TRACE") != "" {
				fmt.Fprintf(os.Stderr, "gimbal dev generation: %v total=%s skgo=%+v observation=%+v workflow=%+v\n", timings, time.Since(started).Round(time.Millisecond), worker.Stats(), observation.Stats(), workflow.Stats())
			}
		}()

		current, err := developmentGeneration(root)
		if err != nil {
			return nil, err
		}
		cfg = current
		if err := generate.Stock(root, nil); err != nil {
			return nil, err
		}
		mark("stock")
		if err := worker.Run(cfg); err != nil {
			return nil, err
		}
		mark("skgo")
		if err := observation.Generate(observationConfiguration(), codegen.Options{TargetDir: filepath.Join(root, "internal/observation"), TypeScript: &codegen.TypeScriptOptions{Dir: filepath.Join(root, "web/src/lib/observation")}}); err != nil {
			return nil, err
		}
		mark("observation")
		err = workflow.Generate(workflowConfiguration(), codegen.Options{
			TargetDir:  filepath.Join(root, "workflow"),
			TypeScript: &codegen.TypeScriptOptions{Dir: filepath.Join(root, "web/src/lib/workflow")},
		})
		mark("workflow")
		return nil, err
	}
	origin := "http://127.0.0.1:" + *port
	vite, err := url.Parse("http://127.0.0.1:" + *vitePort)
	if err != nil {
		return err
	}
	// The Go binary embeds the production manifest even when serving Vite.
	// A fresh checkout therefore needs the same real frontend build once.
	if _, err := os.Stat(filepath.Join(cfg.Web, "build/skgo.manifest.json")); os.IsNotExist(err) {
		if output, err := project(ctx); err != nil {
			return fmt.Errorf("initial generation: %w\n%s", err, output)
		}
		cmd := exec.CommandContext(ctx, "vp", "build")
		cmd.Dir = cfg.Web
		cmd.Env = append(os.Environ(), "ORIGIN="+origin)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return dev.Run(ctx, dev.Config{
		Root: root, Web: cfg.Web, Out: cfg.Out, Package: "./cmd/gimbal",
		Listen: "127.0.0.1:" + *port, Vite: vite,
		ViteCommand: []string{"vp", "dev", "--host", "127.0.0.1", "--port", *vitePort, "--strictPort"},
		ViteEnv:     []string{"ORIGIN=" + origin},
		Env:         []string{"GIMBAL_WEB_PROXY=" + vite.String(), "GIMBAL_WEB_ORIGIN=" + origin},
		Args: func(listen string) []string {
			_, privatePort, _ := net.SplitHostPort(listen)
			args := []string{"--port", privatePort, "--instance-dir", *instance}
			if *noProject {
				args = append(args, "--no-project")
			}
			return args
		},
		Generate: project, RestartFiles: restartFiles, StopBeforeStart: true,
		Debug:      *debug,
		ExtraWatch: []string{filepath.Join(root, "native/skgo-native.json")},
		WatchInputs: func() dev.WatchInputs {
			inputs := worker.Inputs()
			return dev.WatchInputs{Roots: inputs.Roots, Files: inputs.Files, EmbedPatterns: inputs.EmbedPatterns}
		},
	})
}

// Startup generation can repair a codec compiled into this process. Never
// fingerprint the repaired source and then keep executing its old compiled form.
func regenerateCompiledWorkflow(path string, regenerate func() error) error {
	before, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := regenerate(); err != nil {
		return err
	}
	after, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("workflow codecs changed during startup; restart just dev to compile the regenerated %s", path)
	}
	return nil
}

func developmentGeneration(root string) (skgo.Config, error) {
	data, err := os.ReadFile(filepath.Join(root, "native/skgo-native.json"))
	if err != nil {
		return skgo.Config{}, err
	}
	var native struct {
		SwiftRemotes []string `json:"swiftRemotes"`
	}
	if err := json.Unmarshal(data, &native); err != nil {
		return skgo.Config{}, err
	}
	return skgo.Config{
		Web: filepath.Join(root, "web"), Out: filepath.Join(root, "internal/skgo"),
		LocalsPackage: "github.com/tylergannon/gimbal/internal/app",
		HookPackage:   "github.com/tylergannon/gimbal/internal/serverhooks",
		SwiftOut:      filepath.Join(root, "native/Generated.swift"), SwiftRemotes: native.SwiftRemotes,
	}, nil
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func workflowConfiguration() polytype.Configuration {
	return polytype.DeclareType(polytype.TypeSpec{PackagePath: "github.com/tylergannon/gimbal/workflow", Name: "Graph"})
}
