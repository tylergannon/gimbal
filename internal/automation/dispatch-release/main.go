// Dispatch release reuses a tagged build and waits for its publication gate.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type releaseRun struct {
	ID         int64  `json:"id"`
	SHA        string `json:"head_sha"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"html_url"`
}
type dispatcher struct {
	gh    func(...string) ([]byte, error)
	sleep func(time.Duration)
	now   func() time.Time
	// Pushes must not retry a failed candidate when its release commit reaches main.
	retryFailed bool
}

func latestRun(runs []releaseRun, sha string) *releaseRun {
	var result *releaseRun
	for _, r := range runs {
		if r.SHA == sha && (r.Event == "workflow_dispatch" || r.Event == "push") && (result == nil || r.ID > result.ID) {
			copy := r
			result = &copy
		}
	}
	return result
}
func (d dispatcher) list(endpoint, sha string) (*releaseRun, error) {
	raw, err := d.gh("api", endpoint+"/runs?per_page=100")
	if err != nil {
		return nil, err
	}
	var result struct {
		Runs []releaseRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return latestRun(result.Runs, sha), nil
}
func (d dispatcher) wait(tag, sha, repository string, timeout time.Duration) error {
	endpoint := "repos/" + repository + "/actions/workflows/release.yml"
	deadline := d.now().Add(timeout)
	run, err := d.list(endpoint, sha)
	if err != nil {
		return err
	}
	if run == nil || (run.Status == "completed" && run.Conclusion != "success" && d.retryFailed) {
		var previous int64
		if run != nil {
			previous = run.ID
		}
		if _, err := d.gh("api", "--method", "POST", endpoint+"/dispatches", "-f", "ref="+tag, "-f", "inputs[tag]="+tag); err != nil {
			return err
		}
		run = nil
		for run == nil {
			if !d.now().Before(deadline) {
				return fmt.Errorf("release dispatch did not appear before timeout")
			}
			d.sleep(15 * time.Second)
			candidate, err := d.list(endpoint, sha)
			if err != nil {
				return err
			}
			if candidate != nil && candidate.ID > previous {
				run = candidate
			}
		}
	}
	fmt.Printf("Release: %s\n", run.URL)
	for run.Status != "completed" {
		if !d.now().Before(deadline) {
			return fmt.Errorf("release still running: %s", run.URL)
		}
		d.sleep(30 * time.Second)
		raw, err := d.gh("api", fmt.Sprintf("repos/%s/actions/runs/%d", repository, run.ID))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, run); err != nil {
			return err
		}
	}
	if run.Conclusion != "success" {
		return fmt.Errorf("release %s: %s; inspect the failed job, then rerun that tagged Release job or manually dispatch Automatic release to retry", run.Conclusion, run.URL)
	}
	fmt.Printf("Published %s: %s\n", tag, run.URL)
	return nil
}
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: dispatch-release TAG SHA OWNER/REPOSITORY")
		os.Exit(1)
	}
	d := dispatcher{now: time.Now, sleep: time.Sleep, retryFailed: os.Getenv("RETRY_FAILED_RELEASE") == "true", gh: func(args ...string) ([]byte, error) {
		command := exec.Command("gh", args...)
		command.Stderr = os.Stderr
		return command.Output()
	}}
	if err := d.wait(os.Args[1], os.Args[2], os.Args[3], 2*time.Hour); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
