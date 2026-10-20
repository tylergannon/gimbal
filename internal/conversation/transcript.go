package conversation

import (
	"encoding/json"
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
func projectTranscript(item *Conversation, run observation.RunSnapshot) (bool, error) {
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

	matched := map[string]bool{}
	var messages []*sessionstate.Obj
	open := false
	for _, turn := range turns {
		request := requestForTurn(item, matched, turn)
		if request >= 0 {
			matched[item.Requests[request].ID] = true
			item.Requests[request].Delivery = "turn"
		}
		open = turn.Ended == 0 && !turn.Unresolved
		state := run.Transcripts[turn.ID].Snapshot.State
		for _, key := range slices.Sorted(maps.Keys(state.Message)) {
			for _, row := range state.Message[key] {
				if row = turnMessage(item, request, turn, row); row != nil {
					messages = append(messages, row)
				}
			}
		}
	}
	// Steered prompts become part of the turn that took them.
	for index := range item.Requests {
		request := &item.Requests[index]
		if request.Kind != "prompt" || matched[request.ID] {
			continue
		}
		if slices.ContainsFunc(messages, func(row *sessionstate.Obj) bool { return row.Get("requestID") == request.ID }) {
			request.Delivery = "steer"
		}
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

func timeCreated(created int64) *sessionstate.Obj {
	out := sessionstate.NewObj()
	out.Set("created", float64(created))
	return out
}

// requestForTurn pairs a turn with the first not yet paired prompt request
// whose message the turn's prompt carries. Turns run in admission order, so
// repeated identical messages pair one to one.
func requestForTurn(item *Conversation, matched map[string]bool, turn observation.TurnRow) int {
	_, human, found := strings.Cut(turn.Prompt, humanInput)
	if !found {
		return -1
	}
	for index, request := range item.Requests {
		if request.Kind != "prompt" || matched[request.ID] {
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
// protocol asks a harness for {"decision":{"type":"chat","message":...}}, and
// a harness without a tool for it streams that JSON as ordinary text.
func decisionText(row *sessionstate.Obj) {
	parts, _ := row.Get("content").([]any)
	kept := make([]any, 0, len(parts))
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
	if parts != nil {
		row.Set("content", kept)
	}
}

// decisionMessage is the chat message of a complete decision, or the part of
// it that has streamed so far.
func decisionMessage(text string) string {
	var whole struct {
		Decision struct {
			Message string `json:"message"`
		} `json:"decision"`
	}
	if json.Unmarshal([]byte(text), &whole) == nil {
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
