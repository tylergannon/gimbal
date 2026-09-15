// Ported from OpenCode c55ee2a8152603f04a409163bd3edf79c425fbd7. MIT,
// Copyright (c) 2025 opencode. See NOTICE.md beside this file.

package sessionstate

import (
	"encoding/json"
	"strings"
)

// eventMessageID derives a message row ID from the event that created it,
// rewriting only the identifier prefix.
func eventMessageID(id string) string {
	if strings.HasPrefix(id, "evt_") {
		return "msg_" + id[len("evt_"):]
	}
	return id
}

// jsString renders a value the way JavaScript string concatenation does, so a
// delta applied to a missing or non-string field produces the same text the
// upstream updater would have produced.
func jsString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return "null"
	case undefinedType:
		return "undefined"
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case json.Number:
		return typed.String()
	default:
		encoded, err := marshalValue(value)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

// setDefined assigns only when the value exists, which is what Object.assign
// of a withDefined payload does: a member the payload omits keeps whatever
// the target already had.
func setDefined(target *Obj, key string, value any) {
	if isUndefined(value) {
		return
	}
	target.Set(key, value)
}

func obj(pairs ...any) *Obj {
	out := NewObj()
	for i := 0; i+1 < len(pairs); i += 2 {
		out.Set(str(pairs[i]), pairs[i+1])
	}
	return out
}

func appendContent(assistant *Obj, part *Obj) {
	assistant.Set("content", append(arrOf(assistant.Get("content")), part))
}

func toolProgressMetadata(state *Obj, value any) any {
	incoming := objOf(value)
	if incoming == nil || str(incoming.Get("mode")) != "append" {
		return cloneValue(value)
	}
	existing := arrOf(objOf(state.Get("metadata")).Get("transcript"))
	if existing == nil {
		for _, partValue := range arrOf(state.Get("content")) {
			part := objOf(partValue)
			if str(part.Get("type")) == "transcript" {
				existing = arrOf(cloneValue(part.Get("events")))
				break
			}
		}
	}
	transcript := existing
	for _, entry := range arrOf(incoming.Get("transcript")) {
		transcript = append(transcript, cloneValue(entry))
	}
	metadata := incoming.Clone()
	metadata.Delete("mode")
	metadata.Set("transcript", transcript)
	return metadata
}

// Apply reduces one native event. Unknown event types, and the schema entries
// the selected upstream revision has no handler for, change nothing.
func (p *Projection) Apply(event *Obj) {
	d := objOf(event.Get("data"))
	if d == nil {
		d = NewObj()
	}
	sessionID := str(d.Get("sessionID"))
	created := event.Get("created")
	assistantID := str(d.Get("assistantMessageID"))

	switch str(event.Get("type")) {
	case "session.deleted":
		p.removeSession(sessionID)

	case "session.usage.updated":
		if info, ok := p.state.Info[sessionID]; ok {
			info.Set("cost", d.Get("cost"))
			info.Set("tokens", cloneValue(d.Get("tokens")))
		}

	case "session.agent.selected":
		previous := p.state.Info[sessionID].Get("agent")
		if info, ok := p.state.Info[sessionID]; ok {
			info.Set("agent", cloneValue(d.Get("agent")))
		}
		p.insert(sessionID, obj(
			"id", eventMessageID(str(event.Get("id"))),
			"type", "agent-switched",
			"agent", cloneValue(d.Get("agent")),
			"previous", previous,
			"time", obj("created", created),
		))

	case "session.model.selected":
		if info, ok := p.state.Info[sessionID]; ok {
			info.Set("model", cloneValue(d.Get("model")))
		}
		if _, ok := p.state.Message[sessionID]; ok {
			messageID := eventMessageID(str(event.Get("id")))
			p.insert(sessionID, obj(
				"id", messageID,
				"type", "model-switched",
				"model", cloneValue(d.Get("model")),
				"time", obj("created", created),
			))
		}

	case "session.permissions.updated":
		if info, ok := p.state.Info[sessionID]; ok {
			info.Set("permissions", cloneValue(d.Get("permissions")))
		}

	case "session.moved":
		p.moved(event)

	case "session.inbox.enqueued":
		item := obj(
			"id", d.Get("inboxID"),
			"sessionID", sessionID,
			"timeCreated", created,
		)
		for _, key := range objOf(d.Get("item")).Keys() {
			item.Set(key, cloneValue(objOf(d.Get("item")).Get(key)))
		}
		p.admit(item)

	case "session.inbox.delivered":
		p.deliver(sessionID, str(d.Get("inboxID")), created)

	case "session.inbox.delivery.changed":
		p.updatePending(sessionID, str(d.Get("inboxID")), d.Get("delivery"))

	case "session.inbox.cancelled":
		p.retract(sessionID, str(d.Get("inboxID")))

	case "session.instructions.updated":
		if !isUndefined(d.Get("text")) {
			row := obj(
				"id", eventMessageID(str(event.Get("id"))),
				"type", "system",
				"text", d.Get("text"),
				"description", "Instructions updated: "+strings.Join(objOf(d.Get("delta")).Keys(), ", "),
			)
			if !isUndefined(event.Get("metadata")) {
				row.Set("metadata", cloneValue(event.Get("metadata")))
			}
			row.Set("time", obj("created", created))
			p.insert(sessionID, row)
		}

	case "session.synthetic":
		p.insert(sessionID, obj(
			"id", eventMessageID(str(event.Get("id"))),
			"type", "synthetic",
			"text", d.Get("text"),
			"description", d.Get("description"),
			"metadata", cloneValue(d.Get("metadata")),
			"time", obj("created", created),
		))

	case "session.shell.started":
		p.shellStarted(event)

	case "session.shell.ended":
		p.shellEnded(event)

	case "session.step.started":
		p.stepStarted(event)

	case "session.step.streamed":
		p.editAssistant(sessionID, assistantID, func(a *Obj) {
			objOf(a.Get("time")).Set("streamed", created)
		})

	case "session.step.ended":
		p.stepEnded(event)

	case "session.step.failed":
		p.stepFailed(event)

	case "session.text.started":
		p.editAssistant(sessionID, assistantID, func(a *Obj) {
			part := obj("type", "text", "text", "")
			appendContent(a, part)
			p.latestText[partKey(sessionID, assistantID, "")] = part
		})

	case "session.text.delta":
		p.editText(sessionID, assistantID, func(t *Obj) {
			t.Set("text", jsString(t.Get("text"))+jsString(d.Get("delta")))
		})

	case "session.text.ended":
		p.editText(sessionID, assistantID, func(t *Obj) { t.Set("text", d.Get("text")) })

	case "session.tool.input.started":
		p.editAssistant(sessionID, assistantID, func(a *Obj) {
			part := obj(
				"type", "tool",
				"id", d.Get("id"),
				"name", d.Get("name"),
				"time", obj("created", created),
				"state", obj("status", "streaming", "input", ""),
			)
			appendContent(a, part)
			p.tools[partKey(sessionID, assistantID, str(d.Get("id")))] = part
		})

	case "session.tool.input.delta":
		p.editTool(sessionID, assistantID, str(d.Get("id")), func(t *Obj) {
			state := objOf(t.Get("state"))
			if str(state.Get("status")) == "streaming" {
				state.Set("input", jsString(state.Get("input"))+jsString(d.Get("delta")))
			}
		})

	case "session.tool.input.ended":
		p.editTool(sessionID, assistantID, str(d.Get("id")), func(t *Obj) {
			state := objOf(t.Get("state"))
			if str(state.Get("status")) == "streaming" {
				state.Set("input", d.Get("text"))
			}
		})

	case "session.tool.called":
		p.toolCalled(event)

	case "session.tool.progress":
		p.editTool(sessionID, assistantID, str(d.Get("id")), func(t *Obj) {
			state := objOf(t.Get("state"))
			switch str(state.Get("status")) {
			case "running", "completed", "error":
				state.Set("metadata", toolProgressMetadata(state, d.Get("metadata")))
			}
		})

	case "session.tool.success":
		p.toolSuccess(event)

	case "session.tool.failed":
		p.toolFailed(event)

	case "session.reasoning.started":
		p.editAssistant(sessionID, assistantID, func(a *Obj) {
			part := obj(
				"type", "reasoning",
				"text", "",
				"state", cloneValue(d.Get("state")),
				"time", obj("created", created),
			)
			appendContent(a, part)
			key := partKey(sessionID, assistantID, "")
			p.openReasoning[key] = append(p.openReasoning[key], part)
		})

	case "session.reasoning.delta":
		p.editReasoning(sessionID, assistantID, func(r *Obj) {
			r.Set("text", jsString(r.Get("text"))+jsString(d.Get("delta")))
		})

	case "session.reasoning.ended":
		p.reasoningEnded(event)

	case "session.retry.scheduled":
		p.editAssistant(sessionID, assistantID, func(a *Obj) {
			a.Set("retry", obj("attempt", d.Get("attempt"), "at", d.Get("at"), "error", cloneValue(d.Get("error"))))
		})

	case "session.execution.started":
		p.state.Active[sessionID] = "running"

	case "session.execution.succeeded", "session.execution.failed", "session.execution.interrupted":
		p.executionEnded(event)

	case "session.revert.staged":
		if info, ok := p.state.Info[sessionID]; ok {
			info.Set("revert", cloneValue(d.Get("revert")))
		}

	case "session.revert.cleared":
		if info, ok := p.state.Info[sessionID]; ok {
			info.Delete("revert")
		}

	case "session.revert.committed":
		p.revertCommitted(sessionID, str(d.Get("to")))

	case "session.compaction.started":
		p.compactionStarted(event)

	case "session.compaction.delta":
		if row, _ := p.runningCompaction(sessionID); row != nil {
			row.Set("summary", jsString(row.Get("summary"))+jsString(d.Get("text")))
		}

	case "session.compaction.ended":
		p.compactionEnded(event)

	case "session.compaction.failed":
		p.compactionFailed(event)

	case "permission.asked":
		list := p.permissions(sessionID)
		if indexOfID(list, str(d.Get("id"))) < 0 {
			p.state.Permission[sessionID] = append(list, d.Clone())
		}

	case "permission.replied":
		if list, ok := p.state.Permission[sessionID]; ok {
			if index := indexOfID(list, str(d.Get("requestID"))); index >= 0 {
				list[index].Set("reply", cloneValue(d.Get("reply")))
				list[index].Set("repliedAt", created)
			}
		}

	case "form.replied", "form.cancelled":
		p.removeForm(sessionID, str(d.Get("id")), objOf(event.Get("location")))

	case "form.created":
		if truthy(event.Get("location")) {
			form := objOf(d.Get("form"))
			owner := str(form.Get("sessionID"))
			list := p.forms(owner)
			if indexOfID(list, str(form.Get("id"))) < 0 {
				row := form.Clone()
				if owner == "global" {
					row.Set("location", cloneValue(event.Get("location")))
				}
				p.state.Form[owner] = append(list, row)
			}
		}
	}
}

func (p *Projection) permissions(sessionID string) []*Obj {
	if _, ok := p.state.Permission[sessionID]; !ok {
		p.state.Permission[sessionID] = []*Obj{}
	}
	return p.state.Permission[sessionID]
}

func (p *Projection) forms(sessionID string) []*Obj {
	if _, ok := p.state.Form[sessionID]; !ok {
		p.state.Form[sessionID] = []*Obj{}
	}
	return p.state.Form[sessionID]
}

func (p *Projection) pending(sessionID string) []*Obj {
	if _, ok := p.state.Pending[sessionID]; !ok {
		p.state.Pending[sessionID] = []*Obj{}
	}
	return p.state.Pending[sessionID]
}

func (p *Projection) moved(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	info, ok := p.state.Info[sessionID]
	if !ok {
		return
	}
	previous := obj(
		"location", cloneValue(info.Get("location")),
		"projectID", info.Get("projectID"),
		"subpath", info.Get("subpath"),
	)
	info.Set("location", cloneValue(d.Get("location")))
	if truthy(d.Get("projectID")) {
		info.Set("projectID", d.Get("projectID"))
	}
	info.Set("subpath", d.Get("subpath"))
	p.insert(sessionID, obj(
		"id", eventMessageID(str(event.Get("id"))),
		"type", "location-switched",
		"location", cloneValue(d.Get("location")),
		"projectID", d.Get("projectID"),
		"subpath", d.Get("subpath"),
		"previous", previous,
		"time", obj("created", event.Get("created")),
	))
}

func (p *Projection) admit(item *Obj) {
	sessionID := str(item.Get("sessionID"))
	list := p.pending(sessionID)
	at := indexOfID(list, str(item.Get("id")))
	if at < 0 {
		p.state.Pending[sessionID] = append(list, item.Clone())
	} else {
		list[at] = item.Clone()
	}
	kind := str(item.Get("type"))
	if kind != "user" && kind != "synthetic" {
		return
	}
	row := obj("id", item.Get("id"), "type", kind)
	payload := objOf(item.Get("payload"))
	for _, key := range payload.Keys() {
		row.Set(key, cloneValue(payload.Get(key)))
	}
	row.Set("time", obj("created", item.Get("timeCreated")))
	p.upsertMessage(sessionID, row)
}

func (p *Projection) retract(sessionID, id string) {
	if list, ok := p.state.Pending[sessionID]; ok && indexOfID(list, id) >= 0 {
		p.state.Pending[sessionID] = filterOutID(list, id)
	}
	index, ok := p.index[sessionID]
	if !ok {
		return
	}
	if _, ok := index.pos[id]; !ok {
		return
	}
	p.state.Message[sessionID] = filterOutID(p.state.Message[sessionID], id)
	p.rebuildIndexes(sessionID)
}

func (p *Projection) deliver(sessionID, id string, created any) {
	pending, hasPending := p.state.Pending[sessionID]
	admitted := hasPending && indexOfID(pending, id) >= 0
	if admitted {
		p.state.Pending[sessionID] = filterOutID(pending, id)
	}
	list, ok := p.state.Message[sessionID]
	if !ok {
		return
	}
	at := indexOfID(list, id)
	if !admitted || at < 0 {
		return
	}
	row := list[at]
	list = append(list[:at], list[at+1:]...)
	objOf(row.Get("time")).Set("created", created)
	p.state.Message[sessionID] = append(list, row)
	p.rebuildIndexes(sessionID)
}

func (p *Projection) updatePending(sessionID, id string, delivery any) {
	list, ok := p.state.Pending[sessionID]
	if !ok {
		return
	}
	at := indexOfID(list, id)
	if at < 0 {
		return
	}
	if jsonKey(list[at].Get("delivery")) != jsonKey(delivery) {
		list[at].Set("delivery", cloneValue(delivery))
	}
}

func (p *Projection) shellStarted(event *Obj) {
	d := objOf(event.Get("data"))
	shell := objOf(d.Get("shell"))
	metadata := event.Get("metadata")
	if background, ok := objOf(shell.Get("metadata")).Get("background").(bool); ok && background {
		merged := NewObj()
		if base := objOf(event.Get("metadata")); base != nil {
			merged = base.Clone()
		}
		merged.Set("background", true)
		metadata = merged
	}
	p.insert(str(d.Get("sessionID")), obj(
		"id", eventMessageID(str(event.Get("id"))),
		"type", "shell",
		"shellID", shell.Get("id"),
		"command", shell.Get("command"),
		"status", shell.Get("status"),
		"exit", shell.Get("exit"),
		"metadata", cloneValue(metadata),
		"time", obj("created", event.Get("created")),
	))
}

func (p *Projection) shellEnded(event *Obj) {
	d := objOf(event.Get("data"))
	shell := objOf(d.Get("shell"))
	row := p.shell(str(d.Get("sessionID")), str(shell.Get("id")))
	if row == nil {
		return
	}
	row.Set("status", shell.Get("status"))
	row.Set("exit", shell.Get("exit"))
	row.Set("output", cloneValue(d.Get("output")))
	objOf(row.Get("time")).Set("completed", event.Get("created"))
}

func (p *Projection) stepStarted(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	assistantID := str(d.Get("assistantMessageID"))
	created := event.Get("created")
	if existing := p.message(sessionID, assistantID); existing != nil && str(existing.Get("type")) == "assistant" {
		existing.Set("agent", d.Get("agent"))
		existing.Set("model", cloneValue(d.Get("model")))
		for _, key := range []string{"retry", "error", "finish", "rawFinish", "providerState"} {
			existing.Delete(key)
		}
		time := objOf(existing.Get("time"))
		time.Delete("streamed")
		time.Delete("completed")
		p.reopenAssistant(sessionID, assistantID)
		if truthy(d.Get("snapshot")) {
			snapshot := objOf(existing.Get("snapshot")).Clone()
			if snapshot == nil {
				snapshot = NewObj()
			}
			snapshot.Set("start", d.Get("snapshot"))
			existing.Set("snapshot", snapshot)
		}
		return
	}
	if current := p.openAssistant(sessionID); current != nil {
		current.Delete("retry")
		objOf(current.Get("time")).Set("completed", created)
		p.closeAssistant(sessionID, str(current.Get("id")))
	}
	row := obj(
		"id", assistantID,
		"type", "assistant",
		"agent", d.Get("agent"),
		"model", cloneValue(d.Get("model")),
		"metadata", cloneValue(event.Get("metadata")),
		"content", []any{},
	)
	if truthy(d.Get("snapshot")) {
		row.Set("snapshot", obj("start", d.Get("snapshot")))
	}
	row.Set("time", obj("created", created))
	p.insert(sessionID, row)
}

func (p *Projection) stepEnded(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	assistantID := str(d.Get("assistantMessageID"))
	p.editAssistant(sessionID, assistantID, func(a *Obj) {
		objOf(a.Get("time")).Set("completed", event.Get("created"))
		a.Set("finish", d.Get("finish"))
		a.Set("rawFinish", d.Get("rawFinish"))
		a.Set("providerState", cloneValue(d.Get("providerState")))
		a.Set("cost", d.Get("cost"))
		a.Set("tokens", cloneValue(d.Get("tokens")))
		if truthy(d.Get("snapshot")) {
			snapshot := objOf(a.Get("snapshot")).Clone()
			if snapshot == nil {
				snapshot = NewObj()
			}
			snapshot.Set("end", d.Get("snapshot"))
			a.Set("snapshot", snapshot)
		}
		p.closeAssistant(sessionID, assistantID)
	})
}

func (p *Projection) stepFailed(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	assistantID := str(d.Get("assistantMessageID"))
	p.editAssistant(sessionID, assistantID, func(a *Obj) {
		objOf(a.Get("time")).Set("completed", event.Get("created"))
		a.Set("finish", coalesce(d.Get("finish"), "error"))
		a.Set("rawFinish", d.Get("rawFinish"))
		a.Set("providerState", cloneValue(d.Get("providerState")))
		a.Set("error", cloneValue(d.Get("error")))
		a.Delete("retry")
		if !isUndefined(d.Get("cost")) && !isUndefined(d.Get("tokens")) {
			a.Set("cost", d.Get("cost"))
			a.Set("tokens", cloneValue(d.Get("tokens")))
		}
		p.closeAssistant(sessionID, assistantID)
	})
}

func (p *Projection) toolCalled(event *Obj) {
	d := objOf(event.Get("data"))
	p.editTool(str(d.Get("sessionID")), str(d.Get("assistantMessageID")), str(d.Get("id")), func(t *Obj) {
		objOf(t.Get("time")).Set("ran", event.Get("created"))
		t.Set("executed", d.Get("executed"))
		t.Set("providerState", cloneValue(d.Get("state")))
		t.Set("state", obj("status", "running", "input", cloneValue(d.Get("input")), "metadata", NewObj()))
	})
}

// executedAfterResult reproduces d.executed || t.executed === true.
func executedAfterResult(d, t *Obj) any {
	if truthy(d.Get("executed")) {
		return d.Get("executed")
	}
	previous, ok := t.Get("executed").(bool)
	return ok && previous
}

func (p *Projection) toolSuccess(event *Obj) {
	d := objOf(event.Get("data"))
	p.editTool(str(d.Get("sessionID")), str(d.Get("assistantMessageID")), str(d.Get("id")), func(t *Obj) {
		state := objOf(t.Get("state"))
		if str(state.Get("status")) != "running" {
			return
		}
		metadata := d.Get("metadata")
		if isUndefined(metadata) {
			metadata = state.Get("metadata")
		}
		t.Set("state", obj(
			"status", "completed",
			"input", cloneValue(state.Get("input")),
			"metadata", cloneValue(metadata),
			"content", cloneValue(d.Get("content")),
		))
		t.Set("executed", executedAfterResult(d, t))
		t.Set("providerResultState", cloneValue(d.Get("resultState")))
		objOf(t.Get("time")).Set("completed", event.Get("created"))
	})
}

func (p *Projection) toolFailed(event *Obj) {
	d := objOf(event.Get("data"))
	p.editTool(str(d.Get("sessionID")), str(d.Get("assistantMessageID")), str(d.Get("id")), func(t *Obj) {
		state := objOf(t.Get("state"))
		status := str(state.Get("status"))
		if status != "streaming" && status != "running" {
			return
		}
		var input = cloneValue(state.Get("input"))
		if _, ok := state.Get("input").(string); ok {
			input = NewObj()
		}
		metadata := d.Get("metadata")
		if isUndefined(metadata) {
			metadata = state.Get("metadata")
		}
		t.Set("state", obj(
			"status", "error",
			"error", cloneValue(d.Get("error")),
			"input", input,
			"metadata", cloneValue(metadata),
			"content", cloneValue(d.Get("content")),
		))
		t.Set("executed", executedAfterResult(d, t))
		t.Set("providerResultState", cloneValue(d.Get("resultState")))
		objOf(t.Get("time")).Set("completed", event.Get("created"))
	})
}

func (p *Projection) reasoningEnded(event *Obj) {
	d := objOf(event.Get("data"))
	p.editReasoning(str(d.Get("sessionID")), str(d.Get("assistantMessageID")), func(r *Obj) {
		r.Set("text", d.Get("text"))
		r.Set("time", obj(
			"created", coalesce(objOf(r.Get("time")).Get("created"), event.Get("created")),
			"completed", event.Get("created"),
		))
		if !isUndefined(d.Get("state")) {
			r.Set("state", cloneValue(d.Get("state")))
		}
	})
}

func (p *Projection) executionEnded(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	p.state.Active[sessionID] = "idle"
	if assistant := p.openAssistant(sessionID); assistant != nil {
		assistant.Delete("retry")
	}
	if str(event.Get("type")) == "session.execution.interrupted" && str(d.Get("reason")) == "shutdown" {
		return
	}
}

func (p *Projection) revertCommitted(sessionID, to string) {
	if info, ok := p.state.Info[sessionID]; ok {
		info.Delete("revert")
	}
	kept := []*Obj{}
	for _, item := range p.pending(sessionID) {
		if str(item.Get("id")) < to {
			kept = append(kept, item)
		}
	}
	p.state.Pending[sessionID] = kept
	list := p.state.Message[sessionID]
	for at, item := range list {
		if str(item.Get("id")) >= to {
			p.state.Message[sessionID] = list[:at]
			p.rebuildIndexes(sessionID)
			return
		}
	}
}

func (p *Projection) compactionStarted(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	if truthy(d.Get("inputID")) {
		p.state.Pending[sessionID] = filterOutID(p.pending(sessionID), str(d.Get("inputID")))
	}
	p.insert(sessionID, obj(
		"id", coalesce(d.Get("inputID"), eventMessageID(str(event.Get("id")))),
		"type", "compaction",
		"status", "running",
		"reason", d.Get("reason"),
		"summary", "",
		"recent", coalesce(d.Get("recent"), ""),
		"time", obj("created", event.Get("created")),
	))
}

// compactionFields is the assign payload upstream builds for a finished
// compaction, in source order.
func compactionFields(d *Obj) []any {
	return []any{
		"status", "completed",
		"reason", d.Get("reason"),
		"model", cloneValue(d.Get("model")),
		"providerState", cloneValue(d.Get("providerState")),
		"providerContext", cloneValue(d.Get("providerContext")),
		"summary", d.Get("text"),
		"recent", d.Get("recent"),
		"cost", d.Get("cost"),
		"tokens", cloneValue(d.Get("tokens")),
	}
}

func (p *Projection) compactionEnded(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	fields := compactionFields(d)
	if row, at := p.runningCompaction(sessionID); row != nil {
		for i := 0; i+1 < len(fields); i += 2 {
			setDefined(row, str(fields[i]), fields[i+1])
		}
		p.endCompaction(sessionID, at)
		return
	}
	inserted := obj("id", eventMessageID(str(event.Get("id"))), "type", "compaction")
	for i := 0; i+1 < len(fields); i += 2 {
		inserted.Set(str(fields[i]), fields[i+1])
	}
	inserted.Set("time", obj("created", event.Get("created")))
	p.insert(sessionID, inserted)
}

func (p *Projection) compactionFailed(event *Obj) {
	d := objOf(event.Get("data"))
	sessionID := str(d.Get("sessionID"))
	if truthy(d.Get("inputID")) {
		p.state.Pending[sessionID] = filterOutID(p.pending(sessionID), str(d.Get("inputID")))
	}
	p.messages(sessionID)
	row, at := p.runningCompaction(sessionID)

	id := row.Get("id")
	if isUndefined(id) {
		id = coalesce(d.Get("inputID"), eventMessageID(str(event.Get("id"))))
	}
	defaultError := obj("type", "compaction.failed", "message", "Compaction failed before recording an error")
	metadata := event.Get("metadata")
	var timeValue any = obj("created", event.Get("created"))
	if row != nil {
		metadata = row.Get("metadata")
		timeValue = cloneValue(row.Get("time"))
	}
	failed := obj(
		"id", id,
		"type", "compaction",
		"status", "failed",
		"reason", coalesce(d.Get("reason"), "manual"),
		"error", cloneValue(coalesce(d.Get("error"), defaultError)),
		"metadata", cloneValue(metadata),
		"cost", d.Get("cost"),
		"tokens", cloneValue(d.Get("tokens")),
		"time", timeValue,
	)
	if row != nil {
		p.state.Message[sessionID][at] = failed
		p.endCompaction(sessionID, at)
		return
	}
	p.insert(sessionID, failed)
}

func (p *Projection) removeForm(sessionID, id string, ref *Obj) {
	forms, ok := p.state.Form[sessionID]
	if !ok {
		return
	}
	kept := []*Obj{}
	for _, form := range forms {
		if str(form.Get("id")) != id {
			kept = append(kept, form)
			continue
		}
		location := objOf(form.Get("location"))
		if sessionID == "global" && ref != nil && location != nil &&
			jsonKey(location.Get("directory"), location.Get("workspaceID")) != jsonKey(ref.Get("directory"), ref.Get("workspaceID")) {
			kept = append(kept, form)
		}
	}
	p.state.Form[sessionID] = kept
}
