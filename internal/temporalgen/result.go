package temporalgen

import (
	"go/ast"
	"go/types"

	"github.com/tylergannon/gimbal/internal/compiler"
)

func (e *emitter) resultContract(site ast.Node, result types.Type) error {
	if e.results[result] {
		return nil
	}
	if err := compiler.CheckSplitResponse(e.pkg, result, site.Pos()); err != nil {
		return err
	}
	if e.results == nil {
		e.results = map[types.Type]bool{}
	}
	e.results[result] = true
	return nil
}
