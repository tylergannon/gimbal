// Ported from OpenCode c55ee2a8152603f04a409163bd3edf79c425fbd7. MIT,
// Copyright (c) 2025 opencode. See NOTICE.md beside this file.

package sessionstate

import (
	"maps"
	"slices"
	"strings"
)

// ProjectionState is the session slice of the upstream client store, using
// the upstream keys. Every value is native JSON: this port reduces native
// events and holds no Gimbal identity.
type ProjectionState struct {
	Info       map[string]*Obj     `json:"info"`
	Family     map[string][]string `json:"family"`
	Active     map[string]string   `json:"active"`
	Message    map[string][]*Obj   `json:"message"`
	Pending    map[string][]*Obj   `json:"pending"`
	Permission map[string][]*Obj   `json:"permission"`
	Form       map[string][]*Obj   `json:"form"`
}

// NewProjectionState returns the empty slice, with every bucket present so it
// encodes as an object rather than null.
func NewProjectionState() ProjectionState {
	return ProjectionState{
		Info:       map[string]*Obj{},
		Family:     map[string][]string{},
		Active:     map[string]string{},
		Message:    map[string][]*Obj{},
		Pending:    map[string][]*Obj{},
		Permission: map[string][]*Obj{},
		Form:       map[string][]*Obj{},
	}
}

func (s *ProjectionState) ensure() {
	if s.Info == nil {
		s.Info = map[string]*Obj{}
	}
	if s.Family == nil {
		s.Family = map[string][]string{}
	}
	if s.Active == nil {
		s.Active = map[string]string{}
	}
	if s.Message == nil {
		s.Message = map[string][]*Obj{}
	}
	if s.Pending == nil {
		s.Pending = map[string][]*Obj{}
	}
	if s.Permission == nil {
		s.Permission = map[string][]*Obj{}
	}
	if s.Form == nil {
		s.Form = map[string][]*Obj{}
	}
	for key, list := range s.Family {
		if list == nil {
			s.Family[key] = []string{}
		}
	}
	for _, bucket := range []map[string][]*Obj{s.Message, s.Pending, s.Permission, s.Form} {
		for key, list := range bucket {
			if list == nil {
				bucket[key] = []*Obj{}
			}
		}
	}
}

// Clone detaches the state so an observer cannot reach reducer-owned rows.
func (s ProjectionState) Clone() ProjectionState {
	out := NewProjectionState()
	for key, value := range s.Info {
		out.Info[key] = value.Clone()
	}
	for key, value := range s.Family {
		copied := make([]string, len(value))
		copy(copied, value)
		out.Family[key] = copied
	}
	maps.Copy(out.Active, s.Active)
	for _, pair := range []struct {
		from map[string][]*Obj
		to   map[string][]*Obj
	}{{s.Message, out.Message}, {s.Pending, out.Pending}, {s.Permission, out.Permission}, {s.Form, out.Form}} {
		for key, list := range pair.from {
			copied := make([]*Obj, len(list))
			for i, item := range list {
				copied[i] = item.Clone()
			}
			pair.to[key] = copied
		}
	}
	return out
}

// sessionIndex is the private, rebuildable access path into one session's
// message list. It replaces the upstream findLast scans over the whole
// transcript: every selection the reducer makes is bounded by open rows.
// Positions are valid because every mutation that shifts them rebuilds.
type sessionIndex struct {
	pos                map[string]int
	openAssistants     []int
	runningCompactions []int
	shells             map[string]int
}

// Projection reduces native session events into observable state.
type Projection struct {
	state ProjectionState

	index         map[string]*sessionIndex
	latestText    map[string]*Obj
	openReasoning map[string][]*Obj
	tools         map[string]*Obj
}

// New returns a projection seeded with state, which may be the zero value.
func New(state ProjectionState) *Projection {
	state.ensure()
	p := &Projection{state: state.Clone(), index: map[string]*sessionIndex{}, latestText: map[string]*Obj{}, openReasoning: map[string][]*Obj{}, tools: map[string]*Obj{}}
	for sessionID := range p.state.Message {
		p.rebuildIndexes(sessionID)
	}
	return p
}

func (p *Projection) messages(sessionID string) []*Obj {
	if _, ok := p.state.Message[sessionID]; !ok {
		p.state.Message[sessionID] = []*Obj{}
	}
	return p.state.Message[sessionID]
}

func (p *Projection) indexFor(sessionID string) *sessionIndex {
	if existing, ok := p.index[sessionID]; ok {
		return existing
	}
	p.rebuildIndexes(sessionID)
	return p.index[sessionID]
}

func partKey(sessionID, messageID, suffix string) string {
	return sessionID + "\x00" + messageID + "\x00" + suffix
}

func (p *Projection) rebuildIndexes(sessionID string) {
	prefix := sessionID + "\x00"
	for key := range p.latestText {
		if strings.HasPrefix(key, prefix) {
			delete(p.latestText, key)
		}
	}
	for key := range p.openReasoning {
		if strings.HasPrefix(key, prefix) {
			delete(p.openReasoning, key)
		}
	}
	for key := range p.tools {
		if strings.HasPrefix(key, prefix) {
			delete(p.tools, key)
		}
	}
	index := &sessionIndex{pos: map[string]int{}, shells: map[string]int{}}
	for at, item := range p.state.Message[sessionID] {
		index.pos[str(item.Get("id"))] = at
		p.registerRow(sessionID, index, item, at)
	}
	p.index[sessionID] = index
}

// registerRow records one message row in the private access paths. Each path
// answers exactly one upstream findLast: the latest text part, the stack of
// still-open reasoning parts, the last part carrying a tool ID, the open
// assistants, the running compactions and the last row per shell ID.
func (p *Projection) registerRow(sessionID string, index *sessionIndex, item *Obj, at int) {
	id := str(item.Get("id"))
	switch str(item.Get("type")) {
	case "assistant":
		if !truthy(objOf(item.Get("time")).Get("completed")) {
			index.openAssistants = append(index.openAssistants, at)
		}
		for _, entry := range arrOf(item.Get("content")) {
			part := objOf(entry)
			if part == nil {
				continue
			}
			switch str(part.Get("type")) {
			case "text":
				p.latestText[partKey(sessionID, id, "")] = part
			case "reasoning":
				if !truthy(objOf(part.Get("time")).Get("completed")) {
					key := partKey(sessionID, id, "")
					p.openReasoning[key] = append(p.openReasoning[key], part)
				}
			case "tool":
				p.tools[partKey(sessionID, id, str(part.Get("id")))] = part
			}
		}
	case "compaction":
		if str(item.Get("status")) == "running" {
			index.runningCompactions = append(index.runningCompactions, at)
		}
	case "shell":
		index.shells[str(item.Get("shellID"))] = at
	}
}

func (p *Projection) message(sessionID, id string) *Obj {
	index := p.indexFor(sessionID)
	at, ok := index.pos[id]
	if !ok {
		return nil
	}
	list := p.state.Message[sessionID]
	if at >= len(list) {
		return nil
	}
	return list[at]
}

// openAssistant answers upstream's findLast over assistants with no completed
// time, without touching cold history.
func (p *Projection) openAssistant(sessionID string) *Obj {
	index, ok := p.index[sessionID]
	if !ok || len(index.openAssistants) == 0 {
		return nil
	}
	return p.state.Message[sessionID][index.openAssistants[len(index.openAssistants)-1]]
}

// runningCompaction answers upstream's findLast over running compaction rows,
// returning the row and its position.
func (p *Projection) runningCompaction(sessionID string) (*Obj, int) {
	index, ok := p.index[sessionID]
	if !ok || len(index.runningCompactions) == 0 {
		return nil, -1
	}
	at := index.runningCompactions[len(index.runningCompactions)-1]
	return p.state.Message[sessionID][at], at
}

func (p *Projection) shell(sessionID, shellID string) *Obj {
	index, ok := p.index[sessionID]
	if !ok {
		return nil
	}
	at, ok := index.shells[shellID]
	if !ok {
		return nil
	}
	return p.state.Message[sessionID][at]
}

// closeAssistant drops a row from the open set once its completed time is set.
func (p *Projection) closeAssistant(sessionID, id string) {
	index, ok := p.index[sessionID]
	if !ok {
		return
	}
	at, ok := index.pos[id]
	if !ok {
		return
	}
	for i, open := range index.openAssistants {
		if open == at {
			index.openAssistants = append(index.openAssistants[:i], index.openAssistants[i+1:]...)
			return
		}
	}
}

func (p *Projection) reopenAssistant(sessionID, id string) {
	index, ok := p.index[sessionID]
	if !ok {
		return
	}
	at, ok := index.pos[id]
	if !ok {
		return
	}
	if slices.Contains(index.openAssistants, at) {
		return
	}
	index.openAssistants = append(index.openAssistants, at)
	for i := len(index.openAssistants) - 1; i > 0 && index.openAssistants[i-1] > index.openAssistants[i]; i-- {
		index.openAssistants[i-1], index.openAssistants[i] = index.openAssistants[i], index.openAssistants[i-1]
	}
}

func (p *Projection) endCompaction(sessionID string, at int) {
	index, ok := p.index[sessionID]
	if !ok {
		return
	}
	for i, running := range index.runningCompactions {
		if running == at {
			index.runningCompactions = append(index.runningCompactions[:i], index.runningCompactions[i+1:]...)
			return
		}
	}
}

// insert appends a row unless its ID is already present, matching upstream's
// message index guard.
func (p *Projection) insert(sessionID string, item *Obj) {
	list := p.messages(sessionID)
	index := p.indexFor(sessionID)
	id := str(item.Get("id"))
	if _, ok := index.pos[id]; ok {
		return
	}
	copied := item.Clone()
	list = append(list, copied)
	p.state.Message[sessionID] = list
	at := len(list) - 1
	index.pos[id] = at
	p.registerRow(sessionID, index, copied, at)
}

func (p *Projection) upsertMessage(sessionID string, item *Obj) {
	list := p.messages(sessionID)
	at := -1
	if index, ok := p.index[sessionID]; ok {
		if found, ok := index.pos[str(item.Get("id"))]; ok {
			at = found
		}
	}
	if at < 0 {
		p.state.Message[sessionID] = append(list, item.Clone())
	} else {
		list[at] = item.Clone()
	}
	p.rebuildIndexes(sessionID)
}

func (p *Projection) editAssistant(sessionID, id string, edit func(*Obj)) {
	item := p.message(sessionID, id)
	if item != nil && str(item.Get("type")) == "assistant" {
		edit(item)
	}
}

func (p *Projection) editText(sessionID, id string, edit func(*Obj)) {
	p.editAssistant(sessionID, id, func(*Obj) {
		if part, ok := p.latestText[partKey(sessionID, id, "")]; ok {
			edit(part)
		}
	})
}

// editReasoning targets the latest reasoning part that is still open. The
// stack answers upstream's findLast exactly: content is only appended, and
// only the part this selector returns is ever completed, so completing one
// uncovers the one before it.
func (p *Projection) editReasoning(sessionID, id string, edit func(*Obj)) {
	p.editAssistant(sessionID, id, func(*Obj) {
		key := partKey(sessionID, id, "")
		stack := p.openReasoning[key]
		if len(stack) == 0 {
			return
		}
		part := stack[len(stack)-1]
		edit(part)
		if !truthy(objOf(part.Get("time")).Get("completed")) {
			return
		}
		if stack = stack[:len(stack)-1]; len(stack) == 0 {
			delete(p.openReasoning, key)
			return
		}
		p.openReasoning[key] = stack
	})
}

func (p *Projection) editTool(sessionID, id, toolID string, edit func(*Obj)) {
	p.editAssistant(sessionID, id, func(*Obj) {
		if part, ok := p.tools[partKey(sessionID, id, toolID)]; ok {
			edit(part)
		}
	})
}

func (p *Projection) removeSession(sessionID string) {
	delete(p.state.Info, sessionID)
	delete(p.state.Active, sessionID)
	delete(p.state.Message, sessionID)
	delete(p.state.Pending, sessionID)
	delete(p.state.Permission, sessionID)
	delete(p.state.Form, sessionID)
	for root, members := range p.state.Family {
		kept := members[:0]
		for _, member := range members {
			if member != sessionID {
				kept = append(kept, member)
			}
		}
		if len(kept) == 0 {
			delete(p.state.Family, root)
			continue
		}
		p.state.Family[root] = kept
	}
	delete(p.index, sessionID)
	prefix := sessionID + "\x00"
	for _, parts := range []map[string]*Obj{p.latestText, p.tools} {
		for key := range parts {
			if strings.HasPrefix(key, prefix) {
				delete(parts, key)
			}
		}
	}
	for key := range p.openReasoning {
		if strings.HasPrefix(key, prefix) {
			delete(p.openReasoning, key)
		}
	}
}

func filterOutID(list []*Obj, id string) []*Obj {
	out := make([]*Obj, 0, len(list))
	for _, item := range list {
		if str(item.Get("id")) != id {
			out = append(out, item)
		}
	}
	return out
}

func indexOfID(list []*Obj, id string) int {
	for at, item := range list {
		if str(item.Get("id")) == id {
			return at
		}
	}
	return -1
}
