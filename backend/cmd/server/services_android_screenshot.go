//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/androidmedia"
	mediascreenshot "github.com/u-ai/backend/internal/androidmedia/screenshot"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/pkg/resourceuri"
)

const androidScreenCaptureOperation = "screen_capture.capture"

type mediaScreenshotHandler struct {
	bridge   nativebridge.Bridge
	resolver *resourceuri.PhysicalResolver
	dataDir  string
	config   androidmedia.ScreenshotConfig
}

func newMediaScreenshotHandler(
	bridge nativebridge.Bridge,
	resolver *resourceuri.PhysicalResolver,
	dataDir string,
) *mediaScreenshotHandler {
	return &mediaScreenshotHandler{
		bridge:   bridge,
		resolver: resolver,
		dataDir:  strings.TrimSpace(dataDir),
		config:   androidmedia.DefaultScreenshotConfig(),
	}
}

func (h *mediaScreenshotHandler) Execute(
	ctx context.Context,
	request capability.AndroidBridgeRequest,
) capability.AndroidBridgeResponse {
	failure := func(code, message string) capability.AndroidBridgeResponse {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    code,
				Message: message,
			},
		}
	}

	if h == nil || h.bridge == nil || h.resolver == nil || h.dataDir == "" {
		return failure(androidmedia.BLOCKED_ANDROID_NATIVE_HOST_SOURCE, "screen capture persistence is not configured")
	}
	if err := ctx.Err(); err != nil {
		return failure(androidmedia.SCREENSHOT_CANCELLED, err.Error())
	}

	rawRequest, err := json.Marshal(request.Payload)
	if err != nil {
		return failure("INVALID_INPUT", "encode screenshot request: "+err.Error())
	}
	captureRequest, err := mediascreenshot.ParseCaptureRequest(rawRequest)
	if err != nil {
		return failure("INVALID_INPUT", err.Error())
	}
	if err := captureRequest.Validate(); err != nil {
		return failure("INVALID_INPUT", err.Error())
	}

	format := captureRequest.ResolveFormat()
	payload := make(map[string]any, 5)
	payload["format"] = string(format)
	if captureRequest.DisplayID != nil {
		payload["displayId"] = *captureRequest.DisplayID
	} else {
		payload["displayId"] = 0
	}
	if captureRequest.Quality != nil {
		payload["quality"] = *captureRequest.Quality
	}
	if captureRequest.MaxWidth != nil {
		payload["maxWidth"] = *captureRequest.MaxWidth
	}
	if captureRequest.MaxHeight != nil {
		payload["maxHeight"] = *captureRequest.MaxHeight
	}

	bridgeResponse, err := h.bridge.Execute(ctx, nativebridge.Request{
		ProtocolVersion: nativebridge.AndroidBridgeProtocolVersion,
		RequestId:       request.RequestID,
		Platform:        "android",
		Operation:       androidScreenCaptureOperation,
		Payload:         payload,
	})
	if err != nil {
		return failure(androidmedia.SCREENSHOT_CAPTURE_FAILED, err.Error())
	}
	if bridgeResponse.Error != nil {
		code := strings.TrimSpace(bridgeResponse.Error.Code)
		if code == "" {
			code = androidmedia.SCREENSHOT_CAPTURE_FAILED
		}
		return failure(code, bridgeResponse.Error.Message)
	}
	if bridgeResponse.Status != "success" {
		return failure(androidmedia.SCREENSHOT_CAPTURE_FAILED, "native screen capture returned status "+bridgeResponse.Status)
	}

	encoded, _ := bridgeResponse.Result["dataBase64"].(string)
	if strings.TrimSpace(encoded) == "" {
		return failure(androidmedia.SCREENSHOT_ENCODE_FAILED, "native screen capture returned empty image data")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return failure(androidmedia.SCREENSHOT_ENCODE_FAILED, "decode screenshot payload: "+err.Error())
	}
	if len(data) == 0 {
		return failure(androidmedia.SCREENSHOT_ENCODE_FAILED, "native screen capture returned zero bytes")
	}
	maxBytes := h.config.MaxEncodedBytes
	if maxBytes <= 0 {
		maxBytes = androidmedia.DefaultMaxEncodedBytes
	}
	if int64(len(data)) > maxBytes {
		return failure(androidmedia.SCREENSHOT_TOO_LARGE, fmt.Sprintf("encoded screenshot exceeds %d bytes", maxBytes))
	}

	width := numberToInt(bridgeResponse.Result["imageWidth"])
	height := numberToInt(bridgeResponse.Result["imageHeight"])
	if width <= 0 || height <= 0 {
		return failure(androidmedia.SCREENSHOT_ENCODE_FAILED, "native screen capture returned invalid image dimensions")
	}
	maxPixels := h.config.MaxScreenshotPixels
	if maxPixels <= 0 {
		maxPixels = androidmedia.DefaultMaxScreenshotPixels
	}
	if int64(width)*int64(height) > maxPixels {
		return failure(androidmedia.SCREENSHOT_TOO_LARGE, fmt.Sprintf("screenshot exceeds %d pixels", maxPixels))
	}

	mimeType, _ := bridgeResponse.Result["mime"].(string)
	if strings.TrimSpace(mimeType) == "" {
		mimeType = format.MIME()
	}
	if !strings.EqualFold(mimeType, format.MIME()) {
		return failure(androidmedia.SCREENSHOT_ENCODE_FAILED, fmt.Sprintf("native screen capture returned %s for requested %s", mimeType, format.MIME()))
	}

	dir := filepath.Join(h.dataDir, "android_media", "screenshots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return failure(androidmedia.SCREENSHOT_ARTIFACT_WRITE_FAILED, "create screenshot directory: "+err.Error())
	}
	h.gcArtifacts(dir)

	name := mediascreenshot.SafeResourceName(request.RequestID, format.Ext())
	if strings.TrimSpace(request.RequestID) == "" {
		name = mediascreenshot.SafeResourceName(fmt.Sprintf("capture-%d", time.Now().UnixNano()), format.Ext())
	}
	path := filepath.Join(dir, name)
	if err := mediascreenshot.AtomicWrite(path, func(tmp *os.File) error {
		if _, err := tmp.Write(data); err != nil {
			return fmt.Errorf("write screenshot: %w", err)
		}
		if err := tmp.Chmod(0o600); err != nil {
			return fmt.Errorf("chmod screenshot: %w", err)
		}
		return nil
	}); err != nil {
		return failure(androidmedia.SCREENSHOT_ARTIFACT_WRITE_FAILED, err.Error())
	}

	uri, err := h.resolver.Reverse(path)
	if err != nil {
		_ = os.Remove(path)
		return failure(androidmedia.SCREENSHOT_RESOURCE_INVALID, "map screenshot resource: "+err.Error())
	}

	hash := sha256.Sum256(data)
	capturedAt := numberToInt64(bridgeResponse.Result["capturedAt"])
	if capturedAt <= 0 {
		capturedAt = time.Now().UnixMilli()
	}
	result := map[string]any{
		"resourceUri": uri.String(),
		"mimeType":    format.MIME(),
		"width":       width,
		"height":      height,
		"displayId":   numberToInt(bridgeResponse.Result["displayId"]),
		"timestampMs": capturedAt,
		"sizeBytes":   int64(len(data)),
		"contentHash": "sha256:" + hex.EncodeToString(hash[:]),
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          result,
	}
}

func (h *mediaScreenshotHandler) gcArtifacts(dir string) {
	if h == nil {
		return
	}
	ttl := h.config.ArtifactTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-ttl)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}
