package graph

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/types/typeutil"
)

// gimbleCall names the Gimble function or method a call resolves to.
func (e *extractor) gimbleCall(call *ast.CallExpr) (string, bool) {
	callee := typeutil.Callee(e.pkg.TypesInfo, call)
	if callee == nil || callee.Pkg() == nil || callee.Pkg().Path() != gimblePath {
		return "", false
	}
	return callee.Name(), true
}

// isTasksRange reports whether a range statement ranges over a Loop's Tasks.
func (e *extractor) isTasksRange(stmt *ast.RangeStmt) bool {
	selector, ok := unparen(stmt.X).(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Tasks" {
		return false
	}
	return isGimbleTasks(e.pkg.TypesInfo, selector)
}

func isGimbleTasks(info *types.Info, selector *ast.SelectorExpr) bool {
	selection := info.Selections[selector]
	if selection == nil {
		return false
	}
	obj := selection.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == gimblePath && obj.Name() == "Tasks"
}

// receiver is the value a method is called on.
func (e *extractor) receiver(call *ast.CallExpr) ast.Expr {
	fun := unparen(call.Fun)
	switch f := fun.(type) {
	case *ast.IndexExpr:
		fun = unparen(f.X)
	case *ast.IndexListExpr:
		fun = unparen(f.X)
	}
	if selector, ok := fun.(*ast.SelectorExpr); ok {
		return selector.X
	}
	return nil
}

func (e *extractor) object(expr ast.Expr) types.Object {
	if expr == nil {
		return nil
	}
	ident, ok := unparen(expr).(*ast.Ident)
	if !ok {
		return nil
	}
	if obj := e.pkg.TypesInfo.Uses[ident]; obj != nil {
		return obj
	}
	return e.pkg.TypesInfo.Defs[ident]
}

// binding is the session an expression stands for, when it is an identifier
// declared by NewSession or Fork in an enclosing body.
func (e *extractor) binding(expr ast.Expr) (binding, bool) {
	obj := e.object(expr)
	if obj == nil {
		return binding{}, false
	}
	b, ok := e.session[obj]
	return b, ok
}

func (e *extractor) firstObject(targets []ast.Expr) types.Object {
	for _, target := range targets {
		if obj := e.object(target); obj != nil {
			return obj
		}
	}
	return nil
}

// constant is the compile-time string an argument spells out.
func (e *extractor) constant(call *ast.CallExpr, index int) (string, bool) {
	if index >= len(call.Args) {
		return "", false
	}
	value := e.pkg.TypesInfo.Types[call.Args[index]].Value
	if value == nil || value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value), true
}

func (e *extractor) declFor(fn *types.Func) *ast.FuncDecl {
	for _, file := range e.pkg.Syntax {
		for _, d := range file.Decls {
			decl, ok := d.(*ast.FuncDecl)
			if ok && e.pkg.TypesInfo.Defs[decl.Name] == fn {
				return decl
			}
		}
	}
	return nil
}

func isSessionType(t types.Type) bool {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == gimblePath && named.Obj().Name() == "Session"
}

func takesContext(t types.Type) bool {
	if t == nil {
		return false
	}
	signature, ok := t.Underlying().(*types.Signature)
	if !ok {
		return false
	}
	for v := range signature.Params().Variables() {
		named, ok := v.Type().(*types.Named)
		if ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context" {
			return true
		}
	}
	return false
}

// importsGimble reports whether another package of this module imports
// Gimble, which makes a call into it something the extractor does not follow.
func (e *extractor) importsGimble(pkg *types.Package) bool {
	if pkg == nil || e.modulePath == "" || !strings.HasPrefix(pkg.Path(), e.modulePath) {
		return false
	}
	for _, imported := range pkg.Imports() {
		if imported.Path() == gimblePath {
			return true
		}
	}
	return false
}

// holdsOperation reports whether a statement, or a same-package function it
// calls statically, contains a Gimble operation.
func (e *extractor) holdsOperation(node ast.Node) bool {
	if node == nil {
		return false
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if n.Sel.Name == "Tasks" && isGimbleTasks(e.pkg.TypesInfo, n) {
				found = true
			}
		case *ast.CallExpr:
			if name, ok := e.gimbleCall(n); ok && isOperation(name) {
				found = true
				return false
			}
			callee := typeutil.StaticCallee(e.pkg.TypesInfo, n)
			if callee != nil && callee.Pkg() == e.pkg.Types && e.funcHoldsOperation(callee) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func (e *extractor) funcHoldsOperation(fn *types.Func) bool {
	if answer, ok := e.hasOp[fn]; ok {
		return answer
	}
	if e.walking == nil {
		e.walking = make(map[*types.Func]bool)
	}
	if e.walking[fn] {
		return false
	}
	decl := e.declFor(fn)
	if decl == nil || decl.Body == nil {
		e.hasOp[fn] = false
		return false
	}
	e.walking[fn] = true
	answer := e.holdsOperation(decl.Body)
	delete(e.walking, fn)
	e.hasOp[fn] = answer
	return answer
}

func isOperation(name string) bool {
	switch name {
	case "Run", "Scope", "Group", "Go", "Loop", "Tasks", "NewSession", "Fork", "Generate", "Set", "SetJSON", "RunCommand":
		return true
	default:
		return false
	}
}

// text prints one node the way the source writes it.
func text(fset *token.FileSet, node any) string {
	if node == nil {
		return ""
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, fset, node); err != nil {
		return ""
	}
	return buffer.String()
}

func textList(fset *token.FileSet, exprs []ast.Expr) string {
	parts := make([]string, len(exprs))
	for i, expr := range exprs {
		parts[i] = text(fset, expr)
	}
	return strings.Join(parts, ", ")
}

// header is a Go loop's header as written, with no for keyword and no braces.
func header(fset *token.FileSet, stmt ast.Stmt) string {
	switch stmt := stmt.(type) {
	case *ast.ForStmt:
		if stmt.Init == nil && stmt.Post == nil {
			return text(fset, stmt.Cond)
		}
		return text(fset, stmt.Init) + "; " + text(fset, stmt.Cond) + "; " + text(fset, stmt.Post)
	case *ast.RangeStmt:
		over := "range " + text(fset, stmt.X)
		if stmt.Key == nil {
			return over
		}
		names := text(fset, stmt.Key)
		if stmt.Value != nil {
			names += ", " + text(fset, stmt.Value)
		}
		return names + " " + stmt.Tok.String() + " " + over
	}
	return ""
}

func identOf(expr ast.Expr) *ast.Ident {
	switch expr := unparen(expr).(type) {
	case *ast.Ident:
		return expr
	case *ast.SelectorExpr:
		return expr.Sel
	}
	return nil
}

func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

// nested records a Gimble call written in an expression the rules do not
// read, once for the whole expression. The authoring rule is to simplify
// such source, not to guess at what it does.
func (e *extractor) nested(pos token.Pos, where string, nodes ...ast.Node) {
	if slices.ContainsFunc(nodes, e.holdsOperation) {
		e.diag(pos, "a Gimble call nested in %s is not read", where)
		return
	}
}

// unbind drops what an identifier stood for, so a session call the rules
// could not read leaves no stale binding behind.
func (e *extractor) unbind(targets []ast.Expr) {
	if obj := e.firstObject(targets); obj != nil {
		delete(e.session, obj)
	}
}

// plainArguments are the arguments of a Gimble call that are neither a body
// nor an option: the ones whose own calls the rules do not read.
func plainArguments(name string, call *ast.CallExpr) []ast.Node {
	skip := -1
	last := len(call.Args)
	switch name {
	case "Scope":
		skip = 2
	case "Go":
		skip = 1
	case "Generate", "WithSupervisor":
		last = min(last, 2)
	}
	nodes := make([]ast.Node, 0, last)
	for i := range last {
		if i != skip {
			nodes = append(nodes, call.Args[i])
		}
	}
	return nodes
}

func exprNodes(exprs []ast.Expr) []ast.Node {
	nodes := make([]ast.Node, len(exprs))
	for i, expr := range exprs {
		nodes[i] = expr
	}
	return nodes
}
