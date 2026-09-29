package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/contextdata"
)

func TestCompiledStoreConfiguredOnce(t *testing.T) {
	err := runTest(t, nil, func(ctx context.Context) error {
		first := contextdata.Store{Objects: uncomparableObjects{contextdata.FileObjects{Root: t.TempDir()}, nil}, LocalDir: t.TempDir()}
		if err := initializeCompiledContext(ctx, first, first.LocalDir, ""); err != nil {
			return err
		}
		second := contextdata.Store{Objects: contextdata.FileObjects{Root: t.TempDir()}, LocalDir: t.TempDir()}
		if err := initializeCompiledContext(ctx, second, second.LocalDir, ""); err == nil {
			t.Error("run accepted replacement object backend and materialization directory")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type uncomparableObjects struct {
	contextdata.Objects
	unused []byte
}
type stalledObjects struct {
	contextdata.Objects
	entered chan struct{}
	release chan struct{}
}

func (s *stalledObjects) Put(ctx context.Context, id string, b []byte) error {
	close(s.entered)
	<-s.release
	return s.Objects.Put(ctx, id, b)
}

func TestCompiledContextPublicationDoesNotLockScope(t *testing.T) {
	objects := &stalledObjects{Objects: contextdata.FileObjects{Root: t.TempDir()}, entered: make(chan struct{}), release: make(chan struct{})}
	store := contextdata.Store{Objects: objects, LocalDir: t.TempDir()}
	err := runTest(t, nil, func(ctx context.Context) error {
		if err := initializeCompiledContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		completed := make(chan error, 1)
		go func() {
			_, err := WriteContext(ctx, "", contextdata.Entry{Key: "remote", Value: json.RawMessage(`"value"`)})
			completed <- err
		}()
		<-objects.entered
		setDone := make(chan struct{})
		go func() { Set(ctx, "unrelated", "available"); close(setDone) }()
		select {
		case <-setDone:
		case <-time.After(time.Second):
			t.Error("remote publication holds the scope mutex")
		}
		close(objects.release)
		<-setDone
		return <-completed
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompiledCanceledCheckPublishesEvidence(t *testing.T) {
	store := contextdata.Store{Root: t.TempDir()}
	err := runTest(t, nil, func(ctx context.Context) error {
		if err := initializeCompiledContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		ref, record, err := checkCompiledContext(canceled, "", "canceled", ".", "sh", "-c", "echo not-run")
		if !errors.Is(err, context.Canceled) || ref == "" || record.Error == "" {
			t.Fatalf("ref=%q record=%+v err=%v", ref, record, err)
		}
		entries, err := store.Load(t.Context(), ref)
		if err != nil {
			return err
		}
		if len(entries) != 1 || entries[0].Key != "canceled" {
			t.Fatalf("missing canceled command evidence: %+v", entries)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompiledContextPublicationRechecksState(t *testing.T) {
	for _, concurrent := range []string{"duplicate", "finish"} {
		t.Run(concurrent, func(t *testing.T) {
			objects := &stalledObjects{Objects: contextdata.FileObjects{Root: t.TempDir()}, entered: make(chan struct{}), release: make(chan struct{})}
			store := contextdata.Store{Objects: objects, LocalDir: t.TempDir()}
			err := runTest(t, nil, func(ctx context.Context) error {
				if err := initializeCompiledContext(ctx, store, store.LocalDir, ""); err != nil {
					return err
				}
				child, finish, err := OpenScope(ctx, "child")
				if err != nil {
					return err
				}
				completed := make(chan error, 1)
				go func() {
					ref, err := WriteContext(child, "", contextdata.Entry{Key: "remote", Value: json.RawMessage(`"value"`)})
					if ref != "" {
						t.Error("rejected write exposed snapshot")
					}
					completed <- err
				}()
				<-objects.entered
				changed := make(chan error, 1)
				go func() {
					if concurrent == "duplicate" {
						Set(child, "remote", "winner")
						changed <- nil
					} else {
						changed <- finish(nil)
					}
				}()
				select {
				case err = <-changed:
				case <-time.After(time.Second):
					t.Error("publication blocks concurrent scope change")
				}
				close(objects.release)
				if err != nil {
					return err
				}
				if err := <-completed; err == nil {
					t.Error("publication installed after competing write or finish")
				}
				if concurrent == "duplicate" {
					return finish(nil)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

type cancelableObjects struct {
	contextdata.Objects
	entered chan struct{}
	release chan struct{}
}

func (s cancelableObjects) Put(ctx context.Context, id string, b []byte) error {
	close(s.entered)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return s.Objects.Put(ctx, id, b)
	}
}
func TestCompiledContextPublicationHonorsCancellation(t *testing.T) {
	objects := cancelableObjects{Objects: contextdata.FileObjects{Root: t.TempDir()}, entered: make(chan struct{}), release: make(chan struct{})}
	defer close(objects.release)
	store := contextdata.Store{Objects: objects, LocalDir: t.TempDir()}
	err := runTest(t, nil, func(ctx context.Context) error {
		if err := initializeCompiledContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		writing, cancel := context.WithCancel(ctx)
		defer cancel()
		completed := make(chan error, 1)
		go func() {
			_, err := WriteContext(writing, "", contextdata.Entry{Key: "remote", Value: json.RawMessage(`"value"`)})
			completed <- err
		}()
		<-objects.entered
		cancel()
		select {
		case err := <-completed:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("publication cancellation = %v", err)
			}
		case <-time.After(time.Second):
			t.Error("storage did not receive operation cancellation")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
