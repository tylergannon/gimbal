// Report the skill's structural metrics for a filesystem semantic index.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var textExtensions = map[string]bool{".adoc": true, ".csv": true, ".html": true, ".json": true, ".jsonl": true, ".md": true, ".rst": true, ".sqlite": true, ".toml": true, ".tsv": true, ".txt": true, ".xml": true, ".yaml": true, ".yml": true}
var routingNames = map[string]bool{"readme.md": true, "index.md": true, "semantic_index.md": true, "semantic-index.md": true, "taxonomy.md": true, "topics.md": true, "themes.md": true, "routes.md": true, "routes.yaml": true, "routes.yml": true, "manifest.json": true}
var citationRE = regexp.MustCompile(`(?:(?:[\pL\pN_./@+-]+\.(?:md|txt|rst|py|js|ts|tsx|go|rs|ex|exs|java|c|cc|cpp|h|hpp|html|pdf|docx?))(?::L?\d+(?:-L?\d+)?|\s+p{1,2}\.?\s*\d+(?:-\d+)?)|https?://\S+)`)
var debtRE = regexp.MustCompile(`(?i)\b(TODO|FIXME|debt|stale|orphan|manual review|unknown|blocked)\b`)
var linkRE = regexp.MustCompile(`\[.*?\]\(([^)#?]+)\)`)
var bareRE = regexp.MustCompile("`([^`]+\\.(md|yaml|yml|json|jsonl|txt|sqlite))`")

func utcNow() string              { return time.Now().UTC().Format("2006-01-02T15:04:05+00:00") }
func round(value float64) float64 { return math.RoundToEven(value*100) / 100 }
func safeRead(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.Size() > 2000000 {
		return ""
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ToValidUTF8(string(bytes), "")
}
func allFiles(root string) ([]string, error) {
	files := []string{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return files, nil
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := os.Stat(path)
			if err == nil && info.Mode().IsRegular() {
				files = append(files, path)
			}
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
func extensionCounts(files []string) map[string]int {
	counts := map[string]int{}
	for _, path := range files {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "[none]"
		}
		counts[ext]++
	}
	return counts
}
func sourceMetrics(source string) (map[string]any, error) {
	if source == "" {
		return map[string]any{}, nil
	}
	files, err := allFiles(source)
	if err != nil {
		return nil, err
	}
	_, statErr := os.Stat(source)
	return map[string]any{"path": source, "exists": statErr == nil, "file_count": len(files), "extension_counts": extensionCounts(files)}, nil
}
func rel(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(value)
}
func depth(root, path string) int { return len(strings.Split(rel(root, path), "/")) }
func outbound(text, file, root string) map[string]bool {
	refs := map[string]bool{}
	matches := linkRE.FindAllStringSubmatch(text, -1)
	matches = append(matches, bareRE.FindAllStringSubmatch(text, -1)...)
	for _, match := range matches {
		raw := strings.TrimSpace(match[1])
		if strings.HasPrefix(raw, "http") {
			continue
		}
		path := raw
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(file), path)
		}
		path = filepath.Clean(path)
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		relative := rel(root, path)
		if relative != ".." && !strings.HasPrefix(relative, "../") && relative != "" {
			refs[relative] = true
		}
	}
	return refs
}
func fanOut(root string, files []string) map[string]any {
	children := map[string]int{}
	for _, path := range files {
		parts := strings.Split(rel(root, path), "/")
		for i := 0; i < len(parts)-1; i++ {
			parent := "."
			if i > 0 {
				parent = strings.Join(parts[:i+1], "/")
			}
			children[parent]++
		}
		if len(parts) == 1 {
			if _, exists := children["."]; !exists {
				children["."] = 0
			}
		}
	}
	if len(children) == 0 {
		return map[string]any{"mean": 0, "max": 0, "variance": 0, "skewed_nodes": []string{}}
	}
	sum, maxCount := 0, 0
	for _, count := range children {
		sum += count
		if count > maxCount {
			maxCount = count
		}
	}
	mean := float64(sum) / float64(len(children))
	variance := 0.0
	if len(children) > 1 {
		for _, count := range children {
			delta := float64(count) - mean
			variance += delta * delta
		}
		variance /= float64(len(children) - 1)
	}
	skewed := []string{}
	threshold := math.Max(round(mean)*2, 10)
	for node, count := range children {
		if float64(count) > threshold {
			skewed = append(skewed, node)
		}
	}
	sort.Slice(skewed, func(i, j int) bool {
		if children[skewed[i]] == children[skewed[j]] {
			return skewed[i] < skewed[j]
		}
		return children[skewed[i]] > children[skewed[j]]
	})
	if len(skewed) > 10 {
		skewed = skewed[:10]
	}
	return map[string]any{"mean": round(mean), "max": maxCount, "variance": round(variance), "skewed_nodes": skewed}
}
func indexMetrics(index string) (map[string]any, error) {
	files, err := allFiles(index)
	if err != nil {
		return nil, err
	}
	breadth := map[int]map[string]bool{}
	maxDepth := 0
	for _, path := range files {
		parts := strings.Split(rel(index, path), "/")
		if len(parts) > maxDepth {
			maxDepth = len(parts)
		}
		for level := 1; level <= len(parts); level++ {
			if breadth[level] == nil {
				breadth[level] = map[string]bool{}
			}
			breadth[level][strings.Join(parts[:level], "/")] = true
		}
	}
	routing, leaves := []string{}, []string{}
	citations, debt, benchmarks := 0, 0, 0
	var newest time.Time
	refs := map[string]bool{}
	for _, path := range files {
		relative := rel(index, path)
		lower := strings.ToLower(relative)
		isRouting := routingNames[strings.ToLower(filepath.Base(path))] || strings.HasPrefix(lower, ".semantic-index/")
		isText := textExtensions[strings.ToLower(filepath.Ext(path))]
		if isRouting {
			routing = append(routing, relative)
		} else if isText {
			leaves = append(leaves, relative)
		}
		if strings.Contains(lower, "benchmark") || strings.Contains(lower, "eval") {
			benchmarks++
			info, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
		if isText {
			if text := safeRead(path); text != "" {
				citations += len(citationRE.FindAllString(text, -1))
				debt += len(debtRE.FindAllString(text, -1))
				if isRouting {
					for reference := range outbound(text, path, index) {
						refs[reference] = true
					}
				}
			}
		}
	}
	orphans := []string{}
	leafDepth := 0
	for _, leaf := range leaves {
		if !refs[leaf] && !refs[filepath.Base(leaf)] {
			orphans = append(orphans, leaf)
		}
		leafDepth += depth(index, filepath.Join(index, filepath.FromSlash(leaf)))
	}
	meanDepth, balance := 0.0, 0.0
	if len(leaves) > 0 {
		meanDepth = round(float64(leafDepth) / float64(len(leaves)))
		if meanDepth != 0 {
			balance = round(float64(maxDepth) / meanDepth)
		}
	}
	breadthReport := map[string]int{}
	for level, nodes := range breadth {
		breadthReport[strconv.Itoa(level)] = len(nodes)
	}
	var state any
	statePath := filepath.Join(index, ".semantic-index/state.json")
	if bytes, err := os.ReadFile(statePath); err == nil {
		if json.Unmarshal(bytes, &state) != nil {
			state = map[string]string{"error": "state.json is not valid JSON"}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	var lastBenchmark any
	if !newest.IsZero() {
		lastBenchmark = newest.UTC().Format("2006-01-02T15:04:05+00:00")
	}
	citationDensity, routingDensity := 0.0, 0.0
	if len(leaves) > 0 {
		citationDensity = round(float64(citations) / float64(len(leaves)))
	}
	if len(routing) > 0 {
		routingDensity = round(float64(citations) / float64(len(routing)))
	}
	_, statErr := os.Stat(index)
	return map[string]any{"path": index, "exists": statErr == nil, "file_count": len(files), "extension_counts": extensionCounts(files), "max_file_depth": maxDepth, "mean_leaf_depth": meanDepth, "balance_ratio": balance, "breadth_by_level": breadthReport, "fan_out": fanOut(index, files), "routing_file_count": len(routing), "routing_files": routing[:min(40, len(routing))], "likely_leaf_count": len(leaves), "sample_likely_leaves": leaves[:min(40, len(leaves))], "orphan_leaf_count": len(orphans), "orphan_leaves": orphans[:min(20, len(orphans))], "citation_count": citations, "citation_density_per_leaf": citationDensity, "routing_density": routingDensity, "debt_marker_count": debt, "benchmark_file_count": benchmarks, "last_benchmark_at": lastBenchmark, "state": state}, nil
}
func writeState(index string, report map[string]any, mode string) (string, error) {
	dir := filepath.Join(index, ".semantic-index")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "state.json")
	state := map[string]any{"last_housekeeping_at": utcNow(), "last_mode": mode, "source": report["source"], "index_metrics": report["index"]}
	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(bytes, '\n'), 0644)
}
func absolute(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	return path, nil
}
func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("semantic_index_metrics", flag.ContinueOnError)
	indexArg := fs.String("index", "", "absolute semantic index root (required)")
	sourceArg := fs.String("source", "", "optional absolute corpus root")
	mode := fs.String("mode", "audit", "maintenance mode")
	write := fs.Bool("write-state", false, "write housekeeping state")
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
	source := ""
	if *sourceArg != "" {
		source, err = absolute(*sourceArg)
		if err != nil {
			return err
		}
	}
	sourceReport, err := sourceMetrics(source)
	if err != nil {
		return err
	}
	indexReport, err := indexMetrics(index)
	if err != nil {
		return err
	}
	report := map[string]any{"generated_at": utcNow(), "mode": *mode, "source": sourceReport, "index": indexReport}
	if *write {
		path, err := writeState(index, report, *mode)
		if err != nil {
			return err
		}
		report["state_written"] = path
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
