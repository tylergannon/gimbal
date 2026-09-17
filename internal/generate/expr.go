package generate

import (
	"go/ast"
	"go/types"
	"slices"

	"github.com/tylergannon/gimble/workflow"
	"golang.org/x/tools/go/types/typeutil"
)

// exprs walks the expressions of one node for the operations written in
// them. A function literal that is not a Scope or Go callback is not walked.
func (e *extractor) exprs(node ast.Node, out *[]workflow.Operation, en scopeEnv) {
	if node == nil {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			if e.holdsOperation(n.Body) {
				e.diag(n.Pos(), "a function literal that is not a Scope or Go callback holding a Gimble call is not read")
			}
			return false
		case *ast.CallExpr:
			return e.call(n, nil, out, en)
		}
		return true
	})
}

// value walks one right-hand side, telling the call that produced it which
// identifiers it is assigned to.
func (e *extractor) value(expr ast.Expr, targets []ast.Expr, out *[]workflow.Operation, en scopeEnv) {
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok {
		e.exprs(expr, out, en)
		return
	}
	if !e.call(call, targets, out, en) {
		return
	}
	e.exprs(call.Fun, out, en)
	for _, arg := range call.Args {
		e.exprs(arg, out, en)
	}
}

// call reads one call. It reports whether the walk should keep descending
// into the call's own parts.
func (e *extractor) call(call *ast.CallExpr, targets []ast.Expr, out *[]workflow.Operation, en scopeEnv) bool {
	if name, ok := e.gimbleCall(call); ok {
		return e.gimbleOperation(name, call, targets, out, en)
	}
	callee := typeutil.StaticCallee(e.pkg.TypesInfo, call)
	if callee == nil {
		if takesContext(e.pkg.TypesInfo.TypeOf(call.Fun)) {
			e.diag(call.Pos(), "a call through a function value is not read")
		}
		return true
	}
	if callee.Pkg() == e.pkg.Types {
		return e.inline(callee, call, out, en)
	}
	if e.importsGimble(callee.Pkg()) {
		e.diag(call.Pos(), "a call into %s, which imports Gimble, is not followed", callee.Pkg().Path())
	}
	return true
}

// inline walks a package-local helper's body at its call site, so what it
// does stands where the workflow does it.
func (e *extractor) inline(callee *types.Func, call *ast.CallExpr, out *[]workflow.Operation, en scopeEnv) bool {
	decl := e.declFor(callee)
	if decl == nil || decl.Body == nil {
		return true
	}
	if slices.Contains(e.stack, callee) {
		e.diag(call.Pos(), "%s calls itself, and a recursive helper is not read", callee.Name())
		return true
	}
	e.exprs(call.Fun, out, en)
	for _, arg := range call.Args {
		e.exprs(arg, out, en)
	}
	e.bindParameters(decl, callee, call)
	e.stack = append(e.stack, callee)
	e.helpers = append(e.helpers, &helperState{})
	e.block(decl.Body.List, out, scopeEnv{blockTail: en.blockTail, callTail: en.blockTail, inHelper: true})
	e.helpers = e.helpers[:len(e.helpers)-1]
	e.stack = e.stack[:len(e.stack)-1]
	return false
}

// bindParameters binds a helper's session parameters to the sessions passed
// in, so a call the helper makes names the conversation the caller meant.
func (e *extractor) bindParameters(decl *ast.FuncDecl, callee *types.Func, call *ast.CallExpr) {
	signature, ok := callee.Type().(*types.Signature)
	if !ok {
		return
	}
	index := 0
	for _, field := range decl.Type.Params.List {
		for _, name := range field.Names {
			obj := e.pkg.TypesInfo.Defs[name]
			position := index
			index++
			if obj == nil || !isSessionType(obj.Type()) {
				continue
			}
			delete(e.session, obj)
			if signature.Variadic() && position >= signature.Params().Len()-1 {
				continue
			}
			if position >= len(call.Args) {
				continue
			}
			source, ok := e.binding(call.Args[position])
			if !ok {
				e.diag(call.Args[position].Pos(), "%s is given a session that is not one declared in an enclosing body", callee.Name())
				continue
			}
			e.session[obj] = source
		}
	}
}

// gimbleOperation reads one call on the Gimble package. It reports whether
// the walk should keep descending into the call's own parts.
func (e *extractor) gimbleOperation(name string, call *ast.CallExpr, targets []ast.Expr, out *[]workflow.Operation, en scopeEnv) bool {
	e.nested(call.Pos(), "a Gimble call's arguments", plainArguments(name, call)...)
	switch name {
	case "NewSession":
		role, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "NewSession's role is not a constant, so the session it makes is not read")
			e.unbind(targets)
			return false
		}
		e.emit(out, workflow.Session{Source: e.at(call.Pos()), Name: role})
		if obj := e.firstObject(targets); obj != nil {
			e.session[obj] = binding{name: role, role: role}
		}
		return false

	case "Fork":
		source, ok := e.binding(e.receiver(call))
		if !ok {
			e.diag(call.Pos(), "Fork is called on something that is not a session declared in an enclosing body")
			e.unbind(targets)
			return false
		}
		forked, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "Fork's name is not a constant, so the session it makes is not read")
			e.unbind(targets)
			return false
		}
		e.emit(out, workflow.Session{Source: e.at(call.Pos()), Name: forked, From: source.name})
		if obj := e.firstObject(targets); obj != nil {
			e.session[obj] = binding{name: forked, role: source.role}
		}
		return false

	case "Generate":
		speaker, ok := e.binding(e.receiver(call))
		if !ok {
			e.diag(call.Pos(), "Generate is called on something that is not a session declared in an enclosing body")
			return false
		}
		prompt, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "Generate's prompt is not a constant, so the call is not read")
			return false
		}
		e.emit(out, workflow.AgentCall{
			Source:      e.at(call.Pos()),
			Session:     speaker.name,
			Role:        speaker.role,
			Prompt:      prompt,
			Supervisors: e.supervisors(call, 2),
		})
		return false

	case "RunCommand":
		command, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "RunCommand's name is not a constant, so the command is not read")
			return false
		}
		e.emit(out, workflow.Command{Source: e.at(call.Pos()), Name: command})
		return false

	case "Set", "SetJSON":
		key, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "the context key is not a constant, so what is written here is not read")
			return false
		}
		e.emit(out, workflow.Set{Source: e.at(call.Pos()), Key: key})
		return false

	case "Scope":
		scope, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "Scope's name is not a constant, so the scope is not read")
			return false
		}
		body := e.callback(call, 2)
		e.emit(out, workflow.Scope{Source: e.at(call.Pos()), Name: scope, Body: body})
		return false

	case "Group":
		group, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "Group's name is not a constant, so the group is not read")
			return false
		}
		e.emit(out, workflow.Group{Source: e.at(call.Pos()), Name: group, Children: []workflow.GroupChild{}})
		if obj := e.firstObject(targets); obj != nil {
			e.group[obj] = &nodeRef{ops: out, index: len(*out) - 1}
		}
		return false

	case "Go":
		ref := e.group[e.object(e.receiver(call))]
		if ref == nil {
			e.diag(call.Pos(), "Go is called on something that is not a group declared in an enclosing body")
			return false
		}
		if ref.ops != out {
			e.diag(call.Pos(), "Go on a group declared outside this body is not read")
			return false
		}
		child, ok := e.constant(call, 0)
		if !ok {
			e.diag(call.Pos(), "Go's name is not a constant, so the child is not read")
			return false
		}
		body := e.callback(call, 1)
		group, ok := (*ref.ops)[ref.index].(workflow.Group)
		if !ok {
			return false
		}
		group.Children = append(group.Children, workflow.GroupChild{Source: e.at(call.Pos()), Name: child, Body: body})
		(*ref.ops)[ref.index] = group
		return false

	case "PromiseLoop":
		loop, ok := e.constant(call, 1)
		if !ok {
			e.diag(call.Pos(), "PromiseLoop's name is not a constant, so the loop is not read")
			return false
		}
		if len(call.Args) < 4 {
			return false
		}
		planner, ok := e.binding(call.Args[3])
		if !ok {
			e.diag(call.Args[3].Pos(), "PromiseLoop's planner is not a session declared in an enclosing body")
			return false
		}
		e.emit(out, workflow.PromiseLoop{
			Source:  e.at(call.Pos()),
			Name:    loop,
			Planner: planner.name,
			Body:    []workflow.Operation{},
		})
		if obj := e.firstObject(targets); obj != nil {
			e.loop[obj] = &nodeRef{ops: out, index: len(*out) - 1}
		}
		return false

	case "Run":
		e.diag(call.Pos(), "Run starts a workflow and belongs to main, not inside one")
		return false

	default:
		return true
	}
}

// callback walks the body a Scope or Go call is given: a function literal, or
// a package-local function whose block stands in its place.
func (e *extractor) callback(call *ast.CallExpr, index int) []workflow.Operation {
	body := []workflow.Operation{}
	if index >= len(call.Args) {
		return body
	}
	switch fn := unparen(call.Args[index]).(type) {
	case *ast.FuncLit:
		e.callbackBody(fn.Body.List, &body)
	default:
		callee, _ := e.pkg.TypesInfo.Uses[identOf(fn)].(*types.Func)
		if callee == nil || callee.Pkg() != e.pkg.Types {
			e.diag(call.Args[index].Pos(), "the body is not a function literal or a function of this package, so it is not read")
			return body
		}
		decl := e.declFor(callee)
		if decl == nil || decl.Body == nil {
			e.diag(call.Args[index].Pos(), "the body is not a function of this package with a body, so it is not read")
			return body
		}
		if slices.Contains(e.stack, callee) {
			e.diag(call.Args[index].Pos(), "%s calls itself, and a recursive body is not read", callee.Name())
			return body
		}
		e.stack = append(e.stack, callee)
		e.callbackBody(decl.Body.List, &body)
		e.stack = e.stack[:len(e.stack)-1]
	}
	return body
}

// supervisors reads the options of a Generate call: the supervisors watching
// it, and nothing else.
func (e *extractor) supervisors(call *ast.CallExpr, from int) []workflow.Supervisor {
	var watching []workflow.Supervisor
	if from >= len(call.Args) {
		return watching
	}
	for _, option := range call.Args[from:] {
		supervisor, ok := e.supervisor(option)
		if ok {
			watching = append(watching, supervisor)
		}
	}
	return watching
}

func (e *extractor) supervisor(option ast.Expr) (workflow.Supervisor, bool) {
	call, ok := unparen(option).(*ast.CallExpr)
	if !ok {
		e.diag(option.Pos(), "an option that is not a Gimble option is not read")
		return workflow.Supervisor{}, false
	}
	name, ok := e.gimbleCall(call)
	if !ok {
		e.diag(option.Pos(), "an option that is not a Gimble option is not read")
		return workflow.Supervisor{}, false
	}
	if name != "WithSupervisor" {
		return workflow.Supervisor{}, false
	}
	watcher, ok := e.binding(call.Args[0])
	if !ok {
		e.diag(call.Args[0].Pos(), "the supervisor is not a session declared in an enclosing body")
		return workflow.Supervisor{}, false
	}
	instruction, ok := e.constant(call, 1)
	if !ok {
		e.diag(call.Pos(), "the supervisor's instruction is not a constant, so the supervision is not read")
		return workflow.Supervisor{}, false
	}
	return workflow.Supervisor{
		Source:      e.at(call.Pos()),
		Session:     watcher.name,
		Role:        watcher.role,
		Instruction: instruction,
		Supervisors: e.supervisors(call, 2),
	}, true
}
