// Package web embeds the SvelteKit build the skgo adapter writes.
package web

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"io/fs"
)

//go:embed build.zip
var buildArchive []byte

// Build holds the adapter output rooted at build/. The archive is included
// in the published Go module, so a versioned install serves the same app as
// a checkout build.
var Build fs.FS = openBuild()

func openBuild() fs.FS {
	reader, err := zip.NewReader(bytes.NewReader(buildArchive), int64(len(buildArchive)))
	if err != nil {
		panic(err)
	}
	return reader
}
