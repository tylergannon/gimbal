// Package gimblelint checks deterministic workflow authoring mistakes.
package gimblelint

import (
	_ "embed"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	dynamicWorkers = "[GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS]: Workflow control flow must be visible in the source. Call worker functions directly in an if or switch instead of selecting a function dynamically."
	constantKey    = "[GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY]: Context keys must be compile-time constants so a workflow's recorded fields are explicit in its source. Use a named constant or string literal; keep changing data in the value."
	duplicateKey   = "[GIMBLE103-SET-MISUSE/DUPLICATE-KEY]: This scope can set the same context key more than once. Use one write per key in each scope."
	wrongContext   = "[GIMBLE104-SET-MISUSE/WRONG-CONTEXT]: Set must use the context parameter of this child scope."
	unjoinedGo     = "[GIMBLE105-SET-MISUSE/UNJOINED-GOROUTINE]: Set in a raw goroutine can outlive its scope. Use Group."
	reservedTask   = "[GIMBLE106-SET-MISUSE/RESERVED-TASK-KEY]: The task key belongs to Loop.Tasks and cannot be set by the task body."
	noScopeContext = "[GIMBLE107-SET-MISUSE/CONTEXT-NOT-FROM-SCOPE]: Set needs a context supplied by a Gimble scope, not context.Background or context.TODO."
)

const gimblePath = "github.com/tylergannon/gimble"

// rules is the canonical user-facing description of the analyzer. Embedding
// it makes `gimble lint -help` describe the exact rules carried by the binary.
//
//go:embed rules.md
var rules string

// Analyzer checks the accepted Gimble workflow lint rules. Its worker-dispatch
// analysis intentionally follows only source-visible functions in the current
// package; it is not whole-program pointer analysis.
var Analyzer = &analysis.Analyzer{
	Name:     "gimblelint",
	Doc:      rules,
	Requires: []*analysis.Analyzer{inspect.Analyzer, buildssa.Analyzer},
	Run:      run,
}

type boundary struct {
	body   *ast.BlockStmt
	ctxObj types.Object
	tasks  bool
}

type syntaxInfo struct {
	parents    map[ast.Node]ast.Node
	boundaries map[ast.Node]boundary
	units      []*unit
	unitFor    map[ast.Node]*unit
}

type unit struct {
	node        ast.Node
	body        *ast.BlockStmt
	workflow    bool
	reachable   bool
	directCalls map[*types.Func]bool
}

func run(pass *analysis.Pass) (any, error) {
	inspectResult := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := indexSyntax(pass, inspectResult)
	reportSetSyntax(pass, info)
	reportDynamicWorkers(pass, info)
	reportDuplicates(pass, pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA))
	return nil, nil
}

func indexSyntax(pass *analysis.Pass, in *inspector.Inspector) *syntaxInfo {
	si := &syntaxInfo{
		parents:    make(map[ast.Node]ast.Node),
		boundaries: make(map[ast.Node]boundary),
		unitFor:    make(map[ast.Node]*unit),
	}

	// Materialize parent links once. The inspector is the analyzer's shared,
	// pre-indexed syntax input; parent links make the later boundary queries
	// explicit and cheap.
	for cursor := range in.Root().Preorder() {
		if parent := cursor.Parent(); parent.Node() != nil {
			si.parents[cursor.Node()] = parent.Node()
		}
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					u := &unit{node: n, body: n.Body, directCalls: make(map[*types.Func]bool)}
					si.units = append(si.units, u)
					markUnit(si, n.Body, u)
				}
			case *ast.FuncLit:
				u := &unit{node: n, body: n.Body, directCalls: make(map[*types.Func]bool)}
				si.units = append(si.units, u)
				markUnit(si, n.Body, u)
			}
			return true
		})
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				name, ok := gimbleCall(pass, n)
				if ok {
					if u := enclosingUnit(si, n); u != nil && pass.Pkg.Path() != gimblePath && isWorkflowOperation(name) {
						u.workflow = true
					}
					if callbackIndex, ok := callbackArgument(name); ok && callbackIndex < len(n.Args) {
						callback := n.Args[callbackIndex]
						if lit, ok := callback.(*ast.FuncLit); ok {
							si.boundaries[lit] = boundary{body: lit.Body, ctxObj: firstParam(pass, lit.Type)}
							if u := si.unitFor[lit]; u != nil {
								u.workflow = true
							}
						}
						if fn := functionValue(pass, callback); fn != nil {
							if target := unitForFunc(pass, si, fn); target != nil {
								target.workflow = true
							}
						}
					}
				}
				if callee := typeutil.StaticCallee(pass.TypesInfo, n); callee != nil && callee.Pkg() == pass.Pkg {
					// In same-package tests, calls to Gimble's API resolve as local
					// functions. They are workflow nodes, not helper bodies to follow
					// into the runtime implementation.
					apiCall := pass.Pkg.Path() == gimblePath && isWorkflowOperation(callee.Name())
					if u := enclosingUnit(si, n); u != nil && !apiCall {
						u.directCalls[callee] = true
					}
				}
			case *ast.RangeStmt:
				if !isTasksRange(pass, n) {
					break
				}
				ctxObj := identObject(pass, n.Key)
				si.boundaries[n] = boundary{body: n.Body, ctxObj: ctxObj, tasks: true}
				if u := enclosingUnit(si, n); u != nil {
					u.workflow = true
				}
			}
			return true
		})
	}

	// A workflow function's source-visible helpers are part of the same domain.
	// Iterate because a helper may call another helper in the package.
	changed := true
	for changed {
		changed = false
		for _, u := range si.units {
			if !u.workflow && !u.reachable {
				continue
			}
			for fn := range u.directCalls {
				if target := unitForFunc(pass, si, fn); target != nil && !target.reachable {
					target.reachable = true
					changed = true
				}
			}
		}
	}
	return si
}

func markUnit(si *syntaxInfo, body *ast.BlockStmt, u *unit) {
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n != body {
			switch n.(type) {
			case *ast.FuncLit:
				return false
			}
		}
		si.unitFor[n] = u
		return true
	})
	if lit, ok := u.node.(*ast.FuncLit); ok {
		si.unitFor[lit] = u
	}
}

func unitForFunc(pass *analysis.Pass, si *syntaxInfo, fn *types.Func) *unit {
	for _, u := range si.units {
		decl, ok := u.node.(*ast.FuncDecl)
		if ok && pass.TypesInfo.Defs[decl.Name] == fn {
			return u
		}
	}
	return nil
}

func enclosingUnit(si *syntaxInfo, n ast.Node) *unit {
	for current := n; current != nil; current = si.parents[current] {
		if u := si.unitFor[current]; u != nil {
			return u
		}
	}
	return nil
}

func reportSetSyntax(pass *analysis.Pass, si *syntaxInfo) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSetCall(pass, call) || len(call.Args) < 2 {
				return true
			}

			if value := pass.TypesInfo.Types[call.Args[1]].Value; value == nil || value.Kind() != constant.String {
				pass.Reportf(call.Args[1].Pos(), "%s", constantKey)
			}
			if isBackground(pass, call.Args[0]) {
				pass.Reportf(call.Args[0].Pos(), "%s", noScopeContext)
			}

			if b, ok := enclosingBoundary(si, call); ok {
				ctxObj := identObject(pass, call.Args[0])
				if b.ctxObj != nil && ctxObj != b.ctxObj {
					pass.Reportf(call.Args[0].Pos(), "%s", wrongContext)
				}
				if b.tasks && ctxObj == b.ctxObj && constantString(pass, call.Args[1]) == "task" {
					pass.Reportf(call.Args[1].Pos(), "%s", reservedTask)
				}
				if hasRawGoAncestor(si, call, b.body) {
					pass.Reportf(call.Pos(), "%s", unjoinedGo)
				}
			}
			return true
		})
	}
}

func enclosingBoundary(si *syntaxInfo, n ast.Node) (boundary, bool) {
	for current := n; current != nil; current = si.parents[current] {
		if b, ok := si.boundaries[current]; ok {
			return b, true
		}
	}
	return boundary{}, false
}

func hasRawGoAncestor(si *syntaxInfo, n ast.Node, stop ast.Node) bool {
	for current := n; current != nil && current != stop; current = si.parents[current] {
		if _, ok := current.(*ast.GoStmt); ok {
			return true
		}
	}
	return false
}

func reportDynamicWorkers(pass *analysis.Pass, si *syntaxInfo) {
	for _, u := range si.units {
		if !u.workflow && !u.reachable {
			continue
		}
		dynamic := make(map[types.Object]bool)
		multiple := make(map[types.Object]map[*types.Func]bool)

		// Local function values selected from collections, opaque callback
		// parameters, and variables assigned more than one known function are
		// dynamic worker choices. A local alias of one known function is allowed.
		ast.Inspect(u.body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok && lit != u.node {
				return false
			}
			switch n := n.(type) {
			case *ast.AssignStmt:
				for i, lhs := range n.Lhs {
					if i >= len(n.Rhs) {
						continue
					}
					obj := identObject(pass, lhs)
					if obj == nil || !isFunctionType(obj.Type()) {
						continue
					}
					if fromDynamic(pass, n.Rhs[i], dynamic) {
						dynamic[obj] = true
					}
					if fn := functionValue(pass, n.Rhs[i]); fn != nil {
						if multiple[obj] == nil {
							multiple[obj] = make(map[*types.Func]bool)
						}
						multiple[obj][fn] = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range n.Names {
					if i >= len(n.Values) {
						continue
					}
					obj := pass.TypesInfo.Defs[name]
					if obj == nil || !isFunctionType(obj.Type()) {
						continue
					}
					if fromDynamic(pass, n.Values[i], dynamic) {
						dynamic[obj] = true
					}
					if fn := functionValue(pass, n.Values[i]); fn != nil {
						multiple[obj] = map[*types.Func]bool{fn: true}
					}
				}
			}
			return true
		})
		for obj, targets := range multiple {
			if len(targets) > 1 {
				dynamic[obj] = true
			}
		}

		ast.Inspect(u.body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok && lit != u.node {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if isWorkerFunction(pass.TypesInfo.TypeOf(call.Fun)) && isCollectionFunction(pass, call.Fun) {
				pass.Reportf(call.Fun.Pos(), "%s", dynamicWorkers)
				return true
			}
			if id, ok := unparen(call.Fun).(*ast.Ident); ok {
				obj := identObject(pass, id)
				if isWorkerFunction(pass.TypesInfo.TypeOf(call.Fun)) && (dynamic[obj] || isFunctionParameter(pass, u, obj)) {
					pass.Reportf(call.Fun.Pos(), "%s", dynamicWorkers)
				}
			}
			return true
		})
	}
}

func isWorkerFunction(t types.Type) bool {
	if t == nil {
		return false
	}
	signature, ok := t.Underlying().(*types.Signature)
	if !ok {
		return false
	}
	for i := 0; i < signature.Params().Len(); i++ {
		named, ok := signature.Params().At(i).Type().(*types.Named)
		if ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context" {
			return true
		}
	}
	return false
}

func fromDynamic(pass *analysis.Pass, expr ast.Expr, known map[types.Object]bool) bool {
	expr = unparen(expr)
	if isCollectionFunction(pass, expr) {
		return true
	}
	if id, ok := expr.(*ast.Ident); ok {
		return known[identObject(pass, id)]
	}
	return false
}

func functionValue(pass *analysis.Pass, expr ast.Expr) *types.Func {
	expr = unparen(expr)
	switch expr := expr.(type) {
	case *ast.Ident:
		fn, _ := pass.TypesInfo.Uses[expr].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		fn, _ := pass.TypesInfo.Uses[expr.Sel].(*types.Func)
		return fn
	}
	return nil
}

func isFunctionParameter(pass *analysis.Pass, u *unit, obj types.Object) bool {
	v, ok := obj.(*types.Var)
	if !ok || !isFunctionType(v.Type()) {
		return false
	}
	var fields *ast.FieldList
	switch n := u.node.(type) {
	case *ast.FuncDecl:
		fields = n.Type.Params
	case *ast.FuncLit:
		fields = n.Type.Params
	}
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if pass.TypesInfo.Defs[name] == obj {
				return true
			}
		}
	}
	return false
}

func reportDuplicates(pass *analysis.Pass, result *buildssa.SSA) {
	reported := make(map[token.Pos]bool)
	for _, fn := range result.SrcFuncs {
		type setCall struct {
			call  ssa.CallInstruction
			ctx   ssa.Value
			key   string
			block *ssa.BasicBlock
			index int
		}
		var calls []setCall
		for _, block := range fn.Blocks {
			for index, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok || !isSSASet(call) || len(call.Common().Args) < 2 {
					continue
				}
				key, ok := ssaString(call.Common().Args[1])
				if !ok {
					continue
				}
				calls = append(calls, setCall{call: call, ctx: stripSSA(call.Common().Args[0]), key: key, block: block, index: index})
			}
		}

		for laterIndex, later := range calls {
			for earlierIndex := 0; earlierIndex < laterIndex; earlierIndex++ {
				earlier := calls[earlierIndex]
				if earlier.key != later.key || earlier.ctx != later.ctx {
					continue
				}
				if earlier.block.Dominates(later.block) && (earlier.block != later.block || earlier.index < later.index) {
					if !reported[later.call.Pos()] {
						pass.Reportf(later.call.Pos(), "%s", duplicateKey)
						reported[later.call.Pos()] = true
					}
				}
			}
		}

		for _, loop := range naturalLoops(fn) {
			for _, call := range calls {
				if !loop[call.block] || reported[call.call.Pos()] {
					continue
				}
				if defBlock := valueBlock(call.ctx); defBlock == nil || !loop[defBlock] {
					pass.Reportf(call.call.Pos(), "%s", duplicateKey)
					reported[call.call.Pos()] = true
				}
			}
		}

		// Go lowers range-over-func bodies to a synthetic yield function. A
		// captured outer context is a FreeVar and is reused every time the
		// iterator calls yield; the yielded task context is a Parameter and is
		// fresh for each task scope.
		if _, ok := fn.Syntax().(*ast.RangeStmt); ok {
			for _, call := range calls {
				if _, captured := call.ctx.(*ssa.FreeVar); captured && !reported[call.call.Pos()] {
					pass.Reportf(call.call.Pos(), "%s", duplicateKey)
					reported[call.call.Pos()] = true
				}
			}
		}
	}
}

func naturalLoops(fn *ssa.Function) []map[*ssa.BasicBlock]bool {
	var loops []map[*ssa.BasicBlock]bool
	for _, tail := range fn.Blocks {
		for _, header := range tail.Succs {
			if !header.Dominates(tail) {
				continue
			}
			members := map[*ssa.BasicBlock]bool{header: true, tail: true}
			stack := []*ssa.BasicBlock{tail}
			for len(stack) > 0 {
				last := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, pred := range last.Preds {
					if !members[pred] && header.Dominates(pred) {
						members[pred] = true
						stack = append(stack, pred)
					}
				}
			}
			loops = append(loops, members)
		}
	}
	return loops
}

func valueBlock(v ssa.Value) *ssa.BasicBlock {
	if instruction, ok := v.(ssa.Instruction); ok {
		return instruction.Block()
	}
	return nil
}

func stripSSA(v ssa.Value) ssa.Value {
	for {
		switch value := v.(type) {
		case *ssa.MakeInterface:
			v = value.X
		case *ssa.ChangeInterface:
			v = value.X
		case *ssa.Convert:
			v = value.X
		case *ssa.ChangeType:
			v = value.X
		case *ssa.UnOp:
			if value.Op != token.MUL {
				return v
			}
			v = value.X
		default:
			return v
		}
	}
}

func isSSASet(call ssa.CallInstruction) bool {
	callee := call.Common().StaticCallee()
	if callee != nil {
		if origin := callee.Origin(); origin != nil {
			callee = origin
		}
	}
	return callee != nil && callee.Pkg != nil && callee.Pkg.Pkg.Path() == gimblePath && (callee.Name() == "Set" || callee.Name() == "SetJSON")
}

func ssaString(value ssa.Value) (string, bool) {
	value = stripSSA(value)
	c, ok := value.(*ssa.Const)
	if !ok || c.Value == nil || c.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(c.Value), true
}

func gimbleCall(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	callee := typeutil.Callee(pass.TypesInfo, call)
	if callee == nil || callee.Pkg() == nil || callee.Pkg().Path() != gimblePath {
		return "", false
	}
	return callee.Name(), true
}

func isSetCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	name, ok := gimbleCall(pass, call)
	return ok && (name == "Set" || name == "SetJSON")
}

func isWorkflowOperation(name string) bool {
	switch name {
	case "Run", "Scope", "Group", "Go", "Loop", "Tasks", "NewSession", "Fork", "Generate", "Set", "SetJSON":
		return true
	default:
		return false
	}
}

func callbackArgument(name string) (int, bool) {
	switch name {
	case "Run", "Scope":
		return 2, true
	case "Go":
		return 1, true
	default:
		return 0, false
	}
}

func firstParam(pass *analysis.Pass, typ *ast.FuncType) types.Object {
	if typ.Params == nil || len(typ.Params.List) == 0 || len(typ.Params.List[0].Names) == 0 {
		return nil
	}
	return pass.TypesInfo.Defs[typ.Params.List[0].Names[0]]
}

func isTasksRange(pass *analysis.Pass, stmt *ast.RangeStmt) bool {
	sel, ok := unparen(stmt.X).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Tasks" {
		return false
	}
	selection := pass.TypesInfo.Selections[sel]
	if selection == nil {
		return false
	}
	obj := selection.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == gimblePath && obj.Name() == "Tasks"
}

func identObject(pass *analysis.Pass, expr ast.Expr) types.Object {
	id, ok := unparen(expr).(*ast.Ident)
	if !ok {
		return nil
	}
	if obj := pass.TypesInfo.Uses[id]; obj != nil {
		return obj
	}
	return pass.TypesInfo.Defs[id]
}

func constantString(pass *analysis.Pass, expr ast.Expr) string {
	value := pass.TypesInfo.Types[expr].Value
	if value == nil || value.Kind() != constant.String {
		return ""
	}
	return constant.StringVal(value)
}

func isBackground(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}
	callee := typeutil.Callee(pass.TypesInfo, call)
	return callee != nil && callee.Pkg() != nil && callee.Pkg().Path() == "context" && (callee.Name() == "Background" || callee.Name() == "TODO")
}

func isCollectionFunction(pass *analysis.Pass, expr ast.Expr) bool {
	expr = unparen(expr)
	switch expr := expr.(type) {
	case *ast.IndexExpr:
		// Generic instantiation also uses IndexExpr. Only an indexed result
		// whose type itself is callable is worker selection.
		return isFunctionType(pass.TypesInfo.TypeOf(expr)) && !isGenericInstantiation(pass, expr)
	case *ast.IndexListExpr:
		return isFunctionType(pass.TypesInfo.TypeOf(expr)) && !isGenericInstantiation(pass, expr)
	default:
		return false
	}
}

func isGenericInstantiation(pass *analysis.Pass, expr ast.Expr) bool {
	var base ast.Expr
	switch expr := expr.(type) {
	case *ast.IndexExpr:
		base = expr.X
	case *ast.IndexListExpr:
		base = expr.X
	default:
		return false
	}
	switch base := base.(type) {
	case *ast.Ident:
		_, ok := pass.TypesInfo.Instances[base]
		return ok
	case *ast.SelectorExpr:
		_, ok := pass.TypesInfo.Instances[base.Sel]
		return ok
	default:
		return false
	}
}

func isFunctionType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Signature)
	return ok
}

func unparen[T ast.Expr](expr T) ast.Expr {
	var current ast.Expr = expr
	for {
		paren, ok := current.(*ast.ParenExpr)
		if !ok {
			return current
		}
		current = paren.X
	}
}
