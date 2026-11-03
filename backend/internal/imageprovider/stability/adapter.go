package stability

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/timeoutpolicy"
)

const (
	ProviderName   = "stability"
	DefaultModel   = "stable-image-core"
	DefaultBaseURL = "https://api.stability.ai/v2beta"
	DefaultTimeout = 120 * time.Second
	MaxOutputBytes = 64 << 20
)

type Adapter struct {
	client *http.Client
}

func New() *Adapter {
	return NewWithClient(timeoutpolicy.Client(&http.Client{Timeout: DefaultTimeout}))
}

func NewWithClient(client *http.Client) *Adapter {
	if client == nil {
		client = timeoutpolicy.Client(&http.Client{Timeout: DefaultTimeout})
	}
	return &Adapter{client: client}
}

func (a *Adapter) ValidateConfig(_ context.Context, config imageprovider.ImageModelConfig) error {
	if strings.TrimSpace(config.ApiKey) == "" {
		return imageprovider.NewProviderError(imageprovider.ErrCodeConfigMissing, "ApiKey 不能为空", imageprovider.RetryNever, http.StatusBadRequest, nil)
	}
	parsed, err := url.Parse(strings.TrimSpace(config.BaseUrl))
	if err != nil {
		return imageprovider.NewProviderError(imageprovider.ErrCodeConfigMissing, "BaseUrl 无法解析", imageprovider.RetryNever, http.StatusBadRequest, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return imageprovider.NewProviderError(imageprovider.ErrCodeConfigMissing, "BaseUrl 必须是有效的 HTTP 或 HTTPS 地址", imageprovider.RetryNever, http.StatusBadRequest, nil)
	}
	return nil
}

func (a *Adapter) Capabilities(ctx context.Context, config imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	if err := a.ValidateConfig(ctx, config); err != nil {
		return imageprovider.ImageGenerationCapabilities{}, err
	}
	return imageprovider.ImageGenerationCapabilities{
		SupportsReferenceImage: false,
		SupportsMultipleImages: false,
		SupportsNegativePrompt: true,
		SupportsSeed:           true,
		SupportsAsyncOperation: false,
		SupportsCancellation:   false,
		MaxOutputImages:        4,
	}, nil
}

func (a *Adapter) ExtendedCapabilities(ctx context.Context, config imageprovider.ImageModelConfig) (imageprovider.ProviderCapabilities, error) {
	if err := a.ValidateConfig(ctx, config); err != nil {
		return imageprovider.ProviderCapabilities{}, err
	}
	return imageprovider.ProviderCapabilities{
		SchemaVersion:          1,
		Provider:               ProviderName,
		Model:                  config.ModelName,
		SupportedModes:         []imageprovider.GenerationMode{imageprovider.ModeSingleFrame},
		SupportsNegativePrompt: true,
		SupportsSeed:           true,
		MaxOutputImages:        4,
		MaxPromptCharacters:    10000,
		SupportedOutputMIMEs:   []string{"image/png", "image/jpeg", "image/webp"},
		Dimensions: imageprovider.DimensionRule{
			MinWidth:       256,
			MaxWidth:       4096,
			MinHeight:      256,
			MaxHeight:      4096,
			MaxTotalPixels: 4096 * 4096,
			AllowedRatios:  []string{"1:1", "16:9", "9:16"},
		},
		CapabilityVersion: "1.0",
	}, nil
}

func (a *Adapter) Submit(ctx context.Context, config imageprovider.ImageModelConfig, request imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	if err := a.ValidateConfig(ctx, config); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeRequestInvalid, "Prompt 不能为空", imageprovider.RetryNever, http.StatusBadRequest, nil)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("prompt", request.Prompt); err != nil {
		return nil, fmt.Errorf("write stability prompt field: %w", err)
	}
	if request.NegativePrompt != "" {
		if err := writer.WriteField("negative_prompt", request.NegativePrompt); err != nil {
			return nil, fmt.Errorf("write stability negative prompt field: %w", err)
		}
	}
	if ratio, ok := aspectRatio(request.Width, request.Height); ok {
		if err := writer.WriteField("aspect_ratio", ratio); err != nil {
			return nil, fmt.Errorf("write stability aspect ratio field: %w", err)
		}
	}
	if request.Seed != nil {
		if err := writer.WriteField("seed", strconv.FormatInt(*request.Seed, 10)); err != nil {
			return nil, fmt.Errorf("write stability seed field: %w", err)
		}
	}
	if err := writer.WriteField("output_format", "png"); err != nil {
		return nil, fmt.Errorf("write stability output format field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close stability multipart body: %w", err)
	}

	endpoint := strings.TrimRight(strings.TrimSpace(config.BaseUrl), "/") + "/stable-image/generate/core"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build stability request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	req.Header.Set("Accept", "image/*")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeNetworkTimeout, fmt.Sprintf("stability request failed: %v", err), imageprovider.RetrySameCall, http.StatusBadGateway, err)
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxOutputBytes+1))
	if readErr != nil {
		return nil, fmt.Errorf("read stability response: %w", readErr)
	}
	if len(raw) > MaxOutputBytes {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodePixelLimitExceeded, "stability response exceeds size limit", imageprovider.RetryNever, http.StatusBadGateway, nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapHTTPError(resp.StatusCode, string(raw))
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeInvalidFormat, fmt.Sprintf("stability returned non-image content: %s", mimeType), imageprovider.RetryNever, http.StatusBadGateway, nil)
	}
	result := &imageprovider.ImageGenerationResult{
		Images: []imageprovider.GeneratedImage{{
			Bytes:    raw,
			MimeType: mimeType,
		}},
		Provider:    ProviderName,
		Model:       config.ModelName,
		RequestID:   resp.Header.Get("x-request-id"),
		Status:      "succeeded",
		RawMetadata: map[string]any{"requestId": resp.Header.Get("x-request-id")},
	}
	return &imageprovider.ImageGenerationSubmission{
		Status:    "succeeded",
		RequestID: result.RequestID,
		Result:    result,
	}, nil
}

func (a *Adapter) Query(context.Context, imageprovider.ImageModelConfig, string) (*imageprovider.ImageGenerationResult, error) {
	return nil, imageprovider.NewProviderError(imageprovider.ErrCodeStatusUnknown, "stability adapter is synchronous", imageprovider.RetryNever, http.StatusNotImplemented, nil)
}

func (a *Adapter) Cancel(context.Context, imageprovider.ImageModelConfig, string) error {
	return imageprovider.NewProviderError(imageprovider.ErrCodeCancellationNotSupported, "stability adapter does not support cancellation", imageprovider.RetryNever, http.StatusNotImplemented, nil)
}

func (a *Adapter) Probe(ctx context.Context, config imageprovider.ImageModelConfig) (*imageprovider.ImageGenerationSubmission, error) {
	return a.Submit(ctx, config, imageprovider.ImageGenerationRequest{
		Prompt:      "connection test",
		Width:       1024,
		Height:      1024,
		OutputCount: 1,
	})
}

func Register(registry *imageprovider.Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	return registry.RegisterWithDescriptor(ProviderName, New(), imageprovider.ProviderDescriptor{
		Name:           ProviderName,
		DisplayName:    "Stability AI",
		DefaultModel:   DefaultModel,
		DefaultBaseURL: DefaultBaseURL,
		SupportedModes: []imageprovider.GenerationMode{imageprovider.ModeSingleFrame},
	})
}

func aspectRatio(width, height int) (string, bool) {
	switch {
	case width == 1024 && height == 1024:
		return "1:1", true
	case width == 1280 && height == 720:
		return "16:9", true
	case width == 720 && height == 1280:
		return "9:16", true
	default:
		return "", false
	}
}

func mapHTTPError(status int, body string) error {
	message := strings.TrimSpace(body)
	if message == "" {
		message = http.StatusText(status)
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return imageprovider.NewProviderError(imageprovider.ErrCodeAuthFailed, fmt.Sprintf("stability authentication failed (%d): %s", status, message), imageprovider.RetryNever, status, nil)
	case http.StatusPaymentRequired:
		return imageprovider.NewProviderError(imageprovider.ErrCodeInsufficientBalance, fmt.Sprintf("stability balance unavailable (%d): %s", status, message), imageprovider.RetryNever, status, nil)
	case http.StatusTooManyRequests:
		return imageprovider.NewProviderError(imageprovider.ErrCodeRateLimited, fmt.Sprintf("stability rate limited (%d): %s", status, message), imageprovider.RetryAfterDelay, status, nil)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return imageprovider.NewProviderError(imageprovider.ErrCodeRequestInvalid, fmt.Sprintf("stability rejected request (%d): %s", status, message), imageprovider.RetryNever, status, nil)
	default:
		return imageprovider.NewProviderError(imageprovider.ErrCodeServiceError, fmt.Sprintf("stability request failed (%d): %s", status, message), imageprovider.RetryAfterDelay, status, nil)
	}
}

var _ imageprovider.ExtendedProvider = (*Adapter)(nil)
var _ imageprovider.ConnectionProbe = (*Adapter)(nil)
