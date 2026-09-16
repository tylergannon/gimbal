// Package graph reads a workflow's graph out of its Go source: the scopes,
// sessions, agent calls, commands, and control flow the source writes, before
// anything runs. What it cannot read it records as a diagnostic; it never
// guesses.
package graph

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/tylergannon/gimble/workflow"
	"golang.org/x/tools/go/packages"
)

const gimblePath = "github.com/tylergannon/gimble"

// Extract loads the package in dir and returns the graph of the workflow its
// function entry writes, under the name name.
func Extract(dir, entry, name string) (workflow.Graph, error) {
	graph, _, err := extract(dir, entry, name, nil)
	return graph, err
}

func extract(dir, entry, name string, overlay map[string][]byte) (workflow.Graph, entryInfo, error) {
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:     dir,
		Overlay: overlay,
	}
	loaded, err := packages.Load(config, ".")
	if err != nil {
		return workflow.Graph{}, entryInfo{}, fmt.Errorf("graph: %w", err)
	}
	if len(loaded) != 1 {
		return workflow.Graph{}, entryInfo{}, fmt.Errorf("graph: %s holds %d packages", dir, len(loaded))
	}
	pkg := loaded[0]
	if len(pkg.Errors) > 0 {
		return workflow.Graph{}, entryInfo{}, fmt.Errorf("graph: %s: %v", pkg.PkgPath, pkg.Errors[0])
	}
	decl := findFunc(pkg, entry)
	if decl == nil || decl.Body == nil {
		return workflow.Graph{}, entryInfo{}, fmt.Errorf("graph: %s has no function %s with a body", pkg.PkgPath, entry)
	}
	info, err := describe(pkg, decl)
	if err != nil {
		return workflow.Graph{}, entryInfo{}, err
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
	}, info, nil
}

// entryInfo is what the generated command needs about the entry besides its
// body: its input type, empty for an entry that takes only a ctx, and that
// type's fields; the first sentence of its doc comment; and the package's.
type entryInfo struct {
	input   string
	summary string
	long    string
	fields  []field
}

// field is one field of the input type as a flag: its Go name, the flag's
// name, whether the value is a string, an int, or a bool, whether the field
// is a polytype.Optional the flag sets only when given, and its doc.
type field struct {
	name, flag, kind, doc string
	optional              bool
}

// describe reads the entry's signature, its doc, and its input's fields. An
// entry takes a ctx and at most one input, a struct of its own package.
func describe(pkg *packages.Package, decl *ast.FuncDecl) (entryInfo, error) {
	info := entryInfo{summary: (&doc.Package{}).Synopsis(decl.Doc.Text()), long: packageDoc(pkg)}
	fn, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok {
		return info, fmt.Errorf("graph: %s has no type", decl.Name.Name)
	}
	params := fn.Type().(*types.Signature).Params()
	switch params.Len() {
	case 1:
		return info, nil
	case 2:
		named, ok := params.At(1).Type().(*types.Named)
		if !ok || named.Obj().Pkg() != pkg.Types {
			return info, fmt.Errorf("graph: %s's input is %s, not a type of its own package", decl.Name.Name, params.At(1).Type())
		}
		info.input = named.Obj().Name()
		fields, err := inputFields(pkg, named.Obj().Name())
		if err != nil {
			return info, err
		}
		info.fields = fields
		return info, nil
	default:
		return info, fmt.Errorf("graph: %s takes %d parameters; an entry takes a ctx and at most one input", decl.Name.Name, params.Len())
	}
}

func packageDoc(pkg *packages.Package) string {
	for _, file := range pkg.Syntax {
		if file.Doc != nil {
			return strings.TrimSpace(file.Doc.Text())
		}
	}
	return ""
}

// inputFields reads the exported fields of the input struct in source order.
func inputFields(pkg *packages.Package, typeName string) ([]field, error) {
	var spec *ast.TypeSpec
	for _, file := range pkg.Syntax {
		for _, d := range file.Decls {
			if gen, ok := d.(*ast.GenDecl); ok {
				for _, s := range gen.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
						spec = ts
					}
				}
			}
		}
	}
	structType, ok := spec.Type.(*ast.StructType)
	if spec == nil || !ok {
		return nil, fmt.Errorf("graph: the input %s is not a struct", typeName)
	}
	var fields []field
	for _, f := range structType.Fields.List {
		for _, ident := range f.Names {
			if !ident.IsExported() {
				continue
			}
			kind, optional, err := flagKind(pkg.TypesInfo.TypeOf(f.Type))
			if err != nil {
				return nil, fmt.Errorf("graph: %s.%s: %w", typeName, ident.Name, err)
			}
			fields = append(fields, field{name: ident.Name, flag: kebab(ident.Name), kind: kind, optional: optional, doc: strings.TrimSpace(f.Doc.Text())})
		}
	}
	return fields, nil
}

// flagKind is the flag a field's type makes: a string, an int, or a bool,
// or a polytype.Optional of one, which is set only when given.
func flagKind(t types.Type) (kind string, optional bool, err error) {
	if named, ok := t.(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "github.com/tylergannon/polytype" && named.Obj().Name() == "Optional" && named.TypeArgs().Len() == 1 {
		kind, _, err := flagKind(named.TypeArgs().At(0))
		return kind, true, err
	}
	if basic, ok := t.(*types.Basic); ok {
		switch basic.Kind() {
		case types.String:
			return "string", false, nil
		case types.Int:
			return "int", false, nil
		case types.Bool:
			return "bool", false, nil
		}
	}
	return "", false, fmt.Errorf("%s is not a flag: a field is a string, an int, a bool, or a polytype.Optional of one", t)
}

// kebab is a Go name as a flag name: DryRun is dry-run.
func kebab(name string) string {
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
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
