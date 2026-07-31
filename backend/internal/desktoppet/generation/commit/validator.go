package commit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

const defaultMaxPixels int64 = 64 * 1024 * 1024

type ArtifactValidator struct {
	maxPixels int64
}

func NewArtifactValidator() *ArtifactValidator {
	return &ArtifactValidator{maxPixels: defaultMaxPixels}
}

func NewArtifactValidatorWithMaxPixels(maxPixels int64) *ArtifactValidator {
	if maxPixels <= 0 {
		maxPixels = defaultMaxPixels
	}
	return &ArtifactValidator{maxPixels: maxPixels}
}

func (v *ArtifactValidator) ValidateArtifact(data []byte, mimeType string, width int, height int, expectedHash string, maxPixels int64) error {
	_, _, _, err := v.validateInternal(data, mimeType, width, height, expectedHash, maxPixels, true)
	return err
}

func (v *ArtifactValidator) ValidateAndMeasure(data []byte, mimeType string, expectedHash string, maxPixels int64) (int, int, string, error) {
	return v.validateInternal(data, mimeType, 0, 0, expectedHash, maxPixels, false)
}

func (v *ArtifactValidator) validateInternal(data []byte, mimeType string, width int, height int, expectedHash string, maxPixels int64, checkDimensions bool) (int, int, string, error) {
	if len(data) == 0 {
		return 0, 0, "", fmt.Errorf("artifact data is empty")
	}
	if maxPixels <= 0 {
		maxPixels = v.maxPixels
	}

	detectedMIME := http.DetectContentType(data)
	normalizedDetected := strings.TrimSpace(strings.Split(detectedMIME, ";")[0])
	normalizedExpected := strings.TrimSpace(strings.Split(mimeType, ";")[0])
	if normalizedExpected != "" && normalizedExpected != normalizedDetected {
		return 0, 0, "", fmt.Errorf("mime mismatch: expected %s, detected %s", normalizedExpected, normalizedDetected)
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, "", fmt.Errorf("decode artifact failed: %w", err)
	}

	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()
	if imgWidth <= 0 || imgHeight <= 0 {
		return 0, 0, "", fmt.Errorf("invalid image dimensions %dx%d", imgWidth, imgHeight)
	}

	if checkDimensions {
		if width > 0 && imgWidth != width {
			return 0, 0, "", fmt.Errorf("width mismatch: expected %d, actual %d", width, imgWidth)
		}
		if height > 0 && imgHeight != height {
			return 0, 0, "", fmt.Errorf("height mismatch: expected %d, actual %d", height, imgHeight)
		}
	}

	pixelCount := int64(imgWidth) * int64(imgHeight)
	if pixelCount > maxPixels {
		return 0, 0, "", fmt.Errorf("pixel count %d exceeds max %d", pixelCount, maxPixels)
	}

	sum := sha256.Sum256(data)
	actualHash := hex.EncodeToString(sum[:])
	if expectedHash != "" && !strings.EqualFold(expectedHash, actualHash) {
		return 0, 0, "", fmt.Errorf("hash mismatch: expected %s, actual %s", expectedHash, actualHash)
	}

	if err := checkNotFullyTransparent(img, bounds); err != nil {
		return 0, 0, "", err
	}
	if err := checkNotBlank(img, bounds); err != nil {
		return 0, 0, "", err
	}

	return imgWidth, imgHeight, format, nil
}

func checkNotFullyTransparent(img image.Image, bounds image.Rectangle) error {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0 {
				return nil
			}
		}
	}
	return fmt.Errorf("artifact image is fully transparent")
}

func checkNotBlank(img image.Image, bounds image.Rectangle) error {
	first := color.NRGBAModel.Convert(img.At(bounds.Min.X, bounds.Min.Y)).(color.NRGBA)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c != first {
				return nil
			}
		}
	}
	return fmt.Errorf("artifact image is blank: all pixels identical")
}
