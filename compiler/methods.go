package compiler

import (
	"go/types"

	"golang.org/x/tools/go/packages"
)

// Implicit JSON methods are part of split-result admission: accepted bytes must
// decode without arbitrary hooks or a schema promoted from another type.
func checkResultMethods(pkg *packages.Package, result types.Type, fail func(string) error) error {
	seen := map[types.Type]bool{}
	var visit func(types.Type) error
	visit = func(t types.Type) error {
		if t == nil || seen[t] {
			return nil
		}
		seen[t] = true
		for _, methods := range []*types.MethodSet{types.NewMethodSet(t), types.NewMethodSet(types.NewPointer(t))} {
			for method := range methods.Methods() {
				m := method.Obj().(*types.Func)
				switch m.Name() {
				case "MarshalJSON", "UnmarshalJSON", "MarshalText", "UnmarshalText", "IsZero":
					if !PolytypeMethod(pkg, m) {
						return fail("unsupported implicit json method: " + m.Name())
					}
				}
			}
		}
		switch u := t.Underlying().(type) {
		case *types.Interface:
			variants := SealedVariants(t)
			if len(variants) == 0 {
				return fail("opaque interface values cannot be checked for implicit json effects")
			}
			for _, variant := range variants {
				if err := visit(variant); err != nil {
					return err
				}
			}
		case *types.Struct:
			for field := range u.Fields() {
				if field.Embedded() {
					return fail("embedded Generate result fields are unsupported: schema and codec methods can be promoted from a different type")
				}
				if err := visit(field.Type()); err != nil {
					return err
				}
			}
		case *types.Map:
			if err := visit(u.Key()); err != nil {
				return err
			}
			return visit(u.Elem())
		case *types.Pointer:
			return visit(u.Elem())
		case *types.Slice:
			return visit(u.Elem())
		case *types.Array:
			return visit(u.Elem())
		}
		return nil
	}
	return visit(result)
}
