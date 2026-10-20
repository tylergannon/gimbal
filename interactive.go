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
	Waiting        bool                  `json:"waiting"`
}
type interactionRuntime struct {
	ctx     context.Context
	session *Session
	wake    chan struct{}
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
	}
	session.restoreInteraction(node, state.ActiveInput)
	if state.Transcript.EndedBy != "" {
		r.interactionMu.Unlock()
		return state.Transcript, nil
	}
	if _, exists := r.activeInteractions[address]; exists {
		r.interactionMu.Unlock()
		return empty, fmt.Errorf("gimbal: simultaneous Interactive consumers at %q", address)
	}
	// Install the wakeup before inspecting the committed queue. Notifications
	// can coalesce; committed input is always the authority.
	active := interactionRuntime{ctx: context.WithoutCancel(ctx), session: session, wake: make(chan struct{}, 1)}
	r.activeInteractions[address] = active
	r.interactionStates[node] = state
	r.interactionMu.Unlock()
	defer func() {
		r.interactionMu.Lock()
		delete(r.activeInteractions, address)
		r.interactionMu.Unlock()
	}()
	type turnResult struct {
		output interactiveOutput
		err    error
	}
	var result <-chan turnResult
	var cancel context.CancelCauseFunc
	defer func() {
		if cancel != nil {
			cancel(nil)
		}
		if result != nil {
			<-result
		}
	}()
	for {
		if err := context.Cause(ctx); err != nil {
			return state.Transcript, err
		}
		r.interactionMu.Lock()
		r.refreshInteractions()
		inputs := r.pendingInput(state)
		if result == nil {
			// Finish closes ingress at acceptance, including queued messages that
			// were accepted before it. They must never start another native turn.
			for _, input := range inputs {
				if input.Finish && state.ActiveInput == "" {
					state.Consumed = append(state.Consumed, input.ID)
					state.Transcript.EndedBy, state.Waiting = "human", false
					state.ActiveInput, state.Prompt = "", ""
					err = r.commitInteraction(state)
					r.interactionMu.Unlock()
					return state.Transcript, err
				}
			}
			continuing := state.ActiveInput != ""
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
				select {
				case <-ctx.Done():
					return state.Transcript, context.Cause(ctx)
				case <-active.wake:
				}
				continue
			}
			input := inputs[0]
			if input.Disposition == "uncertain" || input.Disposition == "inflight" {
				state.Unresolved = fmt.Sprintf("Input %s delivery is unresolved; inspect the recorded native operation", input.ID)
				_ = r.commitInteraction(state)
				r.interactionMu.Unlock()
				return state.Transcript, errors.New(state.Unresolved)
			}
			if input.Configure {
				err = session.configure(ctx, input.Model, input.Effort)
				if err == nil {
					state.Consumed = append(state.Consumed, input.ID)
					err = r.commitInteraction(state)
				}
				r.interactionMu.Unlock()
				if err != nil {
					return state.Transcript, err
				}
				continue
			}
			if input.Stop {
				state.Consumed = append(state.Consumed, input.ID)
				err = r.commitInteraction(state)
				r.interactionMu.Unlock()
				if err != nil {
					return state.Transcript, err
				}
				continue
			}
			if !continuing {
				state.Consumed = append(state.Consumed, input.ID)
				state.ActiveInput, state.InputPublished = input.ID, false
				state.Prompt = purpose + "\n\nHuman input:\n" + input.Message + `

Respond with {"decision":{"type":"chat","message":"your reply"}}, or {"decision":{"type":"end"}} when finished.`
				if text := scopeText(ctx); text != "" {
					state.Prompt += "\n\n" + text
				}
			}
			state.Waiting = false
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			if err != nil {
				return state.Transcript, err
			}
			turnCtx, stop := context.WithCancelCause(ctx)
			cancel = stop
			turnCtx = context.WithValue(turnCtx, interactionGenerationKey{}, interactionGeneration{Node: node, Input: input.ID})
			done := make(chan turnResult, 1)
			result = done
			prompt := state.Prompt
			go func() {
				output, err := dispatch[interactiveOutput](turnCtx, session, prompt, nil)
				done <- turnResult{output, err}
			}()
			continue
		}
		// Only this loop assigns accepted messages to a native turn. Admission
		// never starts hidden turns or calls back from Session into interaction.
		for _, input := range inputs {
			if input.Finish || input.Stop {
				cancel(errors.New("human requested interruption"))
				continue
			}
			if input.Configure || input.Disposition != "pending" {
				continue
			}
			input.Disposition = "inflight"
			err = r.updateInput(input)
			r.interactionMu.Unlock()
			if err != nil {
				return state.Transcript, err
			}
			landed, deliveryErr := session.Steer(effects.WithInputID(active.ctx, input.ID), input.Message)
			r.interactionMu.Lock()
			input.Disposition = "pending"
			if deliveryErr != nil {
				input.Disposition = "uncertain"
			} else if landed {
				input.Disposition = "delivered"
			}
			if err = r.updateInput(input); err != nil {
				r.interactionMu.Unlock()
				return state.Transcript, err
			}
		}
		r.interactionMu.Unlock()
		select {
		case <-ctx.Done():
			cancel(context.Cause(ctx))
			<-result
			result = nil
			return state.Transcript, context.Cause(ctx)
		case <-active.wake:
			continue
		case completed := <-result:
			cancel(nil)
			result = nil
			session.mu.Lock()
			delivered := session.generationDelivered
			session.mu.Unlock()
			r.interactionMu.Lock()
			r.refreshInteractions()
			input := r.interactionInputs[state.ActiveInput]
			if !state.InputPublished && (completed.err == nil || delivered) {
				state.Transcript.Messages = append(state.Transcript.Messages, InteractiveMessage{Role: "user", Message: input.Message})
				state.InputPublished = true
				input.Disposition = "delivered"
			} else if !state.InputPublished && errors.Is(completed.err, context.Canceled) {
				input.Disposition = "unprocessed"
			}
			if err := r.updateInput(input); err != nil {
				r.interactionMu.Unlock()
				return state.Transcript, err
			}
			finish, stop := false, false
			for _, next := range r.pendingInput(state) {
				if next.Finish {
					finish = true
					state.Consumed = append(state.Consumed, next.ID)
					continue
				}
				if next.Stop {
					stop = true
					state.Consumed = append(state.Consumed, next.ID)
					continue
				}
				if next.Disposition == "delivered" {
					state.Consumed = append(state.Consumed, next.ID)
					state.Transcript.Messages = append(state.Transcript.Messages, InteractiveMessage{Role: "user", Message: next.Message})
				}
				if next.Disposition == "uncertain" || next.Disposition == "inflight" {
					state.Unresolved = fmt.Sprintf("Input %s delivery is unresolved; inspect the recorded native operation", next.ID)
				}
			}
			if isUnresolved(completed.err) {
				state.Unresolved = completed.err.Error()
			}
			if state.Unresolved != "" {
				_ = r.commitInteraction(state)
				r.interactionMu.Unlock()
				return state.Transcript, errors.New(state.Unresolved)
			}
			if completed.err == nil {
				switch decision := completed.output.Decision.(type) {
				case chat:
					state.Transcript.Messages = append(state.Transcript.Messages, InteractiveMessage{Role: "assistant", Message: decision.Message})
				case end:
					state.Transcript.EndedBy = "agent"
				default:
					r.interactionMu.Unlock()
					return state.Transcript, errors.New("gimbal: invalid interaction decision")
				}
			} else if (!finish && !stop) || !errors.Is(completed.err, context.Canceled) {
				r.interactionMu.Unlock()
				return state.Transcript, completed.err
			}
			state.ActiveInput, state.Prompt = "", ""
			state.Waiting = state.Transcript.EndedBy == ""
			if finish {
				state.Transcript.EndedBy, state.Waiting = "human", false
			}
			err = r.commitInteraction(state)
			r.interactionMu.Unlock()
			if err != nil || state.Transcript.EndedBy != "" {
				return state.Transcript, err
			}
		}
	}
}

func (r *run) updateInput(input interactionInput) error {
	if err := r.recordFact(r.rootNode, "", "", "interaction-input", input, runstore.Committed); err != nil {
		return err
	}
	r.interactionInputs[input.ID] = input
	return nil
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
	if r.State() == "terminal" {
		return errors.New("gimbal: interaction owner has ended")
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
		for _, state := range r.interactionStates {
			if state.Address == address && state.Transcript.EndedBy == "" && (!state.Waiting || len(r.pendingInput(state)) > 0) {
				return errors.New("gimbal: wait for pending input before changing model")
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
	for _, accepted := range r.interactionInputs {
		if accepted.Address == address && accepted.Finish {
			return errors.New("gimbal: interaction ingress is closed")
		}
	}
	active, ok := r.activeInteractions[address]
	if !ok {
		return errors.New("gimbal: interaction owner is starting or unavailable; retry input with the same ID")
	}
	input.Disposition = "pending"
	if err := r.updateInput(input); err != nil {
		return err
	}
	r.interactionOrder = append(r.interactionOrder, id)
	select {
	case active.wake <- struct{}{}:
	default:
	}
	return nil
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
