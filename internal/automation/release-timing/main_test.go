package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const tagSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const start = "2026-10-14T04:00:00Z"
const now = "2026-10-14T04:12:00Z"

var testPolicy = policy{Goal: 300, Maximum: 600, HookMaximum: 3, HookGoal: 1, EnforceAfter: start}
var prep = run{ID: 1, SHA: sha, Branch: "main", Event: "push", Created: start, Status: "in_progress", URL: "https://github.com/example/project/actions/runs/1"}
var child = run{ID: 2, SHA: tagSHA, Branch: "v1.2.3", Created: "2026-10-14T04:03:00Z", URL: "https://github.com/example/project/actions/runs/2"}

type fakeGH struct {
	data     map[string]any
	failures map[string]error
	issues   []issue
	created  []map[string]string
	reads    map[string]int
	api      *github
}

func fake(t *testing.T) *fakeGH {
	t.Helper()
	f := &fakeGH{data: map[string]any{}, failures: map[string]error{}, reads: map[string]int{}, issues: []issue{}, created: []map[string]string{}}
	f.data["repos/example/project/commits/"+sha] = map[string]any{"commit": map[string]any{"message": "Application change", "author": map[string]string{"name": "developer"}}}
	f.data["repos/example/project/commits/"+tagSHA] = map[string]any{"commit": map[string]any{"message": "chore: release\n\nRelease-Source: " + sha, "author": map[string]string{"name": "github-actions[bot]"}}}
	f.setJobs(1, nil)
	f.setJobs(2, nil)
	f.data["repos/example/project/actions/workflows/auto-release.yml/runs?per_page=100"] = map[string]any{"workflow_runs": []run{prep}}
	f.data["repos/example/project/actions/workflows/release.yml/runs?per_page=100"] = map[string]any{"workflow_runs": []run{}}
	f.data["repos/example/project/actions/runs/1"] = prep
	f.api = &github{cache: map[string][]byte{}, sleep: func(time.Duration) {}, call: func(args []string, input []byte) ([]byte, error) {
		if args[0] == "--method" {
			if args[1] != "POST" || args[2] != "repos/example/project/issues" {
				t.Fatalf("unexpected mutation %v", args)
			}
			var payload map[string]string
			if err := json.Unmarshal(input, &payload); err != nil {
				t.Fatal(err)
			}
			f.created = append(f.created, payload)
			i := issue{URL: fmt.Sprintf("https://github.com/example/project/issues/%d", 42+len(f.created)-1), Body: payload["body"]}
			f.issues = append(f.issues, i)
			return json.Marshal(i)
		}
		if args[0] == "--paginate" {
			return json.Marshal([][]issue{f.issues})
		}
		path := args[0]
		f.reads[path]++
		if err := f.failures[path]; err != nil {
			return nil, err
		}
		value, exists := f.data[path]
		if !exists {
			return nil, fmt.Errorf("unexpected endpoint %s", path)
		}
		return json.Marshal(value)
	}}
	return f
}
func (f *fakeGH) setJobs(id int64, jobs []job) {
	if jobs == nil {
		jobs = []job{}
	}
	f.data[fmt.Sprintf("repos/example/project/actions/runs/%d/jobs?filter=all&per_page=100", id)] = map[string]any{"jobs": jobs}
}
func (f *fakeGH) publication(tap string) {
	f.setJobs(2, []job{{Name: "Publish GitHub Release and Homebrew Casks", Started: "2026-10-14T04:08:00Z", Completed: "2026-10-14T04:09:00Z", Status: "completed", Conclusion: "success", Steps: []step{{Name: "Update Homebrew CLI and app Casks", Conclusion: "success", Completed: tap}}}})
	f.data["repos/example/project/releases/tags/v1.2.3"] = map[string]any{"draft": false, "published_at": "2026-10-14T04:08:20Z", "assets": []map[string]string{{"name": "appcast.xml", "state": "uploaded", "updated_at": "2026-10-14T04:08:21Z"}}}
}
func measureOK(t *testing.T, f *fakeGH, date string, children []run, preparations []run) measurement {
	t.Helper()
	m, err := measure(f.api, "example/project", prep, children, testPolicy, date, preparations)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func durationJob(name, begin, end string) job {
	return job{Name: name, Started: begin, Completed: end, Status: "completed", Conclusion: "success", Attempt: 1}
}

func TestLimitAndFailedCandidateWallTime(t *testing.T) {
	f := fake(t)
	m := measureOK(t, f, "2026-10-14T04:10:00Z", nil, nil)
	if m.Violation || m.Elapsed != 600 {
		t.Fatalf("boundary %+v", m)
	}
	if i, err := ensureIssue(f.api, "example/project", m); err != nil || i != nil {
		t.Fatalf("unexpected issue %v %v", i, err)
	}
	m = measureOK(t, f, "2026-10-14T04:10:01Z", nil, nil)
	if !m.Violation {
		t.Fatal("strict boundary missed")
	}
	failed := prep
	failed.Status = "completed"
	failed.Conclusion = "failure"
	m, err := measure(f.api, "example/project", failed, nil, testPolicy, now, nil)
	if err != nil || m.Elapsed != 720 || !m.Violation || m.Delivery != "not published" {
		t.Fatalf("failed candidate %+v %v", m, err)
	}
}
func TestIssueDeduplicatesSourceAcrossRetriesAndClosedIssues(t *testing.T) {
	f := fake(t)
	m := measureOK(t, f, now, nil, nil)
	first, err := ensureIssue(f.api, "example/project", m)
	if err != nil || !first.Created {
		t.Fatalf("%+v %v", first, err)
	}
	m.RunID = 99
	second, err := ensureIssue(f.api, "example/project", m)
	if err != nil || second.Created || len(f.created) != 1 {
		t.Fatalf("%+v %v %v", second, err, f.created)
	}
	for _, text := range []string{sha, "720s", "600s maximum", prep.URL} {
		if !strings.Contains(f.created[0]["body"], text) {
			t.Errorf("missing %q", text)
		}
	}
}
func TestPublicationStopsClockAtLatestFeedAndTap(t *testing.T) {
	f := fake(t)
	f.publication("2026-10-14T04:08:50Z")
	m := measureOK(t, f, now, []run{child}, nil)
	if m.Elapsed != 530 || m.Violation || m.Delivery != "published feed and tap" {
		t.Fatalf("%+v", m)
	}
}
func TestParallelAndQualificationIntervalsAreNotSummed(t *testing.T) {
	f := fake(t)
	f.setJobs(1, []job{durationJob("Check release source", start, "2026-10-14T04:00:30Z")})
	f.setJobs(2, []job{durationJob("Prepare shared release inputs", "2026-10-14T04:01:00Z", "2026-10-14T04:02:00Z"), durationJob("Qualify release source", "2026-10-14T04:02:00Z", "2026-10-14T04:06:00Z"), durationJob("CLI (linux_amd64)", "2026-10-14T04:02:00Z", "2026-10-14T04:04:00Z"), durationJob("Signed and notarized macOS release assets", "2026-10-14T04:02:00Z", "2026-10-14T04:05:00Z")})
	m := measureOK(t, f, now, []run{child}, nil)
	if m.Active != 330 || !m.ActiveComplete || m.Platform == nil || *m.Platform != 180 || m.Elapsed != 720 {
		t.Fatalf("%+v", m)
	}
}
func TestRetryStagesKeepAttemptsAndOriginalStart(t *testing.T) {
	f := fake(t)
	first := durationJob("Check release source", start, "2026-10-14T04:02:00Z")
	first.Conclusion = "failure"
	second := durationJob("Check release source", "2026-10-14T04:05:00Z", "2026-10-14T04:06:00Z")
	second.Attempt = 2
	f.setJobs(1, []job{first, second})
	m := measureOK(t, f, now, nil, nil)
	if len(m.Stages) != 2 || m.Stages[0].Attempt != 1 || m.Stages[1].Attempt != 2 || m.Elapsed != 720 {
		t.Fatalf("%+v", m)
	}
}
func TestDocumentationSkipAndFailedSourceCheck(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			f := fake(t)
			check := durationJob("Check release source", start, start)
			if failure {
				check.Conclusion = "failure"
			}
			skip := durationJob("Tag and publish checked source", start, start)
			skip.Conclusion = "skipped"
			f.setJobs(1, []job{check, skip})
			r := prep
			r.Status = "completed"
			r.Conclusion = "success"
			m, err := measure(f.api, "example/project", r, nil, testPolicy, now, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !failure && m.Excluded != "deliberate no-release change" || failure && !m.Violation {
				t.Fatalf("%+v", m)
			}
		})
	}
}
func TestMetadataAndSupersededPreparationsExcluded(t *testing.T) {
	t.Run("metadata", func(t *testing.T) {
		f := fake(t)
		f.data["repos/example/project/commits/"+sha] = map[string]any{"commit": map[string]any{"message": "chore: release\n\nRelease-Source: " + strings.Repeat("c", 40), "author": map[string]string{"name": "github-actions[bot]"}}}
		if m := measureOK(t, f, now, nil, nil); m.Excluded != "release metadata commit" {
			t.Fatalf("%+v", m)
		}
	})
	t.Run("superseded", func(t *testing.T) {
		f := fake(t)
		j := durationJob("Tag and publish checked source", start, now)
		j.Steps = []step{{Name: "Dispatch tagged build and wait for publication", Conclusion: "skipped"}}
		f.setJobs(1, []job{j})
		if m := measureOK(t, f, now, nil, nil); m.Excluded != "superseded preparation" {
			t.Fatalf("%+v", m)
		}
	})
}
func TestMissingPublicationTimestampRemainsError(t *testing.T) {
	f := fake(t)
	f.publication("")
	if _, err := measure(f.api, "example/project", prep, []run{child}, testPolicy, now, nil); err == nil {
		t.Fatal("missing publication timestamp accepted")
	}
}
func TestSeparateRetryRunKeepsFirstMainClock(t *testing.T) {
	f := fake(t)
	retry := prep
	retry.ID = 3
	retry.Event = "workflow_dispatch"
	retry.Created = "2026-10-14T04:07:00Z"
	f.setJobs(3, nil)
	m, err := measure(f.api, "example/project", retry, nil, testPolicy, now, []run{retry, prep})
	if err != nil || m.Start != start || m.Elapsed != 720 {
		t.Fatalf("%+v %v", m, err)
	}
}
func TestQueuedAndSkippedJobsDoNotRequireTimestamps(t *testing.T) {
	f := fake(t)
	f.setJobs(1, []job{{Name: "Release", Status: "completed", Conclusion: "skipped"}, {Name: "Queued", Status: "queued"}})
	m := measureOK(t, f, now, nil, nil)
	if len(m.Stages) != 0 || m.Queue != 720 {
		t.Fatalf("%+v", m)
	}
}
func TestCancelledHistoricalTimestampIsExposedAndScanContinues(t *testing.T) {
	f := fake(t)
	// Exact GitHub job timestamps from cancelled run 38590047563, job 115944920729.
	first := prep
	first.Created = "2026-10-14T15:30:00Z"
	later := prep
	later.ID = 3
	later.SHA = strings.Repeat("c", 40)
	later.Created = "2026-10-14T16:00:00Z"
	f.data["repos/example/project/commits/"+later.SHA] = f.data["repos/example/project/commits/"+sha]
	f.setJobs(1, []job{{Name: "Publish GitHub Release and Homebrew Casks", Started: "2026-10-14T15:36:32Z", Completed: "2026-10-14T15:36:31Z", Status: "completed", Conclusion: "cancelled", Attempt: 2}})
	f.setJobs(3, nil)
	f.data["repos/example/project/actions/workflows/auto-release.yml/runs?per_page=100"] = map[string]any{"workflow_runs": []run{later, first}}
	result := scan(f.api, options{repo: "example/project", scan: true, measure: true}, testPolicy, "2026-10-14T16:12:00Z")
	if len(result.Errors) != 0 || len(result.Measurements) != 2 {
		t.Fatalf("%+v", result)
	}
	m := result.Measurements[0]
	if m.ActiveComplete || len(m.Stages) != 1 || m.Stages[0].Seconds != nil || m.Stages[0].TimingError == "" || m.Stages[0].Start != "2026-10-14T15:36:32Z" || m.Stages[0].End != "2026-10-14T15:36:31Z" {
		t.Fatalf("gap hidden: %+v", m)
	}
	if result.Measurements[1].SHA != later.SHA {
		t.Fatal("later source not processed")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"seconds":null`)) {
		t.Fatal("unavailable duration was manufactured")
	}
}
func TestReadRetriesAreBoundedAndMutationsNotRetried(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		want          int
	}{{"transient", "gh: HTTP 504", 3}, {"rate-limit", "gh: HTTP 429", 3}, {"authorization", "gh: HTTP 403", 1}, {"missing", "gh: HTTP 404", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			waits := []time.Duration{}
			g := &github{cache: map[string][]byte{}, sleep: func(d time.Duration) { waits = append(waits, d) }, call: func([]string, []byte) ([]byte, error) { calls++; return nil, errors.New(tc.message) }}
			if _, err := g.read("endpoint"); err == nil || calls != tc.want {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if len(waits) != tc.want-1 {
				t.Fatalf("waits %v", waits)
			}
		})
	}
	calls := 0
	g := &github{cache: map[string][]byte{}, sleep: func(time.Duration) { t.Fatal("mutation retry") }, call: func([]string, []byte) ([]byte, error) { calls++; return nil, errors.New("HTTP 504") }}
	if _, err := createIssue(g, "example/project", "title", "body"); err == nil || calls != 1 {
		t.Fatalf("calls %d error %v", calls, err)
	}
}
func TestReadRetryRecoveryAndCache(t *testing.T) {
	calls := 0
	g := &github{cache: map[string][]byte{}, sleep: func(time.Duration) {}, call: func([]string, []byte) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("HTTP 502")
		}
		return []byte(`{}`), nil
	}}
	for range 2 {
		if _, err := g.read("endpoint"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls %d", calls)
	}
}
func TestPerSourceAPIFailureDoesNotPreventLaterSourceAndWritesReport(t *testing.T) {
	f := fake(t)
	later := prep
	later.ID = 3
	later.SHA = strings.Repeat("c", 40)
	later.Created = "2026-10-14T04:03:00Z"
	f.data["repos/example/project/commits/"+later.SHA] = f.data["repos/example/project/commits/"+sha]
	f.setJobs(3, nil)
	f.data["repos/example/project/actions/workflows/auto-release.yml/runs?per_page=100"] = map[string]any{"workflow_runs": []run{prep, later}}
	endpoint := "repos/example/project/actions/runs/1/jobs?filter=all&per_page=100"
	f.failures[endpoint] = errors.New("HTTP 504")
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.json")
	reportPath := filepath.Join(dir, "report.json")
	data, err := json.Marshal(testPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err = execute([]string{"--repo", "example/project", "--scan", "--measure", "--policy", policyPath, "--report", reportPath, "--now", now}, f.api, &stdout)
	if err == nil || f.reads[endpoint] != 3 {
		t.Fatalf("reads %d error %v", f.reads[endpoint], err)
	}
	data, err = os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var result report
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 1 || result.Errors[0].SHA != sha || len(result.Measurements) != 1 || result.Measurements[0].SHA != later.SHA || len(f.created) != 0 {
		t.Fatalf("%+v mutations=%v", result, f.created)
	}
}
func TestHookBoundaryAndIncompleteChecksDeduplicate(t *testing.T) {
	f := fake(t)
	elapsed := 3.0
	h := hookReport{SHA: sha, Elapsed: &elapsed, Incomplete: []string{}, Checks: []string{"format", "Go affected tests"}}
	result, err := checkHookReport(f.api, "example/project", h, testPolicy)
	if err != nil || result.Violation {
		t.Fatalf("%+v %v", result, err)
	}
	elapsed = 3.001
	result, err = checkHookReport(f.api, "example/project", h, testPolicy)
	if err != nil || !result.Issue.Created {
		t.Fatalf("%+v %v", result, err)
	}
	elapsed = 2.99
	h.Incomplete = []string{"Go affected tests"}
	result, err = checkHookReport(f.api, "example/project", h, testPolicy)
	if err != nil || result.Issue.Created || len(f.created) != 1 {
		t.Fatalf("%+v %v", result, err)
	}
}
func TestAdvisoryViolationsExitSuccessfully(t *testing.T) {
	for _, hook := range []bool{false, true} {
		t.Run(fmt.Sprint(hook), func(t *testing.T) {
			f := fake(t)
			dir := t.TempDir()
			policyPath := filepath.Join(dir, "policy.json")
			data, err := json.Marshal(testPolicy)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(policyPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"--repo", "example/project", "--policy", policyPath, "--now", now}
			if hook {
				path := filepath.Join(dir, "hook.json")
				if err := os.WriteFile(path, []byte(`{"sha":"`+sha+`","elapsed_seconds":4,"budget_exceeded":true,"incomplete_checks":[]}`), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--hook-report", path)
			} else {
				args = append(args, "--run-id", "1")
			}
			if err := execute(args, f.api, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			if len(f.created) != 1 {
				t.Fatalf("created %v", f.created)
			}
		})
	}
}
func TestMissingAndReversedTimestampValidation(t *testing.T) {
	if _, err := seconds("", now); err == nil {
		t.Fatal("missing timestamp accepted")
	}
	if _, err := seconds(now, start); err == nil {
		t.Fatal("reversed timestamp accepted")
	}
	if _, err := timestamp("2026-10-14T04:00:00"); err == nil {
		t.Fatal("timezone missing accepted")
	}
}
func TestHourlyHookJobIndependentOfObserver(t *testing.T) {
	data, err := os.ReadFile("../../../.github/workflows/release-performance.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	_, hourly, ok := strings.Cut(workflow, "  hook-observe:\n")
	if !ok {
		t.Fatal("no independent hourly hook job")
	}
	if strings.Contains(hourly, "needs:") {
		t.Fatal("hook job depends on release observer")
	}
	if !strings.Contains(hourly, "--benchmark --report /tmp/hook-timing.json") || !strings.Contains(hourly, "--hook-report /tmp/hook-timing.json") {
		t.Fatal("hourly measurement/report commands missing")
	}
}

func TestCompletedJobWithMissingEndPreservesUnavailableTiming(t *testing.T) {
	result := stages([]job{{Name: "Cancelled", Status: "completed", Conclusion: "cancelled", Started: start}}, now)
	if len(result) != 1 || result[0].End != "" || result[0].Seconds != nil || result[0].TimingError == "" {
		t.Fatalf("%+v", result)
	}
}
func TestPlatformGapDoesNotClaimCompleteCriticalPath(t *testing.T) {
	f := fake(t)
	f.setJobs(2, []job{{Name: "CLI (linux_amd64)", Started: now, Completed: start, Status: "completed", Conclusion: "cancelled"}, durationJob("CLI (linux_arm64)", start, "2026-10-14T04:03:00Z")})
	m := measureOK(t, f, now, []run{child}, nil)
	if m.PlatformComplete || m.ActiveComplete || m.Platform == nil || *m.Platform != 180 {
		t.Fatalf("%+v", m)
	}
}
func TestHookProcessingContinuesAfterGlobalScanError(t *testing.T) {
	f := fake(t)
	f.failures["repos/example/project/actions/workflows/auto-release.yml/runs?per_page=100"] = errors.New("HTTP 504")
	dir := t.TempDir()
	p := filepath.Join(dir, "policy.json")
	h := filepath.Join(dir, "hook.json")
	data, err := json.Marshal(testPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h, []byte(`{"sha":"`+sha+`","elapsed_seconds":4,"budget_exceeded":true,"incomplete_checks":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := execute([]string{"--repo", "example/project", "--scan", "--policy", p, "--hook-report", h, "--now", now}, f.api, &stdout); err == nil {
		t.Fatal("API failure falsely green")
	}
	if len(f.created) != 1 || !strings.Contains(f.created[0]["body"], "<!-- hook-timing:") {
		t.Fatalf("hook not observed %v", f.created)
	}
	var result report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Hook == nil || len(result.Errors) != 1 {
		t.Fatalf("%+v", result)
	}
}
