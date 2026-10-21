package workspace

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func renderProjectFileMarkdown(source []byte) (string, error) {
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	var unavailable []*ast.Link
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if link, ok := node.(*ast.Link); ok && entering {
			destination := string(util.URLEscape(link.Destination, true))
			u, err := url.Parse(destination)
			if !strings.HasPrefix(destination, "#") && (err != nil || u.Scheme == "" && u.Host == "") {
				unavailable = append(unavailable, link)
			}
		}
		return ast.WalkContinue, nil
	})
	// Uploads keep only a basename and a document ID, not the source folder.
	// A relative path cannot identify a sibling upload, even by basename.
	// Keep its label readable without emitting a workspace-relative href.
	for _, link := range unavailable {
		parent := link.Parent()
		for child := link.FirstChild(); child != nil; child = link.FirstChild() {
			parent.InsertBefore(parent, link, child)
		}
		parent.InsertBefore(parent, link, ast.NewString([]byte(" (relative link unavailable: uploaded files do not retain source folders)")))
		parent.RemoveChild(parent, link)
	}
	var html bytes.Buffer
	if err := goldmark.DefaultRenderer().Render(&html, source, document); err != nil {
		return "", err
	}
	return html.String(), nil
}
