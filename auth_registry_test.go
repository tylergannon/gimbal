package gimbal

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tylergannon/gimbal/auth"
)

func TestAuthRegistryRejectsMalformedAndDuplicateDefinitions(t *testing.T) {
	var registry Registry
	provider := ProviderDefinition{
		Name: "github", ValidateConfiguration: func(json.RawMessage) error { return nil },
		Connect: func(context.Context, json.RawMessage, string) (auth.Provider, error) { return nil, nil },
	}
	users := UsersDefinition{
		Name: "files", ValidateConfiguration: func(json.RawMessage) error { return nil },
		Connect: func(context.Context, json.RawMessage) (auth.Users, error) { return nil, nil },
	}
	for _, bad := range []ProviderDefinition{{}, {Name: " "}, {Name: "x", Connect: provider.Connect}, {Name: "x", ValidateConfiguration: provider.ValidateConfiguration}} {
		if err := registry.RegisterProvider(bad); err == nil {
			t.Fatal("malformed provider registered")
		}
	}
	for _, bad := range []UsersDefinition{{}, {Name: " "}, {Name: "x", Connect: users.Connect}, {Name: "x", ValidateConfiguration: users.ValidateConfiguration}} {
		if err := registry.RegisterUsers(bad); err == nil {
			t.Fatal("malformed users registered")
		}
	}
	if err := registry.RegisterProvider(provider); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterUsers(users); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterProvider(provider); err == nil {
		t.Fatal("duplicate provider registered")
	}
	if err := registry.RegisterUsers(users); err == nil {
		t.Fatal("duplicate users registered")
	}
	if d, ok := registry.Provider("github"); !ok || d.Connect == nil {
		t.Fatal("provider missing after rejected insertions")
	}
	if d, ok := registry.Users("files"); !ok || d.Connect == nil {
		t.Fatal("users missing after rejected insertions")
	}
	if _, ok := registry.Provider("missing"); ok {
		t.Fatal("unknown provider exists")
	}
	if _, ok := registry.Users("missing"); ok {
		t.Fatal("unknown users exists")
	}
}
