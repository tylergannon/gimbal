package gimballint

import (
	"go/ast"
	"go/constant"

	"golang.org/x/tools/go/analysis"
)

// Declarations are source-visible. Producer declarations apply to one operation,
// while requirements accumulate on the consuming context.
func reportValueDeclarations(pass *analysis.Pass) {
	for _, file := range pass.Files {
		inline := map[ast.Expr]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			name, ok := gimbalCall(pass, call)
			if ok && (name == "Service" || name == "RunCommand" || name == "Check") {
				inline[ast.Unparen(call.Args[0])] = true
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := gimbalCall(pass, call)
			if !ok || (name != "WithYields" && name != "WithRequirements") {
				return true
			}
			if name == "WithYields" && !inline[call] {
				pass.Reportf(call.Pos(), "[GIMBAL113-INLINE-YIELDS]: WithYields must be the inline context argument of Service, RunCommand, or Check, so outputs belong to one producer.")
			}
			seen := map[string]bool{}
			for i, arg := range call.Args[1:] {
				v := pass.TypesInfo.Types[arg].Value
				if v == nil || v.Kind() != constant.String {
					pass.Reportf(arg.Pos(), "[GIMBAL111-CONSTANT-VALUES]: Store and value names must be compile-time strings.")
					continue
				}
				if name == "WithYields" && i > 0 {
					value := constant.StringVal(v)
					if seen[value] {
						pass.Reportf(arg.Pos(), "[GIMBAL112-DUPLICATE-YIELD]: A producer declares each output name once.")
					}
					seen[value] = true
				}
			}
			return true
		})
	}
}
