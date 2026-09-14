package sessionstate

import (
	"encoding/json"
	"testing"
)

func TestPermissionReplyRemainsInHistory(t *testing.T) {
	projection := New(NewProjectionState())
	projection.Apply(nativeEvent("asked", "permission.asked", "sessionID", "ses", "id", "approval", "permission", "command"))
	projection.Apply(nativeEvent("replied", "permission.replied", "sessionID", "ses", "requestID", "approval", "reply", "denied"))
	if len(projection.state.Permission["ses"]) != 1 {
		t.Fatalf("permission history = %#v", projection.state.Permission["ses"])
	}
	record := projection.state.Permission["ses"][0]
	if str(record.Get("reply")) != "denied" || record.Get("repliedAt") == nil {
		t.Fatalf("permission decision = %s at %#v", str(record.Get("reply")), record.Get("repliedAt"))
	}
	if _, ok := record.Get("repliedAt").(json.Number); !ok {
		t.Fatalf("repliedAt type = %T", record.Get("repliedAt"))
	}
}
