package routes

import (
	"context"
	"net/http"
	"strings"

	"github.com/tylergannon/gimbal/auth"
	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/gimbal/internal/webauth"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/skgo"
)

// AuthUserSummary is the only account information exposed to the page.
type AuthUserSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func beginLogin(ctx context.Context, event app.RequestEvent[params.Params]) (string, error) {
	auth := webauth.From(ctx)
	if auth == nil {
		return "", skgo.Errorf(http.StatusNotFound, "Sign-in is not enabled.")
	}
	authorizationURL, cookie, err := auth.BeginLogin(ctx)
	if err != nil {
		return "", skgo.Errorf(http.StatusServiceUnavailable, "Could not start sign-in.")
	}
	secure, httpOnly := cookie.Secure, cookie.HttpOnly
	err = skgo.EventFrom(ctx).SetCookie(cookie.Name, cookie.Value, skgo.CookieOptions{
		Path: cookie.Path, MaxAge: cookie.MaxAge, Expires: cookie.Expires,
		SameSite: cookie.SameSite, Secure: &secure, HTTPOnly: &httpOnly,
	})
	return authorizationURL, err
}

func logout(ctx context.Context, caller app.RequestEvent[params.Params]) (bool, error) {
	auth := webauth.From(ctx)
	if auth == nil {
		return false, skgo.Errorf(http.StatusNotFound, "Sign-in is not enabled.")
	}
	event := skgo.EventFrom(ctx)
	if err := auth.Logout(ctx, event.Request()); err != nil {
		return false, skgo.Errorf(http.StatusServiceUnavailable, "Could not sign out. Please try again.")
	}
	secure := strings.HasPrefix(auth.Origin, "https://")
	if err := event.DeleteCookie(auth.SessionCookieName(), skgo.CookieOptions{Path: "/", Secure: &secure, SameSite: http.SameSiteLaxMode}); err != nil {
		return false, err
	}
	return true, nil
}

type CurrentUser struct {
	User polytype.Nullable[AuthUserSummary] `json:"user"`
}

func currentUser(ctx context.Context) (CurrentUser, error) {
	if webauth.From(ctx) == nil {
		return CurrentUser{}, nil
	}
	user := auth.UserFrom(ctx)
	if user == nil {
		return CurrentUser{}, skgo.Errorf(http.StatusUnauthorized, "Sign in to continue.")
	}
	return CurrentUser{User: polytype.Nullable[AuthUserSummary]{Present: true, Value: AuthUserSummary{ID: user.ID(), Name: user.Name()}}}, nil
}

var _ = skgo.Command(beginLogin)
var _ = skgo.Command(logout)
var _ = skgo.Query(currentUser)
