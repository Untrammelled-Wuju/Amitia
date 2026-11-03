package tongyi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/timeoutpolicy"
)

const (
	ProviderName   = "tongyi"
	DefaultModel   = "wanx2.1-t2i-turbo"
	DefaultBaseURL = "https://dashscope.aliyuncs.com/api/v1"
	DefaultTimeout = 3 * time.Minute
	MaxOutputBytes = 64 << 20
)

type Adapter struct {
	client       *http.Client
	pollInterval time.Duration
}

func New() *Adapter {
	return NewWithClient(timeoutpolicy.Client(&http.Client{Timeout: DefaultTimeout}))
}

func NewWithClient(client *http.Client) *Adapter {
	if client == nil {
		client = timeoutpolicy.Client(&http.Client{Timeout: DefaultTimeout})
	}
	return &Adapter{client: client, pollInterval: 750 * time.Millisecond}
}

func (a *Adapter) ValidateConfig(_ context.Context, config imageprovider.ImageModelConfig) error {
	if strings.TrimSpace(config.ApiKey) == "" {
		return imageprovider.NewProviderError(imageprovider.ErrCodeConfigMissing, "ApiKey 不能为空", imageprovider.RetryNever, http.StatusBadRequest, nil)
	}
	if strings.TrimSpace(config.ModelName) == "" {
		return imageprovider.NewProviderError(imageprovider.ErrCodeConfigMissing, "ModelName 不能为空", imageprovider.RetryNever, http.StatusBadRequest, nil)
	}
	parsed, err := url.Parse(strings.TrimSpace(config.BaseUrl))
	if err != nil {
		return imageprovider.NewProviderError(imageprovider.ErrCodeConfigMissing, "BaseUrl 无法解析", imageprovider.RetryNever, http.StatusBadRequest, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
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
		SupportsMultipleImages: true,
		SupportsNegativePrompt: true,
		SupportsSeed:           true,
		SupportsAsyncOperation: true,
		SupportsCancellation:   true,
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
		SupportsMultipleImages: true,
		SupportsNegativePrompt: true,
		SupportsSeed:           true,
		SupportsAsyncOperation: true,
		SupportsCancellation:   true,
		MaxOutputImages:        4,
		MaxPromptCharacters:    8000,
		SupportedOutputMIMEs:   []string{"image/png", "image/jpeg", "image/webp"},
		Dimensions: imageprovider.DimensionRule{
			MinWidth:       512,
			MaxWidth:       2048,
			MinHeight:      512,
			MaxHeight:      2048,
			MaxTotalPixels: 2048 * 2048,
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
	outputCount := request.OutputCount
	if outputCount <= 0 {
		outputCount = 1
	}
	parameters := map[string]any{"n": outputCount}
	if request.Width > 0 && request.Height > 0 {
		parameters["size"] = fmt.Sprintf("%d*%d", request.Width, request.Height)
	}
	if request.Seed != nil {
		parameters["seed"] = *request.Seed
	}
	payload := map[string]any{
		"model": config.ModelName,
		"input": map[string]any{
			"prompt":          request.Prompt,
			"negative_prompt": request.NegativePrompt,
		},
		"parameters": parameters,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal tongyi request: %w", err)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(config.BaseUrl), "/") + "/services/aigc/text2image/image-synthesis"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build tongyi submit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeNetworkTimeout, fmt.Sprintf("tongyi submit request failed: %v", err), imageprovider.RetrySameCall, http.StatusBadGateway, err)
	}
	raw, readErr := readResponse(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapHTTPError(resp.StatusCode, raw)
	}
	var submission dashScopeResponse
	if err := json.Unmarshal(raw, &submission); err != nil {
		return nil, fmt.Errorf("decode tongyi submit response: %w", err)
	}
	if submission.Output.TaskID == "" {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeStatusUnknown, "tongyi response missing task id", imageprovider.RetryQueryFirst, http.StatusBadGateway, nil)
	}
	result, err := a.waitForTask(ctx, config, submission.Output.TaskID)
	if err != nil {
		return nil, err
	}
	return &imageprovider.ImageGenerationSubmission{
		Status:      "succeeded",
		OperationID: submission.Output.TaskID,
		RequestID:   firstNonEmpty(submission.RequestID, submission.Output.RequestID),
		Result:      result,
	}, nil
}

func (a *Adapter) Query(ctx context.Context, config imageprovider.ImageModelConfig, operationID string) (*imageprovider.ImageGenerationResult, error) {
	if err := a.ValidateConfig(ctx, config); err != nil {
		return nil, err
	}
	if strings.TrimSpace(operationID) == "" {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeRequestInvalid, "operation id is required", imageprovider.RetryNever, http.StatusBadRequest, nil)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(config.BaseUrl), "/") + "/tasks/" + url.PathEscape(operationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build tongyi query request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeNetworkTimeout, fmt.Sprintf("tongyi query request failed: %v", err), imageprovider.RetryQueryFirst, http.StatusBadGateway, err)
	}
	raw, readErr := readResponse(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapHTTPError(resp.StatusCode, raw)
	}
	var task dashScopeResponse
	if err := json.Unmarshal(raw, &task); err != nil {
		return nil, fmt.Errorf("decode tongyi query response: %w", err)
	}
	status := strings.TrimSpace(task.Output.TaskStatus)
	switch status {
	case "SUCCEEDED", "PARTIAL_SUCCEEDED":
		images, err := a.downloadImages(ctx, task.Output.Results)
		if err != nil {
			return nil, err
		}
		if len(images) == 0 {
			return nil, imageprovider.NewProviderError(imageprovider.ErrCodeEmptyResult, "tongyi returned no images", imageprovider.RetryQueryFirst, http.StatusBadGateway, nil)
		}
		return &imageprovider.ImageGenerationResult{
			Images:      images,
			Provider:    ProviderName,
			Model:       config.ModelName,
			RequestID:   firstNonEmpty(task.RequestID, task.Output.RequestID),
			RawMetadata: map[string]any{"status": status},
			Status:      "succeeded",
			OperationID: operationID,
		}, nil
	case "FAILED", "CANCELED", "UNKNOWN":
		message := firstNonEmpty(task.Message, task.Output.Message)
		if message == "" {
			message = "tongyi task failed with status " + status
		}
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeServiceError, message, imageprovider.RetryUserAction, http.StatusBadGateway, nil)
	default:
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodeStatusUnknown, "tongyi task is still running: "+status, imageprovider.RetryQueryFirst, http.StatusAccepted, nil)
	}
}

func (a *Adapter) Cancel(ctx context.Context, config imageprovider.ImageModelConfig, operationID string) error {
	if err := a.ValidateConfig(ctx, config); err != nil {
		return err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(config.BaseUrl), "/") + "/tasks/" + url.PathEscape(operationID) + "/cancel"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build tongyi cancel request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	resp, err := a.client.Do(req)
	if err != nil {
		return imageprovider.NewProviderError(imageprovider.ErrCodeNetworkTimeout, fmt.Sprintf("tongyi cancel request failed: %v", err), imageprovider.RetrySameCall, http.StatusBadGateway, err)
	}
	defer resp.Body.Close()
	raw, readErr := readResponse(resp.Body)
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mapHTTPError(resp.StatusCode, raw)
	}
	return nil
}

func (a *Adapter) Probe(ctx context.Context, config imageprovider.ImageModelConfig) (*imageprovider.ImageGenerationSubmission, error) {
	return a.Submit(ctx, config, imageprovider.ImageGenerationRequest{
		Prompt:      "connection test",
		Width:       1024,
		Height:      1024,
		OutputCount: 1,
	})
}

func (a *Adapter) waitForTask(ctx context.Context, config imageprovider.ImageModelConfig, taskID string) (*imageprovider.ImageGenerationResult, error) {
	for {
		result, err := a.Query(ctx, config, taskID)
		if err == nil {
			return result, nil
		}
		providerErr, ok := err.(*imageprovider.ProviderError)
		if !ok || providerErr.Code != imageprovider.ErrCodeStatusUnknown || providerErr.HTTPStatus != http.StatusAccepted {
			return nil, err
		}
		timer := time.NewTimer(a.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (a *Adapter) downloadImages(ctx context.Context, results []dashScopeResult) ([]imageprovider.GeneratedImage, error) {
	images := make([]imageprovider.GeneratedImage, 0, len(results))
	for _, item := range results {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
		if err != nil {
			return nil, fmt.Errorf("build tongyi image download request: %w", err)
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, imageprovider.NewProviderError(imageprovider.ErrCodeDownloadFailed, fmt.Sprintf("download tongyi image failed: %v", err), imageprovider.RetryQueryFirst, http.StatusBadGateway, err)
		}
		raw, readErr := readResponse(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, imageprovider.NewProviderError(imageprovider.ErrCodeDownloadFailed, fmt.Sprintf("download tongyi image returned %d: %s", resp.StatusCode, string(raw)), imageprovider.RetryQueryFirst, http.StatusBadGateway, nil)
		}
		mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
		if !strings.HasPrefix(mimeType, "image/") {
			mimeType = http.DetectContentType(raw)
		}
		if !strings.HasPrefix(mimeType, "image/") {
			return nil, imageprovider.NewProviderError(imageprovider.ErrCodeInvalidFormat, "tongyi returned non-image content", imageprovider.RetryNever, http.StatusBadGateway, nil)
		}
		images = append(images, imageprovider.GeneratedImage{Bytes: raw, MimeType: mimeType, Metadata: map[string]any{"url": item.URL}})
	}
	return images, nil
}

type dashScopeResponse struct {
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Output    struct {
		TaskID     string            `json:"task_id"`
		TaskStatus string            `json:"task_status"`
		RequestID  string            `json:"request_id"`
		Message    string            `json:"message"`
		Results    []dashScopeResult `json:"results"`
	} `json:"output"`
}

type dashScopeResult struct {
	URL string `json:"url"`
}

func readResponse(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, MaxOutputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read tongyi response: %w", err)
	}
	if len(raw) > MaxOutputBytes {
		return nil, imageprovider.NewProviderError(imageprovider.ErrCodePixelLimitExceeded, "tongyi response exceeds size limit", imageprovider.RetryNever, http.StatusBadGateway, nil)
	}
	return raw, nil
}

func mapHTTPError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	var parsed dashScopeResponse
	if json.Unmarshal(body, &parsed) == nil {
		message = firstNonEmpty(parsed.Message, parsed.Code, message)
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return imageprovider.NewProviderError(imageprovider.ErrCodeAuthFailed, fmt.Sprintf("tongyi authentication failed (%d): %s", status, message), imageprovider.RetryNever, status, nil)
	case http.StatusPaymentRequired:
		return imageprovider.NewProviderError(imageprovider.ErrCodeInsufficientBalance, fmt.Sprintf("tongyi balance unavailable (%d): %s", status, message), imageprovider.RetryNever, status, nil)
	case http.StatusTooManyRequests:
		return imageprovider.NewProviderError(imageprovider.ErrCodeRateLimited, fmt.Sprintf("tongyi rate limited (%d): %s", status, message), imageprovider.RetryAfterDelay, status, nil)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return imageprovider.NewProviderError(imageprovider.ErrCodeRequestInvalid, fmt.Sprintf("tongyi rejected request (%d): %s", status, message), imageprovider.RetryNever, status, nil)
	default:
		return imageprovider.NewProviderError(imageprovider.ErrCodeServiceError, fmt.Sprintf("tongyi request failed (%d): %s", status, message), imageprovider.RetryAfterDelay, status, nil)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func Register(registry *imageprovider.Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	return registry.RegisterWithDescriptor(ProviderName, New(), imageprovider.ProviderDescriptor{
		Name:           ProviderName,
		DisplayName:    "阿里通义万相",
		DefaultModel:   DefaultModel,
		DefaultBaseURL: DefaultBaseURL,
		SupportedModes: []imageprovider.GenerationMode{imageprovider.ModeSingleFrame},
	})
}

var _ imageprovider.ExtendedProvider = (*Adapter)(nil)
var _ imageprovider.ConnectionProbe = (*Adapter)(nil)
