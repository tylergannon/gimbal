package generate

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// entryInfo is what the generated command needs about the entry besides its
// body: its optional workflow parameter type and that type's fields; the first
// sentence of its doc comment; and the package's.
type entryInfo struct {
	params  string
	summary string
	long    string
	fields  []field
}

// field is one field of the parameter type as a flag: its Go name, the flag's
// name, whether the value is a string, an int, or a bool, whether the field
// is a polytype.Optional the flag sets only when given, and its doc.
type field struct {
	name, flag, kind, doc string
	optional              bool
}

// describe reads the entry's signature, its doc, and its parameters' fields.
// An entry takes a ctx, gimble.Env, and at most one workflow parameter struct
// from its own package.
func describe(pkg *packages.Package, decl *ast.FuncDecl) (entryInfo, error) {
	info := entryInfo{summary: (&doc.Package{}).Synopsis(decl.Doc.Text()), long: packageDoc(pkg)}
	fn, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok {
		return info, fmt.Errorf("generate: %s has no type", decl.Name.Name)
	}
	params := fn.Type().(*types.Signature).Params()
	if params.Len() < 2 || !isGimbleEnv(params.At(1).Type()) {
		return info, fmt.Errorf("generate: %s's second parameter is not gimble.Env", decl.Name.Name)
	}
	switch params.Len() {
	case 2:
		return info, nil
	case 3:
		named, ok := params.At(2).Type().(*types.Named)
		if !ok || named.Obj().Pkg() != pkg.Types {
			return info, fmt.Errorf("generate: %s's params are %s, not a type of its own package", decl.Name.Name, params.At(2).Type())
		}
		info.params = named.Obj().Name()
		fields, err := paramFields(pkg, named.Obj().Name())
		if err != nil {
			return info, err
		}
		info.fields = fields
		return info, nil
	default:
		return info, fmt.Errorf("generate: %s takes %d parameters; an entry takes a ctx, gimble.Env, and at most one parameter struct", decl.Name.Name, params.Len())
	}
}

func isGimbleEnv(t types.Type) bool {
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == gimblePath && named.Obj().Name() == "Env"
}

func packageDoc(pkg *packages.Package) string {
	for _, file := range pkg.Syntax {
		if file.Doc != nil {
			return strings.TrimSpace(file.Doc.Text())
		}
	}
	return ""
}

// paramFields reads the exported fields of the parameter struct in source order.
func paramFields(pkg *packages.Package, typeName string) ([]field, error) {
	var spec *ast.TypeSpec
	for _, file := range pkg.Syntax {
		for _, d := range file.Decls {
			if gen, ok := d.(*ast.GenDecl); ok {
				for _, s := range gen.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
						spec = ts
					}
				}
			}
		}
	}
	structType, ok := spec.Type.(*ast.StructType)
	if spec == nil || !ok {
		return nil, fmt.Errorf("generate: the parameter type %s is not a struct", typeName)
	}
	var fields []field
	for _, f := range structType.Fields.List {
		for _, ident := range f.Names {
			if !ident.IsExported() {
				continue
			}
			kind, optional, err := flagKind(pkg.TypesInfo.TypeOf(f.Type))
			if err != nil {
				return nil, fmt.Errorf("generate: %s.%s: %w", typeName, ident.Name, err)
			}
			fields = append(fields, field{name: ident.Name, flag: kebab(ident.Name), kind: kind, optional: optional, doc: strings.TrimSpace(f.Doc.Text())})
		}
	}
	return fields, nil
}

func flagKind(t types.Type) (kind string, optional bool, err error) {
	if named, ok := t.(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "github.com/tylergannon/polytype" && named.Obj().Name() == "Optional" && named.TypeArgs().Len() == 1 {
		kind, _, err := flagKind(named.TypeArgs().At(0))
		return kind, true, err
	}
	if basic, ok := t.(*types.Basic); ok {
		switch basic.Kind() {
		case types.String:
			return "string", false, nil
		case types.Int:
			return "int", false, nil
		case types.Bool:
			return "bool", false, nil
		}
	}
	return "", false, fmt.Errorf("%s is not a flag: a field is a string, an int, a bool, or a polytype.Optional of one", t)
}

func kebab(name string) string {
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
