// Package auth defines provider identities and durable local users and sessions.
// The application owns HTTP login and session handling; plugins implement these
// contracts without registering routes.
package auth

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound means that a user, identity mapping, or session does not exist.
// Other errors are operational failures and must deny access.
var ErrNotFound = errors.New("auth: not found")

// User is a local user, reconstructed by its store on every protected request.
// ID is immutable and nonempty. Name is display text, never an authorization key.
type User interface {
	ID() string
	Name() string
}

// Identity is an external identity verified by the selected provider.
// Issuer and Subject together identify it; Name is display text.
type Identity struct {
	Issuer  string
	Subject string
	Name    string
}

// Session stores the SHA-256 lowercase hex hash of a random bearer token.
// A store never receives the token itself. ExpiresAt is an absolute expiry.
type Session struct {
	Hash      string
	UserID    string
	ExpiresAt time.Time
}

// Users owns durable local users, identity mappings, and sessions. Methods are
// safe concurrently, respect contexts, and durably complete successful mutations.
// CreateUser is a trusted local operator operation: login only uses
// UserForIdentity, so an unknown provider identity cannot sign itself up.
// CreateUser returns the same ID for an existing exact issuer/subject pair.
// DeleteUser and DeleteSession are idempotent. IDs are never reused; deleting a
// user revokes its access even when an old session still exists. Lookups return
// ErrNotFound for absence. CreateSession verifies its user exists and refuses
// an existing hash. Returned values are immutable copies.
type Users interface {
	CreateUser(context.Context, Identity) (User, error)
	UserForIdentity(ctx context.Context, issuer, subject string) (User, error)
	User(context.Context, string) (User, error)
	DeleteUser(context.Context, string) error
	CreateSession(context.Context, Session) error
	Session(context.Context, string) (Session, error)
	DeleteSession(context.Context, string) error
}

// Provider verifies an authorization code using the application's exact callback
// URL and S256 PKCE verifier. The application validates unpredictable state
// before Exchange. An OIDC provider must also verify the supplied nonce.
type Provider interface {
	AuthorizationURL(state, codeChallenge, nonce string) (string, error)
	Exchange(ctx context.Context, code, codeVerifier, nonce string) (Identity, error)
}

type userKey struct{}

// WithUser attaches a verified local user to a request context.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// UserFrom returns the authenticated local user, or nil when none is attached.
func UserFrom(ctx context.Context) User {
	user, _ := ctx.Value(userKey{}).(User)
	return user
}
