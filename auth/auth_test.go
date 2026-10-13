package auth_test

import (
	"context"
	"testing"

	"github.com/tylergannon/gimbal/auth"
)

type customerUser struct {
	id   string
	name string
	team string
}

func (u *customerUser) ID() string   { return u.id }
func (u *customerUser) Name() string { return u.name }

func TestRequestContextPreservesCustomerUser(t *testing.T) {
	ctx := context.Background()
	if auth.UserFrom(ctx) != nil {
		t.Fatal("anonymous context has a user")
	}
	u := &customerUser{id: "operator", name: "Tyler", team: "extra customer data"}
	ctx = auth.WithUser(ctx, u)
	if auth.UserFrom(ctx) != u {
		t.Fatal("customer implementation was replaced")
	}
}
