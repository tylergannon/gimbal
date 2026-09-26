package files

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
)

// This file adds a small BMP decoder so convertImageBytesToPng covers the most
// common BMP variant (BITMAPINFOHEADER, BI_RGB). The upstream uses Photon, which
// decodes BMP; the Go standard library does not. Only uncompressed true-color
// and palette BMPs are supported here.

func init() {
	image.RegisterFormat("bmp", "BM", decodeBMP, decodeBMPConfig)
}

var errInvalidBMP = errors.New("files: invalid BMP")

type bmpFile struct {
	dataOffset  int
	width       int
	height      int
	bpp         int
	compression uint32
	palette     color.Palette
	pixels      []byte
}

func parseBMP(data []byte) (*bmpFile, error) {
	if len(data) < 18 || data[0] != 'B' || data[1] != 'M' {
		return nil, errInvalidBMP
	}
	dataOffset := int(binary.LittleEndian.Uint32(data[10:14]))
	dibSize := int(binary.LittleEndian.Uint32(data[14:18]))
	if dibSize < 12 || 14+dibSize > len(data) {
		return nil, errInvalidBMP
	}
	width := int(int32(binary.LittleEndian.Uint32(data[18:22])))
	height := int(int32(binary.LittleEndian.Uint32(data[22:26])))
	bpp := int(binary.LittleEndian.Uint16(data[28:30]))
	compression := binary.LittleEndian.Uint32(data[30:34])
	if width <= 0 || height == 0 {
		return nil, errInvalidBMP
	}
	topDown := false
	if height < 0 {
		topDown = true
		height = -height
	}

	b := &bmpFile{
		dataOffset:  dataOffset,
		width:       width,
		height:      height,
		bpp:         bpp,
		compression: compression,
	}
	if topDown {
		// Mark top-down by a negative height; the renderer flips rows when
		// height is still negative. Keep the positive magnitude and a flag in
		// compression-adjacent state would be cleaner, but a negative height is
		// the conventional marker.
		b.height = -height
	}

	if bpp <= 8 {
		entries := int(binary.LittleEndian.Uint32(data[46:50]))
		if entries == 0 {
			entries = 1 << bpp
		}
		paletteStart := 14 + dibSize
		palette := make(color.Palette, 0, entries)
		for i := 0; i < entries; i++ {
			off := paletteStart + i*4
			if off+4 > len(data) {
				return nil, errInvalidBMP
			}
			palette = append(palette, color.RGBA{R: data[off+2], G: data[off+1], B: data[off], A: 0xff})
		}
		b.palette = palette
	}

	rowSize := ((width*bpp + 31) / 32) * 4
	absHeight := height
	if b.height < 0 {
		absHeight = -b.height
	}
	if dataOffset+rowSize*absHeight > len(data) {
		return nil, errInvalidBMP
	}
	b.pixels = data[dataOffset:]
	return b, nil
}

func (b *bmpFile) config() image.Config {
	return image.Config{ColorModel: color.RGBAModel, Width: b.width, Height: absInt(b.height)}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (b *bmpFile) decode() (image.Image, error) {
	if b.compression != 0 {
		return nil, errInvalidBMP
	}
	w, h := b.width, absInt(b.height)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	rowSize := ((w*b.bpp + 31) / 32) * 4
	for row := range h {
		srcRow := row
		if b.height >= 0 {
			// Positive height means the bitmap is stored bottom-up.
			srcRow = h - 1 - row
		}
		rowBytes := b.pixels[srcRow*rowSize : srcRow*rowSize+rowSize]
		for x := range w {
			c, ok := b.pixelAt(rowBytes, x)
			if !ok {
				return nil, errInvalidBMP
			}
			dst.SetRGBA(x, row, c)
		}
	}
	return dst, nil
}

func (b *bmpFile) pixelAt(row []byte, x int) (color.RGBA, bool) {
	switch b.bpp {
	case 32:
		off := x * 4
		if off+4 > len(row) {
			return color.RGBA{}, false
		}
		return color.RGBA{R: row[off+2], G: row[off+1], B: row[off], A: row[off+3]}, true
	case 24:
		off := x * 3
		if off+3 > len(row) {
			return color.RGBA{}, false
		}
		return color.RGBA{R: row[off+2], G: row[off+1], B: row[off], A: 0xff}, true
	case 8:
		if x >= len(row) {
			return color.RGBA{}, false
		}
		return b.paletteColor(int(row[x]))
	case 4:
		off := x / 2
		if off >= len(row) {
			return color.RGBA{}, false
		}
		var idx int
		if x%2 == 0 {
			idx = int(row[off] >> 4)
		} else {
			idx = int(row[off] & 0x0f)
		}
		return b.paletteColor(idx)
	case 1:
		off := x / 8
		if off >= len(row) {
			return color.RGBA{}, false
		}
		idx := int((row[off] >> (7 - uint(x%8))) & 1)
		return b.paletteColor(idx)
	default:
		return color.RGBA{}, false
	}
}

func (b *bmpFile) paletteColor(idx int) (color.RGBA, bool) {
	if idx < 0 || idx >= len(b.palette) {
		return color.RGBA{}, false
	}
	c, ok := b.palette[idx].(color.RGBA)
	if !ok {
		return color.RGBA{}, false
	}
	return c, true
}

func decodeBMP(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b, err := parseBMP(data)
	if err != nil {
		return nil, err
	}
	return b.decode()
}

func decodeBMPConfig(r io.Reader) (image.Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return image.Config{}, err
	}
	b, err := parseBMP(data)
	if err != nil {
		return image.Config{}, err
	}
	return b.config(), nil
}
