package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	root, e := filepath.EvalSymlinks(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		if _, e := git(root, true, args...); e != nil {
			t.Fatal(e)
		}
	}
	write(t, filepath.Join(root, "src", "one.go"), "package fixture\n")
	write(t, filepath.Join(root, "README.md"), "# Fixture\n\nBody.\n")
	commit(t, root)
	return root
}
func write(t *testing.T, p, s string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(s), 0644); e != nil {
		t.Fatal(e)
	}
}
func commit(t *testing.T, root string) {
	t.Helper()
	if _, e := git(root, true, "add", "."); e != nil {
		t.Fatal(e)
	}
	if _, e := git(root, true, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "fixture"); e != nil {
		t.Fatal(e)
	}
}
func args(t *testing.T, xs ...string) arguments {
	t.Helper()
	a, e := parseArgs(xs)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func call(t *testing.T, root string, xs ...string) object {
	t.Helper()
	out, e := execute(root, args(t, xs...))
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func rejects(t *testing.T, root, want string, xs ...string) {
	t.Helper()
	_, e := execute(root, args(t, xs...))
	if e == nil || !strings.Contains(e.Error(), want) {
		t.Fatalf("%v: want error containing %q, got %v", xs, want, e)
	}
}
func initPromise(t *testing.T, root, id string, extra ...string) {
	t.Helper()
	xs := []string{"init", id, "--name", "Fixture", "--statement", "Scoped condition", "--include", "src/**", "--gate", "inventory=Scoped inventory", "--plan-step", "Inspect scope", "--runner", "fixture operator", "--evidence-mode", "internal", "--verifier", "fixture-check", "--cost-band", "lt-10", "--cost-basis", "Deterministic fixture"}
	xs = append(xs, extra...)
	call(t, root, xs...)
}
func start(t *testing.T, root, id string) string {
	t.Helper()
	return text(call(t, root, "start", id)["run_id"])
}
func evidence(t *testing.T, root, id string) string {
	t.Helper()
	p := ".promises/" + id + "/evidence/result.json"
	write(t, filepath.Join(root, p), "{\"passed\":true}\n")
	return p
}
func record(t *testing.T, root, id, rid, ref string) {
	t.Helper()
	call(t, root, "record", id, rid, "--kind", "inspection", "--result", "pass", "--gate", "inventory", "--subject", "src/one.go", "--summary", "Inspected file", "--evidence", ref)
}
func fulfill(t *testing.T, root, id, rid string, extra ...string) object {
	t.Helper()
	xs := []string{"finish", id, rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "Verified"}
	return call(t, root, append(xs, extra...)...)
}
func firstStatus(t *testing.T, root, id string) object {
	t.Helper()
	return obj(list(call(t, root, "status", id)["promises"])[0])
}
func state(t *testing.T, root, id string) object {
	t.Helper()
	s, e := loadState(root, id)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestLifecycleCoverageBadgesRetirement(t *testing.T) {
	root := fixture(t)
	initPromise(t, root, "quality", "--include", "README.md")
	ref := evidence(t, root, "quality")
	commit(t, root)
	call(t, root, "validate", "quality")
	rid := start(t, root, "quality")
	record(t, root, "quality", rid, ref)
	out := fulfill(t, root, "quality", rid, "--tokens-total", "0", "--cost-usd-total", "0")
	if text(out["verdict"]) != "fulfilled" {
		t.Fatal(out)
	}
	s := firstStatus(t, root, "quality")
	if !boolValue(s["badge_valid"]) {
		t.Fatal(s)
	}
	b, e := os.ReadFile(filepath.Join(root, "README.md"))
	if e != nil || !strings.Contains(string(b), "fulfilled+") {
		t.Fatalf("badge: %s %v", b, e)
	}
	coverage, e := readJSON(filepath.Join(mustBase(root, "quality"), "coverage.json"))
	if e != nil || text(obj(obj(coverage["subjects"])["src/one.go"])["result"]) != "pass" {
		t.Fatal(coverage, e)
	}
	l := obj(s["lifetime_metrics"])
	if number(l["runs"]) != 1 || !boolValue(l["tokens_complete"]) || !boolValue(l["cost_complete"]) {
		t.Fatal(l)
	}
	call(t, root, "badge")
	call(t, root, "retire", "quality")
	rejects(t, root, "not registered", "start", "quality")
	rejects(t, root, "not registered", "validate", "quality")
	s = firstStatus(t, root, "quality")
	if boolValue(s["badge_valid"]) || boolValue(s["registered"]) || !strings.Contains(text(s["validity"]), "retired") {
		t.Fatal(s)
	}
	call(t, root, "restore", "quality")
	if !boolValue(firstStatus(t, root, "quality")["badge_valid"]) {
		t.Fatal("retained attestation did not restore")
	}
	b, _ = os.ReadFile(filepath.Join(root, "README.md"))
	if !strings.Contains(string(b), badgeStart) {
		t.Fatal("missing restored badge")
	}
	write(t, filepath.Join(root, "src", "one.go"), "package changed\n")
	call(t, root, "badge")
	s = firstStatus(t, root, "quality")
	if boolValue(s["badge_valid"]) || text(s["validity"]) != "promise scope changed" {
		t.Fatal(s)
	}
	b, _ = os.ReadFile(filepath.Join(root, "README.md"))
	if strings.Contains(string(b), badgeStart) {
		t.Fatal("stale badge retained")
	}
}

func TestEvidenceLatestOutcomeVerifierAndCoverage(t *testing.T) {
	root := fixture(t)
	initPromise(t, root, "quality")
	ref := evidence(t, root, "quality")
	rid := start(t, root, "quality")
	rejects(t, root, "requires recorded evidence", "finish", "quality", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "empty")
	rejects(t, root, "requires --evidence", "record", "quality", rid, "--kind", "inspection", "--result", "pass", "--gate", "inventory", "--summary", "empty")
	rejects(t, root, "cannot serve", "record", "quality", rid, "--kind", "decision", "--result", "pass", "--gate", "inventory", "--evidence", ref, "--summary", "decision")
	rejects(t, root, "unknown gate", "record", "quality", rid, "--kind", "test", "--result", "pass", "--gate", "unknown", "--evidence", ref, "--summary", "unknown")
	record(t, root, "quality", rid, ref)
	rejects(t, root, "exactly match", "finish", "quality", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "other", "--summary", "wrong")
	call(t, root, "record", "quality", rid, "--kind", "test", "--result", "fail", "--gate", "inventory", "--subject", "src/one.go", "--summary", "Failed after prior pass")
	call(t, root, "record", "quality", rid, "--kind", "decision", "--result", "info", "--gate", "inventory", "--summary", "Info cannot override failure")
	rejects(t, root, "passing durable evidence", "finish", "quality", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "stale evidence")
	coverage, _ := readJSON(filepath.Join(mustBase(root, "quality"), "coverage.json"))
	if text(obj(obj(coverage["subjects"])["src/one.go"])["result"]) != "fail" {
		t.Fatal(coverage)
	}
	record(t, root, "quality", rid, ref)
	fulfill(t, root, "quality", rid)
}

func TestDirtyEmptyScopeMissingToolsAndChangedMechanism(t *testing.T) {
	t.Run("dirty", func(t *testing.T) {
		root := fixture(t)
		initPromise(t, root, "quality")
		write(t, filepath.Join(root, "src", "one.go"), "package dirty\n")
		rejects(t, root, "uncommitted changes", "start", "quality")
		rid := text(call(t, root, "start", "quality", "--allow-dirty")["run_id"])
		record(t, root, "quality", rid, evidence(t, root, "quality"))
		fulfill(t, root, "quality", rid)
		b, _ := os.ReadFile(filepath.Join(root, "README.md"))
		if !strings.Contains(string(b), "fulfilled+dirty") {
			t.Fatal(string(b))
		}
	})
	t.Run("zero", func(t *testing.T) {
		root := fixture(t)
		initPromise(t, root, "quality", "--exclude", "src/**")
		rejects(t, root, "zero files", "validate", "quality")
		rejects(t, root, "zero files", "start", "quality")
	})
	t.Run("tools", func(t *testing.T) {
		root := fixture(t)
		initPromise(t, root, "quality", "--tool", "tools/check.sh=Check scope")
		call(t, root, "status", "quality")
		call(t, root, "validate", "quality", "--allow-incomplete-tools")
		rejects(t, root, "not a file", "start", "quality")
		p := filepath.Join(mustBase(root, "quality"), "tools", "check.sh")
		write(t, p, "exit 0\n")
		rid := start(t, root, "quality")
		ref := evidence(t, root, "quality")
		record(t, root, "quality", rid, ref)
		write(t, p, "exit 1\n")
		rejects(t, root, "contract or tools changed", "finish", "quality", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "changed")
		if e := os.Remove(p); e != nil {
			t.Fatal(e)
		}
		call(t, root, "record", "quality", rid, "--kind", "blocker", "--result", "blocked", "--summary", "Tool missing")
		call(t, root, "finish", "quality", rid, "--verdict", "blocked", "--gate", "inventory=blocked", "--summary", "Close blocked run")
	})
}

func TestExternalEvidenceFreshnessAndConfinement(t *testing.T) {
	root := fixture(t)
	initPromise(t, root, "quality", "--evidence-mode", "mixed", "--max-age-hours", "1")
	ref := evidence(t, root, "quality")
	for _, p := range []string{ref, "https://example.invalid/report", "provider:request-123", "sha256:" + strings.Repeat("a", 64)} {
		if !evidenceValid(root, "quality", "mixed", p) {
			t.Fatal("invalid reference", p)
		}
	}
	outside := filepath.Join(t.TempDir(), "report")
	write(t, outside, "external")
	if e := os.Symlink(outside, filepath.Join(mustBase(root, "quality"), "evidence", "escape")); e != nil {
		t.Fatal(e)
	}
	if evidenceValid(root, "quality", "internal", ".promises/quality/evidence/escape") {
		t.Fatal("symlink evidence escaped repository")
	}
	if evidenceValid(root, "quality", "internal", "README.md") {
		t.Fatal("internal evidence escaped promise")
	}
	rid := start(t, root, "quality")
	record(t, root, "quality", rid, "provider:request-123")
	fulfill(t, root, "quality", rid)
	s := state(t, root, "quality")
	obj(s["last_fulfillment"])["fulfilled_at"] = stamp(now().Add(-2 * time.Hour))
	if e := writeJSON(filepath.Join(mustBase(root, "quality"), "state.json"), s); e != nil {
		t.Fatal(e)
	}
	if text(firstStatus(t, root, "quality")["validity"]) != "promise fulfillment expired" {
		t.Fatal("freshness did not expire")
	}
}

func TestBudgetsCostApprovalAndAccounting(t *testing.T) {
	root := fixture(t)
	initPromise(t, root, "cost", "--cost-band", "100-1k", "--approval-required", "--max-run-cost-usd", "200", "--max-tokens", "100", "--max-steps", "5")
	rejects(t, root, "approval-ref", "start", "cost")
	rid := text(call(t, root, "start", "cost", "--approval-ref", "ticket-123")["run_id"])
	ref := evidence(t, root, "cost")
	call(t, root, "record", "cost", rid, "--kind", "test", "--result", "pass", "--gate", "inventory", "--evidence", ref, "--summary", "Measured", "--cost-usd", "150", "--tokens", "10", "--tool-calls", "2")
	rejects(t, root, "tokens-total", "finish", "cost", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "Need exact totals")
	rejects(t, root, "cost-usd-total", "finish", "cost", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "Need exact cost", "--tokens-total", "10")
	rejects(t, root, "lower than observed cost", "finish", "cost", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "Underreported", "--tokens-total", "10", "--cost-usd-total", "149")
	rejects(t, root, "exceed Promise cap", "record", "cost", rid, "--kind", "test", "--result", "pass", "--summary", "Exceeds", "--cost-usd", "60")
	call(t, root, "record", "cost", rid, "--kind", "blocker", "--result", "blocked", "--summary", "Actual overrun", "--cost-usd", "60")
	rejects(t, root, "cost exceeded", "finish", "cost", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "Exceeded", "--tokens-total", "10", "--cost-usd-total", "210")
	call(t, root, "finish", "cost", rid, "--verdict", "budget-exhausted", "--gate", "inventory=blocked", "--summary", "Stop", "--cost-usd-total", "210", "--tokens-total", "10")
	s := state(t, root, "cost")
	if s["active_run"] != nil || number(obj(s["lifetime_metrics"])["cost_usd_known"]) != 210 || s["last_fulfillment"] != nil {
		t.Fatal(s)
	}
	for _, flag := range []string{"--max-steps", "--max-tool-calls", "--max-wall-seconds", "--max-tokens"} {
		t.Run(flag, func(t *testing.T) {
			root := fixture(t)
			initPromise(t, root, "quality", flag, "0")
			rid := start(t, root, "quality")
			ref := evidence(t, root, "quality")
			call(t, root, "record", "quality", rid, "--kind", "test", "--result", "pass", "--gate", "inventory", "--evidence", ref, "--summary", "Measure", "--tool-calls", "1", "--tokens", "1")
			p, r, _, e := activeRun(root, "quality", rid)
			if e != nil {
				t.Fatal(e)
			}
			r["active_segment_started_at"] = stamp(now().Add(-time.Second))
			if e = writeJSON(p, r); e != nil {
				t.Fatal(e)
			}
			rejects(t, root, "exceeded", "finish", "quality", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "Budget", "--tokens-total", "1")
		})
	}
}

func TestResumeExcludesIdleAndBlockedPreservesAttestation(t *testing.T) {
	root := fixture(t)
	initPromise(t, root, "quality")
	ref := evidence(t, root, "quality")
	rid := start(t, root, "quality")
	p, r, _, e := activeRun(root, "quality", rid)
	if e != nil {
		t.Fatal(e)
	}
	r["active_segment_started_at"] = stamp(now().Add(-time.Hour))
	r["last_activity_at"] = stamp(now().Add(-time.Hour + 10*time.Second))
	if e = writeJSON(p, r); e != nil {
		t.Fatal(e)
	}
	resumed := call(t, root, "start", "quality")
	if text(resumed["run_id"]) != rid || number(resumed["active_wall_seconds"]) != 10 {
		t.Fatal(resumed)
	}
	record(t, root, "quality", rid, ref)
	out := fulfill(t, root, "quality", rid)
	if number(obj(out["metrics"])["wall_seconds"]) < 10 || number(obj(out["metrics"])["wall_seconds"]) > 30 {
		t.Fatal(out)
	}
	rid = start(t, root, "quality")
	rejects(t, root, "active run", "retire", "quality")
	call(t, root, "finish", "quality", rid, "--verdict", "blocked", "--gate", "inventory=blocked", "--summary", "Blocked safely")
	if !boolValue(firstStatus(t, root, "quality")["badge_valid"]) {
		t.Fatal("blocked run cleared valid attestation")
	}
	rid = start(t, root, "quality")
	call(t, root, "finish", "quality", rid, "--verdict", "not-fulfilled", "--gate", "inventory=blocked", "--summary", "Withdraw")
	if state(t, root, "quality")["last_fulfillment"] != nil {
		t.Fatal("not-fulfilled retained attestation")
	}
}

func TestGlobSymlinkAndBadgeFingerprints(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{{"src/*", "src/a/b", false}, {"src/**", "src/a/b", true}, {"**/*.go", "a.go", true}, {"src/?.go", "src/a.go", true}, {"src/?.go", "src/ab.go", false}, {"a/**/z", "a/z", true}, {"a/**/z", "a/b/z", true}} {
		if got := patternRE(tc.pattern).MatchString(tc.path); got != tc.want {
			t.Fatalf("%q %q: %v", tc.pattern, tc.path, got)
		}
	}
	root := fixture(t)
	initPromise(t, root, "quality", "--include", "README.md")
	c, e := loadContract(root, "quality", false)
	if e != nil {
		t.Fatal(e)
	}
	before, n, e := subjectFingerprint(root, c)
	if e != nil || n != 2 {
		t.Fatal(n, e)
	}
	p := filepath.Join(root, "README.md")
	b, _ := os.ReadFile(p)
	write(t, p, insertBadges(string(b), []string{"[![test](https://example.invalid)](.promises/quality/PROMISE.md)"}))
	after, _, e := subjectFingerprint(root, c)
	if e != nil || before != after {
		t.Fatal("managed badge altered subject", e)
	}
	if dirty, e := dirtyPaths(root, c); e != nil || len(dirty) != 0 {
		t.Fatal(dirty, e)
	}
	link := filepath.Join(root, "src", "link.go")
	if e = os.Symlink("one.go", link); e != nil {
		t.Fatal(e)
	}
	hash, _, e := subjectFingerprint(root, c)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(link); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink("missing.go", link); e != nil {
		t.Fatal(e)
	}
	other, _, e := subjectFingerprint(root, c)
	if e != nil || hash == other {
		t.Fatal("symlink target must alter fingerprint", e)
	}
}
func migrationArgs(id string) []string {
	return []string{"migrate", id, "--runner", "fixture operator", "--evidence-mode", "internal", "--verifier", "fixture-check", "--cost-band", "lt-10", "--cost-basis", "Fixture"}
}
func downgrade(t *testing.T, root, id string, partial bool) {
	t.Helper()
	for _, file := range []string{"contract.json", "state.json", "coverage.json"} {
		p := filepath.Join(mustBase(root, id), file)
		m, e := readJSON(p)
		if e != nil {
			t.Fatal(e)
		}
		if !partial || file == "contract.json" {
			m["schema_version"] = 1
		}
		switch file {
		case "contract.json":
			delete(m, "economics")
			delete(m, "evidence")
			delete(obj(m["operation"]), "runner")
		case "state.json":
			delete(obj(m["lifetime_metrics"]), "cost_usd_known")
			delete(obj(m["lifetime_metrics"]), "cost_complete")
		}
		if e = writeJSON(p, m); e != nil {
			t.Fatal(e)
		}
	}
	r, e := loadRegistry(root)
	if e != nil {
		t.Fatal(e)
	}
	r["schema_version"] = 1
	if e = writeJSON(filepath.Join(root, ".promises", "registry.json"), r); e != nil {
		t.Fatal(e)
	}
}
func TestSchemaOneAndInterruptedMigration(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			root := fixture(t)
			initPromise(t, root, "first")
			initPromise(t, root, "second")
			downgrade(t, root, "first", partial)
			downgrade(t, root, "second", false)
			write(t, filepath.Join(root, "README.md"), insertBadges("# Fixture\n", []string{"[![first](https://example.invalid)](.promises/first/PROMISE.md)", "[![second](https://example.invalid)](.promises/second/PROMISE.md)"}))
			if text(firstStatus(t, root, "first")["status"]) != "migration-required" {
				t.Fatal("old schema not detected")
			}
			rejects(t, root, "schema 1", "start", "first")
			out := call(t, root, migrationArgs("first")...)
			if text(out["status"]) != "migrated" || !boolValue(obj(out["badges"])["deferred"]) {
				t.Fatal(out)
			}
			b, _ := os.ReadFile(filepath.Join(root, "README.md"))
			if strings.Contains(string(b), ".promises/first/PROMISE.md") || !strings.Contains(string(b), ".promises/second/PROMISE.md") {
				t.Fatal("wrong badge migration", string(b))
			}
			call(t, root, migrationArgs("second")...)
			r, e := loadRegistry(root)
			if e != nil || number(r["schema_version"]) != 2 {
				t.Fatal(r, e)
			}
			call(t, root, "validate")
			out = call(t, root, migrationArgs("first")...)
			if text(out["status"]) != "already-schema-2" {
				t.Fatal(out)
			}
		})
	}
}

// Re-execute the test binary to exercise actual OS-level locks and JSON readers.
func TestProcessHelper(t *testing.T) {
	mode := os.Getenv("PROMISE_TEST_MODE")
	if mode == "" {
		return
	}
	root := os.Getenv("PROMISE_TEST_ROOT")
	switch mode {
	case "command":
		if e := run(strings.Split(os.Getenv("PROMISE_TEST_ARGS"), "\x1f")); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(2)
		}
	case "records":
		rid := os.Getenv("PROMISE_TEST_RUN")
		for i := 0; i < 20; i++ {
			a, e := parseArgs([]string{"record", "quality", rid, "--kind", "decision", "--result", "info", "--summary", "Concurrent checkpoint"})
			if e != nil {
				panic(e)
			}
			done := false
			for retry := 0; retry < 200; retry++ {
				_, e = execute(root, a)
				if e == nil {
					done = true
					break
				}
				if !strings.Contains(e.Error(), "another Promise") {
					panic(e)
				}
				time.Sleep(time.Millisecond)
			}
			if !done {
				panic("lock retries exhausted")
			}
		}
	default:
		panic("unknown process helper")
	}
	os.Exit(0)
}
func process(t *testing.T, root, mode string, more ...string) *exec.Cmd {
	t.Helper()
	c := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
	c.Env = append(os.Environ(), "PROMISE_TEST_MODE="+mode, "PROMISE_TEST_ROOT="+root)
	c.Env = append(c.Env, more...)
	return c
}
func TestCrossProcessLockAndConcurrentAtomicState(t *testing.T) {
	root := fixture(t)
	initPromise(t, root, "quality")
	rid := start(t, root, "quality")
	unlock, e := lock(root)
	if e != nil {
		t.Fatal(e)
	}
	c := process(t, root, "command", "PROMISE_TEST_ARGS="+strings.Join([]string{"--project", root, "start", "quality"}, "\x1f"))
	b, e := c.CombinedOutput()
	unlock()
	if e == nil || !strings.Contains(string(b), "another Promise state mutation") {
		t.Fatalf("lock did not reject separate process: %s %v", b, e)
	}
	c = process(t, root, "command", "PROMISE_TEST_ARGS="+strings.Join([]string{"--project", root, "start", "quality"}, "\x1f"))
	b, e = c.CombinedOutput()
	if e != nil {
		t.Fatal(string(b), e)
	}
	var resumed object
	if e = json.Unmarshal(b, &resumed); e != nil || text(resumed["run_id"]) != rid {
		t.Fatal(string(b), e)
	}
	children := []*exec.Cmd{process(t, root, "records", "PROMISE_TEST_RUN="+rid), process(t, root, "records", "PROMISE_TEST_RUN="+rid)}
	outputs := make([]bytes.Buffer, len(children))
	for i, c := range children {
		c.Stdout = &outputs[i]
		c.Stderr = &outputs[i]
		if e = c.Start(); e != nil {
			t.Fatal(e)
		}
	}
	done := make(chan error, 1)
	go func() {
		for _, c := range children {
			if e := c.Wait(); e != nil {
				done <- e
				return
			}
		}
		done <- nil
	}()
	p := filepath.Join(mustBase(root, "quality"), "runs", rid, "run.json")
	reads := 0
	for {
		select {
		case e := <-done:
			if e != nil {
				t.Fatalf("children: %v %s %s", e, outputs[0].String(), outputs[1].String())
			}
			_, r, _, e := activeRun(root, "quality", rid)
			if e != nil || number(obj(r["observed_metrics"])["steps"]) != 40 {
				t.Fatal(r, e)
			}
			b, e := os.ReadFile(filepath.Join(filepath.Dir(p), "events.jsonl"))
			if e != nil || len(strings.Split(strings.TrimSpace(string(b)), "\n")) != 42 {
				t.Fatalf("lost events: %s %v", b, e)
			}
			if reads < 1 {
				t.Fatal("no concurrent reads")
			}
			return
		default:
			if _, e := readJSON(p); e != nil {
				t.Fatal("partial JSON observed", e)
			}
			reads++
			time.Sleep(time.Millisecond)
		}
	}
}

func TestInstalledModuleProjectIndependence(t *testing.T) {
	root := fixture(t)
	source, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	installed := t.TempDir()
	for _, f := range []string{"main.go", "go.mod"} {
		b, e := os.ReadFile(filepath.Join(source, f))
		if e != nil {
			t.Fatal(e)
		}
		write(t, filepath.Join(installed, f), string(b))
	}
	write(t, filepath.Join(root, "go.mod"), "module unrelated.invalid/target\n\ngo 1.27.2\n")
	c := exec.Command("go", "run", "-C", installed, ".", "--project", root, "status")
	c.Dir = root
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatal(string(b), e)
	}
	var out object
	if e = json.Unmarshal(b, &out); e != nil || len(list(out["promises"])) != 0 {
		t.Fatal(string(b), e)
	}
	c = exec.Command("go", "run", "-C", installed, ".", "init", "--help")
	b, e = c.CombinedOutput()
	if e != nil || !strings.Contains(string(b), "--cost-band") {
		t.Fatal(string(b), e)
	}
}

func TestContractConstraintsAndCLI(t *testing.T) {
	root := fixture(t)
	for _, tc := range []struct {
		flags []string
		want  string
	}{{[]string{"--human-signoff"}, "human-signoff"}, {[]string{"--cost-band", "unknown"}, "approval"}, {[]string{"--cost-band", "lt-10", "--max-run-cost-usd", "10"}, "conflicts"}, {[]string{"--cost-band", "10k-100k", "--approval-required", "--max-run-cost-usd", "50000"}, "staging plan"}, {[]string{"--evidence-mode", "external"}, "max_age_hours"}, {[]string{"--include", "../escape"}, "unsafe"}, {[]string{"--readme", "../escape"}, "inside the repository"}} {
		a := args(t, "init", "quality", "--name", "Fixture", "--statement", "Condition", "--include", "src/**", "--gate", "inventory=Inspect", "--plan-step", "Inspect", "--runner", "fixture", "--evidence-mode", "internal", "--verifier", "fixture-check", "--cost-band", "lt-10", "--cost-basis", "Fixture")
		for i := 0; i < len(tc.flags); i++ {
			k := strings.TrimPrefix(tc.flags[i], "--")
			v := "true"
			if !contains(boolOptions, k) {
				i++
				v = tc.flags[i]
			}
			a.flags[k] = []string{v}
		}
		if _, e := execute(root, a); e == nil || !strings.Contains(e.Error(), tc.want) {
			t.Fatalf("%v expected %s got %v", tc.flags, tc.want, e)
		}
	}
	for _, xs := range [][]string{{"init"}, {"record", "quality", "rid", "--kind", "invalid", "--result", "pass", "--summary", "s"}, {"status", "--garbage"}, {"status", "--project"}, {"init", "--max-steps", "1.2"}, {"init", "--max-wall-seconds", "NaN"}, {"badge", "unexpected"}} {
		if _, e := parseArgs(xs); e == nil {
			t.Fatalf("accepted invalid CLI: %v", xs)
		}
	}
	if e := run([]string{"--help"}); e != nil {
		t.Fatal(e)
	}
	for c := range options {
		a, e := parseArgs([]string{c, "--help"})
		if e != nil || !a.help {
			t.Fatal(c, e)
		}
	}
}
func TestFulfillmentRejectsChangedScopeMissingEvidenceAndCorruptEvents(t *testing.T) {
	for _, kind := range []string{"scope", "evidence", "events"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			initPromise(t, root, "quality")
			rid := start(t, root, "quality")
			ref := evidence(t, root, "quality")
			record(t, root, "quality", rid, ref)
			want := ""
			switch kind {
			case "scope":
				write(t, filepath.Join(root, "src", "one.go"), "package changed\n")
				want = "scope changed"
			case "evidence":
				if e := os.Remove(filepath.Join(root, ref)); e != nil {
					t.Fatal(e)
				}
				want = "passing durable evidence"
			case "events":
				p := filepath.Join(mustBase(root, "quality"), "runs", rid, "events.jsonl")
				f, e := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0644)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = f.WriteString("not JSON\n"); e != nil {
					t.Fatal(e)
				}
				if e = f.Close(); e != nil {
					t.Fatal(e)
				}
				want = "invalid JSON"
			}
			rejects(t, root, want, "finish", "quality", rid, "--verdict", "fulfilled", "--gate", "inventory=pass", "--verifier", "fixture-check", "--summary", "Rejected")
			if state(t, root, "quality")["last_fulfillment"] != nil {
				t.Fatal("invalid fulfillment was retained")
			}
		})
	}
}

func TestPartialAccountingAndFreshnessRestore(t *testing.T) {
	root := fixture(t)
	initPromise(t, root, "quality", "--max-age-hours", "1")
	rid := start(t, root, "quality")
	call(t, root, "record", "quality", rid, "--kind", "test", "--result", "pass", "--gate", "inventory", "--evidence", evidence(t, root, "quality"), "--summary", "Partial meters", "--tokens", "5", "--cost-usd", "0.25")
	out := fulfill(t, root, "quality", rid)
	m := obj(out["metrics"])
	if text(m["token_accounting"]) != "partial-event-log" || text(m["cost_accounting"]) != "partial-event-log" {
		t.Fatal(m)
	}
	l := obj(state(t, root, "quality")["lifetime_metrics"])
	if boolValue(l["tokens_complete"]) || boolValue(l["cost_complete"]) {
		t.Fatal(l)
	}
	call(t, root, "retire", "quality")
	s := state(t, root, "quality")
	obj(s["last_fulfillment"])["fulfilled_at"] = stamp(now().Add(-2 * time.Hour))
	if e := writeJSON(filepath.Join(mustBase(root, "quality"), "state.json"), s); e != nil {
		t.Fatal(e)
	}
	call(t, root, "restore", "quality")
	if text(state(t, root, "quality")["status"]) != "configured" || boolValue(firstStatus(t, root, "quality")["badge_valid"]) {
		t.Fatal("expired restore issued a badge")
	}
}
