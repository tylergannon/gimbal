package generate

import (
	"go/ast"
	"slices"

	"github.com/tylergannon/gimbal/workflow"
	"golang.org/x/tools/go/types/typeutil"
)

type valueDeclarations struct {
	requirements []workflow.StoreDeclaration
	yields       []workflow.StoreDeclaration
}

func (e *extractor) callDeclarations(call *ast.CallExpr) valueDeclarations {
	if len(call.Args) == 0 {
		return valueDeclarations{}
	}
	return e.contextDeclarations(call.Args[0])
}

// Declarations follow context values, not the nearest textual scope: a caller
// can retain an earlier context or pass a decorated one into a helper.
func (e *extractor) contextDeclarations(expr ast.Expr) valueDeclarations {
	if expr == nil {
		return valueDeclarations{}
	}
	if obj := e.object(expr); obj != nil {
		return e.declarations[obj]
	}
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return valueDeclarations{}
	}
	name, gimbal := e.gimbalCall(call)
	if !gimbal {
		fn := typeutil.StaticCallee(e.pkg.TypesInfo, call)
		if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "context" ||
			!slices.Contains([]string{"WithCancel", "WithCancelCause", "WithTimeout", "WithTimeoutCause", "WithDeadline", "WithDeadlineCause", "WithoutCancel", "WithValue"}, fn.Name()) {
			return valueDeclarations{}
		}
	}
	inherited := e.contextDeclarations(call.Args[0])
	if !gimbal || (name != "WithRequirements" && name != "WithYields") {
		return inherited
	}
	store, ok := e.constant(call, 1)
	if !ok {
		e.diag(call.Pos(), "%s's store is not constant, so its declaration is not read", name)
		return inherited
	}
	declaration := workflow.StoreDeclaration{Store: store, Names: []string{}}
	for i := 2; i < len(call.Args); i++ {
		value, ok := e.constant(call, i)
		if !ok {
			e.diag(call.Pos(), "%s's names are not constant, so its declaration is not read", name)
			return inherited
		}
		declaration.Names = append(declaration.Names, value)
	}
	if name == "WithYields" {
		inherited.yields = []workflow.StoreDeclaration{declaration}
	} else {
		inherited.requirements = append(slices.Clone(inherited.requirements), declaration)
	}
	return inherited
}

func (e *extractor) bindRangeDeclarations(stmt *ast.RangeStmt, declarations valueDeclarations) {
	for _, expr := range []ast.Expr{stmt.Key, stmt.Value} {
		if obj := e.object(expr); isContextObject(obj) {
			e.declarations[obj] = declarations
		}
	}
}
