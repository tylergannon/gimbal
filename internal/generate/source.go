package generate

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tylergannon/gimble/workflow"
)

// Source writes the workflow entry's graph and Gimble run subcommand into one
// Go file. The graph is registered from init; the command has Gimble's
// environment flags, a flag for each input field, and a model flag for each
// role the entry names.
//
// The file is replaced by an empty package clause while the package is
// read, so one that no longer compiles against the source as it now stands
// does not stop the next generation.
func Source(dir, entry, name, output string) error {
	file := output
	if !filepath.IsAbs(file) {
		file = filepath.Join(dir, file)
	}
	file, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	pkg, err := packageName(dir, file, name)
	if err != nil {
		return err
	}
	graph, info, err := extract(dir, entry, name, map[string][]byte{file: []byte("package " + pkg + "\n")})
	if err != nil {
		return err
	}
	if err := check(info, graph); err != nil {
		return err
	}
	text, err := format.Source([]byte(source(pkg, entry, info, graph)))
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if err := os.WriteFile(file, text, 0o644); err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	return nil
}

// packageName is the package clause of the directory the graph is written
// into, read from a file other than the one being written.
func packageName(dir, output, fallback string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if abs, err := filepath.Abs(path); err == nil && abs == output {
			continue
		}
		parsed, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}
		return parsed.Name.Name, nil
	}
	return fallback, nil
}

// literal is the graph as a Go literal.
func literal(entry string, graph workflow.Graph) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// Graph is the shape of this workflow, read from the source of %s.\n", entry)
	b.WriteString("var Graph = workflow.Graph{\n")
	fmt.Fprintf(&b, "Name: %s,\n", strconv.Quote(graph.Name))
	fmt.Fprintf(&b, "Source: %s,\n", position(graph.Source))
	fmt.Fprintf(&b, "Body: %s,\n", operations(graph.Body))
	if len(graph.Diagnostics) > 0 {
		b.WriteString("Diagnostics: []workflow.Diagnostic{\n")
		for _, d := range graph.Diagnostics {
			fmt.Fprintf(&b, "{Source: %s, Message: %s},\n", position(d.Source), strconv.Quote(d.Message))
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func position(source workflow.Source) string {
	return fmt.Sprintf("workflow.Source{File: %s, Line: %d}", strconv.Quote(source.File), source.Line)
}

func operations(ops []workflow.Operation) string {
	var b strings.Builder
	b.WriteString("[]workflow.Operation{\n")
	for _, op := range ops {
		b.WriteString(operation(op))
		b.WriteString(",\n")
	}
	b.WriteString("}")
	return b.String()
}

func operation(op workflow.Operation) string {
	var b strings.Builder
	switch op := op.(type) {
	case workflow.Session:
		fmt.Fprintf(&b, "workflow.Session{Source: %s, Name: %s, From: %s}", position(op.Source), strconv.Quote(op.Name), strconv.Quote(op.From))
	case workflow.AgentCall:
		fmt.Fprintf(&b, "workflow.AgentCall{Source: %s, Session: %s, Role: %s, Prompt: %s", position(op.Source), strconv.Quote(op.Session), strconv.Quote(op.Role), strconv.Quote(op.Prompt))
		if len(op.Supervisors) > 0 {
			fmt.Fprintf(&b, ", Supervisors: %s", supervisors(op.Supervisors))
		}
		b.WriteString("}")
	case workflow.Command:
		fmt.Fprintf(&b, "workflow.Command{Source: %s, Name: %s}", position(op.Source), strconv.Quote(op.Name))
	case workflow.Set:
		fmt.Fprintf(&b, "workflow.Set{Source: %s, Key: %s}", position(op.Source), strconv.Quote(op.Key))
	case workflow.Scope:
		fmt.Fprintf(&b, "workflow.Scope{Source: %s, Name: %s, Body: %s}", position(op.Source), strconv.Quote(op.Name), operations(op.Body))
	case workflow.Loop:
		fmt.Fprintf(&b, "workflow.Loop{Source: %s, Name: %s, Planner: %s, Body: %s}", position(op.Source), strconv.Quote(op.Name), strconv.Quote(op.Planner), operations(op.Body))
	case workflow.Repeat:
		fmt.Fprintf(&b, "workflow.Repeat{Source: %s, Cond: %s, Body: %s}", position(op.Source), strconv.Quote(op.Cond), operations(op.Body))
	case workflow.Group:
		fmt.Fprintf(&b, "workflow.Group{Source: %s, Name: %s, Children: []workflow.GroupChild{\n", position(op.Source), strconv.Quote(op.Name))
		for _, child := range op.Children {
			fmt.Fprintf(&b, "{Source: %s, Name: %s, Body: %s},\n", position(child.Source), strconv.Quote(child.Name), operations(child.Body))
		}
		b.WriteString("}}")
	case workflow.Condition:
		fmt.Fprintf(&b, "workflow.Condition{Source: %s, Branches: []workflow.Branch{\n", position(op.Source))
		for _, br := range op.Branches {
			fmt.Fprintf(&b, "{Source: %s, Case: %s, Exits: %t, Body: %s},\n", position(br.Source), strconv.Quote(br.Case), br.Exits, operations(br.Body))
		}
		b.WriteString("}}")
	}
	return b.String()
}

func supervisors(watching []workflow.Supervisor) string {
	var b strings.Builder
	b.WriteString("[]workflow.Supervisor{\n")
	for _, s := range watching {
		fmt.Fprintf(&b, "{Source: %s, Session: %s, Role: %s, Instruction: %s", position(s.Source), strconv.Quote(s.Session), strconv.Quote(s.Role), strconv.Quote(s.Instruction))
		if len(s.Supervisors) > 0 {
			fmt.Fprintf(&b, ", Supervisors: %s", supervisors(s.Supervisors))
		}
		b.WriteString("},\n")
	}
	b.WriteString("}")
	return b.String()
}
