package mediaread

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	_ "golang.org/x/image/webp"
)

type ImageNormalizer struct {
	policy Policy
	decoder *ImageDecoder
}

func NewImageNormalizer(policy Policy, decoder *ImageDecoder) *ImageNormalizer {
	return &ImageNormalizer{
		policy:  policy,
		decoder: decoder,
	}
}

func (n *ImageNormalizer) NeedsNormalization(info ImageInfo) bool {
	if n.policy.NormalizeOrientation && info.Orientation != 0 {
		return true
	}
	if n.policy.StripSensitiveMetadata {
		return true
	}
	return false
}

func (n *ImageNormalizer) DecodeFull(ctx context.Context, reader io.Reader, info ImageInfo) (image.Image, error) {
	select {
	case <-ctx.Done():
		return nil, &MediaReadError{Code: MediaReadCancelled, Message: "context cancelled"}
	default:
	}

	if err := safeDecodeMaxPixels(int64(info.Width), int64(info.Height), n.policy.MaxPixels); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, &MediaReadError{Code: MediaReadDecodeFailed, Message: fmt.Sprintf("failed to read image bytes: %v", err)}
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, &MediaReadError{Code: MediaReadDecodeFailed, Message: fmt.Sprintf("failed to decode image: %v", err)}
	}

	return img, nil
}

func (n *ImageNormalizer) EncodeToTemp(ctx context.Context, img image.Image, format string, destPath string) (int64, error) {
	select {
	case <-ctx.Done():
		return 0, &MediaReadError{Code: MediaReadCancelled, Message: "context cancelled"}
	default:
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, &MediaReadError{Code: MediaReadArtifactFailed, Message: fmt.Sprintf("cannot create output dir: %v", err)}
	}

	tmpPath := destPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, &MediaReadError{Code: MediaReadArtifactFailed, Message: fmt.Sprintf("cannot create temp file: %v", err)}
	}

	var encodeErr error
	switch format {
	case FormatJPEG:
		encodeErr = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	case FormatPNG:
		encodeErr = png.Encode(f, img)
	default:
		encodeErr = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	}

	if encodeErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return 0, &MediaReadError{Code: MediaReadNormalizeFailed, Message: fmt.Sprintf("failed to encode image: %v", encodeErr)}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return 0, &MediaReadError{Code: MediaReadNormalizeFailed, Message: fmt.Sprintf("failed to close temp file: %v", err)}
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return 0, &MediaReadError{Code: MediaReadNormalizeFailed, Message: fmt.Sprintf("failed to stat temp file: %v", err)}
	}

	if info.Size() > n.policy.MaxNormalizedBytes {
		os.Remove(tmpPath)
		return 0, &MediaReadError{
			Code:    MediaReadTooLarge,
			Message: fmt.Sprintf("normalized image size %d exceeds limit %d", info.Size(), n.policy.MaxNormalizedBytes),
		}
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return 0, &MediaReadError{Code: MediaReadArtifactFailed, Message: fmt.Sprintf("failed to finalize artifact: %v", err)}
	}

	return info.Size(), nil
}

func (n *ImageNormalizer) ResourceURIFromTemp(requestID, format string) string {
	ext := FormatToExt(format)
	return "amitia://temp/android-media/mediaread/" + requestID + ext
}
