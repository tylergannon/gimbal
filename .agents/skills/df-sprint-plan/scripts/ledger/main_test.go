package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLLedgerFixture(t *testing.T) {
	root := t.TempDir()
	invoke := func(args ...string) (string, error) {
		var out bytes.Buffer
		err := run(append([]string{"--project", root}, args...), &out)
		return out.String(), err
	}
	if _, err := invoke("add", "A: chapter sprint", "--chapter", "CHAPTER-0002"); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke("start", "1"); err != nil {
		t.Fatal(err)
	}
	out, err := invoke("current")
	if err != nil || !strings.Contains(out, "docs/sprints/SPRINT-0001.md") || !strings.Contains(out, "CHAPTER-0002") {
		t.Fatal(out, err)
	}
	if _, err := invoke("set-chapter", "1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke("complete", "1"); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(root, "docs/sprints/SPRINT-0003.md")
	if err := os.WriteFile(doc, []byte("# Sprint 3: Third title\nChapter: [CHAPTER-0004](../chapters/CHAPTER-0004.md)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke("sync"); err != nil {
		t.Fatal(err)
	}
	l, err := open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	one, _ := l.find("1")
	three, _ := l.find("3")
	if one.Status != "done" || one.Chapter != "" || three.Title != "Third title" || three.Chapter != "CHAPTER-0004" {
		t.Fatalf("%#v", l.entries)
	}
	if _, err := invoke("sync"); err != nil {
		t.Fatal(err)
	}
	l, err = open(root, "")
	if err != nil || len(l.entries) != 2 {
		t.Fatal(l, err)
	}
}
func TestTSVFixturesAndAliases(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docs/sprints/ledger.tsv")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(tsvHeader+"\n007\tExisting\tplanned\tx\ty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	l, err := open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	row, err := l.add("", "Next", "in-progress", "CHAPTER-0001")
	if err != nil || row.ID != "008" || row.Status != "in_progress" || row.Chapter != "" {
		t.Fatal(row, err)
	}
	if err := l.save(); err != nil {
		t.Fatal(err)
	}
	l, err = open(root, "")
	if err != nil || l.selectEntry("in-progress").ID != "008" {
		t.Fatal(l, err)
	}
	if _, err := l.canonicalStatus("nonsense"); err == nil {
		t.Fatal("invalid status accepted")
	}
}
