package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
)

type pullRequest struct {
	id     int
	deps   []int
	checks string
}

type queueResult struct {
	Waves   [][]int `json:"waves"`
	Blocked []int   `json:"blocked"`
}

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runCLI(args []string, in io.Reader, out io.Writer, errOut io.Writer) int {
	parallel, err := parseParallelArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 2
	}
	return runWithParallel(in, out, errOut, parallel)
}

func run(in io.Reader, out io.Writer, errOut io.Writer) int {
	return runWithParallel(in, out, errOut, 0)
}

func runWithParallel(in io.Reader, out io.Writer, errOut io.Writer, parallel int) int {
	result, err := planWithParallel(in, parallel)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 2
	}

	if err := json.NewEncoder(out).Encode(result); err != nil {
		_, _ = fmt.Fprintln(errOut, "writing result: "+err.Error())
		return 1
	}
	return 0
}

func parseParallelArgs(args []string) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}
	if len(args) != 2 || args[0] != "--parallel" {
		return 0, errors.New("usage: mergequeue [--parallel N]")
	}
	parallel, err := strconv.Atoi(args[1])
	if err != nil || parallel <= 0 {
		return 0, fmt.Errorf("invalid --parallel value %q: N must be a positive integer", args[1])
	}
	return parallel, nil
}

func plan(in io.Reader) (queueResult, error) {
	return planWithParallel(in, 0)
}

func planWithParallel(in io.Reader, parallel int) (queueResult, error) {
	prs, err := decodeQueue(in)
	if err != nil {
		return queueResult{}, err
	}

	byID := make(map[int]pullRequest, len(prs))
	for _, pr := range prs {
		if pr.id <= 0 {
			return queueResult{}, fmt.Errorf("invalid PR id %d: IDs must be positive", pr.id)
		}
		if _, exists := byID[pr.id]; exists {
			return queueResult{}, fmt.Errorf("duplicate PR id %d", pr.id)
		}
		if pr.checks != "pass" && pr.checks != "fail" && pr.checks != "pending" {
			return queueResult{}, fmt.Errorf("PR %d has invalid checks status %q", pr.id, pr.checks)
		}
		byID[pr.id] = pr
	}

	for _, pr := range prs {
		for _, dep := range pr.deps {
			if _, exists := byID[dep]; !exists {
				return queueResult{}, fmt.Errorf("PR %d depends on missing PR %d", pr.id, dep)
			}
		}
	}

	if cycleID, ok := findCycle(byID); ok {
		return queueResult{}, fmt.Errorf("dependency cycle detected involving PR %d", cycleID)
	}

	blocked := make(map[int]bool, len(byID))
	var isBlocked func(int) bool
	isBlocked = func(id int) bool {
		if known, ok := blocked[id]; ok {
			return known
		}
		pr := byID[id]
		blocked[id] = pr.checks != "pass"
		if !blocked[id] {
			for _, dep := range pr.deps {
				if isBlocked(dep) {
					blocked[id] = true
					break
				}
			}
		}
		return blocked[id]
	}

	ids := sortedIDs(byID)
	blockedIDs := make([]int, 0)
	for _, id := range ids {
		if isBlocked(id) {
			blockedIDs = append(blockedIDs, id)
		}
	}

	scheduled := make(map[int]bool, len(byID))
	waves := make([][]int, 0)
	for {
		wave := make([]int, 0)
		for _, id := range ids {
			if blocked[id] || scheduled[id] {
				continue
			}
			ready := true
			for _, dep := range byID[id].deps {
				if !scheduled[dep] {
					ready = false
					break
				}
			}
			if ready {
				wave = append(wave, id)
			}
		}
		if len(wave) == 0 {
			break
		}
		if parallel > 0 && len(wave) > parallel {
			wave = wave[:parallel]
		}
		for _, id := range wave {
			scheduled[id] = true
		}
		waves = append(waves, wave)
	}

	return queueResult{Waves: waves, Blocked: blockedIDs}, nil
}

func decodeQueue(in io.Reader) ([]pullRequest, error) {
	decoder := json.NewDecoder(in)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %w", err)
	}
	if first := bytes.TrimSpace(raw); len(first) == 0 || first[0] != '{' {
		return nil, errors.New("invalid input: top-level value must be a JSON object")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	prsRaw, ok := fields["prs"]
	if !ok {
		return nil, errors.New("invalid input: missing prs array")
	}
	if first := bytes.TrimSpace(prsRaw); len(first) == 0 || first[0] != '[' {
		return nil, errors.New("invalid input: prs must be an array")
	}

	var rawPRs []json.RawMessage
	if err := json.Unmarshal(prsRaw, &rawPRs); err != nil {
		return nil, fmt.Errorf("invalid prs array: %w", err)
	}
	prs := make([]pullRequest, 0, len(rawPRs))
	for index, rawPR := range rawPRs {
		pr, err := decodePR(rawPR)
		if err != nil {
			return nil, fmt.Errorf("invalid PR at index %d: %w", index, err)
		}
		prs = append(prs, pr)
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid input: extra JSON value after queue")
		}
		return nil, fmt.Errorf("invalid input after queue: %w", err)
	}
	return prs, nil
}

func decodePR(raw json.RawMessage) (pullRequest, error) {
	var fields map[string]json.RawMessage
	if first := bytes.TrimSpace(raw); len(first) == 0 || first[0] != '{' {
		return pullRequest{}, errors.New("PR must be a JSON object")
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return pullRequest{}, fmt.Errorf("PR must be a JSON object: %w", err)
	}

	idRaw, ok := fields["id"]
	if !ok {
		return pullRequest{}, errors.New("missing id")
	}
	var id int
	if err := json.Unmarshal(idRaw, &id); err != nil {
		return pullRequest{}, fmt.Errorf("id must be an integer: %w", err)
	}

	depsRaw, ok := fields["deps"]
	if !ok {
		return pullRequest{}, errors.New("missing deps array")
	}
	if first := bytes.TrimSpace(depsRaw); len(first) == 0 || first[0] != '[' {
		return pullRequest{}, errors.New("deps must be an array")
	}
	var deps []int
	if err := json.Unmarshal(depsRaw, &deps); err != nil {
		return pullRequest{}, fmt.Errorf("deps must contain integers: %w", err)
	}
	if deps == nil {
		deps = []int{}
	}

	checksRaw, ok := fields["checks"]
	if !ok {
		return pullRequest{}, errors.New("missing checks status")
	}
	var checks string
	if err := json.Unmarshal(checksRaw, &checks); err != nil {
		return pullRequest{}, fmt.Errorf("checks must be a string: %w", err)
	}

	return pullRequest{id: id, deps: deps, checks: checks}, nil
}

func findCycle(byID map[int]pullRequest) (int, bool) {
	state := make(map[int]uint8, len(byID))
	ids := sortedIDs(byID)
	var visit func(int) (int, bool)
	visit = func(id int) (int, bool) {
		switch state[id] {
		case 1:
			return id, true
		case 2:
			return 0, false
		}
		state[id] = 1
		for _, dep := range byID[id].deps {
			if cycleID, ok := visit(dep); ok {
				return cycleID, true
			}
		}
		state[id] = 2
		return 0, false
	}

	for _, id := range ids {
		if cycleID, ok := visit(id); ok {
			return cycleID, true
		}
	}
	return 0, false
}

func sortedIDs(byID map[int]pullRequest) []int {
	ids := make([]int, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}
