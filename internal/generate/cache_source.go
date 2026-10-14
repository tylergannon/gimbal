package generate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// Metadata remains live on hits. Go selects files, validates imports and builds
// immutable export artifacts; the cache skips the root checker and extraction.
// Roots are parsed from captured bytes on misses. Dependency source imports are
// computed from captured source, not inferred from compiler export data.
type analysisInputs struct {
	dir            string
	patterns       []string
	overlay, roots map[string][]byte
	imports        map[string]bool
	manifest       inputManifest
	rootIDs        map[string]bool
	bound          bool
	goCache        string
}
type inputManifest struct {
	Dir           string
	Patterns      []string
	Environment   []string
	Toolchain     json.RawMessage
	Packages      []packageIdentity
	Sources       map[string]string
	Overlays      map[string]string
	SourceImports map[string]bool
}
type packageIdentity struct {
	ID, Path, Name, Export string
	Files                  []string
	Module                 *packages.Module
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func cacheSourceSupported() bool {
	if driver := os.Getenv("GOPACKAGESDRIVER"); driver != "" && driver != "off" {
		return false
	}
	if os.Getenv("GOPACKAGESDRIVER") != "off" {
		if _, err := exec.LookPath("gopackagesdriver"); err == nil {
			return false
		}
	}
	flags := os.Getenv("GOFLAGS")
	return !strings.Contains(flags, "-overlay") && !strings.Contains(flags, "-toolexec")
}

func cacheEnvironment() []string {
	var result []string
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(name, "GO") || strings.HasPrefix(name, "CGO_") || name == "PATH" || name == "HOME" || name == "CC" || name == "CXX" || name == "PKG_CONFIG" {
			result = append(result, item)
		}
	}
	slices.Sort(result)
	return result
}

func captureAnalysisInputs(dir string, overlay map[string][]byte, patterns []string) (*analysisInputs, error) {
	if !cacheSourceSupported() {
		return nil, nil
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	in := &analysisInputs{dir: dir, patterns: slices.Clone(patterns), overlay: maps.Clone(overlay), roots: map[string][]byte{}, imports: map[string]bool{}, rootIDs: map[string]bool{}}
	in.manifest = inputManifest{Dir: dir, Patterns: patterns, Environment: cacheEnvironment(), Sources: map[string]string{}, Overlays: map[string]string{}}
	// Include effective settings, including defaults written through go env -w
	// and automatic toolchain selection, rather than only process overrides.
	env := exec.Command("go", "env", "-json", "GOOS", "GOARCH", "GOAMD64", "GOARM", "GOARM64", "GO386", "GOEXPERIMENT", "GOVERSION", "GOROOT", "GOTOOLDIR", "GOTOOLCHAIN", "GOWORK", "GOFLAGS", "GOCACHE", "CGO_ENABLED", "CC", "CXX")
	env.Dir = dir
	toolchain, err := env.Output()
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(toolchain), "-overlay") || strings.Contains(string(toolchain), "-toolexec") {
		return nil, nil
	}
	in.manifest.Toolchain = toolchain
	var effective struct{ GOCACHE string }
	if json.Unmarshal(toolchain, &effective) != nil || !filepath.IsAbs(effective.GOCACHE) {
		return nil, nil
	}
	in.goCache = effective.GOCACHE
	in.manifest.SourceImports = in.imports
	for path, data := range overlay {
		in.manifest.Overlays[path] = hashBytes(data)
	}
	config := &packages.Config{Dir: dir, Overlay: maps.Clone(overlay), Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedModule | packages.NeedExportFile}
	cleanup, err := preserveExports(config)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	roots, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, err
	}
	var modulePaths []string
	for _, p := range roots {
		in.rootIDs[p.ID] = true
		if p.Module != nil {
			modulePaths = append(modulePaths, p.Module.Path)
		}
		// cgo and other compiler-generated root syntax need their own acquisition
		// contract. Keep the ordinary loader for these packages.
		for _, path := range p.CompiledGoFiles {
			if !slices.Contains(p.GoFiles, path) {
				return nil, nil
			}
		}
	}
	var problem error
	packages.Visit(roots, nil, func(p *packages.Package) {
		if problem != nil {
			return
		}
		if len(p.Errors) != 0 {
			problem = fmt.Errorf("generate: %s: %s", p.PkgPath, p.Errors[0])
			return
		}
		identity, err := in.identity(p)
		if err != nil {
			problem = err
			return
		}
		in.manifest.Packages = append(in.manifest.Packages, identity)
		captureImports := slices.ContainsFunc(modulePaths, func(path string) bool { return strings.HasPrefix(p.PkgPath, path) })
		if !in.rootIDs[p.ID] && !captureImports {
			return
		}
		in.imports[p.PkgPath] = false
		for _, path := range p.GoFiles {
			data, ok := overlay[path]
			if !ok {
				data, err = os.ReadFile(path)
			}
			if err != nil {
				problem = err
				return
			}
			if in.rootIDs[p.ID] && !packageOnlyOverlay(path, overlay) {
				in.manifest.Sources[path] = hashBytes(data)
				in.roots[path] = data
			}
			if captureImports {
				file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.ImportsOnly)
				if err != nil {
					problem = err
					return
				}
				for _, imp := range file.Imports {
					value, _ := strconv.Unquote(imp.Path.Value)
					if value == gimbalPath {
						in.imports[p.PkgPath] = true
					}
				}
			}
		}
	})
	if problem != nil {
		return nil, problem
	}
	slices.SortFunc(in.manifest.Packages, func(a, b packageIdentity) int { return strings.Compare(a.ID, b.ID) })
	return in, nil
}

func packageOnlyOverlay(path string, overlay map[string][]byte) bool {
	data, ok := overlay[path]
	if !ok {
		return false
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
	return err == nil && len(file.Decls) == 0
}

func (in *analysisInputs) identity(p *packages.Package) (packageIdentity, error) {
	id := packageIdentity{ID: p.ID, Path: p.PkgPath, Name: p.Name, Module: p.Module}
	for _, path := range p.CompiledGoFiles {
		if !packageOnlyOverlay(path, in.overlay) {
			id.Files = append(id.Files, path)
		}
	}
	if !in.rootIDs[p.ID] && p.PkgPath != "unsafe" {
		if p.ExportFile == "" {
			return id, fmt.Errorf("generate: %s has no immutable compiler export", p.PkgPath)
		}
		var err error
		id.Export, err = compilerExportID(in.goCache, p.ExportFile)
		if err != nil {
			return id, err
		}
	}
	return id, nil
}

// Go's disk cache names outputs by the SHA-256 of their contents (cmd/go/internal/cache).
// Use that immutable identity instead of reading every standard-library archive
// again. Alternative export stores have no such contract here and bypass caching.
func compilerExportID(cacheDir, path string) (string, error) {
	name := filepath.Base(path)
	if len(name) == 66 && strings.HasSuffix(name, "-d") {
		id := name[:64]
		if _, err := hex.DecodeString(id); err == nil && path == filepath.Join(cacheDir, id[:2], name) {
			return id, nil
		}
	}
	return "", fmt.Errorf("generate: unsupported compiler export location %s", path)
}

func (in *analysisInputs) load() (*sourcePackages, error) {
	overlay := maps.Clone(in.overlay)
	if overlay == nil {
		overlay = map[string][]byte{}
	}
	for path, data := range in.roots {
		overlay[path] = data
	}
	config := &packages.Config{Dir: in.dir, Overlay: overlay, Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedModule | packages.NeedExportFile}
	cleanup, err := preserveExports(config)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	parse := config.ParseFile
	if parse == nil {
		parse = func(f *token.FileSet, p string, b []byte) (*ast.File, error) {
			return parser.ParseFile(f, p, b, parser.AllErrors|parser.ParseComments)
		}
	}
	var mu sync.Mutex
	seen := map[string]bool{}
	bound := true
	config.ParseFile = func(fset *token.FileSet, filename string, data []byte) (*ast.File, error) {
		mu.Lock()
		if captured, ok := in.roots[filename]; ok {
			data = captured
			seen[filename] = true
		} else if !packageOnlyOverlay(filename, in.overlay) {
			bound = false
		}
		mu.Unlock()
		return parse(fset, filename, data)
	}
	roots, err := loadSourceTypes(config, in.patterns...)
	if err != nil {
		return nil, err
	}
	var identities []packageIdentity
	packages.Visit(roots, nil, func(p *packages.Package) {
		id, e := in.identity(p)
		if e != nil || len(p.Errors) > 0 {
			bound = false
		}
		identities = append(identities, id)
	})
	slices.SortFunc(identities, func(a, b packageIdentity) int { return strings.Compare(a.ID, b.ID) })
	a, _ := json.Marshal(identities)
	b, _ := json.Marshal(in.manifest.Packages)
	in.bound = bound && len(seen) == len(in.roots) && string(a) == string(b)
	return &sourcePackages{roots: roots, imports: in.imports}, nil
}
