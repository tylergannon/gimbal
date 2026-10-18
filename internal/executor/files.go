// Package executor contains the small file/ownership helpers shared by the two
// local executor protocols. It does not abstract commands and agent turns.
package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/sys/unix"
)

type CommandID string
type SessionID string
type TurnID string

func NewCommandID() CommandID { return CommandID("command_" + ulid.Make().String()) }
func NewSessionID() SessionID { return SessionID("session_" + ulid.Make().String()) }
func NewTurnID() TurnID       { return TurnID("turn_" + ulid.Make().String()) }
func Validate(id, prefix string) error {
	if len(id) != len(prefix)+26 || id[:len(prefix)] != prefix {
		return fmt.Errorf("invalid %s identity %q", prefix, id)
	}
	_, err := ulid.ParseStrict(id[len(prefix):])
	return err
}
func Root(path string) (string, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, ".gimbal", "executors")
	}
	return filepath.Abs(path)
}
func Write(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func Read(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
func Exists(path string) bool { _, err := os.Stat(path); return err == nil }

type Lease struct{ file *os.File }

func Lock(dir string) (*Lease, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "owner.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, nil
		}
		return nil, err
	}
	return &Lease{f}, nil
}
func (l *Lease) Close() { _ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN); _ = l.file.Close() }

// Accept is called only with the operation's lease held, before native work.
func Accept(dir string, input any) error {
	path := filepath.Join(dir, "request.json")
	if Exists(path) {
		return fmt.Errorf("operation already accepted")
	}
	return Write(path, input)
}
func Match(dir string, input any) error {
	want, err := json.Marshal(input)
	if err != nil {
		return err
	}
	have, err := os.ReadFile(filepath.Join(dir, "request.json"))
	if err != nil {
		return err
	}
	if !bytes.Equal(bytes.TrimSpace(have), want) {
		return errors.New("operation identity conflicts with accepted inputs")
	}
	return nil
}

// AssertOwner rechecks the authoritative result after acquiring the lease;
// socket/connection failure alone is never evidence of owner loss.
func AssertOwner(dir string) {
	lease, err := Lock(dir)
	if err != nil {
		panic(fmt.Sprintf("executor ownership check %s: %v", dir, err))
	}
	if lease == nil {
		return
	}
	defer lease.Close()
	if Exists(filepath.Join(dir, "request.json")) && !Exists(filepath.Join(dir, "result.json")) {
		panic(fmt.Sprintf("executor invariant violation: accepted operation %s has neither a live owner nor a terminal result", dir))
	}
}

type Descriptor struct {
	State  string  `json:"state"`
	Socket *string `json:"socket"`
	Events string  `json:"events,omitempty"`
	Stdout string  `json:"stdout,omitempty"`
	Stderr string  `json:"stderr,omitempty"`
	Result string  `json:"result"`
}

func Describe(dir string, agent bool) Descriptor {
	socket := SocketPath(dir)
	d := Descriptor{State: "running", Socket: &socket, Result: filepath.Join(dir, "result.json")}
	if agent {
		d.Events = filepath.Join(dir, "events.jsonl")
	} else {
		d.Stdout = filepath.Join(dir, "stdout.log")
		d.Stderr = filepath.Join(dir, "stderr.log")
	}
	if Exists(d.Result) {
		d.State = "completed"
		d.Socket = nil
	}
	return d
}
func SocketPath(dir string) string {
	sum := sha256.Sum256([]byte(dir))
	return filepath.Join(os.TempDir(), fmt.Sprintf("gimbal-executor-%d-%x.sock", os.Getuid(), sum[:12]))
}

// Launch starts an independent owner. The startup pipe has one descriptor and
// closes before native execution. No caller context or inherited stdio owns it.
func Launch(binary string, args []string, input any, root string) (Descriptor, error) {
	var d Descriptor
	if err := os.MkdirAll(root, 0700); err != nil {
		return d, err
	}
	log, err := os.OpenFile(filepath.Join(root, "diagnostics.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return d, err
	}
	defer log.Close()
	read, write, err := os.Pipe()
	if err != nil {
		return d, err
	}
	defer read.Close()
	raw, err := json.Marshal(input)
	if err != nil {
		write.Close()
		return d, err
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.ExtraFiles = []*os.File{write}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err != nil {
		write.Close()
		return d, err
	}
	write.Close()
	go func() { _ = cmd.Wait() }()
	_ = read.SetReadDeadline(time.Now().Add(2 * time.Minute))
	var response struct {
		Descriptor
		Error string `json:"error,omitempty"`
	}
	err = json.NewDecoder(read).Decode(&response)
	d = response.Descriptor
	if err == nil && response.Error != "" {
		return d, errors.New(response.Error)
	}
	if err != nil {
		return d, fmt.Errorf("executor startup: %w (diagnostics: %s)", err, log.Name())
	}
	return d, nil
}

type Control struct {
	Deadline time.Time `json:"deadline,omitempty"`
	Op       string    `json:"op"`
	After    int64     `json:"after,omitempty"`
	Message  string    `json:"message,omitempty"`
}
type Reply struct {
	Landed bool   `json:"landed,omitempty"`
	Error  string `json:"error,omitempty"`
}
type Server struct {
	dir      string
	listener net.Listener
	mu       sync.Mutex
	changed  chan struct{}
	done     chan struct{}
	once     sync.Once
}

func Listen(dir string, control func(Control) Reply) (*Server, error) {
	path := SocketPath(dir)
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		listener.Close()
		return nil, err
	}
	s := &Server{dir: dir, listener: listener, changed: make(chan struct{}), done: make(chan struct{})}
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			go s.serve(c, control)
		}
	}()
	return s, nil
}
func (s *Server) Notify() {
	s.mu.Lock()
	close(s.changed)
	s.changed = make(chan struct{})
	s.mu.Unlock()
}
func (s *Server) Close() {
	s.once.Do(func() { close(s.done); _ = s.listener.Close(); _ = os.Remove(s.listener.Addr().String()); s.Notify() })
}
func (s *Server) serve(c net.Conn, control func(Control) Reply) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(30 * time.Second))
	var req Control
	if json.NewDecoder(c).Decode(&req) != nil {
		return
	}
	var reply Reply
	if req.Op == "wait" {
		s.mu.Lock()
		changed := s.changed
		s.mu.Unlock()
		size := int64(0)
		if info, err := os.Stat(filepath.Join(s.dir, "events.jsonl")); err == nil {
			size = info.Size()
		}
		if size == req.After && !Exists(filepath.Join(s.dir, "result.json")) {
			select {
			case <-changed:
			case <-s.done:
			case <-time.After(time.Second):
			}
		}
	} else {
		reply = control(req)
	}
	_ = json.NewEncoder(c).Encode(reply)
}
func Call(ctx context.Context, dir string, request Control) (Reply, error) {
	var reply Reply
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", SocketPath(dir))
	if err != nil {
		return reply, err
	}
	defer conn.Close()
	deadline := time.Now().Add(30 * time.Second)
	if request.Op == "wait" {
		deadline = time.Now().Add(2 * time.Second)
	}
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	request.Deadline = deadline
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err = json.NewEncoder(conn).Encode(request); err == nil {
		err = json.NewDecoder(conn).Decode(&reply)
	}
	if err == nil && reply.Error != "" {
		err = errors.New(reply.Error)
	}
	return reply, err
}
func Wait(ctx context.Context, dir string, after int64) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, err := Call(ctx, dir, Control{Op: "wait", After: after})
	if err != nil {
		AssertOwner(dir)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
}

// Copy preserves complete output on disk without retaining it in memory.
func Copy(dst string, src io.Reader) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, src)
	return errors.Join(err, f.Close())
}

// Vacuum retains terminal operations for seven days, which is also their
// deduplication lifetime. Session histories are never removed here.
func Vacuum(root string, now time.Time) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "sessions" {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		info, err := os.Stat(filepath.Join(dir, "result.json"))
		if err != nil || now.Sub(info.ModTime()) < 7*24*time.Hour {
			continue
		}
		lease, err := Lock(dir)
		if err != nil {
			return err
		}
		if lease == nil {
			continue
		}
		err = os.RemoveAll(dir)
		lease.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func PeriodicVacuum(root string) func() {
	done := make(chan struct{})
	go func() {
		_ = Vacuum(root, time.Now())
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				_ = Vacuum(root, now)
			}
		}
	}()
	return func() { close(done) }
}

// Claim distinguishes a new submission from a retained matching operation.
func Claim(dir string, input any) (*Lease, error) {
	for {
		lease, err := Lock(dir)
		if err != nil {
			return nil, err
		}
		if lease != nil {
			if !Exists(filepath.Join(dir, "request.json")) {
				return lease, nil
			}
			err = Match(dir, input)
			if err == nil && !Exists(filepath.Join(dir, "result.json")) {
				lease.Close()
				panic(fmt.Sprintf("executor invariant violation: accepted operation %s has neither a live owner nor a terminal result", dir))
			}
			lease.Close()
			return nil, err
		}
		if Exists(filepath.Join(dir, "request.json")) {
			return nil, Match(dir, input)
		}
		time.Sleep(time.Millisecond)
	}
}
func Startup(d Descriptor) error {
	f := os.NewFile(3, "startup")
	if f == nil {
		return errors.New("startup descriptor is missing")
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(d)
}

// RejectStartup reports pre-acceptance failures on the private startup pipe.
// The public CLI prints the error on stderr and emits no descriptor.
func RejectStartup(err error) {
	f := os.NewFile(3, "startup")
	if f == nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(struct {
		Error string `json:"error"`
	}{err.Error()})
}
