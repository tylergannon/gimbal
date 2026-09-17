package generate

import (
	"go/ast"
	"go/token"

	"github.com/tylergannon/gimble/workflow"
)

// stmt walks one statement: its own expressions, in evaluation order, and
// then whatever shape the statement itself is.
func (e *extractor) stmt(stmt ast.Stmt, out *[]workflow.Operation, en scopeEnv) {
	switch stmt := stmt.(type) {
	case nil:
		return
	case *ast.IfStmt:
		if stmt.Init != nil {
			e.stmt(stmt.Init, out, en)
		}
		e.exprs(stmt.Cond, out, en)
		e.ifChain(stmt, out, en)
	case *ast.SwitchStmt:
		if stmt.Init != nil {
			e.stmt(stmt.Init, out, en)
		}
		if stmt.Tag != nil {
			e.exprs(stmt.Tag, out, en)
		}
		e.switchStmt(stmt, out, en)
	case *ast.ForStmt:
		if stmt.Init != nil {
			e.stmt(stmt.Init, out, en)
		}
		e.nested(stmt.Pos(), "a for header", stmt.Cond, stmt.Post)
		e.repeat(stmt, stmt.Body, header(e.pkg.Fset, stmt), out, en)
	case *ast.RangeStmt:
		e.rangeStmt(stmt, out, en)
	case *ast.BlockStmt:
		e.block(stmt.List, out, en)
	case *ast.LabeledStmt:
		e.stmt(stmt.Stmt, out, en)
	case *ast.AssignStmt:
		e.assign(stmt, out, en)
	case *ast.DeclStmt:
		e.declStmt(stmt, out, en)
	case *ast.TypeSwitchStmt:
		e.unread(stmt, "a type switch")
	case *ast.SelectStmt:
		e.unread(stmt, "a select statement")
	case *ast.GoStmt:
		e.unread(stmt, "a go statement")
	case *ast.DeferStmt:
		e.unread(stmt, "a defer statement")
	default:
		e.exprs(stmt, out, en)
	}
}

// unread records a construct the rules do not read, but only when it holds
// an operation: what it holds would otherwise be lost silently.
func (e *extractor) unread(stmt ast.Stmt, what string) {
	if e.holdsOperation(stmt) {
		e.diag(stmt.Pos(), "%s holding a Gimble call is not read", what)
	}
}

// assign walks an assignment, binding what its right-hand side produces to
// the identifiers on its left.
func (e *extractor) assign(stmt *ast.AssignStmt, out *[]workflow.Operation, en scopeEnv) {
	for i, rhs := range stmt.Rhs {
		targets := stmt.Lhs
		if len(stmt.Rhs) == len(stmt.Lhs) {
			targets = stmt.Lhs[i : i+1]
		}
		e.value(rhs, targets, out, en)
	}
	for _, lhs := range stmt.Lhs {
		if stmt.Tok == token.ASSIGN || e.redeclares(lhs) {
			e.reassigned(lhs)
		}
	}
}

// redeclares reports whether a short declaration reuses an identifier that
// already exists, which assigns to it rather than declaring it.
func (e *extractor) redeclares(lhs ast.Expr) bool {
	ident, ok := unparen(lhs).(*ast.Ident)
	return ok && e.pkg.TypesInfo.Uses[ident] != nil
}

// declStmt walks a var declaration the same way as an assignment.
func (e *extractor) declStmt(stmt *ast.DeclStmt, out *[]workflow.Operation, en scopeEnv) {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || decl.Tok != token.VAR {
		return
	}
	for _, spec := range decl.Specs {
		values, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, value := range values.Values {
			targets := identExprs(values.Names)
			if len(values.Values) == len(values.Names) {
				targets = identExprs(values.Names[i : i+1])
			}
			e.value(value, targets, out, en)
		}
	}
}

func identExprs(names []*ast.Ident) []ast.Expr {
	out := make([]ast.Expr, len(names))
	for i, name := range names {
		out[i] = name
	}
	return out
}

// reassigned drops what an identifier stood for when a plain assignment
// gives it another value. A workflow names each conversation, group, and
// loop once where it is declared; reassignment is not a shape the rules
// read, so what follows it meets the unbound diagnostics.
func (e *extractor) reassigned(lhs ast.Expr) {
	obj := e.object(lhs)
	if obj == nil {
		return
	}
	what := ""
	if _, ok := e.session[obj]; ok {
		delete(e.session, obj)
		what = "session"
	}
	if _, ok := e.group[obj]; ok {
		delete(e.group, obj)
		what = "group"
	}
	if _, ok := e.loop[obj]; ok {
		delete(e.loop, obj)
		what = "loop"
	}
	if what == "" {
		return
	}
	e.dead[obj] = true
	e.diag(lhs.Pos(), "a %s reassigned after its declaration is not read", what)
}

// ifChain records one if/else chain as a single Condition.
func (e *extractor) ifChain(stmt *ast.IfStmt, out *[]workflow.Operation, en scopeEnv) {
	var branches []branch
	current := stmt
	for current != nil {
		if current != stmt {
			e.nested(current.Pos(), "an else-if header", current.Init, current.Cond)
		}
		branches = append(branches, e.branch(e.ifCase(current), current.Pos(), current.Body.List, current.Cond, en))
		switch alt := current.Else.(type) {
		case *ast.IfStmt:
			current = alt
		case *ast.BlockStmt:
			branches = append(branches, e.branch("", alt.Pos(), alt.List, nil, en))
			current = nil
		default:
			current = nil
		}
	}
	e.condition(stmt.Pos(), branches, out, en)
}

// ifCase is the branch's condition as written: "init; cond" when the if has
// an initializer.
func (e *extractor) ifCase(stmt *ast.IfStmt) string {
	cond := text(e.pkg.Fset, stmt.Cond)
	if stmt.Init == nil {
		return cond
	}
	return text(e.pkg.Fset, stmt.Init) + "; " + cond
}

// switchStmt records one switch as a single Condition. A tagged switch's
// case reads as the comparison it stands for.
func (e *extractor) switchStmt(stmt *ast.SwitchStmt, out *[]workflow.Operation, en scopeEnv) {
	var branches []branch
	for _, clause := range stmt.Body.List {
		c, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}
		e.nested(c.Pos(), "a switch case expression", exprNodes(c.List)...)
		label := ""
		if len(c.List) > 0 {
			label = textList(e.pkg.Fset, c.List)
			if stmt.Tag != nil {
				label = text(e.pkg.Fset, stmt.Tag) + " == " + label
			}
		}
		branches = append(branches, e.branch(label, c.Pos(), c.Body, nil, en))
	}
	e.condition(stmt.Pos(), branches, out, en)
}

// branch is one alternative and the expression that guards it, which the
// error-handling rule reads.
type branch struct {
	node workflow.Branch
	cond ast.Expr
	pos  token.Pos
}

func (e *extractor) branch(label string, pos token.Pos, stmts []ast.Stmt, cond ast.Expr, en scopeEnv) branch {
	body := []workflow.Operation{}
	e.scoped(func() {
		e.block(stmts, &body, scopeEnv{blockTail: en.blockTail, callTail: en.callTail, inHelper: en.inHelper})
	})
	return branch{
		node: workflow.Branch{Source: e.at(pos), Case: label, Exits: exits(stmts), Body: body},
		cond: cond,
		pos:  pos,
	}
}

// exits reports whether a branch ends the flow around it.
func exits(stmts []ast.Stmt) bool {
	if len(stmts) == 0 {
		return false
	}
	switch last := stmts[len(stmts)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return last.Tok == token.BREAK || last.Tok == token.CONTINUE
	default:
		return false
	}
}

// condition decides whether a Condition is shape at all, and records it when
// it is. Error handling is not shape; nor is a guard a helper opens with.
func (e *extractor) condition(pos token.Pos, branches []branch, out *[]workflow.Operation, en scopeEnv) {
	holdsOperation := false
	for _, b := range branches {
		if len(b.node.Body) > 0 {
			holdsOperation = true
		}
	}
	if !holdsOperation && e.allErrorTests(branches) {
		return
	}
	emitted := e.emitted()
	kept := make([]workflow.Branch, 0, len(branches))
	for _, b := range branches {
		guard := en.inHelper && !emitted && b.node.Exits && len(b.node.Body) == 0
		if guard {
			continue
		}
		if en.inHelper && emitted && b.node.Exits && len(b.node.Body) == 0 && !en.callTail {
			e.diag(b.pos, "this return ends only the helper it is written in, not the body the helper was inlined into")
		}
		kept = append(kept, b.node)
	}
	recorded := false
	for _, b := range kept {
		if len(b.Body) > 0 || b.Exits {
			recorded = true
		}
	}
	if !recorded {
		return
	}
	e.emit(out, workflow.Condition{Source: e.at(pos), Branches: kept})
}

// allErrorTests reports whether every branch that has a condition tests an
// error against nil, which is Go's error handling rather than a workflow's
// shape.
func (e *extractor) allErrorTests(branches []branch) bool {
	for _, b := range branches {
		if b.node.Case == "" {
			continue
		}
		if b.cond == nil || !e.isErrorTest(b.cond) {
			return false
		}
	}
	return true
}

func (e *extractor) isErrorTest(expr ast.Expr) bool {
	switch expr := unparen(expr).(type) {
	case *ast.BinaryExpr:
		switch expr.Op {
		case token.LAND, token.LOR:
			return e.isErrorTest(expr.X) && e.isErrorTest(expr.Y)
		case token.EQL, token.NEQ:
			return e.isNilErrorOperands(expr.X, expr.Y) || e.isNilErrorOperands(expr.Y, expr.X)
		}
	}
	return false
}

func (e *extractor) isNilErrorOperands(value, nilExpr ast.Expr) bool {
	ident, ok := unparen(nilExpr).(*ast.Ident)
	if !ok || ident.Name != "nil" {
		return false
	}
	t := e.pkg.TypesInfo.TypeOf(unparen(value))
	return t != nil && t.String() == "error"
}

// repeat records a Go loop that holds an operation. The loop creates no
// Gimble scope; how many times it runs is the run's record.
func (e *extractor) repeat(stmt ast.Stmt, block *ast.BlockStmt, cond string, out *[]workflow.Operation, en scopeEnv) {
	if !e.holdsOperation(block) {
		return
	}
	body := []workflow.Operation{}
	e.body(block.List, &body, en)
	e.emit(out, workflow.Repeat{Source: e.at(stmt.Pos()), Cond: cond, Body: body})
}

// rangeStmt is a range over PromiseLoop.Tasks, gimble.Iterate, or an ordinary
// Go range, which is a Repeat.
func (e *extractor) rangeStmt(stmt *ast.RangeStmt, out *[]workflow.Operation, en scopeEnv) {
	if _, ok := e.promiseTasksRange(stmt); ok {
		e.promiseTasksRangeBody(stmt, out, en)
		return
	}
	if call, ok := e.iterateRange(stmt); ok {
		e.iterateRangeBody(stmt, call, out, en)
		return
	}
	e.exprs(stmt.X, out, en)
	e.repeat(stmt, stmt.Body, header(e.pkg.Fset, stmt), out, en)
}

func (e *extractor) promiseTasksRangeBody(stmt *ast.RangeStmt, out *[]workflow.Operation, en scopeEnv) {
	selector, _ := e.promiseTasksRange(stmt)
	obj := e.object(selector.X)
	ref := e.loop[obj]
	if ref == nil {
		e.diag(stmt.X.Pos(), "a Tasks range over something other than a PromiseLoop declared in an enclosing body is not read")
		return
	}
	if ref.ops != out {
		e.diag(stmt.X.Pos(), "a Tasks range outside the body that declared its PromiseLoop is not read")
		return
	}
	body := []workflow.Operation{}
	e.body(stmt.Body.List, &body, en)
	loop, ok := (*ref.ops)[ref.index].(workflow.PromiseLoop)
	if !ok {
		return
	}
	loop.Body = body
	(*ref.ops)[ref.index] = loop
}

func (e *extractor) iterateRangeBody(stmt *ast.RangeStmt, call *ast.CallExpr, out *[]workflow.Operation, en scopeEnv) {
	name, ok := e.constant(call, 1)
	if !ok {
		e.diag(call.Pos(), "Iterate's scope name is not a constant, so the iteration is not read")
		return
	}
	body := []workflow.Operation{}
	e.body(stmt.Body.List, &body, en)
	e.emit(out, workflow.Iterate{Source: e.at(call.Pos()), Name: name, Body: body})
}
