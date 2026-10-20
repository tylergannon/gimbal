package conversation

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/sessionstate"
)

// humanInput marks where Interactive puts the person's message in a turn's
// prompt. Everything before it is the conversation purpose.
const humanInput = "\n\nHuman input:\n"

// projectTranscript rebuilds item.State from the conversation run's recorded
// turns. The run's journal already holds every native session event; each turn's
// projection is the same one the run page shows. This re-keys it for the chat:
// the person's own message replaces the wrapped prompt, and every row carries
// the request it answers. It reports whether the visible state changed.
//
// A turn belongs to the request whose input the interactive consumed to start
// it: the interactive record lists consumed inputs in the order it took them,
// and a prompt starts a turn when it is consumed. A steer never starts one; the
// session journals it as the inbox item turn/steer/<request>, which names the
// request it delivered. A prompt that was stopped before it started is never
// consumed, so a later prompt with the same words cannot be mistaken for it.
// When the run is not available, a transcript already shown is kept.
func projectTranscript(item *Conversation, run observation.RunSnapshot, available bool) (bool, error) {
	if !available && showsTurns(item.State) {
		return false, nil
	}
	before, _ := json.Marshal([]any{item.State, item.Requests})
	turns := make([]observation.TurnRow, 0, len(run.Turns))
	for _, turn := range run.Turns {
		turns = append(turns, turn)
	}
	slices.SortFunc(turns, func(a, b observation.TurnRow) int {
		if a.Started != b.Started {
			return int(a.Started - b.Started)
		}
		return strings.Compare(a.ID, b.ID)
	})

	byID := map[string]int{}
	for index := range item.Requests {
		request := &item.Requests[index]
		if request.Kind == "prompt" {
			byID[request.ID] = index
			if request.Delivery == "turn" || request.Delivery == "steer" {
				request.Delivery = "durable"
			}
		}
	}
	// steered maps each steered request to the turn that took it.
	steered := map[string]string{}
	for _, turn := range turns {
		for _, rows := range run.Transcripts[turn.ID].Snapshot.State.Message {
			for _, row := range rows {
				if id, _ := row.Get("id").(string); strings.HasPrefix(id, turn.ID+"/steer/") {
					steered[strings.TrimPrefix(id, turn.ID+"/steer/")] = turn.ID
				}
			}
		}
	}
	var starters []int
	for _, row := range slices.SortedFunc(maps.Values(run.Interactions), func(a, b observation.InteractiveRow) int {
		return cmp.Or(cmp.Compare(a.Ordinal, b.Ordinal), strings.Compare(a.QuestionID, b.QuestionID))
	}) {
		for _, id := range row.Consumed {
			if index, ok := byID[id]; ok && steered[id] == "" && !slices.Contains(starters, index) {
				starters = append(starters, index)
			}
		}
	}

	matched := map[string]bool{}
	var messages []*sessionstate.Obj
	open, paired := false, -1
	for _, turn := range turns {
		request := requestForTurn(item, starters, matched, turn)
		if request >= 0 {
			matched[item.Requests[request].ID] = true
			item.Requests[request].Delivery = "turn"
		}
		open = turn.Ended == 0 && !turn.Unresolved
		paired = request
		if request >= 0 && !open && (item.Requests[request].Status == "accepted" || item.Requests[request].Status == "running") {
			item.Requests[request].Status = "completed"
		}
		state := run.Transcripts[turn.ID].Snapshot.State
		for _, key := range slices.Sorted(maps.Keys(state.Message)) {
			for _, row := range state.Message[key] {
				if row = turnMessage(item, request, turn, row); row != nil {
					messages = append(messages, row)
				}
			}
		}
	}
	// A steered prompt is part of the turn that took it, not a queued prompt.
	for id, turnID := range steered {
		index, ok := byID[id]
		if !ok {
			continue
		}
		request := &item.Requests[index]
		request.Delivery = "steer"
		if request.Status == "accepted" || request.Status == "running" {
			request.Status = "running"
			if run.Turns[turnID].Ended != 0 || run.Turns[turnID].Unresolved {
				request.Status = "completed"
			}
		}
	}
	if open && paired >= 0 && item.Status == StatusWorking {
		item.ActiveRequest = item.Requests[paired].ID
		item.Requests[paired].Status = "running"
	}
	// A prompt the workflow has accepted but not yet started is the current
	// turn. It shows at once, ahead of the harness's first event.
	if !open {
		for index := range item.Requests {
			request := &item.Requests[index]
			if request.Kind != "prompt" || matched[request.ID] || request.Delivery == "steer" || (request.Status != "accepted" && request.Status != "running") {
				continue
			}
			request.Delivery = "turn"
			if item.Status == StatusWorking {
				item.ActiveRequest, request.Status = request.ID, "running"
			}
			row := sessionstate.NewObj()
			row.Set("id", request.ID+"/prompt")
			row.Set("type", "user")
			row.Set("text", request.Message)
			row.Set("requestID", request.ID)
			row.Set("time", timeCreated(request.Created))
			messages = append(messages, row)
			break
		}
	}

	state := sessionstate.NewProjectionState()
	state.Message[item.ID] = messages
	raw, err := json.Marshal(state)
	if err != nil {
		return false, err
	}
	item.State = gimbal.JSONText(raw)
	after, _ := json.Marshal([]any{item.State, item.Requests})
	return string(before) != string(after), nil
}

// showsTurns reports whether a saved projection holds more than the prompts
// admitted before the run opened.
func showsTurns(state gimbal.JSONText) bool {
	var saved sessionstate.ProjectionState
	if json.Unmarshal([]byte(state), &saved) != nil {
		return false
	}
	for _, rows := range saved.Message {
		for _, row := range rows {
			if row.Get("type") != "user" {
				return true
			}
		}
	}
	return false
}

func timeCreated(created int64) *sessionstate.Obj {
	out := sessionstate.NewObj()
	out.Set("created", float64(created))
	return out
}

// requestForTurn pairs a turn with the next request the interactive consumed to
// start a turn, in the order it consumed them. The turn's prompt must carry that
// request's message; a turn whose prompt matches none is left unpaired.
func requestForTurn(item *Conversation, starters []int, matched map[string]bool, turn observation.TurnRow) int {
	_, human, found := strings.Cut(turn.Prompt, humanInput)
	if !found {
		return -1
	}
	for _, index := range starters {
		request := item.Requests[index]
		if matched[request.ID] {
			continue
		}
		if strings.HasPrefix(human, request.Message+"\n\nRespond with") || (request.FileID != "" && strings.HasPrefix(human, request.Message+"\n\nRead the attached file ")) {
			return index
		}
	}
	return -1
}

// turnMessage re-keys one projected row for the chat, or drops it. request is
// the index of the turn's request, or -1.
func turnMessage(item *Conversation, request int, turn observation.TurnRow, row *sessionstate.Obj) *sessionstate.Obj {
	row = row.Clone()
	id, _ := row.Get("id").(string)
	if request < 0 {
		// The wrapped prompt carries the conversation purpose, not the person's words.
		if id == turn.ID+"/prompt" {
			return nil
		}
		return row
	}
	owner := item.Requests[request].ID
	switch {
	case id == turn.ID+"/prompt":
		row.Set("text", item.Requests[request].Message)
		row.Set("requestID", owner)
		row.Set("id", owner+"/prompt")
		return row
	case strings.HasPrefix(id, turn.ID+"/steer/"):
		row.Set("requestID", strings.TrimPrefix(id, turn.ID+"/steer/"))
		row.Set("steered", true)
	case row.Get("type") == "assistant":
		decisionText(row)
	}
	row.Set("id", owner+"/"+id)
	return row
}

var decisionStart = regexp.MustCompile(`^\s*\{\s*"decision"\s*:\s*\{`)
var decisionMessageKey = regexp.MustCompile(`"message"\s*:\s*"`)

// decisionText shows an agent's chat reply as the reply. The conversation
// protocol asks a harness for {"decision":{"type":"chat","message":...}}. A
// harness without a tool for it streams that JSON as ordinary text. Claude
// delivers it as the input of its StructuredOutput completion tool, which the
// page hides, so the reply is added as text unless the step already shows it.
func decisionText(row *sessionstate.Obj) {
	parts, _ := row.Get("content").([]any)
	kept := make([]any, 0, len(parts)+1)
	for _, value := range parts {
		part, ok := value.(*sessionstate.Obj)
		if text, _ := part.Get("text").(string); ok && part.Get("type") == "text" && decisionStart.MatchString(text) {
			if text = decisionMessage(text); text == "" {
				continue
			}
			part.Set("text", text)
		}
		kept = append(kept, value)
	}
	// Narration may share the step with the completion tool; the reply is added
	// after it unless the step already shows exactly that text.
	for _, value := range parts {
		part, ok := value.(*sessionstate.Obj)
		if !ok || part.Get("type") != "tool" || part.Get("name") != "StructuredOutput" {
			continue
		}
		text := completionMessage(part)
		if text == "" || slices.ContainsFunc(kept, func(other any) bool {
			shown, ok := other.(*sessionstate.Obj)
			return ok && shown.Get("type") == "text" && strings.TrimSpace(fmt.Sprint(shown.Get("text"))) == strings.TrimSpace(text)
		}) {
			continue
		}
		reply := sessionstate.NewObj()
		reply.Set("type", "text")
		reply.Set("text", text)
		kept = append(kept, reply)
	}
	if parts != nil {
		row.Set("content", kept)
	}
}

// completionMessage is the chat message in a StructuredOutput tool's input, or
// the part of it that has streamed so far. The input is a completion envelope,
// {"state":"completed","value":{"decision":...}} or {"state":"waiting",
// "message":...}, and is a JSON string until the tool call is whole.
func completionMessage(tool *sessionstate.Obj) string {
	state, _ := tool.Get("state").(*sessionstate.Obj)
	switch input := state.Get("input").(type) {
	case string:
		return decisionMessage(input)
	case *sessionstate.Obj:
		raw, err := json.Marshal(input)
		if err != nil {
			return ""
		}
		return decisionMessage(string(raw))
	}
	return ""
}

// decisionMessage is the chat message of a decision, bare or inside a
// completion envelope, or the part of it that has streamed so far.
func decisionMessage(text string) string {
	var whole struct {
		Decision struct {
			Message string `json:"message"`
		} `json:"decision"`
	}
	if json.Unmarshal([]byte(text), &whole) == nil && whole.Decision.Message != "" {
		return whole.Decision.Message
	}
	at := decisionMessageKey.FindStringIndex(text)
	if at == nil {
		return ""
	}
	rest := text[at[1]:]
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\\' {
			i++
		} else if rest[i] == '"' {
			end = i
			break
		}
	}
	// A stream can stop inside an escape sequence; show what is whole.
	for cut := end; cut >= 0 && cut >= end-6; cut-- {
		var out string
		if json.Unmarshal([]byte(`"`+rest[:cut]+`"`), &out) == nil {
			return out
		}
	}
	return ""
}
