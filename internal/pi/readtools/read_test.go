package readtools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestReadContentsWithinLimits(t *testing.T) {
	dir := t.TempDir()
	content := "Hello, world!\nLine 2\nLine 3"
	writeFile(t, filepath.Join(dir, "test.txt"), []byte(content))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if out != content {
		t.Fatalf("read = %q, want %q", out, content)
	}
	if strings.Contains(out, "Use offset=") {
		t.Fatalf("unexpected truncation notice: %q", out)
	}
	if res.Details != nil {
		t.Fatalf("details = %#v, want nil", res.Details)
	}
}

func TestReadNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	def := ReadTool(dir, nil)
	_, err := runTool(t, def, map[string]any{"path": "nonexistent.txt"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "no such file") {
		t.Fatalf("expected ENOENT-style error, got %v", err)
	}
}

func TestReadTruncatesByLineLimit(t *testing.T) {
	dir := t.TempDir()
	lines := make([]string, 2500)
	for i := range lines {
		lines[i] = "Line " + itoaTest(i+1)
	}
	writeFile(t, filepath.Join(dir, "large.txt"), []byte(strings.Join(lines, "\n")))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "large.txt"})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if !strings.Contains(out, "Line 1") || !strings.Contains(out, "Line 2000") {
		t.Fatalf("expected first 2000 lines, got: %q", out[:min(120, len(out))])
	}
	if strings.Contains(out, "Line 2001") {
		t.Fatal("line 2001 should have been truncated")
	}
	if !strings.Contains(out, "[Showing lines 1-2000 of 2500. Use offset=2001 to continue.]") {
		t.Fatalf("missing line-limit notice: %q", out)
	}
	details, ok := res.Details.(*ReadToolDetails)
	if !ok || details.Truncation == nil {
		t.Fatalf("details = %#v, want truncation", res.Details)
	}
	if details.Truncation.TruncatedBy == nil || *details.Truncation.TruncatedBy != "lines" {
		t.Fatalf("truncatedBy = %v, want lines", details.Truncation.TruncatedBy)
	}
	if details.Truncation.TotalLines != 2500 || details.Truncation.OutputLines != 2000 {
		t.Fatalf("truncation counts = %+v", details.Truncation)
	}
}

func TestReadTruncatesByByteLimit(t *testing.T) {
	dir := t.TempDir()
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = "Line " + itoaTest(i+1) + ": " + strings.Repeat("x", 200)
	}
	writeFile(t, filepath.Join(dir, "large-bytes.txt"), []byte(strings.Join(lines, "\n")))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "large-bytes.txt"})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if !strings.Contains(out, "Line 1:") {
		t.Fatalf("expected first line, got %q", out[:min(80, len(out))])
	}
	if !strings.Contains(out, "(50.0KB limit). Use offset=") {
		t.Fatalf("missing byte-limit notice: %q", tail(out, 200))
	}
}

func TestReadOffset(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "offset.txt"), []byte(numberedLines(100)))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "offset.txt", "offset": float64(51)})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if strings.Contains(out, "Line 50") {
		t.Fatal("line 50 should have been skipped")
	}
	if !strings.Contains(out, "Line 51") || !strings.Contains(out, "Line 100") {
		t.Fatalf("offset read wrong: %q", tail(out, 120))
	}
	if strings.Contains(out, "Use offset=") {
		t.Fatalf("unexpected truncation notice: %q", tail(out, 120))
	}
}

func TestReadLimit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "limit.txt"), []byte(numberedLines(100)))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "limit.txt", "limit": float64(10)})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if !strings.Contains(out, "Line 1") || !strings.Contains(out, "Line 10") {
		t.Fatalf("limit read wrong: %q", tail(out, 160))
	}
	if strings.Contains(out, "Line 11") {
		t.Fatal("line 11 should not appear")
	}
	if !strings.Contains(out, "[90 more lines in file. Use offset=11 to continue.]") {
		t.Fatalf("missing limit notice: %q", tail(out, 160))
	}
}

func TestReadOffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "offset-limit.txt"), []byte(numberedLines(100)))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "offset-limit.txt", "offset": float64(41), "limit": float64(20)})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if strings.Contains(out, "Line 40") || strings.Contains(out, "Line 61") {
		t.Fatalf("offset+limit window wrong: %q", tail(out, 200))
	}
	if !strings.Contains(out, "Line 41") || !strings.Contains(out, "Line 60") {
		t.Fatalf("offset+limit window wrong: %q", tail(out, 200))
	}
	if !strings.Contains(out, "[40 more lines in file. Use offset=61 to continue.]") {
		t.Fatalf("missing continuation notice: %q", tail(out, 200))
	}
}

func TestReadOffsetBeyondEOF(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "short.txt"), []byte("Line 1\nLine 2\nLine 3"))

	def := ReadTool(dir, nil)
	_, err := runTool(t, def, map[string]any{"path": "short.txt", "offset": float64(100)})
	if _, ok := errors.AsType[*OffsetBeyondEOFError](err); !ok {
		t.Fatalf("expected OffsetBeyondEOFError, got %v", err)
	}
	if err.Error() != "Offset 100 is beyond end of file (3 lines total)" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestReadDetectsImageMimeFromMagic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "image.txt"), decodeBase64(t, tinyPNG1x1))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "image.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content[0].ContentType() != "text" {
		t.Fatalf("first block type = %q, want text", res.Content[0].ContentType())
	}
	if !strings.Contains(resultText(t, res), "Read image file [image/png]") {
		t.Fatalf("image note wrong: %q", resultText(t, res))
	}
	img, ok := res.Content[1].(model.ImageContent)
	if !ok {
		t.Fatalf("second block = %#v, want image", res.Content[1])
	}
	if img.MimeType != "image/png" || img.Data == "" {
		t.Fatalf("image block = %+v", img)
	}
}

func TestReadConvertsBMPToPNG(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "image.bmp"), tinyBMP1x1Red24bpp())

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "image.bmp"})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if !strings.Contains(out, "Read image file [image/png]") {
		t.Fatalf("image note wrong: %q", out)
	}
	if !strings.Contains(out, "[Image converted from image/bmp to image/png.]") {
		t.Fatalf("missing conversion hint: %q", out)
	}
	img, ok := res.Content[1].(model.ImageContent)
	if !ok || img.MimeType != "image/png" {
		t.Fatalf("image block = %#v", res.Content[1])
	}
	raw := decodeBase64(t, img.Data)
	if len(raw) == 0 || raw[0] != 0x89 {
		t.Fatalf("converted image is not PNG: %x", raw[:min(4, len(raw))])
	}
}

func TestReadTreatsNonImageContentAsText(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "not-an-image.png"), []byte("definitely not a png"))

	def := ReadTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": "not-an-image.png"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(t, res), "definitely not a png") {
		t.Fatalf("text content wrong: %q", resultText(t, res))
	}
	for _, c := range res.Content {
		if c.ContentType() == "image" {
			t.Fatal("non-image was attached as an image")
		}
	}
}

func TestReadOperationsAreConsulted(t *testing.T) {
	var readPath, accessPath, mimePath string
	ops := &ReadOperations{
		ReadFile: func(_ context.Context, p string) ([]byte, error) {
			readPath = p
			return []byte("from custom ops"), nil
		},
		Access: func(_ context.Context, p string) error {
			accessPath = p
			return nil
		},
		DetectImageMimeType: func(_ context.Context, p string) string {
			mimePath = p
			return ""
		},
	}
	def := ReadTool("/base", &ReadToolOptions{Operations: ops})
	res, err := runTool(t, def, map[string]any{"path": "f.txt"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/base", "f.txt")
	if readPath != want || accessPath != want || mimePath != want {
		t.Fatalf("ops paths = %q/%q/%q, want %q", readPath, accessPath, mimePath, want)
	}
	if resultText(t, res) != "from custom ops" {
		t.Fatalf("read = %q", resultText(t, res))
	}
}

func TestReadAccessFailureStopsTheRead(t *testing.T) {
	read := false
	ops := &ReadOperations{
		ReadFile: func(_ context.Context, _ string) ([]byte, error) {
			read = true
			return nil, nil
		},
		Access: func(_ context.Context, _ string) error {
			return errors.New("denied")
		},
	}
	def := ReadTool("/base", &ReadToolOptions{Operations: ops})
	if _, err := runTool(t, def, map[string]any{"path": "f.txt"}); err == nil || err.Error() != "denied" {
		t.Fatalf("expected access error, got %v", err)
	}
	if read {
		t.Fatal("ReadFile must not run after a failed access")
	}
}

func TestReadDirectorySurfacesError(t *testing.T) {
	dir := t.TempDir()
	def := ReadTool(dir, nil)
	_, err := runTool(t, def, map[string]any{"path": "."})
	if err == nil {
		t.Fatal("reading a directory should fail")
	}
}

func numberedLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "Line " + itoaTest(i+1)
	}
	return strings.Join(lines, "\n")
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
