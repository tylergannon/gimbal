package gimballint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// effectFact carries the source-visible call graph's first prohibited operation.
// Empty means that the function and all its statically resolved helpers are pure.
type effectFact struct{ Reason string }

func (*effectFact) AFact()           {}
func (f *effectFact) String() string { return f.Reason }

const libraryEffects = "[GIMBAL111-LIBRARY-EFFECTS]: Workflow effects and nondeterministic observations must use Gimbal operations: "

type effectBody struct {
	body   *ast.BlockStmt
	reason string
	calls  []*types.Func
}

func reportEffects(pass *analysis.Pass, si *syntaxInfo) {
	bodies := map[*types.Func]*effectBody{}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			obj, _ := pass.TypesInfo.Defs[fn.Name].(*types.Func)
			if obj != nil {
				bodies[obj] = inspectEffects(pass, fn.Body)
			}
		}
	}
	// Propagate through local recursion to a fixed point. Imported facts already
	// include the dependency's source-visible helper graph.
	changed := true
	for changed {
		changed = false
		for _, b := range bodies {
			if b.reason != "" {
				continue
			}
			for _, fn := range b.calls {
				reason := ""
				if target := bodies[fn]; target != nil {
					reason = target.reason
				} else {
					fact := effectFact{}
					if pass.ImportObjectFact(fn, &fact) {
						reason = fact.Reason
					} else {
						reason = "unknown opaque call " + fn.FullName()
					}
				}
				if reason != "" {
					b.reason = reason
					changed = true
					break
				}
			}
		}
	}
	for fn, b := range bodies {
		if fn.Exported() {
			pass.ExportObjectFact(fn, &effectFact{Reason: b.reason})
		}
	}
	if pass.Pkg.Path() == gimbalPath {
		return
	}
	for _, u := range si.units {
		if strings.HasSuffix(pass.Fset.Position(u.body.Pos()).Filename, "_test.go") {
			continue
		}
		if !u.workflow && !u.reachable {
			continue
		}
		// Hosts may call Run to launch a workflow; only its callback is authored.
		b := inspectEffects(pass, u.body)
		if b.reason == "" {
			for _, fn := range b.calls {
				if target := bodies[fn]; target != nil {
					b.reason = target.reason
				} else {
					fact := effectFact{}
					if pass.ImportObjectFact(fn, &fact) {
						b.reason = fact.Reason
					} else {
						b.reason = "unknown opaque call " + fn.FullName()
					}
				}
				if b.reason != "" {
					break
				}
			}
		}
		if b.reason != "" {
			pass.Reportf(u.body.Pos(), "%s%s", libraryEffects, b.reason)
		}
	}
}

func inspectEffects(pass *analysis.Pass, body *ast.BlockStmt) *effectBody {
	result := &effectBody{body: body}
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	literals := map[types.Object]bool{}
	aliases := map[types.Object]*types.Func{}
	assignments := map[types.Object]int{}
	ast.Inspect(body, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok {
			for _, left := range a.Lhs {
				if id, ok := left.(*ast.Ident); ok {
					obj := pass.TypesInfo.Defs[id]
					if obj == nil {
						obj = pass.TypesInfo.Uses[id]
					}
					assignments[obj]++
				}
			}
			for i, r := range a.Rhs {
				if i < len(a.Lhs) {
					if id, ok := a.Lhs[i].(*ast.Ident); ok {
						obj := pass.TypesInfo.Defs[id]
						if obj == nil {
							obj = pass.TypesInfo.Uses[id]
						}
						if f, ok := typeutil.Callee(pass.TypesInfo, &ast.CallExpr{Fun: r}).(*types.Func); ok {
							aliases[obj] = f
						}
					}
				}
				if _, ok := r.(*ast.FuncLit); ok && i < len(a.Lhs) {
					if id, ok := a.Lhs[i].(*ast.Ident); ok {
						obj := pass.TypesInfo.Defs[id]
						if obj == nil {
							obj = pass.TypesInfo.Uses[id]
						}
						literals[obj] = true
					}
				}
			}
		}
		return true
	})
	reject := func(reason string) {
		if result.reason == "" {
			result.reason = reason
		}
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if v, ok := pass.TypesInfo.Uses[n.Sel].(*types.Var); ok && v.Pkg() != nil && v.Parent() == v.Pkg().Scope() {
				pkg, name := v.Pkg().Path(), v.Name()
				sentinel := (pkg == "io" && name == "EOF") || (pkg == "context" && (name == "Canceled" || name == "DeadlineExceeded")) || (pkg == "os" && strings.HasPrefix(name, "Err")) || (pkg == "time" && name == "UTC")
				if pkg != gimbalPath && pkg != pass.Pkg.Path() && !sentinel {
					reject("ambient package variable " + pkg + "." + name)
				}
			}
		case *ast.GoStmt:
			reject("raw goroutine")
		case *ast.DeferStmt:
			reject("authored defer")
		case *ast.SelectStmt:
			reject("select")
		case *ast.SendStmt:
			reject("channel send")
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				reject("channel receive")
			}
		case *ast.RangeStmt:
			if t := pass.TypesInfo.TypeOf(n.X); t != nil {
				switch t.Underlying().(type) {
				case *types.Map:
					reject("map iteration; sort keys before iteration")
				case *types.Chan:
					reject("channel iteration")
				}
			}
		case *ast.CallExpr:
			if tv, ok := pass.TypesInfo.Types[n.Fun]; ok && tv.IsType() {
				break
			}
			if id, ok := n.Fun.(*ast.Ident); ok {
				if builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin); ok {
					switch builtin.Name() {
					case "close":
						reject("channel close")
					case "len", "cap":
						if len(n.Args) > 0 {
							if typ := pass.TypesInfo.TypeOf(n.Args[0]); typ != nil {
								if _, ok := typ.Underlying().(*types.Chan); ok {
									reject("channel observation")
								}
							}
						}
					case "recover":
						reject("authored recover")
					case "make":
						if _, ok := pass.TypesInfo.TypeOf(n).Underlying().(*types.Chan); ok {
							reject("channel construction")
						}
					}
					break
				}
			}
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" && len(n.Args) == 0 && types.Identical(pass.TypesInfo.TypeOf(sel.X), types.Universe.Lookup("error").Type()) {
				break
			}
			fn := typeutil.StaticCallee(pass.TypesInfo, n)
			if fn == nil {
				if id, ok := n.Fun.(*ast.Ident); ok {
					if target := aliases[pass.TypesInfo.Uses[id]]; target != nil && assignments[pass.TypesInfo.Uses[id]] == 1 {
						if target.Pkg() != nil && (target.Pkg().Path() == gimbalPath || pureOpaque(target)) {
							break
						}
						result.calls = append(result.calls, target)
						break
					}
				}
				if id, ok := n.Fun.(*ast.Ident); ok && literals[pass.TypesInfo.Uses[id]] && assignments[pass.TypesInfo.Uses[id]] == 1 {
					break
				}
				if _, ok := n.Fun.(*ast.FuncLit); !ok {
					reject("dynamic or opaque function call")
				}
				break
			}
			if fn.Pkg() == nil {
				break
			}
			if fn.Pkg().Path() == "maps" && fn.Name() == "Keys" {
				parent, ok := parents[n].(*ast.CallExpr)
				if !ok {
					reject("maps.Keys requires slices.Sorted before traversal")
					break
				}
				outer := typeutil.StaticCallee(pass.TypesInfo, parent)
				if outer == nil || outer.Pkg() == nil || outer.Pkg().Path() != "slices" || outer.Name() != "Sorted" {
					reject("maps.Keys requires slices.Sorted before traversal")
					break
				}
			}
			if fn.Pkg().Path() == "slices" && fn.Name() == "Sorted" {
				if len(n.Args) != 1 {
					reject("unknown iterator passed to slices.Sorted")
					break
				}
				inner, ok := n.Args[0].(*ast.CallExpr)
				if !ok {
					reject("unknown iterator passed to slices.Sorted")
					break
				}
				source := typeutil.StaticCallee(pass.TypesInfo, inner)
				if source == nil || source.Pkg() == nil || source.Pkg().Path() != "maps" || source.Name() != "Keys" {
					reject("unknown iterator passed to slices.Sorted")
					break
				}
			}
			if fn.Pkg().Path() == "slices" && fn.Name() == "SortFunc" {
				if len(n.Args) != 2 {
					reject("unknown sorting callback")
					break
				}
				if _, ok := n.Args[1].(*ast.FuncLit); !ok {
					reject("unknown sorting callback")
					break
				}
			}

			if fn.Pkg().Path() == gimbalPath && fn.Name() == "Interactive" {
				if len(n.Args) < 2 || pass.TypesInfo.Types[n.Args[1]].Value == nil {
					reject("Interactive name must be a compile-time constant")
					break
				}
			}
			if fn.Pkg().Path() == gimbalPath || pureOpaque(fn) {
				break
			}
			result.calls = append(result.calls, fn)
		}
		return true
	})
	return result
}

// Keep this table function-specific. Package membership never grants an effect
// exemption. These operations observe only their arguments, without callbacks.
func pureOpaque(fn *types.Func) bool {
	pkg, name := fn.Pkg().Path(), fn.Name()
	table := map[string]string{
		"fmt":                             "Sprintf Sprint Sprintln Errorf",
		"errors":                          "New Join Is As Unwrap",
		"strings":                         "Clone Compare Contains ContainsAny ContainsRune Count Cut CutPrefix CutSuffix EqualFold Fields HasPrefix HasSuffix Index IndexAny IndexByte Join LastIndex Repeat Replace ReplaceAll Split SplitN ToLower ToUpper Trim TrimLeft TrimPrefix TrimRight TrimSpace TrimSuffix Builder WriteString WriteByte String Len Reset",
		"strconv":                         "Atoi Itoa AppendBool AppendFloat AppendInt AppendQuote AppendUint FormatBool FormatFloat FormatInt FormatUint ParseBool ParseFloat ParseInt ParseUint Quote Unquote",
		"path":                            "Base Clean Dir Ext IsAbs Join Match Split",
		"path/filepath":                   "Base Clean Dir Ext FromSlash IsAbs IsLocal Join Match Rel Split ToSlash VolumeName",
		"encoding/json":                   "Marshal MarshalIndent Unmarshal Valid Compact Indent HTMLEscape",
		"bytes":                           "NewReader NewBuffer NewBufferString Equal Compare Contains HasPrefix HasSuffix Join Split TrimSpace Write WriteString WriteByte Bytes String Len",
		"sort":                            "Strings Ints Float64s SearchInts SearchStrings",
		"slices":                          "Clone Clip Compact Concat Contains Delete Equal Index Insert Max Min Repeat Reverse Sort SortFunc Sorted",
		"maps":                            "Keys Clone Equal",
		"math":                            "Abs Ceil Floor Max Min Round Sqrt Pow IsInf IsNaN",
		"time":                            "Parse ParseDuration Seconds Milliseconds Microseconds Nanoseconds String Format UTC Unix UnixMilli UnixMicro UnixNano Sub Add Before After Equal",
		"context":                         "Background TODO WithValue",
		"net/url":                         "Parse ParseRequestURI PathEscape PathUnescape QueryEscape QueryUnescape String Hostname Port IsAbs EscapedPath",
		"regexp":                          "Compile MustCompile MatchString Match FindString FindStringSubmatch ReplaceAllString",
		"gopkg.in/yaml.v3":                "NewDecoder KnownFields Decode",
		"embed":                           "ReadFile ReadDir",
		"github.com/tylergannon/polytype": "Some None Null Value IsSet IsNull Get",
	}
	for _, allowed := range strings.Fields(table[pkg]) {
		if name == allowed {
			return true
		}
	}
	return false
}
