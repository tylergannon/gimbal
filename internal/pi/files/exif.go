package files

import (
	"bytes"
	"encoding/binary"
	"image"
)

// applyExifOrientationFromBytes applies the EXIF orientation found in data to
// img. It is a port of pi's applyExifOrientation restricted to the cases the
// in-process pipeline needs (JPEG and WebP EXIF).
func applyExifOrientationFromBytes(img image.Image, data []byte) image.Image {
	orientation := exifOrientationFromBytes(data)
	if orientation <= 1 {
		return img
	}
	return applyOrientation(img, orientation)
}

// exifOrientationFromBytes reads the EXIF orientation (1-8) from JPEG or WebP
// bytes, returning 1 when absent.
func exifOrientationFromBytes(data []byte) int {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return jpegOrientation(data)
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return webpOrientation(data)
	}
	return 1
}

func jpegOrientation(data []byte) int {
	off := findJpegTiffOffset(data)
	if off < 0 {
		return 1
	}
	return tiffOrientation(data, off)
}

var exifHeader = []byte("Exif\x00\x00")

func hasExifHeader(b []byte) bool { return bytes.HasPrefix(b, exifHeader) }

// findJpegTiffOffset walks the JPEG marker segments and returns the offset of
// the TIFF header carried by the first APP1 with an "Exif\0\0" header, or -1.
func findJpegTiffOffset(data []byte) int {
	for off := 2; off < len(data)-1; {
		if data[off] != 0xFF {
			return -1
		}
		marker := data[off+1]
		if marker == 0xFF { // fill byte
			off++
			continue
		}
		if marker == 0xE1 { // APP1
			segStart := off + 4
			if segStart+6 > len(data) {
				return -1
			}
			if hasExifHeader(data[segStart:]) {
				return segStart + 6
			}
		}
		if off+4 > len(data) {
			return -1
		}
		off += 2 + int(binary.BigEndian.Uint16(data[off+2:off+4]))
	}
	return -1
}

func webpOrientation(data []byte) int {
	off := findWebpTiffOffset(data)
	if off < 0 {
		return 1
	}
	return tiffOrientation(data, off)
}

// findWebpTiffOffset returns the offset of the TIFF header in the WebP EXIF
// chunk, or -1.
func findWebpTiffOffset(data []byte) int {
	for off := 12; off+8 <= len(data); {
		chunkID := string(data[off : off+4])
		chunkSize := int(int32(binary.LittleEndian.Uint32(data[off+4 : off+8])))
		dataStart := off + 8
		if chunkID == "EXIF" {
			if dataStart+chunkSize > len(data) {
				return -1
			}
			if chunkSize >= 6 && hasExifHeader(data[dataStart:]) {
				return dataStart + 6
			}
			return dataStart
		}
		if chunkSize < 0 {
			return -1
		}
		off = dataStart + chunkSize + chunkSize%2
	}
	return -1
}

// tiffOrientation reads the Orientation tag (0x0112) out of the TIFF header at
// tiffStart, returning 1 when there is none.
func tiffOrientation(data []byte, tiffStart int) int {
	if tiffStart+8 > len(data) {
		return 1
	}
	le := data[tiffStart] == 'I' && data[tiffStart+1] == 'I'
	byteAt := func(pos int) int {
		if pos < 0 || pos >= len(data) {
			return 0
		}
		return int(data[pos])
	}
	read16 := func(pos int) int {
		if le {
			return byteAt(pos) | byteAt(pos+1)<<8
		}
		return byteAt(pos)<<8 | byteAt(pos+1)
	}
	read32 := func(pos int) int {
		if le {
			u := uint32(byteAt(pos+3))<<24 | uint32(byteAt(pos+2))<<16 | uint32(byteAt(pos+1))<<8 | uint32(byteAt(pos))
			return int(int32(u))
		}
		return int(uint32(byteAt(pos))<<24 | uint32(byteAt(pos+1))<<16 | uint32(byteAt(pos+2))<<8 | uint32(byteAt(pos+3)))
	}

	ifdStart := tiffStart + read32(tiffStart+4)
	if ifdStart+2 > len(data) {
		return 1
	}
	for n, count := 0, read16(ifdStart); n < count; n++ {
		entry := ifdStart + 2 + n*12
		if entry+12 > len(data) {
			return 1
		}
		if read16(entry) == 0x0112 { // Orientation
			if v := read16(entry + 8); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation rotates/flips img per the EXIF orientation value (1-8),
// matching pi's applyExifOrientation pixel mapping.
func applyOrientation(img image.Image, orientation int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	transform := func(dstW, dstH int, mapXY func(x, y int) (int, int)) image.Image {
		dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
		for y := range dstH {
			for x := range dstW {
				sx, sy := mapXY(x, y)
				dst.Set(x, y, img.At(b.Min.X+sx, b.Min.Y+sy))
			}
		}
		return dst
	}
	switch orientation {
	case 2: // flip horizontal
		return transform(w, h, func(x, y int) (int, int) { return w - 1 - x, y })
	case 3: // rotate 180
		return transform(w, h, func(x, y int) (int, int) { return w - 1 - x, h - 1 - y })
	case 4: // flip vertical
		return transform(w, h, func(x, y int) (int, int) { return x, h - 1 - y })
	case 5: // transpose
		return transform(h, w, func(x, y int) (int, int) { return y, x })
	case 6: // rotate 90 CW
		return transform(h, w, func(x, y int) (int, int) { return y, h - 1 - x })
	case 7: // transverse
		return transform(h, w, func(x, y int) (int, int) { return w - 1 - y, h - 1 - x })
	case 8: // rotate 90 CCW
		return transform(h, w, func(x, y int) (int, int) { return w - 1 - y, x })
	default:
		return img
	}
}
