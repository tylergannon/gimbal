package main

import (
	"embed"
	"os"
	"path/filepath"
)

//go:embed testdata/*
var fixture embed.FS

// prepareFixture copies a fixed consumer project into the fresh run workspace.
// Exclusive creation prevents accidentally overwriting prior work.
func prepareFixture(dir string) error {
	entries, err := fixture.ReadDir("testdata")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		body, err := fixture.ReadFile("testdata/" + entry.Name())
		if err != nil {
			return err
		}
		name := entry.Name()
		if name == "go.mod.txt" {
			name = "go.mod"
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		_, err = f.Write(body)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
