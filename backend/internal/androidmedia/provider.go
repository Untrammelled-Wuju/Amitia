package androidmedia

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/androidmedia/camera"
	"github.com/u-ai/backend/internal/androidmedia/ffmpeg"
	"github.com/u-ai/backend/internal/androidmedia/mediaread"
	"github.com/u-ai/backend/internal/androidmedia/screenframe"
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

type ScreenFrameProvider interface {
	Start(ctx context.Context, owner screenframe.SessionOwner, request screenframe.StartRequest) (screenframe.StartResult, error)
	Latest(ctx context.Context, owner screenframe.SessionOwner, request screenframe.LatestRequest) (screenframe.LatestResult, error)
	Stop(ctx context.Context, owner screenframe.SessionOwner, sessionID screenframe.ScreenFrameSessionID) (screenframe.StopResult, error)
	Status(ctx context.Context, owner screenframe.SessionOwner) (screenframe.StatusResult, error)
}

type blockedScreenFrameProvider struct{}

func NewBlockedScreenFrameProvider() ScreenFrameProvider {
	return &blockedScreenFrameProvider{}
}

func (b *blockedScreenFrameProvider) Start(ctx context.Context, owner screenframe.SessionOwner, request screenframe.StartRequest) (screenframe.StartResult, error) {
	return screenframe.StartResult{}, screenframe.NewFrameError(screenframe.ErrBlockedNativeHost, "android native host source not available; screen frame capture blocked")
}

func (b *blockedScreenFrameProvider) Latest(ctx context.Context, owner screenframe.SessionOwner, request screenframe.LatestRequest) (screenframe.LatestResult, error) {
	return screenframe.LatestResult{}, screenframe.NewFrameError(screenframe.ErrBlockedNativeHost, "android native host source not available; screen frame latest blocked")
}

func (b *blockedScreenFrameProvider) Stop(ctx context.Context, owner screenframe.SessionOwner, sessionID screenframe.ScreenFrameSessionID) (screenframe.StopResult, error) {
	return screenframe.StopResult{}, screenframe.NewFrameError(screenframe.ErrBlockedNativeHost, "android native host source not available; screen frame stop blocked")
}

func (b *blockedScreenFrameProvider) Status(ctx context.Context, owner screenframe.SessionOwner) (screenframe.StatusResult, error) {
	return screenframe.StatusResult{
		Supported:          false,
		PermissionState:    "native_host_unavailable",
		ActiveSession:      false,
		UserActionRequired: true,
		State:              "native_host_missing",
	}, nil
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

type CameraCapabilityConstants struct {
	StatusHandlerID  string
	ListHandlerID    string
	CaptureHandlerID string
	PermissionID     string
}

func BuildCameraCapabilityConstants() CameraCapabilityConstants {
	return CameraCapabilityConstants{
		StatusHandlerID:  ToolIDCameraStatus,
		ListHandlerID:    ToolIDCameraList,
		CaptureHandlerID: ToolIDCameraCapture,
		PermissionID:     PermissionCamera,
	}
}

func BuildCameraTools() ([]capability.ToolDefinition, error) {
	status, err := camera.BuildStatusToolDefinition()
	if err != nil {
		return nil, err
	}
	list, err := camera.BuildListToolDefinition()
	if err != nil {
		return nil, err
	}
	capture, err := camera.BuildCaptureToolDefinition()
	if err != nil {
		return nil, err
	}
	return []capability.ToolDefinition{status, list, capture}, nil
}

func BuildCameraPermissionDefinitions() []camera.PermissionDefinition {
	return []camera.PermissionDefinition{
		camera.BuildPermissionDefinition(),
	}
}

type MediaReadCapabilityConstants struct {
	InfoHandlerID  string
	ImageHandlerID string
	PermissionID   string
}

type MediaReadProvider interface {
	Info(ctx context.Context, uri string) (mediaread.ImageInfo, error)
	Image(ctx context.Context, uri string, opts mediaread.DecodeOptions) (mediaread.NormalizedImage, error)
}

type blockedMediaReadProvider struct{}

func NewBlockedMediaReadProvider() MediaReadProvider {
	return &blockedMediaReadProvider{}
}

func (b *blockedMediaReadProvider) Info(ctx context.Context, uri string) (mediaread.ImageInfo, error) {
	return mediaread.ImageInfo{}, &MediaReadError{
		Code:    BLOCKED_ANDROID_NATIVE_HOST_SOURCE,
		Message: "android native host source not available; media read blocked",
	}
}

func (b *blockedMediaReadProvider) Image(ctx context.Context, uri string, opts mediaread.DecodeOptions) (mediaread.NormalizedImage, error) {
	return mediaread.NormalizedImage{}, &MediaReadError{
		Code:    BLOCKED_ANDROID_NATIVE_HOST_SOURCE,
		Message: "android native host source not available; media read blocked",
	}
}

type MediaReadError struct {
	Code    string
	Message string
}

func (e *MediaReadError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

func BuildMediaReadCapabilityConstants() MediaReadCapabilityConstants {
	return MediaReadCapabilityConstants{
		InfoHandlerID:  ToolIDMediaReadInfo,
		ImageHandlerID: ToolIDMediaReadImage,
		PermissionID:   PermissionMediaRead,
	}
}

func BuildMediaReadTools() ([]capability.ToolDefinition, error) {
	infoTool, err := mediaread.BuildInfoToolDefinition()
	if err != nil {
		return nil, err
	}
	imageTool, err := mediaread.BuildImageToolDefinition()
	if err != nil {
		return nil, err
	}
	return []capability.ToolDefinition{infoTool, imageTool}, nil
}

func BuildMediaReadPermissionDefinitions() []mediaread.PermissionDefinition {
	return []mediaread.PermissionDefinition{
		mediaread.BuildPermissionDefinition(),
	}
}

type FFmpegCapabilityConstants struct {
	PermissionID string
}

func BuildFFmpegCapabilityConstants() FFmpegCapabilityConstants {
	return FFmpegCapabilityConstants{
		PermissionID: PermissionFFmpeg,
	}
}

func NewDefaultFFmpegProvider() ffmpeg.FFmpegProvider {
	return ffmpeg.NewBlockedFFmpegProvider()
}
