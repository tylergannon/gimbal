package host

import (
	"context"
	"testing"
)

func TestCloseStopsProjectPromisesWorker(t *testing.T) {
	o := New(context.Background(), t.TempDir())
	p, err := o.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if p.Promises() == nil {
		t.Fatal("project has no ledger")
	}
	o.Close()
	select {
	case <-p.promisesDone:
	default:
		t.Fatal("close did not join worker")
	}
}
