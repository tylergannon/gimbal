// Package semanticindex builds a local semantic routing tree over a read-only
// source corpus. The corpus is never modified and the output is committed only
// after every generated citation and artifact has passed validation.
package semanticindex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/codex"
)

//go:generate go tool polytype --validate

// Input selects a local corpus and index output.
type Input struct {
	Source string `json:"source"`
	Output string `json:"output"`
	Model  string `json:"model"`
	Mode   string `json:"mode"`
	DryRun bool   `json:"dry_run"`
}

type Citation struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Note      string `json:"note"`
}

type readerOutput struct {
	Summary   string     `json:"summary"`
	Themes    []string   `json:"themes"`
	Citations []Citation `json:"citations"`
	Gotchas   []string   `json:"gotchas"`
	Recipes   []string   `json:"recipes"`
}

type sourceState struct {
	Hash       string     `json:"hash"`
	Leaf       string     `json:"leaf"`
	Themes     []string   `json:"themes,omitempty"`
	Citations  []Citation `json:"citations,omitempty"`
	Recipes    []string   `json:"recipes,omitempty"`
	Route      string     `json:"route,omitempty"`
	RouteChain []string   `json:"route_chain,omitempty"`
}

var _ gimble.Output = readerOutput{}

type state struct {
	Source    string                 `json:"source"`
	BuiltAt   string                 `json:"built_at"`
	Mode      string                 `json:"mode"`
	Sources   map[string]sourceState `json:"sources"`
	FileCount int                    `json:"file_count"`
	Debt      []string               `json:"debt"`
	Routes    []string               `json:"routes,omitempty"`
}

type sourceFile struct {
	Rel   string
	Path  string
	Hash  string
	Lines int
}

const (
	defaultModel   = "gpt-5.6-luna"
	defaultMode    = "auto"
	stateFile      = ".semantic-index/state.json"
	maxSourceBytes = 256 * 1024
)

// Build creates or updates an index and returns its absolute README path.
// It assumes ctx belongs to a gimble.Run.
func Build(ctx context.Context, in Input, out io.Writer) (string, error) {
	return build(ctx, in, out, codex.New())
}

func build(ctx context.Context, in Input, out io.Writer, adapter gimble.HarnessAdapter) (string, error) {
	if err := normalize(&in); err != nil {
		return "", err
	}
	if err := validatePaths(in); err != nil {
		return "", err
	}
	realSource, err := filepath.EvalSymlinks(in.Source)
	if err != nil {
		return "", err
	}
	in.Source = realSource
	in.Output, err = resolvedPath(in.Output)
	if err != nil {
		return "", err
	}
	gimble.Set(ctx, "source_root", in.Source)
	gimble.Set(ctx, "index_output", in.Output)
	if in.Mode == "auto" {
		if _, err := os.Stat(filepath.Join(in.Output, stateFile)); err == nil {
			in.Mode = "update"
		} else if errors.Is(err, os.ErrNotExist) {
			in.Mode = "build"
		} else {
			return "", err
		}
	}
	if err := validateOutput(in.Output); err != nil {
		return "", err
	}
	if in.DryRun {
		_, _ = fmt.Fprintf(out, "semantic index: %s -> %s (%s)\n", in.Source, in.Output, in.Mode)
		return filepath.Join(in.Output, "README.md"), nil
	}
	files, debt, err := readSources(ctx, in.Source)
	if err != nil {
		return "", err
	}
	if in.Mode == "audit" {
		return audit(in, files, debt, out)
	}
	previous := state{Sources: map[string]sourceState{}}
	if _, err := os.Stat(filepath.Join(in.Output, stateFile)); err == nil {
		previous, err = readState(in.Output, in.Source)
		if err != nil {
			return "", fmt.Errorf("semanticindex: existing index is invalid: %w", err)
		}
		if in.Mode == "update" {
			if err := validateIndexedFiles(in.Output, previous, true); err != nil {
				return "", err
			}
		}
	} else if in.Mode == "update" {
		return "", fmt.Errorf("semanticindex: update requires an existing index: %w", err)
	}
	ownedRoutes := append([]string(nil), previous.Routes...)
	stage, err := os.MkdirTemp(filepath.Dir(in.Output), ".semantic-index-stage-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if err := copyDir(in.Output, stage); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if in.Mode == "build" {
		if err := os.RemoveAll(filepath.Join(stage, "sources")); err != nil {
			return "", err
		}
		previous = state{Sources: map[string]sourceState{}}
	}

	if err := os.MkdirAll(filepath.Join(stage, "sources"), 0o755); err != nil {
		return "", err
	}
	next := state{Source: in.Source, BuiltAt: time.Now().UTC().Format(time.RFC3339), Mode: in.Mode, Sources: map[string]sourceState{}, FileCount: len(files), Debt: debt}
	changed := make([]sourceFile, 0)
	for _, file := range files {
		leaf := filepath.ToSlash(filepath.Join("sources", leafName(file.Rel)))
		next.Sources[file.Rel] = sourceState{Hash: file.Hash, Leaf: leaf}
		old, unchanged := previous.Sources[file.Rel]
		if unchanged && old.Hash == file.Hash && old.Leaf == leaf {
			if _, err := os.Stat(filepath.Join(stage, filepath.FromSlash(leaf))); err != nil {
				changed = append(changed, file)
				continue
			}
			saved := next.Sources[file.Rel]
			saved.Themes = old.Themes
			saved.Citations = old.Citations
			saved.Recipes = old.Recipes
			next.Sources[file.Rel] = saved
			continue
		}
		changed = append(changed, file)
	}
	results := make(map[string]readerOutput, len(changed))
	if err := readChanged(ctx, in, adapter, changed, results); err != nil {
		return "", err
	}
	for rel, old := range previous.Sources {
		if _, ok := next.Sources[rel]; !ok {
			_ = os.Remove(filepath.Join(stage, filepath.FromSlash(old.Leaf)))
		}
	}
	for _, file := range changed {
		leaf := next.Sources[file.Rel].Leaf
		result := results[file.Rel]
		if err := validateReaderResult(in.Source, file, result); err != nil {
			return "", err
		}
		saved := next.Sources[file.Rel]
		saved.Themes = result.Themes
		saved.Citations = result.Citations
		saved.Recipes = result.Recipes
		next.Sources[file.Rel] = saved
		if err := writeLeaf(filepath.Join(stage, filepath.FromSlash(leaf)), file, result); err != nil {
			return "", err
		}
	}
	if err := writeRoutes(stage, files, &next, results, ownedRoutes); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(stage, stateFile), next); err != nil {
		return "", err
	}
	if err := writeEvals(stage, files, next); err != nil {
		return "", err
	}
	if err := writeREADME(stage, in, next); err != nil {
		return "", err
	}
	if err := verifySources(ctx, in.Source, files, debt); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := install(stage, in.Output); err != nil {
		return "", err
	}
	_, _ = fmt.Fprintf(out, "semantic index: %s (%d indexed sources, %d source debt)\n", filepath.Join(in.Output, "README.md"), len(files), len(debt))
	return filepath.Join(in.Output, "README.md"), nil
}

func readChanged(ctx context.Context, in Input, adapter gimble.HarnessAdapter, files []sourceFile, results map[string]readerOutput) error {
	if len(files) <= 10 {
		for _, file := range files {
			result, err := readOne(ctx, in, adapter, file)
			if err != nil {
				return err
			}
			results[file.Rel] = result
		}
		return nil
	}
	for start := 0; start < len(files); start += 3 {
		end := min(start+3, len(files))
		var mu sync.Mutex
		group := gimble.Group(ctx, "readers")
		for _, file := range files[start:end] {
			group.Go("reader", func(ctx context.Context) error {
				result, err := readOne(ctx, in, adapter, file)
				if err != nil {
					return err
				}
				mu.Lock()
				results[file.Rel] = result
				mu.Unlock()
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func normalize(in *Input) error {
	if strings.TrimSpace(in.Source) == "" || strings.TrimSpace(in.Output) == "" {
		return errors.New("semanticindex: source and output are required")
	}
	if in.Model == "" {
		in.Model = defaultModel
	}
	if in.Mode == "" || in.Mode == "auto" {
		in.Mode = defaultMode
	}
	if in.Mode != "auto" && in.Mode != "build" && in.Mode != "update" && in.Mode != "audit" {
		return fmt.Errorf("semanticindex: unsupported mode %q", in.Mode)
	}
	var err error
	if in.Source, err = filepath.Abs(in.Source); err != nil {
		return err
	}
	if in.Output, err = filepath.Abs(in.Output); err != nil {
		return err
	}
	return nil
}

func validatePaths(in Input) error {
	source, err := filepath.EvalSymlinks(in.Source)
	if err != nil {
		return fmt.Errorf("semanticindex: source: %w", err)
	}
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("semanticindex: source must be a directory")
	}
	output, err := resolvedPath(in.Output)
	if err != nil {
		return err
	}
	if sameOrInside(source, output) || sameOrInside(output, source) {
		return errors.New("semanticindex: source and output overlap, including through symlinks")
	}
	return nil
}

// resolvedPath follows every existing ancestor, including a symlink above a
// nonexistent suffix. This keeps direct Build calls from aliasing the corpus.
func resolvedPath(path string) (string, error) {
	path = filepath.Clean(path)
	var suffix []string
	for {
		real, err := filepath.EvalSymlinks(path)
		if err == nil {
			for _, s := range slices.Backward(suffix) {
				real = filepath.Join(real, s)
			}
			return real, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}

func validateOutput(output string) error {
	entries, err := os.ReadDir(output)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(output, "README.md")); err != nil {
		return errors.New("semanticindex: output exists but is not an owned semantic index")
	}
	if _, err := os.Stat(filepath.Join(output, stateFile)); err != nil {
		return errors.New("semanticindex: output is missing semantic-index state")
	}
	return nil
}

func sameOrInside(parent, path string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

func readSources(ctx context.Context, root string) ([]sourceFile, []string, error) {
	var files []sourceFile
	var debt []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			debt = append(debt, rel+": symlink unsupported")
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			debt = append(debt, rel+": non-regular file unsupported")
			return nil
		}
		if info.Size() > maxSourceBytes {
			debt = append(debt, rel+": exceeds 256 KiB reader limit")
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) > maxSourceBytes {
			debt = append(debt, rel+": exceeds 256 KiB reader limit")
			return nil
		}
		if !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
			debt = append(debt, rel+": binary or non-UTF-8 source unsupported")
			return nil
		}
		h := sha256.Sum256(data)
		files = append(files, sourceFile{Rel: rel, Path: path, Hash: hex.EncodeToString(h[:]), Lines: 1 + strings.Count(string(data), "\n")})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	sort.Strings(debt)
	return files, debt, err
}

func leafName(rel string) string {
	h := sha256.Sum256([]byte(rel))
	return hex.EncodeToString(h[:8]) + ".md"
}

func readOne(ctx context.Context, in Input, adapter gimble.HarnessAdapter, file sourceFile) (readerOutput, error) {
	var result readerOutput
	err := gimble.Scope(ctx, "source", func(ctx context.Context) error {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return err
		}
		if len(data) > maxSourceBytes {
			return fmt.Errorf("semanticindex: source grew before reader: %s", file.Rel)
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != file.Hash {
			return fmt.Errorf("semanticindex: source changed before reader: %s", file.Rel)
		}
		gimble.Set(ctx, "source_path", file.Rel)
		gimble.Set(ctx, "source_hash", file.Hash)
		session := gimble.NewSession(ctx, "reader", adapter, in.Model, in.Source)
		prompt := fmt.Sprintf("Read this local source file and produce a dense semantic-index leaf. Cite only %s using path and 1-based line ranges. Include task-oriented themes, gotchas, and recipes. Do not modify files.\n\n%s\n\nSOURCE CONTENT:\n%s", file.Rel, gimble.ScopeText(ctx), string(data))
		result, err = session.Generate[readerOutput](ctx, prompt)
		return err
	})
	return result, err
}

func validateReaderResult(_ string, file sourceFile, result readerOutput) error {
	if strings.TrimSpace(result.Summary) == "" {
		return fmt.Errorf("semanticindex: empty summary for %s", file.Rel)
	}
	if len(result.Citations) == 0 {
		return fmt.Errorf("semanticindex: no citations for %s", file.Rel)
	}
	for _, theme := range result.Themes {
		if strings.TrimSpace(theme) == "" || strings.ContainsAny(theme, "\r\n[]()") {
			return fmt.Errorf("semanticindex: invalid theme for %s", file.Rel)
		}
	}
	for _, citation := range result.Citations {
		if filepath.ToSlash(citation.Path) != file.Rel {
			return fmt.Errorf("semanticindex: citation %q escapes assigned source %s", citation.Path, file.Rel)
		}
		if citation.StartLine < 1 || citation.EndLine < citation.StartLine {
			return fmt.Errorf("semanticindex: invalid citation lines for %s", file.Rel)
		}
		if citation.EndLine > file.Lines || strings.TrimSpace(citation.Note) == "" {
			return fmt.Errorf("semanticindex: invalid citation target for %s", file.Rel)
		}
	}
	return nil
}

func writeLeaf(path string, file sourceFile, result readerOutput) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nPurpose: %s\n\n## Themes\n", file.Rel, result.Summary)
	for _, theme := range result.Themes {
		fmt.Fprintf(&b, "- %s\n", theme)
	}
	b.WriteString("\n## Citations\n")
	for _, c := range result.Citations {
		fmt.Fprintf(&b, "- `%s:%d-%d` — %s\n", c.Path, c.StartLine, c.EndLine, c.Note)
	}
	if len(result.Gotchas) > 0 {
		b.WriteString("\n## Gotchas\n")
		for _, v := range result.Gotchas {
			fmt.Fprintf(&b, "- %s\n", v)
		}
	}
	if len(result.Recipes) > 0 {
		b.WriteString("\n## Recipes\n")
		for _, v := range result.Recipes {
			fmt.Fprintf(&b, "- %s\n", v)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeRoutes(stage string, files []sourceFile, next *state, results map[string]readerOutput, ownedRoutes []string) error {
	themes := map[string][]string{}
	labels := map[string]string{}
	for _, file := range files {
		leaf := next.Sources[file.Rel].Leaf
		labels[leaf] = file.Rel
		if result, ok := results[file.Rel]; ok && len(result.Themes) > 0 {
			for _, theme := range result.Themes {
				themes["theme: "+theme] = append(themes["theme: "+theme], leaf)
			}
		} else if saved := next.Sources[file.Rel].Themes; len(saved) > 0 {
			for _, theme := range saved {
				themes["theme: "+theme] = append(themes["theme: "+theme], leaf)
			}
		} else {
			themes["source family: "+filepath.Dir(file.Rel)] = append(themes["source family: "+filepath.Dir(file.Rel)], leaf)
		}
	}
	keys := make([]string, 0, len(themes))
	for k := range themes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	owned := map[string]bool{}
	for _, route := range ownedRoutes {
		owned[route] = true
		if err := os.Remove(filepath.Join(stage, filepath.FromSlash(route))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	next.Routes = nil
	for rel, entry := range next.Sources {
		entry.Route = ""
		entry.RouteChain = nil
		next.Sources[rel] = entry
	}
	page := 0
	newPage := func() string { page++; return fmt.Sprintf("routes/page-%04d.md", page) }
	writePage := func(path, content string) error {
		if strings.HasPrefix(path, "routes/") && !owned[path] {
			if _, err := os.Lstat(filepath.Join(stage, filepath.FromSlash(path))); err == nil {
				return fmt.Errorf("semanticindex: generated route path conflicts with unrelated file: %s", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(stage, path)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(stage, path), []byte(content), 0o644)
	}
	markLeaf := func(leaf string, chain []string) {
		for rel, entry := range next.Sources {
			if entry.Leaf == leaf && entry.Route == "" && len(chain) > 0 {
				entry.Route = chain[len(chain)-1]
				entry.RouteChain = append([]string(nil), chain...)
				next.Sources[rel] = entry
			}
		}
	}
	labelGroup := func(items []string) string {
		if len(items) <= 8 {
			return strings.Join(items, "; ")
		}
		return fmt.Sprintf("%s; %s; %s; %s; %s (%d choices)", items[0], items[1], items[len(items)/2], items[len(items)-2], items[len(items)-1], len(items))
	}
	var renderLeaves func(string, []string, string, []string) error
	renderLeaves = func(key string, leaves []string, path string, chain []string) error {
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\nChoose the source that matches your task, then inspect its citations.\n\n", key)
		if len(leaves) <= 8 {
			for _, leaf := range leaves {
				markLeaf(leaf, chain)
				href := leaf
				if path != "routes.md" {
					href = "../" + leaf
				}
				fmt.Fprintf(&b, "- [%s](%s)\n", labels[leaf], href)
			}
		} else {
			chunk := (len(leaves) + 7) / 8
			for start := 0; start < len(leaves); start += chunk {
				end := min(start+chunk, len(leaves))
				child := newPage()
				if err := renderLeaves(key, leaves[start:end], child, append(append([]string(nil), chain...), child)); err != nil {
					return err
				}
				next.Routes = append(next.Routes, child)
				href := child
				if path != "routes.md" {
					href = filepath.Base(child)
				}
				var sources []string
				for _, leaf := range leaves[start:end] {
					sources = append(sources, labels[leaf])
				}
				fmt.Fprintf(&b, "- [%s](%s)\n", labelGroup(sources), href)
			}
		}
		return writePage(path, b.String())
	}
	var render func([]string, string, []string) error
	render = func(section []string, path string, chain []string) error {
		var b strings.Builder
		b.WriteString("# Semantic routes\n\nChoose by task theme, then open a leaf and its cited source lines.\n\n")
		links := 0
		for _, key := range section {
			links += len(themes[key])
		}
		if links <= 8 {
			for _, key := range section {
				fmt.Fprintf(&b, "## %s\n\n", key)
				for _, leaf := range themes[key] {
					markLeaf(leaf, chain)
					href := leaf
					if path != "routes.md" {
						href = "../" + leaf
					}
					fmt.Fprintf(&b, "- [%s](%s)\n", labels[leaf], href)
				}
				b.WriteString("\n")
			}
		} else if len(section) == 1 {
			return renderLeaves(section[0], themes[section[0]], path, chain)
		} else {
			chunk := (len(section) + 7) / 8
			for start := 0; start < len(section); start += chunk {
				end := min(start+chunk, len(section))
				child := newPage()
				if err := render(section[start:end], child, append(append([]string(nil), chain...), child)); err != nil {
					return err
				}
				next.Routes = append(next.Routes, child)
				href := child
				if path != "routes.md" {
					href = filepath.Base(child)
				}
				fmt.Fprintf(&b, "- [%s](%s)\n", labelGroup(section[start:end]), href)
			}
		}
		return writePage(path, b.String())
	}
	if err := render(keys, "routes.md", nil); err != nil {
		return err
	}
	sort.Strings(next.Routes)
	return nil
}

func writeREADME(stage string, in Input, s state) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Semantic Index\n\nSource corpus: `%s`\n\nRoutes: [routes.md](routes.md)\n\nLeaves live under `sources/` and cite source paths with 1-based line ranges.\n\nBuilt: %s\n\nCoverage: %d indexed source files; %d unsupported or over-budget files.\n\nRetrieval queries: [.semantic-index/evals.jsonl](.semantic-index/evals.jsonl). Structural reachability does not prove useful retrieval; run a query against the actual routes and citations.\n\n", in.Source, s.BuiltAt, s.FileCount, len(s.Debt))
	if len(s.Debt) > 0 {
		b.WriteString("## Source debt\n\n")
		for _, v := range s.Debt {
			fmt.Fprintf(&b, "- %s\n", v)
		}
		b.WriteString("\n")
	}
	b.WriteString("Freshness and structural metrics are in `.semantic-index/state.json`; retrieval usefulness needs a native benchmark.\n")
	return os.WriteFile(filepath.Join(stage, "README.md"), []byte(b.String()), 0o644)
}

func writeEvals(stage string, files []sourceFile, s state) error {
	path := filepath.Join(stage, ".semantic-index", "evals.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var existing []byte
	if data, err := os.ReadFile(path); err == nil {
		existing = data
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var custom [][]byte
	for line := range strings.SplitSeq(strings.TrimSpace(string(existing)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var item struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return fmt.Errorf("semanticindex: malformed eval: %w", err)
		}
		if !strings.HasPrefix(item.ID, "starter-") {
			custom = append(custom, []byte(line))
		}
	}
	var generated [][]byte
	for i, file := range files {
		if i >= 5 {
			break
		}
		entry := s.Sources[file.Rel]
		if len(entry.Citations) == 0 {
			continue
		}
		query := "Where is " + file.Rel + " explained?"
		if len(entry.Recipes) > 0 {
			query = "How do I " + strings.TrimSpace(entry.Recipes[0]) + "?"
		} else if len(entry.Themes) > 0 {
			query = "What does the index say about " + strings.TrimSpace(entry.Themes[0]) + "?"
		}
		expected := []string{"routes.md"}
		if len(entry.RouteChain) > 0 {
			expected = append(expected, entry.RouteChain...)
		} else if entry.Route != "" {
			expected = append(expected, entry.Route)
		}
		expected = append(expected, entry.Leaf)
		item := struct {
			ID                string   `json:"id"`
			Query             string   `json:"query"`
			ExpectedRoutes    []string `json:"expected_routes"`
			ExpectedCitations []string `json:"expected_citations"`
			Notes             string   `json:"notes"`
		}{fmt.Sprintf("starter-%03d", i+1), query, expected, []string{fmt.Sprintf("%s:%d-%d", entry.Citations[0].Path, entry.Citations[0].StartLine, entry.Citations[0].EndLine)}, "Run against the actual routes and verify the cited source lines."}
		line, err := json.Marshal(item)
		if err != nil {
			return err
		}
		generated = append(generated, line)
	}
	lines := append(custom, generated...)
	if len(lines) == 0 {
		return os.WriteFile(path, nil, 0o644)
	}
	return os.WriteFile(path, append(bytesJoin(lines), '\n'), 0o644)
}

func bytesJoin(lines [][]byte) []byte {
	var b []byte
	for i, line := range lines {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, line...)
	}
	return b
}

func readState(dir, source string) (state, error) {
	var s state
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.Source != source {
		return s, fmt.Errorf("semanticindex: stored source %q differs from %q", s.Source, source)
	}
	if s.Sources == nil || s.FileCount != len(s.Sources) || s.BuiltAt == "" {
		return s, errors.New("semanticindex: corrupt source state")
	}
	for rel, entry := range s.Sources {
		if rel == "" || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) || entry.Hash == "" || entry.Leaf != "sources/"+leafName(rel) {
			return s, fmt.Errorf("semanticindex: corrupt source state for %q", rel)
		}
		if len(entry.Citations) == 0 {
			return s, fmt.Errorf("semanticindex: missing citation state for %q", rel)
		}
		for _, c := range entry.Citations {
			if c.Path != rel || c.StartLine < 1 || c.EndLine < c.StartLine || strings.TrimSpace(c.Note) == "" {
				return s, fmt.Errorf("semanticindex: corrupt citation state for %q", rel)
			}
		}
		if entry.Route != "" && !strings.HasPrefix(entry.Route, "routes/") {
			return s, fmt.Errorf("semanticindex: invalid route state for %q", rel)
		}
		for _, route := range entry.RouteChain {
			if !strings.HasPrefix(route, "routes/") || filepath.Clean(route) != route {
				return s, fmt.Errorf("semanticindex: invalid route chain for %q", rel)
			}
		}
		if len(entry.RouteChain) > 0 && entry.RouteChain[len(entry.RouteChain)-1] != entry.Route {
			return s, fmt.Errorf("semanticindex: inconsistent route chain for %q", rel)
		}
	}
	for _, route := range s.Routes {
		if !strings.HasPrefix(route, "routes/") || filepath.Clean(route) != route {
			return s, fmt.Errorf("semanticindex: invalid route state %q", route)
		}
	}
	return s, nil
}

func validateIndexedFiles(dir string, s state, allowMissingLeaf bool) error {
	visited := map[string]bool{}
	leaves := map[string]bool{}
	edges := map[string]map[string]bool{}
	queue := []string{"routes.md"}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if visited[path] {
			continue
		}
		visited[path] = true
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("semanticindex: missing route %s: %w", path, err)
		}
		for _, href := range markdownTargets(string(data)) {
			resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(href))))
			if edges[path] == nil {
				edges[path] = map[string]bool{}
			}
			edges[path][resolved] = true
			if strings.HasPrefix(resolved, "routes/") {
				queue = append(queue, resolved)
			} else if strings.HasPrefix(resolved, "sources/") {
				leaves[resolved] = true
			} else {
				return fmt.Errorf("semanticindex: route target escapes index: %q", href)
			}
		}
	}
	for _, path := range s.Routes {
		if !visited[path] {
			return fmt.Errorf("semanticindex: orphan route page %s", path)
		}
	}
	for rel, entry := range s.Sources {
		if !leaves[entry.Leaf] || entry.Route != "" && !visited[entry.Route] {
			return fmt.Errorf("semanticindex: orphan leaf for %s", rel)
		}
		if len(entry.RouteChain) > 0 {
			parent := "routes.md"
			for _, route := range entry.RouteChain {
				if !edges[parent][route] {
					return fmt.Errorf("semanticindex: broken route chain for %s", rel)
				}
				parent = route
			}
			if !edges[parent][entry.Leaf] {
				return fmt.Errorf("semanticindex: route does not lead to leaf for %s", rel)
			}
		}
		leaf, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(entry.Leaf)))
		if err != nil {
			if allowMissingLeaf && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("semanticindex: missing leaf for %s: %w", rel, err)
		}
		for _, c := range entry.Citations {
			anchor := fmt.Sprintf("`%s:%d-%d`", c.Path, c.StartLine, c.EndLine)
			if !strings.Contains(string(leaf), anchor) {
				return fmt.Errorf("semanticindex: missing citation in leaf for %s", rel)
			}
		}
	}
	return nil
}

func markdownTargets(data string) []string {
	var targets []string
	for {
		start := strings.Index(data, "](")
		if start < 0 {
			break
		}
		data = data[start+2:]
		end := strings.IndexByte(data, ')')
		if end < 0 {
			break
		}
		targets = append(targets, data[:end])
		data = data[end+1:]
	}
	return targets
}

func verifySources(ctx context.Context, root string, original []sourceFile, debt []string) error {
	current, currentDebt, err := readSources(ctx, root)
	if err != nil {
		return err
	}
	if len(current) != len(original) || len(currentDebt) != len(debt) {
		return errors.New("semanticindex: source corpus changed during build")
	}
	for i := range current {
		if current[i].Rel != original[i].Rel || current[i].Hash != original[i].Hash {
			return fmt.Errorf("semanticindex: source changed during build: %s", original[i].Rel)
		}
	}
	for i := range debt {
		if currentDebt[i] != debt[i] {
			return errors.New("semanticindex: source debt changed during build")
		}
	}
	return nil
}

func audit(in Input, files []sourceFile, debt []string, out io.Writer) (string, error) {
	s, err := readState(in.Output, in.Source)
	if err != nil {
		return "", err
	}
	if err := validateIndexedFiles(in.Output, s, false); err != nil {
		return "", err
	}
	current := make(map[string]sourceFile, len(files))
	for _, file := range files {
		current[file.Rel] = file
	}
	stale, deleted, added, citations := 0, 0, 0, 0
	for rel, entry := range s.Sources {
		file, ok := current[rel]
		if !ok {
			deleted++
			continue
		}
		if file.Hash != entry.Hash {
			stale++
		}
		for _, c := range entry.Citations {
			citations++
			if c.EndLine > file.Lines {
				stale++
			}
		}
	}
	for rel := range current {
		if _, ok := s.Sources[rel]; !ok {
			added++
		}
	}
	debtChanged := len(debt) != len(s.Debt)
	if !debtChanged {
		for i := range debt {
			if debt[i] != s.Debt[i] {
				debtChanged = true
				break
			}
		}
	}
	_, _ = fmt.Fprintf(out, "semantic index audit: %s\nindexed=%d current=%d citations=%d stale=%d deleted=%d added=%d source_debt=%d debt_changed=%t\nstructural routes and leaves checked; retrieval usefulness requires a native query benchmark\n", filepath.Join(in.Output, "README.md"), len(s.Sources), len(files), citations, stale, deleted, added, len(debt), debtChanged)
	if stale+deleted+added > 0 || debtChanged {
		return "", errors.New("semanticindex: audit found stale coverage; run update")
	}
	return filepath.Join(in.Output, "README.md"), nil
}
func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func copyDir(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("semanticindex: output symlink is not owned: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}

func install(stage, output string) error {
	parent, err := os.MkdirTemp(filepath.Dir(output), ".semantic-index-backup-")
	if err != nil {
		return err
	}
	old := filepath.Join(parent, filepath.Base(output))
	defer func() { _ = os.RemoveAll(parent) }()
	if _, err := os.Stat(output); err == nil {
		if err := os.Rename(output, old); err != nil {
			return err
		}
	}
	if err := os.Rename(stage, output); err != nil {
		_ = os.Rename(old, output)
		return err
	}
	_ = os.RemoveAll(old)
	return nil
}
