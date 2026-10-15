package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

type connectionView struct {
	cancel   context.CancelFunc
	closedAt time.Time
}

// WatchConnectionView gives observation an explicit lifetime independent of the
// client framework's garbage collector. Closing it never cancels the attempt.
func (s *Service) WatchConnectionView(ctx context.Context, in ConnectionSubscription, yield func(ConnectionProgress) error) error {
	if in.View == "" || in.Machine == "" {
		return fmt.Errorf("connection view and machine required")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	view := &connectionView{cancel: cancel}
	s.mu.Lock()
	previous := s.views[in.View]
	if previous != nil && !previous.closedAt.IsZero() {
		s.mu.Unlock()
		return nil
	}
	if previous != nil {
		previous.cancel()
	}
	s.views[in.View] = view
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.views[in.View] == view {
			delete(s.views, in.View)
		}
		s.mu.Unlock()
	}()
	return s.WatchConnection(ctx, in.Machine, yield)
}

func (s *Service) CloseConnectionView(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if view := s.views[id]; view != nil && view.cancel != nil {
		view.cancel()
	}
	// Preserve a short tombstone so close-before-subscribe cannot leak a stream.
	// The existing service maintenance loop expires it.
	s.views[id] = &connectionView{closedAt: time.Now()}
}

type connectionAttempt struct {
	machine  Machine
	revision string
	progress ConnectionProgress
	running  bool
	changed  chan struct{}
}

// Connection returns a snapshot of the checks for one draft or saved machine.
func (s *Service) Connection(id string) ConnectionProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return connectionSnapshot(id, s.checks[id].progress)
}

func connectionSnapshot(id string, p ConnectionProgress) ConnectionProgress {
	p.ID = id
	p.Checks = slices.Clone(p.Checks)
	if p.Checks == nil {
		p.Checks = []ConnectionCheck{}
	}
	return p
}

// WatchConnection sleeps between actual progress changes. The snapshot and its
// notification channel are captured under one lock, including before any check
// exists, so a change during yield cannot be missed before the following wait.
func (s *Service) WatchConnection(ctx context.Context, id string, yield func(ConnectionProgress) error) error {
	for {
		if ctx.Err() != nil || s.ctx.Err() != nil {
			return nil
		}
		s.mu.Lock()
		attempt := s.checks[id]
		if attempt.changed == nil {
			attempt.changed = make(chan struct{})
			s.checks[id] = attempt
		}
		progress := connectionSnapshot(id, attempt.progress)
		changed := attempt.changed
		s.mu.Unlock()
		if err := yield(progress); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.ctx.Done():
			return nil
		case <-changed:
		}
	}
}

// The caller holds mu. Each watcher keeps the channel that accompanied its last
// snapshot; close wakes those watchers and the replacement serves the next one.
func (s *Service) publishConnectionLocked(id string, attempt connectionAttempt) {
	if changed := s.checks[id].changed; changed != nil {
		close(changed)
	}
	attempt.changed = make(chan struct{})
	s.checks[id] = attempt
}

// ConnectMachine checks the submitted configuration once, then either saves it
// or waits for deliberate confirmation of this exact attempt's warnings.
func (s *Service) ConnectMachine(ctx context.Context, in MachineEdit) (ConnectionProgress, error) {
	if err := s.validateMachine(in.Machine); err != nil {
		return s.Connection(in.Machine.ID), err
	}
	s.mu.Lock()
	if err := s.refreshLocked(); err != nil {
		s.mu.Unlock()
		return s.Connection(in.Machine.ID), err
	}
	if !s.document.Valid {
		s.mu.Unlock()
		return s.Connection(in.Machine.ID), fmt.Errorf("repair malformed environments.json before connecting")
	}
	if s.document.Revision != in.Revision {
		s.mu.Unlock()
		return s.Connection(in.Machine.ID), fmt.Errorf("configuration changed; reload Machines before connecting")
	}
	attempt := s.checks[in.Machine.ID]
	if s.closed || attempt.running {
		s.mu.Unlock()
		return s.Connection(in.Machine.ID), fmt.Errorf("connection checks are already running or the service closed")
	}
	if in.ConnectAnyway {
		checked, _ := json.Marshal(attempt.machine)
		requested, _ := json.Marshal(in.Machine)
		if in.Attempt == "" || in.Attempt != attempt.progress.Attempt || in.Revision != attempt.revision || !attempt.progress.Complete || !attempt.progress.CanConnect || string(checked) != string(requested) {
			s.mu.Unlock()
			return s.Connection(in.Machine.ID), fmt.Errorf("the machine or checks changed; run Connect again before saving")
		}
		// Consume confirmation under the same lock as the revision-checked write.
		err := ctx.Err()
		if err == nil {
			err = s.editLocked(in.Revision, "machines", in.Machine.ID, in.Machine, false)
		}
		if err == nil {
			attempt.progress.Saved = true
			attempt.progress.CanConnect = false
			s.publishConnectionLocked(in.Machine.ID, attempt)
		}
		progress := connectionSnapshot(in.Machine.ID, attempt.progress)
		s.mu.Unlock()
		return progress, err
	}
	s.beginConnectionLocked(in.Machine, in.Revision)
	s.mu.Unlock()
	err := s.runConnectionChecks(ctx, in.Machine)
	s.mu.Lock()
	attempt = s.checks[in.Machine.ID]
	attempt.running = false
	attempt.progress.Complete = true
	attempt.progress.CanConnect = err == nil && slices.ContainsFunc(attempt.progress.Checks, func(check ConnectionCheck) bool { return check.State == "warning" })
	if err == nil && !attempt.progress.CanConnect {
		err = ctx.Err()
		if err == nil {
			err = s.editLocked(in.Revision, "machines", in.Machine.ID, in.Machine, false)
		}
		attempt.progress.Saved = err == nil
	}
	s.publishConnectionLocked(in.Machine.ID, attempt)
	progress := connectionSnapshot(in.Machine.ID, attempt.progress)
	s.mu.Unlock()
	return progress, err
}

// The caller holds mu until this attempt is reserved through its final save.
func (s *Service) beginConnectionLocked(m Machine, revision string) {
	s.publishConnectionLocked(m.ID, connectionAttempt{machine: m, revision: revision, running: true, progress: ConnectionProgress{ID: m.ID, Attempt: ulid.Make().String(), Checks: []ConnectionCheck{}}})
}

func (s *Service) connectionCheck(id string, check ConnectionCheck) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt := s.checks[id]
	index := slices.IndexFunc(attempt.progress.Checks, func(item ConnectionCheck) bool { return item.ID == check.ID })
	if index < 0 {
		attempt.progress.Checks = append(attempt.progress.Checks, check)
	} else {
		attempt.progress.Checks[index] = check
	}
	s.publishConnectionLocked(id, attempt)
}

func (s *Service) checkConnection(ctx context.Context, m Machine) error {
	s.mu.Lock()
	if s.closed || s.checks[m.ID].running {
		s.mu.Unlock()
		return fmt.Errorf("connection checks are already running or the service closed")
	}
	s.beginConnectionLocked(m, "")
	s.mu.Unlock()
	err := s.runConnectionChecks(ctx, m)
	s.mu.Lock()
	attempt := s.checks[m.ID]
	attempt.running = false
	attempt.progress.Complete = true
	// Reconnecting a saved machine does not grant a draft save confirmation.
	s.publishConnectionLocked(m.ID, attempt)
	s.mu.Unlock()
	return err
}

func (s *Service) runConnectionChecks(ctx context.Context, m Machine) error {
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	defer func() { stop(); cancel() }()
	pending := func(id, label string) {
		s.connectionCheck(m.ID, ConnectionCheck{ID: id, Label: label, State: "pending"})
	}
	completed := func(id, label, cause string) {
		s.connectionCheck(m.ID, ConnectionCheck{ID: id, Label: label, State: "completed", Cause: cause})
	}
	failed := func(id, label string, err error, consequence, fix string) error {
		s.connectionCheck(m.ID, ConnectionCheck{ID: id, Label: label, State: "failed", Cause: err.Error(), Consequence: consequence, Fix: fix, Fatal: true})
		s.setMachineError(m, err)
		return err
	}
	endpoint := strings.TrimRight(m.Address, "/")
	if m.Kind == KindSSH {
		pending("sign-in", "SSH sign-in")
		if _, err := connectionSSH(ctx, m, "true"); err != nil {
			return failed("sign-in", "SSH sign-in", err, "Gimbal cannot reach this machine. New sessions cannot start; retained sessions may still be running.", "Open in Terminal to verify SSH access and host trust.")
		}
		completed("sign-in", "SSH sign-in", "Signed in using your system SSH configuration.")
		pending("executable", "Gimbal found")
		executable := m.Executable
		if executable == "" {
			executable = "gimbal"
		}
		path, err := connectionSSH(ctx, m, "command -v "+shellQuote(executable))
		if err != nil {
			return failed("executable", "Gimbal found", err, "A daemon cannot be started without Gimbal installed on this machine.", "Install Gimbal on "+m.Name+" in Terminal, or correct the executable path.")
		}
		completed("executable", "Gimbal found", strings.TrimSpace(path))
		pending("daemon", "Daemon running")
		startedDaemon := false
		endpoint, err = s.sshConnection(ctx, m, true, func() {
			startedDaemon = true
			s.connectionCheck(m.ID, ConnectionCheck{ID: "daemon", Label: "Daemon running", State: "pending", Cause: "Daemon did not answer. Starting the installed Gimbal."})
		})
		if err != nil {
			return failed("daemon", "Daemon running", err, "No daemon became reachable through the SSH tunnel.", "Open in Terminal to check the daemon and its workspace root.")
		}
		detail := "The existing daemon answers through the SSH tunnel."
		if startedDaemon {
			detail = "Started the installed Gimbal; its daemon now answers through the SSH tunnel."
		}
		completed("daemon", "Daemon running", detail)
	}
	if m.Kind != KindRoute {
		pending("health", "Daemon health")
		started := time.Now()
		if err := health(ctx, endpoint); err != nil {
			return failed("health", "Daemon health", err, "Sessions cannot start until this daemon answers.", "Check the daemon address and that Gimbal is running on the machine.")
		}
		latency := time.Since(started).Milliseconds()
		completed("health", "Daemon health", fmt.Sprintf("Connected · %d ms round trip", latency))
		s.mu.Lock()
		state := s.states[m.ID]
		state.LatencyMS = latency
		s.states[m.ID] = state
		s.mu.Unlock()
	}
	inspectionLabel := "Harness and build inspection"
	if m.Kind == KindRoute {
		inspectionLabel = "Worker inspection"
	}
	pending("inspection", inspectionLabel)
	result, err := s.operation(ctx, Target{Machine: m}, HelperRequest{Operation: "inspect"}, "")
	if err != nil {
		fix := "Open in Terminal to check the Gimbal executable and daemon on this machine."
		if m.Kind == KindRoute {
			fix = "Check the backend route in environments.json and that its worker can run Gimbal."
		}
		return failed("inspection", inspectionLabel, err, "Machine and harness readiness could not be inspected. Existing sessions remain owned; readiness for new sessions is unknown.", fix)
	}
	completed("inspection", inspectionLabel, "The machine's Gimbal helper returned harness and build facts.")
	problems := []string{}
	for _, harness := range result.Harnesses {
		switch harness.State {
		case HarnessMissing:
			problems = append(problems, harness.ID+" is not installed")
		case HarnessSignInRequired:
			problems = append(problems, harness.ID+" needs sign-in")
		case HarnessReady:
			continue
		default:
			problems = append(problems, harness.ID+" sign-in is unverified")
		}
	}
	if len(result.Harnesses) == 0 {
		problems = append(problems, "No harness availability was reported")
	}
	if len(problems) != 0 {
		s.connectionCheck(m.ID, ConnectionCheck{ID: "harnesses", Label: "Harnesses", State: "warning", Cause: strings.Join(problems, "; ") + ".", Consequence: "Missing harnesses cannot start sessions; signed-out harnesses need sign-in. Unverified sign-in may fail on the first turn. Other ready harnesses remain available.", Fix: "Open in Terminal on " + m.Name + " to install or sign in to the harness you want."})
	} else {
		completed("harnesses", "Harnesses", "Harnesses are ready on "+m.Name+".")
	}
	localBuild := buildRevision()
	if result.Build == "" || result.Build == "unknown" || localBuild == "" || localBuild == "unknown" {
		s.connectionCheck(m.ID, ConnectionCheck{ID: "build", Label: "Gimbal build", State: "warning", Cause: "Build compatibility could not be verified.", Consequence: "Sessions are allowed. Native workflow plugins may fail to load when builds differ.", Fix: "Install the same Gimbal build on this Mac and " + m.Name + " before running workflows there."})
	} else if result.Build != localBuild {
		s.connectionCheck(m.ID, ConnectionCheck{ID: "build", Label: "Gimbal build", State: "warning", Cause: "This Mac: " + localBuild + ". " + m.Name + ": " + result.Build + ".", Consequence: "Sessions are allowed. Native workflow plugins may fail to load because builds differ.", Fix: "Update Gimbal on " + m.Name + " to this Mac's build before running workflows there."})
	} else {
		completed("build", "Gimbal build", "Reported build matches this Mac · "+result.Build)
	}
	return nil
}

func connectionSSH(ctx context.Context, m Machine, command string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	args := append(sshArgs(m), "--", m.SSH.Host, command)
	output, err := exec.CommandContext(runCtx, "ssh", args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if len(detail) > 2048 {
			detail = detail[:2048]
		}
		return "", fmt.Errorf("SSH %s: %w %s", m.SSH.Host, err, detail)
	}
	return string(output), nil
}
