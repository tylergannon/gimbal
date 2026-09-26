package files

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/png"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Small 2x2 red PNG image (base64) - generated with ImageMagick.
const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACAQMAAABIeJ9nAAAAIGNIUk0AAHomAACAhAAA+gAAAIDoAAB1MAAA6mAAADqYAAAXcJy6UTwAAAAGUExURf8AAP///0EdNBEAAAABYktHRAH/Ai3eAAAAB3RJTUUH6gEOADM5Ddoh/wAAAAxJREFUCNdjYGBgAAAABAABJzQnCgAAACV0RVh0ZGF0ZTpjcmVhdGUAMjAyNi0wMS0xNFQwMDo1MTo1NyswMDowMOnKzHgAAAAldEVYdGRhdGU6bW9kaWZ5ADIwMjYtMDEtMTRUMDA6NTE6NTcrMDA6MDCYl3TEAAAAKHRFWHRkYXRlOnRpbWVzdGFtcAAyMDI2LTAxLTE0VDAwOjUxOjU3KzAwOjAwz4JVGwAAAABJRU5ErkJggg=="

// Small 2x2 blue JPEG image (base64) - generated with ImageMagick.
const tinyJPEG = "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMUFRUVDA8XGBYUGBIUFRT/2wBDAQMEBAUEBQkFBQkUDQsNFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBT/wAARCAACAAIDAREAAhEBAxEB/8QAFAABAAAAAAAAAAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/xAAVAQEBAAAAAAAAAAAAAAAAAAAGCf/EABQRAQAAAAAAAAAAAAAAAAAAAAD/2gAMAwEAAhEDEQA/AD3VTB3/2Q=="

// Small 1x2 blue JPEG image (base64) - generated with ImageMagick.
const tinyJPEG2x1 = "/9j/4AAQSkZJRgABAgAAAQABAAD/wAARCAABAAIDAREAAhEBAxEB/9sAQwADAgIDAgIDAwMDBAMDBAUIBQUEBAUKBwcGCAwKDAwLCgsLDQ4SEA0OEQ4LCxAWEBETFBUVFQwPFxgWFBgSFBUU/9sAQwEDBAQFBAUJBQUJFA0LDRQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQU/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwD4H8Q/8h/Uv+vmX/0M1/o1wJ/ySWU/9g1D/wBNRMOM/wDkp8z/AOv9b/05I//Z"

// 100x100 gray PNG.
const mediumPNG100x100 = "iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAAAAABVicqIAAAAAmJLR0QA/4ePzL8AAAAHdElNRQfqAQ4AMzkN2iH/AAAAP0lEQVRo3u3NQQEAAAQEMASXXYrz2gqst/Lm4ZBIJBKJRCKRSCQSiUQikUgkEolEIpFIJBKJRCKRSCQSiSTsAP1cAUZeKtreAAAAJXRFWHRkYXRlOmNyZWF0ZQAyMDI2LTAxLTE0VDAwOjUxOjU3KzAwOjAw6crMeAAAACV0RVh0ZGF0ZTptb2RpZnkAMjAyNi0wMS0xNFQwMDo1MTo1NyswMDowMJiXdMQAAAAodEVYdGRhdGU6dGltZXN0YW1wADIwMjYtMDEtMTRUMDA6NTE6NTcrMDA6MDDPglUbAAAAAElFTkSuQmCC"

// 200x200 colored PNG.
const largePNG200x200 = "iVBORw0KGgoAAAANSUhEUgAAAMgAAADIEAIAAAByquWKAAAAIGNIUk0AAHomAACAhAAA+gAAAIDoAAB1MAAA6mAAADqYAAAXcJy6UTwAAAAGYktHRP///////wlY99wAAAAHdElNRQfqAQ4AMzqU03BFAAAEEElEQVR42u3dvWqUURSF4XXMRAyIhSAWQvwnWtjaWusFx9bW1kKD/wELESxEiBjDjJdwysOC57mCXW32+zGcGbtdACpsdi+T8Xz1GABzm91xMl6sHgNgblzcT/Y+rh4DYG6cv0/2H64eA2BOEgI1fHQHaoyzbXIwVo8BMCcJgRqSEKgxft9Lrn5aPQbA3Ph1klw7Wj0GwJwkBGpYWECN8fMiub63egyAORcWUMPCAmqMH3eTG59XjwEwN76/S24+Wj0GwJwkBGpYWECN8e1fcmuzegyAORcWUMPCAmqM09vJ4dfVYwDMjS9vkzuPV48BMCcJgRqeSAZqjA/nyYP91WMAzElCoIYkBGqMk8Pk6HT1GABzm+3L5JIkBApIQqCGj+5AjfHmb/Lk8uoxAOYkIVBDEgI1LCygxmZ7nFyShEABFxZQw8ICaozXf5KnV1aPATDnwgJqWFhADQsLqOFnDUANFxZQw8ICaoxXZ8mzg9VjAMy5sIAaFhZQw3tYQA1/QgHUkIRADUkI1JCEQA1JCNSQhEANFxZQwzcsoIYkBGpIQqCGJARqSEKghiQEalhYQA1/QgHUcGEBNSwsoIYkBGq4sIAaFhZQw8ICaviGBdRwYQE1LCyghiQEariwgBoWFlDDe1hADS+OAjUkIVBDEgI1JCFQQxICNSQhUMOFBdTwDQuoIQmBGpIQqCEJgRqSEKghCYEaFhZQw4ujQA0XFlDDwgJqSEKghgsLqGFhATUsLKCGb1hADRcWUMPCAmpIQqCGCwuoYWEBNbyHBdTw4ihQQxICNSQhUEMSAjUkIVBDEgI1XFhADd+wgBqSEKghCYEakhCoIQmBGpIQqGFhATW8OArUcGEBNSwsoIYkBGq4sIAaFhZQw8ICaviGBdRwYQE1LCyghiQEariwgBoWFlDDe1hADS+OAjUkIVBDEgI1JCFQQxICNSQhUMOFBdTwDQuoIQmBGpIQqCEJgRqSEKghCYEa49lB8ups9RgAc14cBWpIQqCGhQXUkIRADRcWUMPCAmqMp1eS139WjwEw5xsWUEMSAjUsLKCGJARquLCAGhYWUGM8uZy8+bt6DIA5L44CNSQhUMMTyUCNcXSanByuHgNgThICNSQhUGM82E8+nK8eA2DOzxqAGpIQqDHuPE6+vF09BsDcOPyanN5ePQbAnCQEavgdFlBj3Nok3/6tHgNgzoujQA1JCNQYNx8l39+tHgNgbtz4nPy4u3oMgDlJCNSwsIAa4/pe8vNi9RgAc37WANSQhECNce0o+XWyegyAuXH1U/L73uoxAOYkIVDDwgJqjIORnG1XjwEw508ogBqSEKgx9h8m5+9XjwEwN/Y+Jhf3V48BMCcJgRpjPE+2x6vHAJgbSbLbrR4DYO4/GqiSgXN+ksgAAAAldEVYdGRhdGU6Y3JlYXRlADIwMjYtMDEtMTRUMDA6NTE6NTcrMDA6MDDpysx4AAAAJXRFWHRkYXRlOm1vZGlmeQAyMDI2LTAxLTE0VDAwOjUxOjU3KzAwOjAwmJd0xAAAACh0RVh0ZGF0ZTp0aW1lc3RhbXAAMjAyNi0wMS0xNFQwMDo1MTo1NyswMDowMM+CVRsAAAAASUVORK5CYII="

func decodeBase64(t *testing.T, data string) []byte {
	t.Helper()
	out, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	return out
}

func app1Segment(payload []byte) []byte {
	segment := make([]byte, len(payload)+4)
	segment[0] = 0xff
	segment[1] = 0xe1
	binary.BigEndian.PutUint16(segment[2:4], uint16(len(payload)+2))
	copy(segment[4:], payload)
	return segment
}

func jpegWithXmpBeforeOrientation(t *testing.T) []byte {
	t.Helper()
	jpeg := decodeBase64(t, tinyJPEG2x1)
	xmp := app1Segment([]byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta xmlns:x=\"adobe:ns:meta/\"/>"))
	tiff, err := hex.DecodeString("49492a0008000000010012010300010000000600000000000000")
	if err != nil {
		t.Fatal(err)
	}
	orientation6 := app1Segment(append([]byte("Exif\x00\x00"), tiff...))
	out := append([]byte{}, jpeg[:2]...)
	out = append(out, xmp...)
	out = append(out, orientation6...)
	out = append(out, jpeg[2:]...)
	return out
}

func TestConvertToPngReturnsPNGUnchanged(t *testing.T) {
	data, mimeType, ok := ConvertToPng(tinyPNG, "image/png")
	if !ok {
		t.Fatal("ConvertToPng PNG ok = false")
	}
	if data != tinyPNG || mimeType != "image/png" {
		t.Errorf("ConvertToPng PNG = %q/%q", mimeType, data)
	}
}

func TestConvertToPngConvertsJPEG(t *testing.T) {
	data, mimeType, ok := ConvertToPng(tinyJPEG, "image/jpeg")
	if !ok {
		t.Fatal("ConvertToPng JPEG ok = false")
	}
	if mimeType != "image/png" {
		t.Errorf("mimeType = %q, want image/png", mimeType)
	}
	decoded := decodeBase64(t, data)
	if len(decoded) < 4 || decoded[0] != 0x89 || decoded[1] != 0x50 || decoded[2] != 0x4e || decoded[3] != 0x47 {
		t.Errorf("not a PNG: % x", decoded[:min(4, len(decoded))])
	}
}

func TestConvertToPngAppliesExifAfterXMP(t *testing.T) {
	jpeg := jpegWithXmpBeforeOrientation(t)
	data, mimeType, ok := ConvertToPng(base64.StdEncoding.EncodeToString(jpeg), "image/jpeg")
	if !ok {
		t.Fatal("ConvertToPng oriented JPEG ok = false")
	}
	if mimeType != "image/png" {
		t.Errorf("mimeType = %q, want image/png", mimeType)
	}
	pngBytes := decodeBase64(t, data)
	if w := binary.BigEndian.Uint32(pngBytes[16:20]); w != 1 {
		t.Errorf("PNG width = %d, want 1", w)
	}
	if h := binary.BigEndian.Uint32(pngBytes[20:24]); h != 2 {
		t.Errorf("PNG height = %d, want 2", h)
	}
}

func TestResizeImageKeepsCallerInputBytesIntact(t *testing.T) {
	input := decodeBase64(t, tinyPNG)
	originalLen := len(input)
	originalFirst := input[0]

	result, ok := ResizeImage(input, "image/png", &model.ModelImageResizeOptions{
		MaxWidth:  new(100),
		MaxHeight: new(100),
		MaxBytes:  new(1024 * 1024),
	})
	if !ok || result == nil {
		t.Fatal("ResizeImage ok = false")
	}
	if len(input) != originalLen || input[0] != originalFirst {
		t.Error("ResizeImage mutated the caller input")
	}
}

func TestResizeImageReturnsOriginalWithinLimits(t *testing.T) {
	result, ok := ResizeImage(decodeBase64(t, tinyPNG), "image/png", &model.ModelImageResizeOptions{
		MaxWidth:  new(100),
		MaxHeight: new(100),
		MaxBytes:  new(1024 * 1024),
	})
	if !ok || result == nil {
		t.Fatal("ResizeImage ok = false")
	}
	if result.WasResized {
		t.Error("WasResized = true, want false")
	}
	if result.Data != tinyPNG {
		t.Error("Data should be the original base64")
	}
	if result.OriginalWidth != 2 || result.OriginalHeight != 2 || result.Width != 2 || result.Height != 2 {
		t.Errorf("dims = %dx%d -> %dx%d, want 2x2 -> 2x2", result.OriginalWidth, result.OriginalHeight, result.Width, result.Height)
	}
}

func TestResizeImageExceedingDimensionLimits(t *testing.T) {
	result, ok := ResizeImage(decodeBase64(t, mediumPNG100x100), "image/png", &model.ModelImageResizeOptions{
		MaxWidth:  new(50),
		MaxHeight: new(50),
		MaxBytes:  new(1024 * 1024),
	})
	if !ok || result == nil {
		t.Fatal("ResizeImage ok = false")
	}
	if !result.WasResized {
		t.Error("WasResized = false, want true")
	}
	if result.OriginalWidth != 100 || result.OriginalHeight != 100 {
		t.Errorf("original = %dx%d, want 100x100", result.OriginalWidth, result.OriginalHeight)
	}
	if result.Width > 50 || result.Height > 50 {
		t.Errorf("resized = %dx%d, want <= 50x50", result.Width, result.Height)
	}
}

func TestResizeImageExceedingByteLimit(t *testing.T) {
	originalBuffer := decodeBase64(t, largePNG200x200)
	originalSize := len(originalBuffer)

	result, ok := ResizeImage(originalBuffer, "image/png", &model.ModelImageResizeOptions{
		MaxWidth:  new(2000),
		MaxHeight: new(2000),
		MaxBytes:  new(len(largePNG200x200) * 9 / 10),
	})
	if !ok || result == nil {
		t.Fatal("ResizeImage ok = false")
	}
	resultBuffer := decodeBase64(t, result.Data)
	if len(resultBuffer) >= originalSize {
		t.Errorf("resized byte length = %d, want < %d", len(resultBuffer), originalSize)
	}
	if len(result.Data) >= len(largePNG200x200) {
		t.Errorf("resized base64 length = %d, want < %d", len(result.Data), len(largePNG200x200))
	}
}

func TestResizeImageCannotFitReturnsNull(t *testing.T) {
	result, ok := ResizeImage(decodeBase64(t, largePNG200x200), "image/png", &model.ModelImageResizeOptions{
		MaxWidth:  new(2000),
		MaxHeight: new(2000),
		MaxBytes:  new(1),
	})
	if ok || result != nil {
		t.Error("ResizeImage should fail for maxBytes=1")
	}
}

func TestResizeImageHandlesJPEG(t *testing.T) {
	result, ok := ResizeImage(decodeBase64(t, tinyJPEG), "image/jpeg", &model.ModelImageResizeOptions{
		MaxWidth:  new(100),
		MaxHeight: new(100),
		MaxBytes:  new(1024 * 1024),
	})
	if !ok || result == nil {
		t.Fatal("ResizeImage JPEG ok = false")
	}
	if result.WasResized {
		t.Error("WasResized = true, want false")
	}
	if result.OriginalWidth != 2 || result.OriginalHeight != 2 {
		t.Errorf("original = %dx%d, want 2x2", result.OriginalWidth, result.OriginalHeight)
	}
}

func TestFormatDimensionNote(t *testing.T) {
	if note := FormatDimensionNote(&ResizedImage{
		MimeType: "image/png", OriginalWidth: 100, OriginalHeight: 100,
		Width: 100, Height: 100, WasResized: false,
	}); note != "" {
		t.Errorf("non-resized note = %q, want empty", note)
	}
	note := FormatDimensionNote(&ResizedImage{
		MimeType: "image/png", OriginalWidth: 2000, OriginalHeight: 1000,
		Width: 1000, Height: 500, WasResized: true,
	})
	for _, want := range []string{"original 2000x1000", "displayed at 1000x500", "2.00"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q does not contain %q", note, want)
		}
	}
}

func createTinyBMP1x1Red24bpp() []byte {
	buf := make([]byte, 58)
	copy(buf[0:2], "BM")
	binary.LittleEndian.PutUint32(buf[2:6], uint32(len(buf)))
	binary.LittleEndian.PutUint32(buf[10:14], 54)
	binary.LittleEndian.PutUint32(buf[14:18], 40)
	binary.LittleEndian.PutUint32(buf[18:22], 1)
	binary.LittleEndian.PutUint32(buf[22:26], 1)
	binary.LittleEndian.PutUint16(buf[26:28], 1)
	binary.LittleEndian.PutUint16(buf[28:30], 24)
	binary.LittleEndian.PutUint32(buf[30:34], 0)
	binary.LittleEndian.PutUint32(buf[34:38], 4)
	buf[56] = 0xff
	return buf
}

func expectPngMagic(t *testing.T, base64Data string) {
	t.Helper()
	buffer := decodeBase64(t, base64Data)
	if len(buffer) < 4 || buffer[0] != 0x89 || buffer[1] != 0x50 || buffer[2] != 0x4e || buffer[3] != 0x47 {
		t.Errorf("not a PNG: % x", buffer[:min(4, len(buffer))])
	}
}

func TestProcessImageConvertsBMPWithoutAutoResize(t *testing.T) {
	result := ProcessImage(createTinyBMP1x1Red24bpp(), "image/bmp", &ProcessImageOptions{AutoResizeImages: new(false)})
	if !result.Ok {
		t.Fatalf("ok = false: %s", result.Message)
	}
	if result.MimeType != "image/png" {
		t.Errorf("mimeType = %q, want image/png", result.MimeType)
	}
	if !containsString(result.Hints, "[Image converted from image/bmp to image/png.]") {
		t.Errorf("hints = %v", result.Hints)
	}
	expectPngMagic(t, result.Data)
}

func TestProcessImageConvertsBMPBeforeAutoResize(t *testing.T) {
	result := ProcessImage(createTinyBMP1x1Red24bpp(), "image/bmp", nil)
	if !result.Ok {
		t.Fatalf("ok = false: %s", result.Message)
	}
	if result.MimeType != "image/png" {
		t.Errorf("mimeType = %q, want image/png", result.MimeType)
	}
	if !containsString(result.Hints, "[Image converted from image/bmp to image/png.]") {
		t.Errorf("hints = %v", result.Hints)
	}
	expectPngMagic(t, result.Data)
}

//go:fix inline

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}

func createPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func readPNGDimensions(t *testing.T, base64Data string) (uint32, uint32) {
	t.Helper()
	buffer := decodeBase64(t, base64Data)
	return binary.BigEndian.Uint32(buffer[16:20]), binary.BigEndian.Uint32(buffer[20:24])
}

func TestNormalizeToolResultImagesNoImages(t *testing.T) {
	content := model.ContentList{model.TextContent{Text: "no images here"}}
	got, changed := NormalizeToolResultImages(content, nil)
	if changed {
		t.Error("changed = true, want false")
	}
	if !reflect.DeepEqual(got, content) {
		t.Errorf("content changed: %#v", got)
	}
}

func TestNormalizeToolResultImagesAlreadyWithinLimits(t *testing.T) {
	content := model.ContentList{
		model.TextContent{Text: "screenshot"},
		model.ImageContent{Data: tinyPNG, MimeType: "image/png"},
	}
	got, changed := NormalizeToolResultImages(content, nil)
	if changed {
		t.Error("changed = true, want false")
	}
	if !reflect.DeepEqual(got, content) {
		t.Errorf("content changed: %#v", got)
	}
}

func TestNormalizeToolResultImagesResizesOversized(t *testing.T) {
	content := model.ContentList{
		model.ImageContent{Data: base64.StdEncoding.EncodeToString(createPNG(t, 2400, 4800)), MimeType: "image/png"},
	}
	normalized, changed := NormalizeToolResultImages(content, nil)
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if len(normalized) != 2 {
		t.Fatalf("len = %d, want 2", len(normalized))
	}
	img, ok := normalized[0].(model.ImageContent)
	if !ok {
		t.Fatalf("normalized[0] is %T", normalized[0])
	}
	width, height := readPNGDimensions(t, img.Data)
	if width > 2000 || height > 2000 {
		t.Errorf("resized = %dx%d, want <= 2000x2000", width, height)
	}
	note, ok := normalized[1].(model.TextContent)
	if !ok {
		t.Fatalf("normalized[1] is %T", normalized[1])
	}
	if !strings.Contains(note.Text, "original 2400x4800") {
		t.Errorf("note = %q", note.Text)
	}
}

func TestNormalizeToolResultImagesLeavesAloneWhenAutoResizeDisabled(t *testing.T) {
	content := model.ContentList{
		model.ImageContent{Data: base64.StdEncoding.EncodeToString(createPNG(t, 2400, 4800)), MimeType: "image/png"},
	}
	got, changed := NormalizeToolResultImages(content, &NormalizeToolResultImagesOptions{AutoResizeImages: new(false)})
	if changed {
		t.Error("changed = true, want false")
	}
	if !reflect.DeepEqual(got, content) {
		t.Errorf("content changed: %#v", got)
	}
}

func TestNormalizeToolResultImagesConvertsUnsupportedWhenAutoResizeDisabled(t *testing.T) {
	content := model.ContentList{
		model.ImageContent{Data: base64.StdEncoding.EncodeToString(createTinyBMP1x1Red24bpp()), MimeType: "image/bmp"},
	}
	normalized, changed := NormalizeToolResultImages(content, &NormalizeToolResultImagesOptions{AutoResizeImages: new(false)})
	if !changed {
		t.Fatal("changed = false, want true")
	}
	img, ok := normalized[0].(model.ImageContent)
	if !ok || img.MimeType != "image/png" {
		t.Errorf("normalized[0] = %#v", normalized[0])
	}
	note, ok := normalized[1].(model.TextContent)
	if !ok || note.Text != "[Image converted from image/bmp to image/png.]" {
		t.Errorf("normalized[1] = %#v", normalized[1])
	}
}

func TestNormalizeToolResultImagesKeepsUndecodable(t *testing.T) {
	content := model.ContentList{
		model.ImageContent{Data: "bm90LWFuLWltYWdl", MimeType: "image/png"},
	}
	got, changed := NormalizeToolResultImages(content, nil)
	if changed {
		t.Error("changed = true, want false")
	}
	if !reflect.DeepEqual(got, content) {
		t.Errorf("content changed: %#v", got)
	}
}

func TestNormalizeToolResultImagesPreservesOrder(t *testing.T) {
	content := model.ContentList{
		model.TextContent{Text: "before"},
		model.ImageContent{Data: base64.StdEncoding.EncodeToString(createPNG(t, 2400, 100)), MimeType: "image/png"},
		model.TextContent{Text: "after"},
	}
	normalized, changed := NormalizeToolResultImages(content, nil)
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if len(normalized) != 4 {
		t.Fatalf("len = %d, want 4", len(normalized))
	}
	types := make([]string, len(normalized))
	for i, block := range normalized {
		types[i] = block.ContentType()
	}
	want := []string{"text", "image", "text", "text"}
	if !reflect.DeepEqual(types, want) {
		t.Errorf("types = %v, want %v", types, want)
	}
	if normalized[0] != content[0] || normalized[3] != content[2] {
		t.Error("surrounding text blocks were not preserved")
	}
}
