package gimbal

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/tiktoken-go/tokenizer"
	"github.com/tylergannon/polytype"
)

const (
	contextTokenLimit      = 15_000
	contextEntryTokenLimit = 4_000
	artifactPreviewBytes   = 32 << 10
)

type artifactDescriptor struct {
	file    string
	size    int64
	format  string
	preview string
}

var contextTokenizer = sync.OnceValues(func() (tokenizer.Codec, error) {
	return tokenizer.Get(tokenizer.O200kBase)
})

func tokenCount(text string) int {
	codec, err := contextTokenizer()
	if err != nil {
		panic(fmt.Sprintf("gimbal: initialize context tokenizer: %v", err))
	}
	count, err := codec.Count(text)
	if err != nil {
		panic(fmt.Sprintf("gimbal: count context tokens: %v", err))
	}
	return count
}

// encodedComponent is collision-free and cannot be interpreted as a path
// component by the host filesystem, regardless of what a workflow named.
func encodedComponent(name string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(name))
	if len(encoded) <= 118 {
		return "x-" + encoded
	}
	parts := []string{"long-" + strconv.Itoa(len(encoded))}
	for len(encoded) > 0 {
		n := min(118, len(encoded))
		parts = append(parts, "x-"+encoded[:n])
		encoded = encoded[n:]
	}
	return filepath.Join(parts...)
}

func encodedPath(name string) string {
	if name == "" {
		return "root"
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		parts[i] = encodedComponent(parts[i])
	}
	return filepath.Join(parts...)
}

func (r *run) artifactName(relative string) (string, error) {
	root := filepath.Join(r.dir, "artifacts")
	name := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, name)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("gimbal: artifact path escapes run: %q", relative)
	}
	return name, nil
}

func (r *run) writeArtifact(relative string, data []byte) (artifactDescriptor, error) {
	name, err := r.artifactName(relative)
	if err != nil {
		return artifactDescriptor{}, err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return artifactDescriptor{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(name), ".gimbal-artifact-*")
	if err != nil {
		return artifactDescriptor{}, err
	}
	tempName := temp.Name()
	ok := false
	defer func() {
		_ = temp.Close()
		if !ok {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		return artifactDescriptor{}, err
	}
	if err := temp.Chmod(0o644); err != nil {
		return artifactDescriptor{}, err
	}
	if err := temp.Sync(); err != nil {
		return artifactDescriptor{}, err
	}
	if err := temp.Close(); err != nil {
		return artifactDescriptor{}, err
	}
	// Linking the complete temp file into place is atomic and refuses to
	// overwrite an existing immutable artifact.
	if err := os.Link(tempName, name); err != nil {
		return artifactDescriptor{}, err
	}
	if err := os.Remove(tempName); err != nil {
		return artifactDescriptor{}, err
	}
	ok = true
	return artifactDescriptor{file: filepath.ToSlash(filepath.Join("artifacts", relative)), size: int64(len(data))}, nil
}

func (r *run) writeContentArtifact(kind, text string) (artifactDescriptor, error) {
	digest := sha256.Sum256([]byte(text))
	relative := filepath.ToSlash(filepath.Join("context", encodedComponent(kind), fmt.Sprintf("%x.txt", digest[:12])))
	name, err := r.artifactName(relative)
	if err != nil {
		return artifactDescriptor{}, err
	}
	if info, err := os.Stat(name); err == nil {
		return artifactDescriptor{file: filepath.ToSlash(filepath.Join("artifacts", relative)), size: info.Size(), format: "text", preview: preview(text)}, nil
	}
	desc, err := r.writeArtifact(relative, []byte(text))
	if err != nil {
		// A concurrent identical render may have won the immutable name.
		if info, statErr := os.Stat(name); statErr == nil {
			return artifactDescriptor{file: filepath.ToSlash(filepath.Join("artifacts", relative)), size: info.Size(), format: "text", preview: preview(text)}, nil
		}
		return artifactDescriptor{}, err
	}
	desc.format, desc.preview = "text", preview(text)
	return desc, nil
}

func (r *run) openArtifact(relative string) (*os.File, string, error) {
	name, err := r.artifactName(relative)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return nil, "", err
	}
	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, "", err
	}
	return file, filepath.ToSlash(filepath.Join("artifacts", relative)), nil
}

func (r *run) readArtifact(desc artifactDescriptor) ([]byte, error) {
	name := filepath.Join(r.dir, filepath.FromSlash(desc.file))
	return os.ReadFile(name)
}

func preview(text string) string {
	return excerptBytes(text, artifactPreviewBytes, "")
}

func excerptBytes(text string, kept int, absolutePath string) string {
	text = strings.ToValidUTF8(text, "\uFFFD")
	if kept >= len(text) {
		return text
	}
	if kept < 0 {
		kept = 0
	}
	head := kept / 2
	tail := kept - head
	for head > 0 && head < len(text) && !utf8.RuneStart(text[head]) {
		head--
	}
	startTail := len(text) - tail
	for startTail < len(text) && startTail > 0 && !utf8.RuneStart(text[startTail]) {
		startTail++
	}
	omitted := len(text) - head - (len(text) - startTail)
	marker := fmt.Sprintf("\n\n[... %d bytes omitted ...]", omitted)
	if absolutePath != "" {
		marker += "\n\nComplete value: " + absolutePath
	}
	return text[:head] + marker + "\n\n" + text[startTail:]
}

func fitExcerpt(text, absolutePath string, maxTokens int) string {
	if tokenCount(text) <= maxTokens {
		return text
	}
	return fitExcerptBytes(text, absolutePath, maxTokens, artifactPreviewBytes)
}

func fitExcerptBytes(text, absolutePath string, maxTokens, maxBytes int) string {
	low, high := 0, min(len(text), maxBytes)
	best := excerptBytes(text, 0, absolutePath)
	for low <= high {
		middle := low + (high-low)/2
		candidate := excerptBytes(text, middle, absolutePath)
		if tokenCount(candidate) <= maxTokens {
			best = candidate
			low = middle + 1
		} else {
			high = middle - 1
		}
	}
	return best
}

func artifactAbsolute(r *run, desc artifactDescriptor) string {
	return filepath.Join(r.dir, filepath.FromSlash(desc.file))
}

func budgetRenderedText(ctx context.Context, kind, text string, limit int) (string, error) {
	if tokenCount(text) <= limit {
		return text, nil
	}
	s, err := current(ctx)
	if err != nil {
		return "", err
	}
	desc, err := s.run.writeContentArtifact(kind, text)
	if err != nil {
		return "", fmt.Errorf("gimbal: write %s artifact: %w", kind, err)
	}
	return fitExcerpt(text, artifactAbsolute(s.run, desc), limit), nil
}

func valueArtifact(desc artifactDescriptor) *ValueArtifact {
	return &ValueArtifact{File: desc.file, Size: desc.size, Format: desc.format, Preview: desc.preview}
}

func valueEvent(key string, value *scopeValue) ValueSet {
	if value.artifact != nil {
		return ValueSet{Key: key, Artifact: polytype.Optional[ValueArtifact]{Present: true, Value: *valueArtifact(*value.artifact)}}
	}
	return ValueSet{Key: key, Value: polytype.Optional[JSONText]{Present: true, Value: JSONText(value.raw)}}
}

func valueBytes(value *scopeValue) ([]byte, error) {
	value.owner.mu.Lock()
	raw := append([]byte(nil), value.raw...)
	var artifact *artifactDescriptor
	if value.artifact != nil {
		copy := *value.artifact
		artifact = &copy
	}
	value.owner.mu.Unlock()
	if artifact == nil {
		return raw, nil
	}
	raw, err := value.owner.run.readArtifact(*artifact)
	if err != nil {
		return nil, err
	}
	if artifact.format == "text" {
		return json.Marshal(string(raw))
	}
	return raw, nil
}
