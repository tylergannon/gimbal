package main

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestKeychainStoresReloadsScopesAndDeletesSession(t *testing.T) {
	account := fmt.Sprintf("https://keychain-test-%d.invalid", time.Now().UnixNano())
	store := keychainStore{account: account}
	t.Cleanup(func() {
		if err := store.save(nil); err != nil {
			t.Error(err)
		}
	})
	raw := []byte(`{"name":"gimbal-session","value":"test-only","expires":"2030-01-01T00:00:00Z"}`)
	if err := store.save(raw); err != nil {
		t.Fatal(err)
	}
	reopened := keychainStore{account: account}
	if got, err := reopened.load(); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("Keychain did not restore credential: %v", err)
	}
	other := keychainStore{account: account + ":443"}
	if got, err := other.load(); err != nil || len(got) != 0 {
		t.Fatalf("different server received saved credential: %v", err)
	}
	if err := reopened.save(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := reopened.load(); err != nil || len(got) != 0 {
		t.Fatalf("logout retained credential: %v", err)
	}
}
