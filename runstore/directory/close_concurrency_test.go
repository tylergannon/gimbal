package directory

import (
	"errors"
	"github.com/tylergannon/gimbal/runstore"
	"os"
	"testing"
)

func TestCloseExcludesAppendDuringFinalFlush(t *testing.T) {
	_, handle, h := create(t)
	r := handle.(*run)
	// Stop the periodic flusher so only Close determines the final watermark.
	close(r.stop)
	<-r.done
	r.done = nil
	if _, err := r.Append(ctx, []runstore.Fact{fact(h.Root, "event")}, runstore.Buffered); err != nil {
		t.Fatal(err)
	}
	synced, release := make(chan struct{}), make(chan struct{})
	r.syncFile = func(f *os.File) error {
		err := f.Sync()
		close(synced)
		<-release
		return err
	}
	finished := make(chan error, 1)
	go func() { finished <- r.Close() }()
	<-synced
	_, appendErr := r.Append(ctx, []runstore.Fact{fact(h.Root, "event")}, runstore.Buffered)
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(appendErr, runstore.ErrClosed) {
		t.Fatalf("append during Close: %v", appendErr)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pendingBytes != 0 || r.committed != uint64(len(r.records)) {
		t.Fatalf("Close acknowledged accepted unsynced records: committed=%d total=%d pendingBytes=%d", r.committed, len(r.records), r.pendingBytes)
	}
}
