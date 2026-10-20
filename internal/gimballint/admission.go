package gimballint

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// Admit checks the entry and its source-visible helpers, including repository
// package boundaries. It is the generation/admission counterpart of the vet
// analyzer; ordinary host setup outside the selected entry is not inspected.
func Admit(pkg *packages.Package, entry string) error {
	loaded := map[string]*packages.Package{pkg.PkgPath: pkg}
	visited := map[string]bool{}
	var walk func(*packages.Package, *ast.FuncDecl) error
	walk = func(p *packages.Package, decl *ast.FuncDecl) error {
		key := p.PkgPath + "." + decl.Name.Name
		if decl.Recv != nil {
			key = p.PkgPath + "." + types.TypeString(p.TypesInfo.TypeOf(decl.Recv.List[0].Type), nil) + "." + decl.Name.Name
		}
		if visited[key] {
			return nil
		}
		visited[key] = true
		pass := &analysis.Pass{Fset: p.Fset, Pkg: p.Types, TypesInfo: p.TypesInfo, Files: p.Syntax}
		body := inspectEffects(pass, decl.Body)
		if body.reason != "" {
			return fmt.Errorf("%s: %s%s", p.Fset.Position(decl.Pos()), libraryEffects, body.reason)
		}
		for _, fn := range body.calls {
			path := fn.Pkg().Path()
			if path != pkg.PkgPath && (pkg.Module == nil || (path != pkg.Module.Path && !strings.HasPrefix(path, pkg.Module.Path+"/"))) {
				return fmt.Errorf("%s: %sunknown opaque call %s", p.Fset.Position(decl.Pos()), libraryEffects, fn.FullName())
			}
			target := loaded[path]
			if target == nil {
				roots, err := packages.Load(&packages.Config{Dir: filepath.Dir(pkg.GoFiles[0]), Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedModule}, path)
				if err != nil {
					return err
				}
				if len(roots) != 1 {
					return fmt.Errorf("admission: cannot read helper package %s", path)
				}
				target = roots[0]
				loaded[path] = target
			}
			var found *ast.FuncDecl
			for _, file := range target.Syntax {
				for _, d := range file.Decls {
					if f, ok := d.(*ast.FuncDecl); ok && f.Body != nil {
						obj, _ := target.TypesInfo.Defs[f.Name].(*types.Func)
						if obj != nil && obj.FullName() == fn.FullName() {
							found = f
							break
						}
					}
				}
			}
			if found == nil {
				return fmt.Errorf("%s: %sunknown opaque call %s", p.Fset.Position(decl.Pos()), libraryEffects, fn.FullName())
			}
			if err := walk(target, found); err != nil {
				return err
			}
		}
		return nil
	}
	for _, f := range pkg.Syntax {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == entry && fn.Body != nil {
				return walk(pkg, fn)
			}
		}
	}
	return fmt.Errorf("admission: no workflow entry %s", entry)
}
