// Build with go build -buildmode=plugin -o auth-files.so ./plugins/auth-files.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/auth"
)

type configuration struct {
	Directory string `json:"directory"`
}

type user struct {
	LocalID string `json:"id"`
	Display string `json:"name"`
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

func (u user) ID() string   { return u.LocalID }
func (u user) Name() string { return u.Display }

type state struct {
	Version  int                     `json:"version"`
	NextUser uint64                  `json:"nextUser"`
	Users    map[string]user         `json:"users"`
	Sessions map[string]auth.Session `json:"sessions"`
}

type store struct {
	directory string
	lock      *os.File
	gate      chan struct{}
	state     state
	closed    bool
	failure   error
}

func parse(raw json.RawMessage) (configuration, error) {
	var c configuration
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, errors.New("files auth configuration must contain directory")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, errors.New("files auth configuration has trailing content")
	}
	if !filepath.IsAbs(c.Directory) || filepath.Clean(c.Directory) == string(filepath.Separator) {
		return c, errors.New("files auth directory must be an absolute instance-owned directory")
	}
	return c, nil
}

func connect(ctx context.Context, raw json.RawMessage) (auth.Users, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, err := parse(raw)
	if err != nil {
		return nil, err
	}
	err = os.Mkdir(c.Directory, 0700)
	created := err == nil
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(c.Directory)
	if err != nil || !info.IsDir() {
		return nil, errors.New("files auth directory must be a real directory")
	}
	if err := os.Chmod(c.Directory, 0700); err != nil {
		return nil, err
	}
	if created {
		parent, err := os.Open(filepath.Dir(c.Directory))
		if err != nil {
			return nil, err
		}
		err = errors.Join(parent.Sync(), parent.Close())
		if err != nil {
			return nil, err
		}
	}
	lock, err := os.OpenFile(filepath.Join(c.Directory, ".lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("files auth directory is already owned: %w", err)
	}
	s := &store{directory: c.Directory, lock: lock, gate: make(chan struct{}, 1)}
	s.gate <- struct{}{}
	success := false
	defer func() {
		if !success {
			_ = s.Close()
		}
	}()
	path := filepath.Join(c.Directory, "state.json")
	info, err = os.Lstat(path)
	if os.IsNotExist(err) && created {
		initial := state{Version: 1, Users: map[string]user{}, Sessions: map[string]auth.Session{}}
		if err := s.persist(ctx, initial); err != nil {
			return nil, err
		}
	} else {
		if err != nil {
			return nil, fmt.Errorf("files auth existing state is unavailable: %w", err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return nil, errors.New("files auth state must be a regular file with mode 0600")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.state); err != nil {
			return nil, errors.New("files auth state is malformed")
		}
		if err := d.Decode(new(any)); err != io.EOF {
			return nil, errors.New("files auth state has trailing content")
		}
		if err := validate(s.state); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	success = true
	return s, nil
}

func validate(s state) error {
	if s.Version != 1 || s.Users == nil || s.Sessions == nil {
		return errors.New("files auth state has an invalid schema")
	}
	seen := map[[2]string]bool{}
	for id, u := range s.Users {
		n, err := strconv.ParseUint(strings.TrimPrefix(id, "u"), 10, 64)
		if err != nil || n == 0 || n > s.NextUser || id != "u"+strconv.FormatUint(n, 10) || u.LocalID != id || strings.TrimSpace(u.Issuer) == "" || strings.TrimSpace(u.Subject) == "" {
			return errors.New("files auth state contains an invalid user")
		}
		key := [2]string{u.Issuer, u.Subject}
		if seen[key] {
			return errors.New("files auth state contains a duplicate identity")
		}
		seen[key] = true
	}
	for hash, session := range s.Sessions {
		_, exists := s.Users[session.UserID]
		if !validHash(hash) || session.Hash != hash || !exists || session.ExpiresAt.IsZero() {
			return errors.New("files auth state contains an invalid session")
		}
	}
	return nil
}

func validHash(hash string) bool {
	_, err := hex.DecodeString(hash)
	return err == nil && len(hash) == 64 && strings.ToLower(hash) == hash
}

func (s *store) acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.gate:
	}
	var err error
	switch {
	case ctx.Err() != nil:
		err = ctx.Err()
	case s.closed:
		err = errors.New("files auth store is closed")
	case s.failure != nil:
		err = s.failure
	default:
		var info os.FileInfo
		info, err = os.Lstat(filepath.Join(s.directory, "state.json"))
		if err == nil && (!info.Mode().IsRegular() || info.Mode().Perm() != 0600) {
			err = errors.New("files auth state is unavailable or has unsafe permissions")
		}
	}
	if err != nil {
		s.release()
	}
	return err
}

func (s *store) release() { s.gate <- struct{}{} }

func (s *store) copy() state {
	return state{Version: s.state.Version, NextUser: s.state.NextUser, Users: maps.Clone(s.state.Users), Sessions: maps.Clone(s.state.Sessions)}
}

func (s *store) persist(ctx context.Context, next state) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for hash, session := range next.Sessions {
		if !time.Now().Before(session.ExpiresAt) {
			delete(next.Sessions, hash)
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.directory, ".state-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), filepath.Join(s.directory, "state.json")); err != nil {
		return err
	}
	// A renamed state is visible but is not durably committed until the directory
	// is flushed. If that fails, stop serving rather than expose the old state.
	directory, err := os.Open(s.directory)
	if err == nil {
		err = directory.Sync()
		err = errors.Join(err, directory.Close())
	}
	if err != nil {
		s.failure = fmt.Errorf("files auth durability failed: %w", err)
		return s.failure
	}
	s.state = next
	return nil
}

func (s *store) CreateUser(ctx context.Context, identity auth.Identity) (auth.User, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	if strings.TrimSpace(identity.Issuer) == "" || strings.TrimSpace(identity.Subject) == "" {
		return nil, errors.New("files auth identity requires issuer and subject")
	}
	for _, u := range s.state.Users {
		if u.Issuer == identity.Issuer && u.Subject == identity.Subject {
			return u, nil
		}
	}
	if s.state.NextUser == math.MaxUint64 {
		return nil, errors.New("files auth user IDs exhausted")
	}
	next := s.copy()
	next.NextUser++
	id := "u" + strconv.FormatUint(next.NextUser, 10)
	u := user{LocalID: id, Display: identity.Name, Issuer: identity.Issuer, Subject: identity.Subject}
	next.Users[id] = u
	if err := s.persist(ctx, next); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *store) UserForIdentity(ctx context.Context, issuer, subject string) (auth.User, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	for _, u := range s.state.Users {
		if u.Issuer == issuer && u.Subject == subject {
			return u, nil
		}
	}
	return nil, auth.ErrNotFound
}

func (s *store) User(ctx context.Context, id string) (auth.User, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	u, exists := s.state.Users[id]
	if !exists {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func (s *store) DeleteUser(ctx context.Context, id string) error {
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer s.release()
	if _, exists := s.state.Users[id]; !exists {
		return nil
	}
	next := s.copy()
	delete(next.Users, id)
	for hash, session := range next.Sessions {
		if session.UserID == id {
			delete(next.Sessions, hash)
		}
	}
	return s.persist(ctx, next)
}

func (s *store) CreateSession(ctx context.Context, session auth.Session) error {
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer s.release()
	if !validHash(session.Hash) || !time.Now().Before(session.ExpiresAt) {
		return errors.New("files auth session requires a SHA-256 hash and future expiry")
	}
	if _, exists := s.state.Users[session.UserID]; !exists {
		return auth.ErrNotFound
	}
	if _, exists := s.state.Sessions[session.Hash]; exists {
		return errors.New("files auth session hash already exists")
	}
	next := s.copy()
	next.Sessions[session.Hash] = session
	return s.persist(ctx, next)
}

func (s *store) Session(ctx context.Context, hash string) (auth.Session, error) {
	if err := s.acquire(ctx); err != nil {
		return auth.Session{}, err
	}
	defer s.release()
	session, exists := s.state.Sessions[hash]
	if !exists {
		return auth.Session{}, auth.ErrNotFound
	}
	return session, nil
}

func (s *store) DeleteSession(ctx context.Context, hash string) error {
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer s.release()
	if _, exists := s.state.Sessions[hash]; !exists {
		return nil
	}
	next := s.copy()
	delete(next.Sessions, hash)
	return s.persist(ctx, next)
}

func (s *store) Close() error {
	<-s.gate
	defer s.release()
	if s.closed {
		return nil
	}
	s.closed = true
	return errors.Join(syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN), s.lock.Close())
}

func Register(r *gimbal.Registry) error {
	return r.RegisterUsers(gimbal.UsersDefinition{
		Name: "files", ValidateConfiguration: func(raw json.RawMessage) error { _, err := parse(raw); return err }, Connect: connect,
	})
}
