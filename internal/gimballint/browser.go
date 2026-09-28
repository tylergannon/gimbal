package gimballint

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

const browserEscape = "[GIMBAL110-BROWSER/ESCAPE]: a *gimbal.Browser must not outlive the scope that created it; keep it in that scope's body and pass it to calls or WithBrowser."

// browserCheck carries GIMBAL110's per-package state. A browser value is any
// expression whose type holds *gimbal.Browser. An option value is an
// AgentOption, or a slice or array of them, whose local flow reaches back to
// a browser through WithBrowser, another Gimbal option constructor, a slice
// literal, append or an alias. Each tracked value has an owner: a function
// body, or the body of an Iterate or PromiseLoop.Tasks range. The check is
// local and flow-insensitive; it does not follow values through helpers.
type browserCheck struct {
	pass *analysis.Pass
	si   *syntaxInfo

	// bodyParent is the enclosing owner body of each owner body, nil at the
	// top of a package-level function.
	bodyParent map[*ast.BlockStmt]*ast.BlockStmt
	// declBody is the owner body that declares each local variable.
	declBody map[types.Object]*ast.BlockStmt
	// owner is each tracked variable's owner: for a browser-typed variable,
	// the innermost owner of any value assigned to it, else its declaring
	// body; for an option variable, the innermost owner of the tracked
	// values assigned to it. An untracked option variable has none.
	owner map[types.Object]*ast.BlockStmt
	flows []browserFlow
	// groups holds the owner body of each variable whose only value is a
	// gimbal.Group result.
	groups map[types.Object]*ast.BlockStmt

	holds    map[types.Type]bool
	reported map[token.Pos]bool
}

// browserFlow is one value reaching a variable: an assignment, a declaration,
// a store into a local option slice's element, or a range variable. index is
// the value's position in a multi-value expression, or -1.
type browserFlow struct {
	target types.Object
	value  ast.Expr
	index  int
	ranged bool
}

func reportBrowserEscapes(pass *analysis.Pass, si *syntaxInfo) {
	// The runtime itself keeps browsers in its scopes and options.
	if pass.Pkg.Path() == gimbalPath {
		return
	}
	c := &browserCheck{
		pass:       pass,
		si:         si,
		bodyParent: make(map[*ast.BlockStmt]*ast.BlockStmt),
		declBody:   make(map[types.Object]*ast.BlockStmt),
		owner:      make(map[types.Object]*ast.BlockStmt),
		groups:     make(map[types.Object]*ast.BlockStmt),
		holds:      make(map[types.Type]bool),
		reported:   make(map[token.Pos]bool),
	}
	c.indexBodies()
	c.indexFlows()
	c.resolveOwners()
	c.check()
}

func (c *browserCheck) report(pos token.Pos) {
	if !c.reported[pos] {
		c.reported[pos] = true
		c.pass.Reportf(pos, "%s", browserEscape)
	}
}

// indexBodies records every owner body and the body that declares every
// local variable. Parameters and results belong to their function's body,
// and variables a lifetime range defines belong to the range body.
func (c *browserCheck) indexBodies() {
	params := func(body *ast.BlockStmt, lists ...*ast.FieldList) {
		for _, list := range lists {
			if list == nil {
				continue
			}
			for _, field := range list.List {
				for _, name := range field.Names {
					if obj := c.pass.TypesInfo.Defs[name]; obj != nil {
						c.declBody[obj] = body
					}
				}
			}
		}
	}
	var bodies []ast.Node
	for _, file := range c.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					c.bodyParent[n.Body] = nil
					bodies = append(bodies, n)
					params(n.Body, n.Recv, n.Type.Params, n.Type.Results)
				}
			case *ast.FuncLit:
				c.bodyParent[n.Body] = nil
				bodies = append(bodies, n)
				params(n.Body, n.Type.Params, n.Type.Results)
			case *ast.RangeStmt:
				if _, ok := scopedRange(c.pass, n); ok {
					c.bodyParent[n.Body] = nil
					bodies = append(bodies, n)
					if n.Tok == token.DEFINE {
						for _, e := range []ast.Expr{n.Key, n.Value} {
							if obj := identObject(c.pass, e); obj != nil {
								c.declBody[obj] = n.Body
							}
						}
					}
				}
			}
			return true
		})
	}
	for _, n := range bodies {
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		case *ast.RangeStmt:
			body = n.Body
		}
		c.bodyParent[body] = c.enclosingBody(c.si.parents[n])
	}
	// A type switch's clause variables are implicit, one per clause.
	for node, obj := range c.pass.TypesInfo.Implicits {
		if clause, ok := node.(*ast.CaseClause); ok {
			c.declBody[obj] = c.enclosingBody(clause)
		}
	}
	for id, obj := range c.pass.TypesInfo.Defs {
		if _, ok := obj.(*types.Var); !ok || obj.Parent() == c.pass.Pkg.Scope() {
			continue
		}
		if _, ok := c.declBody[obj]; !ok {
			c.declBody[obj] = c.enclosingBody(id)
		}
	}
}

// enclosingBody is the innermost owner body containing n, or n itself.
func (c *browserCheck) enclosingBody(n ast.Node) *ast.BlockStmt {
	for current := n; current != nil; current = c.si.parents[current] {
		if block, ok := current.(*ast.BlockStmt); ok {
			if _, isBody := c.bodyParent[block]; isBody {
				return block
			}
		}
	}
	return nil
}

// within reports whether inner is outer or a body nested in it.
func (c *browserCheck) within(inner, outer *ast.BlockStmt) bool {
	for body := inner; body != nil; body = c.bodyParent[body] {
		if body == outer {
			return true
		}
	}
	return false
}

func (c *browserCheck) depth(body *ast.BlockStmt) int {
	n := 0
	for ; body != nil; body = c.bodyParent[body] {
		n++
	}
	return n
}

// inner is the shorter-lived of two owners; nil is no owner.
func (c *browserCheck) inner(a, b *ast.BlockStmt) *ast.BlockStmt {
	if a == nil || (b != nil && c.depth(b) > c.depth(a)) {
		return b
	}
	return a
}

func (c *browserCheck) indexFlows() {
	for _, file := range c.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for i, lhs := range n.Lhs {
					value, index := n.Rhs[0], i
					if len(n.Lhs) == len(n.Rhs) {
						value, index = n.Rhs[i], -1
					}
					if target := c.flowTarget(lhs); target != nil {
						c.flows = append(c.flows, browserFlow{target: target, value: value, index: index})
					}
				}
			case *ast.ValueSpec:
				for i, name := range n.Names {
					obj := c.pass.TypesInfo.Defs[name]
					if obj == nil || len(n.Values) == 0 {
						continue
					}
					value, index := n.Values[0], i
					if len(n.Names) == len(n.Values) {
						value, index = n.Values[i], -1
					}
					c.flows = append(c.flows, browserFlow{target: obj, value: value, index: index})
				}
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if obj := identObject(c.pass, e); obj != nil {
						c.flows = append(c.flows, browserFlow{target: obj, value: n.X, index: -1, ranged: true})
					}
				}
			}
			return true
		})
	}

	// A group variable is known only when its one value is a Group result.
	values := make(map[types.Object][]browserFlow)
	for _, flow := range c.flows {
		values[flow.target] = append(values[flow.target], flow)
	}
	for obj, flows := range values {
		if len(flows) != 1 || flows[0].index != -1 {
			continue
		}
		if call, ok := unparen(flows[0].value).(*ast.CallExpr); ok && c.isGroupCall(call) {
			c.groups[obj] = c.declBody[obj]
		}
	}
}

// flowTarget is the local variable an assignment to lhs changes: the
// variable itself, or a local option slice whose element it stores.
func (c *browserCheck) flowTarget(lhs ast.Expr) types.Object {
	lhs = unparen(lhs)
	if index, ok := lhs.(*ast.IndexExpr); ok {
		obj := identObject(c.pass, index.X)
		if obj != nil && c.isOptionType(obj.Type()) {
			return obj
		}
		return nil
	}
	obj := identObject(c.pass, lhs)
	if _, ok := obj.(*types.Var); !ok {
		return nil
	}
	return obj
}

// resolveOwners computes each variable's owner. A browser-typed variable
// no value reaches, such as a parameter, starts at its declaring body; any
// other starts with none. Owners only move inward, so repeating until nothing
// changes terminates.
func (c *browserCheck) resolveOwners() {
	reached := make(map[types.Object]bool)
	for _, flow := range c.flows {
		reached[flow.target] = true
	}
	for obj, body := range c.declBody {
		if !reached[obj] && c.holdsBrowser(obj.Type()) {
			c.owner[obj] = body
		}
	}
	for changed := true; changed; {
		changed = false
		for _, flow := range c.flows {
			if _, local := c.declBody[flow.target]; !local {
				continue
			}
			owner := c.flowOwner(flow, nil)
			if owner == nil {
				continue
			}
			merged := c.inner(c.owner[flow.target], owner)
			if merged != c.owner[flow.target] {
				c.owner[flow.target] = merged
				changed = true
			}
		}
	}
	for obj, body := range c.declBody {
		if c.owner[obj] == nil && c.holdsBrowser(obj.Type()) {
			c.owner[obj] = body
		}
	}
}

// flowOwner is the owner of the value a flow carries into its target, not
// counting the target variable's own value.
func (c *browserCheck) flowOwner(flow browserFlow, exclude types.Object) *ast.BlockStmt {
	if flow.ranged {
		if c.holdsBrowser(flow.target.Type()) || c.isOptionType(flow.target.Type()) {
			return c.valueOwner(flow.value, exclude)
		}
		return nil
	}
	if flow.index >= 0 {
		// One element of a multi-value call: its results belong to the
		// body containing the call. Options from a call are opaque.
		tuple, ok := c.pass.TypesInfo.TypeOf(flow.value).(*types.Tuple)
		if ok && flow.index < tuple.Len() && c.holdsBrowser(tuple.At(flow.index).Type()) {
			return c.enclosingBody(flow.value)
		}
		return nil
	}
	return c.valueOwner(flow.value, exclude)
}

// valueOwner is the owner of a browser or option value, nil when e carries
// no browser the analysis can see.
func (c *browserCheck) valueOwner(e ast.Expr, exclude types.Object) *ast.BlockStmt {
	e = unparen(e)
	t := c.pass.TypesInfo.TypeOf(e)
	if t == nil {
		return nil
	}
	if c.holdsBrowser(t) {
		// Reading a local keeps its owner, so a fresh alias of an outer
		// browser is still the outer scope's. Anything else, such as a
		// helper's result, belongs to the body that holds it.
		if base := c.baseVariable(e); base != nil {
			if base == exclude {
				return nil
			}
			return c.owner[base]
		}
		return c.enclosingBody(e)
	}
	if !c.isOptionType(t) {
		return nil
	}
	switch e := e.(type) {
	case *ast.Ident, *ast.IndexExpr, *ast.SliceExpr:
		if base := c.baseVariable(e); base != nil && base != exclude {
			return c.owner[base]
		}
	case *ast.CompositeLit:
		var owner *ast.BlockStmt
		for _, elt := range e.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				elt = kv.Value
			}
			owner = c.inner(owner, c.valueOwner(elt, exclude))
		}
		return owner
	case *ast.CallExpr:
		if tv := c.pass.TypesInfo.Types[e.Fun]; tv.IsType() && len(e.Args) == 1 {
			return c.valueOwner(e.Args[0], exclude)
		}
		if c.isBuiltin(e, "append") || c.isOptionConstructor(e) {
			var owner *ast.BlockStmt
			for _, arg := range e.Args {
				owner = c.inner(owner, c.valueOwner(arg, exclude))
			}
			return owner
		}
	}
	return nil
}

// baseVariable is the local variable e reads through selectors, indexes,
// slices and dereferences, or nil.
func (c *browserCheck) baseVariable(e ast.Expr) types.Object {
	for {
		switch x := unparen(e).(type) {
		case *ast.Ident:
			obj, ok := c.pass.TypesInfo.Uses[x].(*types.Var)
			if !ok {
				return nil
			}
			if _, local := c.declBody[obj]; !local {
				return nil
			}
			return obj
		case *ast.SelectorExpr:
			if _, isField := c.pass.TypesInfo.Selections[x]; !isField {
				return nil
			}
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// isOptionConstructor reports a call to a Gimbal function returning an
// AgentOption, such as WithBrowser or WithSupervisor: the option it returns
// carries its browser and option arguments.
func (c *browserCheck) isOptionConstructor(call *ast.CallExpr) bool {
	fn, ok := typeutil.Callee(c.pass.TypesInfo, call).(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != gimbalPath {
		return false
	}
	sig := fn.Type().(*types.Signature)
	return sig.Results().Len() == 1 && c.isOption(sig.Results().At(0).Type())
}

func (c *browserCheck) isGroupCall(call *ast.CallExpr) bool {
	fn, ok := typeutil.Callee(c.pass.TypesInfo, call).(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == gimbalPath && fn.Name() == "Group" && fn.Type().(*types.Signature).Recv() == nil
}

func (c *browserCheck) isBuiltin(call *ast.CallExpr, name string) bool {
	id, ok := unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := c.pass.TypesInfo.Uses[id].(*types.Builtin)
	return ok && builtin.Name() == name
}

// holdsBrowser reports whether t is, or contains, *gimbal.Browser: through
// pointers, slices, arrays, maps, channels, struct fields and function
// results. Other Gimbal types are opaque.
func (c *browserCheck) holdsBrowser(t types.Type) bool {
	if t == nil {
		return false
	}
	if held, ok := c.holds[t]; ok {
		return held
	}
	c.holds[t] = false // a recursive type holds a browser only through another part
	var held bool
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		if obj := t.Obj(); obj.Pkg() != nil && obj.Pkg().Path() == gimbalPath {
			held = obj.Name() == "Browser"
		} else {
			held = c.holdsBrowser(t.Underlying())
		}
	case *types.Pointer:
		held = c.holdsBrowser(t.Elem())
	case *types.Slice:
		held = c.holdsBrowser(t.Elem())
	case *types.Array:
		held = c.holdsBrowser(t.Elem())
	case *types.Chan:
		held = c.holdsBrowser(t.Elem())
	case *types.Map:
		held = c.holdsBrowser(t.Key()) || c.holdsBrowser(t.Elem())
	case *types.Struct:
		for field := range t.Fields() {
			held = held || c.holdsBrowser(field.Type())
		}
	case *types.Signature:
		held = c.holdsBrowser(t.Results())
	case *types.Tuple:
		for v := range t.Variables() {
			held = held || c.holdsBrowser(v.Type())
		}
	}
	c.holds[t] = held
	return held
}

func (c *browserCheck) isOption(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == gimbalPath && named.Obj().Name() == "AgentOption"
}

// isOptionType reports an AgentOption or a slice or array of them.
func (c *browserCheck) isOptionType(t types.Type) bool {
	switch u := types.Unalias(t).(type) {
	case *types.Slice:
		return c.isOption(u.Elem())
	case *types.Array:
		return c.isOption(u.Elem())
	}
	return c.isOption(t)
}

// carried reports whether e is a browser value or a tracked option value.
func (c *browserCheck) carried(e ast.Expr) bool {
	t := c.pass.TypesInfo.TypeOf(e)
	if t == nil {
		return false
	}
	if c.holdsBrowser(t) {
		return true
	}
	return c.isOptionType(t) && c.valueOwner(e, nil) != nil
}

// trackedVar reports a local browser-typed or tracked option variable.
func (c *browserCheck) trackedVar(obj types.Object) bool {
	if _, local := c.declBody[obj]; !local {
		return false
	}
	return c.holdsBrowser(obj.Type()) || c.owner[obj] != nil
}

func (c *browserCheck) check() {
	for _, file := range c.pass.Files {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				for _, spec := range gen.Specs {
					for _, name := range spec.(*ast.ValueSpec).Names {
						if obj := c.pass.TypesInfo.Defs[name]; obj != nil && c.holdsBrowser(obj.Type()) {
							c.report(name.Pos()) // rule 6: a package variable
						}
					}
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.StructType:
				for _, field := range n.Fields.List {
					if c.holdsBrowser(c.pass.TypesInfo.TypeOf(field.Type)) {
						c.report(field.Type.Pos()) // rule 6: a struct field
					}
				}
			case *ast.AssignStmt:
				c.checkAssign(n)
			case *ast.ValueSpec:
				if n.Type != nil && types.IsInterface(c.pass.TypesInfo.TypeOf(n.Type)) {
					for _, value := range n.Values {
						c.checkInterface(value)
					}
				}
			case *ast.SendStmt:
				if c.carried(n.Value) {
					c.report(n.Value.Pos()) // rule 2: a channel send
				}
			case *ast.CallExpr:
				c.checkCall(n)
			case *ast.ReturnStmt:
				c.checkReturn(n)
			case *ast.CompositeLit:
				c.checkComposite(n)
			case *ast.FuncLit:
				c.checkCaptures(n)
			case *ast.GoStmt:
				c.checkGo(n)
			}
			return true
		})
	}
}

// checkAssign applies rules 1-3 to each assignment of a browser or tracked
// option value.
func (c *browserCheck) checkAssign(n *ast.AssignStmt) {
	for i, lhs := range n.Lhs {
		lhs = unparen(lhs)
		if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
			continue
		}
		var flow browserFlow
		if len(n.Lhs) == len(n.Rhs) {
			flow = browserFlow{value: n.Rhs[i], index: -1}
			if !c.carried(n.Rhs[i]) {
				if index, ok := lhs.(*ast.IndexExpr); ok && c.holdsBrowser(c.pass.TypesInfo.TypeOf(index.Index)) {
					c.report(index.Index.Pos()) // rule 2: a map key
				}
				continue
			}
			if types.IsInterface(c.pass.TypesInfo.TypeOf(lhs)) {
				c.checkInterface(n.Rhs[i])
				continue
			}
		} else {
			flow = browserFlow{value: n.Rhs[0], index: i}
			if c.flowOwner(flow, nil) == nil {
				continue
			}
			if types.IsInterface(c.pass.TypesInfo.TypeOf(lhs)) {
				c.report(lhs.Pos()) // rule 3
				continue
			}
		}
		switch target := lhs.(type) {
		case *ast.Ident:
			obj := identObject(c.pass, target)
			body, local := c.declBody[obj]
			if !local {
				c.report(lhs.Pos()) // rule 1: a package variable
				continue
			}
			// The target's own value does not count: appending to an outer
			// slice in its owner is not an escape of what it already holds.
			if owner := c.flowOwner(flow, obj); owner != nil && !c.within(body, owner) {
				c.report(lhs.Pos()) // rule 1: an outer, captured or result variable
			}
		case *ast.IndexExpr:
			obj := c.flowTarget(target)
			if obj == nil || flow.index >= 0 || c.holdsBrowser(c.pass.TypesInfo.TypeOf(n.Rhs[i])) {
				c.report(lhs.Pos()) // rule 2: an index or map element
				continue
			}
			if owner := c.flowOwner(flow, obj); owner != nil && !c.within(c.declBody[obj], owner) {
				c.report(lhs.Pos()) // rule 1: an outer option slice
			}
		default:
			c.report(lhs.Pos()) // rule 2: a field, *p, or a package variable
		}
	}
}

// checkInterface reports a browser or tracked option value converted to an
// interface (rule 3): its type is lost past that point.
func (c *browserCheck) checkInterface(value ast.Expr) {
	t := c.pass.TypesInfo.TypeOf(value)
	if t != nil && !types.IsInterface(t) && c.carried(value) {
		c.report(value.Pos())
	}
}

func (c *browserCheck) checkCall(call *ast.CallExpr) {
	if tv := c.pass.TypesInfo.Types[call.Fun]; tv.IsType() {
		if types.IsInterface(tv.Type) && len(call.Args) == 1 {
			c.checkInterface(call.Args[0])
		}
		return
	}
	if c.isBuiltin(call, "append") {
		for _, arg := range call.Args[1:] {
			if c.holdsBrowser(c.pass.TypesInfo.TypeOf(arg)) {
				c.report(arg.Pos()) // rule 2: append
			}
		}
		return
	}
	sig, ok := types.Unalias(c.pass.TypesInfo.TypeOf(call.Fun)).(*types.Signature)
	if !ok {
		return
	}
	// sig is instantiated: a type parameter inferred as *gimbal.Browser keeps
	// the type, and one instantiated with an interface converts to it.
	for i, arg := range call.Args {
		if param := parameterType(sig, i, call.Ellipsis.IsValid()); param != nil && types.IsInterface(param) {
			c.checkInterface(arg)
		}
	}
}

func parameterType(sig *types.Signature, i int, spread bool) types.Type {
	params := sig.Params()
	if params.Len() == 0 {
		return nil
	}
	if sig.Variadic() && i >= params.Len()-1 {
		last := params.At(params.Len() - 1).Type()
		if spread {
			return last
		}
		if slice, ok := last.Underlying().(*types.Slice); ok {
			return slice.Elem()
		}
		return nil
	}
	if i >= params.Len() {
		return nil
	}
	return params.At(i).Type()
}

// checkReturn reports a returned value converted to an interface, and one
// whose owner is a lifetime range inside the returning function: the return
// carries it out of the range's scope.
func (c *browserCheck) checkReturn(ret *ast.ReturnStmt) {
	sig, body := c.enclosingFunction(ret)
	if sig == nil {
		return
	}
	for i, result := range ret.Results {
		if len(ret.Results) == sig.Results().Len() && types.IsInterface(sig.Results().At(i).Type()) {
			c.checkInterface(result)
			continue
		}
		var owner *ast.BlockStmt
		if tuple, ok := c.pass.TypesInfo.TypeOf(result).(*types.Tuple); ok {
			if c.holdsBrowser(tuple) {
				owner = c.enclosingBody(result)
			}
		} else if c.carried(result) {
			owner = c.valueOwner(result, nil)
		}
		// A return outside the owner carries a variable that already
		// escaped; that assignment is the one reported.
		if owner != nil && owner != body && c.within(owner, body) && c.within(c.enclosingBody(ret), owner) {
			c.report(result.Pos())
		}
	}
}

func (c *browserCheck) enclosingFunction(n ast.Node) (*types.Signature, *ast.BlockStmt) {
	for current := c.si.parents[n]; current != nil; current = c.si.parents[current] {
		switch fn := current.(type) {
		case *ast.FuncLit:
			sig, _ := c.pass.TypesInfo.TypeOf(fn).(*types.Signature)
			return sig, fn.Body
		case *ast.FuncDecl:
			obj, _ := c.pass.TypesInfo.Defs[fn.Name].(*types.Func)
			if obj == nil {
				return nil, nil
			}
			return obj.Type().(*types.Signature), fn.Body
		}
	}
	return nil, nil
}

// checkComposite reports browser values stored as elements or fields (rule
// 2) and as interfaces (rule 3). A tracked option is allowed as an element
// of an option slice or array literal: the literal carries its owner.
func (c *browserCheck) checkComposite(lit *ast.CompositeLit) {
	t := c.pass.TypesInfo.TypeOf(lit)
	if t == nil {
		return
	}
	aggregate := c.isOptionType(t) && !c.isOption(t)
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if _, isMap := t.Underlying().(*types.Map); isMap && c.carried(kv.Key) {
				c.report(kv.Key.Pos())
			}
			elt = kv.Value
		}
		if !c.carried(elt) || (aggregate && !c.holdsBrowser(c.pass.TypesInfo.TypeOf(elt))) {
			continue
		}
		c.report(elt.Pos())
	}
}

// checkCaptures reports a function literal's references to tracked
// variables declared outside it (rule 4), unless the literal is a Scope
// callback, or the Go callback of a group whose owner body is the
// variable's owner or nested in it.
func (c *browserCheck) checkCaptures(lit *ast.FuncLit) {
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj, ok := c.pass.TypesInfo.Uses[id].(*types.Var)
		if !ok || !c.trackedVar(obj) || c.within(c.declBody[obj], lit.Body) {
			return true
		}
		if !c.allowedCapture(lit, obj) {
			c.report(id.Pos())
		}
		return true
	})
}

func (c *browserCheck) allowedCapture(lit *ast.FuncLit, obj types.Object) bool {
	call, ok := c.si.parents[lit].(*ast.CallExpr)
	if !ok {
		return false
	}
	name, ok := gimbalCall(c.pass, call)
	if !ok {
		return false
	}
	switch name {
	case "Scope":
		return len(call.Args) > 2 && call.Args[2] == lit
	case "Go":
		sel, ok := unparen(call.Fun).(*ast.SelectorExpr)
		if !ok || len(call.Args) < 2 || call.Args[1] != lit {
			return false
		}
		var group *ast.BlockStmt
		if receiver, ok := unparen(sel.X).(*ast.CallExpr); ok && c.isGroupCall(receiver) {
			group = c.enclosingBody(receiver)
		} else if v := identObject(c.pass, sel.X); v != nil {
			group = c.groups[v]
		}
		owner := c.owner[obj]
		if owner == nil {
			owner = c.declBody[obj]
		}
		return group != nil && c.within(group, owner)
	}
	return false
}

// checkGo reports every browser or tracked option reference inside a raw go
// statement (rule 5): nothing joins it to the owner scope.
func (c *browserCheck) checkGo(stmt *ast.GoStmt) {
	ast.Inspect(stmt.Call, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if obj, ok := c.pass.TypesInfo.Uses[n].(*types.Var); ok && c.trackedVar(obj) {
				c.report(n.Pos())
			}
		case *ast.CallExpr:
			if c.holdsBrowser(c.pass.TypesInfo.TypeOf(n)) {
				c.report(n.Pos())
			}
		}
		return true
	})
}
