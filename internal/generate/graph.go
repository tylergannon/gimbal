// Package generate reads a workflow's graph and command metadata out of its Go
// source, then emits the generated command and graph. What it cannot read it
// records as a diagnostic; it never guesses.
package generate

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"

	"github.com/tylergannon/gimbal/internal/gimballint"
	"github.com/tylergannon/gimbal/workflow"
	"golang.org/x/tools/go/packages"
)

const gimbalPath = "github.com/tylergannon/gimbal"

// Extract loads the package in dir and returns the graph of the workflow its
// function entry writes, under the name name. It returns the live Go graph,
// preserving nil distinctions that the generated JSON wire format normalizes.
func Extract(dir, entry, name string) (workflow.Graph, error) {
	graph, _, err := extractInspected(dir, entry, name, nil, true, nil)
	return graph, err
}

func extract(dir, entry, name string, overlay map[string][]byte) (workflow.Graph, entryInfo, error) {
	return extractSource(dir, entry, name, overlay, true)
}

func extractSource(dir, entry, name string, overlay map[string][]byte, commandMetadata bool) (workflow.Graph, entryInfo, error) {
	result, err := analyzeOne(dir, entry, name, overlay, commandMetadata, false)
	return result.Graph, result.Entry.entryInfo(), err
}

func extractInspected(dir, entry, name string, overlay map[string][]byte, commandMetadata bool, inspection *inspectionData) (workflow.Graph, entryInfo, error) {
	loaded, err := loadSource(dir, overlay, ".")
	if err != nil {
		return workflow.Graph{}, entryInfo{}, err
	}
	if len(loaded.roots) != 1 {
		return workflow.Graph{}, entryInfo{}, fmt.Errorf("generate: %s holds %d packages", dir, len(loaded.roots))
	}
	return extractLoaded(loaded.roots[0], loaded.imports, entry, name, commandMetadata, inspection)
}

// Walk state belongs to one extraction; package syntax and types belong to the
// source snapshot and may serve several entries or projections in this run.
func extractLoaded(pkg *packages.Package, imports map[string]bool, entry, name string, commandMetadata bool, inspection *inspectionData) (workflow.Graph, entryInfo, error) {
	if len(pkg.Errors) > 0 {
		return workflow.Graph{}, entryInfo{}, fmt.Errorf("generate: %s: %v", pkg.PkgPath, pkg.Errors[0])
	}
	decl := findFunc(pkg, entry)
	if decl == nil || decl.Body == nil {
		return workflow.Graph{}, entryInfo{}, fmt.Errorf("generate: %s has no function %s with a body", pkg.PkgPath, entry)
	}
	if err := gimballint.Admit(pkg, entry); err != nil {
		return workflow.Graph{}, entryInfo{}, err
	}
	var info entryInfo
	var err error
	if commandMetadata {
		info, err = describe(pkg, decl)
		if err != nil {
			return workflow.Graph{}, entryInfo{}, err
		}
	}
	module, modulePath := "", ""
	if pkg.Module != nil {
		module, modulePath = pkg.Module.Dir, pkg.Module.Path
	}
	e := &extractor{
		sourceImports: imports,
		inspection:    inspection,
		declarations:  make(map[types.Object]valueDeclarations),
		pkg:           pkg,
		module:        module,
		modulePath:    modulePath,
		session:       make(map[types.Object]binding),
		group:         make(map[types.Object]*nodeRef),
		loop:          make(map[types.Object]*nodeRef),
		hasOp:         make(map[*types.Func]bool),
		dead:          make(map[types.Object]bool),
	}
	if inspection != nil {
		inspection.Module = module
		e.inspectDiagramComments()
		e.inspectionContext = e.contextParameter(decl.Type)
	}

	// The entry parameter object identifies direct selectors, never aliases or expressions.
	if fn, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func); ok {
		params := fn.Type().(*types.Signature).Params()
		if params.Len() == 3 {
			e.input = params.At(2)
			if details, err := describe(pkg, decl); err == nil {
				e.inputFields = make(map[string]string)
				for _, f := range details.fields {
					if f.kind == "string" && !f.optional {
						e.inputFields[f.name] = f.json
					}
				}
			}
		}
	}

	if e.input != nil {
		ast.Inspect(decl.Body, func(n ast.Node) bool {
			var targets []ast.Expr
			switch x := n.(type) {
			case *ast.AssignStmt:
				targets = x.Lhs
			case *ast.IncDecStmt:
				targets = []ast.Expr{x.X}
			case *ast.CallExpr:
				if selector, ok := unparen(x.Fun).(*ast.SelectorExpr); ok && e.object(selector.X) == e.input {
					e.inputMutated = true
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					targets = []ast.Expr{x.X}
				}
			}
			for _, target := range targets {
				if selector, ok := unparen(target).(*ast.SelectorExpr); ok {
					target = selector.X
				}
				if e.object(target) == e.input {
					e.inputMutated = true
				}
			}
			return true
		})
	}
	body := []workflow.Operation{}
	services := []workflow.Service{}
	e.services = &services
	e.block(decl.Body.List, &body, scopeEnv{blockTail: true, callTail: true})
	// A call inside syntax the graph walker could not follow must not disappear
	// from the inventory and become a silently executable selection.
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := e.gimbalCall(call)
		if !ok || name != "WithEnvironment" {
			return true
		}
		source := e.at(call.Pos())
		for _, d := range e.environments {
			if d.Source == source {
				return true
			}
		}
		e.environments = append(e.environments, workflow.EnvironmentDeclaration{Source: source})
		return true
	})
	return workflow.Graph{
		Name:         name,
		Environments: e.environments,
		Source:       e.at(decl.Pos()),
		Services:     services,
		Body:         body,
		Diagnostics:  e.diagnostics,
	}, info, nil
}

// binding is what an identifier of session type stands for: the name the
// runtime gives that conversation and the role it runs as.
type binding struct{ name, role string }

// nodeRef points at a node already placed in a body, so a later call can add
// to it: a Group's children, a PromiseLoop's per-task body.
type nodeRef struct {
	ops          *[]workflow.Operation
	index        int
	declarations valueDeclarations
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
	input             types.Object
	inputFields       map[string]string
	inputMutated      bool
	environments      []workflow.EnvironmentDeclaration
	sourceImports     map[string]bool
	declarations      map[types.Object]valueDeclarations
	inspection        *inspectionData
	inspectionContext types.Object
	pkg               *packages.Package
	module            string
	modulePath        string
	diagnostics       []workflow.Diagnostic
	session           map[types.Object]binding
	group             map[types.Object]*nodeRef
	loop              map[types.Object]*nodeRef
	// dead holds every identifier a reassignment made unreadable; a scoped
	// walk's restore does not bring one back.
	dead     map[types.Object]bool
	stack    []*types.Func
	helpers  []*helperState
	hasOp    map[*types.Func]bool
	walking  map[*types.Func]bool
	services *[]workflow.Service
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
	e.emittedShape()
}

func (e *extractor) emitService(service workflow.Service) {
	*e.services = append(*e.services, service)
	e.emittedShape()
}

func (e *extractor) emittedShape() {
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

// ownedBody walks a runtime scope body with a fresh service collection. The
// caller stores both the ordered operations and the declarations its scope
// owns.
func (e *extractor) ownedBody(stmts []ast.Stmt, en scopeEnv) ([]workflow.Operation, []workflow.Service) {
	body := []workflow.Operation{}
	services := []workflow.Service{}
	previous := e.services
	e.services = &services
	e.body(stmts, &body, en)
	e.services = previous
	return body, services
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
	declarations := maps.Clone(e.declarations)
	walk()
	e.session, e.group, e.loop = session, group, loop
	e.declarations = declarations
	for obj := range e.dead {
		delete(e.session, obj)
		delete(e.group, obj)
		delete(e.loop, obj)
	}
}
