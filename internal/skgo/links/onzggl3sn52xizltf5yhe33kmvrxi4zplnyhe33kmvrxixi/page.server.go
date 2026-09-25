// Package routes serves the project's runs list.
package routes

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
)

// RunItem is the one list row and the presentation text derived from its
// observation. Keeping the text beside the row avoids a second map-shaped
// state model at the load boundary.
type RunItem struct {
	Run          observation.RunRow `json:"run"`
	Summary      string             `json:"summary"`
	Elapsed      string             `json:"elapsed"`
	ActivityAt   int64              `json:"activity_at"`
	Cost         string             `json:"cost"`
	SessionCount int                `json:"session_count"`
	TurnCount    int                `json:"turn_count"`
	Instruction  string             `json:"instruction"`
}

// RunsData is the complete server snapshot needed by RunsList.
type RunsData struct {
	Items     []RunItem                  `json:"items"`
	Attention []observation.InterviewRow `json:"attention"`
	Now       int64                      `json:"now"`
}

func load(ctx context.Context) (RunsData, error) {
	event := skgo.EventFrom(ctx)
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	registry := observation.FromContext(ctx)
	projectDir := host.ProjectDir(ctx)
	if registry == nil || projectDir == "" {
		return RunsData{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no project observation in its context, so its runs cannot be listed.")
	}

	now := time.Now().UnixMilli()
	entries, err := os.ReadDir(filepath.Join(projectDir, "runs"))
	if os.IsNotExist(err) {
		return RunsData{Items: []RunItem{}, Attention: []observation.InterviewRow{}, Now: now}, nil
	}
	if err != nil {
		return RunsData{}, err
	}

	data := RunsData{Items: []RunItem{}, Attention: []observation.InterviewRow{}, Now: now}
	disk := observation.NewRegistry(projectDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var snapshot observation.RunSnapshot
		if store, live := registry.Live(entry.Name()); live {
			snapshot = store.Snapshot()
		} else {
			snapshot, err = disk.Snapshot(entry.Name())
		}
		if err != nil {
			return RunsData{}, fmt.Errorf("read run %s: %w", entry.Name(), err)
		}
		pending := pendingInterviews(snapshot)
		data.Attention = append(data.Attention, pending...)
		data.Items = append(data.Items, RunItem{
			Run:          snapshot.Run,
			Summary:      summarizeRun(snapshot, pending),
			Elapsed:      formatElapsed(snapshot.Run, now),
			ActivityAt:   latestActivity(snapshot),
			Cost:         formatCost(snapshot),
			SessionCount: len(snapshot.Sessions),
			TurnCount:    len(snapshot.Turns),
			Instruction:  latestInstruction(snapshot),
		})
	}

	slices.SortStableFunc(data.Items, func(left, right RunItem) int {
		leftRunning := left.Run.Status == observation.StatusRunning
		rightRunning := right.Run.Status == observation.StatusRunning
		if leftRunning != rightRunning {
			if leftRunning {
				return -1
			}
			return 1
		}
		return -compareInt64(left.Run.Started, right.Run.Started)
	})
	slices.SortStableFunc(data.Attention, func(left, right observation.InterviewRow) int {
		return -compareInt64(left.Asked, right.Asked)
	})
	return data, nil
}

func latestActivity(snapshot observation.RunSnapshot) int64 {
	latest := max(snapshot.Run.Started, snapshot.Run.Ended)
	for _, scope := range snapshot.Scopes {
		latest = max(latest, scope.Began, scope.Ended)
	}
	for _, session := range snapshot.Sessions {
		latest = max(latest, session.Created)
	}
	for _, interview := range snapshot.Interviews {
		latest = max(latest, interview.Asked, interview.Answered)
	}
	for _, turn := range snapshot.Turns {
		latest = max(latest, turn.Started, turn.Ended)
	}
	for _, calls := range snapshot.ModelCalls {
		for _, call := range calls {
			latest = max(latest, call.Started, call.Ended)
		}
	}
	for _, command := range snapshot.Commands {
		latest = max(latest, command.Started, command.Ended)
	}
	return latest
}

func latestInstruction(snapshot observation.RunSnapshot) string {
	var latest observation.TurnRow
	for _, turn := range snapshot.Turns {
		if turn.Started > latest.Started || turn.Started == latest.Started && turn.ID > latest.ID {
			latest = turn
		}
	}
	return latest.Prompt
}

func formatCost(snapshot observation.RunSnapshot) string {
	total, ok := snapshot.Totals.Scopes[""]
	if !ok {
		return "—"
	}
	cost, ok := observation.TotalCost(total)
	if !ok {
		return "—"
	}
	if cost > 0 && cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

func pendingInterviews(snapshot observation.RunSnapshot) []observation.InterviewRow {
	pending := make([]observation.InterviewRow, 0, len(snapshot.Interviews))
	for _, interview := range snapshot.Interviews {
		if interview.Status == observation.InterviewStatusPending {
			pending = append(pending, interview)
		}
	}
	slices.SortFunc(pending, func(left, right observation.InterviewRow) int {
		return compareInt64(left.Asked, right.Asked)
	})
	return pending
}

func summarizeRun(snapshot observation.RunSnapshot, pending []observation.InterviewRow) string {
	if len(pending) > 0 {
		scopes := make([]string, 0, len(pending))
		for _, interview := range pending {
			if !slices.Contains(scopes, interview.Scope) {
				scopes = append(scopes, interview.Scope)
			}
		}
		return fmt.Sprintf("%d %s waiting · %s", len(pending), plural(len(pending), "question", "questions"), strings.Join(scopes, " · "))
	}
	if snapshot.Run.Error != "" {
		return snapshot.Run.Error
	}
	if snapshot.Run.Status == observation.StatusCancelled {
		return "Cancelled"
	}
	if snapshot.Run.Status == observation.StatusCompleted {
		return "Completed"
	}

	var current observation.TurnRow
	for _, turn := range snapshot.Turns {
		if turn.Ended == 0 && turn.Started >= current.Started {
			current = turn
		}
	}
	if current.ID == "" {
		return "In progress"
	}
	name := current.Session
	if session, ok := snapshot.Sessions[current.Session]; ok {
		name = session.Name
	}
	if current.Scope == "" {
		return name + " / " + current.ID
	}
	return name + " / " + current.ID + " in " + current.Scope
}

func formatElapsed(run observation.RunRow, now int64) string {
	if run.Started == 0 {
		return "—"
	}
	end := run.Ended
	if end == 0 {
		end = now
	}
	seconds := max(int64(0), (end-run.Started)/1000)
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds%60)
	}
	return fmt.Sprintf("%dh %02dm", minutes/60, minutes%60)
}

func compareInt64(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

var _ = skgo.Load(load)
