package taskdaemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/backend"
)

func TestProducerWithoutSecretsRejectsBeforeNativeWork(t *testing.T) {
	daemon := New(t.TempDir())
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	process := backend.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Run: "run", Task: "producer", Command: "sh", Args: []string{"-c", `touch "$1"`, "sh", marker}, Produces: true, Store: "private", Yields: []string{"TOKEN"}}
	if err := daemon.process(process); err == nil || !strings.Contains(err.Error(), "secrets backend") {
		t.Fatalf("producer admission: %v", err)
	}
	if len(daemon.tasks) != 0 {
		t.Fatal("native process was admitted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("producer side effect: %v", err)
	}
}
