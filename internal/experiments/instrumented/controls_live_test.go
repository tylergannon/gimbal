package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/experiments/instrumented/fanout"
	"go.temporal.io/sdk/client"
)

func TestLiveSelectedBranchSteering(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_CONTROLS") != "1" {
		t.Skip("requires dedicated live workers and paid Haiku access")
	}
	c, err := client.Dial(client.Options{DataConverter: dataConverter(stateRoot())})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("steering-%d", time.Now().UnixMilli())
	input, err := prepareInput(stateRoot(), id, "Independent steerable file writes.")
	if err != nil {
		t.Fatal(err)
	}
	items := [2]fanout.Work{{File: "first.txt", Receipt: "first", Delay: 35}, {File: "second.txt", Receipt: "second", Delay: 35}}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, FanoutWorkflow, fanoutInput{Input: input, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("workflow %s", id)
	dir := filepath.Join(stateRoot(), environmentID(id), "workspace", ".gimbal", "runs")
	var runs []string
	for {
		runs, _ = filepath.Glob(filepath.Join(dir, "*"))
		if len(runs) == 1 {
			raw, readErr := os.ReadFile(filepath.Join(runs[0], "turns.json"))
			var turns []struct{ Ended int64 }
			if readErr == nil && json.Unmarshal(raw, &turns) == nil && len(turns) == 2 {
				active := 0
				for _, turn := range turns {
					if turn.Ended == 0 {
						active++
					}
				}
				if active == 2 {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	port, err := exec.CommandContext(ctx, "docker", "port", environmentID(id), "8080/tcp").Output()
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://" + strings.TrimSpace(string(port))
	t.Logf("live UI %s; Gimbal run %s", endpoint, filepath.Base(runs[0]))
	const message = "Continue only your assigned second.txt edit; retain receipt second. This message is for the second branch only."
	payload, _ := json.Marshal(map[string]string{"Run": filepath.Base(runs[0]), "Session": "workers.1/right.1/coder.1", "Message": message})
	for {
		request, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/control/steer", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var reply struct{ Landed bool }
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		err = json.Unmarshal(body, &reply)
		if err != nil || response.StatusCode != 200 {
			t.Fatalf("steer %d %+v %v: %s", response.StatusCode, reply, err, body)
		}
		if reply.Landed {
			break
		}
		// TurnStarted precedes native readiness. Retry only a definitive drop;
		// never retry ambiguous delivery or send again after acceptance.
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	var out [2]Report
	if err = run.Get(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Receipt != "first" || out[1].Receipt != "second" {
		t.Fatalf("misassigned results: %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(runs[0], "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	landed := 0
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var record struct {
			Session string
			Event   struct {
				Kind, Message, Target string
				Landed                bool
			}
		}
		if err = json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record.Event.Kind == "steer" && record.Event.Message == message {
			if record.Event.Target != "workers.1/right.1/coder.1" {
				t.Fatalf("wrong steering recipient %+v", record)
			}
			if record.Event.Landed {
				landed++
			}
		}
	}
	if landed != 1 {
		t.Fatalf("steer records=%d", landed)
	}
	replayRecorded(t, c, id, stateRoot())
	t.Log("selected branch steer landed exactly once; both results retained; replay passed")
}
