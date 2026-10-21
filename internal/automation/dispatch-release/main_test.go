package main

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDispatch(t *testing.T) {
	for _, test := range []struct {
		name, status, conclusion string
		post                     bool
		failure                  bool
		retry                    bool
	}{
		{"new candidate dispatched", "", "", true, false, false},
		{"active reused", "in_progress", "", false, false, false},
		{"success reused", "completed", "success", false, false, false},
		{"failed candidate reported without redispatch", "completed", "failure", false, true, false},
		{"cancelled candidate reported without redispatch", "completed", "cancelled", false, true, false},
		{"failed explicitly retried", "completed", "failure", true, false, true},
		{"active reused during manual recovery", "in_progress", "", false, false, true},
		{"success reused during manual recovery", "completed", "success", false, false, true},
		{"publication failure", "queued", "", false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := [][]string{}
			listed := 0
			d := dispatcher{now: time.Now, sleep: func(time.Duration) {}, retryFailed: test.retry, gh: func(args ...string) ([]byte, error) {
				calls = append(calls, args)
				if slices.Contains(args, "POST") {
					if !slices.Contains(args, "ref=v0.13.4") || !slices.Contains(args, "inputs[tag]=v0.13.4") {
						t.Fatal(args)
					}
					return nil, nil
				}
				r := releaseRun{ID: 12, SHA: "abc", Event: "workflow_dispatch", Status: test.status, Conclusion: test.conclusion, URL: "https://example.invalid/run"}
				if strings.Contains(args[1], "/runs?") {
					listed++
					if test.status == "" && listed == 1 {
						return []byte(`{"workflow_runs":[]}`), nil
					}
					if listed > 1 {
						r.ID = 13
						r.Status = "completed"
						r.Conclusion = "success"
					}
					return json.Marshal(map[string]any{"workflow_runs": []releaseRun{r}})
				}
				r.Status = "completed"
				r.Conclusion = "success"
				if test.failure {
					r.Conclusion = "failure"
				}
				return json.Marshal(r)
			}}
			err := d.wait("v0.13.4", "abc", "owner/repo", time.Minute)
			if (err != nil) != test.failure {
				t.Fatalf("error=%v", err)
			}
			if test.status == "completed" && test.conclusion != "success" && !test.retry {
				if len(calls) != 1 || !strings.Contains(err.Error(), "https://example.invalid/run") || !strings.Contains(err.Error(), "inspect the failed job") {
					t.Fatalf("failed candidate was not reported directly: calls=%v error=%v", calls, err)
				}
			}
			posts := 0
			for _, call := range calls {
				if slices.Contains(call, "POST") {
					posts++
				}
			}
			if (posts == 1) != test.post {
				t.Fatal(calls)
			}
			if test.status == "completed" && test.conclusion == "success" && len(calls) != 1 {
				t.Fatal(calls)
			}
		})
	}
}
func TestDispatchIgnoresOtherSourcesAndOldFailedRunAfterRetry(t *testing.T) {
	now := time.Now()
	posts := 0
	lists := 0
	d := dispatcher{now: func() time.Time { return now }, sleep: func(value time.Duration) { now = now.Add(value) }, retryFailed: true, gh: func(args ...string) ([]byte, error) {
		if slices.Contains(args, "POST") {
			posts++
			return nil, nil
		}
		lists++
		runs := []releaseRun{{ID: 20, SHA: "other", Event: "workflow_dispatch", Status: "completed", Conclusion: "success"}, {ID: 12, SHA: "abc", Event: "workflow_dispatch", Status: "completed", Conclusion: "failure"}}
		if lists > 2 {
			runs = append(runs, releaseRun{ID: 13, SHA: "abc", Event: "workflow_dispatch", Status: "completed", Conclusion: "success"})
		}
		return json.Marshal(map[string]any{"workflow_runs": runs})
	}}
	if err := d.wait("v1.0.0", "abc", "owner/repo", time.Minute); err != nil {
		t.Fatal(err)
	}
	if posts != 1 || lists != 3 {
		t.Fatalf("posts=%d lists=%d", posts, lists)
	}
}
func TestDispatchDeadlineAndReadError(t *testing.T) {
	for _, readError := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "API error"}[readError], func(t *testing.T) {
			now := time.Now()
			d := dispatcher{now: func() time.Time { return now }, sleep: func(value time.Duration) { now = now.Add(value) }, gh: func(args ...string) ([]byte, error) {
				if readError {
					return nil, errors.New("API unavailable")
				}
				if slices.Contains(args, "POST") {
					return nil, nil
				}
				return []byte(`{"workflow_runs":[]}`), nil
			}}
			err := d.wait("v1.0.0", "abc", "owner/repo", time.Second)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
