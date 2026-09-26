package files

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestWithFileMutationQueueSerializesSameFile(t *testing.T) {
	const path = "/tmp/file-mutation-queue-same"
	var mu sync.Mutex
	var order []string
	record := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}

	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		_, _ = WithFileMutationQueue(path, func() (int, error) {
			record("first:start")
			close(started)
			<-release
			record("first:end")
			return 0, nil
		})
		close(firstDone)
	}()
	<-started

	secondDone := make(chan struct{})
	go func() {
		_, _ = WithFileMutationQueue(path, func() (int, error) {
			record("second:start")
			record("second:end")
			return 0, nil
		})
		close(secondDone)
	}()

	select {
	case <-secondDone:
		t.Fatal("second operation ran before the first released the queue")
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	<-firstDone
	<-secondDone

	want := []string{"first:start", "first:end", "second:start", "second:end"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestWithFileMutationQueueDifferentFilesRunInParallel(t *testing.T) {
	var mu sync.Mutex
	var order []string
	record := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}

	startA := make(chan struct{})
	startB := make(chan struct{})
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})

	go func() {
		_, _ = WithFileMutationQueue("/tmp/file-mutation-queue-a", func() (int, error) {
			record("a:start")
			close(startA)
			<-releaseA
			record("a:end")
			return 0, nil
		})
	}()
	<-startA
	go func() {
		_, _ = WithFileMutationQueue("/tmp/file-mutation-queue-b", func() (int, error) {
			record("b:start")
			close(startB)
			<-releaseB
			record("b:end")
			return 0, nil
		})
	}()
	select {
	case <-startB:
	case <-time.After(time.Second):
		t.Fatal("different files did not proceed in parallel")
	}
	close(releaseA)
	close(releaseB)

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(order) == 4
	})

	index := func(s string) int {
		for i, v := range order {
			if v == s {
				return i
			}
		}
		return -1
	}
	if index("a:start") > index("a:end") {
		t.Errorf("a ordering wrong: %v", order)
	}
	if index("b:start") > index("b:end") {
		t.Errorf("b ordering wrong: %v", order)
	}
	if index("b:start") > index("a:end") {
		t.Errorf("different files should interleave, got %v", order)
	}
}

func TestWithFileMutationQueueUsesSameQueueForSymlinkAliases(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.txt")
	symlinkPath := filepath.Join(dir, "alias.txt")
	if err := os.WriteFile(targetPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	var mu sync.Mutex
	var order []string
	record := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}

	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		_, _ = WithFileMutationQueue(targetPath, func() (int, error) {
			record("target:start")
			close(started)
			<-release
			record("target:end")
			return 0, nil
		})
		close(firstDone)
	}()
	<-started

	secondDone := make(chan struct{})
	go func() {
		_, _ = WithFileMutationQueue(symlinkPath, func() (int, error) {
			record("alias:start")
			record("alias:end")
			return 0, nil
		})
		close(secondDone)
	}()

	select {
	case <-secondDone:
		t.Fatal("symlink alias did not share the target's queue")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	<-firstDone
	<-secondDone

	want := []string{"target:start", "target:end", "alias:start", "alias:end"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met in time")
}
