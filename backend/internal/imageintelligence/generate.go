package imageintelligence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider"
)

type ImageGenerateRequest struct {
	RequestID             string                       `json:"requestId,omitempty"`
	Prompt                string                       `json:"prompt"`
	Count                 int                          `json:"count,omitempty"`
	Width                 int                          `json:"width,omitempty"`
	Height                int                          `json:"height,omitempty"`
	Quality               string                       `json:"quality,omitempty"`
	Mode                  imageprovider.GenerationMode `json:"mode,omitempty"`
	NegativePrompt        string                       `json:"negativePrompt,omitempty"`
	Seed                  *int64                       `json:"seed,omitempty"`
	ReferenceResourceURIs []string                     `json:"referenceResourceUris,omitempty"`
	ReferenceImages       []imageprovider.ImageInput   `json:"-"`
	OwnerSpaceID          string                       `json:"-"`
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
	Provider string           `json:"provider"`
}

type GenerateProvider struct {
	imagegenSvc imagegen.Service
	registry    *imageprovider.Registry
	resolver    *ImageResourceResolver
	artifacts   ArtifactCreator
	maxCount    int
}

type ArtifactCreator interface {
	Create(ctx context.Context, req artifact.CreateRequest) (artifact.Artifact, error)
}

func NewGenerateProvider(imagegenSvc imagegen.Service, registry *imageprovider.Registry, resolver *ImageResourceResolver, artifacts ArtifactCreator) *GenerateProvider {
	return &GenerateProvider{
		imagegenSvc: imagegenSvc,
		registry:    registry,
		resolver:    resolver,
		artifacts:   artifacts,
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
	if err != nil {
		return nil, &Error{Code: ErrGenUnavailable, Message: err.Error(), HTTPStatus: http.StatusServiceUnavailable}
	}
	if activeConfig == nil {
		return nil, &Error{Code: ErrGenUnavailable, Message: "no active image generation config", HTTPStatus: http.StatusServiceUnavailable}
	}
	providerName, adapter, ok := p.registry.Select(activeConfig.ApiType, activeConfig.ApiKey)
	if !ok {
		return nil, &Error{Code: ErrGenUnavailable, Message: fmt.Sprintf("provider %s is not registered", providerName), Provider: providerName, HTTPStatus: http.StatusServiceUnavailable}
	}
	if activeConfig.ApiKey == "" && providerName != "cloud_core" {
		return nil, &Error{Code: ErrProviderAuth, Message: "image generation API key not configured", Provider: providerName, HTTPStatus: http.StatusUnauthorized}
	}

	providerConfig := imageprovider.ImageModelConfig{
		ConfigID:  activeConfig.ID,
		Name:      providerName,
		ApiType:   activeConfig.ApiType,
		ApiKey:    activeConfig.ApiKey,
		ModelName: activeConfig.ModelName,
		BaseUrl:   activeConfig.BaseUrl,
	}
	if providerName == "cloud_core" {
		providerConfig.ConfigID = 0
	}
	referenceImages, referenceErr := p.referenceImages(ctx, req)
	if referenceErr != nil {
		return nil, referenceErr
	}
	providerCaps, capsErr := adapter.Capabilities(ctx, providerConfig)
	if capsErr != nil {
		return nil, p.mapProviderError(providerName, capsErr)
	}
	if len(referenceImages) > 0 && !providerCaps.SupportsReferenceImage {
		return nil, &Error{Code: ErrGenUnavailable, Message: fmt.Sprintf("provider %s does not support reference images", providerName), Provider: providerName, HTTPStatus: http.StatusBadRequest}
	}
	if providerCaps.MaxReferenceImages > 0 && len(referenceImages) > providerCaps.MaxReferenceImages {
		return nil, &Error{Code: ErrInvalid, Message: fmt.Sprintf("reference image count %d exceeds provider limit %d", len(referenceImages), providerCaps.MaxReferenceImages), Provider: providerName, HTTPStatus: http.StatusBadRequest}
	}
	if providerCaps.MaxOutputImages > 0 && count > providerCaps.MaxOutputImages {
		return nil, &Error{Code: ErrInvalid, Message: fmt.Sprintf("output count %d exceeds provider limit %d", count, providerCaps.MaxOutputImages), Provider: providerName, HTTPStatus: http.StatusBadRequest}
	}
	if req.Mode != "" {
		extended, ok := adapter.(imageprovider.ExtendedProvider)
		if !ok {
			return nil, &Error{Code: ErrGenUnavailable, Message: fmt.Sprintf("provider %s does not expose generation modes", providerName), Provider: providerName, HTTPStatus: http.StatusBadRequest}
		}
		extendedCaps, extendedErr := extended.ExtendedCapabilities(ctx, providerConfig)
		if extendedErr != nil {
			return nil, p.mapProviderError(providerName, extendedErr)
		}
		if !extendedCaps.SupportsMode(req.Mode) {
			return nil, &Error{Code: ErrInvalid, Message: fmt.Sprintf("provider %s does not support mode %s", providerName, req.Mode), Provider: providerName, HTTPStatus: http.StatusBadRequest}
		}
	}

	var images []GeneratedImage
	for i := 0; i < count; i++ {
		submitReq := imageprovider.ImageGenerationRequest{
			RequestID:       reqRequestID(req),
			Mode:            req.Mode,
			Prompt:          req.Prompt,
			NegativePrompt:  req.NegativePrompt,
			ReferenceImages: referenceImages,
			Width:           width,
			Height:          height,
			Seed:            req.Seed,
			OutputCount:     1,
		}

		submission, submitErr := adapter.Submit(ctx, providerConfig, submitReq)
		if submitErr != nil {
			mapped := p.mapProviderError(providerName, submitErr)
			return nil, mapped
		}

		if submission == nil || submission.Result == nil || len(submission.Result.Images) == 0 {
			return nil, &Error{Code: ErrGenInvalidResponse, Message: "provider returned no images", HTTPStatus: http.StatusBadGateway}
		}

		for _, img := range submission.Result.Images {
			genImg, convErr := p.resourceifyImage(ctx, req.OwnerSpaceID, img)
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

func (p *GenerateProvider) resourceifyImage(ctx context.Context, ownerSpaceID string, img imageprovider.GeneratedImage) (*GeneratedImage, *Error) {
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

	ext := ".png"
	if mime == "image/jpeg" {
		ext = ".jpg"
	} else if mime == "image/webp" {
		ext = ".webp"
	}
	filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
	if p.artifacts != nil {
		if strings.TrimSpace(ownerSpaceID) == "" {
			return nil, &Error{Code: ErrGenFailed, Message: "generated image owner is missing", HTTPStatus: http.StatusInternalServerError}
		}
		art, err := p.artifacts.Create(ctx, artifact.CreateRequest{
			OwnerSpaceID: ownerSpaceID,
			Kind:         artifact.KindImage,
			MIMEType:     mime,
			Filename:     filename,
			Source:       artifact.SourceGenerated,
			Width:        w,
			Height:       h,
			Reader:       bytes.NewReader(img.Bytes),
		})
		if err != nil {
			return nil, &Error{Code: ErrGenFailed, Message: fmt.Sprintf("failed to persist generated image: %v", err), HTTPStatus: http.StatusInternalServerError}
		}
		return &GeneratedImage{
			ResourceURI: artifact.URI(art.ID),
			MIMEType:    art.MIMEType,
			Width:       w,
			Height:      h,
			SizeBytes:   art.SizeBytes,
		}, nil
	}

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

func (p *GenerateProvider) referenceImages(ctx context.Context, req ImageGenerateRequest) ([]imageprovider.ImageInput, *Error) {
	if len(req.ReferenceResourceURIs) == 0 {
		return append([]imageprovider.ImageInput(nil), req.ReferenceImages...), nil
	}
	if p.resolver == nil {
		return nil, &Error{Code: ErrGenUnavailable, Message: "image resource resolver is unavailable", HTTPStatus: http.StatusServiceUnavailable}
	}
	caps := ImageCapabilities{
		SupportedInputMIMEs: []string{"image/png", "image/jpeg", "image/webp"},
		MaxInputBytes:       20 * 1024 * 1024,
		MaxWidth:            10000,
		MaxHeight:           10000,
		MaxPixels:           40000000,
	}
	images := make([]imageprovider.ImageInput, 0, len(req.ReferenceResourceURIs))
	for _, uri := range req.ReferenceResourceURIs {
		data, summary, resolveErr := p.resolver.ResolveAndValidate(ctx, ImageInput{ResourceURI: uri}, caps)
		if resolveErr != nil {
			return nil, resolveErr
		}
		images = append(images, imageprovider.ImageInput{Bytes: data, MimeType: summary.MIME})
	}
	return images, nil
}

func reqRequestID(req ImageGenerateRequest) string {
	if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
		return requestID
	}
	return fmt.Sprintf("image-%d", time.Now().UnixNano())
}

func (p *GenerateProvider) mapProviderError(provider string, err error) *Error {
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) {
		switch coded.ErrorCode() {
		case imageprovider.ErrCodeAuthFailed:
			return &Error{Code: ErrProviderAuth, Message: err.Error(), Provider: provider, Retryable: false, HTTPStatus: http.StatusUnauthorized}
		case imageprovider.ErrCodeInsufficientBalance:
			return &Error{Code: ErrProviderAuth, Message: err.Error(), Provider: provider, Retryable: false, HTTPStatus: http.StatusPaymentRequired}
		case imageprovider.ErrCodeRateLimited:
			return &Error{Code: ErrProviderRateLimited, Message: err.Error(), Provider: provider, Retryable: true, HTTPStatus: http.StatusTooManyRequests}
		case imageprovider.ErrCodeRequestInvalid, "IMAGE_GENERATION_PROVIDER_REJECTED":
			return &Error{Code: ErrInvalid, Message: err.Error(), Provider: provider, Retryable: false, HTTPStatus: http.StatusBadRequest}
		case imageprovider.ErrCodeNetworkTimeout, "IMAGE_GENERATION_TIMEOUT":
			return &Error{Code: ErrProviderUnavailable, Message: err.Error(), Provider: provider, Retryable: true, HTTPStatus: http.StatusGatewayTimeout}
		case imageprovider.ErrCodeInvalidFormat, imageprovider.ErrCodeEmptyResult, imageprovider.ErrCodeDownloadFailed:
			return &Error{Code: ErrProviderInvalidResponse, Message: err.Error(), Provider: provider, Retryable: true, HTTPStatus: http.StatusBadGateway}
		}
	}
	var pe *imageprovider.ProviderError
	if errors.As(err, &pe) {
		retryClass := string(pe.RetryClass)
		switch pe.Code {
		case imageprovider.ErrCodeAuthFailed:
			return &Error{Code: ErrProviderAuth, Message: err.Error(), Provider: provider, Retryable: false, HTTPStatus: http.StatusUnauthorized}
		case imageprovider.ErrCodeInsufficientBalance:
			return &Error{Code: ErrProviderAuth, Message: err.Error(), Provider: provider, Retryable: false, HTTPStatus: http.StatusPaymentRequired}
		case imageprovider.ErrCodeRateLimited:
			return &Error{Code: ErrProviderRateLimited, Message: err.Error(), Provider: provider, Retryable: true, HTTPStatus: http.StatusTooManyRequests}
		case imageprovider.ErrCodeRequestInvalid:
			return &Error{Code: ErrInvalid, Message: err.Error(), Provider: provider, Retryable: false, HTTPStatus: http.StatusBadRequest}
		case imageprovider.ErrCodeNetworkTimeout:
			return &Error{Code: ErrProviderUnavailable, Message: err.Error(), Provider: provider, Retryable: true, HTTPStatus: http.StatusGatewayTimeout}
		case imageprovider.ErrCodeInvalidFormat, imageprovider.ErrCodeEmptyResult, imageprovider.ErrCodeDownloadFailed:
			return &Error{Code: ErrProviderInvalidResponse, Message: err.Error(), Provider: provider, Retryable: true, HTTPStatus: http.StatusBadGateway}
		}
		return mapProviderErrorToDomain(provider, retryClass, err.Error())
	}
	return &Error{Code: ErrGenFailed, Message: err.Error(), Provider: provider, Retryable: false, HTTPStatus: http.StatusBadGateway}
}
