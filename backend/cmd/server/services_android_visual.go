//go:build linux

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/androidnative/interaction"
	"github.com/u-ai/backend/internal/androidnative/uitree"
	"github.com/u-ai/backend/internal/imageintelligence"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type screenshotMeta struct {
	modelWidth  int
	modelHeight int
}

type screenshotMetaRegistry struct {
	mu    sync.RWMutex
	items map[string]screenshotMeta
}

func newScreenshotMetaRegistry() *screenshotMetaRegistry {
	return &screenshotMetaRegistry{items: make(map[string]screenshotMeta)}
}

func (r *screenshotMetaRegistry) put(uri string, meta screenshotMeta) {
	r.mu.Lock()
	r.items[uri] = meta
	r.mu.Unlock()
}

func (r *screenshotMetaRegistry) get(uri string) (screenshotMeta, bool) {
	r.mu.RLock()
	meta, ok := r.items[uri]
	r.mu.RUnlock()
	return meta, ok
}

type bridgeScreenshotProvider struct {
	bridge   nativebridge.Bridge
	resolver *resourceuri.PhysicalResolver
	dataDir  string
	meta     *screenshotMetaRegistry
}

func (p *bridgeScreenshotProvider) Available(ctx context.Context) (bool, string) {
	if p == nil || p.bridge == nil || p.resolver == nil || strings.TrimSpace(p.dataDir) == "" {
		return false, "screen capture persistence is not configured"
	}
	resp, err := p.bridge.Execute(ctx, nativebridge.Request{
		ProtocolVersion: nativebridge.AndroidBridgeProtocolVersion,
		RequestId:       uuid.NewString(),
		Platform:        "android",
		Operation:       "screen_capture.status",
		Payload:         map[string]any{},
	})
	if err != nil {
		return false, err.Error()
	}
	if resp.Error != nil {
		return false, resp.Error.Message
	}
	available, _ := resp.Result["available"].(bool)
	reason, _ := resp.Result["reason"].(string)
	return available, reason
}

func (p *bridgeScreenshotProvider) Capture(ctx context.Context, displayID int) (*interaction.ScreenshotResult, error) {
	if ok, reason := p.Available(ctx); !ok {
		return nil, fmt.Errorf("screen capture unavailable: %s", reason)
	}
	resp, err := p.bridge.Execute(ctx, nativebridge.Request{
		ProtocolVersion: nativebridge.AndroidBridgeProtocolVersion,
		RequestId:       uuid.NewString(),
		Platform:        "android",
		Operation:       "screen_capture.capture",
		Payload: map[string]any{
			"displayId": displayID,
			"maxWidth":  1440,
			"maxHeight": 2560,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
	}
	encoded, _ := resp.Result["dataBase64"].(string)
	if strings.TrimSpace(encoded) == "" {
		return nil, fmt.Errorf("screen capture returned empty image")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	if len(data) == 0 || len(data) > 20*1024*1024 {
		return nil, fmt.Errorf("invalid screenshot payload size: %d", len(data))
	}

	dir := filepath.Join(p.dataDir, "android_automation", "screenshots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create screenshot directory: %w", err)
	}
	p.gcScreenshots(dir)
	path := filepath.Join(dir, fmt.Sprintf("capture-%d-%s.png", time.Now().UnixMilli(), uuid.NewString()))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, fmt.Errorf("persist screenshot: %w", err)
	}
	uri, err := p.resolver.Reverse(path)
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("map screenshot resource: %w", err)
	}

	result := &interaction.ScreenshotResult{
		ResourceURI: uri.String(),
		DisplayID:   numberToInt(resp.Result["displayId"]),
		Width:       numberToInt(resp.Result["screenWidth"]),
		Height:      numberToInt(resp.Result["screenHeight"]),
		ModelWidth:  numberToInt(resp.Result["imageWidth"]),
		ModelHeight: numberToInt(resp.Result["imageHeight"]),
		CapturedAt:  numberToInt64(resp.Result["capturedAt"]),
		Generation:  numberToInt64(resp.Result["generation"]),
	}
	result.StateToken, _ = resp.Result["stateToken"].(string)
	if result.Width <= 0 {
		result.Width = result.ModelWidth
	}
	if result.Height <= 0 {
		result.Height = result.ModelHeight
	}
	if result.CapturedAt <= 0 {
		result.CapturedAt = time.Now().UnixMilli()
	}
	if p.meta != nil {
		p.meta.put(result.ResourceURI, screenshotMeta{modelWidth: result.ModelWidth, modelHeight: result.ModelHeight})
	}
	return result, nil
}

func (p *bridgeScreenshotProvider) gcScreenshots(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-15 * time.Minute)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "capture-") {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

type imageOCRAdapter struct {
	image imageintelligence.ImageIntelligence
	meta  *screenshotMetaRegistry
}

func (a *imageOCRAdapter) Available(ctx context.Context) (bool, string) {
	if a == nil || a.image == nil {
		return false, "image intelligence is not configured"
	}
	status := a.image.Status(ctx)
	if !status.Capabilities.OCR {
		return false, "OCR provider is not configured"
	}
	return true, ""
}

func (a *imageOCRAdapter) Recognize(ctx context.Context, imageRef string) (*interaction.OCRResult, error) {
	result, err := a.image.OCR(ctx, imageintelligence.ImageOCRRequest{Image: imageintelligence.ImageInput{ResourceURI: imageRef}, IncludeBoxes: true})
	if err != nil {
		return nil, err
	}
	meta, _ := a.meta.get(imageRef)
	lines := make([]interaction.OCRLine, 0, len(result.Blocks))
	for _, block := range result.Blocks {
		if strings.TrimSpace(block.Text) == "" || block.Box == nil || meta.modelWidth <= 0 || meta.modelHeight <= 0 {
			continue
		}
		confidence := 0.75
		if block.Confidence != nil {
			confidence = *block.Confidence
		}
		box := block.Box
		lines = append(lines, interaction.OCRLine{
			Text: block.Text,
			Bounds: uitree.Rect{
				Left:   int(box.X * float64(meta.modelWidth)),
				Top:    int(box.Y * float64(meta.modelHeight)),
				Right:  int((box.X + box.Width) * float64(meta.modelWidth)),
				Bottom: int((box.Y + box.Height) * float64(meta.modelHeight)),
			},
			Confidence: confidence,
		})
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("OCR provider returned no positioned text blocks")
	}
	return &interaction.OCRResult{Lines: lines}, nil
}

type imageUnderstandAdapter struct {
	image imageintelligence.ImageIntelligence
	meta  *screenshotMetaRegistry
}

type visualLocateEnvelope struct {
	Candidates []struct {
		X           float64 `json:"x"`
		Y           float64 `json:"y"`
		Width       float64 `json:"width"`
		Height      float64 `json:"height"`
		Confidence  float64 `json:"confidence"`
		Description string  `json:"description"`
	} `json:"candidates"`
}

func (a *imageUnderstandAdapter) Available(ctx context.Context) (bool, string) {
	if a == nil || a.image == nil {
		return false, "image intelligence is not configured"
	}
	status := a.image.Status(ctx)
	if !status.Capabilities.Understand {
		return false, "image understanding provider is not configured"
	}
	return true, ""
}

func (a *imageUnderstandAdapter) Understand(ctx context.Context, imageRef, description string) (*interaction.UnderstandResult, error) {
	prompt := "Locate the requested UI element in this screenshot. Target: " + description + `. Return ONLY JSON: {"candidates":[{"x":0.0,"y":0.0,"width":0.0,"height":0.0,"confidence":0.0,"description":""}]}. x/y/width/height are normalized 0..1 coordinates relative to the image. Do not include markdown.`
	result, err := a.image.Understand(ctx, imageintelligence.ImageUnderstandRequest{Image: imageintelligence.ImageInput{ResourceURI: imageRef}, Prompt: prompt})
	if err != nil {
		return nil, err
	}
	var envelope visualLocateEnvelope
	if err := json.Unmarshal([]byte(stripJSONFence(result.Text)), &envelope); err != nil {
		return nil, fmt.Errorf("parse visual locate response: %w", err)
	}
	meta, ok := a.meta.get(imageRef)
	if !ok || meta.modelWidth <= 0 || meta.modelHeight <= 0 {
		return nil, fmt.Errorf("screenshot metadata not found")
	}
	out := make([]interaction.VisualCandidate, 0, len(envelope.Candidates))
	for _, c := range envelope.Candidates {
		if c.Width <= 0 || c.Height <= 0 || c.Confidence <= 0 {
			continue
		}
		left := int(clamp01(c.X) * float64(meta.modelWidth))
		top := int(clamp01(c.Y) * float64(meta.modelHeight))
		right := int(clamp01(c.X+c.Width) * float64(meta.modelWidth))
		bottom := int(clamp01(c.Y+c.Height) * float64(meta.modelHeight))
		if right <= left || bottom <= top {
			continue
		}
		out = append(out, interaction.VisualCandidate{Description: c.Description, Bounds: uitree.Rect{Left: left, Top: top, Right: right, Bottom: bottom}, Confidence: c.Confidence})
	}
	return &interaction.UnderstandResult{Candidates: out}, nil
}

func stripJSONFence(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "```") {
		value = strings.TrimPrefix(value, "```json")
		value = strings.TrimPrefix(value, "```")
		value = strings.TrimSuffix(strings.TrimSpace(value), "```")
	}
	return strings.TrimSpace(value)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func numberToInt(v any) int { return int(numberToInt64(v)) }
func numberToInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case int32:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}
