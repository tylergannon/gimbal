// Package graph reads a workflow's graph out of its Go source: the scopes,
// sessions, agent calls, commands, and control flow the source writes, before
// anything runs. What it cannot read it records as a diagnostic; it never
// guesses.
package graph

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"

	"github.com/tylergannon/gimble/workflow"
	"golang.org/x/tools/go/packages"
)

const gimblePath = "github.com/tylergannon/gimble"

// Extract loads the package in dir and returns the graph of the workflow its
// function entry writes, under the name name.
func Extract(dir, entry, name string) (workflow.Graph, error) {
	return extract(dir, entry, name, nil)
}

func extract(dir, entry, name string, overlay map[string][]byte) (workflow.Graph, error) {
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:     dir,
		Overlay: overlay,
	}
	loaded, err := packages.Load(config, ".")
	if err != nil {
		return workflow.Graph{}, fmt.Errorf("graph: %w", err)
	}
	if len(loaded) != 1 {
		return workflow.Graph{}, fmt.Errorf("graph: %s holds %d packages", dir, len(loaded))
	}
	pkg := loaded[0]
	if len(pkg.Errors) > 0 {
		return workflow.Graph{}, fmt.Errorf("graph: %s: %v", pkg.PkgPath, pkg.Errors[0])
	}
	decl := findFunc(pkg, entry)
	if decl == nil || decl.Body == nil {
		return workflow.Graph{}, fmt.Errorf("graph: %s has no function %s with a body", pkg.PkgPath, entry)
	}
	module, modulePath := "", ""
	if pkg.Module != nil {
		module, modulePath = pkg.Module.Dir, pkg.Module.Path
	}
	e := &extractor{
		pkg:        pkg,
		module:     module,
		modulePath: modulePath,
		session:    make(map[types.Object]binding),
		group:      make(map[types.Object]*nodeRef),
		loop:       make(map[types.Object]*nodeRef),
		hasOp:      make(map[*types.Func]bool),
		dead:       make(map[types.Object]bool),
	}
	body := []workflow.Operation{}
	e.block(decl.Body.List, &body, scopeEnv{blockTail: true, callTail: true})
	return workflow.Graph{
		Name:        name,
		Source:      e.at(decl.Pos()),
		Body:        body,
		Diagnostics: e.diagnostics,
	}, nil
}

// binding is what an identifier of session type stands for: the name the
// runtime gives that conversation and the role it runs as.
type binding struct{ name, role string }

// nodeRef points at a node already placed in a body, so a later call can add
// to it: a Group's children, a Loop's per-task body.
type nodeRef struct {
	ops   *[]workflow.Operation
	index int
}

// scopeEnv is what the walk knows about where it is. blockTail says a return
// here ends the current body; callTail says the inlined helper this walk is
// inside was called in tail position; inHelper says it is inside one at all.
type scopeEnv struct {
	blockTail bool
	callTail  bool
	inHelper  bool
}

type helperState struct{ emitted bool }

type extractor struct {
	pkg         *packages.Package
	module      string
	modulePath  string
	diagnostics []workflow.Diagnostic
	session     map[types.Object]binding
	group       map[types.Object]*nodeRef
	loop        map[types.Object]*nodeRef
	// dead holds every identifier a reassignment made unreadable; a scoped
	// walk's restore does not bring one back.
	dead    map[types.Object]bool
	stack   []*types.Func
	helpers []*helperState
	hasOp   map[*types.Func]bool
	walking map[*types.Func]bool
}

func findFunc(pkg *packages.Package, name string) *ast.FuncDecl {
	for _, file := range pkg.Syntax {
		for _, d := range file.Decls {
			if decl, ok := d.(*ast.FuncDecl); ok && decl.Recv == nil && decl.Name.Name == name {
				return decl
			}
		}
	}
	return nil
}

// at is where a node is written, relative to the module root.
func (e *extractor) at(pos token.Pos) workflow.Source {
	position := e.pkg.Fset.Position(pos)
	file := position.Filename
	if e.module != "" {
		if rel, err := filepath.Rel(e.module, file); err == nil {
			file = rel
		}
	}
	return workflow.Source{File: filepath.ToSlash(file), Line: position.Line}
}

func (e *extractor) diag(pos token.Pos, format string, args ...any) {
	e.diagnostics = append(e.diagnostics, workflow.Diagnostic{
		Source:  e.at(pos),
		Message: fmt.Sprintf(format, args...),
	})
}

// emit places one operation in a body and records that every helper the walk
// is inside has now produced shape.
func (e *extractor) emit(out *[]workflow.Operation, op workflow.Operation) {
	*out = append(*out, op)
	for _, h := range e.helpers {
		h.emitted = true
	}
}

func (e *extractor) emitted() bool {
	return len(e.helpers) > 0 && e.helpers[len(e.helpers)-1].emitted
}

// block walks the statements of one block in order. A statement is in tail
// position when the block is and no later statement in it holds an operation.
func (e *extractor) block(stmts []ast.Stmt, out *[]workflow.Operation, en scopeEnv) {
	for i, stmt := range stmts {
		here := en
		here.blockTail = en.blockTail && !e.laterHoldsOperation(stmts[i+1:])
		e.stmt(stmt, out, here)
	}
}

func (e *extractor) laterHoldsOperation(stmts []ast.Stmt) bool {
	return slices.ContainsFunc(stmts, func(s ast.Stmt) bool { return e.holdsOperation(s) })
}

// body walks a Tasks range or a Repeat. A return there ends that body, so
// tail position starts again.
func (e *extractor) body(stmts []ast.Stmt, out *[]workflow.Operation, en scopeEnv) {
	e.scoped(func() {
		e.block(stmts, out, scopeEnv{blockTail: true, callTail: true, inHelper: en.inHelper})
	})
}

// callbackBody walks a Scope or Go callback, which is a function of its own:
// a return in it ends the callback, never the helper the callback is
// written in.
func (e *extractor) callbackBody(stmts []ast.Stmt, out *[]workflow.Operation) {
	e.scoped(func() {
		e.block(stmts, out, scopeEnv{blockTail: true, callTail: true})
	})
}

// scoped walks a block with its own bindings: what is declared inside it is
// gone at the join, so a use after the join is not a session, group, or loop
// declared in an enclosing body. What the block reassigned stays unread
// after it, since the block ran.
func (e *extractor) scoped(walk func()) {
	session, group, loop := maps.Clone(e.session), maps.Clone(e.group), maps.Clone(e.loop)
	walk()
	e.session, e.group, e.loop = session, group, loop
	for obj := range e.dead {
		delete(e.session, obj)
		delete(e.group, obj)
		delete(e.loop, obj)
	}
}
