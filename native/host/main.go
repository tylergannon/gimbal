package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
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
}

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
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(server)
		r.Out.Header.Set("X-Gimbal-Desktop", "macos")
		if r.In.Header.Get("Origin") != "" {
			r.Out.Header.Set("Origin", server.String())
		}
		if ref, err := url.Parse(r.In.Header.Get("Referer")); err == nil && ref.Host != "" {
			ref.Scheme = server.Scheme
			ref.Host = server.Host
			r.Out.Header.Set("Referer", ref.String())
		}
	}, ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) {
		message := "Gimbal server unavailable. Check the configured external server; pending recording text is not saved."
		if strings.TrimSpace(os.Getenv("GIMBAL_SERVER_URL")) == "" {
			message = "Gimbal's bundled server is unavailable. Quit and reopen the app to restart it; pending recording text is not saved."
		}
		http.Error(w, message, http.StatusServiceUnavailable)
	}}
	return proxy, nil
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
	cmd := exec.Command(path,
		"--instance-dir", instanceDir(home),
		"--no-project", "--port", "0", "--ready")
	cmd.Env = serverEnvironment(home)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("prepare bundled Gimbal server startup: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start bundled Gimbal server: %w", err)
	}
	stopped := make(chan struct{})
	bundledServer.cmd = cmd
	bundledServer.stopped = stopped
	go func() {
		_ = cmd.Wait()
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
}

//export SKGoHostStart
func SKGoHostStart() *C.char {
	origin, err := host.Start()
	if err != nil {
		log.Print(err)
		return nil
	}
	return C.CString(origin)
}

//export SKGoHostRecover
func SKGoHostRecover() C.int {
	if host.Recover() != nil {
		return 0
	}
	return 1
}

//export SKGoHostStop
func SKGoHostStop() {
	_ = host.Stop()
	bundledServer.Lock()
	stopBundledServerLocked()
	bundledServer.Unlock()
}
func main() {}
