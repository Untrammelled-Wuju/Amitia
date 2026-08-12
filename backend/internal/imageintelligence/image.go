package imageintelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/pkg/resourceuri"
)

type ImageInput struct {
	ResourceURI string `json:"resourceUri"`
}

type ImageInputSummary struct {
	MIME     string `json:"mime"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int64  `json:"bytes"`
	Provider string `json:"provider,omitempty"`
}

type ImageResourceResolver struct {
	physicalResolver *resourceuri.PhysicalResolver
}

func NewImageResourceResolver(physicalResolver *resourceuri.PhysicalResolver) *ImageResourceResolver {
	return &ImageResourceResolver{physicalResolver: physicalResolver}
}

func (r *ImageResourceResolver) ResolveAndValidate(ctx context.Context, input ImageInput, caps ImageCapabilities) ([]byte, ImageInputSummary, *Error) {
	uri, err := resourceuri.Parse(input.ResourceURI)
	if err != nil {
		return nil, ImageInputSummary{}, &Error{Code: ErrResourceNotFound, Message: fmt.Sprintf("invalid resource URI: %v", err), HTTPStatus: http.StatusNotFound}
	}

	if uri.Root() == resourceuri.ResourceRootNative {
		return nil, ImageInputSummary{}, &Error{Code: ErrResourceNotFound, Message: "native resources are not supported as image input", HTTPStatus: http.StatusBadRequest}
	}

	resolved, err := r.physicalResolver.Resolve(uri)
	if err != nil {
		return nil, ImageInputSummary{}, &Error{Code: ErrResourceNotFound, Message: fmt.Sprintf("resource not found: %v", err), HTTPStatus: http.StatusNotFound}
	}

	localPath := resolved.LocalPath
	if localPath == "" {
		return nil, ImageInputSummary{}, &Error{Code: ErrResourceNotFound, Message: "resolved resource has no local path", HTTPStatus: http.StatusNotFound}
	}

	if strings.Contains(input.ResourceURI, "..") {
		return nil, ImageInputSummary{}, &Error{Code: ErrResourceDenied, Message: "path traversal detected", HTTPStatus: http.StatusForbidden}
	}

	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ImageInputSummary{}, &Error{Code: ErrResourceNotFound, Message: "image file not found", HTTPStatus: http.StatusNotFound}
		}
		return nil, ImageInputSummary{}, &Error{Code: ErrResourceDenied, Message: fmt.Sprintf("cannot access image: %v", err), HTTPStatus: http.StatusForbidden}
	}

	fileSize := info.Size()
	if fileSize > caps.MaxInputBytes {
		return nil, ImageInputSummary{}, &Error{Code: ErrTooLarge, Message: fmt.Sprintf("image file size %d exceeds limit %d", fileSize, caps.MaxInputBytes), HTTPStatus: http.StatusRequestEntityTooLarge}
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, ImageInputSummary{}, &Error{Code: ErrResourceDenied, Message: fmt.Sprintf("cannot read image: %v", err), HTTPStatus: http.StatusForbidden}
	}

	mimeType := detectMIME(data)
	if !isSupportedMIME(mimeType, caps.SupportedInputMIMEs) {
		return nil, ImageInputSummary{}, &Error{Code: ErrFormatUnsupported, Message: fmt.Sprintf("unsupported image format: %s", mimeType), HTTPStatus: http.StatusUnsupportedMediaType}
	}

	config, formatName, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ImageInputSummary{}, &Error{Code: ErrDecodeFailed, Message: fmt.Sprintf("image decode failed: %v", err), HTTPStatus: http.StatusUnprocessableEntity}
	}
	_ = formatName

	if config.Width > caps.MaxWidth || config.Height > caps.MaxHeight {
		return nil, ImageInputSummary{}, &Error{Code: ErrDimensionsTooLarge, Message: fmt.Sprintf("image dimensions %dx%d exceed limit %dx%d", config.Width, config.Height, caps.MaxWidth, caps.MaxHeight), HTTPStatus: http.StatusRequestEntityTooLarge}
	}

	pixels := int64(config.Width) * int64(config.Height)
	if pixels > caps.MaxPixels {
		return nil, ImageInputSummary{}, &Error{Code: ErrTooLarge, Message: fmt.Sprintf("image pixel count %d exceeds limit %d", pixels, caps.MaxPixels), HTTPStatus: http.StatusRequestEntityTooLarge}
	}

	summary := ImageInputSummary{
		MIME:  mimeType,
		Width: config.Width,
		Height: config.Height,
		Bytes: fileSize,
	}

	return data, summary, nil
}

func detectMIME(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}
	mimeType := http.DetectContentType(data)
	if mimeType == "application/octet-stream" {
		if isPNG(data) {
			return "image/png"
		}
		if isJPEG(data) {
			return "image/jpeg"
		}
		if isWebP(data) {
			return "image/webp"
		}
	}
	return mimeType
}

func isPNG(data []byte) bool {
	return len(data) > 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
}

func isJPEG(data []byte) bool {
	return len(data) > 2 && data[0] == 0xFF && data[1] == 0xD8
}

func isWebP(data []byte) bool {
	return len(data) > 12 && bytes.Equal(data[:4], []byte{'R', 'I', 'F', 'F'}) && bytes.Equal(data[8:12], []byte{'W', 'E', 'B', 'P'})
}

func isSupportedMIME(mime string, supported []string) bool {
	for _, s := range supported {
		if mime == s {
			return true
		}
	}
	return false
}

func encodeBase64(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var buf bytes.Buffer
	buf.Grow((len(data)+2)/3 * 4)
	for i := 0; i < len(data); i += 3 {
		var n int
		var b1, b2, b3 byte
		b1 = data[i]
		if i+1 < len(data) {
			b2 = data[i+1]
			n++
		}
		if i+2 < len(data) {
			b3 = data[i+2]
			n++
		}
		buf.WriteByte(alphabet[(b1>>2)&0x3F])
		buf.WriteByte(alphabet[((b1<<4)|(b2>>4))&0x3F])
		if n > 1 {
			buf.WriteByte(alphabet[((b2<<2)|(b3>>6))&0x3F])
		} else {
			buf.WriteByte('=')
		}
		if n > 2 {
			buf.WriteByte(alphabet[b3&0x3F])
		} else {
			buf.WriteByte('=')
		}
	}
	return buf.String()
}

func buildDataURI(mime string, data []byte) string {
	return "data:" + mime + ";base64," + encodeBase64(data)
}

func validateImageBytes(data []byte, caps ImageCapabilities) *Error {
	mime := detectMIME(data)
	if !isSupportedMIME(mime, caps.SupportedInputMIMEs) {
		return &Error{Code: ErrFormatUnsupported, Message: fmt.Sprintf("unsupported image format: %s", mime), HTTPStatus: http.StatusUnsupportedMediaType}
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return &Error{Code: ErrDecodeFailed, Message: fmt.Sprintf("image decode failed: %v", err), HTTPStatus: http.StatusUnprocessableEntity}
	}
	if config.Width > caps.MaxWidth || config.Height > caps.MaxHeight {
		return &Error{Code: ErrDimensionsTooLarge, Message: fmt.Sprintf("image dimensions %dx%d exceed limit %dx%d", config.Width, config.Height, caps.MaxWidth, caps.MaxHeight), HTTPStatus: http.StatusRequestEntityTooLarge}
	}
	pixels := int64(config.Width) * int64(config.Height)
	if pixels > caps.MaxPixels {
		return &Error{Code: ErrTooLarge, Message: fmt.Sprintf("image pixel count %d exceeds limit %d", pixels, caps.MaxPixels), HTTPStatus: http.StatusRequestEntityTooLarge}
	}
	return nil
}

func readAndValidateImage(localPath string, caps ImageCapabilities) ([]byte, *Error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, &Error{Code: ErrResourceDenied, Message: fmt.Sprintf("cannot read image: %v", err), HTTPStatus: http.StatusForbidden}
	}
	verr := validateImageBytes(data, caps)
	if verr != nil {
		return nil, verr
	}
	return data, nil
}

func copyWithLimit(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	limited := io.LimitReader(src, limit+1)
	n, err := io.Copy(dst, limited)
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("content exceeds limit %d", limit)
	}
	return n, nil
}

func sanitizePath(baseDir, userInput string) (string, error) {
	clean := filepath.Clean(filepath.Join(baseDir, userInput))
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absClean, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	if absClean != absBase && !strings.HasPrefix(absClean, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal detected")
	}
	return absClean, nil
}

func marshalJSONSafe(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
