package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/auth"
)

func openStore(t *testing.T, dir string) *store {
	t.Helper()
	raw, _ := json.Marshal(configuration{Directory: dir})
	u, err := connect(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	s := u.(*store)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUsersConcurrentProvisioningAndRestart(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "auth")
	s := openStore(t, dir)
	identity := auth.Identity{Issuer: "https://github.com", Subject: "123456", Name: "Tyler"}
	ids := make(chan string, 30)
	var workers sync.WaitGroup
	for range 30 {
		workers.Go(func() {
			u, err := s.CreateUser(ctx, identity)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- u.ID()
		})
	}
	workers.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id == "" {
			id = got
		}
		if id != got || id == "" {
			t.Fatalf("concurrent provisioning IDs differ: %q, %q", id, got)
		}
	}
	if _, err := s.UserForIdentity(ctx, identity.Issuer, "unprovisioned"); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("unknown identity admitted: %v", err)
	}
	for _, separate := range []auth.Identity{{Issuer: "a:b", Subject: "c", Name: "Tyler"}, {Issuer: "a", Subject: "b:c", Name: "Tyler"}} {
		if u, err := s.CreateUser(ctx, separate); err != nil || u.ID() == id {
			t.Fatalf("distinct identities linked: %v", err)
		}
	}
	session := auth.Session{Hash: strings.Repeat("a", 64), UserID: id, ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if err := s.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, session); err == nil {
		t.Fatal("duplicate session hash accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openStore(t, dir)
	u, err := s.UserForIdentity(ctx, identity.Issuer, identity.Subject)
	if err != nil || u.ID() != id || u.Name() != identity.Name {
		t.Fatalf("user did not survive restart: %v", err)
	}
	got, err := s.Session(ctx, session.Hash)
	if err != nil || got != session {
		t.Fatalf("session did not survive restart: %+v, %v", got, err)
	}
	got.UserID = "mutable copy"
	if original, _ := s.Session(ctx, session.Hash); original.UserID != id {
		t.Fatal("returned session mutates the stored value")
	}
	if err := s.DeleteUser(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, id); err != nil {
		t.Fatal("user deletion is not idempotent:", err)
	}
	if _, err := s.User(ctx, id); !errors.Is(err, auth.ErrNotFound) {
		t.Fatal("deleted user still present:", err)
	}
	if _, err := s.Session(ctx, session.Hash); !errors.Is(err, auth.ErrNotFound) {
		t.Fatal("deleted user's session survived:", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openStore(t, dir)
	u, err = s.CreateUser(ctx, identity)
	if err != nil || u.ID() == id {
		t.Fatalf("recreated user reused old ID: %v", err)
	}
	if err := s.CreateSession(ctx, auth.Session{Hash: strings.Repeat("b", 64), UserID: id, ExpiresAt: time.Now().Add(time.Hour)}); !errors.Is(err, auth.ErrNotFound) {
		t.Fatal("session issued to deleted user:", err)
	}
	for _, path := range []string{dir, filepath.Join(dir, "state.json"), filepath.Join(dir, ".lock")} {
		info, err := os.Stat(path)
		want := os.FileMode(0600)
		if path == dir {
			want = 0700
		}
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("unsafe state permissions at %s: %v", path, err)
		}
	}
}

func TestSessionRevocationIsDurable(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "auth")
	s := openStore(t, dir)
	u, err := s.CreateUser(ctx, auth.Identity{Issuer: "github", Subject: "1"})
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("c", 64)
	if err := s.CreateSession(ctx, auth.Session{Hash: hash, UserID: u.ID(), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s = openStore(t, dir)
	if _, err := s.Session(ctx, hash); !errors.Is(err, auth.ErrNotFound) {
		t.Fatal("revoked session survived restart:", err)
	}
}

func TestStoreOwnershipContextsAndUnavailableState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "auth")
	s := openStore(t, dir)
	raw, _ := json.Marshal(configuration{Directory: dir})
	if _, err := connect(context.Background(), raw); err == nil {
		t.Fatal("second store owner acquired directory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.CreateUser(ctx, auth.Identity{Issuer: "github", Subject: "1"}); !errors.Is(err, context.Canceled) {
		t.Fatal("CreateUser ignored canceled context:", err)
	}
	if _, err := s.UserForIdentity(ctx, "github", "1"); !errors.Is(err, context.Canceled) {
		t.Fatal("UserForIdentity ignored canceled context:", err)
	}
	if _, err := s.User(ctx, "u1"); !errors.Is(err, context.Canceled) {
		t.Fatal("User ignored canceled context:", err)
	}
	if err := s.DeleteUser(ctx, "u1"); !errors.Is(err, context.Canceled) {
		t.Fatal("DeleteUser ignored canceled context:", err)
	}
	if err := s.CreateSession(ctx, auth.Session{}); !errors.Is(err, context.Canceled) {
		t.Fatal("CreateSession ignored canceled context:", err)
	}
	if _, err := s.Session(ctx, "hash"); !errors.Is(err, context.Canceled) {
		t.Fatal("Session ignored canceled context:", err)
	}
	if err := s.DeleteSession(ctx, "hash"); !errors.Is(err, context.Canceled) {
		t.Fatal("DeleteSession ignored canceled context:", err)
	}
	<-s.gate
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := s.User(ctx, "u1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("waiting for concurrent mutation ignored context:", err)
	}
	s.release()
	if err := os.Remove(filepath.Join(dir, "state.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.User(context.Background(), "u1"); err == nil || errors.Is(err, auth.ErrNotFound) {
		t.Fatal("lost state did not return operational failure:", err)
	}
}

func TestExistingMissingMalformedOrInconsistentStateFails(t *testing.T) {
	for _, body := range []string{
		"", "not JSON", `{"version":1,"nextUser":0,"users":null,"sessions":{}}`,
		`{"version":1,"nextUser":1,"users":{"u1":{"id":"wrong","issuer":"github","subject":"1"}},"sessions":{}}`,
		`{"version":1,"nextUser":2,"users":{"u1":{"id":"u1","issuer":"github","subject":"1"},"u2":{"id":"u2","issuer":"github","subject":"1"}},"sessions":{}}`,
		`{"version":1,"nextUser":0,"users":{},"sessions":{"invalid":{"Hash":"invalid","UserID":"missing","ExpiresAt":"2030-01-01T00:00:00Z"}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			if body != "" {
				if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			raw, _ := json.Marshal(configuration{Directory: dir})
			if _, err := connect(context.Background(), raw); err == nil {
				t.Fatal("invalid existing store initialized or admitted")
			}
		})
	}
}

func TestMutationSurvivesImmediateProcessExit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "auth")
	command := exec.Command(os.Args[0], "-test.run=^TestFilesCrashHelper$")
	command.Env = append(os.Environ(), "GIMBAL_AUTH_FILES_CRASH_DIRECTORY="+dir)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("writer process: %v: %s", err, out)
	}
	s := openStore(t, dir)
	u, err := s.UserForIdentity(context.Background(), "github", "1")
	if err != nil || u.ID() != "u1" {
		t.Fatal("successful user creation lost after process exit:", err)
	}
	if _, err := s.Session(context.Background(), strings.Repeat("d", 64)); err != nil {
		t.Fatal("successful session creation lost after process exit:", err)
	}
}

func TestFilesCrashHelper(t *testing.T) {
	dir := os.Getenv("GIMBAL_AUTH_FILES_CRASH_DIRECTORY")
	if dir == "" {
		return
	}
	s := openStore(t, dir)
	u, err := s.CreateUser(context.Background(), auth.Identity{Issuer: "github", Subject: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(context.Background(), auth.Session{Hash: strings.Repeat("d", 64), UserID: u.ID(), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	os.Exit(0) // The process ends without releasing the lock or closing the store.
}

func TestRegisterFilesPlugin(t *testing.T) {
	var r gimbal.Registry
	if err := Register(&r); err != nil {
		t.Fatal(err)
	}
	d, ok := r.Users("files")
	if !ok || d.Connect == nil || d.ValidateConfiguration == nil {
		t.Fatal("files plugin not registered")
	}
	for _, raw := range []string{`{}`, `{"directory":"relative"}`, `{"directory":"/"}`, `{"directory":"/tmp/auth","unexpected":true}`, `null`, `{} {}`} {
		if d.ValidateConfiguration(json.RawMessage(raw)) == nil {
			t.Fatal("invalid configuration accepted:", raw)
		}
	}
}
