package environments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

type operation struct {
	Request        backend.MessageRequest
	Input          HelperRequest
	Target         Target
	Result         *HelperResult
	Attempted      bool
	CleanupPending bool
	Error          string
}

func (s *Service) Inspect(ctx context.Context, t Target, path string) (Directory, error) {
	r, e := s.operation(ctx, t, HelperRequest{Operation: "inspect", Path: path}, "")
	return r.Directory, e
}
func (s *Service) ListDirectories(ctx context.Context, t Target, path string) (Directory, error) {
	r, e := s.operation(ctx, t, HelperRequest{Operation: "list", Path: path}, "")
	return r.Directory, e
}
func (s *Service) Prepare(ctx context.Context, t Target, c Checkout, id string) (Prepared, error) {
	if !validOperationID(id) {
		return Prepared{}, fmt.Errorf("conversation identity required")
	}
	r, e := s.operation(ctx, t, HelperRequest{Operation: "prepare", Path: t.Directory, Checkout: c, OperationID: id}, id)
	r.Prepared.Target = t
	r.Prepared.Target.Directory = r.Prepared.Directory
	r.Prepared.Target.Capabilities = capabilities(t.Machine, r.Directory)
	return r.Prepared, e
}
func (s *Service) Clone(ctx context.Context, t Target, origin, path, id string) (Directory, error) {
	if !validOperationID(id) {
		return Directory{}, fmt.Errorf("clone operation identity required")
	}
	r, e := s.operation(ctx, t, HelperRequest{Operation: "clone", Path: path, Origin: origin, OperationID: id}, id)
	return r.Directory, e
}
func (s *Service) operation(ctx context.Context, t Target, in HelperRequest, id string) (out HelperResult, retErr error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	timeout := 30 * time.Second
	if in.Operation == "prepare" {
		timeout = 2 * time.Minute
	}
	if in.Operation == "clone" {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if t.Machine.ID == "" || t.Machine.ID == Local {
		r, e := RunHelper(ctx, in)
		s.recordFacts(t, r, e)
		return r, e
	}
	if in.Path == "" {
		in.Path = t.Directory
	}
	mutation := id != ""
	if id == "" {
		id = ulid.Make().String()
	}
	owner := backend.Owner{Run: "desktop-probe-" + id, Scope: "probe", Task: in.Operation}
	if in.Operation == "prepare" {
		owner = backend.Owner{Run: id, Scope: id + "/prepare", Task: "prepare"}
	}
	identity := backend.NewExecutionIdentity("process")
	req := effects.Request(identity.RequestID, "identified-process", owner)
	req.ExecutionIdentity = identity
	record := operation{Request: req, Input: in, Target: t}
	journal := filepath.Join(s.options.InstanceDir, "environment-operations", in.Operation+"-"+id+".json")
	if mutation {
		if b, e := os.ReadFile(journal); e == nil {
			var saved operation
			if e = json.Unmarshal(b, &saved); e != nil {
				return HelperResult{}, e
			}
			record = saved
			a, _ := json.Marshal(record.Input)
			b, _ := json.Marshal(in)
			ta, _ := json.Marshal(record.Target)
			tb, _ := json.Marshal(t)
			if string(a) != string(b) || string(ta) != string(tb) {
				return HelperResult{}, fmt.Errorf("operation identity already belongs to different placement/choices")
			}
			if record.CleanupPending {
				b, e := s.binding(ctx, t, record.Request.Owner, false)
				if e == nil {
					e = s.cleanupBinding(ctx, b)
				}
				if e != nil {
					return HelperResult{}, &backend.DeliveryUncertain{RequestID: record.Request.ID, Err: e}
				}
				record.CleanupPending = false
				_ = writeReceipt(journal, record)
			}
			if record.Result != nil || record.Error != "" {

				if record.Error != "" {
					return HelperResult{}, errors.New(record.Error)
				}
				return *record.Result, nil
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return HelperResult{}, e
		}
	}
	if e := writeReceipt(journal, record); e != nil {
		return HelperResult{}, e
	}
	b, e := s.Acquire(ctx, t, record.Request.Owner)
	if e != nil {
		record.CleanupPending = true
		_ = writeReceipt(journal, record)
		if b.Connection.Messages != nil || b.Connection.Execution != nil {
			cleanupErr := s.cleanupBinding(ctx, b)
			if cleanupErr == nil {
				record.CleanupPending = false
				_ = writeReceipt(journal, record)
			}
			e = errors.Join(e, cleanupErr)
		}
		return HelperResult{}, e
	}
	requestCtx := effects.WithEnvironment(effects.WithConnection(ctx, b.Connection), b.Target.Route.Environment)
	executable := t.Machine.Executable
	if executable == "" {
		executable = "gimbal"
	}
	raw, _ := json.Marshal(in)
	p := backend.Process{ExecutionIdentity: record.Request.ExecutionIdentity, Owner: record.Request.Owner, Command: executable, Args: []string{"desktop-environment", string(raw)}}
	record.Request.Process = &p
	cleanup := func() error {
		e := s.cleanupBinding(ctx, b)
		record.CleanupPending = e != nil
		if e != nil {
			_ = writeReceipt(journal, record)
		} else if !mutation {
			_ = os.Remove(journal)
		} else {
			_ = writeReceipt(journal, record)
		}
		return e
	}
	helperFailed := false
	if !mutation {
		defer func() { retErr = errors.Join(retErr, cleanup()) }()
	} else {
		defer func() {
			if retErr != nil {
				var rejected *backend.ContractRejected
				if errors.As(retErr, &rejected) {
					record.Result = nil
					record.Error = ""
					record.Attempted = false
					retErr = errors.Join(retErr, cleanup(), writeReceipt(journal, record))
				} else if helperFailed {
					record.Error = retErr.Error()
					retErr = errors.Join(retErr, cleanup())
				} else {
					var uncertain *backend.DeliveryUncertain
					if !errors.As(retErr, &uncertain) {
						retErr = &backend.DeliveryUncertain{RequestID: record.Request.ID, Err: retErr}
					}
					record.CleanupPending = true
					_ = writeReceipt(journal, record)
				}
			}
		}()
	}
	var stdout []byte
	if b.Connection.Messages != nil {
		if !record.Attempted {
			record.Attempted = true
			if mutation {
				if e = writeReceipt(journal, record); e != nil {
					return HelperResult{}, e
				}
			}
			if e = effects.Submit(requestCtx, record.Request); e != nil {
				return HelperResult{}, e
			}
		}
		event, e := effects.Await(requestCtx, record.Request, "completed", nil)
		if e != nil {
			helperFailed = event.Result != nil && !event.Result.Unresolved && event.Result.Error != ""
			return HelperResult{}, e
		}
		if event.Result == nil || event.Result.Process == nil {
			return HelperResult{}, fmt.Errorf("helper completion has no process result")
		}
		if p := event.Result.Process; p.ExitCode != 0 || p.Error != "" || p.Interrupted {
			helperFailed = true
			return HelperResult{}, fmt.Errorf("desktop helper unavailable or failed; upgrade Gimbal on %s: %s %s", t.Machine.Name, p.Error, event.Result.Stderr)
		}
		stdout = event.Result.Stdout
	} else {
		target := backend.Target{Run: p.Run, ID: p.Task}
		if !record.Attempted {
			record.Attempted = true
			if mutation {
				if e = writeReceipt(journal, record); e != nil {
					return HelperResult{}, e
				}
			}
			if e = b.Connection.Execution.Process(ctx, p); e != nil {
				if reconcile := effects.ConfirmStarted(requestCtx, "backend", target, e); reconcile != nil {
					var rejected *backend.ContractRejected
					if errors.As(reconcile, &rejected) {
						return HelperResult{}, reconcile
					}
					return HelperResult{}, &backend.DeliveryUncertain{RequestID: id, Err: errors.Join(e, reconcile)}
				}
			}
		}
		result, e := effects.Wait(requestCtx, "backend", target, nil)
		if e != nil {
			return HelperResult{}, e
		}
		var pr backend.ProcessResult
		if e = json.Unmarshal(result, &pr); e != nil {
			return HelperResult{}, e
		}
		if pr.ExitCode != 0 || pr.Interrupted || pr.Error != "" {
			helperFailed = true
			er, e := b.Connection.Execution.ReadBytes(ctx, backend.Artifact{Owner: p.Owner, Name: "stderr"})
			detail := pr.Error
			if e == nil {
				bytes, _ := io.ReadAll(io.LimitReader(er, 8192))
				_ = er.Close()
				detail += " " + string(bytes)
			}
			return HelperResult{}, fmt.Errorf("desktop helper unavailable or failed; upgrade Gimbal on %s: %s", t.Machine.Name, detail)
		}
		reader, e := b.Connection.Execution.ReadBytes(ctx, backend.Artifact{Owner: p.Owner, Name: "stdout"})
		if e != nil {
			return HelperResult{}, e
		}
		stdout, e = io.ReadAll(io.LimitReader(reader, 4<<20))
		e = errors.Join(e, reader.Close())
		if e != nil {
			return HelperResult{}, e
		}
	}
	var result HelperResult
	if e = json.Unmarshal(stdout, &result); e != nil {
		return result, fmt.Errorf("desktop helper returned invalid JSON; check remote Gimbal build: %w", e)
	}
	s.recordFacts(t, result, nil)
	if mutation {
		record.Result = &result
		if e = writeReceipt(journal, record); e != nil {
			return result, e
		}
		if e = cleanup(); e != nil {
			return result, e
		}
	}
	return result, nil
}
func (s *Service) recordFacts(t Target, r HelperResult, err error) {
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[t.Machine.ID]
	state.ID = t.Machine.ID
	state.Name = t.Machine.Name
	state.Kind = t.Machine.Kind
	if state.State != StateConnected {
		state.Since = time.Now().UnixMilli()
	}
	state.State = StateConnected
	state.Build = r.Build
	state.Harnesses = r.Harnesses
	state.Capabilities = capabilities(t.Machine, r.Directory)
	state.Detail = "Connected"
	s.states[t.Machine.ID] = state
	if t.Environment.ID != "" {
		s.facts[t.Environment.Project+"\x00"+t.Environment.ID] = EnvironmentRow{Environment: DescribeEnvironment(t.Environment), Directory: r.Directory, Capabilities: state.Capabilities, Observed: time.Now().UnixMilli(), Issues: []Issue{}}
	}
}

func (s *Service) cleanupBinding(ctx context.Context, b Binding) error {
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	cc := effects.WithEnvironment(effects.WithConnection(c, b.Connection), b.Target.Route.Environment)
	if b.environmentID == "" {
		if routing, ok := b.Connection.Orchestrator.(backend.EnvironmentRouting); ok {
			ds, err := routing.Destinations(c, backend.MessageRequest{Owner: b.Owner, Kind: "process", Process: &backend.Process{Owner: b.Owner}})
			if err == nil && len(ds) == 1 {
				b.environmentID = ds[0].ID
			}
		}
	}
	e := effects.Cleanup(cc, "backend", b.Owner.Run, b.Owner.Scope)
	if e == nil {
		s.updateProvisionedState(c, b)
	}
	return e
}
func (s *Service) retryProbeCleanup() {
	entries, _ := os.ReadDir(filepath.Join(s.options.InstanceDir, "environment-operations"))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(s.options.InstanceDir, "environment-operations", entry.Name())
		raw, e := os.ReadFile(path)
		if e != nil {
			continue
		}
		var op operation
		if json.Unmarshal(raw, &op) != nil || !op.CleanupPending || (op.Input.Operation != "inspect" && op.Input.Operation != "list") {
			continue
		}
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		b, e := s.binding(ctx, op.Target, op.Request.Owner, false)
		if e == nil {
			e = s.cleanupBinding(ctx, b)
		}
		cancel()
		if e == nil {
			_ = os.Remove(path)
		}
	}
}
func (s *Service) pendingIssues() []Issue {
	out := []Issue{}
	dir := filepath.Join(s.options.InstanceDir, "environment-operations")
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, e := os.ReadFile(filepath.Join(dir, entry.Name()))
		if e != nil {
			continue
		}
		var op operation
		if json.Unmarshal(raw, &op) == nil && op.CleanupPending {
			out = append(out, Issue{Entry: op.Target.Machine.ID, Index: -1, Message: fmt.Sprintf("%s on %s has an unconfirmed outcome or cleanup (%s); ownership retained", op.Input.Operation, op.Target.Machine.Name, op.Request.ID)})
		}
	}
	return out
}

// Readiness belongs to the orchestrator. Finishing a metadata probe may release
// the last worker, while another conversation's owner may keep it ready.
func (s *Service) updateProvisionedState(ctx context.Context, b Binding) {
	if b.Target.Machine.Kind != KindRoute || b.Target.Route.Environment == "" || b.Connection.Orchestrator == nil {
		return
	}
	routing, ok := b.Connection.Orchestrator.(backend.EnvironmentRouting)
	if !ok {
		return
	}
	destinations, err := routing.ActiveEnvironments(ctx)
	state, detail := StateNotStarted, "Starts when needed"
	if err != nil {
		state = StateUnreachable
		detail = "Environment readiness is unavailable: " + err.Error()
	} else if b.environmentID == "" && len(destinations) > 0 {
		state = StateConnecting
		detail = "Checking environment readiness"
	} else {
		for _, d := range destinations {
			if d.ID == b.environmentID {
				state = StateConnected
				detail = "Connected"
				break
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, found := s.machineLocked(b.Target.Machine.ID); found {
		a, _ := json.Marshal(current)
		v, _ := json.Marshal(b.Target.Machine)
		if string(a) != string(v) {
			return
		}
	}
	st := s.states[b.Target.Machine.ID]
	st.ID = b.Target.Machine.ID
	st.Name = b.Target.Machine.Name
	st.Kind = b.Target.Machine.Kind
	if st.State != state {
		st.Since = time.Now().UnixMilli()
	}
	st.State = state
	st.Detail = detail
	s.states[st.ID] = st
}
