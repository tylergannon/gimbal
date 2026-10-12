// Package secretfiles provides a private, shared filesystem secrets backend.
package secretfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

type Store struct{ directory string }
type record struct {
	Scope  backend.Scope
	Stores map[string]map[string]string
}

func Open(directory string) (*Store, error) {
	if !filepath.IsAbs(directory) {
		return nil, fmt.Errorf("secrets directory must be absolute")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, err
	}
	return &Store{directory}, nil
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
	file := filepath.Join(s.directory, "values.json")
	raw, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var records []record
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &records); err != nil {
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
	raw, err = json.Marshal(records)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.directory, ".values-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), file)
}
func (s *Store) Publish(ctx context.Context, scope backend.Scope, store string, values map[string]string) error {
	return s.access(ctx, true, func(v backend.ScopedValues) error { return v.Publish(scope, store, values) })
}
func (s *Store) Acquire(ctx context.Context, scope backend.Scope, r backend.Requirements) (values map[string]string, err error) {
	err = s.access(ctx, false, func(v backend.ScopedValues) error { values, err = v.Acquire(scope, r); return err })
	return
}
func (s *Store) Release(ctx context.Context, scope backend.Scope) error {
	return s.access(ctx, true, func(v backend.ScopedValues) error { v.Release(scope); return nil })
}
