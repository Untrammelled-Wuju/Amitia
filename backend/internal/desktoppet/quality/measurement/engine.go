// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package measurement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

const (
	MaxFileBytesToHash = 10 * 1024 * 1024
	MagicBytesToRead   = 512
)

type ImageMeasurementEngineImpl struct {
	cache quality.QualityRepository
}

func NewImageMeasurementEngine(cache quality.QualityRepository) *ImageMeasurementEngineImpl {
	return &ImageMeasurementEngineImpl{cache: cache}
}

func (e *ImageMeasurementEngineImpl) MeasureFrame(ctx context.Context, framePath string, contentHash string, frameArtifactID string) (*quality.FrameMeasurementResult, error) {
	fileInfo, err := os.Stat(framePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("frame file not found: %s", framePath)
		}
		return nil, err
	}
	fileSize := fileInfo.Size()

	mimeType, err := detectMIMEType(framePath)
	if err != nil {
		mimeType = inferMimeType(framePath)
	}

	cached, err := e.cache.GetMeasurementCache(ctx, frameArtifactID, contentHash)
	if err == nil && cached != nil {
		return cacheRecordToResult(cached, framePath, fileSize), nil
	}

	file, err := os.Open(framePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileHash, err := computeFileHash(file, fileInfo.Size())
	if err != nil {
		fileHash = "error_hashing_file"
	}

	_, _, err = image.DecodeConfig(file)
	if err != nil {
		result := &quality.FrameMeasurementResult{
			Decodable: false,
			FileSize:  fileSize,
			MimeType:  mimeType,
			FileHash:  fileHash,
			PixelHash: "",
		}
		e.createCache(ctx, frameArtifactID, contentHash, result)
		return result, nil
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	img, _, err := image.Decode(file)
	if err != nil {
		result := &quality.FrameMeasurementResult{
			Decodable: false,
			FileSize:  fileSize,
			MimeType:  mimeType,
			FileHash:  fileHash,
			PixelHash: "",
		}
		e.createCache(ctx, frameArtifactID, contentHash, result)
		return result, nil
	}

	realWidth := img.Bounds().Dx()
	realHeight := img.Bounds().Dy()

	colorModel := img.ColorModel()
	hasAlphaChannel := colorModel == color.NRGBAModel || colorModel == color.RGBAModel

	var fullyTransparent, semiTransparent, opaque int64
	hasher := sha256.New()
	for y := 0; y < realHeight; y++ {
		for x := 0; x < realWidth; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			a8 := uint8(a >> 8)
			hasher.Write([]byte{r8, g8, b8, a8})
			if a8 == 0 {
				fullyTransparent++
			} else if a8 == 255 {
				opaque++
			} else {
				semiTransparent++
			}
		}
	}

	totalPixels := int64(realWidth * realHeight)
	var fullyTransparentRatio, semiTransparentRatio, opaqueRatio, alphaCoverage float64
	if totalPixels > 0 {
		fullyTransparentRatio = float64(fullyTransparent) / float64(totalPixels)
		semiTransparentRatio = float64(semiTransparent) / float64(totalPixels)
		opaqueRatio = float64(opaque) / float64(totalPixels)
		alphaCoverage = 1.0 - fullyTransparentRatio
	}

	pixelHash := hex.EncodeToString(hasher.Sum(nil))

	result := &quality.FrameMeasurementResult{
		Width:                 realWidth,
		Height:                realHeight,
		HasAlphaChannel:       hasAlphaChannel,
		AlphaCoverage:         alphaCoverage,
		FullyTransparentRatio: fullyTransparentRatio,
		SemiTransparentRatio:  semiTransparentRatio,
		OpaqueRatio:           opaqueRatio,
		Decodable:             true,
		MimeType:              mimeType,
		PixelHash:             pixelHash,
		FileSize:              fileSize,
		FileHash:              fileHash,
	}

	e.createCache(ctx, frameArtifactID, contentHash, result)
	return result, nil
}

func computeFileHash(file io.Reader, fileSize int64) (string, error) {
	h := sha256.New()
	toRead := fileSize
	if toRead > MaxFileBytesToHash {
		toRead = MaxFileBytesToHash
	}
	if _, err := io.CopyN(h, file, toRead); err != nil && err != io.EOF {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func detectMIMEType(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, MagicBytesToRead)
	if _, err := io.ReadFull(file, buf); err != nil {
		return "", err
	}

	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) {
	}

	if len(buf) >= 3 && buf[0] == 0x89 && buf[1] == 0x50 && buf[2] == 0x4E {
		return "image/png", nil
	}
	if len(buf) >= 3 && buf[0] == 0xFF && buf[1] == 0xD8 && buf[2] == 0xFF {
		return "image/jpeg", nil
	}
	if len(buf) >= 4 && string(buf[:4]) == "GIF8" {
		return "image/gif", nil
	}
	if len(buf) >= 2 && buf[0] == 0x42 && buf[1] == 0x4D {
		return "image/bmp", nil
	}
	if len(buf) >= 4 && string(buf[:4]) == "RIFF" && len(buf) >= 12 && string(buf[8:12]) == "WEBP" {
		return "image/webp", nil
	}

	return "", fmt.Errorf("unknown mime type for file")
}

func inferMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

func cacheRecordToResult(cached *quality.QualityMeasurementCacheRecord, framePath string, fileSize int64) *quality.FrameMeasurementResult {
	var realFileSize int64
	if fi, err := os.Stat(framePath); err == nil {
		realFileSize = fi.Size()
	} else {
		realFileSize = fileSize
	}

	fileHash := ""
	if realFileSize > 0 {
		if file, err := os.Open(framePath); err == nil {
			if fh, fhErr := computeFileHash(file, realFileSize); fhErr == nil {
				fileHash = fh
			}
			file.Close()
		}
	}

	return &quality.FrameMeasurementResult{
		Width:                 cached.Width,
		Height:                cached.Height,
		HasAlphaChannel:       cached.HasAlphaChannel,
		AlphaCoverage:         cached.AlphaCoverage,
		FullyTransparentRatio: cached.FullyTransparentRatio,
		SemiTransparentRatio:  cached.SemiTransparentRatio,
		OpaqueRatio:           cached.OpaqueRatio,
		Decodable:             cached.Decodable,
		MimeType:              cached.MimeType,
		PixelHash:             cached.PixelHash,
		FileSize:              realFileSize,
		FileHash:              fileHash,
	}
}

func (e *ImageMeasurementEngineImpl) createCache(ctx context.Context, frameArtifactID string, contentHash string, result *quality.FrameMeasurementResult) {
	if result == nil {
		return
	}
	record := &quality.QualityMeasurementCacheRecord{
		FrameArtifactID:       frameArtifactID,
		ContentHash:           contentHash,
		Width:                 result.Width,
		Height:                result.Height,
		HasAlphaChannel:       result.HasAlphaChannel,
		AlphaCoverage:         result.AlphaCoverage,
		FullyTransparentRatio: result.FullyTransparentRatio,
		SemiTransparentRatio:  result.SemiTransparentRatio,
		OpaqueRatio:           result.OpaqueRatio,
		Decodable:             result.Decodable,
		MimeType:              result.MimeType,
		PixelHash:             result.PixelHash,
	}
	_ = e.cache.CreateMeasurementCache(ctx, record)
}
