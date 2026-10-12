package gimballint

import (
	"go/ast"
	"go/constant"

	"golang.org/x/tools/go/analysis"
)

// Declarations are deliberately source-visible. This checks authorship rather
// than adding a second permissions or dependency engine to native execution.
func reportValueDeclarations(pass *analysis.Pass) {
	check := func(expr ast.Expr) {
		v := pass.TypesInfo.Types[expr].Value
		if v == nil || v.Kind() != constant.String {
			pass.Reportf(expr.Pos(), "[GIMBAL111-CONSTANT-VALUES]: Store and value names must be compile-time strings.")
		}
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := gimbalCall(pass, call)
			if !ok {
				return true
			}
			switch name {
			case "WithYields":
				seen := map[string]bool{}
				for i, arg := range call.Args[1:] {
					check(arg)
					value := constantString(pass, arg)
					if i > 0 && value != "" {
						if seen[value] {
							pass.Reportf(arg.Pos(), "[GIMBAL112-DUPLICATE-YIELD]: A producer declares each output name once.")
						}
						seen[value] = true
					}
				}
			case "WithRequirements":
				for _, arg := range call.Args[1:] {
					lit, ok := arg.(*ast.CompositeLit)
					if !ok {
						pass.Reportf(arg.Pos(), "[GIMBAL111-CONSTANT-VALUES]: Declare store requirements with source-visible literals containing constant names.")
						continue
					}
					for i, item := range lit.Elts {
						field, value := "Store", item
						if i == 1 {
							field = "Names"
						}
						if kv, ok := item.(*ast.KeyValueExpr); ok {
							field = kv.Key.(*ast.Ident).Name
							value = kv.Value
						}
						if field == "Store" {
							check(value)
							continue
						}
						if names, ok := value.(*ast.CompositeLit); ok {
							for _, name := range names.Elts {
								check(name)
							}
						} else {
							pass.Reportf(value.Pos(), "[GIMBAL111-CONSTANT-VALUES]: Declare required names in a literal list of compile-time strings.")
						}
					}
				}
			}
			return true
		})
	}
}
