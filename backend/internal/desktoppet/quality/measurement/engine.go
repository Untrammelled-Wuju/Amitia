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

type ImageMeasurementEngineImpl struct {
	cache quality.QualityRepository
}

func NewImageMeasurementEngine(cache quality.QualityRepository) *ImageMeasurementEngineImpl {
	return &ImageMeasurementEngineImpl{cache: cache}
}

func (e *ImageMeasurementEngineImpl) MeasureFrame(ctx context.Context, framePath string, contentHash string, frameArtifactID string) (*quality.FrameMeasurementResult, error) {
	cached, err := e.cache.GetMeasurementCache(ctx, frameArtifactID, contentHash)
	if err == nil && cached != nil {
		return cacheRecordToResult(cached, framePath), nil
	}

	fileInfo, err := os.Stat(framePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("frame file not found: %s", framePath)
		}
		return nil, err
	}
	fileSize := fileInfo.Size()
	mimeType := inferMimeType(framePath)

	file, err := os.Open(framePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		result := &quality.FrameMeasurementResult{
			Decodable: false,
			FileSize:  fileSize,
			MimeType:  mimeType,
		}
		e.createCache(ctx, frameArtifactID, contentHash, result)
		return result, nil
	}

	hasAlphaChannel := config.ColorModel == color.RGBAModel || config.ColorModel == color.NRGBAModel

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	img, _, err := image.Decode(file)
	if err != nil {
		result := &quality.FrameMeasurementResult{
			Decodable: false,
			FileSize:  fileSize,
			MimeType:  mimeType,
		}
		e.createCache(ctx, frameArtifactID, contentHash, result)
		return result, nil
	}

	realWidth := img.Bounds().Dx()
	realHeight := img.Bounds().Dy()

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

	totalPixels := float64(realWidth * realHeight)
	var fullyTransparentRatio, semiTransparentRatio, opaqueRatio, alphaCoverage float64
	if totalPixels > 0 {
		fullyTransparentRatio = float64(fullyTransparent) / totalPixels
		semiTransparentRatio = float64(semiTransparent) / totalPixels
		opaqueRatio = float64(opaque) / totalPixels
		alphaCoverage = 1.0 - fullyTransparentRatio
	}

	pixelHash := hex.EncodeToString(hasher.Sum(nil))

	result := &quality.FrameMeasurementResult{
		Width:                  realWidth,
		Height:                 realHeight,
		HasAlphaChannel:        hasAlphaChannel,
		AlphaCoverage:          alphaCoverage,
		FullyTransparentRatio:  fullyTransparentRatio,
		SemiTransparentRatio:   semiTransparentRatio,
		OpaqueRatio:            opaqueRatio,
		Decodable:              true,
		MimeType:               mimeType,
		PixelHash:              pixelHash,
		FileSize:               fileSize,
	}

	e.createCache(ctx, frameArtifactID, contentHash, result)
	return result, nil
}

func (e *ImageMeasurementEngineImpl) createCache(ctx context.Context, frameArtifactID string, contentHash string, result *quality.FrameMeasurementResult) {
	if result == nil {
		return
	}
	record := &quality.QualityMeasurementCacheRecord{
		FrameArtifactID:        frameArtifactID,
		ContentHash:            contentHash,
		Width:                  result.Width,
		Height:                 result.Height,
		HasAlphaChannel:        result.HasAlphaChannel,
		AlphaCoverage:          result.AlphaCoverage,
		FullyTransparentRatio:  result.FullyTransparentRatio,
		SemiTransparentRatio:   result.SemiTransparentRatio,
		OpaqueRatio:            result.OpaqueRatio,
		Decodable:              result.Decodable,
		MimeType:               result.MimeType,
		PixelHash:              result.PixelHash,
	}
	_ = e.cache.CreateMeasurementCache(ctx, record)
}

func cacheRecordToResult(cached *quality.QualityMeasurementCacheRecord, framePath string) *quality.FrameMeasurementResult {
	var fileSize int64
	if fi, err := os.Stat(framePath); err == nil {
		fileSize = fi.Size()
	}
	return &quality.FrameMeasurementResult{
		Width:                  cached.Width,
		Height:                 cached.Height,
		HasAlphaChannel:        cached.HasAlphaChannel,
		AlphaCoverage:          cached.AlphaCoverage,
		FullyTransparentRatio:  cached.FullyTransparentRatio,
		SemiTransparentRatio:   cached.SemiTransparentRatio,
		OpaqueRatio:            cached.OpaqueRatio,
		Decodable:              cached.Decodable,
		MimeType:               cached.MimeType,
		PixelHash:              cached.PixelHash,
		FileSize:               fileSize,
	}
}

func inferMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}
