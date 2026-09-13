package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunEmptyQueueUsesEmptyArrays(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(strings.NewReader(`{"prs":[]}`), &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d, stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "{\"waves\":[],\"blocked\":[]}\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPlanDiamondAndInputOrder(t *testing.T) {
	inputs := []string{
		`{"prs":[{"id":4,"deps":[2,3],"checks":"pass"},{"id":2,"deps":[1],"checks":"pass"},{"id":1,"deps":[],"checks":"pass"},{"id":3,"deps":[1],"checks":"pass"}]}`,
		`{"prs":[{"id":1,"deps":[],"checks":"pass"},{"id":3,"deps":[1],"checks":"pass"},{"id":2,"deps":[1],"checks":"pass"},{"id":4,"deps":[2,3],"checks":"pass"}]}`,
	}
	want := `{"waves":[[1],[2,3],[4]],"blocked":[]}`
	for _, input := range inputs {
		var stdout, stderr bytes.Buffer
		if code := run(strings.NewReader(input), &stdout, &stderr); code != 0 {
			t.Fatalf("run exit code = %d, stderr = %q", code, stderr.String())
		}
		if got := strings.TrimSpace(stdout.String()); got != want {
			t.Errorf("stdout = %s, want %s", got, want)
		}
	}
}

func TestParallelLimitsReadyPRsAndPreservesIDOrder(t *testing.T) {
	input := `{"prs":[{"id":5,"deps":[1],"checks":"pass"},{"id":4,"deps":[],"checks":"pass"},{"id":3,"deps":[2],"checks":"pass"},{"id":2,"deps":[],"checks":"pass"},{"id":1,"deps":[],"checks":"pass"}]}`
	var stdout, stderr bytes.Buffer
	if code := runWithParallel(strings.NewReader(input), &stdout, &stderr, 2); code != 0 {
		t.Fatalf("run exit code = %d, stderr = %q", code, stderr.String())
	}
	want := `{"waves":[[1,2],[3,4],[5]],"blocked":[]}`
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("stdout = %s, want %s", got, want)
	}
}

func TestParallelDefaultsToUnlimited(t *testing.T) {
	parallel, err := parseParallelArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if parallel != 0 {
		t.Fatalf("parallel = %d, want unlimited sentinel 0", parallel)
	}
}

func TestInvalidParallelArgumentsExitTwoWithDiagnosticAndNoOutput(t *testing.T) {
	tests := [][]string{
		{"--parallel"},
		{"--parallel", "0"},
		{"--parallel", "-1"},
		{"--parallel", "cat"},
	}
	input := strings.NewReader(`{"prs":[]}`)
	for _, args := range tests {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runCLI(args, input, &stdout, &stderr); code != 2 {
				t.Fatalf("run exit code = %d, want 2; stderr = %q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if strings.TrimSpace(stderr.String()) == "" {
				t.Fatal("stderr is empty, want diagnostic")
			}
		})
	}
}

func TestPlanBlocksTransitively(t *testing.T) {
	input := `{"prs":[{"id":6,"deps":[5],"checks":"pass"},{"id":1,"deps":[],"checks":"fail"},{"id":5,"deps":[2],"checks":"pass"},{"id":3,"deps":[1],"checks":"pass"},{"id":4,"deps":[],"checks":"pass"},{"id":2,"deps":[1],"checks":"pass"}]}`
	var stdout, stderr bytes.Buffer
	if code := run(strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d, stderr = %q", code, stderr.String())
	}
	want := `{"waves":[[4]],"blocked":[1,2,3,5,6]}`
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("stdout = %s, want %s", got, want)
	}
}

func TestInvalidInputExitsTwoWithDiagnosticAndNoOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"duplicate IDs", `{"prs":[{"id":1,"deps":[],"checks":"pass"},{"id":1,"deps":[],"checks":"pass"}]}`},
		{"missing dependency", `{"prs":[{"id":1,"deps":[2],"checks":"pass"}]}`},
		{"cycle", `{"prs":[{"id":1,"deps":[2],"checks":"pass"},{"id":2,"deps":[1],"checks":"pass"}]}`},
		{"cycle through failed PR", `{"prs":[{"id":1,"deps":[2],"checks":"fail"},{"id":2,"deps":[1],"checks":"pending"}]}`},
		{"zero ID", `{"prs":[{"id":0,"deps":[],"checks":"pass"}]}`},
		{"negative ID", `{"prs":[{"id":-1,"deps":[],"checks":"pass"}]}`},
		{"bad status", `{"prs":[{"id":1,"deps":[],"checks":"unknown"}]}`},
		{"malformed JSON", `{"prs":`},
		{"trailing JSON", `{"prs":[]} {"prs":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(strings.NewReader(tt.input), &stdout, &stderr); code != 2 {
				t.Fatalf("run exit code = %d, want 2; stderr = %q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if strings.TrimSpace(stderr.String()) == "" {
				t.Fatal("stderr is empty, want diagnostic")
			}
		})
	}
}

func TestPlanOutputIsJSON(t *testing.T) {
	result, err := plan(strings.NewReader(`{"prs":[{"id":2,"deps":[],"checks":"pending"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"waves":[],"blocked":[2]}`; got != want {
		t.Fatalf("marshaled result = %s, want %s", got, want)
	}
}
