//go:build ignore

// This is an executable research probe, not part of Gimble's package graph.
// Run it explicitly from the repository root with:
//
//	go run ./ephemeral/research/graphs/go-feasibility-probe.go
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedDeps | packages.NeedSyntax | packages.NeedTypes |
	packages.NeedTypesInfo

func main() {
	cfg := &packages.Config{Mode: loadMode, Tests: false}
	pkgs, err := packages.Load(cfg, ".", "./internal/workflows/sprint")
	if err != nil {
		fatal(err)
	}
	if n := packages.PrintErrors(pkgs); n != 0 {
		fatal(fmt.Errorf("package load reported %d errors", n))
	}

	var root, sprint *packages.Package
	for _, pkg := range pkgs {
		switch pkg.PkgPath {
		case "github.com/tylergannon/gimble":
			root = pkg
		case "github.com/tylergannon/gimble/internal/workflows/sprint":
			sprint = pkg
		}
	}
	if root == nil || sprint == nil {
		fatal(fmt.Errorf("missing root or sprint package (root=%v sprint=%v)", root != nil, sprint != nil))
	}

	fmt.Printf("go=%s\n", runtime.Version())
	fmt.Printf("loaded=%s,%s\n", root.PkgPath, sprint.PkgPath)
	printGenerate(root)
	printCalls(sprint)
	printScopeCallbacks(sprint)
	printSSA(sprint, pkgs)
}

func printGenerate(pkg *packages.Package) {
	obj := pkg.Types.Scope().Lookup("Session")
	named, ok := obj.(*types.TypeName)
	if !ok {
		fatal(fmt.Errorf("Session is %T, want *types.TypeName", obj))
	}
	methods := types.NewMethodSet(types.NewPointer(named.Type()))
	for i := 0; i < methods.Len(); i++ {
		fn, ok := methods.At(i).Obj().(*types.Func)
		if !ok || fn.Name() != "Generate" {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			fatal(fmt.Errorf("Generate has type %T", fn.Type()))
		}
		fmt.Printf("generate=%s typeparams=%d\n", sig, sig.TypeParams().Len())
		return
	}
	fatal(fmt.Errorf("generic Session.Generate method not found"))
}

func printCalls(pkg *packages.Package) {
	type call struct {
		file string
		line int
		text string
	}
	var calls []call
	for _, file := range pkg.Syntax {
		filename := pkg.Fset.Position(file.Pos()).Filename
		ast.Inspect(file, func(node ast.Node) bool {
			callExpr, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := callExpr.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			fn, ok := pkg.TypesInfo.Uses[selector.Sel].(*types.Func)
			if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "os/exec" {
				return true
			}
			calls = append(calls, call{file: filename, line: pkg.Fset.Position(callExpr.Pos()).Line, text: selector.Sel.Name})
			return true
		})
	}
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].file+fmt.Sprint(calls[i].line) < calls[j].file+fmt.Sprint(calls[j].line)
	})
	for _, c := range calls {
		fmt.Printf("exec-call=%s:%d %s\n", c.file, c.line, c.text)
	}
}

func printScopeCallbacks(pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(node ast.Node) bool {
			if rangeStmt, ok := node.(*ast.RangeStmt); ok {
				if selector, ok := rangeStmt.X.(*ast.SelectorExpr); ok && selector.Sel.Name == "Tasks" {
					pos := pkg.Fset.Position(rangeStmt.Pos())
					fmt.Printf("scope-callback=%s:%d callee=range.%s params=%s\n", pos.Filename, pos.Line, selector.Sel.Name, rangeParams(rangeStmt, pkg.TypesInfo))
				}
				return true
			}
			callExpr, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := ""
			switch fun := callExpr.Fun.(type) {
			case *ast.Ident:
				callee = fun.Name
			case *ast.SelectorExpr:
				callee = fun.Sel.Name
			}
			if callee != "Scope" && callee != "Run" && callee != "Go" && callee != "Tasks" {
				return true
			}
			for _, arg := range callExpr.Args {
				if lit, ok := arg.(*ast.FuncLit); ok {
					pos := pkg.Fset.Position(lit.Pos())
					fmt.Printf("scope-callback=%s:%d callee=%s params=%s\n", pos.Filename, pos.Line, callee, params(lit, pkg.TypesInfo))
				}
			}
			return true
		})
	}
}

func rangeParams(stmt *ast.RangeStmt, info *types.Info) string {
	var names []string
	for _, expr := range []*ast.Ident{identOf(stmt.Key), identOf(stmt.Value)} {
		if expr == nil {
			continue
		}
		if obj := info.Defs[expr]; obj != nil {
			names = append(names, expr.Name+":"+types.TypeString(obj.Type(), func(*types.Package) string { return "" }))
		}
	}
	return strings.Join(names, ",")
}

func identOf(expr ast.Expr) *ast.Ident {
	ident, _ := expr.(*ast.Ident)
	return ident
}

func params(lit *ast.FuncLit, info *types.Info) string {
	var names []string
	if lit.Type.Params == nil {
		return ""
	}
	for _, field := range lit.Type.Params.List {
		for _, name := range field.Names {
			obj := info.Defs[name]
			if obj == nil {
				names = append(names, name.Name)
				continue
			}
			names = append(names, name.Name+":"+types.TypeString(obj.Type(), func(*types.Package) string { return "" }))
		}
	}
	return strings.Join(names, ",")
}

func printSSA(pkg *packages.Package, all []*packages.Package) {
	program, initial := ssautil.Packages(all, 0)
	program.Build()
	var ssaPkg *ssa.Package
	for _, candidate := range initial {
		if candidate != nil && candidate.Pkg.Path() == pkg.PkgPath {
			ssaPkg = candidate
			break
		}
	}
	if ssaPkg == nil {
		fatal(fmt.Errorf("no SSA package for %s", pkg.PkgPath))
	}
	fnCount, blockCount := 0, 0
	var visit func(*ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil {
			return
		}
		if fn.Blocks != nil {
			fnCount++
			blockCount += len(fn.Blocks)
		}
		for _, child := range fn.AnonFuncs {
			visit(child)
		}
	}
	for _, member := range ssaPkg.Members {
		fn, ok := member.(*ssa.Function)
		if !ok {
			continue
		}
		visit(fn)
	}
	fmt.Printf("ssa-package=%s functions-with-bodies=%d blocks=%d\n", pkg.PkgPath, fnCount, blockCount)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
