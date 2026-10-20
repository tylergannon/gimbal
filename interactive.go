package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
)

// InteractiveMessage is one confirmed message, in conversation order.
type InteractiveMessage struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

// InteractiveTranscript is the ordered history of one interaction occurrence.
// EndedBy is agent or human on normal completion, and empty on error.
type InteractiveTranscript struct {
	Messages []InteractiveMessage `json:"messages"`
	EndedBy  string               `json:"ended_by"`
}

type interactiveDecision interface{ interactiveDecision() }
type chat struct {
	Message string `json:"message"`
}
type end struct{}

func (chat) interactiveDecision() {}
func (end) interactiveDecision()  {}

type interactiveOutput struct {
	Decision interactiveDecision `json:"decision"`
}
type interactionInput = runstate.InteractionInput

type interactionState struct {
	Prompt         string                `json:"prompt"`
	InputPublished bool                  `json:"input_published"`
	Ordinal        int                   `json:"ordinal"`
	Unresolved     string                `json:"unresolved"`
	Scope          string                `json:"scope"`
	Session        string                `json:"session"`
	ActiveInput    string                `json:"active_input"`
	Address        string                `json:"address"`
	Node           runstore.NodeID       `json:"node"`
	Transcript     InteractiveTranscript `json:"transcript"`
	Consumed       []string              `json:"consumed"`
	Turns          int                   `json:"turns"`
	Generations    int                   `json:"generations"`
	Waiting        bool                  `json:"waiting"`
}
type interactionRuntime struct {
	ctx     context.Context
	session *Session
	cancel  context.CancelCauseFunc
}

// Interactive waits for human input before each occurrence's first turn. A
// chat reply atomically establishes the next durable wait; end returns the
// transcript. Human finish returns normally with EndedBy human.
func Interactive(ctx context.Context, name string, session *Session, purpose string) (InteractiveTranscript, error) {
	empty := InteractiveTranscript{Messages: []InteractiveMessage{}}
	if session == nil {
		return empty, errors.New("gimbal: Interactive requires a session")
	}
	s, err := current(ctx)
	if err != nil {
		return empty, err
	}
	_, node := s.nextNode(runstore.Action, name)
	r := s.run
	address := path.Join(s.key, name)
	s.mu.Lock()
	ordinal := s.ordinals[name]
	s.mu.Unlock()
	state := interactionState{Ordinal: ordinal, Scope: s.key, Session: session.id, Address: address, Node: node, Transcript: empty, Waiting: true}
	r.interactionMu.Lock()
	r.refreshInteractions()
	if old, ok := r.interactionStates[node]; ok {
		state = old
		session.mu.Lock()
		session.turns = max(session.turns, state.Turns)
		session.generations = max(session.generations, state.Generations)
		session.mu.Unlock()
	}
	r.interactionStates[node] = state
	r.interactionMu.Unlock()
	if state.Transcript.EndedBy != "" {
		return state.Transcript, nil
	}
	for {
		r.interactionMu.Lock()
		r.refreshInteractions()
		inputs := r.pendingInput(state)
		continuing := !state.Waiting && state.ActiveInput != ""
		if continuing {
			inputs = append([]interactionInput{r.interactionInputs[state.ActiveInput]}, inputs...)
		}
		if len(inputs) == 0 {
			state.Waiting = true
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			if err != nil {
				return state.Transcript, err
			}
			panic(suspension{})
		}
		input := inputs[0]
		if input.Disposition == "uncertain" {
			if !continuing {
				for _, control := range inputs {
					if control.Finish {
						state.Consumed = append(state.Consumed, control.ID)
						state.Unresolved, state.Transcript.EndedBy, state.Waiting = "", "human", false
						err = r.commitInteraction(state)
						r.interactionMu.Unlock()
						return state.Transcript, err
					}
				}
			}
			state.Unresolved = fmt.Sprintf("Input %s delivery is unresolved", input.ID)
			_ = r.commitInteraction(state)
			r.interactionMu.Unlock()
			panic(suspension{})
		}
		if input.Configure {
			if session.native != "" {
				if configurable, ok := session.adapter.(interface {
					ConfigureSession(context.Context, string, string, string) error
				}); ok {
					err = configurable.ConfigureSession(ctx, session.native, input.Model, input.Effort)
				} else {
					err = errors.New("gimbal: executor cannot configure this session")
				}
			}
			if err == nil {
				session.model, session.effort = input.Model, input.Effort
				err = r.recordFact(session.node, s.node, session.node, "session-configuration", map[string]string{"model": input.Model, "effort": input.Effort}, runstore.Committed)
			}
			if err != nil {
				r.interactionMu.Unlock()
				return state.Transcript, err
			}
			state.Consumed = append(state.Consumed, input.ID)
			state.Waiting = true
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			if err != nil {
				return state.Transcript, err
			}
			continue
		}
		if input.Stop {
			state.Consumed = append(state.Consumed, input.ID)
			state.Waiting = true
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			if err != nil {
				return state.Transcript, err
			}
			continue
		}
		if input.Finish {
			state.Consumed = append(state.Consumed, input.ID)
			state.Transcript.EndedBy = "human"
			state.Waiting = false
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			return state.Transcript, err
		}
		if !continuing {
			state.Consumed = append(state.Consumed, input.ID)
			state.ActiveInput = input.ID
			state.InputPublished = false
			state.Prompt = ""
		}
		state.Waiting = false
		if state.Prompt == "" {
			promptTranscript := state.Transcript
			promptTranscript.Messages = append(append([]InteractiveMessage(nil), state.Transcript.Messages...), InteractiveMessage{Role: "user", Message: input.Message})
			raw, _ := json.Marshal(promptTranscript)
			state.Prompt = purpose + "\n\nConversation:\n" + string(raw) + `

Respond with {"decision":{"type":"chat","message":"your reply"}}, or {"decision":{"type":"end"}} when finished.`
			if text := scopeText(ctx); text != "" {
				state.Prompt += "\n\n" + text
			}
		}
		err = r.commitInteraction(state)
		turnCtx, cancel := context.WithCancelCause(ctx)
		r.activeInteractions[address] = interactionRuntime{ctx: context.WithoutCancel(ctx), session: session, cancel: cancel}
		session.mu.Lock()
		session.interactiveDelivery = func() error {
			r.interactionMu.Lock()
			defer r.interactionMu.Unlock()
			if !state.InputPublished {
				state.Transcript.Messages = append(state.Transcript.Messages, InteractiveMessage{Role: "user", Message: input.Message})
				state.InputPublished = true
				return r.commitInteraction(state)
			}
			return nil
		}
		session.mu.Unlock()
		r.interactionMu.Unlock()
		if err != nil {
			cancel(nil)
			return state.Transcript, err
		}
		output, turnErr := dispatch[interactiveOutput](turnCtx, session, state.Prompt, nil)
		cancel(nil)
		session.mu.Lock()
		delivered := session.generationDelivered
		session.interactiveDelivery = nil
		session.mu.Unlock()
		r.interactionMu.Lock()
		delete(r.activeInteractions, address)
		r.refreshInteractions()
		pending := r.pendingInput(state)
		if !isUnresolved(turnErr) {
			session.mu.Lock()
			state.Turns = session.turns
			state.Generations = session.generations
			session.mu.Unlock()
		}

		if !state.InputPublished && (turnErr == nil || delivered) {
			state.Transcript.Messages = append(state.Transcript.Messages, InteractiveMessage{Role: "user", Message: input.Message})
			state.InputPublished = true
		}
		if turnErr == nil {
			state.ActiveInput = ""
			state.Prompt = ""
		}
		finish := false
		stop := false
		for _, next := range pending {
			if next.Stop {
				state.Consumed = append(state.Consumed, next.ID)
				stop = true
				continue
			}
			if next.Finish {
				state.Consumed = append(state.Consumed, next.ID)
				finish = true
				break
			}
			if next.Disposition == "delivered" {
				state.Consumed = append(state.Consumed, next.ID)
				state.Transcript.Messages = append(state.Transcript.Messages, InteractiveMessage{Role: "user", Message: next.Message})
			}
		}
		if finish && !isUnresolved(turnErr) {
			state.Transcript.EndedBy = "human"
			state.Waiting = false
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			return state.Transcript, err
		}
		if stop && !isUnresolved(turnErr) {
			state.Waiting = true
			state.ActiveInput = ""
			state.Prompt = ""
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			if err != nil {
				return state.Transcript, err
			}
			continue
		}
		if turnErr != nil {
			if isUnresolved(turnErr) {
				state.Unresolved = turnErr.Error()
				_ = r.commitInteraction(state)
				r.interactionMu.Unlock()
				panic(suspension{})
			}
			r.interactionMu.Unlock()
			return state.Transcript, turnErr
		}
		session.mu.Lock()
		state.Turns = session.turns
		state.Generations = session.generations
		session.mu.Unlock()
		switch decision := output.Decision.(type) {
		case chat:
			state.Transcript.Messages = append(state.Transcript.Messages, InteractiveMessage{Role: "assistant", Message: decision.Message})
			state.Waiting = true
		case end:
			for _, next := range pending {
				if next.Disposition == "uncertain" {
					state.Unresolved = fmt.Sprintf("Input %s delivery is unresolved", next.ID)
				}
			}
			state.Waiting = state.Unresolved != ""
			if !state.Waiting {
				state.Transcript.EndedBy = "agent"
			}
		default:
			r.interactionMu.Unlock()
			return state.Transcript, errors.New("gimbal: invalid interaction decision")
		}
		err = r.commitInteraction(state)
		r.interactionMu.Unlock()
		if err != nil || state.Transcript.EndedBy != "" {
			return state.Transcript, err
		}
		if state.Unresolved != "" {
			panic(suspension{})
		}
	}
}

// State changes and inbox entries use the same committed run journal. Its
// sequence provides acceptance order independently of wall-clock timestamps.
func (r *run) refreshInteractions() {
	for {
		records, err := r.records.Read(context.Background(), r.interactionCursor, 4096)
		if err != nil {
			r.recordFailure("read interaction", err)
			return
		}
		for _, record := range records {
			r.interactionCursor = record.Sequence
			switch record.Kind {
			case "lifecycle":
				var lifecycle struct {
					Scope string `json:"scope"`
					Event struct {
						Kind string `json:"kind"`
					} `json:"event"`
				}
				if json.Unmarshal(record.Payload, &lifecycle) == nil {
					if r.interactionEnded == nil {
						r.interactionEnded = map[string]bool{}
					}
					switch lifecycle.Event.Kind {
					case "run_ended":
						r.interactionEnded[""] = true
					case "scope_ended":
						r.interactionEnded[lifecycle.Scope] = true
					}
				}
			case "interaction":
				var state interactionState
				if json.Unmarshal(record.Payload, &state) == nil {
					r.interactionStates[state.Node] = state
				}
			case "interaction-input":
				var input interactionInput
				if json.Unmarshal(record.Payload, &input) == nil {
					if _, ok := r.interactionInputs[input.ID]; !ok {
						r.interactionOrder = append(r.interactionOrder, input.ID)
					}
					r.interactionInputs[input.ID] = input
				}
			}
		}
		if len(records) < 4096 {
			return
		}
	}
}
func (r *run) pendingInput(state interactionState) []interactionInput {
	consumed := map[string]bool{}
	// An accepted message belongs to the stable address, but can be consumed
	// by only one occurrence at that address.
	for _, other := range r.interactionStates {
		for _, id := range other.Consumed {
			consumed[id] = true
		}
	}
	for _, id := range state.Consumed {
		consumed[id] = true
	}
	var pending []interactionInput
	for _, id := range r.interactionOrder {
		input := r.interactionInputs[id]
		if input.Address == state.Address && input.Disposition != "unprocessed" && !consumed[id] {
			pending = append(pending, input)
		}
	}
	return pending
}
func (r *run) commitInteraction(state interactionState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	status := "active"
	if state.Waiting {
		status = "waiting"
		if len(r.pendingInput(state)) > 0 {
			status = "runnable"
		}
	}
	if state.Unresolved != "" {
		status = "unresolved"
	}
	if state.Transcript.EndedBy != "" {
		status = "ended"
	}
	event := InteractiveChanged{Consumed: state.Consumed, Status: status, Ordinal: state.Ordinal, ID: string(state.Node), Name: path.Base(state.Address), Address: state.Address, Transcript: state.Transcript, Waiting: state.Waiting, Error: state.Unresolved}
	lifecycle := LifecycleRecord{Time: time.Now().UTC(), Scope: state.Scope, Session: optionalString(state.Session), Event: event}
	projection, err := json.Marshal(lifecycle)
	if err != nil {
		return err
	}
	_, err = r.records.Append(context.Background(), []runstore.Fact{
		{Version: 1, ID: runstore.NewRecordID(), Node: state.Node, Kind: "interaction", Payload: raw},
		{Version: 1, ID: runstore.NewRecordID(), Node: state.Node, Kind: "lifecycle", Payload: projection},
	}, runstore.Committed)
	if err != nil {
		return err
	}
	r.interactionStates[state.Node] = state
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	if r.writer != nil {
		_, err = r.writer.writeLifecycleRecord(lifecycle, true)
		r.observationFailure("interaction projection", err)
	}
	r.observationFailure("interaction projection", r.observeLifecycle("", "", "", event, projection))
	return nil
}

// SubmitInteractive accepts a retryable durable input before attempting native
// delivery. A request ID reused with different contents is rejected.
func (r *run) acceptInput(input interactionInput) error { return r.acceptInputAtTurn(input, "") }

func (r *run) acceptInputAtTurn(input interactionInput, expectedTurn string) error {
	address, id, message, finish := input.Address, input.ID, input.Message, input.Finish
	r.interactionMu.Lock()
	defer r.interactionMu.Unlock()
	if r.State() == "terminal" {
		return errors.New("gimbal: interaction owner has ended")
	}
	r.refreshInteractions()
	if id == "" {
		return errors.New("gimbal: interaction input requires an id")
	}
	if old, ok := r.interactionInputs[id]; ok {
		if old.Address != address || old.Message != message || old.Finish != finish || old.Stop != input.Stop || old.Configure != input.Configure || old.Model != input.Model || old.Effort != input.Effort {
			return runstore.ErrConflict
		}
		return nil
	}
	if expectedTurn != "" {
		active, ok := r.activeInteractions[address]
		if !ok {
			return errors.New("gimbal: interaction turn has ended")
		}
		active.session.mu.Lock()
		matches := active.session.running && active.session.turnID == expectedTurn
		active.session.mu.Unlock()
		if !matches {
			return errors.New("gimbal: interaction turn has ended")
		}
	}
	if input.Configure {
		if _, active := r.activeInteractions[address]; active {
			return errors.New("gimbal: wait for the active turn before changing model")
		}
		for _, state := range r.interactionStates {
			if state.Address == address && state.Transcript.EndedBy == "" && (!state.Waiting || len(r.pendingInput(state)) > 0) {
				return errors.New("gimbal: wait for pending input before changing model")
			}
		}
	}
	if finish || input.Stop {
		for node, state := range r.interactionStates {
			if state.Address == address && state.Unresolved != "" {
				state.Unresolved = ""
				r.interactionStates[node] = state
				if err := r.commitInteraction(state); err != nil {
					return err
				}
			}
		}
	}
	owner := path.Dir(address)
	if owner == "." {
		owner = ""
	}
	if r.interactionEnded[""] || r.interactionEnded[owner] {
		return errors.New("gimbal: interaction owner has ended")
	}
	input.Disposition = "pending"
	if err := r.recordFact(r.rootNode, "", "", "interaction-input", input, runstore.Committed); err != nil {
		return err
	}
	r.interactionInputs[id] = input
	r.interactionOrder = append(r.interactionOrder, id)
	for _, state := range r.interactionStates {
		if state.Address == address && state.Transcript.EndedBy == "" {
			if err := r.commitInteraction(state); err != nil {
				return err
			}
		}
	}
	active, ok := r.activeInteractions[address]
	if !ok {
		return nil
	}
	if finish || input.Stop {
		reason := "human finished interaction"
		if input.Stop {
			reason = "human stopped interaction turn"
		}
		active.cancel(errors.New(reason))
		return nil
	}
	r.interactionMu.Unlock()
	landed, err := active.session.Steer(effects.WithInputID(active.ctx, input.ID), message)
	r.interactionMu.Lock()
	if err != nil {
		input.Disposition = "uncertain"
	} else if landed {
		input.Disposition = "delivered"
	}
	if e := r.recordFact(r.rootNode, "", "", "interaction-input", input, runstore.Committed); e != nil {
		return e
	}
	r.interactionInputs[id] = input
	return err
}
func (r *run) Runnable() bool {
	if r.rootCancellation() != nil {
		return true
	}
	r.interactionMu.Lock()
	defer r.interactionMu.Unlock()
	r.refreshInteractions()
	for _, state := range r.interactionStates {
		if state.Transcript.EndedBy == "" && state.Unresolved == "" && len(r.pendingInput(state)) > 0 {
			return true
		}
	}
	return false
}
func isUnresolved(err error) bool {
	var uncertain *backend.DeliveryUncertain
	return errors.As(err, &uncertain)
}

func (r *run) SubmitInteractive(address, request string) error {
	var input interactionInput
	if err := json.Unmarshal([]byte(request), &input); err != nil {
		return err
	}
	input.Address = address
	return r.acceptInput(input)
}

func (r *run) acceptInteractive(address, id, message string, finish bool) error {
	return r.acceptInput(interactionInput{Address: address, ID: id, Message: message, Finish: finish})
}

func (r *run) closeInteractionInputs(scope string) {
	r.interactionMu.Lock()
	defer r.interactionMu.Unlock()
	r.refreshInteractions()
	consumed := map[string]bool{}
	for _, state := range r.interactionStates {
		for _, id := range state.Consumed {
			consumed[id] = true
		}
	}
	for _, id := range r.interactionOrder {
		input := r.interactionInputs[id]
		owner := path.Dir(input.Address)
		if owner == "." {
			owner = ""
		}
		if consumed[id] || (input.Disposition == "unprocessed" || input.Disposition == "uncertain") || (scope != "" && owner != scope && !strings.HasPrefix(owner, scope+"/")) {
			continue
		}
		input.Disposition = "unprocessed"
		if err := r.recordFact(r.rootNode, "", "", "interaction-input", input, runstore.Committed); err != nil {
			return
		}
		r.interactionInputs[id] = input
	}
}
