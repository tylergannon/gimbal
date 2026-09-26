package config

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeCredentialsMaskStoredCredentials(t *testing.T) {
	storage := NewInMemoryCredentialStore(map[string]Credential{
		"anthropic": {Type: CredentialAPIKey, Key: "stored-key"},
	})
	credentials := NewRuntimeCredentials(storage)
	credentials.SetRuntimeAPIKey("anthropic", "runtime-key")

	got, err := credentials.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Key != "runtime-key" {
		t.Fatalf("runtime read = %+v; want runtime-key", got)
	}
	stored, err := storage.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Key != "stored-key" {
		t.Fatalf("stored read = %+v; want stored-key", stored)
	}

	credentials.RemoveRuntimeAPIKey("anthropic")
	got, err = credentials.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Key != "stored-key" {
		t.Fatalf("after removal = %+v; want stored-key", got)
	}
}

func TestRuntimeCredentialsEnumerationMergesOverrides(t *testing.T) {
	storage := NewInMemoryCredentialStore(map[string]Credential{
		"anthropic": {Type: CredentialOAuth, Access: "access", Refresh: "refresh", Expires: 1},
	})
	credentials := NewRuntimeCredentials(storage)
	credentials.SetRuntimeAPIKey("anthropic", "runtime-key")
	credentials.SetRuntimeAPIKey("openai", "other-runtime-key")

	got, err := credentials.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []CredentialInfo{
		{ProviderID: "anthropic", Type: CredentialAPIKey},
		{ProviderID: "openai", Type: CredentialAPIKey},
	}
	if len(got) != len(want) {
		t.Fatalf("list = %+v; want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("list[%d] = %+v; want %+v", i, got[i], want[i])
		}
	}
}

func TestRuntimeCredentialsForwardsSignals(t *testing.T) {
	type signalKey struct{}
	ctx := context.WithValue(context.Background(), signalKey{}, true)
	var received []context.Context
	storage := &recordingCredentialStore{received: &received}
	credentials := NewRuntimeCredentials(storage)

	if _, err := credentials.Read(ctx, "anthropic"); err != nil {
		t.Fatal(err)
	}
	if _, err := credentials.List(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := credentials.Modify(ctx, "anthropic", func(*Credential) (*Credential, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if err := credentials.Delete(ctx, "anthropic"); err != nil {
		t.Fatal(err)
	}
	if len(received) != 4 {
		t.Fatalf("received %d contexts; want 4", len(received))
	}
	for _, got := range received {
		if got.Value(signalKey{}) != true {
			t.Fatal("context was not forwarded")
		}
	}
}

func TestRuntimeCredentialsKeepsOverrideWhenDeleteCancelled(t *testing.T) {
	cancelled := errors.New("cancelled")
	storage := NewInMemoryCredentialStore(map[string]Credential{
		"anthropic": {Type: CredentialAPIKey, Key: "stored-key"},
	})
	failing := &failingDeleteStore{CredentialStore: storage, err: cancelled}
	credentials := NewRuntimeCredentials(failing)
	credentials.SetRuntimeAPIKey("anthropic", "runtime-key")

	if err := credentials.Delete(context.Background(), "anthropic"); !errors.Is(err, cancelled) {
		t.Fatalf("delete error = %v; want cancelled", err)
	}
	got, err := credentials.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Key != "runtime-key" {
		t.Fatalf("read after cancelled delete = %+v; want runtime-key", got)
	}
}

func TestRuntimeCredentialsDeleteClearsBoth(t *testing.T) {
	storage := NewInMemoryCredentialStore(map[string]Credential{
		"anthropic": {Type: CredentialAPIKey, Key: "stored-key"},
	})
	credentials := NewRuntimeCredentials(storage)
	credentials.SetRuntimeAPIKey("anthropic", "runtime-key")

	if err := credentials.Delete(context.Background(), "anthropic"); err != nil {
		t.Fatal(err)
	}
	got, err := credentials.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("read after delete = %+v; want nil", got)
	}
	list, err := credentials.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list after delete = %+v; want empty", list)
	}
}

type recordingCredentialStore struct {
	received *[]context.Context
}

func (s *recordingCredentialStore) Read(ctx context.Context, _ string) (*Credential, error) {
	*s.received = append(*s.received, ctx)
	return nil, nil
}

func (s *recordingCredentialStore) List(ctx context.Context) ([]CredentialInfo, error) {
	*s.received = append(*s.received, ctx)
	return nil, nil
}

func (s *recordingCredentialStore) Modify(ctx context.Context, _ string, _ func(*Credential) (*Credential, error)) (*Credential, error) {
	*s.received = append(*s.received, ctx)
	return nil, nil
}

func (s *recordingCredentialStore) Delete(ctx context.Context, _ string) error {
	*s.received = append(*s.received, ctx)
	return nil
}

type failingDeleteStore struct {
	CredentialStore
	err error
}

func (s *failingDeleteStore) Delete(context.Context, string) error { return s.err }
