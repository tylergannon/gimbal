package sessionstate

import "testing"

func TestConnectionNoticeSurvivesReloadWithoutAnAssistant(t *testing.T) {
	p := New(NewProjectionState())
	p.Apply(nativeEvent("lost", "session.connection", "sessionID", "ses", "assistantMessageID", "connection.1", "state", "paused", "message", "Inspect partial work before resume", "attempt", 3))
	p = New(p.Snapshot().State)
	p.Apply(nativeEvent("restored", "session.connection", "sessionID", "ses", "assistantMessageID", "connection.1", "state", "recovered", "message", "Codex connection recovered", "attempt", 1))
	rows := p.state.Message["ses"]
	if len(rows) != 1 || str(rows[0].Get("state")) != "recovered" || str(rows[0].Get("message")) != "Codex connection recovered" {
		t.Fatalf("connection history = %#v", rows)
	}
}
