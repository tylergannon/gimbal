package claude

import (
	"context"
	"testing"
)

// TestCloseIsIdempotent covers acceptance item 4: closing a Claude session
// twice returns nil both times. CreateSession only mints an id (Claude
// Code's process is launched per turn, never held by the adapter), so this
// needs no live process either.
func TestCloseIsIdempotent(t *testing.T) {
	ad := New().(*adapter)
	id, err := ad.CreateSession(context.Background(), "model", ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := ad.Close(context.Background(), id); err != nil {
		t.Fatalf("first Close = %v, want nil", err)
	}
	if err := ad.Close(context.Background(), id); err != nil {
		t.Fatalf("second Close = %v, want nil", err)
	}
}

// TestSteerWithNoStreamLiveIsDropped: a steer on a session with no turn
// running has no live SDK stream to send on. The adapter reports it
// dropped, false and no error, without launching Claude Code. A steer on an
// unknown session is the error it always was.
func TestSteerWithNoStreamLiveIsDropped(t *testing.T) {
	ad := New().(*adapter)
	id, err := ad.CreateSession(context.Background(), "model", ".")
	if err != nil {
		t.Fatal(err)
	}
	landed, err := ad.Steer(context.Background(), id, "change course")
	if err != nil {
		t.Fatalf("Steer = %v, want nil: a dropped steer is not an error", err)
	}
	if landed {
		t.Fatal("Steer landed with no stream live")
	}
	if _, err := ad.Steer(context.Background(), "unknown-session", "change course"); err == nil {
		t.Fatal("Steer on an unknown session = nil, want an error")
	}
}
