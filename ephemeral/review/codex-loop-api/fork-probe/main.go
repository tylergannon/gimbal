package main

import (
	"context"
	"fmt"
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/codex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	base, _ := filepath.Abs("ephemeral/review/loop-api/fork-probe")
	os.Setenv("PATH", filepath.Join(base, "bin")+":"+os.Getenv("PATH"))
	err := gimble.Run(gimble.Project(context.Background(), filepath.Join(base, "project")), "fork-leak", func(ctx context.Context) error {
		s := gimble.NewSession(ctx, "researcher", codex.New(), "fake", ".")
		if _, err := s.Generate[gimble.Text](ctx, "prime"); err != nil {
			return err
		}
		return gimble.Scope(ctx, "candidate", func(ctx context.Context) error { _, err := s.Fork(ctx, "unused"); return err })
	})
	fmt.Printf("Run returned: %v\n", err)
	raw, _ := os.ReadFile(filepath.Join(base, "pids.txt"))
	for _, line := range strings.Fields(string(raw)) {
		pid, _ := strconv.Atoi(line)
		err := syscall.Kill(pid, 0)
		fmt.Printf("mock app-server PID %d alive after Run and scope close: %t\n", pid, err == nil)
		if err == nil {
			syscall.Kill(pid, syscall.SIGTERM)
		}
	}
}
