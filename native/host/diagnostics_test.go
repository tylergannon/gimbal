package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDiagnosticsRotationAndExport(t *testing.T) {
	d := &diagnosticLog{dir: filepath.Join(t.TempDir(), "logs"), maxBytes: 300, backups: 3}
	for i := range 20 {
		if err := d.record("recorder", fmt.Sprintf("failure-%02d microphone unavailable", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d.dir, "private-transcript.json"), []byte("DO NOT EXPORT"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "desktop.log") {
			continue
		}
		count++
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > d.maxBytes || info.Mode().Perm() != 0o600 {
			t.Fatalf("unbounded or non-private log: %s %v", entry.Name(), info)
		}
	}
	if count != 4 {
		t.Fatalf("retained %d files; want current + 3 backups", count)
	}
	destination := filepath.Join(t.TempDir(), "diagnostics.log")
	if err := d.export(destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "failure-19") || strings.Contains(text, "failure-00") || strings.Contains(text, "DO NOT EXPORT") {
		t.Fatalf("incorrect retained/exported content: %s", text)
	}
	if strings.Index(text, "failure-18") > strings.Index(text, "failure-19") {
		t.Fatal("export is not oldest-to-newest")
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("export permissions: %v %v", info, err)
	}
	if err := d.export(filepath.Join(d.dir, "desktop.log")); err == nil {
		t.Fatal("export may overwrite active diagnostics")
	}
}

func TestDiagnosticsConcurrentInstances(t *testing.T) {
	directory := t.TempDir()
	var wg sync.WaitGroup
	for instance := range 2 {
		d := &diagnosticLog{dir: directory, maxBytes: 2 << 20, backups: 3}
		wg.Go(func() {
			for i := range 100 {
				if err := d.record("server", fmt.Sprintf("instance=%d entry=%d", instance, i)); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	d := &diagnosticLog{dir: directory, maxBytes: 2 << 20, backups: 3}
	data, err := d.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "[server]") != 200 {
		t.Fatal("concurrent instances lost log entries")
	}
}

func TestDiagnosticsWriteFailureIsReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	d := &diagnosticLog{dir: path, maxBytes: 1024, backups: 3}
	if err := d.record("recorder", "failure"); err == nil {
		t.Fatal("log write failure was silently discarded")
	}
	if _, err := d.snapshot(); err == nil {
		t.Fatal("export failure was silently discarded")
	}
}

func TestDiagnosticsOversizedMultilineEntryIsBounded(t *testing.T) {
	d := &diagnosticLog{dir: t.TempDir(), maxBytes: 1024, backups: 3}
	if err := d.record("server", "start"+strings.Repeat("\n", 32000)+"end"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(d.dir, "desktop.log"))
	if err != nil || info.Size() > d.maxBytes {
		t.Fatalf("oversized log entry: %v %v", info, err)
	}
}
