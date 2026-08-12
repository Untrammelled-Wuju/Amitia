package screenshot

import (
	"encoding/json"
	"fmt"
)

type ScreenshotFormat string

const (
	FormatPNG  ScreenshotFormat = "png"
	FormatJPEG ScreenshotFormat = "jpeg"
	FormatWebP ScreenshotFormat = "webp"
)

var validFormats = map[ScreenshotFormat]bool{
	FormatPNG:  true,
	FormatJPEG: true,
	FormatWebP: true,
}

var formatToMIME = map[ScreenshotFormat]string{
	FormatPNG:  "image/png",
	FormatJPEG: "image/jpeg",
	FormatWebP: "image/webp",
}

var formatToExt = map[ScreenshotFormat]string{
	FormatPNG:  ".png",
	FormatJPEG: ".jpg",
	FormatWebP: ".webp",
}

func (f ScreenshotFormat) MIME() string {
	if mime, ok := formatToMIME[f]; ok {
		return mime
	}
	return "application/octet-stream"
}

func (f ScreenshotFormat) Ext() string {
	if ext, ok := formatToExt[f]; ok {
		return ext
	}
	return ".bin"
}

func (f ScreenshotFormat) IsValid() bool {
	return validFormats[f]
}

type CaptureRequest struct {
	DisplayID *int             `json:"displayId,omitempty"`
	Format    *ScreenshotFormat `json:"format,omitempty"`
	Quality   *int             `json:"quality,omitempty"`
	MaxWidth  *int             `json:"maxWidth,omitempty"`
	MaxHeight *int             `json:"maxHeight,omitempty"`
}

func (r CaptureRequest) Validate() error {
	if r.Format != nil && !r.Format.IsValid() {
		return fmt.Errorf("unsupported screenshot format: %s", *r.Format)
	}
	if r.Quality != nil {
		q := *r.Quality
		if q < 1 || q > 100 {
			return fmt.Errorf("quality must be between 1 and 100, got %d", q)
		}
	}
	if r.MaxWidth != nil && *r.MaxWidth <= 0 {
		return fmt.Errorf("maxWidth must be positive")
	}
	if r.MaxHeight != nil && *r.MaxHeight <= 0 {
		return fmt.Errorf("maxHeight must be positive")
	}
	return nil
}

func (r CaptureRequest) ResolveFormat() ScreenshotFormat {
	if r.Format != nil && r.Format.IsValid() {
		return *r.Format
	}
	return FormatPNG
}

func ParseCaptureRequest(raw json.RawMessage) (CaptureRequest, error) {
	var req CaptureRequest
	if len(raw) == 0 {
		return req, nil
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return CaptureRequest{}, fmt.Errorf("invalid screenshot request: %w", err)
	}
	return req, nil
}
