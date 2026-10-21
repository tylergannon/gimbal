package secretshttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"filippo.io/age"
	"github.com/tylergannon/gimbal/backend"
)

// Store persists only age ciphertext. Each access takes the file lock and reads
// a fresh snapshot, allowing multiple server processes to share the directory.
type Store struct {
	directory string
	identity  *age.X25519Identity
}

type record struct {
	Scope  backend.Scope
	Stores map[string]map[string]string
}

func Open(directory, identityFile string) (*Store, error) {
	if !filepath.IsAbs(directory) || !filepath.IsAbs(identityFile) {
		return nil, fmt.Errorf("secrets directory and identity file must be absolute")
	}
	raw, err := os.ReadFile(identityFile)
	if err != nil {
		return nil, fmt.Errorf("read secrets identity: %w", err)
	}
	identities, err := age.ParseIdentities(bytes.NewReader(raw))
	if err != nil || len(identities) != 1 {
		return nil, fmt.Errorf("identity file must contain one native age X25519 identity")
	}
	identity, ok := identities[0].(*age.X25519Identity)
	if !ok {
		return nil, fmt.Errorf("identity file must contain one native age X25519 identity")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, err
	}
	s := &Store{directory: directory, identity: identity}
	// Fail startup on a wrong identity or damaged file, and initialize a missing
	// store before accepting requests. Never silently replace unreadable data.
	err = s.access(context.Background(), true, func(backend.ScopedValues) error { return nil })
	return s, err
}

func (s *Store) access(ctx context.Context, write bool, fn func(backend.ScopedValues) error) error {
	lock, err := os.OpenFile(filepath.Join(s.directory, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	filename := filepath.Join(s.directory, "values.age")
	ciphertext, err := os.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var records []record
	if err == nil {
		reader, err := age.Decrypt(bytes.NewReader(ciphertext), s.identity)
		if err != nil {
			return fmt.Errorf("decrypt secrets store: %w", err)
		}
		// Read through EOF to authenticate the entire age stream before using it.
		plaintext, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("authenticate secrets store: %w", err)
		}
		if err := json.Unmarshal(plaintext, &records); err != nil {
			return fmt.Errorf("invalid secrets store data")
		}
	}
	values := backend.ScopedValues{}
	for _, r := range records {
		values[r.Scope] = r.Stores
	}
	if err := fn(values); err != nil {
		return err
	}
	if !write {
		return nil
	}
	records = nil
	for scope, stores := range values {
		records = append(records, record{scope, stores})
	}
	tmp, err := os.CreateTemp(s.directory, ".values-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()
	encrypted, err := age.Encrypt(tmp, s.identity.Recipient())
	if err != nil {
		return err
	}
	if err := json.NewEncoder(encrypted).Encode(records); err != nil {
		return err
	}
	if err := encrypted.Close(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filename); err != nil {
		return err
	}
	dir, err := os.Open(s.directory)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

func (s *Store) Publish(ctx context.Context, scope backend.Scope, write backend.StoreWrite) error {
	return s.access(ctx, true, func(v backend.ScopedValues) error { return v.Publish(scope, write) })
}
func (s *Store) Acquire(ctx context.Context, scope backend.Scope, requirements backend.Requirements) (values map[string]string, err error) {
	err = s.access(ctx, false, func(v backend.ScopedValues) error { values, err = v.Acquire(scope, requirements); return err })
	return
}
func (s *Store) Release(ctx context.Context, scope backend.Scope) error {
	return s.access(ctx, true, func(v backend.ScopedValues) error { v.Release(scope); return nil })
}
