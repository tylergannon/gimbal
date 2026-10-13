package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/devalue/v5"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/auth"
	generated "github.com/tylergannon/gimbal/internal/skgo"
	"github.com/tylergannon/gimbal/internal/webauth"
)

// Customer fields do not become session data or public user fields. Each lookup
// reconstructs a new customer value; changing its name exercises fresh hydration.
type customerAuthUser struct {
	id, name             string
	PrivateCustomerField string
}

func (u customerAuthUser) ID() string   { return u.id }
func (u customerAuthUser) Name() string { return u.name }

type customerAuthUsers struct {
	mu             sync.Mutex
	users          map[string]customerAuthUser
	identities     map[[2]string]string
	sessions       map[string]auth.Session
	creates, reads int
}

func (s *customerAuthUsers) CreateUser(_ context.Context, identity auth.Identity) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creates++
	key := [2]string{identity.Issuer, identity.Subject}
	if id, ok := s.identities[key]; ok {
		return s.users[id], nil
	}
	id := fmt.Sprintf("customer-%d", s.creates)
	s.users[id] = customerAuthUser{id: id, name: identity.Name, PrivateCustomerField: "never-public-customer-data"}
	s.identities[key] = id
	return s.users[id], nil
}
func (s *customerAuthUsers) UserForIdentity(_ context.Context, issuer, subject string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.identities[[2]string{issuer, subject}]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return s.users[id], nil
}
func (s *customerAuthUsers) User(_ context.Context, id string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	u, ok := s.users[id]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}
func (s *customerAuthUsers) DeleteUser(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, id)
	for key, existing := range s.identities {
		if existing == id {
			delete(s.identities, key)
		}
	}
	return nil
}
func (s *customerAuthUsers) CreateSession(_ context.Context, session auth.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[session.UserID]; !ok {
		return auth.ErrNotFound
	}
	if _, ok := s.sessions[session.Hash]; ok {
		return fmt.Errorf("duplicate session")
	}
	s.sessions[session.Hash] = session
	return nil
}
func (s *customerAuthUsers) Session(_ context.Context, hash string) (auth.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[hash]
	if !ok {
		return session, auth.ErrNotFound
	}
	return session, nil
}
func (s *customerAuthUsers) DeleteSession(_ context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, hash)
	return nil
}

type customerAuthProvider struct{}

func (*customerAuthProvider) AuthorizationURL(state, challenge, nonce string) (string, error) {
	return "https://github.com/login/oauth/authorize?" + url.Values{"state": {state}, "code_challenge": {challenge}}.Encode(), nil
}
func (*customerAuthProvider) Exchange(_ context.Context, code, verifier, nonce string) (auth.Identity, error) {
	if verifier == "" || nonce == "" || (code != "50593" && code != "unprovisioned") {
		return auth.Identity{}, fmt.Errorf("invalid exchange")
	}
	return auth.Identity{Issuer: "https://github.com", Subject: code, Name: "provider display text"}, nil
}

func authRemotePath(t *testing.T, name string) string {
	t.Helper()
	for _, remote := range generated.Remotes() {
		if remote.Module() == "src/routes/auth.remote.ts" && remote.Name() == name {
			return "/_app/remote/" + remote.ID()
		}
	}
	t.Fatalf("missing generated auth remote %s", name)
	return ""
}
func authField(t *testing.T, value any, key string) any {
	t.Helper()
	object, ok := value.(*devalue.Object)
	if !ok {
		t.Fatalf("expected object for %s, got %T", key, value)
	}
	field, ok := object.Get(key)
	if !ok {
		t.Fatalf("missing field %s in %v", key, object.Keys())
	}
	return field
}
func authRemoteResult(t *testing.T, raw []byte) any {
	t.Helper()
	var envelope struct{ Type, Data string }
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Type != "result" {
		t.Fatalf("remote failed: %s", raw)
	}
	value, err := devalue.Parse(envelope.Data, nil)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestAuthProductionStackWithCustomerUsers(t *testing.T) {
	t.Setenv("GIMBAL_WEB_ORIGIN", "")
	t.Setenv("GIMBAL_WEB_PROXY", "")
	root := t.TempDir()
	project := filepath.Join(root, "private-project-do-not-leak")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	store := &customerAuthUsers{users: map[string]customerAuthUser{}, identities: map[[2]string]string{}, sessions: map[string]auth.Session{}}
	name := "auth-test-" + filepath.Base(filepath.Dir(root))
	validate := func(json.RawMessage) error { return nil }
	const publicOrigin = "https://gimbal.test"
	if err := gimbal.DefaultRegistry.RegisterProvider(gimbal.ProviderDefinition{
		Name: name, ValidateConfiguration: validate,
		Connect: func(_ context.Context, _ json.RawMessage, callback string) (auth.Provider, error) {
			if callback != publicOrigin+"/auth/callback" {
				return nil, fmt.Errorf("wrong callback %s", callback)
			}
			return &customerAuthProvider{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := gimbal.DefaultRegistry.RegisterUsers(gimbal.UsersDefinition{
		Name: name, ValidateConfiguration: validate,
		Connect: func(context.Context, json.RawMessage) (auth.Users, error) { return store, nil },
	}); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "auth.json")
	raw, err := json.Marshal(map[string]any{"provider": map[string]any{"name": name, "configuration": map[string]any{}}, "users": map[string]any{"name": name, "configuration": map[string]any{}}, "origin": publicOrigin})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(root, "instance")
	instance, err := NewInstance(t.Context(), instanceDir, []string{project}, WithPort(0), WithAuthFile(config))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.cancel(nil); instance.Wait() })
	owner, err := instance.Owner.Project(project)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(client.CloseIdleConnections)
	base := "http://" + instance.Address()
	request := func(method, path, origin string, cookie *http.Cookie, body string, navigation bool) (*http.Response, []byte) {
		t.Helper()
		r, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if navigation {
			r.Header.Set("Sec-Fetch-Mode", "navigate")
			r.Header.Set("Sec-Fetch-Dest", "document")
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		return response, raw
	}
	for _, path := range []string{"/auth/login", "/auth/login/__data.json"} {
		response, body := request("GET", path, "", nil, "", false)
		if response.StatusCode != 200 {
			t.Fatalf("anonymous login %s: %d %s", path, response.StatusCode, body)
		}
		if strings.Contains(string(body), project) || strings.Contains(string(body), owner.ID()) || strings.Contains(string(body), "Sign out") {
			t.Fatalf("anonymous login leaked protected layout: %s", body)
		}
		if path == "/auth/login" && !strings.Contains(string(body), "Sign in with GitHub") {
			t.Fatalf("missing sign-in button: %s", body)
		}
		for _, remote := range generated.Remotes() {
			if remote.Name() == "watchRecording" && strings.Contains(string(body), remote.ID()+"/") {
				t.Fatal("anonymous login serialized a recorder subscription")
			}
		}
	}
	asset := ""
	if err := fs.WalkDir(Build, "build/client", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".css") && asset == "" {
			asset = "/" + strings.TrimPrefix(path, "build/client/")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if asset == "" {
		t.Fatal("production build has no stylesheet")
	}
	assetResponse, assetBody := request("GET", asset, "", nil, "", false)
	if assetResponse.StatusCode != 200 || len(assetBody) == 0 {
		t.Fatalf("anonymous login asset %s: %d", asset, assetResponse.StatusCode)
	}
	currentPath := authRemotePath(t, "currentUser")
	logoutPath := authRemotePath(t, "logout")
	for _, test := range []struct{ method, path string }{
		{"GET", "/"}, {"GET", "/projects/unknown"}, {"GET", "/projects/" + owner.ID()},
		{"GET", currentPath}, {"POST", logoutPath}, {"GET", "/_app/remote/unknown/currentUser"},
		{"GET", "/projects/unknown/files/private/body"}, {"GET", "/projects/" + owner.ID() + "/files/private/body"},
		{"GET", "/api/runs/private/events"}, {"GET", "/projects/" + owner.ID() + "/api/runs/private/events"},
		{"GET", "/api/runs/private/files/private.txt"},
	} {
		response, body := request(test.method, test.path, publicOrigin, nil, "", false)
		if response.StatusCode != 401 {
			t.Fatalf("anonymous %s %s reached project/app: %d %s", test.method, test.path, response.StatusCode, body)
		}
	}
	response, body := request("GET", "/projects/unknown/__data.json", "", nil, "", false)
	var redirect struct {
		Type     string
		Status   int
		Location string
	}
	if err := json.Unmarshal(body, &redirect); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || redirect.Type != "redirect" || redirect.Status != 303 || redirect.Location != "/auth/login" {
		t.Fatalf("route-data redirect: %d %s", response.StatusCode, body)
	}
	response, _ = request("GET", "/projects/unknown/files/private/body", "", nil, "", true)
	if response.StatusCode != 303 || response.Header.Get("Location") != "/auth/login" {
		t.Fatalf("document file navigation: %d %v", response.StatusCode, response.Header)
	}

	beginPath := authRemotePath(t, "beginLogin")
	response, _ = request("POST", beginPath, "https://evil.test", nil, `{"payload":"","refreshes":[]}`, false)
	if response.StatusCode != 403 || len(response.Cookies()) != 0 {
		t.Fatalf("beginLogin origin check: %d", response.StatusCode)
	}
	login := func(code string) (*http.Response, []byte) {
		t.Helper()
		response, body := request("POST", beginPath, publicOrigin, nil, `{"payload":"","refreshes":[]}`, false)
		if response.StatusCode != 200 {
			t.Fatalf("beginLogin: %d %s", response.StatusCode, body)
		}
		target, ok := authField(t, authRemoteResult(t, body), "_").(string)
		if !ok {
			t.Fatal("authorization URL is not a string")
		}
		parsed, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		var flow *http.Cookie
		for _, cookie := range response.Cookies() {
			if cookie.Name == webauth.FlowCookie {
				flow = cookie
			}
		}
		if flow == nil {
			t.Fatal("beginLogin did not issue a flow cookie")
		}
		return request("GET", "/auth/callback?"+url.Values{"state": {parsed.Query().Get("state")}, "code": {code}}.Encode(), "", flow, "", false)
	}
	response, _ = login("unprovisioned")
	if response.StatusCode != 403 {
		t.Fatalf("unprovisioned callback: %d", response.StatusCode)
	}
	store.mu.Lock()
	if store.creates != 0 || len(store.sessions) != 0 {
		t.Fatal("callback created an unprovisioned user or session")
	}
	store.mu.Unlock()

	discoveries, err := filepath.Glob(filepath.Join(instanceDir, "control", "*.json"))
	if err != nil || len(discoveries) != 1 {
		t.Fatalf("control discoveries: %v %v", discoveries, err)
	}
	raw, err = os.ReadFile(discoveries[0])
	if err != nil {
		t.Fatal(err)
	}
	var discovery controlDiscovery
	if err := json.Unmarshal(raw, &discovery); err != nil {
		t.Fatal(err)
	}
	control := controlClient(discovery.Socket)
	t.Cleanup(control.CloseIdleConnections)
	controlRequest := func(method, path, body string) (*http.Response, []byte) {
		t.Helper()
		r, err := http.NewRequest(method, "http://control"+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response, err := control.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		return response, raw
	}
	response, body = controlRequest("POST", "/control/users", `{"Issuer":"https://github.com","Subject":"50593","Name":"Tyler"}`)
	if response.StatusCode != 200 {
		t.Fatalf("control provision: %d %s", response.StatusCode, body)
	}
	var admitted struct{ ID, Name string }
	if err := json.Unmarshal(body, &admitted); err != nil {
		t.Fatal(err)
	}
	response, _ = login("50593")
	if response.StatusCode != 303 || response.Header.Get("Location") != "/" {
		t.Fatalf("admitted callback: %d %v", response.StatusCode, response.Header)
	}
	var session *http.Cookie
	for _, cookie := range response.Cookies() {
		if cookie.Name == webauth.SessionCookie {
			session = cookie
		}
	}
	if session == nil {
		t.Fatal("admitted login did not set session")
	}
	probe := func(wantName string) {
		t.Helper()
		response, body := request("GET", currentPath, "", session, "", false)
		if response.StatusCode != 200 {
			t.Fatalf("currentUser: %d %s", response.StatusCode, body)
		}
		data := authRemoteResult(t, body)
		id := strings.TrimPrefix(currentPath, "/_app/remote/")
		node := authField(t, authField(t, data, "q"), id+"/")
		user := authField(t, authField(t, node, "v"), "user")
		if authField(t, user, "id") != admitted.ID || authField(t, user, "name") != wantName {
			t.Fatalf("currentUser DTO: %s", body)
		}
		if strings.Contains(string(body), "never-public-customer-data") || strings.Contains(string(body), "PrivateCustomerField") {
			t.Fatal("customer fields serialized")
		}
	}
	probe("Tyler")
	store.mu.Lock()
	u := store.users[admitted.ID]
	u.name = "Rehydrated Tyler"
	store.users[admitted.ID] = u
	reads := store.reads
	store.mu.Unlock()
	probe("Rehydrated Tyler")
	response, body = request("GET", "/", "", session, "", true)
	if response.StatusCode != 200 || !strings.Contains(string(body), "Rehydrated Tyler") || !strings.Contains(string(body), project) {
		t.Fatalf("authenticated root SSR: %d %s", response.StatusCode, body)
	}
	if strings.Contains(string(body), "never-public-customer-data") {
		t.Fatal("SSR serialized private customer fields")
	}
	store.mu.Lock()
	if store.reads <= reads {
		t.Fatal("subsequent request did not rehydrate customer User")
	}
	if store.creates != 1 {
		t.Fatal("login provisioned a user")
	}
	store.mu.Unlock()
	response, body = controlRequest("DELETE", "/control/users/"+admitted.ID, "")
	if response.StatusCode != 204 {
		t.Fatalf("control remove: %d %s", response.StatusCode, body)
	}
	response, _ = request("GET", currentPath, "", session, "", false)
	if response.StatusCode != 401 {
		t.Fatalf("removed customer's old session retained access: %d", response.StatusCode)
	}
}
