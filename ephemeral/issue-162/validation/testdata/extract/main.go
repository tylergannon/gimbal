// This proof probe reads switch alternatives without executing the workflow.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
)

func main() {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, os.Args[1], nil, 0)
	if err != nil {
		panic(err)
	}
	text := func(node ast.Node) string {
		var buffer bytes.Buffer
		if err := format.Node(&buffer, fset, node); err != nil {
			panic(err)
		}
		return buffer.String()
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switchNode, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		fmt.Println("switch", text(switchNode.Tag))
		for _, statement := range switchNode.Body.List {
			clause := statement.(*ast.CaseClause)
			label := "default"
			if len(clause.List) > 0 {
				label = text(clause.List[0])
			}
			ast.Inspect(clause, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					fmt.Printf("%s -> %s\n", label, text(call))
				}
				return true
			})
		}
		return false
	})
}
