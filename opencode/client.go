// Package opencode manages Gimble's shared OpenCode server and speaks the
// small legacy HTTP surface needed by the OpenCode harness.
package opencode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxErrorBody = 64 << 10

// Client is a project-scoped client for the shared OpenCode server.
type Client struct {
	baseURL  string
	username string
	password string
	workdir  string
	http     *http.Client
}

// RawEvent is one JSON value received from OpenCode's process-wide SSE
// stream. Raw is retained verbatim so later normalization cannot discard an
// event or a field that OpenCode sent.
type RawEvent struct {
	Directory string          `json:"directory,omitempty"`
	Project   string          `json:"project,omitempty"`
	Workspace string          `json:"workspace,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	Raw       json.RawMessage `json:"-"`
	Malformed string          `json:"-"`
}

// SessionCreateInput is the small inline request body of POST /session.
// The response itself uses the generated upstream Session component.
type SessionCreateInput struct {
	Title *string       `json:"title,omitempty"`
	Agent *string       `json:"agent,omitempty"`
	Model *SessionModel `json:"model,omitempty"`
}

// SessionModel selects the initial model on a newly created session.
type SessionModel struct {
	ID         string  `json:"id"`
	ProviderID string  `json:"providerID"`
	Variant    *string `json:"variant,omitempty"`
}

// PromptInput is the text-only inline request body used by the Gimble adapter.
// OutputFormat and TextPartInput are generated from their upstream components.
type PromptInput struct {
	MessageID *string          `json:"messageID,omitempty"`
	Model     *PromptModel     `json:"model,omitempty"`
	Agent     *string          `json:"agent,omitempty"`
	NoReply   *bool            `json:"noReply,omitempty"`
	Tools     *map[string]bool `json:"tools,omitempty"`
	Format    *OutputFormat    `json:"format,omitempty"`
	System    *string          `json:"system,omitempty"`
	Variant   *string          `json:"variant,omitempty"`
	Parts     []TextPartInput  `json:"parts"`
}

// PromptModel selects the provider and model for one prompt.
type PromptModel struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

// PromptResponse is the small inline response envelope of the synchronous
// prompt endpoint. Its message and part unions are generated upstream types.
type PromptResponse struct {
	Info  AssistantMessage `json:"info"`
	Parts []Part           `json:"parts"`
}

// MessageRecord is one inline list envelope from GET /session/{id}/message.
type MessageRecord struct {
	Info  Message `json:"info"`
	Parts []Part  `json:"parts"`
}

// ForkInput is the optional inline request body of POST /session/{id}/fork.
type ForkInput struct {
	MessageID *string `json:"messageID,omitempty"`
}

// Connect starts or discovers the shared server and returns a client bound to
// workdir. An empty stateDir uses DefaultStateDir.
func Connect(ctx context.Context, workdir, stateDir string) (*Client, error) {
	absolute, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("resolve OpenCode workdir: %w", err)
	}
	if err := validateWorkdir(absolute); err != nil {
		return nil, fmt.Errorf("OpenCode workdir %s: %w", absolute, err)
	}
	info, secret, _, err := ensureServer(ctx, stateDir)
	if err != nil {
		return nil, err
	}
	return newClient(info.URL, secret.Username, secret.Password, absolute, http.DefaultClient), nil
}

func newClient(baseURL, username, password, workdir string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		workdir:  workdir,
		http:     httpClient,
	}
}

// CreateSession creates a legacy OpenCode session in the client's project.
func (c *Client) CreateSession(ctx context.Context, input SessionCreateInput) (Session, error) {
	return requestJSON[Session](ctx, c, http.MethodPost, "/session", input)
}

// Prompt posts one legacy prompt and waits for OpenCode's authoritative final
// response. Live events are observed independently through Events.
func (c *Client) Prompt(ctx context.Context, sessionID string, input PromptInput) (PromptResponse, error) {
	return requestJSON[PromptResponse](ctx, c, http.MethodPost, sessionPath(sessionID, "message"), input)
}

// Abort interrupts the active work in a legacy session.
func (c *Client) Abort(ctx context.Context, sessionID string) (bool, error) {
	return requestJSON[bool](ctx, c, http.MethodPost, sessionPath(sessionID, "abort"), nil)
}

// Fork copies a legacy conversation, optionally at a particular message.
func (c *Client) Fork(ctx context.Context, sessionID string, input ForkInput) (Session, error) {
	return requestJSON[Session](ctx, c, http.MethodPost, sessionPath(sessionID, "fork"), input)
}

// Messages returns the persisted messages and parts in a legacy session.
func (c *Client) Messages(ctx context.Context, sessionID string) ([]MessageRecord, error) {
	return requestJSON[[]MessageRecord](ctx, c, http.MethodGet, sessionPath(sessionID, "message"), nil)
}

// Events reads the shared process-wide SSE stream until ctx is cancelled, the
// callback fails, or the server connection ends. The callback receives every
// data value, including event types it does not recognize.
func (c *Client) Events(ctx context.Context, onEvent func(RawEvent) error) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/global/event", nil, false)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("OpenCode events: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return responseError(response)
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	var data []byte
	dispatch := func() error {
		if len(data) == 0 {
			return nil
		}
		raw := append(json.RawMessage(nil), data...)
		var event RawEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			event.Malformed = err.Error()
		}
		event.Raw = raw
		data = data[:0]
		return onEvent(event)
	}
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			value := line[len("data:"):]
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}
			if len(data) > 0 {
				data = append(data, '\n')
			}
			data = append(data, value...)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read OpenCode events: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

func requestJSON[T any](ctx context.Context, client *Client, method, requestPath string, input any) (T, error) {
	var zero T
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return zero, fmt.Errorf("encode OpenCode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := client.newRequest(ctx, method, requestPath, body, true)
	if err != nil {
		return zero, err
	}
	response, err := client.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("OpenCode %s %s: %w", method, requestPath, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return zero, responseError(response)
	}
	if err := json.NewDecoder(response.Body).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode OpenCode %s %s response: %w", method, requestPath, err)
	}
	return zero, nil
}

func (c *Client) newRequest(ctx context.Context, method, requestPath string, body io.Reader, project bool) (*http.Request, error) {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse OpenCode URL: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, requestPath)
	if project {
		query := endpoint.Query()
		query.Set("directory", c.workdir)
		endpoint.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("make OpenCode request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	if project {
		req.Header.Set("X-OpenCode-Directory", c.workdir)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func responseError(response *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	if err != nil {
		return fmt.Errorf("OpenCode returned %s (read body: %w)", response.Status, err)
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("OpenCode returned %s", response.Status)
	}
	return fmt.Errorf("OpenCode returned %s: %s", response.Status, message)
}

func sessionPath(sessionID, operation string) string {
	return "/session/" + url.PathEscape(sessionID) + "/" + operation
}

func validateWorkdir(workdir string) error {
	info, err := os.Stat(workdir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}
