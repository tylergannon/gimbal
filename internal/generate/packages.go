package generate

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/packages"
)

// A snapshot is local to one generation. Subsequent calls reload authored
// source; there is no process-wide package or import cache.
type sourcePackages struct {
	roots   []*packages.Package
	imports map[string]bool
}

func loadSource(dir string, overlay map[string][]byte, patterns ...string) (*sourcePackages, error) {
	config := &packages.Config{Dir: dir, Overlay: overlay}
	cleanup, err := preserveExports(config)
	if err != nil {
		return nil, fmt.Errorf("generate: preparing source: %w", err)
	}
	defer cleanup()
	// Export data omits imports used only by a non-inlinable helper's body.
	// Read source import metadata without rechecking dependency syntax/types.
	config.Mode = packages.NeedName | packages.NeedImports | packages.NeedDeps
	metadata, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	imports := map[string]bool{}
	var loadErr error
	packages.Visit(metadata, nil, func(pkg *packages.Package) {
		if len(pkg.Errors) > 0 && loadErr == nil {
			loadErr = fmt.Errorf("generate: %s: %v", pkg.PkgPath, pkg.Errors[0])
		}
		_, imports[pkg.PkgPath] = pkg.Imports[gimbalPath]
	})
	if loadErr != nil {
		return nil, loadErr
	}
	config.Mode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
		packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
		packages.NeedImports | packages.NeedModule
	roots, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	return &sourcePackages{roots: roots, imports: imports}, nil
}

func (s *sourcePackages) packageAt(dir string) (*packages.Package, error) {
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	for _, pkg := range s.roots {
		if len(pkg.GoFiles) == 0 {
			continue
		}
		pkgDir, err := filepath.EvalSymlinks(filepath.Dir(pkg.GoFiles[0]))
		if err == nil && pkgDir == canonical {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("generate: %s was not loaded", dir)
}

// Config.Overlay invalidates dependency compiler exports in x/tools v0.51.0.
// As in SKGO's loader, pass existing replacements to the Go command's overlay
// and the root parser so dependencies retain Go's ordinary export caching.
func preserveExports(config *packages.Config) (func(), error) {
	noop := func() {}
	if len(config.Overlay) == 0 {
		return noop, nil
	}
	if driver := os.Getenv("GOPACKAGESDRIVER"); driver != "off" {
		if driver != "" {
			return noop, nil
		}
		if _, err := exec.LookPath("gopackagesdriver"); err == nil {
			return noop, nil
		}
	}
	logicalDir, err := filepath.Abs(config.Dir)
	if err != nil {
		return noop, err
	}
	physicalDir, err := filepath.EvalSymlinks(logicalDir)
	if err != nil {
		return noop, err
	}
	contents := map[string][]byte{}
	var paths []string
	for path, source := range config.Overlay {
		canonical, err := filepath.EvalSymlinks(path)
		if os.IsNotExist(err) {
			// An absent generated file containing only a package clause adds
			// no declarations. Omitting it avoids inventing a temporary module.
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, source, 0)
			if parseErr == nil && len(file.Decls) == 0 {
				continue
			}
			// Arbitrary new source files retain go/packages overlay semantics.
			return noop, nil
		}
		if err != nil {
			return noop, err
		}
		contents[canonical] = source
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		config.Overlay = nil
		return noop, nil
	}
	slices.Sort(paths)
	dir, err := os.MkdirTemp("", "gimbal-source-overlay-")
	if err != nil {
		return noop, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	replacements := map[string]string{}
	for i, path := range paths {
		backing := filepath.Join(dir, fmt.Sprintf("%d.go", i))
		if err := os.WriteFile(backing, config.Overlay[path], 0600); err != nil {
			cleanup()
			return noop, err
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			cleanup()
			return noop, err
		}
		relative, err := filepath.Rel(physicalDir, canonical)
		if err != nil {
			cleanup()
			return noop, err
		}
		replacements[path] = backing
		replacements[canonical] = backing
		replacements[filepath.Join(logicalDir, relative)] = backing
	}
	raw, err := json.Marshal(struct{ Replace map[string]string }{replacements})
	if err != nil {
		cleanup()
		return noop, err
	}
	file := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		cleanup()
		return noop, err
	}
	config.BuildFlags = append(config.BuildFlags, "-overlay="+file)
	config.Overlay = nil
	config.ParseFile = func(fset *token.FileSet, filename string, source []byte) (*ast.File, error) {
		canonical, err := filepath.EvalSymlinks(filename)
		if err != nil {
			return nil, err
		}
		if current, ok := contents[canonical]; ok {
			source = current
		}
		return parser.ParseFile(fset, filename, source, parser.AllErrors|parser.ParseComments)
	}
	return cleanup, nil
}
