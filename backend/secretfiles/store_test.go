package secretfiles

import (
	"context"
	"testing"

	"github.com/tylergannon/gimbal/backend"
)

func TestSharedScopedWholeStores(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	root := backend.Scope{Run: "run", Scope: "root"}
	child := backend.Scope{Run: "run", Scope: "root/child.1"}
	sibling := backend.Scope{Run: "run", Scope: "root/sibling.1"}
	if err := a.Publish(ctx, root, "database", map[string]string{"DATABASE_URL": "root", "EXTRA": "whole-store"}); err != nil {
		t.Fatal(err)
	}
	values, err := b.Acquire(ctx, child, backend.Requirements{{Store: "database", Names: []string{"DATABASE_URL"}}})
	if err != nil || values["EXTRA"] != "whole-store" {
		t.Fatalf("whole store %v: %v", values, err)
	}
	if err := b.Publish(ctx, child, "private", map[string]string{"DATABASE_URL": "child"}); err != nil {
		t.Fatal(err)
	}
	if values, err := a.Acquire(ctx, child, backend.Requirements{{Store: "database", Names: []string{"DATABASE_URL"}}}); err == nil || values != nil {
		t.Fatal("inaccessible child definition must hide ancestor")
	}
	if err := a.Publish(ctx, child, "other", map[string]string{"DATABASE_URL": "duplicate"}); err == nil {
		t.Fatal("duplicate name allowed")
	}
	if err := a.Release(ctx, child); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []backend.Scope{child, sibling} {
		values, err := b.Acquire(ctx, scope, backend.Requirements{{Store: "database", Names: []string{"DATABASE_URL"}}})
		if err != nil || values["DATABASE_URL"] != "root" {
			t.Fatalf("ancestor preserved %v: %v", values, err)
		}
	}
	if err := b.Release(ctx, root); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Acquire(ctx, child, backend.Requirements{{Store: "database", Names: []string{"DATABASE_URL"}}}); err == nil {
		t.Fatal("released values visible")
	}
}
