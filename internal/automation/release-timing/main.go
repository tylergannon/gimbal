// Release timing measures main-to-publication wall time and files advisory issues.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type github struct {
	cache map[string][]byte
	call  func([]string, []byte) ([]byte, error)
	sleep func(time.Duration)
}

func newGitHub() *github {
	return &github{cache: map[string][]byte{}, sleep: time.Sleep, call: func(args []string, input []byte) ([]byte, error) {
		cmd := exec.Command("gh", append([]string{"api"}, args...)...)
		cmd.Stdin = bytes.NewReader(input)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("gh api %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return output, nil
	}}
}

var transientRead = regexp.MustCompile(`(?i)HTTP (429|5[0-9]{2})|connection reset|connection refused|i/o timeout|TLS handshake timeout|temporary failure|error connecting to|unexpected EOF`)

func (g *github) read(args ...string) ([]byte, error) {
	key := strings.Join(args, "\x00")
	if data, ok := g.cache[key]; ok {
		return data, nil
	}
	var data []byte
	var err error
	for attempt := range 3 {
		data, err = g.call(args, nil)
		if err == nil {
			g.cache[key] = data
			return data, nil
		}
		if !transientRead.MatchString(err.Error()) || attempt == 2 {
			break
		}
		g.sleep(time.Duration(1<<attempt) * 250 * time.Millisecond)
	}
	return nil, err
}

func get[T any](g *github, endpoint string) (T, error) {
	var value T
	data, err := g.read(endpoint)
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(data, &value)
	return value, err
}

func collection[T any](g *github, endpoint, key, oldest string) ([]T, error) {
	items := []T{}
	for page := 1; ; page++ {
		path := endpoint
		if page > 1 {
			path += "&page=" + strconv.Itoa(page)
		}
		data, err := get[map[string]json.RawMessage](g, path)
		if err != nil {
			return items, err
		}
		raw, ok := data[key]
		if !ok {
			return items, fmt.Errorf("%s has no %s collection", path, key)
		}
		var values []T
		if err := json.Unmarshal(raw, &values); err != nil {
			return items, err
		}
		items = append(items, values...)
		if len(values) < 100 {
			return items, nil
		}
		if oldest != "" {
			var dates []struct {
				Created string `json:"created_at"`
			}
			if err := json.Unmarshal(raw, &dates); err != nil {
				return items, err
			}
			last, err := timestamp(dates[len(dates)-1].Created)
			if err != nil {
				return items, err
			}
			cutoff, err := timestamp(oldest)
			if err != nil {
				return items, err
			}
			if last.Before(cutoff) {
				return items, nil
			}
		}
	}
}

type policy struct {
	Goal         float64 `json:"goal_seconds"`
	Maximum      float64 `json:"maximum_seconds"`
	HookMaximum  float64 `json:"commit_hook_maximum_seconds"`
	HookGoal     float64 `json:"commit_hook_goal_seconds"`
	EnforceAfter string  `json:"enforce_after"`
}
type run struct {
	ID         int64  `json:"id"`
	SHA        string `json:"head_sha"`
	Branch     string `json:"head_branch"`
	Event      string `json:"event"`
	Created    string `json:"created_at"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"html_url"`
}
type step struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
	Started    string `json:"started_at"`
	Completed  string `json:"completed_at"`
}
type job struct {
	Name             string `json:"name"`
	Started          string `json:"started_at"`
	Completed        string `json:"completed_at"`
	Status           string `json:"status"`
	Conclusion       string `json:"conclusion"`
	Attempt          int    `json:"run_attempt"`
	Steps            []step `json:"steps"`
	PreparationRunID int64  `json:"-"`
	ReleaseRunID     int64  `json:"-"`
}
type commit struct {
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
}
type release struct {
	Draft     bool   `json:"draft"`
	Published string `json:"published_at"`
	Assets    []struct {
		Name    string `json:"name"`
		State   string `json:"state"`
		Updated string `json:"updated_at"`
	} `json:"assets"`
}
type stage struct {
	Name         string   `json:"name"`
	Attempt      int      `json:"attempt"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	Seconds      *float64 `json:"seconds"`
	Status       string   `json:"status"`
	Conclusion   string   `json:"conclusion"`
	ReleaseRunID int64    `json:"release_run_id,omitempty"`
	TimingError  string   `json:"timing_error,omitempty"`
}
type issueResult struct {
	URL     string `json:"url"`
	Created bool   `json:"created"`
}
type measurement struct {
	RunID            int64        `json:"run_id"`
	Excluded         string       `json:"excluded,omitempty"`
	SHA              string       `json:"source_sha,omitempty"`
	URL              string       `json:"source_url,omitempty"`
	Start            string       `json:"start,omitempty"`
	End              *string      `json:"end"`
	Observed         string       `json:"observed_at,omitempty"`
	Elapsed          float64      `json:"elapsed_seconds"`
	Goal             float64      `json:"goal_seconds"`
	Maximum          float64      `json:"maximum_seconds"`
	OverGoal         bool         `json:"over_goal"`
	Violation        bool         `json:"violation"`
	Delivery         string       `json:"delivery,omitempty"`
	Conclusion       string       `json:"preparation_conclusion,omitempty"`
	Tag              *string      `json:"tag"`
	ReleaseURLs      []string     `json:"release_urls"`
	Queue            float64      `json:"preparation_initial_queue_seconds"`
	Active           float64      `json:"active_critical_path_seconds"`
	ActiveComplete   bool         `json:"active_critical_path_complete"`
	Platform         *float64     `json:"parallel_platform_critical_path_seconds"`
	PlatformComplete bool         `json:"parallel_platform_critical_path_complete"`
	Stages           []stage      `json:"stages"`
	Issue            *issueResult `json:"issue"`
}
type scanError struct {
	RunID int64  `json:"run_id,omitempty"`
	SHA   string `json:"source_sha,omitempty"`
	Error string `json:"error"`
}
type report struct {
	Policy       policy        `json:"policy"`
	Measurements []measurement `json:"measurements"`
	Errors       []scanError   `json:"errors"`
	Hook         any           `json:"hook_measurement,omitempty"`
}

func timestamp(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("required timestamp is missing")
	}
	date, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp %q: %w", value, err)
	}
	return date, nil
}
func seconds(start, end string) (float64, error) {
	begin, err := timestamp(start)
	if err != nil {
		return 0, err
	}
	finish, err := timestamp(end)
	if err != nil {
		return 0, err
	}
	elapsed := finish.Sub(begin).Seconds()
	if elapsed < 0 {
		return 0, fmt.Errorf("timestamp order is invalid: %s -> %s", start, end)
	}
	return elapsed, nil
}

var releaseSource = regexp.MustCompile(`(?m)^Release-Source: ([0-9a-f]{40})$`)

func sourceMarker(c commit) string {
	match := releaseSource.FindStringSubmatch(c.Commit.Message)
	if len(match) == 0 {
		return ""
	}
	return match[1]
}
func queued(j job) bool {
	return slices.Contains([]string{"queued", "waiting", "pending", "requested"}, j.Status)
}
func stages(jobs []job, now string) []stage {
	result := []stage{}
	for _, j := range jobs {
		if queued(j) || j.Conclusion == "skipped" {
			continue
		}
		end := j.Completed
		if end == "" && j.Status != "completed" {
			end = now
		}
		attempt := j.Attempt
		if attempt == 0 {
			attempt = 1
		}
		s := stage{Name: j.Name, Attempt: attempt, Start: j.Started, End: end, Status: j.Status, Conclusion: j.Conclusion, ReleaseRunID: j.ReleaseRunID}
		duration, err := seconds(s.Start, s.End)
		if err != nil {
			s.TimingError = err.Error()
		} else {
			s.Seconds = &duration
		}
		result = append(result, s)
	}
	return result
}
func mainPreparation(r run) bool {
	return r.Branch == "main" && (r.Event == "push" || r.Event == "workflow_dispatch")
}
func findStep(j job, name string) *step {
	for _, s := range j.Steps {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

func measure(g *github, repo string, r run, releases []run, p policy, now string, preparations []run) (measurement, error) {
	result := measurement{RunID: r.ID}
	exclude := func(reason string) (measurement, error) { result.Excluded = reason; return result, nil }
	if !mainPreparation(r) {
		return exclude("not a main release preparation")
	}
	base := "repos/" + repo
	c, err := get[commit](g, base+"/commits/"+r.SHA)
	if err != nil {
		return result, err
	}
	if sourceMarker(c) != "" && c.Commit.Author.Name == "github-actions[bot]" {
		return exclude("release metadata commit")
	}
	if len(preparations) == 0 {
		preparations = []run{r}
	}
	slices.SortFunc(preparations, func(a, b run) int { return strings.Compare(a.Created, b.Created) })
	latest := preparations[len(preparations)-1]
	if latest.Conclusion == "cancelled" {
		return exclude("coalesced or explicitly cancelled preparation")
	}
	start := preparations[0].Created
	startTime, err := timestamp(start)
	if err != nil {
		return result, err
	}
	sourceJobs := []job{}
	for _, prep := range preparations {
		jobs, err := collection[job](g, fmt.Sprintf("%s/actions/runs/%d/jobs?filter=all&per_page=100", base, prep.ID), "jobs", "")
		if err != nil {
			return result, err
		}
		for _, j := range jobs {
			j.PreparationRunID = prep.ID
			sourceJobs = append(sourceJobs, j)
		}
	}
	latestAttempt := 1
	for _, j := range sourceJobs {
		if j.PreparationRunID == latest.ID {
			latestAttempt = max(latestAttempt, j.Attempt)
		}
	}
	var preparation *job
	failed := false
	for _, j := range sourceJobs {
		attempt := max(1, j.Attempt)
		if j.PreparationRunID != latest.ID || attempt != latestAttempt {
			continue
		}
		if j.Name == "Tag and publish checked source" {
			preparation = &j
		}
		if slices.Contains([]string{"failure", "timed_out", "cancelled", "action_required"}, j.Conclusion) {
			failed = true
		}
	}
	if preparation != nil {
		if preparation.Conclusion == "skipped" && !failed && latest.Status == "completed" {
			return exclude("deliberate no-release change")
		}
		dispatch := findStep(*preparation, "Dispatch tagged build and wait for publication")
		if preparation.Conclusion == "success" && dispatch != nil && dispatch.Conclusion == "skipped" {
			return exclude("superseded preparation")
		}
	}
	matches := []run{}
	for _, candidate := range releases {
		created, err := timestamp(candidate.Created)
		if err != nil {
			return result, err
		}
		if created.Before(startTime) {
			continue
		}
		tagged, err := get[commit](g, base+"/commits/"+candidate.SHA)
		if err != nil {
			return result, err
		}
		if sourceMarker(tagged) == r.SHA {
			matches = append(matches, candidate)
		}
	}
	slices.SortFunc(matches, func(a, b run) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	releaseJobs := []job{}
	for _, child := range matches {
		jobs, err := collection[job](g, fmt.Sprintf("%s/actions/runs/%d/jobs?filter=all&per_page=100", base, child.ID), "jobs", "")
		if err != nil {
			return result, err
		}
		for _, j := range jobs {
			j.ReleaseRunID = child.ID
			releaseJobs = append(releaseJobs, j)
		}
	}
	var end, tag *string
	if len(matches) > 0 {
		tag = &matches[len(matches)-1].Branch
	}
	tapEnds := []time.Time{}
	for _, j := range releaseJobs {
		for _, s := range j.Steps {
			if s.Name == "Update Homebrew CLI and app Casks" && s.Conclusion == "success" {
				finish, err := timestamp(s.Completed)
				if err != nil {
					return result, err
				}
				tapEnds = append(tapEnds, finish)
			}
		}
	}
	if len(tapEnds) > 0 {
		published, err := get[release](g, base+"/releases/tags/"+*tag)
		if err != nil {
			return result, err
		}
		if published.Draft {
			return result, errors.New("homebrew gate succeeded but release is still a draft")
		}
		appcastTimes := []string{}
		for _, asset := range published.Assets {
			if asset.Name == "appcast.xml" && asset.State == "uploaded" {
				appcastTimes = append(appcastTimes, asset.Updated)
			}
		}
		if len(appcastTimes) != 1 {
			return result, errors.New("published release has no unique uploaded appcast.xml")
		}
		finish := slices.MinFunc(tapEnds, time.Time.Compare)
		for _, date := range []string{published.Published, appcastTimes[0]} {
			dateTime, err := timestamp(date)
			if err != nil {
				return result, err
			}
			if dateTime.After(finish) {
				finish = dateTime
			}
		}
		value := finish.Format(time.RFC3339Nano)
		end = &value
	}
	elapsedEnd := now
	if end != nil {
		elapsedEnd = *end
	}
	elapsed, err := seconds(start, elapsedEnd)
	if err != nil {
		return result, err
	}
	allStages := append(stages(sourceJobs, now), stages(releaseJobs, now)...)
	type interval struct{ begin, end time.Time }
	intervals := []interval{}
	activeComplete, platformComplete := true, true
	platformAttempts := map[[2]int64]float64{}
	for _, s := range allStages {
		platform := strings.HasPrefix(s.Name, "CLI (") || s.Name == "Signed and notarized macOS release assets"
		if s.Seconds == nil {
			activeComplete = false
			if platform {
				platformComplete = false
			}
			continue
		}
		if platform {
			key := [2]int64{s.ReleaseRunID, int64(s.Attempt)}
			platformAttempts[key] = max(platformAttempts[key], *s.Seconds)
		}
		if s.Name != "Tag and publish checked source" {
			begin, _ := timestamp(s.Start)
			finish, _ := timestamp(s.End)
			intervals = append(intervals, interval{begin, finish})
		}
	}
	for _, j := range sourceJobs {
		if j.Name != "Tag and publish checked source" || j.Conclusion == "skipped" || queued(j) {
			continue
		}
		prefixEnd := j.Completed
		if prefixEnd == "" {
			prefixEnd = now
		}
		if dispatch := findStep(j, "Dispatch tagged build and wait for publication"); dispatch != nil && dispatch.Started != "" {
			prefixEnd = dispatch.Started
		}
		if _, err := seconds(j.Started, prefixEnd); err != nil {
			activeComplete = false
			continue
		}
		begin, _ := timestamp(j.Started)
		finish, _ := timestamp(prefixEnd)
		intervals = append(intervals, interval{begin, finish})
	}
	slices.SortFunc(intervals, func(a, b interval) int { return a.begin.Compare(b.begin) })
	var lastEnd time.Time
	active := 0.0
	for _, interval := range intervals {
		if lastEnd.IsZero() || interval.begin.After(lastEnd) {
			active += interval.end.Sub(interval.begin).Seconds()
		} else if interval.end.After(lastEnd) {
			active += interval.end.Sub(lastEnd).Seconds()
		}
		if interval.end.After(lastEnd) {
			lastEnd = interval.end
		}
	}
	started := []time.Time{}
	for _, j := range sourceJobs {
		if j.Started != "" && !queued(j) {
			begin, err := timestamp(j.Started)
			if err == nil && !begin.Before(startTime) {
				started = append(started, begin)
			} else {
				activeComplete = false
			}
		}
	}
	queue := elapsed
	if len(started) > 0 {
		queue = slices.MinFunc(started, time.Time.Compare).Sub(startTime).Seconds()
	}
	var platform *float64
	if len(platformAttempts) > 0 {
		sum := 0.0
		for _, duration := range platformAttempts {
			sum += duration
		}
		platform = &sum
	}
	urls := []string{}
	for _, child := range matches {
		urls = append(urls, child.URL)
	}
	delivery := "not published"
	if end != nil {
		delivery = "published feed and tap"
	}
	return measurement{RunID: r.ID, SHA: r.SHA, URL: r.URL, Start: start, End: end, Observed: now, Elapsed: elapsed, Goal: p.Goal, Maximum: p.Maximum, OverGoal: elapsed > p.Goal, Violation: elapsed > p.Maximum, Delivery: delivery, Conclusion: latest.Conclusion, Tag: tag, ReleaseURLs: urls, Queue: queue, Active: active, ActiveComplete: activeComplete, Platform: platform, PlatformComplete: platformComplete, Stages: allStages}, nil
}

type issue struct {
	URL         string          `json:"html_url"`
	Body        string          `json:"body"`
	PullRequest json.RawMessage `json:"pull_request,omitempty"`
}

func existingIssue(g *github, repo, marker string) (*issueResult, error) {
	data, err := g.read("--paginate", "--slurp", "repos/"+repo+"/issues?state=all&sort=created&direction=desc&per_page=100")
	if err != nil {
		return nil, err
	}
	var pages [][]issue
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, err
	}
	for _, page := range pages {
		for _, i := range page {
			if (len(i.PullRequest) == 0 || string(i.PullRequest) == "null") && strings.Contains(i.Body, marker) {
				return &issueResult{URL: i.URL}, nil
			}
		}
	}
	return nil, nil
}
func createIssue(g *github, repo, title, body string) (*issueResult, error) {
	input, err := json.Marshal(map[string]string{"title": title, "body": body})
	if err != nil {
		return nil, err
	}
	// Mutations are never retried: a lost response may already have created the issue.
	data, err := g.call([]string{"--method", "POST", "repos/" + repo + "/issues", "--input", "-"}, input)
	if err != nil {
		return nil, err
	}
	var i issue
	if err := json.Unmarshal(data, &i); err != nil {
		return nil, err
	}
	// A subsequent lookup in this same scan must see the newly created issue.
	delete(g.cache, strings.Join([]string{"--paginate", "--slurp", "repos/" + repo + "/issues?state=all&sort=created&direction=desc&per_page=100"}, "\x00"))
	return &issueResult{URL: i.URL, Created: true}, nil
}
func ensureIssue(g *github, repo string, m measurement) (*issueResult, error) {
	if !m.Violation {
		return nil, nil
	}
	marker := "<!-- release-timing:" + m.SHA + " -->"
	existing, err := existingIssue(g, repo, marker)
	if err != nil || existing != nil {
		return existing, err
	}
	end := "not published by " + m.Observed
	if m.End != nil {
		end = *m.End
	}
	rows := []string{"| Stage / attempt | Seconds | Result |", "| --- | ---: | --- |"}
	for _, s := range m.Stages {
		duration := "unavailable: " + s.TimingError
		if s.Seconds != nil {
			duration = fmt.Sprintf("%.0f", *s.Seconds)
		}
		conclusion := s.Conclusion
		if conclusion == "" {
			conclusion = s.Status
		}
		rows = append(rows, fmt.Sprintf("| %s / %d | %s | %s |", s.Name, s.Attempt, duration, conclusion))
	}
	lines := []string{marker, fmt.Sprintf("Main-to-publication time exceeded the %.0fs maximum (%.0fs goal).", m.Maximum, m.Goal), fmt.Sprintf("Source: `%s`. Preparation: %s.", m.SHA, m.URL), fmt.Sprintf("Start: %s. End: %s.", m.Start, end), fmt.Sprintf("Elapsed: **%.0fs**. Delivery: %s.", m.Elapsed, m.Delivery)}
	lines = append(lines, m.ReleaseURLs...)
	lines = append(lines, "", "Wall time includes queueing and retries. Parent tagging waits overlap the child pipeline; do not sum the stage table. Active critical-path time counts overlapping execution intervals once.", "Source qualification overlaps platform packaging; Linux amd64 also checks its packaged candidate.", fmt.Sprintf("Active critical path: %.0fs. Complete timing: %t.", m.Active, m.ActiveComplete))
	lines = append(lines, rows...)
	return createIssue(g, repo, "Release timing exceeded: source "+m.SHA[:min(12, len(m.SHA))], strings.Join(lines, "\n"))
}

type hookReport struct {
	SHA            string   `json:"sha"`
	Elapsed        *float64 `json:"elapsed_seconds"`
	BudgetExceeded bool     `json:"budget_exceeded"`
	Incomplete     []string `json:"incomplete_checks"`
	Checks         []string `json:"checks"`
	RunURL         string   `json:"run_url"`
	ProofURL       string   `json:"proof_url"`
}
type hookResult struct {
	SHA        string       `json:"source_sha"`
	Elapsed    float64      `json:"elapsed_seconds"`
	Maximum    float64      `json:"maximum_seconds"`
	Goal       float64      `json:"goal_seconds"`
	Incomplete []string     `json:"incomplete_checks"`
	Violation  bool         `json:"violation"`
	Issue      *issueResult `json:"issue,omitempty"`
}

var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

func checkHookReport(g *github, repo string, h hookReport, p policy) (hookResult, error) {
	result := hookResult{}
	if h.Elapsed == nil || *h.Elapsed < 0 {
		return result, errors.New("hook elapsed_seconds must be a nonnegative number")
	}
	if !fullSHA.MatchString(h.SHA) {
		return result, errors.New("hook report must identify its full source SHA")
	}
	if p.HookMaximum <= 0 || p.HookGoal <= 0 || p.HookGoal > p.HookMaximum {
		return result, errors.New("hook goal and maximum must be positive and ordered")
	}
	if h.Incomplete == nil {
		return result, errors.New("hook incomplete_checks must be an array")
	}
	result = hookResult{SHA: h.SHA, Elapsed: *h.Elapsed, Maximum: p.HookMaximum, Goal: p.HookGoal, Incomplete: h.Incomplete, Violation: *h.Elapsed > p.HookMaximum || h.BudgetExceeded || len(h.Incomplete) > 0}
	if !result.Violation {
		return result, nil
	}
	marker := "<!-- hook-timing:" + h.SHA + " -->"
	existing, err := existingIssue(g, repo, marker)
	if err != nil || existing != nil {
		result.Issue = existing
		return result, err
	}
	runURL := h.RunURL
	if runURL == "" {
		runURL = h.ProofURL
	}
	if runURL == "" && os.Getenv("GITHUB_RUN_ID") != "" {
		runURL = "https://github.com/" + repo + "/actions/runs/" + os.Getenv("GITHUB_RUN_ID")
	}
	if runURL == "" {
		runURL = "local hook report; no uploaded measurement URL supplied"
	}
	incomplete, err := json.Marshal(h.Incomplete)
	if err != nil {
		return result, err
	}
	checks, err := json.Marshal(h.Checks)
	if err != nil {
		return result, err
	}
	body := strings.Join([]string{marker, fmt.Sprintf("Commit-hook timing or completion exceeded the %gs budget (%gs goal). Performance findings do not block shipping.", p.HookMaximum, p.HookGoal), fmt.Sprintf("Source: [%s](https://github.com/%s/commit/%s).", h.SHA, repo, h.SHA), fmt.Sprintf("Measured elapsed: **%.3fs**. Budget exceeded: %t.", *h.Elapsed, h.BudgetExceeded), "Incomplete checks: " + string(incomplete) + ".", "Checks: " + string(checks) + ".", "Measurement: " + runURL + "."}, "\n")
	result.Issue, err = createIssue(g, repo, "Commit hook timing exceeded: source "+h.SHA[:12], body)
	return result, err
}

type runIDs []int64

func (ids *runIDs) String() string { return fmt.Sprint([]int64(*ids)) }
func (ids *runIDs) Set(value string) error {
	id, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		*ids = append(*ids, id)
	}
	return err
}

type options struct {
	repo                      string
	ids                       runIDs
	hook, policy, now, report string
	scan, measure             bool
}

func parseOptions(args []string) (options, error) {
	var o options
	flags := flag.NewFlagSet("release-timing", flag.ContinueOnError)
	flags.StringVar(&o.repo, "repo", "tylergannon/gimbal", "GitHub owner/repository")
	flags.Var(&o.ids, "run-id", "Preparation run ID; may be repeated")
	flags.StringVar(&o.hook, "hook-report", "", "Commit-hook JSON measurement")
	flags.StringVar(&o.policy, "policy", "scripts/release-performance.json", "Timing policy JSON")
	flags.StringVar(&o.now, "now", "", "Observation time in RFC3339")
	flags.StringVar(&o.report, "report", "", "Write complete measurements as JSON")
	flags.BoolVar(&o.scan, "scan", false, "Check recent main preparations, including queued/failed runs")
	flags.BoolVar(&o.measure, "measure", false, "Measure without creating issues or applying adoption cutoff")
	if err := flags.Parse(args); err != nil {
		return o, err
	}
	if flags.NArg() != 0 {
		return o, errors.New("unexpected positional arguments")
	}
	if !o.scan && len(o.ids) == 0 && o.hook == "" {
		return o, errors.New("provide --scan, --run-id, or --hook-report")
	}
	return o, nil
}
func scan(g *github, o options, p policy, now string) report {
	output := report{Policy: p, Measurements: []measurement{}, Errors: []scanError{}}
	recordError := func(r run, err error) {
		output.Errors = append(output.Errors, scanError{RunID: r.ID, SHA: r.SHA, Error: err.Error()})
	}
	base := "repos/" + o.repo + "/actions"
	runs := []run{}
	if o.scan || len(o.ids) > 0 {
		values, err := collection[run](g, base+"/workflows/auto-release.yml/runs?per_page=100", "workflow_runs", p.EnforceAfter)
		runs = append(runs, values...)
		if err != nil {
			recordError(run{}, err)
		}
	}
	for _, id := range o.ids {
		r, err := get[run](g, fmt.Sprintf("%s/runs/%d", base, id))
		if err != nil {
			recordError(run{ID: id}, err)
		} else {
			runs = append(runs, r)
		}
	}
	oldest := now
	for _, r := range runs {
		if r.Created < oldest {
			oldest = r.Created
		}
	}
	releases := []run{}
	if len(runs) > 0 {
		values, err := collection[run](g, base+"/workflows/release.yml/runs?per_page=100", "workflow_runs", oldest)
		releases = values
		if err != nil {
			// A truncated release list cannot establish whether a source was delivered.
			recordError(run{}, err)
			return output
		}
	}
	selected := map[string]bool{}
	for _, r := range runs {
		if slices.Contains(o.ids, r.ID) {
			selected[r.SHA] = true
		}
	}
	slices.SortFunc(runs, func(a, b run) int { return strings.Compare(a.Created, b.Created) })
	seen := map[string]bool{}
	for _, r := range runs {
		if !mainPreparation(r) || seen[r.SHA] || (len(o.ids) > 0 && !o.scan && !selected[r.SHA]) {
			continue
		}
		seen[r.SHA] = true
		created, err := timestamp(r.Created)
		if err != nil {
			recordError(r, err)
			continue
		}
		if !o.measure && !selected[r.SHA] {
			cutoff, err := timestamp(p.EnforceAfter)
			if err != nil {
				recordError(r, err)
				continue
			}
			if created.Before(cutoff) {
				continue
			}
		}
		preparations := []run{}
		preparationIDs := map[int64]bool{}
		for _, candidate := range runs {
			if candidate.SHA == r.SHA && mainPreparation(candidate) && !preparationIDs[candidate.ID] {
				preparations = append(preparations, candidate)
				preparationIDs[candidate.ID] = true
			}
		}
		m, err := measure(g, o.repo, r, releases, p, now, preparations)
		if err != nil {
			recordError(r, err)
			continue
		}
		if !o.measure {
			m.Issue, err = ensureIssue(g, o.repo, m)
			if err != nil {
				recordError(r, err)
			}
		}
		output.Measurements = append(output.Measurements, m)
	}
	return output
}
func execute(args []string, g *github, stdout io.Writer) error {
	o, err := parseOptions(args)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(o.policy)
	if err != nil {
		return err
	}
	var p policy
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.Goal <= 0 || p.Goal > p.Maximum {
		return errors.New("release goal and maximum must be positive and ordered")
	}
	now := o.now
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if _, err := timestamp(now); err != nil {
		return err
	}
	output := scan(g, o, p, now)
	if o.hook != "" {
		data, err := os.ReadFile(o.hook)
		if err != nil {
			output.Errors = append(output.Errors, scanError{Error: err.Error()})
		} else if o.measure {
			var hook any
			if err := json.Unmarshal(data, &hook); err != nil {
				output.Errors = append(output.Errors, scanError{Error: err.Error()})
			} else {
				output.Hook = hook
			}
		} else {
			var hook hookReport
			if err := json.Unmarshal(data, &hook); err != nil {
				output.Errors = append(output.Errors, scanError{Error: err.Error()})
			} else {
				result, err := checkHookReport(g, o.repo, hook, p)
				if err != nil {
					output.Errors = append(output.Errors, scanError{Error: err.Error()})
				} else {
					output.Hook = result
				}
			}
		}
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if o.report != "" {
		if err := os.WriteFile(o.report, encoded, 0o644); err != nil {
			return err
		}
	}
	if _, err := stdout.Write(encoded); err != nil {
		return err
	}
	if len(output.Errors) > 0 {
		return fmt.Errorf("release timing scan completed with %d observation errors; see report", len(output.Errors))
	}
	return nil
}
func main() {
	if err := execute(os.Args[1:], newGitHub(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Release timing check failed:", err)
		os.Exit(1)
	}
}
