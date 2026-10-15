package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

type connectionAttempt struct {
	machine  Machine
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

// ConnectMachine is the user-facing checked connection. SaveMachine remains the
// lower-level configuration edit, including saving deliberately offline entries.
func (s *Service) ConnectMachine(ctx context.Context, in MachineEdit) (ConnectionProgress, error) {
	if err := s.validateMachine(in.Machine); err != nil {
		return s.Connection(in.Machine.ID), err
	}
	if in.ConnectAnyway {
		s.mu.Lock()
		attempt := s.checks[in.Machine.ID]
		checked, _ := json.Marshal(attempt.machine)
		requested, _ := json.Marshal(in.Machine)
		valid := attempt.progress.Complete && attempt.progress.CanConnect && !attempt.running && string(checked) == string(requested)
		s.mu.Unlock()
		if !valid {
			return s.Connection(in.Machine.ID), fmt.Errorf("the machine changed or has a fatal check; run Connect again before saving")
		}
	} else {
		if err := s.checkConnection(ctx, in.Machine); err != nil {
			return s.Connection(in.Machine.ID), err
		}
		if progress := s.Connection(in.Machine.ID); progress.CanConnect {
			return progress, nil
		}
	}
	// The checks above established readiness. Do not repeat bootstrap or submit
	// another helper operation merely to persist the same checked configuration.
	in.ConnectAnyway = true
	if _, err := s.SaveMachine(ctx, in); err != nil {
		return s.Connection(in.Machine.ID), err
	}
	s.mu.Lock()
	attempt := s.checks[in.Machine.ID]
	attempt.progress.Saved = true
	attempt.progress.CanConnect = false
	s.publishConnectionLocked(in.Machine.ID, attempt)
	s.mu.Unlock()
	return s.Connection(in.Machine.ID), nil
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

func (s *Service) checkConnection(ctx context.Context, m Machine) (retErr error) {
	s.mu.Lock()
	if s.checks[m.ID].running {
		s.mu.Unlock()
		return fmt.Errorf("connection checks are already running for this machine")
	}
	s.publishConnectionLocked(m.ID, connectionAttempt{machine: m, running: true, progress: ConnectionProgress{ID: m.ID, Checks: []ConnectionCheck{}}})
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		attempt := s.checks[m.ID]
		attempt.running = false
		attempt.progress.Complete = true
		attempt.progress.CanConnect = retErr == nil && slices.ContainsFunc(attempt.progress.Checks, func(check ConnectionCheck) bool { return check.State == "warning" })
		s.publishConnectionLocked(m.ID, attempt)
		s.mu.Unlock()
	}()
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
			return failed("sign-in", "SSH sign-in", err, "Gimbal cannot reach this machine. Nothing was saved.", "Open in Terminal to verify SSH access and host trust.")
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
		endpoint, err = s.sshEndpoint(ctx, m, true)
		if err != nil {
			return failed("daemon", "Daemon running", err, "No daemon became reachable through the SSH tunnel.", "Open in Terminal to check the daemon and its workspace root.")
		}
		completed("daemon", "Daemon running", "An existing or newly started daemon answers through the SSH tunnel.")
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
	inspectionLabel := "Harnesses"
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
		return failed("inspection", inspectionLabel, err, "Machine and harness readiness could not be inspected; session readiness is unknown.", fix)
	}
	completed("inspection", inspectionLabel, "The machine's Gimbal helper returned harness and build facts.")
	if m.Kind == KindRoute {
		pending("harnesses", "Harnesses")
	}
	harnessID := "inspection"
	if m.Kind == KindRoute {
		harnessID = "harnesses"
	}
	problems := []string{}
	for _, harness := range result.Harnesses {
		switch harness.State {
		case HarnessMissing:
			problems = append(problems, harness.ID+" is not installed")
		case HarnessSignInRequired:
			problems = append(problems, harness.ID+" needs sign-in")
		case HarnessUnknown:
			problems = append(problems, harness.ID+" sign-in is unverified")
		}
	}
	if len(result.Harnesses) == 0 {
		problems = append(problems, "No harness availability was reported")
	}
	if len(problems) != 0 {
		s.connectionCheck(m.ID, ConnectionCheck{ID: harnessID, Label: "Harnesses", State: "warning", Cause: strings.Join(problems, "; ") + ".", Consequence: "Only installed harnesses can start sessions; an unverified harness may require sign-in before its first turn.", Fix: "Open in Terminal on " + m.Name + " to install or sign in to the harness you want."})
	} else {
		completed(harnessID, "Harnesses", "Harnesses are ready on "+m.Name+".")
	}
	pending("build", "Gimbal build")
	localBuild, _ := localFacts()
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
