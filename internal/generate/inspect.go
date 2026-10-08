package generate

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal/workflow"
	"golang.org/x/tools/go/types/typeutil"
)

// Inspection is a sidecar to the existing graph extraction. Runtime graphs and
// their generated codecs do not need source expressions or context projections.
type inspectionData struct {
	Annotations []diagramAnnotation
	Module      string                               `json:"module"`
	Calls       map[workflow.Source][]callInspection `json:"-"`
	Controls    map[controlKey][]*controlInspection
	Services    map[*workflow.Command]callInspection
}
type controlKey struct {
	Source workflow.Source
	Kind   string
}
type controlInspection struct {
	Code    string   `json:"code"`
	Actions []string `json:"actions,omitempty"`
	key     controlKey
}

func (e *extractor) inspectControl(kind string, node ast.Node) *controlInspection {
	if e.inspection == nil {
		return nil
	}
	if e.inspection.Controls == nil {
		e.inspection.Controls = map[controlKey][]*controlInspection{}
	}
	key := controlKey{Source: e.at(node.Pos()), Kind: kind}
	info := &controlInspection{Code: text(e.pkg.Fset, node), key: key}
	e.inspection.Controls[key] = append(e.inspection.Controls[key], info)
	return info
}
func (e *extractor) discardControl(info *controlInspection) {
	if info != nil {
		e.inspection.Controls[info.key] = slices.DeleteFunc(e.inspection.Controls[info.key], func(x *controlInspection) bool { return x == info })
	}
}

type callInspection struct {
	Kind              string      `json:"kind"`
	Expression        string      `json:"expression"`
	PromptExpression  string      `json:"promptExpression,omitempty"`
	PromptKnown       bool        `json:"promptKnown"`
	KeyExpression     string      `json:"keyExpression,omitempty"`
	KeyKnown          bool        `json:"keyKnown"`
	ValueExpression   string      `json:"valueExpression,omitempty"`
	ContextExpression string      `json:"contextExpression,omitempty"`
	ContextKnown      bool        `json:"contextKnown"`
	Shape             *valueShape `json:"shape,omitempty"`
}
type valueShape struct {
	Type    string       `json:"type"`
	Kind    string       `json:"kind"`
	Fields  []shapeField `json:"fields,omitempty"`
	Element *valueShape  `json:"element,omitempty"`
	Note    string       `json:"note,omitempty"`
	GoOnly  bool         `json:"goOnly,omitempty"`
}
type shapeField struct {
	Name     string      `json:"name"`
	Shape    *valueShape `json:"shape"`
	Optional bool        `json:"optional,omitempty"`
	Tag      string      `json:"tag,omitempty"`
}

func (e *extractor) inspectCall(kind string, call *ast.CallExpr) {
	if e.inspection == nil {
		return
	}
	d := callInspection{Kind: kind, Expression: text(e.pkg.Fset, call)}
	if len(call.Args) > 0 {
		d.ContextExpression = text(e.pkg.Fset, call.Args[0])
		d.ContextKnown = e.inspectionContext != nil && e.object(call.Args[0]) == e.inspectionContext
		if !d.ContextKnown {
			e.diag(call.Pos(), "context expression %s is not the current scope parameter; its context shape is unresolved", d.ContextExpression)
		}
	}
	if kind == "Generate" {
		_, d.PromptKnown = e.constant(call, 1)
		if len(call.Args) > 1 {
			d.PromptExpression = text(e.pkg.Fset, call.Args[1])
		}
	} else {
		_, d.KeyKnown = e.constant(call, 1)
		if len(call.Args) > 1 {
			d.KeyExpression = text(e.pkg.Fset, call.Args[1])
		}
		if len(call.Args) > 2 {
			d.ValueExpression = text(e.pkg.Fset, call.Args[2])
			d.Shape = staticShape(e.pkg.TypesInfo.TypeOf(call.Args[2]), map[types.Type]bool{})
		}
	}
	if e.inspection.Calls == nil {
		e.inspection.Calls = map[workflow.Source][]callInspection{}
	}
	src := e.at(call.Pos())
	e.inspection.Calls[src] = append(e.inspection.Calls[src], d)
}

// Read shape from go/types. Never execute Schema, MarshalJSON, or workflow code.
func staticShape(t types.Type, seen map[types.Type]bool) *valueShape {
	if t == nil {
		return &valueShape{Type: "unknown", Kind: "unknown", Note: "No static type available."}
	}
	name := types.TypeString(t, func(p *types.Package) string { return p.Name() })
	s := &valueShape{Type: name, Kind: "scalar"}
	if seen[t] {
		s.Kind = "reference"
		return s
	}
	seen = maps.Clone(seen)
	seen[t] = true
	// A custom marshaler makes the Go fields useful but does not establish JSON.
	for _, candidate := range []types.Type{t, types.NewPointer(t)} {
		methods := types.NewMethodSet(candidate)
		for i := 0; i < methods.Len(); i++ {
			n := methods.At(i).Obj().Name()
			if n == "MarshalJSON" || n == "MarshalText" {
				s.GoOnly = true
				s.Note = "Go shape only: custom " + n + " serialization is unresolved."
			}
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		s.Kind = "pointer"
		s.Element = staticShape(u.Elem(), seen)
	case *types.Slice:
		s.Kind = "array"
		s.Element = staticShape(u.Elem(), seen)
	case *types.Array:
		s.Kind = "array"
		s.Element = staticShape(u.Elem(), seen)
	case *types.Map:
		s.Kind = "map"
		s.Element = staticShape(u.Elem(), seen)
		s.Note = "Map keys are runtime data; key type: " + types.TypeString(u.Key(), nil) + "."
	case *types.Interface:
		s.Kind = "unknown"
		s.Note = "Dynamic value: nested fields cannot be established from this interface."
	case *types.Struct:
		s.Kind = "object"
		names := map[string]bool{}
		for i := 0; i < u.NumFields(); i++ {
			f := u.Field(i)
			if !f.Exported() {
				continue
			}
			tag := jsonTag(u.Tag(i))
			parts := strings.Split(tag, ",")
			if parts[0] == "-" {
				continue
			}
			fieldName := f.Name()
			if parts[0] != "" {
				fieldName = parts[0]
			}
			if f.Anonymous() {
				s.GoOnly = true
				s.Note = "Go shape only: embedded-field JSON promotion is unresolved."
			}
			if names[fieldName] {
				s.GoOnly = true
				s.Note = "Go shape only: conflicting JSON field names are unresolved."
			}
			names[fieldName] = true
			fs := staticShape(f.Type(), seen)
			if slices.Contains(parts[1:], "string") {
				fs.Note = "JSON tag requests string encoding. " + fs.Note
			}
			s.Fields = append(s.Fields, shapeField{Name: fieldName, Shape: fs, Optional: slices.Contains(parts[1:], "omitempty") || slices.Contains(parts[1:], "omitzero"), Tag: tag})
		}
	}
	if s.GoOnly {
		if u, ok := t.Underlying().(*types.Struct); ok {
			s.Fields = nil
			for i := 0; i < u.NumFields(); i++ {
				f := u.Field(i)
				if f.Exported() {
					s.Fields = append(s.Fields, shapeField{Name: f.Name(), Shape: staticShape(f.Type(), seen), Tag: jsonTag(u.Tag(i))})
				}
			}
		}
	}
	return s
}

// Minimal struct-tag reader, without runtime reflection.
func jsonTag(tag string) string {
	for tag != "" {
		tag = strings.TrimLeft(tag, " ")
		colon := strings.IndexByte(tag, ':')
		if colon < 1 {
			return ""
		}
		key := tag[:colon]
		tag = tag[colon+1:]
		if len(tag) == 0 || tag[0] != '"' {
			return ""
		}
		i := 1
		for i < len(tag) {
			if tag[i] == '\\' {
				i += 2
				continue
			}
			if tag[i] == '"' {
				break
			}
			i++
		}
		if i >= len(tag) {
			return ""
		}
		val, err := strconv.Unquote(tag[:i+1])
		if err != nil {
			return ""
		}
		tag = tag[i+1:]
		if key == "json" {
			return val
		}
	}
	return ""
}

type contextField struct {
	Key      string      `json:"key"`
	Dynamic  bool        `json:"dynamic,omitempty"`
	Shape    *valueShape `json:"shape"`
	Write    string      `json:"write"`
	Scope    string      `json:"scope"`
	When     string      `json:"when,omitempty"`
	Optional bool        `json:"optional,omitempty"`
	Shadowed []string    `json:"shadowed,omitempty"`
}
type viewNode struct {
	Title        string             `json:"title,omitempty"`
	Description  string             `json:"description,omitempty"`
	ID           string             `json:"id"`
	Kind         string             `json:"kind"`
	Label        string             `json:"label"`
	Source       workflow.Source    `json:"source"`
	Scope        string             `json:"scope"`
	Session      string             `json:"session,omitempty"`
	From         string             `json:"from,omitempty"`
	Prompt       string             `json:"prompt,omitempty"`
	Detail       *callInspection    `json:"detail,omitempty"`
	Control      *controlInspection `json:"control,omitempty"`
	Context      []contextField     `json:"context"`
	Children     [][]*viewNode      `json:"children,omitempty"`
	BranchLabels []string           `json:"branchLabels,omitempty"`
	BranchExits  []bool             `json:"branchExits,omitempty"`
	Note         string             `json:"note,omitempty"`
}
type sourcePage struct {
	Name        string                `json:"name"`
	Entry       string                `json:"entry"`
	Module      string                `json:"module"`
	Source      workflow.Source       `json:"source"`
	Body        []*viewNode           `json:"body"`
	Diagnostics []workflow.Diagnostic `json:"diagnostics"`
}
type pageBuilder struct {
	details        *inspectionData
	cursors        map[workflow.Source]int
	controlCursors map[controlKey]int
	next           int
}

func (b *pageBuilder) takeControl(kind string, src workflow.Source) *controlInspection {
	if b.controlCursors == nil {
		b.controlCursors = map[controlKey]int{}
	}
	key := controlKey{Source: src, Kind: kind}
	i := b.controlCursors[key]
	b.controlCursors[key]++
	if i >= len(b.details.Controls[key]) {
		return nil
	}
	return b.details.Controls[key][i]
}

func inspectGraph(g workflow.Graph, entry string, d *inspectionData) sourcePage {
	b := pageBuilder{details: d, cursors: map[workflow.Source]int{}}
	body, _ := b.body(g.Body, nil, "root", "")
	applyDiagramAnnotations(body, d.Annotations)
	p := sourcePage{Name: g.Name, Entry: entry, Module: d.Module, Source: g.Source, Body: body, Diagnostics: append([]workflow.Diagnostic{}, g.Diagnostics...)}
	return p
}
func (b *pageBuilder) take(src workflow.Source) *callInspection {
	i := b.cursors[src]
	b.cursors[src]++
	if i >= len(b.details.Calls[src]) {
		return nil
	}
	d := b.details.Calls[src][i]
	return &d
}
func (b *pageBuilder) node(kind, label, scope string, src workflow.Source, ctx []contextField) *viewNode {
	b.next++
	return &viewNode{ID: fmt.Sprintf("n%d", b.next), Kind: kind, Label: label, Source: src, Scope: scope, Context: slices.Clone(ctx)}
}
func putContext(ctx []contextField, v contextField) []contextField {
	out := make([]contextField, 0, len(ctx)+1)
	for _, old := range ctx {
		if !v.Dynamic && !old.Dynamic && old.Key == v.Key {
			if old.Scope != v.Scope {
				v.Shadowed = append(v.Shadowed, old.Write)
			}
			continue
		}
		out = append(out, old)
	}
	return append(out, v)
}
func conditionText(parent, cond string) string {
	if parent == "" {
		return cond
	}
	if cond == "" {
		return parent
	}
	return parent + " AND (" + cond + ")"
}

// Merge only non-exiting paths. Preserve each alternative's write site and
// condition; no child's fields enter its parent's context at a group join.
func mergeContexts(paths [][]contextField) []contextField {
	out := []contextField{}
	counts := map[string]int{}
	for _, path := range paths {
		pathKeys := map[string]bool{}
		for _, v := range path {
			if !slices.ContainsFunc(out, func(x contextField) bool { return x.Write == v.Write && x.When == v.When }) {
				out = append(out, v)
			}
			pathKeys[v.Key] = true
		}
		for key := range pathKeys {
			counts[key]++
		}
	}
	for i := range out {
		out[i].Optional = out[i].Optional || counts[out[i].Key] < len(paths)
	}
	return out
}
func (b *pageBuilder) body(ops []workflow.Operation, initial []contextField, scope, when string) ([]*viewNode, []contextField) {
	ctx := append([]contextField{}, initial...)
	nodes := []*viewNode{}
	for _, op := range ops {
		var n *viewNode
		switch op := op.(type) {
		case workflow.Set:
			d := b.take(op.Source)
			kind := "Set"
			label := op.Key
			if d != nil {
				kind = d.Kind
				if !d.KeyKnown {
					label = d.KeyExpression
				}
			}
			n = b.node(kind, label, scope, op.Source, ctx)
			n.Detail = d
			shape := &valueShape{Type: "unknown", Kind: "unknown"}
			dynamic := false
			if d != nil {
				shape = d.Shape
				dynamic = !d.KeyKnown
			}
			if d == nil || d.ContextKnown {
				ctx = putContext(ctx, contextField{Key: label, Dynamic: dynamic, Shape: shape, Write: n.ID, Scope: scope, When: when})
			}
			n.Context = slices.Clone(ctx)
		case workflow.AgentCall:
			d := b.take(op.Source)
			label := op.Session + ".Generate"
			if d != nil {
				label = d.PromptExpression
			}
			n = b.node("Generate", label, scope, op.Source, ctx)
			n.Detail = d
			n.Prompt = op.Prompt
			n.Session = op.Session
		case workflow.Session:
			kind := "NewSession"
			if op.From != "" {
				kind = "Fork"
			}
			n = b.node(kind, op.Name, scope, op.Source, ctx)
			n.From = op.From
			n.Session = op.Name
		case *workflow.Command:
			// Pointer markers exist only in inspector extraction, preserving a
			// service's source position without changing the runtime graph model.
			d := b.details.Services[op]
			n = b.node("Service", op.Name, scope, op.Source, ctx)
			n.Detail = &d
		case workflow.Command:
			n = b.node("Command", op.Name, scope, op.Source, ctx)
			n.Note = "Runs a command. Any context it writes is not shown here."
		case workflow.Interview:
			n = b.node("Interview", op.Name, scope, op.Source, ctx)
			n.Session = op.Session
		case workflow.Scope:
			n = b.node("Scope", op.Name, scope, op.Source, ctx)
			child, _ := b.body(op.Body, ctx, scope+"/"+op.Name, when)
			n.Children = [][]*viewNode{child}
		case workflow.Group:
			n = b.node("Group", op.Name, scope, op.Source, ctx)
			n.Note = "Children run concurrently in separate scopes. Child context does not merge at the join."
			for _, child := range op.Children {
				lane, _ := b.body(child.Body, ctx, scope+"/"+op.Name+"/"+child.Name, when)
				// The lane itself is inspectable even when its body is empty.
				c := b.node("Go", child.Name, scope+"/"+op.Name+"/"+child.Name, child.Source, ctx)
				c.Children = [][]*viewNode{lane}
				n.Children = append(n.Children, []*viewNode{c})
				n.BranchLabels = append(n.BranchLabels, child.Name)
			}
		case workflow.Condition:
			n = b.node("Condition", "if / switch", scope, op.Source, ctx)
			n.Control = b.takeControl("Condition", op.Source)
			paths := [][]contextField{}
			hasDefault := false
			previous := []string{}
			for _, br := range op.Branches {
				label := br.Case
				cond := label
				if label == "" {
					label = "else / default"
					hasDefault = true
					cond = "otherwise"
				} else if len(previous) > 0 {
					cond = "not (" + strings.Join(previous, " OR ") + ") AND (" + label + ")"
				}
				lane, next := b.body(br.Body, ctx, scope, conditionText(when, cond))
				n.Children = append(n.Children, lane)
				n.BranchLabels = append(n.BranchLabels, label)
				n.BranchExits = append(n.BranchExits, br.Exits)
				if !br.Exits {
					paths = append(paths, next)
				}
				previous = append(previous, br.Case)
			}
			if !hasDefault {
				paths = append(paths, ctx)
			}
			ctx = mergeContexts(paths)
		case workflow.Repeat:
			n = b.node("Repeat", op.Cond, scope, op.Source, ctx)
			n.Control = b.takeControl("Repeat", op.Source)
			child, next := b.body(op.Body, ctx, scope, conditionText(when, "loop body: "+op.Cond))
			n.Children = [][]*viewNode{child}
			n.Note = "Repeats this body. The number of iterations is decided when the workflow runs."
			ctx = mergeContexts([][]contextField{ctx, next})
		case workflow.Iterate:
			n = b.node("Iterate", op.Name, scope, op.Source, ctx)
			child, _ := b.body(op.Body, ctx, scope+"/"+op.Name, when)
			n.Children = [][]*viewNode{child}
			n.Note = "Runs this body for each item, in its own scope."
		case workflow.PromiseLoop:
			n = b.node("PromiseLoop", op.Name, scope, op.Source, ctx)
			n.Session = op.Planner
			child, _ := b.body(op.Body, ctx, scope+"/"+op.Name+"/task", when)
			n.Children = [][]*viewNode{child}
			n.Note = "Runs this body for each task chosen by the planner. Task data and planner feedback are known when the workflow runs."
		}
		if n != nil {
			if n.Detail != nil && !n.Detail.ContextKnown {
				n.Context = nil
				n.Note = "Uses a different context value, so the context visible here is not known."
			}
			nodes = append(nodes, n)
		}
	}
	return nodes, ctx
}

func sourceHTML(dir, entry, name, output string) error {
	d := &inspectionData{}
	g, _, err := extractInspected(dir, entry, name, nil, false, d)
	if err != nil {
		return err
	}
	p := inspectGraph(g, entry, d)
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	page := strings.Replace(inspectorHTML, `"SOURCE_DATA_PLACEHOLDER"`, string(data), 1)
	if !filepath.IsAbs(output) {
		output = filepath.Join(dir, output)
	}
	return os.WriteFile(output, []byte(page), 0644)
}

func signatureParameters(fn *ast.FuncType) []*ast.Ident {
	var out []*ast.Ident
	if fn.Params != nil {
		for _, f := range fn.Params.List {
			out = append(out, f.Names...)
		}
	}
	return out
}
func isContextObject(obj types.Object) bool {
	if obj == nil {
		return false
	}
	t, ok := obj.Type().(*types.Named)
	return ok && t.Obj().Pkg() != nil && t.Obj().Pkg().Path() == "context" && t.Obj().Name() == "Context"
}
func (e *extractor) contextParameter(fn *ast.FuncType) types.Object {
	for _, param := range signatureParameters(fn) {
		obj := e.pkg.TypesInfo.Defs[param]
		if isContextObject(obj) {
			return obj
		}
	}
	return nil
}

func (e *extractor) inspectionRangeContext(stmt *ast.RangeStmt) func() {
	previous := e.inspectionContext
	if e.inspection != nil {
		e.inspectionContext = nil
		for _, expr := range []ast.Expr{stmt.Key, stmt.Value} {
			if obj := e.object(expr); isContextObject(obj) {
				e.inspectionContext = obj
				break
			}
		}
	}
	return func() { e.inspectionContext = previous }
}

// These standard context constructors preserve Value lookups. A replacement
// with Background, or an unrecognized helper, does not establish a scope.
func (e *extractor) preservesInspectionContext(expr ast.Expr) bool {
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) == 0 || e.object(call.Args[0]) != e.inspectionContext {
		return false
	}
	fn := typeutil.StaticCallee(e.pkg.TypesInfo, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "context" {
		return false
	}
	return slices.Contains([]string{"WithCancel", "WithCancelCause", "WithTimeout", "WithTimeoutCause", "WithDeadline", "WithDeadlineCause", "WithoutCancel"}, fn.Name())
}
