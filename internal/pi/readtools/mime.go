package readtools

import (
	"io"
	"os"
)

// imageTypeSniffBytes is how many leading bytes the mime sniffer reads.
const imageTypeSniffBytes = 4100

var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

// detectSupportedImageMimeType identifies a supported image type from its magic
// bytes (port of utils/mime.ts detectSupportedImageMimeType). It returns "" for
// CMYK JPEG (ffd8fff7), animated PNG (acTL), non-IHDR PNG, and anything that is
// not a supported image.
func detectSupportedImageMimeType(buf []byte) string {
	if bytesStartWith(buf, []byte{0xff, 0xd8, 0xff}) {
		if len(buf) > 3 && buf[3] == 0xf7 {
			return ""
		}
		return "image/jpeg"
	}
	if bytesStartWith(buf, pngSignature) {
		if isPNG(buf) && !isAnimatedPNG(buf) {
			return "image/png"
		}
		return ""
	}
	if startsWithASCII(buf, 0, "GIF87a") || startsWithASCII(buf, 0, "GIF89a") {
		return "image/gif"
	}
	if startsWithASCII(buf, 0, "RIFF") && startsWithASCII(buf, 8, "WEBP") {
		return "image/webp"
	}
	if startsWithASCII(buf, 0, "BM") && isBMP(buf) {
		return "image/bmp"
	}
	return ""
}

// detectSupportedImageMimeTypeFromFile reads up to the sniff window from a file
// and identifies a supported image type (mime.ts
// detectSupportedImageMimeTypeFromFile). It returns "" for a non-image or an
// unreadable file, matching pi's null.
func detectSupportedImageMimeTypeFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, imageTypeSniffBytes)
	// ReadFull so a short first read (pipes, network filesystems) cannot
	// truncate the sniff window; EOF just means the file is small.
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return ""
	}
	return detectSupportedImageMimeType(buf[:n])
}

// isPNG validates the PNG signature plus a 13-byte IHDR at offset 12.
func isPNG(buf []byte) bool {
	return len(buf) >= 16 && readUint32BE(buf, len(pngSignature)) == 13 && startsWithASCII(buf, 12, "IHDR")
}

// isAnimatedPNG walks PNG chunks looking for acTL before IDAT.
func isAnimatedPNG(buf []byte) bool {
	offset := len(pngSignature)
	for offset+8 <= len(buf) {
		chunkLength := readUint32BE(buf, offset)
		chunkTypeOffset := offset + 4
		if startsWithASCII(buf, chunkTypeOffset, "acTL") {
			return true
		}
		if startsWithASCII(buf, chunkTypeOffset, "IDAT") {
			return false
		}
		nextOffset := offset + 8 + chunkLength + 4
		if nextOffset <= offset || nextOffset > len(buf) {
			return false
		}
		offset = nextOffset
	}
	return false
}

// isBMP validates the BMP magic and DIB header (port of utils/mime.ts isBmp).
func isBMP(buf []byte) bool {
	if len(buf) < 26 {
		return false
	}
	declaredFileSize := readUint32LE(buf, 2)
	pixelDataOffset := readUint32LE(buf, 10)
	dibHeaderSize := readUint32LE(buf, 14)
	if declaredFileSize != 0 && declaredFileSize < 26 {
		return false
	}
	if pixelDataOffset < 14+dibHeaderSize {
		return false
	}
	if declaredFileSize != 0 && pixelDataOffset >= declaredFileSize {
		return false
	}

	var colorPlanes, bitsPerPixel int
	switch {
	case dibHeaderSize == 12:
		colorPlanes = readUint16LE(buf, 22)
		bitsPerPixel = readUint16LE(buf, 24)
	case dibHeaderSize >= 40 && dibHeaderSize <= 124:
		if len(buf) < 30 {
			return false
		}
		colorPlanes = readUint16LE(buf, 26)
		bitsPerPixel = readUint16LE(buf, 28)
	default:
		return false
	}
	if colorPlanes != 1 {
		return false
	}
	switch bitsPerPixel {
	case 1, 4, 8, 16, 24, 32:
		return true
	default:
		return false
	}
}

func readUint16LE(buf []byte, offset int) int {
	return byteAt(buf, offset) + (byteAt(buf, offset+1) << 8)
}

func readUint32LE(buf []byte, offset int) int {
	return byteAt(buf, offset) + (byteAt(buf, offset+1) << 8) + (byteAt(buf, offset+2) << 16) + byteAt(buf, offset+3)*0x1000000
}

func readUint32BE(buf []byte, offset int) int {
	return byteAt(buf, offset)*0x1000000 + (byteAt(buf, offset+1) << 16) + (byteAt(buf, offset+2) << 8) + byteAt(buf, offset+3)
}

func byteAt(buf []byte, i int) int {
	if i < len(buf) {
		return int(buf[i])
	}
	return 0
}

func bytesStartWith(buf, prefix []byte) bool {
	if len(buf) < len(prefix) {
		return false
	}
	for i := range prefix {
		if buf[i] != prefix[i] {
			return false
		}
	}
	return true
}

func startsWithASCII(buf []byte, offset int, text string) bool {
	if len(buf) < offset+len(text) {
		return false
	}
	for i := 0; i < len(text); i++ {
		if buf[offset+i] != text[i] {
			return false
		}
	}
	return true
}
