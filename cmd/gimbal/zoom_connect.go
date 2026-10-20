package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/zoomconnect"
	"github.com/tylergannon/gimbal/web"
	"golang.org/x/sys/unix"
)

func newZoomConnectCommand(stdout io.Writer, getenv func(string) string) *cobra.Command {
	command := &cobra.Command{
		Use:   "zoom-connect",
		Short: "Record a Zoom Connect call into a Gimbal conversation",
	}
	var cfg zoomconnect.Config
	var workDir, conversationID string
	record := &cobra.Command{
		Use:   "record",
		Short: "Stream a call's transcript, chat and participant events into a conversation",
		Long: `Reads GET /api/v1/calls/{call}/stream from a Zoom Connect server and appends each
transcript line, public chat message and participant or stream event to a saved
record-only conversation in the project. The API key comes from
ZOOM_CONNECT_API_KEY. When a gimbal instance is running on the project, events go
through its control socket and its own conversation manager writes them; the
project must be admitted by that instance (run it with --project, or start it in
the project). Otherwise the recorder opens .gimbal/conversations itself.
Without --conversation the call has one conversation, created the first time and
resumed after that; with it, an existing record-only conversation is used. Either
way it resumes after the last event saved, so stopping and restarting never
duplicates. It records text events only, not audio, and refuses a conversation
that has a workflow run.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg.Token = getenv("ZOOM_CONNECT_API_KEY")
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return recordZoomCall(ctx, stdout, workDir, conversationID, cfg)
		},
	}
	record.Flags().StringVar(&cfg.BaseURL, "url", "", "Zoom Connect server URL")
	record.Flags().StringVar(&cfg.CallID, "call", "", "Zoom Connect call ID")
	record.Flags().StringVar(&conversationID, "conversation", "", "conversation to resume; blank creates one")
	record.Flags().StringVar(&workDir, "work-dir", ".", "project directory owning the .gimbal state")
	record.Flags().DurationVar(&cfg.Retry, "retry", 0, "wait before reconnecting a dropped stream (default 2s)")
	_ = record.MarkFlagRequired("url")
	_ = record.MarkFlagRequired("call")
	command.AddCommand(record)
	return command
}

// recordZoomCall records through the project's running host when one answers on
// its control socket, so the host's own conversation manager makes every write.
// Only when no host answers does it open the saved conversations itself.
func recordZoomCall(ctx context.Context, stdout io.Writer, workDir, conversationID string, cfg zoomconnect.Config) error {
	runtimes, err := discoverRuntimes(workDir)
	if err != nil {
		return err
	}
	for _, runtime := range runtimes {
		if !runtime.live(ctx) {
			continue
		}
		_, _ = fmt.Fprintf(stdout, "recording call %s through the running gimbal instance (pid %d)\n", cfg.Origin(), runtime.pid)
		return zoomconnect.Record(ctx, cfg, controlStore{ctx: ctx, runtime: runtime}, conversationID, nil)
	}
	return recordOffline(ctx, stdout, workDir, conversationID, cfg)
}

func recordOffline(ctx context.Context, stdout io.Writer, workDir, conversationID string, cfg zoomconnect.Config) error {
	project, err := host.CanonicalProject(workDir)
	if err != nil {
		return err
	}
	state := filepath.Join(project, ".gimbal")
	if err := os.MkdirAll(state, 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(state, "owner.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("project %s is owned by a gimbal instance whose control socket did not answer; stop it to record", project)
	}
	// The recorder writes saved conversations directly and never starts a
	// harness, so its adapter factory has nothing to build.
	manager, err := conversation.New(ctx, state, func(string) (gimbal.HarnessAdapter, error) { return nil, nil }, "", state, project, nil)
	if err != nil {
		return err
	}
	defer manager.Close()
	_, _ = fmt.Fprintf(stdout, "recording call %s into %s (no running gimbal instance)\n", cfg.Origin(), state)
	err = zoomconnect.Record(ctx, cfg, manager, conversationID, nil)
	if errors.Is(err, context.Canceled) {
		err = nil
	}
	return err
}

// controlStore writes through a running host's control socket. The host decides
// every outcome; a lost reply is retried from the cursor the host reports.
type controlStore struct {
	ctx     context.Context
	runtime runtimeClient
}

func (s controlStore) Recording(_ context.Context, source, title string) (string, error) {
	recorded, err := s.send(web.ControlRecord{Source: source, Title: title})
	return recorded.Conversation, err
}

func (s controlStore) Cursor(id, source string) (int64, error) {
	recorded, err := s.send(web.ControlRecord{Conversation: id, Source: source})
	return recorded.Cursor, err
}

func (s controlStore) Record(id, source string, entries []conversation.Entry) (int, error) {
	recorded, err := s.send(web.ControlRecord{Conversation: id, Source: source, Entries: entries})
	return recorded.Appended, err
}

func (s controlStore) send(request web.ControlRecord) (web.ControlRecorded, error) {
	var recorded web.ControlRecorded
	body, err := json.Marshal(request)
	if err != nil {
		return recorded, err
	}
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	response, cleanup, err := s.runtime.request(ctx, http.MethodPost, "/control/conversations/record", bytes.NewReader(body))
	if err != nil {
		return recorded, fmt.Errorf("%w: %v", zoomconnect.ErrUnavailable, err)
	}
	defer cleanup()
	defer func() { _ = response.Body.Close() }()
	message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	switch {
	case response.StatusCode == http.StatusOK:
		if err := json.Unmarshal(message, &recorded); err != nil {
			return recorded, fmt.Errorf("%w: unreadable reply: %v", zoomconnect.ErrUnavailable, err)
		}
		return recorded, nil
	case response.StatusCode == http.StatusConflict:
		return recorded, conversation.ErrWorkflowBacked
	case response.StatusCode == http.StatusNotFound && strings.Contains(string(message), "project is not admitted"):
		return recorded, fmt.Errorf("the running gimbal instance has not admitted this project")
	case response.StatusCode == http.StatusNotFound:
		return recorded, conversation.ErrNotFound
	case response.StatusCode >= 500:
		return recorded, fmt.Errorf("%w: host answered %s", zoomconnect.ErrUnavailable, response.Status)
	}
	return recorded, fmt.Errorf("host answered %s: %s", response.Status, strings.TrimSpace(string(message)))
}

// live reports whether the runtime behind a discovery file is answering.
func (r runtimeClient) live(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	response, cleanup, err := r.request(ctx, http.MethodGet, "/control/live", nil)
	if err != nil {
		return false
	}
	defer cleanup()
	_ = response.Body.Close()
	return response.StatusCode == http.StatusOK
}
