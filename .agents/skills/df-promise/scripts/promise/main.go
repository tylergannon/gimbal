// Promise manages durable repository Promise Loop state and README badges.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type object = map[string]any

const schema = 2
const badgeStart = "<!-- df-promise-badges:start -->"
const badgeEnd = "<!-- df-promise-badges:end -->"

var idRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var badgeRE = regexp.MustCompile(`(?s)(?:\r?\n)?<!-- df-promise-badges:start -->.*?<!-- df-promise-badges:end -->(?:\r?\n)?`)
var badgeItemRE = regexp.MustCompile(`\[!\[[^\]]+\]\([^)]+\)\]\(([^)]+)\)`)
var now = func() time.Time { return time.Now().UTC() }
var costBands = []string{"lt-10", "10-100", "100-1k", "1k-10k", "10k-100k", "gte-100k", "unknown"}
var gateKinds = []string{"coverage", "inspection", "test", "visual", "tool", "verification"}

func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func obj(v any) object { m, _ := v.(map[string]any); return m }
func list(v any) []any { x, _ := v.([]any); return x }
func strs(v any) []string {
	out := []string{}
	for _, x := range list(v) {
		out = append(out, text(x))
	}
	return out
}
func values(x []string) []any {
	r := make([]any, len(x))
	for i, v := range x {
		r[i] = v
	}
	return r
}
func number(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		x, _ := n.Float64()
		return x
	}
	return 0
}
func numeric(v any) bool {
	switch v.(type) {
	case float64, int, int64, json.Number:
		return true
	}
	return false
}
func rounded(v float64, places int) float64 {
	s := math.Pow10(places)
	return math.RoundToEven(v*s) / s
}
func stamp(t time.Time) string             { return t.UTC().Format("2006-01-02T15:04:05Z") }
func parseTime(v any) (time.Time, error)   { return time.Parse(time.RFC3339Nano, text(v)) }
func nonempty(v any) bool                  { return strings.TrimSpace(text(v)) != "" }
func bad(format string, args ...any) error { return fmt.Errorf(format, args...) }
func contains(x []string, v string) bool   { return slices.Contains(x, v) }
func boolValue(v any) bool                 { b, _ := v.(bool); return b }
func safeRelative(s string) bool {
	if s == "" || filepath.IsAbs(s) {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == ".." {
			return false
		}
	}
	return filepath.Clean(s) != "."
}
func base(root, id string) (string, error) {
	if !idRE.MatchString(id) {
		return "", bad("promise id must use lowercase ASCII kebab-case")
	}
	return filepath.Join(root, ".promises", id), nil
}
func mustBase(root, id string) string { p, _ := base(root, id); return p }
func exists(p string) bool            { _, e := os.Stat(p); return e == nil }
func isFile(p string) bool            { s, e := os.Stat(p); return e == nil && s.Mode().IsRegular() }
func readJSON(p string) (object, error) {
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, bad("missing required state file: %s: %w", p, e)
	}
	var m object
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, bad("invalid JSON in %s: %w", p, e)
	}
	if m == nil {
		return nil, bad("expected a JSON object in %s", p)
	}
	return m, nil
}
func writeJSON(p string, m object) error {
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(p, append(b, '\n'))
}
func atomicWrite(p string, b []byte) error {
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	tmp := filepath.Join(filepath.Dir(p), fmt.Sprintf(".%s.%d.tmp", filepath.Base(p), os.Getpid()))
	if e := os.WriteFile(tmp, b, 0644); e != nil {
		return e
	}
	if e := os.Rename(tmp, p); e != nil {
		_ = os.Remove(tmp)
		return e
	}
	return nil
}
func appendJSON(p string, m object) error {
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	_, e = f.Write(append(b, '\n'))
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func git(root string, check bool, args ...string) ([]byte, error) {
	c := exec.Command("git", args...)
	c.Dir = root
	var stderr bytes.Buffer
	c.Stderr = &stderr
	b, e := c.Output()
	if e != nil && check {
		return nil, bad("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return b, nil
}
func gitText(root string, check bool, args ...string) (string, error) {
	b, e := git(root, check, args...)
	return strings.TrimSpace(string(b)), e
}
func repoRoot(project string) (string, error) {
	b, e := gitText(project, true, "rev-parse", "--show-toplevel")
	if e != nil {
		return "", bad("df-promise must run inside a Git repository: %w", e)
	}
	return filepath.EvalSymlinks(b)
}

// Resolve existing symlinks, including an existing ancestor of a new file.
func confined(root, value string) (string, error) {
	p := value
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	p = filepath.Clean(p)
	tail := []string{}
	q := p
	for {
		r, e := filepath.EvalSymlinks(q)
		if e == nil {
			p = r
			for i := len(tail) - 1; i >= 0; i-- {
				p = filepath.Join(p, tail[i])
			}
			break
		}
		if !errors.Is(e, os.ErrNotExist) {
			return "", e
		}
		parent := filepath.Dir(q)
		if parent == q {
			return "", e
		}
		tail = append(tail, filepath.Base(q))
		q = parent
	}
	rel, e := filepath.Rel(root, p)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", bad("path must remain inside the repository: %s", value)
	}
	return p, nil
}
func lock(root string) (func(), error) {
	d := filepath.Join(root, ".promises", ".work")
	if e := os.MkdirAll(d, 0755); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(d, "state.lock"), os.O_CREATE|os.O_RDWR, 0644)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		_ = f.Close()
		return nil, bad("another Promise state mutation is active; retry this bounded command")
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func loadRegistry(root string) (object, error) {
	p := filepath.Join(root, ".promises", "registry.json")
	if !exists(p) {
		return object{"schema_version": schema, "promises": []any{}}, nil
	}
	m, e := readJSON(p)
	if e != nil {
		return nil, e
	}
	if number(m["schema_version"]) != 1 && number(m["schema_version"]) != schema {
		return nil, bad("unsupported Promise registry schema; upgrade df-promise before continuing")
	}
	entries, ok := m["promises"].([]any)
	if !ok {
		return nil, bad("registry promises field must be a list")
	}
	seen := map[string]bool{}
	for i, v := range entries {
		x := obj(v)
		id := text(x["id"])
		if !idRE.MatchString(id) {
			return nil, bad("registry promise entry %d has an invalid id", i)
		}
		if seen[id] {
			return nil, bad("registry contains duplicate promise id: %s", id)
		}
		seen[id] = true
		if !nonempty(x["name"]) || text(x["contract"]) != ".promises/"+id+"/contract.json" || !safeRelative(text(x["readme"])) {
			return nil, bad("invalid registry entry for %s", id)
		}
	}
	return m, nil
}
func registered(root, id string) error {
	r, e := loadRegistry(root)
	if e != nil {
		return e
	}
	for _, v := range list(r["promises"]) {
		if text(obj(v)["id"]) == id {
			return nil
		}
	}
	return bad("promise is not registered: %s; restore it before running", id)
}
func loadContract(root, id string, requireTools bool) (object, error) {
	p, e := base(root, id)
	if e != nil {
		return nil, e
	}
	m, e := readJSON(filepath.Join(p, "contract.json"))
	if e != nil {
		return nil, e
	}
	return m, validateContract(root, id, m, requireTools)
}
func loadState(root, id string) (object, error) {
	m, e := readJSON(filepath.Join(mustBase(root, id), "state.json"))
	if e != nil {
		return nil, e
	}
	if number(m["schema_version"]) != schema || text(m["id"]) != id {
		return nil, bad("invalid state identity or schema for %s", id)
	}
	if _, ok := m["status"].(string); !ok {
		return nil, bad("state for %s needs a status", id)
	}
	for _, k := range []string{"active_run", "last_run"} {
		if m[k] != nil {
			if _, ok := m[k].(string); !ok {
				return nil, bad("state for %s has an invalid %s", id, k)
			}
		}
	}
	if m["last_fulfillment"] != nil && obj(m["last_fulfillment"]) == nil {
		return nil, bad("invalid last_fulfillment for %s", id)
	}
	l := obj(m["lifetime_metrics"])
	if l == nil {
		return nil, bad("state for %s needs lifetime_metrics", id)
	}
	for _, k := range []string{"runs", "steps", "tool_calls", "tokens_known", "cost_usd_known", "wall_seconds"} {
		if !numeric(l[k]) || number(l[k]) < 0 {
			return nil, bad("invalid lifetime metric %s", k)
		}
	}
	for _, k := range []string{"tokens_complete", "cost_complete"} {
		if _, ok := l[k].(bool); !ok {
			return nil, bad("invalid lifetime metric %s", k)
		}
	}
	return m, nil
}
func validateContract(root, id string, c object, requireTools bool) error {
	if number(c["schema_version"]) != schema {
		if number(c["schema_version"]) == 1 {
			return bad("contract schema 1 for %s requires: promise migrate %s ...", id, id)
		}
		return bad("unsupported contract schema for %s", id)
	}
	if text(c["id"]) != id {
		return bad("contract id does not match directory for %s", id)
	}
	if !nonempty(c["name"]) || !nonempty(c["statement"]) {
		return bad("contract for %s needs a name and statement", id)
	}
	if _, ok := c["human_signoff_required"].(bool); !ok {
		return bad("contract needs boolean human_signoff_required")
	}
	gates, ok := c["acceptance_gates"].([]any)
	if !ok || len(gates) == 0 {
		return bad("contract needs at least one acceptance gate")
	}
	ids := map[string]bool{}
	for _, v := range gates {
		g := obj(v)
		k := text(g["id"])
		if !idRE.MatchString(k) || !nonempty(g["description"]) || ids[k] {
			return bad("invalid or duplicate acceptance gate: %s", k)
		}
		ids[k] = true
	}
	if boolValue(c["human_signoff_required"]) && !ids["human-signoff"] {
		return bad("contract requires a human-signoff acceptance gate")
	}
	scope := obj(c["scope"])
	if scope == nil {
		return bad("contract needs a scope object")
	}
	for _, k := range []string{"include", "exclude"} {
		v := scope[k]
		if k == "exclude" && v == nil {
			continue
		}
		xs, ok := v.([]any)
		if !ok || (k == "include" && len(xs) == 0) {
			return bad("contract needs valid scope.%s", k)
		}
		for _, x := range xs {
			if !nonempty(x) || filepath.IsAbs(text(x)) || contains(strings.Split(text(x), "/"), "..") {
				return bad("unsafe scope.%s pattern: %v", k, x)
			}
		}
	}
	op := obj(c["operation"])
	for _, k := range []string{"cadence", "runner", "authority"} {
		if !nonempty(op[k]) {
			return bad("contract needs operation %s", k)
		}
	}
	ps, ok := op["plan_steps"].([]any)
	if !ok || len(ps) == 0 {
		return bad("contract needs operating-plan steps")
	}
	for _, x := range ps {
		if !nonempty(x) {
			return bad("invalid operating-plan step")
		}
	}
	budgets := obj(op["budgets"])
	for _, k := range []string{"max_steps", "max_tool_calls", "max_tokens", "max_wall_seconds"} {
		v, ok := budgets[k]
		if !ok {
			return bad("contract needs all operation budget fields")
		}
		if v == nil {
			continue
		}
		if !numeric(v) || number(v) < 0 {
			return bad("invalid operation budget %s", k)
		}
		if k != "max_wall_seconds" && math.Trunc(number(v)) != number(v) {
			return bad("budget %s must be an integer", k)
		}
	}
	inv := obj(c["invalidation"])
	if inv["scope_change"] != true {
		return bad("contract must invalidate on scope changes")
	}
	age := inv["max_age_hours"]
	if age != nil && (!numeric(age) || number(age) < 0) {
		return bad("invalid invalidation.max_age_hours")
	}
	mode := text(obj(c["evidence"])["mode"])
	if !contains([]string{"internal", "external", "mixed"}, mode) {
		return bad("contract needs evidence.mode internal, external, or mixed")
	}
	if mode != "internal" && age == nil {
		return bad("external evidence needs max_age_hours")
	}
	if !nonempty(obj(c["verifier"])["identity"]) {
		return bad("contract needs verifier.identity")
	}
	econ := obj(c["economics"])
	band := text(econ["full_run_cost_band"])
	if !contains(costBands, band) {
		return bad("invalid economics.full_run_cost_band")
	}
	if !nonempty(econ["estimate_basis"]) {
		return bad("contract needs economics.estimate_basis")
	}
	if _, ok := econ["approval_required"].(bool); !ok {
		return bad("contract needs boolean economics.approval_required")
	}
	cap := econ["max_run_cost_usd"]
	if cap != nil && (!numeric(cap) || number(cap) < 0) {
		return bad("invalid economics.max_run_cost_usd")
	}
	if contains(costBands[2:], band) && !boolValue(econ["approval_required"]) {
		return bad("cost band %s requires per-run approval", band)
	}
	if boolValue(econ["approval_required"]) && (cap == nil || number(cap) <= 0) {
		return bad("approval requires a positive max_run_cost_usd")
	}
	bounds := map[string][2]float64{"lt-10": {0, 10}, "10-100": {10, 100}, "100-1k": {100, 1000}, "1k-10k": {1000, 10000}, "10k-100k": {10000, 100000}, "gte-100k": {100000, math.Inf(1)}}
	if b, ok := bounds[band]; ok && cap != nil {
		n := number(cap)
		if n < b[0] || n > b[1] || (band == "lt-10" && n >= b[1]) {
			return bad("max_run_cost_usd conflicts with cost band %s", band)
		}
	}
	stage := econ["staging_plan"]
	if stage != nil && !nonempty(stage) {
		return bad("invalid economics.staging_plan")
	}
	if contains([]string{"10k-100k", "gte-100k"}, band) && stage == nil {
		return bad("cost band %s requires a staging plan", band)
	}
	tools, ok := c["tools"].([]any)
	if !ok {
		return bad("contract needs a tools list")
	}
	for _, x := range tools {
		t := obj(x)
		p := text(t["path"])
		if !nonempty(t["purpose"]) || !safeRelative(p) || strings.Split(p, "/")[0] != "tools" {
			return bad("tool escapes promise-local tools/: %s", p)
		}
		if requireTools && !isFile(filepath.Join(mustBase(root, id), p)) {
			return bad("declared promise tool is not a file: %s", p)
		}
	}
	b := obj(c["badge"])
	if !nonempty(b["label"]) || !nonempty(b["readme"]) {
		return bad("contract needs badge label and README path")
	}
	p, e := confined(root, text(b["readme"]))
	if e != nil {
		return e
	}
	if st, e := os.Stat(p); e == nil && st.IsDir() {
		return bad("badge README path is a directory: %s", p)
	}
	return nil
}
func pair(v, label string) (string, string, error) {
	k, d, ok := strings.Cut(v, "=")
	k = strings.TrimSpace(k)
	d = strings.TrimSpace(d)
	if !ok || !idRE.MatchString(k) || d == "" {
		return "", "", bad("%s must use id=description syntax: %s", label, v)
	}
	return k, d, nil
}
func parseTool(v string) (object, error) {
	p, d, ok := strings.Cut(v, "=")
	p = strings.TrimSpace(p)
	d = strings.TrimSpace(d)
	if !ok || p == "" || d == "" || !safeRelative(p) {
		return nil, bad("invalid tool declaration: %s", v)
	}
	if strings.Split(p, "/")[0] != "tools" {
		p = "tools/" + p
	}
	return object{"path": filepath.ToSlash(filepath.Clean(p)), "purpose": d}, nil
}
func evidenceValid(root, id, mode string, v any) bool {
	r := strings.TrimSpace(text(v))
	if r == "" {
		return false
	}
	if mode == "internal" || mode == "mixed" {
		p, e := confined(root, r)
		if e == nil {
			rel, e := filepath.Rel(mustBase(root, id), p)
			if e == nil && rel != ".." && !strings.HasPrefix(rel, "../") && isFile(p) {
				return true
			}
		}
		if mode == "internal" {
			return false
		}
	}
	if regexp.MustCompile(`^sha256:[0-9a-fA-F]{64}$`).MatchString(r) {
		return true
	}
	u, e := url.Parse(r)
	return e == nil && contains([]string{"https", "s3", "gs", "artifact", "provider", "request"}, u.Scheme) && (u.Host != "" || u.Path != "" || u.Opaque != "") && len(r) >= 12
}
func patternRE(p string) *regexp.Regexp {
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '*':
			if i+1 < len(p) && p[i+1] == '*' {
				i += 2
				for i < len(p) && p[i] == '*' {
					i++
				}
				if i < len(p) && p[i] == '/' {
					b.WriteString(`(?:.*/)?`)
				} else {
					b.WriteString(`.*`)
					i--
				}
			} else {
				b.WriteString(`[^/]*`)
			}
		case '?':
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexp.QuoteMeta(string(p[i])))
		}
	}
	b.WriteByte('$')
	return regexp.MustCompile(b.String())
}
func inScope(p string, c object) bool {
	if p == ".promises" || strings.HasPrefix(p, ".promises/") || p == ".git" || strings.HasPrefix(p, ".git/") {
		return false
	}
	s := obj(c["scope"])
	ok := false
	for _, pat := range strs(s["include"]) {
		if patternRE(pat).MatchString(p) {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	for _, pat := range strs(s["exclude"]) {
		if patternRE(pat).MatchString(p) {
			return false
		}
	}
	return true
}
func managedReadmes(root string, c object) (map[string]bool, error) {
	out := map[string]bool{}
	add := func(v string) error {
		p, e := confined(root, v)
		if e != nil {
			return e
		}
		r, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		out[filepath.ToSlash(r)] = true
		return nil
	}
	if e := add(text(obj(c["badge"])["readme"])); e != nil {
		return nil, e
	}
	r, e := loadRegistry(root)
	if e != nil {
		return nil, e
	}
	for _, x := range list(r["promises"]) {
		if e := add(text(obj(x)["readme"])); e != nil {
			return nil, e
		}
	}
	return out, nil
}
func stripBadges(s string) string { return badgeRE.ReplaceAllString(s, "") }
func insertBadges(s string, badges []string) string {
	s = stripBadges(s)
	if len(badges) == 0 {
		return s
	}
	slices.Sort(badges)
	block := badgeStart + "\n" + strings.Join(badges, " ") + "\n" + badgeEnd
	if strings.HasPrefix(s, "# ") {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return s + "\n" + block
		}
		return s[:i+1] + "\n" + block + "\n" + s[i+1:]
	}
	if s != "" {
		return block + "\n" + s
	}
	return block
}
func badgeItems(s string) [][]string {
	return badgeItemRE.FindAllStringSubmatch(badgeRE.FindString(s), -1)
}
func removeBadge(root, id, readme string) ([]string, error) {
	out := []string{}
	p, e := confined(root, readme)
	if e != nil {
		return nil, e
	}
	if !exists(p) {
		return out, nil
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, e
	}
	items := badgeItems(string(b))
	if len(items) == 0 {
		return out, nil
	}
	keep := []string{}
	for _, it := range items {
		if !strings.HasSuffix(it[1], ".promises/"+id+"/PROMISE.md") {
			keep = append(keep, it[0])
		}
	}
	s := insertBadges(string(b), keep)
	if s != string(b) {
		if e = atomicWrite(p, []byte(s)); e != nil {
			return nil, e
		}
		r, _ := filepath.Rel(root, p)
		out = append(out, filepath.ToSlash(r))
	}
	return out, nil
}
func repoFiles(root string) ([]string, error) {
	b, e := git(root, true, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, v := range bytes.Split(b, []byte{0}) {
		if len(v) > 0 {
			out = append(out, string(v))
		}
	}
	slices.Sort(out)
	return out, nil
}
func fileBytes(p string) ([]byte, error) {
	s, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if s.Mode()&os.ModeSymlink != 0 {
		l, e := os.Readlink(p)
		return []byte(l), e
	}
	return os.ReadFile(p)
}
func subjectFingerprint(root string, c object) (string, int, error) {
	h := sha256.New()
	files, e := repoFiles(root)
	if e != nil {
		return "", 0, e
	}
	readmes, e := managedReadmes(root, c)
	if e != nil {
		return "", 0, e
	}
	count := 0
	for _, rel := range files {
		if !inScope(rel, c) {
			continue
		}
		p := filepath.Join(root, rel)
		s, e := os.Lstat(p)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return "", 0, e
		}
		if s.Mode()&os.ModeSymlink == 0 && !s.Mode().IsRegular() {
			continue
		}
		b, e := fileBytes(p)
		if e != nil {
			return "", 0, e
		}
		if readmes[rel] {
			b = []byte(stripBadges(string(b)))
		}
		d := sha256.Sum256(b)
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(d[:])
		count++
	}
	return hex.EncodeToString(h.Sum(nil)), count, nil
}
func mechanismFingerprint(root, id string) (string, error) {
	p := mustBase(root, id)
	c, e := readJSON(filepath.Join(p, "contract.json"))
	if e != nil {
		return "", e
	}
	paths := []string{"contract.json", "PROMISE.md"}
	for _, t := range list(c["tools"]) {
		paths = append(paths, text(obj(t)["path"]))
	}
	h := sha256.New()
	for _, r := range paths {
		b, e := fileBytes(filepath.Join(p, r))
		if e != nil {
			return "", bad("missing promise mechanism file: %s: %w", r, e)
		}
		d := sha256.Sum256(b)
		_, _ = h.Write([]byte(r))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(d[:])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func dirtyPaths(root string, c object) ([]string, error) {
	changed := map[string]bool{}
	for _, args := range [][]string{{"diff", "--name-only", "HEAD"}, {"ls-files", "--others", "--exclude-standard"}} {
		s, e := gitText(root, true, args...)
		if e != nil {
			return nil, e
		}
		for _, r := range strings.Split(s, "\n") {
			if r != "" {
				changed[r] = true
			}
		}
	}
	readmes, e := managedReadmes(root, c)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for r := range changed {
		if !inScope(r, c) {
			continue
		}
		if readmes[r] && exists(filepath.Join(root, r)) {
			head, _ := git(root, false, "show", "HEAD:"+r)
			b, e := os.ReadFile(filepath.Join(root, r))
			if e != nil {
				return nil, e
			}
			if stripBadges(string(head)) == stripBadges(string(b)) {
				continue
			}
		}
		out = append(out, r)
	}
	slices.Sort(out)
	return out, nil
}
func validity(root, id string, c, s object) (bool, string, error) {
	f := obj(s["last_fulfillment"])
	if f == nil {
		return false, "never fulfilled", nil
	}
	for _, k := range []string{"fulfilled_at", "validated_commit", "subject_fingerprint", "mechanism_fingerprint"} {
		if !nonempty(f[k]) {
			return false, "fulfillment state is incomplete", nil
		}
	}
	hash, n, e := subjectFingerprint(root, c)
	if e != nil {
		return false, "", e
	}
	if n == 0 {
		return false, "promise scope resolves to zero files", nil
	}
	if hash != text(f["subject_fingerprint"]) {
		return false, "promise scope changed", nil
	}
	mech, e := mechanismFingerprint(root, id)
	if e != nil || mech != text(f["mechanism_fingerprint"]) {
		return false, "promise contract or tools changed", nil
	}
	age := obj(c["invalidation"])["max_age_hours"]
	if age != nil {
		t, e := parseTime(f["fulfilled_at"])
		if e != nil {
			return false, "fulfillment state is incomplete", nil
		}
		if now().Sub(t).Seconds() > number(age)*3600 {
			return false, "promise fulfillment expired", nil
		}
	}
	return true, "fulfilled", nil
}
func badgeMarkdown(root, id string, c, s object) (string, error) {
	f := obj(s["last_fulfillment"])
	date := text(f["fulfilled_at"])
	if len(date) > 10 {
		date = date[:10]
	}
	commit := text(f["validated_commit"])
	if len(commit) > 7 {
		commit = commit[:7]
	}
	msg := "fulfilled"
	if len(list(f["dirty_subject_at_start"])) > 0 {
		msg += " dirty"
	}
	msg += " " + date + " @ " + commit
	label := text(obj(c["badge"])["label"])
	q := url.Values{"label": {label}, "message": {msg}, "color": {"brightgreen"}}
	readme, e := confined(root, text(obj(c["badge"])["readme"]))
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(filepath.Dir(readme), filepath.Join(mustBase(root, id), "PROMISE.md"))
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("[![%s](https://img.shields.io/static/v1?%s)](%s)", label, q.Encode(), filepath.ToSlash(rel)), nil
}
func renderBadges(root string, extra []string) (object, error) {
	r, e := loadRegistry(root)
	if e != nil {
		return nil, e
	}
	by := map[string][]string{}
	known := map[string]bool{}
	valid := []string{}
	errs := []any{}
	search := exec.Command("git", "grep", "-lz", "--untracked", "-F", badgeStart, "--")
	search.Dir = root
	var stderr bytes.Buffer
	search.Stderr = &stderr
	b, e := search.Output()
	var exit *exec.ExitError
	if e != nil && (!errors.As(e, &exit) || exit.ExitCode() != 1) {
		return nil, bad("git grep failed while locating managed badges: %s: %w", strings.TrimSpace(stderr.String()), e)
	}
	for _, raw := range bytes.Split(b, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		p, e := confined(root, string(raw))
		if e != nil {
			return nil, e
		}
		s, e := os.Lstat(p)
		if e != nil || s.Mode()&os.ModeSymlink != 0 || !s.Mode().IsRegular() {
			continue
		}
		content, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		lines := strings.Split(string(content), "\n")
		start, end := false, false
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == badgeStart {
				start = true
			}
			if l == badgeEnd {
				end = true
			}
		}
		if start && end {
			known[p] = true
		}
	}
	for _, v := range extra {
		p, e := confined(root, v)
		if e != nil {
			return nil, e
		}
		known[p] = true
	}
	for _, v := range list(r["promises"]) {
		entry := obj(v)
		id := text(entry["id"])
		readme, e := confined(root, text(entry["readme"]))
		if e != nil {
			return nil, e
		}
		known[readme] = true
		c, e := loadContract(root, id, false)
		var s object
		if e == nil {
			s, e = loadState(root, id)
		}
		if e != nil {
			errs = append(errs, object{"id": id, "error": e.Error()})
			if strings.Contains(e.Error(), "contract schema 1") {
				content, _ := os.ReadFile(readme)
				for _, it := range badgeItems(string(content)) {
					if strings.HasSuffix(it[1], ".promises/"+id+"/PROMISE.md") {
						by[readme] = append(by[readme], it[0])
						break
					}
				}
			}
			continue
		}
		readme, e = confined(root, text(obj(c["badge"])["readme"]))
		if e != nil {
			return nil, e
		}
		known[readme] = true
		ok, _, e := validity(root, id, c, s)
		if e != nil {
			return nil, e
		}
		if !ok {
			continue
		}
		badge, e := badgeMarkdown(root, id, c, s)
		if e != nil {
			return nil, e
		}
		by[readme] = append(by[readme], badge)
		valid = append(valid, id)
	}
	paths := []string{}
	for p := range known {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	updated := []string{}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
		s := insertBadges(string(b), by[p])
		if s != string(b) {
			if e = atomicWrite(p, []byte(s)); e != nil {
				return nil, e
			}
			rel, _ := filepath.Rel(root, p)
			updated = append(updated, filepath.ToSlash(rel))
		}
	}
	slices.Sort(valid)
	return object{"updated": values(updated), "valid": values(valid), "errors": errs}, nil
}
func document(c object) string {
	gates, includes, excludes, steps, tools := []string{}, []string{}, []string{}, []string{}, []string{}
	for _, v := range list(c["acceptance_gates"]) {
		g := obj(v)
		gates = append(gates, "- `"+text(g["id"])+"` — "+text(g["description"]))
	}
	s := obj(c["scope"])
	for _, v := range strs(s["include"]) {
		includes = append(includes, "- `"+v+"`")
	}
	for _, v := range strs(s["exclude"]) {
		excludes = append(excludes, "- `"+v+"`")
	}
	if len(excludes) == 0 {
		excludes = append(excludes, "- None beyond Promise state.")
	}
	op := obj(c["operation"])
	for i, v := range strs(op["plan_steps"]) {
		steps = append(steps, fmt.Sprintf("%d. %s", i+1, v))
	}
	for _, v := range list(c["tools"]) {
		t := obj(v)
		tools = append(tools, "- `"+text(t["path"])+"` — "+text(t["purpose"]))
	}
	if len(tools) == 0 {
		tools = append(tools, "- No promise-local executable tools are required initially.")
	}
	human := "not required"
	if boolValue(c["human_signoff_required"]) {
		human = "required"
	}
	econ := obj(c["economics"])
	approval := "not required"
	if boolValue(econ["approval_required"]) {
		approval = "required for every run"
	}
	cap := "not set"
	if econ["max_run_cost_usd"] != nil {
		cap = fmt.Sprintf("$%.2f", number(econ["max_run_cost_usd"]))
	}
	staging := text(econ["staging_plan"])
	if staging == "" {
		staging = "Not required for this cost band."
	}
	return fmt.Sprintf(`# Promise: %s

%s

## Scope

Included:

%s

Excluded:

%s

## Fulfillment gates

%s

Human sign-off is %s. A fulfilled run is pinned to its reviewed commit,
subject fingerprint, contract and tool fingerprint, verifier, evidence, and
resource metrics.

## Operating plan

%s

Cadence: **%s**

Runner: **%s**

Default authority: **%s**

## Economics and evidence

Estimated full-run cost: **%s** (USD band)

Estimate basis: %s

Per-run approval: **%s**. Maximum run cost: **%s**.

Staging plan: %s

Evidence mode: **%s**. Every fulfillment gate needs a
passing event linked to durable evidence.

Verifier: **%s**

## Promise-local tools

%s

## State

Current machine-readable state is in `+"`state.json`"+`. Compact run summaries and
append-only events live under `+"`runs/`"+`; latest per-subject coverage is in
`+"`coverage.json`"+`. Large or sensitive raw artifacts belong in ignored `+"`.work/`"+`,
with hashes or durable external references recorded as evidence.
`, text(c["name"]), text(c["statement"]), strings.Join(includes, "\n"), strings.Join(excludes, "\n"), strings.Join(gates, "\n"), human, strings.Join(steps, "\n"), text(op["cadence"]), text(op["runner"]), text(op["authority"]), text(econ["full_run_cost_band"]), text(econ["estimate_basis"]), approval, cap, staging, text(obj(c["evidence"])["mode"]), text(obj(c["verifier"])["identity"]), strings.Join(tools, "\n"))
}

type arguments struct {
	command    string
	positional []string
	flags      map[string][]string
	help       bool
}

func (a arguments) s(k string) string {
	xs := a.flags[k]
	if len(xs) > 0 {
		return xs[len(xs)-1]
	}
	return ""
}
func (a arguments) has(k string) bool { return len(a.flags[k]) > 0 }
func (a arguments) n(k string) any {
	if !a.has(k) {
		return nil
	}
	v, _ := strconv.ParseFloat(a.s(k), 64)
	return v
}
func (a arguments) b(k string) bool { return a.has(k) }
func (a arguments) id() string {
	if len(a.positional) > 0 {
		return a.positional[0]
	}
	return ""
}
func (a arguments) runID() string {
	if len(a.positional) > 1 {
		return a.positional[1]
	}
	return ""
}

var options = map[string]string{
	"init":     "name statement include exclude gate plan-step tool cadence runner authority evidence-mode verifier cost-band cost-basis approval-required max-run-cost-usd staging-plan max-age-hours max-steps max-tool-calls max-tokens max-wall-seconds human-signoff readme badge-label",
	"validate": "allow-incomplete-tools", "start": "allow-dirty approval-ref", "record": "kind result subject gate summary evidence steps tool-calls tokens cost-usd", "finish": "verdict gate verifier summary steps-total tool-calls-total tokens-total cost-usd-total", "status": "", "migrate": "runner evidence-mode verifier cost-band cost-basis approval-required max-run-cost-usd staging-plan max-age-hours", "retire": "", "restore": "", "badge": "",
}
var boolOptions = []string{"approval-required", "human-signoff", "allow-incomplete-tools", "allow-dirty"}
var intOptions = []string{"max-steps", "max-tool-calls", "max-tokens", "steps", "tool-calls", "tokens", "steps-total", "tool-calls-total", "tokens-total"}
var floatOptions = []string{"max-run-cost-usd", "max-age-hours", "max-wall-seconds", "cost-usd", "cost-usd-total"}

func parseArgs(xs []string) (arguments, error) {
	a := arguments{flags: map[string][]string{}, positional: []string{}}
	for i := 0; i < len(xs); i++ {
		v := xs[i]
		if v == "--help" || v == "-h" {
			a.help = true
			continue
		}
		if strings.HasPrefix(v, "--") {
			k, val, inline := strings.Cut(strings.TrimPrefix(v, "--"), "=")
			if k != "project" && !contains(strings.Fields(options[a.command]), k) {
				return a, bad("unrecognized argument: --%s", k)
			}
			if contains(boolOptions, k) {
				if inline {
					return a, bad("--%s takes no value", k)
				}
				val = "true"
			} else if !inline {
				i++
				if i == len(xs) {
					return a, bad("--%s requires a value", k)
				}
				val = xs[i]
			}
			if contains(intOptions, k) {
				if _, e := strconv.ParseInt(val, 10, 64); e != nil {
					return a, bad("--%s requires an integer", k)
				}
			}
			if contains(floatOptions, k) {
				n, e := strconv.ParseFloat(val, 64)
				if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
					return a, bad("--%s requires a finite number", k)
				}
			}
			a.flags[k] = append(a.flags[k], val)
			continue
		}
		if a.command == "" {
			if _, ok := options[v]; !ok {
				return a, bad("unknown command: %s", v)
			}
			a.command = v
		} else {
			a.positional = append(a.positional, v)
		}
	}
	if a.help {
		return a, nil
	}
	if a.command == "" {
		return a, bad("a command is required")
	}
	min, max := 1, 1
	if contains([]string{"status", "validate"}, a.command) {
		min = 0
	}
	if a.command == "badge" {
		min = 0
		max = 0
	}
	if contains([]string{"record", "finish"}, a.command) {
		min = 2
		max = 2
	}
	if len(a.positional) < min || len(a.positional) > max {
		return a, bad("%s requires %d positional argument(s)", a.command, min)
	}
	required := map[string]string{"init": "name statement include gate plan-step runner evidence-mode verifier cost-band cost-basis", "record": "kind result summary", "finish": "verdict gate summary", "migrate": "runner evidence-mode verifier cost-band cost-basis"}
	for _, k := range strings.Fields(required[a.command]) {
		if !a.has(k) {
			return a, bad("--%s is required", k)
		}
	}
	choices := map[string][]string{"evidence-mode": {"internal", "external", "mixed"}, "cost-band": costBands, "kind": {"coverage", "inspection", "finding", "test", "visual", "tool", "decision", "verification", "blocker"}, "result": {"pass", "fail", "info", "blocked"}, "verdict": {"fulfilled", "not-fulfilled", "blocked", "budget-exhausted"}}
	for k, v := range choices {
		if a.has(k) && !contains(v, a.s(k)) {
			return a, bad("invalid --%s: %s", k, a.s(k))
		}
	}
	for k, v := range map[string]string{"cadence": "manual", "authority": "read-only", "readme": "README.md", "steps": "1", "tool-calls": "0"} {
		if !a.has(k) {
			a.flags[k] = []string{v}
		}
	}
	return a, nil
}
func help(command string) string {
	if command == "" {
		return "Manage durable repository Promise Loop state and README badges.\nUsage: go run -C <skill>/scripts/promise . --project <repository> <command> [flags]\nCommands: init validate start record finish status migrate retire restore badge\nUse <command> --help for options.\n"
	}
	positional := " <promise-id>"
	switch command {
	case "status", "validate":
		positional = " [promise-id]"
	case "record", "finish":
		positional += " <run-id>"
	case "badge":
		positional = ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: promise --project <repository> %s%s [flags]\n\nOptions:\n  --project <repository>  Target Git repository (independent of helper module).\n", command, positional)
	required := map[string]string{"init": "name statement include gate plan-step runner evidence-mode verifier cost-band cost-basis", "record": "kind result summary", "finish": "verdict gate summary", "migrate": "runner evidence-mode verifier cost-band cost-basis"}
	choices := map[string][]string{"evidence-mode": {"internal", "external", "mixed"}, "cost-band": costBands, "kind": {"coverage", "inspection", "finding", "test", "visual", "tool", "decision", "verification", "blocker"}, "result": {"pass", "fail", "info", "blocked"}, "verdict": {"fulfilled", "not-fulfilled", "blocked", "budget-exhausted"}}
	for _, flag := range strings.Fields(options[command]) {
		fmt.Fprintf(&b, "  --%s", flag)
		if !contains(boolOptions, flag) {
			kind := "value"
			if contains(intOptions, flag) {
				kind = "integer"
			} else if contains(floatOptions, flag) {
				kind = "number"
			}
			fmt.Fprintf(&b, " <%s>", kind)
		}
		if contains(strings.Fields(required[command]), flag) {
			b.WriteString(" (required)")
		}
		if contains([]string{"include", "exclude", "gate", "plan-step", "tool"}, flag) {
			b.WriteString(" (repeatable)")
		}
		if xs := choices[flag]; len(xs) > 0 {
			fmt.Fprintf(&b, " [%s]", strings.Join(xs, "|"))
		}
		b.WriteByte('\n')
	}
	return b.String()
}
func initialContract(root string, a arguments) (object, object, error) {
	id := a.id()
	p, e := base(root, id)
	if e != nil {
		return nil, nil, e
	}
	if exists(p) {
		return nil, nil, bad("promise already exists: %s", p)
	}
	r, e := loadRegistry(root)
	if e != nil {
		return nil, nil, e
	}
	gates := []any{}
	seen := map[string]bool{}
	for _, v := range a.flags["gate"] {
		k, d, e := pair(v, "gate")
		if e != nil {
			return nil, nil, e
		}
		if seen[k] {
			return nil, nil, bad("acceptance gate ids must be unique")
		}
		seen[k] = true
		gates = append(gates, object{"id": k, "description": d})
	}
	tools := []any{}
	for _, v := range a.flags["tool"] {
		t, e := parseTool(v)
		if e != nil {
			return nil, nil, e
		}
		tools = append(tools, t)
	}
	label := a.s("badge-label")
	if label == "" {
		label = "Promise: " + a.s("name")
	}
	approval := a.b("approval-required")
	c := object{"schema_version": schema, "id": id, "name": a.s("name"), "statement": a.s("statement"), "created_at": stamp(now()), "scope": object{"include": values(a.flags["include"]), "exclude": values(a.flags["exclude"])}, "acceptance_gates": gates, "human_signoff_required": a.b("human-signoff"), "invalidation": object{"scope_change": true, "max_age_hours": a.n("max-age-hours")}, "evidence": object{"mode": a.s("evidence-mode")}, "verifier": object{"identity": a.s("verifier")}, "economics": object{"full_run_cost_band": a.s("cost-band"), "estimate_basis": a.s("cost-basis"), "approval_required": approval, "max_run_cost_usd": a.n("max-run-cost-usd"), "staging_plan": nullable(a.s("staging-plan"))}, "operation": object{"cadence": a.s("cadence"), "runner": a.s("runner"), "authority": a.s("authority"), "plan_steps": values(a.flags["plan-step"]), "budgets": object{"max_steps": a.n("max-steps"), "max_tool_calls": a.n("max-tool-calls"), "max_tokens": a.n("max-tokens"), "max_wall_seconds": a.n("max-wall-seconds")}}, "tools": tools, "badge": object{"label": label, "readme": a.s("readme")}}
	return c, r, validateContract(root, id, c, false)
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func sortRegistry(r object) {
	x := list(r["promises"])
	slices.SortFunc(x, func(a, b any) int { return strings.Compare(text(obj(a)["id"]), text(obj(b)["id"])) })
	r["promises"] = x
}
func commandInit(root string, a arguments) (object, error) {
	if _, _, e := initialContract(root, a); e != nil {
		return nil, e
	}
	unlock, e := lock(root)
	if e != nil {
		return nil, e
	}
	defer unlock()
	c, r, e := initialContract(root, a)
	if e != nil {
		return nil, e
	}
	p := mustBase(root, a.id())
	for _, d := range []string{"", "tools", "runs", "evidence", ".work"} {
		if e = os.MkdirAll(filepath.Join(p, d), 0755); e != nil {
			return nil, e
		}
	}
	if e = writeJSON(filepath.Join(p, "contract.json"), c); e != nil {
		return nil, e
	}
	if e = atomicWrite(filepath.Join(p, "PROMISE.md"), []byte(document(c))); e != nil {
		return nil, e
	}
	s := object{"schema_version": schema, "id": a.id(), "status": "configured", "active_run": nil, "last_run": nil, "last_fulfillment": nil, "lifetime_metrics": object{"runs": 0, "steps": 0, "tool_calls": 0, "tokens_known": 0, "tokens_complete": true, "cost_usd_known": 0.0, "cost_complete": true, "wall_seconds": 0.0}, "updated_at": stamp(now())}
	if e = writeJSON(filepath.Join(p, "state.json"), s); e != nil {
		return nil, e
	}
	if e = writeJSON(filepath.Join(p, "coverage.json"), object{"schema_version": schema, "subjects": object{}}); e != nil {
		return nil, e
	}
	ignore := filepath.Join(root, ".promises", ".gitignore")
	b, _ := os.ReadFile(ignore)
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(b) == 0 {
		lines = []string{}
	}
	for _, line := range []string{"**/.work/", "**/.*.tmp"} {
		if !contains(lines, line) {
			lines = append(lines, line)
		}
	}
	if e = atomicWrite(ignore, []byte(strings.Join(lines, "\n")+"\n")); e != nil {
		return nil, e
	}
	r["promises"] = append(list(r["promises"]), object{"id": a.id(), "name": a.s("name"), "contract": ".promises/" + a.id() + "/contract.json", "readme": a.s("readme")})
	sortRegistry(r)
	if e = writeJSON(filepath.Join(root, ".promises", "registry.json"), r); e != nil {
		return nil, e
	}
	return object{"id": a.id(), "path": p, "status": "configured"}, nil
}
func idsFor(root, id string) ([]string, error) {
	if id != "" {
		return []string{id}, nil
	}
	r, e := loadRegistry(root)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, v := range list(r["promises"]) {
		out = append(out, text(obj(v)["id"]))
	}
	return out, nil
}
func commandValidate(root string, a arguments) (object, error) {
	if a.id() != "" {
		if e := registered(root, a.id()); e != nil {
			return nil, e
		}
	}
	ids, e := idsFor(root, a.id())
	if e != nil {
		return nil, e
	}
	out := []any{}
	for _, id := range ids {
		c, e := loadContract(root, id, !a.b("allow-incomplete-tools"))
		var s object
		if e == nil {
			s, e = loadState(root, id)
		}
		if e != nil {
			if a.id() != "" {
				return nil, e
			}
			out = append(out, object{"id": id, "error": e.Error()})
			continue
		}
		hash, n, e := subjectFingerprint(root, c)
		if e != nil {
			return nil, e
		}
		if n == 0 {
			e = bad("promise scope resolves to zero files; fix scope.include before running")
			if a.id() != "" {
				return nil, e
			}
			out = append(out, object{"id": id, "subject_files": 0, "error": e.Error()})
			continue
		}
		mech, me := mechanismFingerprint(root, id)
		if me != nil && !a.b("allow-incomplete-tools") {
			return nil, me
		}
		ok, reason, e := validity(root, id, c, s)
		if e != nil {
			return nil, e
		}
		result := object{"id": id, "subject_files": n, "subject_fingerprint": hash, "mechanism_fingerprint": nullable(mech), "fulfilled": ok, "reason": reason}
		if me != nil {
			result["mechanism_error"] = me.Error()
		}
		out = append(out, result)
	}
	return object{"promises": out}, nil
}
func wallSegment(start, end any) (float64, error) {
	s, e := parseTime(start)
	if e != nil {
		return 0, bad("active run has invalid wall-time segment state")
	}
	t, e := parseTime(end)
	if e != nil {
		return 0, bad("active run has invalid wall-time segment state")
	}
	return math.Max(0, t.Sub(s).Seconds()), nil
}
func activeWall(r object) (float64, error) {
	v := r["active_wall_seconds"]
	if v == nil {
		return 0, nil
	}
	if !numeric(v) || number(v) < 0 {
		return 0, bad("active run has invalid accumulated wall time")
	}
	return number(v), nil
}
func segmentStart(r object) any {
	if v, ok := r["active_segment_started_at"]; ok {
		return v
	}
	return r["started_at"]
}
func activeRun(root, id, runID string) (string, object, object, error) {
	s, e := loadState(root, id)
	if e != nil {
		return "", nil, nil, e
	}
	if text(s["active_run"]) != runID {
		return "", nil, nil, bad("%s is not the active run for %s", runID, id)
	}
	p := filepath.Join(mustBase(root, id), "runs", runID, "run.json")
	r, e := readJSON(p)
	if e != nil {
		return "", nil, nil, e
	}
	if text(r["status"]) != "running" {
		return "", nil, nil, bad("run is not active: %s", runID)
	}
	return p, r, s, nil
}
func commandStart(root string, a arguments) (object, error) {
	id := a.id()
	if e := registered(root, id); e != nil {
		return nil, e
	}
	c, e := loadContract(root, id, true)
	if e != nil {
		return nil, e
	}
	s, e := loadState(root, id)
	if e != nil {
		return nil, e
	}
	if rid := text(s["active_run"]); rid != "" {
		p, r, s, e := activeRun(root, id, rid)
		if e != nil {
			return nil, e
		}
		t := stamp(now())
		last := r["last_activity_at"]
		if last == nil {
			last = segmentStart(r)
		}
		seg, e := wallSegment(segmentStart(r), last)
		if e != nil {
			return nil, e
		}
		prior, e := activeWall(r)
		if e != nil {
			return nil, e
		}
		r["active_wall_seconds"] = rounded(prior+seg, 6)
		r["active_segment_started_at"] = t
		r["last_activity_at"] = t
		r["resume_count"] = number(r["resume_count"]) + 1
		if e = writeJSON(p, r); e != nil {
			return nil, e
		}
		if e = appendJSON(filepath.Join(filepath.Dir(p), "events.jsonl"), object{"at": t, "kind": "run-resume", "result": "info", "prior_segment_wall_seconds": seg}); e != nil {
			return nil, e
		}
		s["updated_at"] = t
		if e = writeJSON(filepath.Join(mustBase(root, id), "state.json"), s); e != nil {
			return nil, e
		}
		return object{"id": id, "run_id": rid, "status": "resume", "active_wall_seconds": r["active_wall_seconds"]}, nil
	}
	approval := strings.TrimSpace(a.s("approval-ref"))
	if boolValue(obj(c["economics"])["approval_required"]) && approval == "" {
		return nil, bad("this Promise cost band requires --approval-ref before a run can start")
	}
	head, e := gitText(root, false, "rev-parse", "--verify", "HEAD")
	if e != nil {
		return nil, e
	}
	if head == "" {
		return nil, bad("commit the repository at least once before starting a Promise run")
	}
	dirty, e := dirtyPaths(root, c)
	if e != nil {
		return nil, e
	}
	if len(dirty) > 0 && !a.b("allow-dirty") {
		return nil, bad("promise scope has uncommitted changes; commit or pass --allow-dirty: %s", strings.Join(dirty[:min(10, len(dirty))], ", "))
	}
	t := now()
	random := make([]byte, 4)
	if _, e = rand.Read(random); e != nil {
		return nil, e
	}
	rid := t.Format("20060102T150405Z") + "-" + hex.EncodeToString(random)
	p := filepath.Join(mustBase(root, id), "runs", rid)
	hash, n, e := subjectFingerprint(root, c)
	if e != nil {
		return nil, e
	}
	if n == 0 {
		return nil, bad("promise scope resolves to zero files; fix scope.include before starting")
	}
	if e = os.MkdirAll(p, 0755); e != nil {
		return nil, e
	}
	mech, e := mechanismFingerprint(root, id)
	if e != nil {
		return nil, e
	}
	r := object{"schema_version": schema, "promise_id": id, "run_id": rid, "status": "running", "verdict": nil, "started_at": stamp(t), "active_segment_started_at": stamp(t), "last_activity_at": stamp(t), "active_wall_seconds": 0.0, "resume_count": 0, "ended_at": nil, "base_commit": head, "subject_fingerprint": hash, "subject_file_count": n, "mechanism_fingerprint": mech, "approval_ref": nullable(approval), "dirty_subject_at_start": values(dirty), "observed_metrics": object{"steps": 0, "tool_calls": 0, "tokens_known": 0, "token_samples": 0, "cost_usd_known": 0.0, "cost_samples": 0}, "metrics": nil, "gates": object{}, "verifier": nil, "summary": nil}
	if e = writeJSON(filepath.Join(p, "run.json"), r); e != nil {
		return nil, e
	}
	if e = appendJSON(filepath.Join(p, "events.jsonl"), object{"at": stamp(t), "kind": "run-start", "result": "info", "commit": head, "approval_ref": r["approval_ref"]}); e != nil {
		return nil, e
	}
	s["status"] = "running"
	s["active_run"] = rid
	s["last_run"] = rid
	s["updated_at"] = stamp(t)
	if e = writeJSON(filepath.Join(mustBase(root, id), "state.json"), s); e != nil {
		return nil, e
	}
	return object{"id": id, "run_id": rid, "status": "started"}, nil
}
func expectedGates(c object) []string {
	out := []string{}
	for _, v := range list(c["acceptance_gates"]) {
		out = append(out, text(obj(v)["id"]))
	}
	slices.Sort(out)
	return out
}
func unique(xs []string) []string { x := slices.Clone(xs); slices.Sort(x); return slices.Compact(x) }
func commandRecord(root string, a arguments) (object, error) {
	id := a.id()
	if e := registered(root, id); e != nil {
		return nil, e
	}
	c, e := loadContract(root, id, false)
	if e != nil {
		return nil, e
	}
	p, r, _, e := activeRun(root, id, a.runID())
	if e != nil {
		return nil, e
	}
	gates := unique(a.flags["gate"])
	if gates == nil {
		gates = []string{}
	}
	expected := expectedGates(c)
	for _, g := range gates {
		if !contains(expected, g) {
			return nil, bad("record references unknown gate: %s", g)
		}
	}
	pass := len(gates) > 0 && a.s("result") == "pass"
	if pass && strings.TrimSpace(a.s("evidence")) == "" {
		return nil, bad("passing gate evidence requires --evidence")
	}
	if a.has("cost-usd") && number(a.n("cost-usd")) < 0 {
		return nil, bad("recorded cost_usd cannot be negative")
	}
	observed := obj(r["observed_metrics"])
	if observed == nil {
		return nil, bad("active run has invalid observed_metrics")
	}
	projected := number(observed["cost_usd_known"])
	if a.has("cost-usd") {
		projected += number(a.n("cost-usd"))
	}
	cap := obj(c["economics"])["max_run_cost_usd"]
	if cap != nil && projected > number(cap) && (a.s("kind") != "blocker" || a.s("result") != "blocked") {
		return nil, bad("cost would exceed Promise cap ($%.2f > $%.2f); stop work or record the actual overrun as a blocked blocker event", projected, number(cap))
	}
	if pass && !contains(gateKinds, a.s("kind")) {
		return nil, bad("%s events cannot serve as passing gate evidence", a.s("kind"))
	}
	if pass && !evidenceValid(root, id, text(obj(c["evidence"])["mode"]), a.s("evidence")) {
		return nil, bad("invalid %s gate evidence reference: %s", text(obj(c["evidence"])["mode"]), a.s("evidence"))
	}
	head, e := gitText(root, true, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	t := stamp(now())
	event := object{"at": t, "kind": a.s("kind"), "result": a.s("result"), "subject": nullable(a.s("subject")), "summary": a.s("summary"), "evidence": nullable(a.s("evidence")), "gates": values(gates), "commit": head, "metrics": object{"steps": a.n("steps"), "tool_calls": a.n("tool-calls"), "tokens": a.n("tokens"), "cost_usd": a.n("cost-usd")}}
	if e = appendJSON(filepath.Join(filepath.Dir(p), "events.jsonl"), event); e != nil {
		return nil, e
	}
	observed["steps"] = number(observed["steps"]) + number(a.n("steps"))
	observed["tool_calls"] = number(observed["tool_calls"]) + number(a.n("tool-calls"))
	if a.has("tokens") {
		observed["tokens_known"] = number(observed["tokens_known"]) + number(a.n("tokens"))
		observed["token_samples"] = number(observed["token_samples"]) + 1
	}
	if a.has("cost-usd") {
		observed["cost_usd_known"] = rounded(projected, 6)
		observed["cost_samples"] = number(observed["cost_samples"]) + 1
	}
	r["last_activity_at"] = t
	if e = writeJSON(p, r); e != nil {
		return nil, e
	}
	if a.s("subject") != "" && contains([]string{"coverage", "inspection", "visual", "test"}, a.s("kind")) {
		cp := filepath.Join(mustBase(root, id), "coverage.json")
		coverage, e := readJSON(cp)
		if e != nil {
			return nil, e
		}
		subjects := obj(coverage["subjects"])
		if subjects == nil {
			subjects = object{}
			coverage["subjects"] = subjects
		}
		subjects[a.s("subject")] = object{"reviewed_at": t, "run_id": a.runID(), "commit": head, "kind": a.s("kind"), "result": a.s("result"), "summary": a.s("summary"), "evidence": event["evidence"]}
		if e = writeJSON(cp, coverage); e != nil {
			return nil, e
		}
	}
	return object{"run_id": a.runID(), "recorded": a.s("kind"), "gates": values(gates), "metrics": observed}, nil
}
func passingGates(root, id, mode, p string) (map[string]bool, error) {
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, bad("missing run event log: %s: %w", p, e)
	}
	latest := map[string]bool{}
	for i, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event object
		if e = json.Unmarshal([]byte(line), &event); e != nil {
			return nil, bad("invalid JSON in %s:%d: %w", p, i+1, e)
		}
		if event == nil {
			return nil, bad("expected a JSON object in %s:%d", p, i+1)
		}
		gates, ok := event["gates"].([]any)
		if ok && text(event["result"]) != "info" {
			pass := text(event["result"]) == "pass" && contains(gateKinds, text(event["kind"])) && evidenceValid(root, id, mode, event["evidence"])
			for _, g := range gates {
				if v, ok := g.(string); ok {
					latest[v] = pass
				}
			}
		}
	}
	return latest, nil
}
func gateResults(xs []string) (object, error) {
	out := object{}
	for _, v := range xs {
		k, r, e := pair(v, "gate result")
		if e != nil {
			return nil, e
		}
		if !contains([]string{"pass", "fail", "blocked"}, r) {
			return nil, bad("gate result must be pass, fail, or blocked: %s", v)
		}
		if _, ok := out[k]; ok {
			return nil, bad("duplicate gate result: %s", k)
		}
		out[k] = r
	}
	return out, nil
}
func metrics(a arguments, r object, ended time.Time) (object, error) {
	o := obj(r["observed_metrics"])
	if o == nil {
		return nil, bad("active run has invalid observed_metrics")
	}
	m := object{}
	for _, k := range []string{"steps", "tool_calls"} {
		flag := strings.ReplaceAll(k, "_", "-") + "-total"
		v := o[k]
		source := "event-log"
		if a.has(flag) {
			v = a.n(flag)
			source = "runtime-total"
		}
		if number(v) < number(o[k]) {
			return nil, bad("final metric totals cannot be lower than observed event totals")
		}
		m[k] = v
		m[k+"_source"] = source
	}
	tokens := o["tokens_known"]
	var token any
	ta := "unavailable"
	if a.has("tokens-total") {
		token = a.n("tokens-total")
		if number(token) < number(tokens) {
			return nil, bad("final token total cannot be lower than observed tokens")
		}
		ta = "runtime-total"
	} else if number(o["token_samples"]) > 0 {
		token = tokens
		ta = "partial-event-log"
	}
	m["tokens"] = token
	m["token_accounting"] = ta
	var cost any
	ca := "unavailable"
	if a.has("cost-usd-total") {
		cost = rounded(number(a.n("cost-usd-total")), 6)
		if number(cost) < 0 || number(a.n("cost-usd-total")) < number(o["cost_usd_known"]) {
			return nil, bad("final cost total cannot be negative or lower than observed cost")
		}
		ca = "runtime-total"
	} else if number(o["cost_samples"]) > 0 {
		cost = rounded(number(o["cost_usd_known"]), 6)
		ca = "partial-event-log"
	}
	m["cost_usd"] = cost
	m["cost_accounting"] = ca
	prior, e := activeWall(r)
	if e != nil {
		return nil, e
	}
	segment, e := wallSegment(segmentStart(r), stamp(ended))
	if e != nil {
		return nil, e
	}
	m["wall_seconds"] = rounded(prior+segment, 6)
	m["wall_accounting"] = "active-segments"
	return m, nil
}
func enforceBudgets(c, m object) error {
	b := obj(obj(c["operation"])["budgets"])
	for _, k := range []string{"steps", "tool_calls", "wall_seconds"} {
		cap := b["max_"+k]
		if cap != nil && number(m[k]) > number(cap) {
			return bad("%s exceeded promise budget (%v > %v); finish with --verdict budget-exhausted and non-passing gate results", k, m[k], cap)
		}
	}
	if cap := b["max_tokens"]; cap != nil {
		if m["tokens"] == nil || text(m["token_accounting"]) != "runtime-total" {
			return bad("max_tokens requires an exact --tokens-total at finish")
		}
		if number(m["tokens"]) > number(cap) {
			return bad("tokens exceeded promise budget")
		}
	}
	if cap := obj(c["economics"])["max_run_cost_usd"]; cap != nil {
		if m["cost_usd"] == nil || text(m["cost_accounting"]) != "runtime-total" {
			return bad("max_run_cost_usd requires an exact --cost-usd-total at finish")
		}
		if number(m["cost_usd"]) > number(cap) {
			return bad("cost exceeded Promise cap")
		}
	}
	return nil
}
func commandFinish(root string, a arguments) (object, error) {
	id := a.id()
	if e := registered(root, id); e != nil {
		return nil, e
	}
	c, e := loadContract(root, id, false)
	if e != nil {
		return nil, e
	}
	p, r, s, e := activeRun(root, id, a.runID())
	if e != nil {
		return nil, e
	}
	ended := now()
	gates, e := gateResults(a.flags["gate"])
	if e != nil {
		return nil, e
	}
	expected := expectedGates(c)
	if len(gates) != len(expected) {
		return nil, bad("gate results must match contract")
	}
	for _, g := range expected {
		if _, ok := gates[g]; !ok {
			return nil, bad("gate results must match contract; missing=%s", g)
		}
	}
	m, e := metrics(a, r, ended)
	if e != nil {
		return nil, e
	}
	verdict := a.s("verdict")
	if verdict == "fulfilled" {
		for _, v := range gates {
			if text(v) != "pass" {
				return nil, bad("a fulfilled run requires every acceptance gate to pass")
			}
		}
		if strings.TrimSpace(a.s("verifier")) != text(obj(c["verifier"])["identity"]) {
			return nil, bad("fulfilled verifier must exactly match contract identity: %s", text(obj(c["verifier"])["identity"]))
		}
		if number(obj(r["observed_metrics"])["steps"]) < 1 {
			return nil, bad("a fulfilled run requires recorded evidence events")
		}
		passing, e := passingGates(root, id, text(obj(c["evidence"])["mode"]), filepath.Join(filepath.Dir(p), "events.jsonl"))
		if e != nil {
			return nil, e
		}
		for _, g := range expected {
			if !passing[g] {
				return nil, bad("fulfilled gates need passing durable evidence: %s", g)
			}
		}
		hash, n, e := subjectFingerprint(root, c)
		if e != nil {
			return nil, e
		}
		if n == 0 {
			return nil, bad("a fulfilled run cannot attest to an empty promise scope")
		}
		if hash != text(r["subject_fingerprint"]) {
			return nil, bad("promise scope changed during the run; start a new run")
		}
		mech, e := mechanismFingerprint(root, id)
		if e != nil {
			return nil, e
		}
		if mech != text(r["mechanism_fingerprint"]) {
			return nil, bad("promise contract or tools changed during the run; start a new run")
		}
		if e = enforceBudgets(c, m); e != nil {
			return nil, e
		}
	}
	r["status"] = "finished"
	r["verdict"] = verdict
	r["ended_at"] = stamp(ended)
	r["metrics"] = m
	r["gates"] = gates
	r["verifier"] = a.s("verifier")
	r["summary"] = a.s("summary")
	if e = writeJSON(p, r); e != nil {
		return nil, e
	}
	if e = appendJSON(filepath.Join(filepath.Dir(p), "events.jsonl"), object{"at": stamp(ended), "kind": "run-finish", "result": verdict, "summary": a.s("summary"), "verifier": a.s("verifier"), "gates": gates, "metrics": m}); e != nil {
		return nil, e
	}
	s["status"] = verdict
	s["active_run"] = nil
	s["last_run"] = a.runID()
	s["updated_at"] = stamp(ended)
	l := obj(s["lifetime_metrics"])
	for _, k := range []string{"runs", "steps", "tool_calls"} {
		n := 1.0
		if k != "runs" {
			n = number(m[k])
		}
		l[k] = number(l[k]) + n
	}
	l["wall_seconds"] = rounded(number(l["wall_seconds"])+number(m["wall_seconds"]), 3)
	if m["tokens"] != nil {
		l["tokens_known"] = number(l["tokens_known"]) + number(m["tokens"])
	}
	if text(m["token_accounting"]) != "runtime-total" {
		l["tokens_complete"] = false
	}
	if m["cost_usd"] != nil {
		l["cost_usd_known"] = rounded(number(l["cost_usd_known"])+number(m["cost_usd"]), 6)
	}
	if text(m["cost_accounting"]) != "runtime-total" {
		l["cost_complete"] = false
	}
	if verdict == "fulfilled" {
		s["last_fulfillment"] = object{"run_id": a.runID(), "fulfilled_at": stamp(ended), "validated_commit": r["base_commit"], "subject_fingerprint": r["subject_fingerprint"], "mechanism_fingerprint": r["mechanism_fingerprint"], "verifier": a.s("verifier"), "gates": gates, "metrics": m, "summary": a.s("summary"), "dirty_subject_at_start": r["dirty_subject_at_start"], "approval_ref": r["approval_ref"]}
	} else {
		clear := verdict == "not-fulfilled"
		for _, v := range gates {
			if text(v) == "fail" {
				clear = true
			}
		}
		if clear {
			s["last_fulfillment"] = nil
		}
	}
	if e = writeJSON(filepath.Join(mustBase(root, id), "state.json"), s); e != nil {
		return nil, e
	}
	badges, e := renderBadges(root, nil)
	if e != nil {
		return nil, bad("run finished, but badge reconciliation failed: %w; repair registry or README, then run promise badge", e)
	}
	return object{"id": id, "run_id": a.runID(), "verdict": verdict, "metrics": m, "badges": badges}, nil
}
func commandStatus(root string, a arguments) (object, error) {
	ids, e := idsFor(root, a.id())
	if e != nil {
		return nil, e
	}
	r, e := loadRegistry(root)
	if e != nil {
		return nil, e
	}
	reg := map[string]bool{}
	for _, v := range list(r["promises"]) {
		reg[text(obj(v)["id"])] = true
	}
	out := []any{}
	for _, id := range ids {
		c, e := loadContract(root, id, false)
		var s object
		if e == nil {
			s, e = loadState(root, id)
		}
		if e != nil {
			status := "invalid"
			if strings.Contains(e.Error(), "schema 1") {
				status = "migration-required"
			}
			out = append(out, object{"id": id, "status": status, "error": e.Error()})
			continue
		}
		ok, reason, e := validity(root, id, c, s)
		if e != nil {
			return nil, e
		}
		if ok && !reg[id] {
			reason = "valid attestation retained; promise is retired or unregistered"
		}
		out = append(out, object{"id": id, "name": c["name"], "status": s["status"], "registered": reg[id], "active_run": s["active_run"], "badge_valid": ok && reg[id], "validity": reason, "last_fulfillment": s["last_fulfillment"], "lifetime_metrics": s["lifetime_metrics"]})
	}
	return object{"promises": out}, nil
}
func allSchema2(root string, r object, candidateID string, candidate object) bool {
	for _, v := range list(r["promises"]) {
		id := text(obj(v)["id"])
		c := candidate
		if id != candidateID || c == nil {
			var e error
			c, e = readJSON(filepath.Join(mustBase(root, id), "contract.json"))
			if e != nil {
				return false
			}
		}
		if number(c["schema_version"]) != schema {
			return false
		}
	}
	return true
}
func economicsArgs(a arguments) object {
	return object{"full_run_cost_band": a.s("cost-band"), "estimate_basis": a.s("cost-basis"), "approval_required": a.b("approval-required"), "max_run_cost_usd": a.n("max-run-cost-usd"), "staging_plan": nullable(a.s("staging-plan"))}
}
func commandMigrate(root string, a arguments) (object, error) {
	id := a.id()
	p, e := base(root, id)
	if e != nil {
		return nil, e
	}
	if !exists(p) {
		return nil, bad("promise does not exist: %s", id)
	}
	c, e := readJSON(filepath.Join(p, "contract.json"))
	if e != nil {
		return nil, e
	}
	s, e := readJSON(filepath.Join(p, "state.json"))
	if e != nil {
		return nil, e
	}
	coverage, e := readJSON(filepath.Join(p, "coverage.json"))
	if e != nil {
		return nil, e
	}
	all := true
	for _, m := range []object{c, s, coverage} {
		v := number(m["schema_version"])
		if v != 1 && v != schema {
			return nil, bad("cannot migrate unsupported schemas for %s", id)
		}
		if v != schema {
			all = false
		}
	}
	r, e := loadRegistry(root)
	if e != nil {
		return nil, e
	}
	if all {
		if e = validateContract(root, id, c, true); e != nil {
			return nil, e
		}
		if _, e = loadState(root, id); e != nil {
			return nil, e
		}
		all2 := allSchema2(root, r, "", nil)
		if all2 && number(r["schema_version"]) != schema {
			r["schema_version"] = schema
			if e = writeJSON(filepath.Join(root, ".promises", "registry.json"), r); e != nil {
				return nil, e
			}
		}
		ok, _, e := validity(root, id, c, s)
		if e != nil {
			return nil, e
		}
		updated := []string{}
		if !ok {
			updated, e = removeBadge(root, id, text(obj(c["badge"])["readme"]))
			if e != nil {
				return nil, e
			}
		}
		valid := []string{}
		if ok && registered(root, id) == nil {
			valid = append(valid, id)
		}
		return object{"id": id, "status": "already-schema-2", "badges": object{"updated": values(updated), "valid": values(valid), "deferred": !all2}}, nil
	}
	if nonempty(s["active_run"]) {
		return nil, bad("finish the active run before migrating this Promise")
	}
	if text(s["id"]) != id {
		return nil, bad("state does not match %s", id)
	}
	verifier := obj(c["verifier"])
	if verifier == nil {
		verifier = object{}
	}
	verifier["identity"] = a.s("verifier")
	c["verifier"] = verifier
	c["schema_version"] = schema
	op := obj(c["operation"])
	if op == nil {
		return nil, bad("contract needs an operation object")
	}
	op["runner"] = a.s("runner")
	c["evidence"] = object{"mode": a.s("evidence-mode")}
	c["economics"] = economicsArgs(a)
	if a.has("max-age-hours") {
		inv := obj(c["invalidation"])
		if inv == nil {
			return nil, bad("contract needs invalidation")
		}
		inv["max_age_hours"] = a.n("max-age-hours")
	}
	if e = validateContract(root, id, c, true); e != nil {
		return nil, e
	}
	l := obj(s["lifetime_metrics"])
	if l == nil {
		return nil, bad("schema-1 state needs lifetime_metrics")
	}
	l["cost_usd_known"] = 0.0
	l["cost_complete"] = number(l["runs"]) == 0
	s["schema_version"] = schema
	s["updated_at"] = stamp(now())
	coverage["schema_version"] = schema
	all2 := allSchema2(root, r, id, c)
	if all2 {
		r["schema_version"] = schema
	} // Keep interrupted migrations resumable: state and coverage precede contract.
	if e = writeJSON(filepath.Join(p, "state.json"), s); e != nil {
		return nil, e
	}
	if e = writeJSON(filepath.Join(p, "coverage.json"), coverage); e != nil {
		return nil, e
	}
	if e = atomicWrite(filepath.Join(p, "PROMISE.md"), []byte(document(c))); e != nil {
		return nil, e
	}
	if e = writeJSON(filepath.Join(p, "contract.json"), c); e != nil {
		return nil, e
	}
	if e = writeJSON(filepath.Join(root, ".promises", "registry.json"), r); e != nil {
		return nil, e
	}
	var badges object
	if all2 {
		badges, e = renderBadges(root, nil)
	} else {
		var updated []string
		updated, e = removeBadge(root, id, text(obj(c["badge"])["readme"]))
		badges = object{"updated": values(updated), "valid": []any{}, "deferred": true}
	}
	if e != nil {
		return nil, e
	}
	return object{"id": id, "status": "migrated", "badges": badges}, nil
}
func commandRetire(root string, a arguments) (object, error) {
	r, e := loadRegistry(root)
	if e != nil {
		return nil, e
	}
	kept := []any{}
	var entry object
	for _, v := range list(r["promises"]) {
		if text(obj(v)["id"]) == a.id() {
			entry = obj(v)
		} else {
			kept = append(kept, v)
		}
	}
	if entry == nil {
		return nil, bad("promise is not registered: %s", a.id())
	}
	p := filepath.Join(mustBase(root, a.id()), "state.json")
	if exists(p) {
		s, e := loadState(root, a.id())
		if e != nil {
			return nil, e
		}
		if nonempty(s["active_run"]) {
			return nil, bad("finish the active run before retiring this promise")
		}
		s["status"] = "retired"
		s["updated_at"] = stamp(now())
		if e = writeJSON(p, s); e != nil {
			return nil, e
		}
	}
	r["promises"] = kept
	if e = writeJSON(filepath.Join(root, ".promises", "registry.json"), r); e != nil {
		return nil, e
	}
	badges, e := renderBadges(root, []string{text(entry["readme"])})
	if e != nil {
		return nil, e
	}
	return object{"id": a.id(), "status": "retired", "badges": badges}, nil
}
func commandRestore(root string, a arguments) (object, error) {
	r, e := loadRegistry(root)
	if e != nil {
		return nil, e
	}
	for _, v := range list(r["promises"]) {
		if text(obj(v)["id"]) == a.id() {
			return nil, bad("promise is already registered: %s", a.id())
		}
	}
	c, e := loadContract(root, a.id(), false)
	if e != nil {
		return nil, e
	}
	s, e := loadState(root, a.id())
	if e != nil {
		return nil, e
	}
	ok, _, e := validity(root, a.id(), c, s)
	if e != nil {
		return nil, e
	}
	r["promises"] = append(list(r["promises"]), object{"id": a.id(), "name": c["name"], "contract": ".promises/" + a.id() + "/contract.json", "readme": obj(c["badge"])["readme"]})
	sortRegistry(r)
	if e = writeJSON(filepath.Join(root, ".promises", "registry.json"), r); e != nil {
		return nil, e
	}
	if !nonempty(s["active_run"]) {
		s["status"] = "configured"
		if ok {
			s["status"] = "fulfilled"
		}
	}
	s["updated_at"] = stamp(now())
	if e = writeJSON(filepath.Join(mustBase(root, a.id()), "state.json"), s); e != nil {
		return nil, e
	}
	badges, e := renderBadges(root, nil)
	if e != nil {
		return nil, e
	}
	return object{"id": a.id(), "status": "restored", "badges": badges}, nil
}
func execute(root string, a arguments) (object, error) {
	if a.command == "init" {
		return commandInit(root, a)
	}
	if a.id() != "" {
		if _, e := base(root, a.id()); e != nil {
			return nil, e
		}
	}
	if a.command != "status" && a.command != "validate" && exists(filepath.Join(root, ".promises")) {
		unlock, e := lock(root)
		if e != nil {
			return nil, e
		}
		defer unlock()
	}
	switch a.command {
	case "validate":
		return commandValidate(root, a)
	case "start":
		return commandStart(root, a)
	case "record":
		return commandRecord(root, a)
	case "finish":
		return commandFinish(root, a)
	case "status":
		return commandStatus(root, a)
	case "migrate":
		return commandMigrate(root, a)
	case "retire":
		return commandRetire(root, a)
	case "restore":
		return commandRestore(root, a)
	case "badge":
		return renderBadges(root, nil)
	}
	return nil, bad("unknown command: %s", a.command)
}
func run(xs []string) error {
	a, e := parseArgs(xs)
	if e != nil {
		return e
	}
	if a.help {
		fmt.Print(help(a.command))
		return nil
	}
	project := a.s("project")
	if project == "" {
		project, e = os.Getwd()
		if e != nil {
			return e
		}
	}
	root, e := repoRoot(project)
	if e != nil {
		return e
	}
	out, e := execute(root, a)
	if e != nil {
		return e
	}
	b, e := json.MarshalIndent(out, "", "  ")
	if e != nil {
		return e
	}
	fmt.Println(string(b))
	return nil
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "error:", e)
		os.Exit(2)
	}
}
