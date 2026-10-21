package workspace

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

func TestProjectFileRelativeLinksRemainReadableWithoutNavigation(t *testing.T) {
	for _, destination := range []string{
		"skills/gimbal-release/references/distribution.md", "./sibling.md", "../secret.md",
		"/etc/passwd", "?download=1", "", "./sibling.md#section", "skills&#47;release.md",
	} {
		t.Run(destination, func(t *testing.T) {
			source := []byte("# README\n\nBefore [**distribution** instructions](" + destination + ") and [another](missing.md). After.\n")
			html, err := renderProjectFileMarkdown(source)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(html, "<a ") || strings.Count(html, "relative link unavailable") != 2 {
				t.Fatalf("relative links must explain unavailability without navigation: %s", html)
			}
			if !strings.Contains(html, "Before <strong>distribution</strong> instructions") || !strings.Contains(html, "After.") {
				t.Fatalf("document content/formatting lost: %s", html)
			}
		})
	}
}

func TestProjectFileOtherMarkdownKeepsExistingRenderingPolicy(t *testing.T) {
	for _, source := range []string{
		"[external](https://example.com/docs?q=1&x=2#part \"Docs\")",
		"[external](//example.com/docs) [email](mailto:reader@example.com)",
		"[section](#distribution) [escaped](https\\://example.com)",
		"[entity](https&#58;//example.com)",
		"<https://example.com>\n\n![image](https://example.com/image.png)",
		"[unsafe](javascript:alert%281%29)\n\n<script>alert(1)</script>",
		"[reference][doc]\n\n[doc]: https://example.com/docs",
	} {
		t.Run(source, func(t *testing.T) {
			var original bytes.Buffer
			if err := goldmark.Convert([]byte(source), &original); err != nil {
				t.Fatal(err)
			}
			got, err := renderProjectFileMarkdown([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if got != original.String() {
				t.Fatalf("existing rendering policy changed:\ngot: %s\nwant: %s", got, original.String())
			}
		})
	}
}
