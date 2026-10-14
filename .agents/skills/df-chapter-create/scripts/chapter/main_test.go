package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChapterFixture(t *testing.T) {
	root := t.TempDir()
	invoke := func(args ...string) (string, error) {
		var out bytes.Buffer
		err := run(append([]string{"--project", root}, args...), &out)
		return out.String(), err
	}
	out, err := invoke("add", "Runtime: durability", "--doc", "docs/chapters/CHAPTER-0001.md", "--summary", "One: two\nthree", "--sprint-id", "SPRINT-0003", "--sprint-id", "SPRINT-0004")
	if err != nil || out != "CHAPTER-0001\n" {
		t.Fatalf("%q %v", out, err)
	}
	data, err := load(filepath.Join(root, "docs/chapters/ledger.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	entry := data.Chapters[0]
	if entry.Title != "Runtime: durability" || entry.Summary != "One: two\nthree" || len(entry.Sprints) != 2 || entry.Created != entry.Updated {
		t.Fatalf("%#v", entry)
	}
	out, err = invoke("get", "CHAPTER-0001")
	if err != nil || !strings.Contains(out, "sprint_ids: SPRINT-0003, SPRINT-0004") {
		t.Fatal(out, err)
	}
	out, err = invoke("next-id")
	if err != nil || out != "CHAPTER-0002\n" {
		t.Fatal(out, err)
	}
	if _, err = invoke("add", "Duplicate", "--id", "CHAPTER-0001", "--doc", "x", "--summary", "x"); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err = os.Stat(filepath.Join(root, "docs/chapters/ledger.yaml")); err != nil {
		t.Fatal(err)
	}
}
func TestFoldedExistingYAML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ledger.yaml")
	if err := os.WriteFile(path, []byte("chapters:\n  - id: CHAPTER-0009\n    summary: >-\n      One sentence\n      continued.\n    sprint_ids:\n      - SPRINT-0001\n"), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if data.Chapters[0].Summary != "One sentence continued." || nextID(data) != "CHAPTER-0010" {
		t.Fatalf("%#v", data)
	}
}
