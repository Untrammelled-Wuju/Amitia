// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package referenceasset

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	_ "image/jpeg"

	_ "golang.org/x/image/webp"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrInvalidSourcePath = errors.New("invalid source path")
	ErrInvalidOutputPath = errors.New("invalid output path")
	ErrImageDecodeFailed = errors.New("image decode failed")
	ErrImageEncodeFailed = errors.New("image encode failed")
	ErrFileTooLarge      = errors.New("file exceeds max bytes limit")
)

func Normalize(sourcePath, outputPath string, config NormalizeConfig) (*ReferenceAsset, error) {
	if err := validatePath(sourcePath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSourcePath, err)
	}
	if err := validatePath(outputPath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOutputPath, err)
	}

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source file: %w", err)
	}

	if config.MaxBytes > 0 && int64(len(sourceData)) > config.MaxBytes {
		return nil, ErrFileTooLarge
	}

	sourceSum := sha256.Sum256(sourceData)
	sourceHashStr := hex.EncodeToString(sourceSum[:])

	img, format, err := image.Decode(bytes.NewReader(sourceData))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImageDecodeFailed, err)
	}

	sourceMIME := formatToMIME(format)

	bounds := img.Bounds()
	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()

	orientation := readEXIFOrientation(sourceData)
	img = applyOrientation(img, orientation)

	rgba := convertToRGBA(img)

	bgColor := parseBackgroundColor(config.BackgroundColor)

	normalized := normalizeSize(rgba, config.TargetWidth, config.TargetHeight, bgColor)

	normBounds := normalized.Bounds()
	normWidth := normBounds.Dx()
	normHeight := normBounds.Dy()

	var buf bytes.Buffer
	if err := png.Encode(&buf, normalized); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImageEncodeFailed, err)
	}

	normalizedData := buf.Bytes()
	normSum := sha256.Sum256(normalizedData)
	normalizedHashStr := hex.EncodeToString(normSum[:])

	configHash := computeConfigHash(config)

	if err := WriteAtomically(outputPath, normalizedData); err != nil {
		return nil, fmt.Errorf("write normalized file: %w", err)
	}

	return &ReferenceAsset{
		SourcePath:       sourcePath,
		SourceHash:       sourceHashStr,
		SourceMIME:       sourceMIME,
		SourceWidth:      sourceWidth,
		SourceHeight:     sourceHeight,
		NormalizedPath:   outputPath,
		NormalizedHash:   normalizedHashStr,
		NormalizedMIME:   "image/png",
		NormalizedWidth:  normWidth,
		NormalizedHeight: normHeight,
		ConfigHash:       configHash,
		CreatedAt:        time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func formatToMIME(format string) string {
	switch strings.ToLower(format) {
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func readEXIFOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}

	offset := 2
	for offset+1 < len(data) {
		if data[offset] != 0xFF {
			break
		}
		marker := data[offset+1]
		if marker == 0xD9 || marker == 0xDA {
			break
		}

		if offset+3 >= len(data) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		if segLen < 2 || offset+2+segLen > len(data) {
			break
		}

		if marker == 0xE1 && segLen > 6 {
			segStart := offset + 4
			if segStart+6 <= len(data) && string(data[segStart:segStart+4]) == "Exif" {
				exifEnd := offset + 2 + segLen
				if segStart+6 < exifEnd {
					return parseEXIFIFD(data[segStart+6 : exifEnd])
				}
			}
		}

		offset += 2 + segLen
	}

	return 1
}

func parseEXIFIFD(exifData []byte) int {
	if len(exifData) < 8 {
		return 1
	}

	var byteOrder binary.ByteOrder
	switch {
	case bytes.Equal(exifData[0:2], []byte{0x49, 0x49}):
		byteOrder = binary.LittleEndian
	case bytes.Equal(exifData[0:2], []byte{0x4D, 0x4D}):
		byteOrder = binary.BigEndian
	default:
		return 1
	}

	if byteOrder.Uint16(exifData[2:4]) != 0x002A {
		return 1
	}

	ifdOffset := int(byteOrder.Uint32(exifData[4:8]))
	if ifdOffset+2 > len(exifData) {
		return 1
	}

	entryCount := int(byteOrder.Uint16(exifData[ifdOffset : ifdOffset+2]))
	for i := 0; i < entryCount; i++ {
		entryOffset := ifdOffset + 2 + i*12
		if entryOffset+12 > len(exifData) {
			break
		}
		tag := byteOrder.Uint16(exifData[entryOffset : entryOffset+2])
		if tag == 0x0112 {
			value := byteOrder.Uint16(exifData[entryOffset+8 : entryOffset+10])
			return int(value)
		}
	}

	return 1
}

func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 1:
		return img
	case 2:
		return flipHorizontal(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipVertical(img)
	case 5:
		return flipHorizontal(rotate90(img))
	case 6:
		return rotate90(img)
	case 7:
		return flipHorizontal(rotate270(img))
	case 8:
		return rotate270(img)
	default:
		return img
	}
}

func flipHorizontal(img image.Image) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, srcW, srcH))
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			srcX := bounds.Min.X + (srcW - 1 - x)
			srcY := bounds.Min.Y + y
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func flipVertical(img image.Image) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, srcW, srcH))
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			srcX := bounds.Min.X + x
			srcY := bounds.Min.Y + (srcH - 1 - y)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func rotate90(img image.Image) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, srcH, srcW))
	for y := 0; y < srcW; y++ {
		for x := 0; x < srcH; x++ {
			srcX := bounds.Min.X + (srcW - 1 - y)
			srcY := bounds.Min.Y + x
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func rotate180(img image.Image) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, srcW, srcH))
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			srcX := bounds.Min.X + (srcW - 1 - x)
			srcY := bounds.Min.Y + (srcH - 1 - y)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func rotate270(img image.Image) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, srcH, srcW))
	for y := 0; y < srcW; y++ {
		for x := 0; x < srcH; x++ {
			srcX := bounds.Min.X + y
			srcY := bounds.Min.Y + (srcH - 1 - x)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func convertToRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		bounds := rgba.Bounds()
		if bounds.Min.X == 0 && bounds.Min.Y == 0 {
			return rgba
		}
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return rgba
}

func normalizeSize(src *image.RGBA, targetW, targetH int, bg color.Color) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	if targetW <= 0 {
		targetW = srcW
	}
	if targetH <= 0 {
		targetH = srcH
	}

	scaleW := float64(targetW) / float64(srcW)
	scaleH := float64(targetH) / float64(srcH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}

	newW := srcW
	newH := srcH
	if scale < 1.0 {
		newW = int(math.Round(float64(srcW) * scale))
		newH = int(math.Round(float64(srcH) * scale))
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}
	}

	var resized *image.RGBA
	if newW == srcW && newH == srcH {
		resized = src
	} else {
		resized = bilinearResize(src, newW, newH)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	fillColor(canvas, bg)

	offsetX := (targetW - newW) / 2
	offsetY := (targetH - newH) / 2

	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			canvas.SetRGBA(offsetX+x, offsetY+y, resized.RGBAAt(x, y))
		}
	}

	return canvas
}

func bilinearResize(src *image.RGBA, newW, newH int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	scaleX := float64(srcW) / float64(newW)
	scaleY := float64(srcH) / float64(newH)

	for y := 0; y < newH; y++ {
		srcY := (float64(y)+0.5)*scaleY - 0.5
		y0 := int(math.Floor(srcY))
		y1 := y0 + 1
		fy := srcY - float64(y0)
		if y0 < 0 {
			y0 = 0
		}
		if y0 >= srcH {
			y0 = srcH - 1
		}
		if y1 < 0 {
			y1 = 0
		}
		if y1 >= srcH {
			y1 = srcH - 1
		}

		for x := 0; x < newW; x++ {
			srcX := (float64(x)+0.5)*scaleX - 0.5
			x0 := int(math.Floor(srcX))
			x1 := x0 + 1
			fx := srcX - float64(x0)
			if x0 < 0 {
				x0 = 0
			}
			if x0 >= srcW {
				x0 = srcW - 1
			}
			if x1 < 0 {
				x1 = 0
			}
			if x1 >= srcW {
				x1 = srcW - 1
			}

			c00 := src.RGBAAt(x0, y0)
			c10 := src.RGBAAt(x1, y0)
			c01 := src.RGBAAt(x0, y1)
			c11 := src.RGBAAt(x1, y1)

			r := bilinearChannel(c00.R, c10.R, c01.R, c11.R, fx, fy)
			g := bilinearChannel(c00.G, c10.G, c01.G, c11.G, fx, fy)
			b := bilinearChannel(c00.B, c10.B, c01.B, c11.B, fx, fy)
			a := bilinearChannel(c00.A, c10.A, c01.A, c11.A, fx, fy)

			dst.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return dst
}

func bilinearChannel(c00, c10, c01, c11 uint8, fx, fy float64) uint8 {
	v00 := float64(c00)
	v10 := float64(c10)
	v01 := float64(c01)
	v11 := float64(c11)
	top := v00*(1-fx) + v10*fx
	bottom := v01*(1-fx) + v11*fx
	val := top*(1-fy) + bottom*fy
	if val < 0 {
		val = 0
	}
	if val > 255 {
		val = 255
	}
	return uint8(math.Round(val))
}

func fillColor(img *image.RGBA, c color.Color) {
	r, g, b, a := c.RGBA()
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)
	a8 := uint8(a >> 8)
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{R: r8, G: g8, B: b8, A: a8})
		}
	}
}

func parseBackgroundColor(s string) color.Color {
	s = strings.TrimSpace(s)
	if s == "" || s == "transparent" {
		return color.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	if strings.HasPrefix(s, "#") {
		hex := s[1:]
		switch len(hex) {
		case 6:
			r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
			g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
			b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
			if err1 != nil || err2 != nil || err3 != nil {
				return color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
		case 8:
			r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
			g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
			b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
			a, err4 := strconv.ParseUint(hex[6:8], 16, 8)
			if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
				return color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}
		}
	}
	return color.RGBA{R: 255, G: 255, B: 255, A: 255}
}

func computeConfigHash(config NormalizeConfig) string {
	data := fmt.Sprintf("%d|%d|%s|%d|%s",
		config.TargetWidth,
		config.TargetHeight,
		config.TargetMIME,
		config.MaxBytes,
		config.BackgroundColor,
	)
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
