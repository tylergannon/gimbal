package indexfeedback

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
)

// Locate this evaluation by its unique recorded value, never by "latest run".
// This reads only the opening records, before any agent output is recorded.
func findRun(runs, id string) (string, error) {
	dirs, err := filepath.Glob(filepath.Join(runs, "*.index-feedback"))
	if err != nil {
		return "", err
	}
	for _, dir := range dirs {
		f, err := os.Open(filepath.Join(dir, "run.jsonl"))
		if err != nil {
			continue
		}
		decoder := json.NewDecoder(f)
		found := false
		for range 12 {
			var record gimble.LifecycleRecord
			if decoder.Decode(&record) != nil {
				break
			}
			if value, ok := record.Event.(gimble.ValueSet); ok && value.Key == "evaluation id" && value.Value.Present {
				var recorded string
				if json.Unmarshal([]byte(value.Value.Value), &recorded) == nil && recorded == id {
					found = true
				}
				break
			}
		}
		_ = f.Close()
		if found {
			return dir, nil
		}
	}
	return "", fmt.Errorf("evaluation run record not found")
}

func measureRetrieval(dir, scope string) string {
	f, err := os.Open(filepath.Join(dir, "run.jsonl"))
	if err != nil {
		return "Accounting unavailable: " + err.Error()
	}
	defer func(f *os.File) { _ = f.Close() }(f)
	decoder := json.NewDecoder(f)
	session := ""
	var usage []gimble.ModelUsage
	for {
		var record gimble.LifecycleRecord
		if err := decoder.Decode(&record); err != nil {
			if err != io.EOF {
				return "Accounting unavailable: " + err.Error()
			}
			break
		}
		if record.Scope != scope {
			continue
		}
		switch event := record.Event.(type) {
		case gimble.SessionCreated:
			if event.Name == "index-retrieval" {
				session = record.Session.Value
			}
		case gimble.TurnEnded:
			usage = append(usage, event.Usage...)
		}
	}
	if session == "" {
		return "Accounting unavailable: retrieval session not recorded"
	}
	f, err = os.Open(filepath.Join(dir, "sessions", session+".jsonl"))
	if err != nil {
		return "Accounting unavailable: " + err.Error()
	}
	defer func(f *os.File) { _ = f.Close() }(f)
	counts, err := countTools(f)
	if err != nil {
		return "Accounting incomplete: " + err.Error()
	}
	tokens := "Provider token accounting unavailable."
	if len(usage) != 0 {
		encoded, _ := json.Marshal(usage)
		tokens = "Recorded provider usage (separate from returned text): " + string(encoded)
	}
	return fmt.Sprintf("Observed retrieval tool calls: %d; failed: %d; unfinished: %d; recorded returned text: %d bytes.\n\n%s\n\nCounts deduplicate tool identities and exclude submit_result bookkeeping. Returned bytes count recorded text/error messages, not physical file reads; harness truncation and unreported output may reduce them. Provider-unreported token fields appear as zero.", counts.calls, counts.failed, counts.unfinished, counts.bytes, tokens)
}

type toolCounts struct{ calls, failed, unfinished, bytes int }

func countTools(r io.Reader) (toolCounts, error) {
	type tool struct {
		name                 string
		called, done, failed bool
		bytes                int
	}
	tools := map[string]*tool{}
	decoder := json.NewDecoder(r)
	for {
		var record gimble.AgentRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			return toolCounts{}, err
		}
		if !strings.HasPrefix(record.Event.Type, "session.tool.") {
			continue
		}
		var data struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Executed *bool  `json:"executed"`
			Content  []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(record.Event.Data, &data); err != nil {
			return toolCounts{}, err
		}
		if data.ID == "" {
			continue
		}
		key := record.Turn + "/" + data.ID
		if tools[key] == nil {
			tools[key] = &tool{}
		}
		t := tools[key]
		if data.Name != "" {
			t.name = data.Name
		}
		switch record.Event.Type {
		case "session.tool.called":
			t.called = data.Executed == nil || *data.Executed
		case "session.tool.success", "session.tool.failed":
			t.called = data.Executed == nil || *data.Executed
			t.done = true
			t.failed = record.Event.Type == "session.tool.failed"
			t.bytes = len(data.Error.Message)
			for _, content := range data.Content {
				if content.Type == "text" {
					t.bytes += len(content.Text)
				}
			}
		}
	}
	var counts toolCounts
	for _, tool := range tools {
		if !tool.called || tool.name == "submit_result" {
			continue
		}
		counts.calls++
		counts.bytes += tool.bytes
		if tool.failed {
			counts.failed++
		}
		if !tool.done {
			counts.unfinished++
		}
	}
	return counts, nil
}
