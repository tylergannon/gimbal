package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/binding"
	"github.com/tylergannon/gimble/web"

	// The workflows built into this binary. Each registers itself from the
	// init of the file gimble graph wrote in its package.
	_ "github.com/tylergannon/gimble/internal/workflows/easyloop"
	_ "github.com/tylergannon/gimble/internal/workflows/execute"
	_ "github.com/tylergannon/gimble/internal/workflows/plan"
	_ "github.com/tylergannon/gimble/internal/workflows/sprint"
)

func newLsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List the workflows built into this binary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			for _, r := range gimble.Registered() {
				_, _ = fmt.Fprintf(w, "%s\t%s\n", r.Name, r.Summary)
			}
			return w.Flush()
		},
	}
}

func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run <workflow>",
		Short: "Run a workflow built into this binary; gimble run <workflow> --help shows its flags",
	}
	for _, r := range gimble.Registered() {
		run.AddCommand(workflowCommand(r))
	}
	return run
}

// workflowCommand is the subcommand that runs one registered workflow. Its
// flags are read from the workflow itself: one per property of its input
// schema, one per role its graph names, for the model that role runs on,
// and the web application's own.
func workflowCommand(r gimble.Registration) *cobra.Command {
	var server serverFlags
	var model, repo string
	cmd := &cobra.Command{Use: r.Name, Short: r.Summary, Args: cobra.NoArgs}
	input, err := inputFlags(r.Input, cmd.Flags())
	if err != nil {
		panic(fmt.Sprintf("gimble run %s: %v", r.Name, err))
	}
	roles := r.Graph.Roles()
	specs := make(map[string]*string, len(roles))
	for _, role := range roles {
		if cmd.Flags().Lookup(role) != nil {
			panic(fmt.Sprintf("gimble run %s: the role %s is named like an input", r.Name, role))
		}
		specs[role] = cmd.Flags().String(role, "", "the model for role "+role+", as model or model:effort; --model when not given")
	}
	cmd.Flags().StringVar(&model, "model", "", "model for every role not given its own, as model or model:effort")
	cmd.Flags().StringVar(&repo, "repo", ".", "the repository the workflow runs in, whose .gimble holds its runs")
	server.bind(cmd.Flags())
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		dir, err := filepath.Abs(repo)
		if err != nil {
			return err
		}
		raw, err := input(cmd.Flags(), dir)
		if err != nil {
			return err
		}
		models := make(map[string]gimble.ModelBinding, len(roles))
		for _, role := range roles {
			spec := cmp.Or(*specs[role], model)
			if spec == "" {
				return fmt.Errorf("%s: give --%s or --model", r.Name, role)
			}
			bound, err := binding.Parse(spec)
			if err != nil {
				return fmt.Errorf("--%s: %w", role, err)
			}
			models[role] = bound
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		runtime, err := web.NewRuntime(ctx, filepath.Join(dir, ".gimble"), server.options()...)
		if err != nil {
			return err
		}
		return runtime.Run(ctx, r.Name, models, func(ctx context.Context) error {
			return r.Run(ctx, raw)
		})
	}
	return cmd
}

// property is one property of an input schema as a flag.
type property struct {
	name, flag, kind, description string
	required                      bool
}

// inputFlags declares one flag per property of an input schema on fs: a
// string, integer, or boolean, named as the property with its underscores
// as dashes, described by the property's description, and required where
// the schema requires the property, except that a boolean is never required
// to be given. The repo property is the run's own --repo. It returns the
// function that reads the flags back into the input's JSON, leaving out an
// optional flag that was not given so the input's Optional field stays
// absent.
func inputFlags(schema json.RawMessage, fs *pflag.FlagSet) (func(fs *pflag.FlagSet, repo string) (json.RawMessage, error), error) {
	if len(schema) == 0 {
		return func(*pflag.FlagSet, string) (json.RawMessage, error) { return nil, nil }, nil
	}
	properties, err := schemaProperties(schema)
	if err != nil {
		return nil, err
	}
	hasRepo := false
	for _, p := range properties {
		if p.name == "repo" {
			hasRepo = true
			continue
		}
		given := p.required && p.kind != "boolean"
		usage := p.description
		if given {
			usage += " (required)"
		}
		switch p.kind {
		case "string":
			fs.String(p.flag, "", usage)
		case "integer":
			fs.Int(p.flag, 0, usage)
		case "boolean":
			fs.Bool(p.flag, false, usage)
		default:
			return nil, fmt.Errorf("input %s has type %s, which is not a flag", p.name, p.kind)
		}
		if given {
			_ = cobra.MarkFlagRequired(fs, p.flag)
		}
	}
	return func(fs *pflag.FlagSet, repo string) (json.RawMessage, error) {
		input := map[string]any{}
		if hasRepo {
			input["repo"] = repo
		}
		for _, p := range properties {
			if p.name == "repo" || (!p.required && !fs.Changed(p.flag)) {
				continue
			}
			var err error
			switch p.kind {
			case "string":
				input[p.name], err = fs.GetString(p.flag)
			case "integer":
				input[p.name], err = fs.GetInt(p.flag)
			case "boolean":
				input[p.name], err = fs.GetBool(p.flag)
			}
			if err != nil {
				return nil, err
			}
		}
		return json.Marshal(input)
	}, nil
}

// schemaProperties reads an object schema's properties in the order they are
// written, which is the input struct's field order.
func schemaProperties(schema json.RawMessage) ([]property, error) {
	var object struct {
		Properties json.RawMessage `json:"properties"`
		Required   []string        `json:"required"`
	}
	if err := json.Unmarshal(schema, &object); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(object.Properties))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var properties []property
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var p struct {
			Type        any    `json:"type"`
			Description string `json:"description"`
		}
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		name, _ := key.(string)
		properties = append(properties, property{
			name:        name,
			flag:        strings.ReplaceAll(name, "_", "-"),
			kind:        kindOf(p.Type),
			description: p.Description,
			required:    slices.Contains(object.Required, name),
		})
	}
	return properties, nil
}

// kindOf is a schema's type, which polytype writes as one name, or as a list
// with null for a value that may be null.
func kindOf(t any) string {
	switch t := t.(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" {
				return s
			}
		}
	}
	return fmt.Sprint(t)
}
