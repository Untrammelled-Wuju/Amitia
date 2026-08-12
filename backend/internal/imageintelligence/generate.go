package imageintelligence

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider"
)

type ImageGenerateRequest struct {
	Prompt  string `json:"prompt"`
	Count   int    `json:"count,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Quality string `json:"quality,omitempty"`
}

type GeneratedImage struct {
	ResourceURI string `json:"resourceUri"`
	MIMEType    string `json:"mimeType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type ImageGenerateResult struct {
	Images   []GeneratedImage `json:"images"`
	Provider string            `json:"provider"`
}

type GenerateProvider struct {
	imagegenSvc imagegen.Service
	registry    *imageprovider.Registry
	maxCount    int
}

func NewGenerateProvider(imagegenSvc imagegen.Service, registry *imageprovider.Registry) *GenerateProvider {
	return &GenerateProvider{
		imagegenSvc: imagegenSvc,
		registry:    registry,
		maxCount:    4,
	}
}

func (p *GenerateProvider) Generate(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResult, *Error) {
	if p.registry == nil {
		return nil, &Error{Code: ErrGenUnavailable, Message: "image generation provider not configured", HTTPStatus: http.StatusServiceUnavailable}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		return nil, &Error{Code: ErrInvalid, Message: "prompt is required", HTTPStatus: http.StatusBadRequest}
	}

	if len(req.Prompt) > 4096 {
		return nil, &Error{Code: ErrInvalid, Message: "prompt exceeds maximum length of 4096 characters", HTTPStatus: http.StatusBadRequest}
	}

	count := req.Count
	if count < 1 {
		count = 1
	}
	if count > p.maxCount {
		return nil, &Error{Code: ErrInvalid, Message: fmt.Sprintf("count %d exceeds maximum %d", count, p.maxCount), HTTPStatus: http.StatusBadRequest}
	}

	width, height, sizeErr := p.resolveSize(req.Width, req.Height)
	if sizeErr != nil {
		return nil, sizeErr
	}

	if p.imagegenSvc == nil {
		return nil, &Error{Code: ErrGenUnavailable, Message: "image generation config service not available", HTTPStatus: http.StatusServiceUnavailable}
	}

	activeConfig, err := p.imagegenSvc.GetActive()
	if err != nil || activeConfig == nil {
		return nil, &Error{Code: ErrGenUnavailable, Message: "no active image generation config", HTTPStatus: http.StatusServiceUnavailable}
	}
	if activeConfig.ApiKey == "" {
		return nil, &Error{Code: ErrProviderAuth, Message: "image generation API key not configured", HTTPStatus: http.StatusUnauthorized}
	}

	providerName := ""
	names := p.registry.Names()
	for _, name := range names {
		if name == "seedream" {
			providerName = name
			break
		}
	}
	if providerName == "" && len(names) > 0 {
		providerName = names[0]
	}
	if providerName == "" {
		return nil, &Error{Code: ErrGenUnavailable, Message: "no image generation providers registered", HTTPStatus: http.StatusServiceUnavailable}
	}

	adapter, ok := p.registry.Get(providerName)
	if !ok {
		return nil, &Error{Code: ErrGenUnavailable, Message: fmt.Sprintf("provider %s not found", providerName), HTTPStatus: http.StatusServiceUnavailable}
	}

	providerConfig := imageprovider.ImageModelConfig{
		Name:      providerName,
		ApiType:   activeConfig.ApiType,
		ApiKey:    activeConfig.ApiKey,
		ModelName: activeConfig.ModelName,
		BaseUrl:   activeConfig.BaseUrl,
	}

	var images []GeneratedImage
	for i := 0; i < count; i++ {
		submitReq := imageprovider.ImageGenerationRequest{
			Prompt:      req.Prompt,
			Width:       width,
			Height:      height,
			OutputCount: 1,
		}

		submission, submitErr := adapter.Submit(ctx, providerConfig, submitReq)
		if submitErr != nil {
			mapped := p.mapProviderError(submitErr)
			return nil, mapped
		}

		if submission == nil || submission.Result == nil || len(submission.Result.Images) == 0 {
			return nil, &Error{Code: ErrGenInvalidResponse, Message: "provider returned no images", HTTPStatus: http.StatusBadGateway}
		}

		for _, img := range submission.Result.Images {
			genImg, convErr := p.resourceifyImage(img)
			if convErr != nil {
				return nil, convErr
			}
			images = append(images, *genImg)
		}
	}

	return &ImageGenerateResult{
		Images:   images,
		Provider: providerName,
	}, nil
}

func (p *GenerateProvider) resolveSize(width, height int) (int, int, *Error) {
	if width == 0 && height == 0 {
		return 1024, 1024, nil
	}
	if width == 0 || height == 0 {
		return 0, 0, &Error{Code: ErrInvalid, Message: "both width and height must be specified together", HTTPStatus: http.StatusBadRequest}
	}
	if width < 256 || width > 4096 || height < 256 || height > 4096 {
		return 0, 0, &Error{Code: ErrInvalid, Message: "dimensions must be between 256 and 4096", HTTPStatus: http.StatusBadRequest}
	}

	supported := [][2]int{{1024, 1024}, {1280, 720}, {720, 1280}}
	matched := false
	for _, size := range supported {
		if size[0] == width && size[1] == height {
			matched = true
			break
		}
	}
	if !matched {
		return 0, 0, &Error{Code: ErrInvalid, Message: fmt.Sprintf("unsupported size %dx%d, supported: 1024x1024, 1280x720, 720x1280", width, height), HTTPStatus: http.StatusBadRequest}
	}

	return width, height, nil
}

func (p *GenerateProvider) resourceifyImage(img imageprovider.GeneratedImage) (*GeneratedImage, *Error) {
	if len(img.Bytes) == 0 {
		return nil, &Error{Code: ErrGenOutputInvalid, Message: "generated image has no data", HTTPStatus: http.StatusBadGateway}
	}

	mime := img.MimeType
	if mime == "" {
		mime = detectMIME(img.Bytes)
	}
	if !isSupportedMIME(mime, []string{"image/png", "image/jpeg", "image/webp"}) {
		return nil, &Error{Code: ErrGenOutputInvalid, Message: fmt.Sprintf("unsupported output MIME type: %s", mime), HTTPStatus: http.StatusBadGateway}
	}

	w, h := img.Width, img.Height
	if w == 0 || h == 0 {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Bytes)); err == nil {
			w = cfg.Width
			h = cfg.Height
		}
	}

	timestamp := time.Now().UnixNano()
	ext := ".png"
	if mime == "image/jpeg" {
		ext = ".jpg"
	} else if mime == "image/webp" {
		ext = ".webp"
	}
	filename := fmt.Sprintf("img_%d%s", timestamp, ext)

	tmpDir := os.TempDir()
	localPath := filepath.Join(tmpDir, filename)

	if err := os.WriteFile(localPath, img.Bytes, 0644); err != nil {
		return nil, &Error{Code: ErrGenFailed, Message: fmt.Sprintf("failed to save generated image: %v", err), HTTPStatus: http.StatusInternalServerError}
	}

	info, statErr := os.Stat(localPath)
	sizeBytes := int64(len(img.Bytes))
	if statErr == nil {
		sizeBytes = info.Size()
	}

	resolvedURI := "amitia://temp/" + filename

	return &GeneratedImage{
		ResourceURI: resolvedURI,
		MIMEType:    mime,
		Width:       w,
		Height:      h,
		SizeBytes:   sizeBytes,
	}, nil
}

func (p *GenerateProvider) mapProviderError(err error) *Error {
	if pe, ok := err.(*imageprovider.ProviderError); ok {
		retryClass := string(pe.RetryClass)
		switch pe.Code {
		case imageprovider.ErrCodeAuthFailed:
			return &Error{Code: ErrProviderAuth, Message: pe.Message, Provider: "seedream", Retryable: false, HTTPStatus: http.StatusUnauthorized}
		case imageprovider.ErrCodeRateLimited:
			return &Error{Code: ErrProviderRateLimited, Message: pe.Message, Provider: "seedream", Retryable: false, HTTPStatus: http.StatusTooManyRequests}
		case imageprovider.ErrCodeRequestInvalid:
			return &Error{Code: ErrInvalid, Message: pe.Message, Provider: "seedream", Retryable: false, HTTPStatus: http.StatusBadRequest}
		}
		return mapProviderErrorToDomain("seedream", retryClass, pe.Message)
	}
	return &Error{Code: ErrGenFailed, Message: err.Error(), Provider: "seedream", Retryable: false, HTTPStatus: http.StatusBadGateway}
}
