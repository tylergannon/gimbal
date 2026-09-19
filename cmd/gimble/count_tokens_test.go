package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiktoken-go/tokenizer"
)

func TestCountTokens(t *testing.T) {
	file := filepath.Join(t.TempDir(), "document.md")
	text := "Research-backed documents compress the important knowledge."
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		t.Fatal(err)
	}
	want, err := codec.Count(text)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := run([]string{"count-tokens", file}, &stdout, &stderr, os.Getenv); err != nil {
		t.Fatalf("count-tokens: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != fmt.Sprint(want) {
		t.Fatalf("count-tokens output = %q, want %d", got, want)
	}
}
