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
	if err := a.Publish(ctx, root, backend.StoreWrite{Store: "database", Values: []backend.NamedValue{{Name: "DATABASE_URL", Value: "root"}, {Name: "EXTRA", Value: "whole-store"}}}); err != nil {
		t.Fatal(err)
	}
	values, err := b.Acquire(ctx, child, backend.Requirements{{Store: "database", Names: []string{"DATABASE_URL"}}})
	if err != nil || values["EXTRA"] != "whole-store" {
		t.Fatalf("whole store %v: %v", values, err)
	}
	if err := b.Publish(ctx, child, backend.StoreWrite{Store: "private", Values: []backend.NamedValue{{Name: "DATABASE_URL", Value: "child"}}}); err != nil {
		t.Fatal(err)
	}
	if values, err := a.Acquire(ctx, child, backend.Requirements{{Store: "database", Names: []string{"DATABASE_URL"}}}); err == nil || values != nil {
		t.Fatal("inaccessible child definition must hide ancestor")
	}
	if err := a.Publish(ctx, child, backend.StoreWrite{Store: "other", Values: []backend.NamedValue{{Name: "DATABASE_URL", Value: "duplicate"}}}); err == nil {
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

func TestDuplicateWriteFailsAtomically(t *testing.T) {
	for _, kind := range []string{"memory", "files"} {
		t.Run(kind, func(t *testing.T) {
			store := backend.NewMemorySecrets()
			if kind == "files" {
				files, err := Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				store = files
			}
			scope := backend.Scope{Run: "run"}
			write := backend.StoreWrite{Store: "private", Values: []backend.NamedValue{
				{Name: "VALID", Value: "must-not-publish"},
				{Name: "DUPLICATE", Value: "one"},
				{Name: "DUPLICATE", Value: "two"},
			}}
			if err := store.Publish(t.Context(), scope, write); err == nil {
				t.Fatal("duplicate write succeeded")
			}
			values, err := store.Acquire(t.Context(), scope, backend.Requirements{{Store: "private"}})
			if err != nil || len(values) != 0 {
				t.Fatalf("failed write published values: %v, %v", values, err)
			}
			if err := store.Publish(t.Context(), scope, backend.StoreWrite{Store: "private", Values: []backend.NamedValue{{Name: "VALID", Value: "accepted"}}}); err != nil {
				t.Fatal(err)
			}
			values, err = store.Acquire(t.Context(), scope, backend.Requirements{{Store: "private", Names: []string{"VALID"}}})
			if err != nil || values["VALID"] != "accepted" {
				t.Fatalf("valid retry failed: %v, %v", values, err)
			}
		})
	}
}
