package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/skgo/nativehost"
)

var bundledServer struct {
	sync.Mutex
	cmd     *exec.Cmd
	stopped chan struct{}
	// owner is the write end of the server's stdin. It is never written; the server stops when it
	// closes, which the kernel does when this process exits, so a crash or force quit can't orphan it.
	owner io.WriteCloser
}

var serverOrigin string

// The desktop owns only the CLI child it starts. An explicit server URL stays
// external and is never discovered, stopped, or otherwise managed here.
var host = nativehost.New(func(origin string) (http.Handler, error) {
	target, err := serverTarget()
	if err != nil {
		return nil, err
	}
	server, err := url.Parse(target)
	if err != nil || server.Host == "" {
		return nil, fmt.Errorf("invalid GIMBAL_SERVER_URL")
	}
	if server.Scheme != "http" && server.Scheme != "https" || server.User != nil || server.Path != "" || server.RawQuery != "" || server.Fragment != "" {
		return nil, fmt.Errorf("GIMBAL_SERVER_URL must be an HTTP(S) origin")
	}
	serverOrigin = server.String()
	return newServerProxy(server, origin, keychainStore{account: serverOrigin})
})

func bundledServerPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate Gimbal.app executable: %w", err)
	}
	path := filepath.Join(filepath.Dir(filepath.Dir(executable)), "Resources", "gimbal")
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("bundled Gimbal server is missing at %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("bundled Gimbal server is not an executable file: %s", path)
	}
	return path, nil
}

func serverTarget() (string, error) {
	if target := strings.TrimSpace(os.Getenv("GIMBAL_SERVER_URL")); target != "" {
		return target, nil
	}
	return startBundledServer()
}

func startBundledServer() (string, error) {
	bundledServer.Lock()
	defer bundledServer.Unlock()
	if bundledServer.cmd != nil {
		select {
		case <-bundledServer.stopped:
			bundledServer.cmd = nil
			bundledServer.stopped = nil
			bundledServer.owner = nil
		default:
			return "", fmt.Errorf("bundled Gimbal server is already running")
		}
	}
	path, err := bundledServerPath()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("locate the user's home directory for Gimbal state")
	}
	args, err := bundledServerArguments(instanceDir(home))
	if err != nil {
		return "", err
	}
	cmd := exec.Command(path, args...)
	cmd.Env = serverEnvironment(home)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("prepare bundled Gimbal server startup: %w", err)
	}
	owner, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("prepare bundled Gimbal server startup: %w", err)
	}
	cmd.Stderr = serverDiagnostics{}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start bundled Gimbal server: %w", err)
	}
	writeDiagnostic("server", fmt.Sprintf("Started bundled server pid=%d", cmd.Process.Pid))
	stopped := make(chan struct{})
	bundledServer.cmd = cmd
	bundledServer.stopped = stopped
	bundledServer.owner = owner
	go func() {
		err := cmd.Wait()
		writeDiagnostic("server", fmt.Sprintf("Bundled server pid=%d exited: %v", cmd.Process.Pid, err))
		close(stopped)
	}()
	ready := make(chan string, 1)
	readErr := make(chan error, 1)
	reader := bufio.NewReader(stdout)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			readErr <- err
			return
		}
		ready <- strings.TrimSpace(line)
	}()
	select {
	case line := <-ready:
		if !strings.HasPrefix(line, "READY http://127.0.0.1:") {
			stopBundledServerLocked()
			return "", fmt.Errorf("bundled Gimbal server returned an invalid readiness message")
		}
		serverURL := strings.TrimPrefix(line, "READY ")
		parsed, err := url.Parse(serverURL)
		if err != nil || parsed.Host == "" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" {
			stopBundledServerLocked()
			return "", fmt.Errorf("bundled Gimbal server returned an invalid loopback URL")
		}
		go func() { _, _ = io.Copy(io.Discard, reader) }()
		return serverURL, nil
	case err := <-readErr:
		stopBundledServerLocked()
		return "", fmt.Errorf("bundled Gimbal server did not become ready: %w", err)
	case <-stopped:
		return "", fmt.Errorf("bundled Gimbal server exited before becoming ready")
	case <-time.After(20 * time.Second):
		stopBundledServerLocked()
		return "", fmt.Errorf("bundled Gimbal server did not become ready within 20 seconds")
	}
}

// bundledServerArguments loads compatible plugins installed for this catalog.
// An isolated catalog never inherits plugins from the personal desktop catalog.
func bundledServerArguments(directory string) ([]string, error) {
	args := []string{"--instance-dir", directory, "--no-project", "--port", "0", "--ready", "--owned-by-stdin"}
	plugins := filepath.Join(directory, "plugins")
	if info, err := os.Stat(plugins); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("gimbal plugin location is not a directory: %s", plugins)
		}
		args = append(args, "--plugin-dir", plugins)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read Gimbal plugin directory: %w", err)
	}
	return args, nil
}

// instanceDir is the bundled server's state directory: GIMBAL_INSTANCE_DIR when set, so one app
// can run against several catalogs, else ~/.gimbal/desktop.
func instanceDir(home string) string {
	if dir := strings.TrimSpace(os.Getenv("GIMBAL_INSTANCE_DIR")); dir != "" {
		return dir
	}
	return filepath.Join(home, ".gimbal", "desktop")
}

func serverEnvironment(home string) []string {
	env := os.Environ()
	path := ""
	for i, entry := range env {
		if strings.HasPrefix(entry, "PATH=") {
			path = strings.TrimPrefix(entry, "PATH=")
			env = append(env[:i], env[i+1:]...)
			break
		}
	}
	paths := make([]string, 0, 9)
	seen := make(map[string]bool, 9)
	for _, entry := range strings.Split(path, string(os.PathListSeparator)) {
		if entry != "" {
			paths = append(paths, entry)
			seen[entry] = true
		}
	}
	for _, entry := range []string{
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".local", "share", "mise", "shims"),
		"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin",
	} {
		if !seen[entry] {
			paths = append(paths, entry)
			seen[entry] = true
		}
	}
	return append(env, "PATH="+strings.Join(paths, string(os.PathListSeparator)))
}

func stopBundledServerLocked() {
	cmd, stopped := bundledServer.cmd, bundledServer.stopped
	if cmd == nil || stopped == nil {
		return
	}
	select {
	case <-stopped:
	default:
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-stopped
		}
	}
	bundledServer.cmd = nil
	bundledServer.stopped = nil
	bundledServer.owner = nil
}

//export SKGoHostStart
func SKGoHostStart() *C.char {
	writeDiagnostic("host", "Starting desktop host")
	origin, err := host.Start()
	if err != nil {
		writeDiagnostic("host", "Startup failed: "+err.Error())
		return nil
	}
	return C.CString(origin)
}

//export SKGoHostRecover
func SKGoHostRecover() C.int {
	if err := host.Recover(); err != nil {
		writeDiagnostic("host", "Recovery failed: "+err.Error())
		return 0
	}
	writeDiagnostic("host", "Recovery completed")
	return 1
}

//export SKGoHostServerURL
func SKGoHostServerURL() *C.char { return C.CString(serverOrigin) }

//export SKGoHostStop
func SKGoHostStop() {
	writeDiagnostic("host", "Stopping desktop host")
	_ = host.Stop()
	bundledServer.Lock()
	stopBundledServerLocked()
	bundledServer.Unlock()
}

//export SKGoHostDiagnostic
func SKGoHostDiagnostic(component, message *C.char) {
	writeDiagnostic(C.GoString(component), C.GoString(message))
}

//export SKGoHostLogsDirectory
func SKGoHostLogsDirectory() *C.char {
	d, err := desktopDiagnostics()
	if err != nil {
		return nil
	}
	return C.CString(d.dir)
}

//export SKGoHostExportDiagnostics
func SKGoHostExportDiagnostics(destination *C.char) *C.char {
	writeDiagnostic("host", "Exporting diagnostics")
	if err := exportDiagnostics(C.GoString(destination)); err != nil {
		writeDiagnostic("host", "Diagnostics export failed: "+err.Error())
		return C.CString(err.Error())
	}
	return nil
}
func main() {}
