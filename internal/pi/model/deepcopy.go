package model

// CloneSessionEntry returns a deep copy of a session entry.
func CloneSessionEntry(entry SessionEntry) SessionEntry {
	switch e := entry.(type) {
	case SessionMessageEntry:
		out := e
		if e.ParentID != nil {
			p := *e.ParentID
			out.ParentID = &p
		}
		out.Message = CloneMessage(e.Message)
		return out
	case *SessionMessageEntry:
		if e == nil {
			return (*SessionMessageEntry)(nil)
		}
		out := CloneSessionEntry(*e).(SessionMessageEntry)
		return &out
	case ThinkingLevelChangeEntry:
		return e
	case *ThinkingLevelChangeEntry:
		if e == nil {
			return (*ThinkingLevelChangeEntry)(nil)
		}
		v := *e
		return &v
	case ModelChangeEntry:
		return e
	case *ModelChangeEntry:
		if e == nil {
			return (*ModelChangeEntry)(nil)
		}
		v := *e
		return &v
	case UsageEntry:
		return e
	case *UsageEntry:
		if e == nil {
			return (*UsageEntry)(nil)
		}
		v := *e
		return &v
	case CompactionEntry:
		out := e
		if e.ParentID != nil {
			p := *e.ParentID
			out.ParentID = &p
		}
		out.Details = cloneValue(e.Details)
		if e.Usage != nil {
			u := *e.Usage
			out.Usage = &u
		}
		if e.SystemMessage != nil {
			s := e.SystemMessage.Clone()
			out.SystemMessage = &s
		}
		return out
	case *CompactionEntry:
		if e == nil {
			return (*CompactionEntry)(nil)
		}
		out := CloneSessionEntry(*e).(CompactionEntry)
		return &out
	case BranchSummaryEntry:
		out := e
		if e.ParentID != nil {
			p := *e.ParentID
			out.ParentID = &p
		}
		out.Details = cloneValue(e.Details)
		if e.Usage != nil {
			u := *e.Usage
			out.Usage = &u
		}
		return out
	case *BranchSummaryEntry:
		if e == nil {
			return (*BranchSummaryEntry)(nil)
		}
		out := CloneSessionEntry(*e).(BranchSummaryEntry)
		return &out
	case CustomEntry:
		out := e
		if e.ParentID != nil {
			p := *e.ParentID
			out.ParentID = &p
		}
		out.Data = cloneValue(e.Data)
		return out
	case *CustomEntry:
		if e == nil {
			return (*CustomEntry)(nil)
		}
		out := CloneSessionEntry(*e).(CustomEntry)
		return &out
	case CustomMessageEntry:
		out := e
		if e.ParentID != nil {
			p := *e.ParentID
			out.ParentID = &p
		}
		out.Content = e.Content.Clone()
		out.Details = cloneValue(e.Details)
		return out
	case *CustomMessageEntry:
		if e == nil {
			return (*CustomMessageEntry)(nil)
		}
		out := CloneSessionEntry(*e).(CustomMessageEntry)
		return &out
	case ContextEditEntry:
		out := e
		if e.ParentID != nil {
			p := *e.ParentID
			out.ParentID = &p
		}
		if e.Replacement != nil {
			r := ContextEditableContent{Content: e.Replacement.Content.Clone()}
			out.Replacement = &r
		}
		return out
	case *ContextEditEntry:
		if e == nil {
			return (*ContextEditEntry)(nil)
		}
		out := CloneSessionEntry(*e).(ContextEditEntry)
		return &out
	case LabelEntry:
		out := e
		if e.ParentID != nil {
			p := *e.ParentID
			out.ParentID = &p
		}
		if e.Label != nil {
			l := *e.Label
			out.Label = &l
		}
		return out
	case *LabelEntry:
		if e == nil {
			return (*LabelEntry)(nil)
		}
		out := CloneSessionEntry(*e).(LabelEntry)
		return &out
	case SessionInfoEntry:
		return e
	case *SessionInfoEntry:
		if e == nil {
			return (*SessionInfoEntry)(nil)
		}
		v := *e
		return &v
	default:
		return entry
	}
}

// CloneSessionEntries returns deep copies of a session entry slice.
func CloneSessionEntries(entries []SessionEntry) []SessionEntry {
	if entries == nil {
		return nil
	}
	out := make([]SessionEntry, len(entries))
	for i, e := range entries {
		out[i] = CloneSessionEntry(e)
	}
	return out
}
