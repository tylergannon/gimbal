// Run the skill's structural retrieval benchmarks over a filesystem index.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var linkRE = regexp.MustCompile(`\[.*?\]\(([^)#?]+)\)`)
var wordRE = regexp.MustCompile(`[\pL\pN_]{4,}`)
var textExtensions = map[string]bool{".md": true, ".yaml": true, ".yml": true, ".json": true, ".jsonl": true, ".txt": true, ".rst": true, ".toml": true, ".xml": true}

type eval struct {
	ID        string   `json:"id"`
	Query     string   `json:"query"`
	Routes    []string `json:"expected_routes"`
	Citations []string `json:"expected_citations"`
	Families  []string `json:"acceptable_families"`
	Notes     any      `json:"notes"`
}
type step struct {
	file         string
	depth, score int
}

func safeRead(path string) string {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.ToValidUTF8(string(bytes), "")
	runes := []rune(text)
	if len(runes) > 500000 {
		runes = runes[:500000]
	}
	return string(runes)
}
func entrypoint(index string) string {
	for _, name := range []string{"README.md", "readme.md", "INDEX.md", "index.md", "TAXONOMY.md"} {
		path := filepath.Join(index, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
func contained(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func resolveLink(raw, from, index string) string {
	if real, err := filepath.EvalSymlinks(index); err == nil {
		index = real
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "http") {
		return ""
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(from), path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !contained(index, resolved) || !textExtensions[filepath.Ext(resolved)] {
		return ""
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return resolved
}
func route(start, index string, terms map[string]bool, maxHops, maxFiles int) []step {
	type item struct {
		path  string
		depth int
	}
	queue := []item{{start, 0}}
	seen := map[string]bool{}
	steps := []step{}
	for len(queue) > 0 && len(steps) < maxFiles {
		next := queue[0]
		queue = queue[1:]
		rel, err := filepath.Rel(index, next.path)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		text := safeRead(next.path)
		lower := strings.ToLower(text)
		score := 0
		for term := range terms {
			if strings.Contains(lower, term) {
				score++
			}
		}
		steps = append(steps, step{rel, next.depth, score})
		if next.depth < maxHops && text != "" {
			for _, match := range linkRE.FindAllStringSubmatch(text, -1) {
				path := resolveLink(match[1], next.path, index)
				if path != "" {
					queue = append(queue, item{path, next.depth + 1})
				}
			}
		}
	}
	return steps
}
func scoreQuery(record eval, index string, maxHops, maxFiles int) map[string]any {
	if real, err := filepath.EvalSymlinks(index); err == nil {
		index = real
	}
	if record.ID == "" {
		record.ID = "?"
	}
	start := entrypoint(index)
	if start == "" {
		return map[string]any{"id": record.ID, "query": record.Query, "error": "no_entrypoint", "success_at_1": false, "success_at_3": false, "citation_precision": 0.0, "route_found": false, "files_read": 0}
	}
	terms := map[string]bool{}
	for _, word := range wordRE.FindAllString(record.Query, -1) {
		terms[strings.ToLower(word)] = true
	}
	steps := route(start, index, terms, maxHops, maxFiles)
	routeHit := len(record.Routes) == 0 && len(steps) > 0
	for _, expected := range record.Routes {
		for _, visited := range steps {
			if strings.Contains(visited.file, expected) || strings.HasSuffix(visited.file, expected) {
				routeHit = true
			}
		}
	}
	hits := 0
	for _, citation := range record.Citations {
		path := citation
		if !filepath.IsAbs(path) {
			path = filepath.Join(index, path)
		}
		_, err := os.Stat(path)
		hit := err == nil
		for _, visited := range steps {
			if visited.file == citation || strings.HasSuffix(visited.file, citation) {
				hit = true
			}
		}
		if !hit {
			for _, family := range record.Families {
				if strings.HasPrefix(citation, family) {
					hit = true
				}
			}
		}
		if hit {
			hits++
		}
	}
	denominator := len(record.Citations)
	if denominator < 1 {
		denominator = 1
	}
	success3 := hits >= min(3, len(record.Citations))
	if len(record.Citations) == 0 {
		success3 = hits >= 1
	}
	toolCalls := 0
	var first any
	for _, visited := range steps {
		if visited.score > 0 {
			toolCalls++
			if first == nil {
				first = visited.depth
			}
		}
	}
	if toolCalls == 0 {
		toolCalls = len(steps)
	}
	citations := record.Citations
	if citations == nil {
		citations = []string{}
	}
	return map[string]any{"id": record.ID, "query": record.Query, "success_at_1": hits >= 1, "success_at_3": success3, "citation_precision": round(float64(hits)/float64(denominator), 2), "route_found": routeHit, "files_read": len(steps), "tool_call_estimate": toolCalls, "path_length_to_first_hit": first, "expected_citations": citations, "citations_matched": hits, "notes": record.Notes}
}
func round(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.RoundToEven(value*factor) / factor
}
func aggregate(results []map[string]any) map[string]any {
	if len(results) == 0 {
		return map[string]any{}
	}
	var success1, success3, precision, routeHits, calls, lengths float64
	callCount, lengthCount := 0, 0
	misses := []string{}
	for _, result := range results {
		if result["success_at_1"].(bool) {
			success1++
		} else {
			misses = append(misses, result["id"].(string))
		}
		if result["success_at_3"].(bool) {
			success3++
		}
		precision += result["citation_precision"].(float64)
		if result["route_found"].(bool) {
			routeHits++
		}
		if value, exists := result["tool_call_estimate"]; exists && value != nil {
			calls += float64(value.(int))
			callCount++
		} else if !exists {
			callCount++
		}
		if value := result["path_length_to_first_hit"]; value != nil {
			lengths += float64(value.(int))
			lengthCount++
		}
	}
	var meanCalls, meanLength any
	if callCount > 0 {
		meanCalls = round(calls/float64(callCount), 2)
	}
	if lengthCount > 0 {
		meanLength = round(lengths/float64(lengthCount), 2)
	}
	n := float64(len(results))
	return map[string]any{"eval_count": len(results), "success_at_1": round(success1/n, 3), "success_at_3": round(success3/n, 3), "mean_citation_precision": round(precision/n, 3), "route_hit_rate": round(routeHits/n, 3), "mean_tool_call_estimate": meanCalls, "mean_path_length": meanLength, "misses": misses}
}
func loadEvals(path string, warnings io.Writer) ([]eval, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 65536), 16*1024*1024)
	entries := []eval{}
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") || strings.HasPrefix(text, "//") {
			continue
		}
		var entry eval
		if err := json.Unmarshal([]byte(text), &entry); err != nil {
			if _, err := fmt.Fprintf(warnings, "  [warn] skipping evals line %d: %v\n", line, err); err != nil {
				return nil, err
			}
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}
func absolute(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}
func emit(out io.Writer, result any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("run_evals", flag.ContinueOnError)
	indexArg := fs.String("index", "", "absolute semantic index root (required)")
	evalPath := fs.String("evals", "", "absolute eval JSONL path")
	maxHops := fs.Int("max-hops", 6, "max routing hops")
	maxFiles := fs.Int("max-files", 30, "max files read")
	write := fs.Bool("write-results", false, "write benchmark report")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *indexArg == "" {
		return fmt.Errorf("--index is required")
	}
	index, err := absolute(*indexArg)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(index); err == nil {
		index = resolved
	}
	path := filepath.Join(index, ".semantic-index/evals.jsonl")
	if *evalPath != "" {
		path, err = absolute(*evalPath)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(path); err != nil {
		if writeErr := emit(out, map[string]string{"error": fmt.Sprintf("No eval file found at %s or %s. Create one first.", path, filepath.Join(index, "evals.md"))}); writeErr != nil {
			return writeErr
		}
		return fmt.Errorf("eval file unavailable")
	}
	entries, err := loadEvals(path, out)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if err := emit(out, map[string]string{"error": "Eval file exists but contains no valid entries."}); err != nil {
			return err
		}
		return fmt.Errorf("no valid eval entries")
	}
	results := []map[string]any{}
	for _, entry := range entries {
		results = append(results, scoreQuery(entry, index, *maxHops, *maxFiles))
	}
	report := map[string]any{"run_at": time.Now().UTC().Format("2006-01-02T15:04:05+00:00"), "index": index, "evals_path": path, "aggregate": aggregate(results), "results": results}
	if err := emit(out, report); err != nil {
		return err
	}
	if *write {
		dir := filepath.Join(index, ".semantic-index/benchmarks")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		path := filepath.Join(dir, "run-"+time.Now().UTC().Format("20060102T150405Z")+".json")
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		writeErr := emit(file, report)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		_, err = fmt.Fprintln(out, "\n[benchmarks] Written to", path)
		return err
	}
	return nil
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
