package files

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // register the GIF decoder so image.Decode recognizes GIF
	"image/jpeg"
	"image/png"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

const (
	imgMaxWidth           = 2000
	imgMaxHeight          = 2000
	imgDefaultJPEGQuality = 80
)

// imgMaxBase64Bytes is 4.5MB of base64 payload, headroom below Anthropic's 5MB
// limit.
var imgMaxBase64Bytes = int(4.5 * 1024 * 1024)

// resizeProfile is pi's `{ ...DEFAULT_OPTIONS, ...options }`: the resize limits
// after a model's resize options have narrowed the defaults.
type resizeProfile struct {
	maxWidth      int
	maxHeight     int
	maxBytes      int
	jpegQualities []int
}

// resolveResizeProfile applies a model's resize options over the pipeline
// defaults. A nil profile, or a nil field inside one, keeps the default.
func resolveResizeProfile(o *model.ModelImageResizeOptions) resizeProfile {
	p := resizeProfile{maxWidth: imgMaxWidth, maxHeight: imgMaxHeight, maxBytes: imgMaxBase64Bytes}
	quality := imgDefaultJPEGQuality
	if o != nil {
		if o.MaxWidth != nil {
			p.maxWidth = *o.MaxWidth
		}
		if o.MaxHeight != nil {
			p.maxHeight = *o.MaxHeight
		}
		if o.MaxBytes != nil {
			p.maxBytes = *o.MaxBytes
		}
		if o.JPEGQuality != nil {
			quality = *o.JPEGQuality
		}
	}
	// pi: Array.from(new Set([opts.jpegQuality, 85, 70, 55, 40])) — insertion
	// order, first occurrence wins.
	for _, q := range []int{quality, 85, 70, 55, 40} {
		seen := slices.Contains(p.jpegQualities, q)
		if !seen {
			p.jpegQualities = append(p.jpegQualities, q)
		}
	}
	return p
}

// ResizedImage mirrors the object pi's resizeImage returns. Data is base64.
type ResizedImage struct {
	Data           string
	MimeType       string
	OriginalWidth  int
	OriginalHeight int
	Width          int
	Height         int
	WasResized     bool
}

// ProcessImageOptions controls ProcessImage.
type ProcessImageOptions struct {
	// AutoResizeImages resizes images to inline provider limits. Nil defaults to true.
	AutoResizeImages *bool
	// ResizeOptions are optional resize overrides. Defaults apply when nil.
	ResizeOptions *model.ModelImageResizeOptions
}

// ProcessImageResult mirrors pi's discriminated ProcessImageResult. On success
// Ok is true and Data (base64), MimeType, and Hints are populated; on failure
// Ok is false and Message holds the omission note.
type ProcessImageResult struct {
	Ok       bool
	Data     string
	MimeType string
	Hints    []string
	Message  string
}

// NormalizeToolResultImagesOptions controls NormalizeToolResultImages.
type NormalizeToolResultImagesOptions struct {
	// AutoResizeImages resizes oversized images to inline provider limits. Nil defaults to true.
	AutoResizeImages *bool
	// ResizeOptions is a model-specific resize profile.
	ResizeOptions *model.ModelImageResizeOptions
}

// base64Size returns the encoded length of n bytes — ceil(n/3)*4.
func base64Size(n int) int { return ((n + 2) / 3) * 4 }

// jsRound mirrors JS Math.round (round half toward +Infinity) for non-negative x.
func jsRound(x float64) int { return int(math.Floor(x + 0.5)) }

// ResizeImage resizes an image to fit within the specified max dimensions and
// encoded file size. It returns the result and true, or nil and false when the
// image cannot be brought under the byte limit (pi returns null).
func ResizeImage(inputBytes []byte, mimeType string, options *model.ModelImageResizeOptions) (*ResizedImage, bool) {
	p := resolveResizeProfile(options)
	inputB64 := base64Size(len(inputBytes))

	img, format, err := image.Decode(bytes.NewReader(inputBytes))
	if err != nil {
		return nil, false
	}

	oriented := applyExifOrientationFromBytes(img, inputBytes)
	ob := oriented.Bounds()
	ow, oh := ob.Dx(), ob.Dy()

	if mimeType == "" {
		mimeType = "image/" + format
	}

	if ow <= p.maxWidth && oh <= p.maxHeight && inputB64 < p.maxBytes {
		return &ResizedImage{
			Data:           base64.StdEncoding.EncodeToString(inputBytes),
			MimeType:       mimeType,
			OriginalWidth:  ow,
			OriginalHeight: oh,
			Width:          ow,
			Height:         oh,
			WasResized:     false,
		}, true
	}

	tw, th := ow, oh
	if tw > p.maxWidth {
		th = jsRound(float64(th) * float64(p.maxWidth) / float64(tw))
		tw = p.maxWidth
	}
	if th > p.maxHeight {
		tw = jsRound(float64(tw) * float64(p.maxHeight) / float64(th))
		th = p.maxHeight
	}

	cw, ch := tw, th
	for {
		scaled := oriented
		if cw != ow || ch != oh {
			scaled = bilinearResize(oriented, cw, ch)
		}
		if data, mime, fit := encodeUnderLimit(scaled, p); fit {
			return &ResizedImage{
				Data:           data,
				MimeType:       mime,
				OriginalWidth:  ow,
				OriginalHeight: oh,
				Width:          cw,
				Height:         ch,
				WasResized:     true,
			}, true
		}
		if cw == 1 && ch == 1 {
			break
		}
		nw, nh := cw, ch
		if cw != 1 {
			nw = max1(int(math.Floor(float64(cw) * 0.75)))
		}
		if ch != 1 {
			nh = max1(int(math.Floor(float64(ch) * 0.75)))
		}
		if nw == cw && nh == ch {
			break
		}
		cw, ch = nw, nh
	}
	return nil, false
}

// ConvertToPng converts an image to PNG when needed (already-PNG input is
// returned unchanged). It returns the base64 data, mime type, and ok.
func ConvertToPng(base64Data, mimeType string) (data string, outMimeType string, ok bool) {
	if mimeType == "image/png" {
		return base64Data, mimeType, true
	}
	bytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		bytes = decodeNodeBase64(base64Data)
	}
	pngBytes := convertImageBytesToPng(bytes)
	if pngBytes == nil {
		return "", "", false
	}
	return base64.StdEncoding.EncodeToString(pngBytes), "image/png", true
}

// convertImageBytesToPng decodes arbitrary image bytes, applies EXIF
// orientation, and re-encodes as PNG. It returns nil on decode failure.
func convertImageBytesToPng(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	oriented := applyExifOrientationFromBytes(img, data)
	var buf bytes.Buffer
	if err := png.Encode(&buf, oriented); err != nil {
		return nil
	}
	return buf.Bytes()
}

// ProcessImage normalizes an image to a supported inline mime type, optionally
// auto-resizes it below the inline limit, and reports processing hints. It is a
// faithful port of pi's processImage.
func ProcessImage(data []byte, mimeType string, options *ProcessImageOptions) ProcessImageResult {
	autoResize := true
	if options != nil && options.AutoResizeImages != nil {
		autoResize = *options.AutoResizeImages
	}
	var resizeOptions *model.ModelImageResizeOptions
	if options != nil {
		resizeOptions = options.ResizeOptions
	}

	normalizedMime := normalizeSupportedImageMimeType(mimeType)
	normBytes := data
	convertedFrom := ""
	if normalizedMime == "" {
		pngBytes := convertImageBytesToPng(data)
		if pngBytes == nil {
			return ProcessImageResult{
				Ok:      false,
				Message: "[Image omitted: could not be converted to a supported inline image format.]",
			}
		}
		normBytes = pngBytes
		normalizedMime = "image/png"
		convertedFrom = baseMimeType(mimeType)
	}

	if autoResize {
		resized, ok := ResizeImage(normBytes, normalizedMime, resizeOptions)
		if !ok {
			return ProcessImageResult{
				Ok:      false,
				Message: "[Image omitted: could not be resized below the inline image size limit.]",
			}
		}
		var hints []string
		if h := conversionHint(convertedFrom, resized.MimeType); h != "" {
			hints = append(hints, h)
		}
		if dn := FormatDimensionNote(resized); dn != "" {
			hints = append(hints, dn)
		}
		return ProcessImageResult{Ok: true, Data: resized.Data, MimeType: resized.MimeType, Hints: hints}
	}

	var hints []string
	if h := conversionHint(convertedFrom, normalizedMime); h != "" {
		hints = append(hints, h)
	}
	return ProcessImageResult{
		Ok:       true,
		Data:     base64.StdEncoding.EncodeToString(normBytes),
		MimeType: normalizedMime,
		Hints:    hints,
	}
}

// NormalizeToolResultImages normalizes the image blocks of a tool result,
// porting pi's normalizeToolResultImages. The second return value reports
// whether anything changed, so callers can skip rewriting the result.
func NormalizeToolResultImages(content model.ContentList, options *NormalizeToolResultImagesOptions) (model.ContentList, bool) {
	hasImage := false
	for _, block := range content {
		if _, ok := block.(model.ImageContent); ok {
			hasImage = true
			break
		}
	}
	if !hasImage {
		return content, false
	}

	autoResize := true
	if options != nil && options.AutoResizeImages != nil {
		autoResize = *options.AutoResizeImages
	}
	var resizeOptions *model.ModelImageResizeOptions
	if options != nil {
		resizeOptions = options.ResizeOptions
	}

	normalized := make(model.ContentList, 0, len(content))
	changed := false
	for _, block := range content {
		img, ok := block.(model.ImageContent)
		if !ok {
			normalized = append(normalized, block)
			continue
		}
		processed := ProcessImage(
			decodeNodeBase64(img.Data),
			img.MimeType,
			&ProcessImageOptions{AutoResizeImages: &autoResize, ResizeOptions: resizeOptions},
		)
		// Unlike `read`, keep the original block whenever processing fails. The
		// tool already produced this image and the failure may just be an
		// unavailable backend, so passing it through preserves the behavior
		// tools have today instead of silently deleting their output.
		if !processed.Ok {
			normalized = append(normalized, block)
			continue
		}
		if processed.Data == img.Data && processed.MimeType == img.MimeType && len(processed.Hints) == 0 {
			normalized = append(normalized, block)
			continue
		}
		normalized = append(normalized, model.ImageContent{Data: processed.Data, MimeType: processed.MimeType})
		if len(processed.Hints) > 0 {
			normalized = append(normalized, model.TextContent{Text: strings.Join(processed.Hints, "\n")})
		}
		changed = true
	}
	if !changed {
		return content, false
	}
	return normalized, true
}

// FormatDimensionNote returns a coordinate-mapping hint, or "" when the image
// was not resized.
func FormatDimensionNote(r *ResizedImage) string {
	if r == nil || !r.WasResized || r.Width == 0 {
		return ""
	}
	scale := float64(r.OriginalWidth) / float64(r.Width)
	return fmt.Sprintf("[Image: original %dx%d, displayed at %dx%d. Multiply coordinates by %s to map to original image.]",
		r.OriginalWidth, r.OriginalHeight, r.Width, r.Height, toFixed2(scale))
}

// normalizeSupportedImageMimeType returns the inline mime type models accept, or
// "" when the input must be converted.
func normalizeSupportedImageMimeType(mimeType string) string {
	switch baseMimeType(mimeType) {
	case "image/png":
		return "image/png"
	case "image/jpeg", "image/jpg":
		return "image/jpeg"
	case "image/gif":
		return "image/gif"
	case "image/webp":
		return "image/webp"
	default:
		return ""
	}
}

func conversionHint(from, to string) string {
	if from == "" || from == to {
		return ""
	}
	return fmt.Sprintf("[Image converted from %s to %s.]", from, to)
}

// decodeNodeBase64 decodes base64 the way Node's Buffer.from(value, "base64")
// does: it ignores characters outside the base64 alphabet, accepts the base64url
// alphabet, treats padding as optional, and stops at the first '='.
func decodeNodeBase64(value string) []byte {
	var b strings.Builder
	b.Grow(len(value))
scan:
	for _, r := range value {
		switch {
		case r == '=':
			break scan
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/':
			b.WriteRune(r)
		case r == '-': // base64url
			b.WriteByte('+')
		case r == '_': // base64url
			b.WriteByte('/')
		}
	}
	cleaned := b.String()
	// A group of one leftover character carries no whole byte; Node discards it.
	if len(cleaned)%4 == 1 {
		cleaned = cleaned[:len(cleaned)-1]
	}
	out, _ := base64.RawStdEncoding.DecodeString(cleaned)
	return out
}

// encodeUnderLimit encodes img as PNG, then JPEG at each quality step, returning
// the first base64 candidate under the byte limit.
func encodeUnderLimit(img image.Image, p resizeProfile) (data, mimeType string, ok bool) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err == nil {
		s := base64.StdEncoding.EncodeToString(buf.Bytes())
		if len(s) < p.maxBytes {
			return s, "image/png", true
		}
	}
	for _, q := range p.jpegQualities {
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err == nil {
			s := base64.StdEncoding.EncodeToString(buf.Bytes())
			if len(s) < p.maxBytes {
				return s, "image/jpeg", true
			}
		}
	}
	return "", "", false
}

// bilinearResize downscales src to tw×th using bilinear interpolation.
func bilinearResize(src image.Image, tw, th int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	if sw == 0 || sh == 0 {
		return dst
	}
	xRatio := float64(sw-1) / float64(max1(tw-1))
	yRatio := float64(sh-1) / float64(max1(th-1))
	for y := range th {
		fy := float64(y) * yRatio
		y0 := int(fy)
		dy := fy - float64(y0)
		for x := range tw {
			fx := float64(x) * xRatio
			x0 := int(fx)
			dx := fx - float64(x0)
			r00, g00, b00, a00 := at(src, sb.Min.X+x0, sb.Min.Y+y0)
			r10, g10, b10, a10 := at(src, sb.Min.X+x0+1, sb.Min.Y+y0)
			r01, g01, b01, a01 := at(src, sb.Min.X+x0, sb.Min.Y+y0+1)
			r11, g11, b11, a11 := at(src, sb.Min.X+x0+1, sb.Min.Y+y0+1)
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(lerp2(r00, r10, r01, r11, dx, dy)),
				G: uint8(lerp2(g00, g10, g01, g11, dx, dy)),
				B: uint8(lerp2(b00, b10, b01, b11, dx, dy)),
				A: uint8(lerp2(a00, a10, a01, a11, dx, dy)),
			})
		}
	}
	return dst
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func at(img image.Image, x, y int) (r, g, b, a uint32) {
	bb := img.Bounds()
	if x >= bb.Max.X {
		x = bb.Max.X - 1
	}
	if y >= bb.Max.Y {
		y = bb.Max.Y - 1
	}
	return img.At(x, y).RGBA()
}

func lerp2(c00, c10, c01, c11 uint32, dx, dy float64) uint32 {
	top := float64(c00>>8)*(1-dx) + float64(c10>>8)*dx
	bot := float64(c01>>8)*(1-dx) + float64(c11>>8)*dx
	return uint32(top*(1-dy) + bot*dy)
}

// toFixed2 formats x with exactly two decimals the way JS Number.toFixed(2)
// does: round half up, computed exactly on x's binary value.
func toFixed2(x float64) string {
	r := new(big.Rat).SetFloat64(x)
	if r == nil || x < 0 {
		return strconv.FormatFloat(x, 'f', 2, 64)
	}
	r.Mul(r, big.NewRat(100, 1))
	r.Add(r, big.NewRat(1, 2))
	digits := new(big.Int).Quo(r.Num(), r.Denom()).String()
	for len(digits) < 3 {
		digits = "0" + digits
	}
	return digits[:len(digits)-2] + "." + digits[len(digits)-2:]
}
