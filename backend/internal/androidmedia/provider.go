package androidmedia

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ScreenshotFormat string

const (
	ScreenshotFormatPNG  ScreenshotFormat = "png"
	ScreenshotFormatJPEG ScreenshotFormat = "jpeg"
	ScreenshotFormatWebP ScreenshotFormat = "webp"
)

var SupportedScreenshotFormats = []ScreenshotFormat{
	ScreenshotFormatPNG,
	ScreenshotFormatJPEG,
	ScreenshotFormatWebP,
}

type ScreenshotCaptureRequest struct {
	DisplayID *int              `json:"displayId,omitempty"`
	Format    *ScreenshotFormat `json:"format,omitempty"`
	Quality   *int              `json:"quality,omitempty"`
	MaxWidth  *int              `json:"maxWidth,omitempty"`
	MaxHeight *int              `json:"maxHeight,omitempty"`
}

type ScreenshotCaptureResult struct {
	ResourceURI string `json:"resourceUri"`
	MIMEType    string `json:"mimeType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	DisplayID   int    `json:"displayId"`
	TimestampMs int64  `json:"timestampMs"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentHash string `json:"contentHash,omitempty"`
}

type ScreenshotCapabilityState struct {
	Supported                bool     `json:"supported"`
	CaptureBackend           string   `json:"captureBackend"`
	AccessibilityEnabled     bool     `json:"accessibilityEnabled"`
	AccessibilityConnected   bool     `json:"accessibilityConnected"`
	CanTakeScreenshot        bool     `json:"canTakeScreenshot"`
	SupportsMultipleDisplays bool     `json:"supportsMultipleDisplays"`
	SupportedFormats         []string `json:"supportedFormats"`
	MaxPixels                int64    `json:"maxPixels"`
	Reason                   string   `json:"reason"`
}

type ScreenshotConfig struct {
	MaxScreenshotPixels int64
	MaxEncodedBytes     int64
	DefaultFormat       ScreenshotFormat
	DefaultJPEGQuality  int
	ArtifactTTL         time.Duration
}

func DefaultScreenshotConfig() ScreenshotConfig {
	return ScreenshotConfig{
		MaxScreenshotPixels: DefaultMaxScreenshotPixels,
		MaxEncodedBytes:     DefaultMaxEncodedBytes,
		DefaultFormat:       ScreenshotFormatPNG,
		DefaultJPEGQuality:  DefaultJPEGQuality,
		ArtifactTTL:         30 * time.Minute,
	}
}

type ScreenshotProvider interface {
	Capture(ctx context.Context, request ScreenshotCaptureRequest) (ScreenshotCaptureResult, error)
	CapabilityState(ctx context.Context) ScreenshotCapabilityState
}

type NativeScreenshotProvider interface {
	ScreenshotProvider
	NativeOperation(ctx context.Context, operation string, payload map[string]any) (map[string]any, error)
}

type blockedScreenshotProvider struct {
	mu     sync.RWMutex
	config ScreenshotConfig
}

func NewBlockedScreenshotProvider() ScreenshotProvider {
	return &blockedScreenshotProvider{config: DefaultScreenshotConfig()}
}

func (b *blockedScreenshotProvider) Capture(ctx context.Context, request ScreenshotCaptureRequest) (ScreenshotCaptureResult, error) {
	return ScreenshotCaptureResult{}, &ScreenshotError{
		Code:    BLOCKED_ANDROID_NATIVE_HOST_SOURCE,
		Message: "android native host source not available; screenshot capture blocked",
	}
}

func (b *blockedScreenshotProvider) CapabilityState(ctx context.Context) ScreenshotCapabilityState {
	return ScreenshotCapabilityState{
		Supported: false,
		MaxPixels: b.config.MaxScreenshotPixels,
		Reason:    BLOCKED_ANDROID_NATIVE_HOST_SOURCE,
	}
}

type ScreenshotError struct {
	Code    string
	Message string
}

func (e *ScreenshotError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

type ScreenshotArtifactRecord struct {
	ResourceURI      string    `json:"resourceUri"`
	MIMEType         string    `json:"mimeType"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	SizeBytes        int64     `json:"sizeBytes"`
	ContentHash      string    `json:"contentHash,omitempty"`
	DisplayID        int       `json:"displayId"`
	CaptureTimestamp time.Time `json:"captureTimestamp"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type ScreenshotCapabilityConstants struct {
	CapabilityID capability.CapabilityID
	PermissionID string
}

func BuildScreenshotCapabilityConstants() ScreenshotCapabilityConstants {
	return ScreenshotCapabilityConstants{
		CapabilityID: capability.CapabilityID(
			capability.BuildCapabilityID(
				capability.CapabilitySourceBuiltin,
				"android",
				"screen.screenshot",
			),
		),
		PermissionID: PermissionScreenCapture,
	}
}
