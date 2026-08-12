package mediaread

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"
)

const (
	decodeMaxReadBytes = 1<<31 - 2
)

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, &MediaReadError{Code: MediaReadTooLarge, Message: "read limit must be positive"}
	}
	lr := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &MediaReadError{Code: MediaReadTooLarge, Message: "resource exceeds read limit"}
	}
	return data, nil
}

type ImageDecoder struct {
	policy Policy
}

func NewImageDecoder(policy Policy) *ImageDecoder {
	return &ImageDecoder{policy: policy}
}

func (d *ImageDecoder) Inspect(ctx context.Context, reader io.Reader, declaredMIME string, fileSize int64) (ImageInfo, error) {
	select {
	case <-ctx.Done():
		return ImageInfo{}, &MediaReadError{Code: MediaReadCancelled, Message: "context cancelled"}
	default:
	}

	limit := d.policy.MaxInputBytes
	if fileSize > 0 && fileSize < limit {
		limit = fileSize
	}

	data, err := readLimited(reader, limit)
	if err != nil {
		return ImageInfo{}, err
	}

	mimeType := DetectMIMEFromBytes(data)
	if mimeType == "application/octet-stream" {
		mimeType = declaredMIME
	}
	if mimeType == "" {
		mimeType = DeclaredMIME(declaredMIME)
	}

	format := MIMEToFormat(mimeType)
	if format == "" {
		return ImageInfo{}, &MediaReadError{Code: MediaReadUnsupportedFormat, Message: "unsupported image format: " + mimeType}
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageInfo{}, &MediaReadError{Code: MediaReadDecodeFailed, Message: fmt.Sprintf("failed to decode image config: %v", err)}
	}

	pixels := int64(config.Width) * int64(config.Height)
	if pixels > d.policy.MaxPixels {
		return ImageInfo{}, &MediaReadError{
			Code:    MediaReadTooLarge,
			Message: fmt.Sprintf("image pixel count %d exceeds limit %d", pixels, d.policy.MaxPixels),
		}
	}
	if config.Width > d.policy.MaxWidth || config.Height > d.policy.MaxHeight {
		return ImageInfo{}, &MediaReadError{
			Code:    MediaReadTooLarge,
			Message: fmt.Sprintf("image dimensions %dx%d exceed limit %dx%d", config.Width, config.Height, d.policy.MaxWidth, d.policy.MaxHeight),
		}
	}

	info := ImageInfo{
		MIMEType:  mimeType,
		Format:    format,
		SizeBytes: fileSize,
		Width:     config.Width,
		Height:    config.Height,
		HasAlpha:  hasAlphaChannel(data),
		Animated:  isAnimatedWebP(data) || isAnimatedGIFConfig(config),
	}

	return info, nil
}

func DeclaredMIME(mime string) string {
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/bmp", "image/heic", "image/heif":
		return mime
	}
	return "application/octet-stream"
}

func hasAlphaChannel(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if isPNGSignature(data) {
		return true
	}
	if isWebPSignature(data) {
		return true
	}
	return false
}

func isAnimatedGIF(data []byte) bool {
	return false
}

func isAnimatedGIFConfig(config image.Config) bool {
	return false
}

func isAnimatedWebP(data []byte) bool {
	if len(data) < 20 {
		return false
	}
	if !bytes.Equal(data[:4], []byte{'R', 'I', 'F', 'F'}) {
		return false
	}
	return bytes.Contains(data[:20], []byte("ANIM")) || bytes.Contains(data[:20], []byte("ANMF"))
}

func safeDecodeMaxPixels(width, height, maxPixels int64) error {
	pixels := int64(width) * int64(height)
	if pixels > maxPixels {
		return &MediaReadError{
			Code:    MediaReadTooLarge,
			Message: fmt.Sprintf("decoded image pixels %d exceed limit %d", pixels, maxPixels),
		}
	}
	estimated := pixels * 4
	if estimated < 0 {
		return &MediaReadError{Code: MediaReadTooLarge, Message: "image buffer size overflow"}
	}
	return nil
}
