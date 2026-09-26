package readtools

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func runTool(t *testing.T, def model.ToolDefinition, params map[string]any) (model.AgentToolResult, error) {
	t.Helper()
	return def.Execute(context.Background(), "test-call", params, nil)
}

func resultText(t *testing.T, r model.AgentToolResult) string {
	t.Helper()
	text, ok := r.Content.TextContentText()
	if !ok {
		t.Fatalf("result has no text content: %#v", r.Content)
	}
	return text
}

// tinyPNG1x1 is a 1x1 PNG, the same fixture the upstream read tool test uses.
const tinyPNG1x1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNgYGD4DwABBAEAX+XDSwAAAABJRU5ErkJggg=="

func decodeBase64(t *testing.T, data string) []byte {
	t.Helper()
	out, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	return out
}

// tinyBMP1x1Red24bpp builds the 1x1 24-bit BMP fixture from the upstream test.
func tinyBMP1x1Red24bpp() []byte {
	buffer := make([]byte, 58)
	copy(buffer, "BM")
	putUint32LE(buffer, 2, uint32(len(buffer)))
	putUint32LE(buffer, 10, 54)
	putUint32LE(buffer, 14, 40)
	putUint32LE(buffer, 18, 1)
	putUint32LE(buffer, 22, 1)
	putUint16LE(buffer, 26, 1)
	putUint16LE(buffer, 28, 24)
	putUint32LE(buffer, 30, 0)
	putUint32LE(buffer, 34, 4)
	buffer[56] = 0xff
	return buffer
}

func putUint16LE(b []byte, off int, v uint16) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
}

func putUint32LE(b []byte, off int, v uint32) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}
