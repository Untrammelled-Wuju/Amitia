// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package seedream

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/imageprovider"
)

const (
	ProviderName           = "seedream"
	DefaultModel           = "doubao-seedream-5-0"
	DefaultBaseURL         = "https://ark.cn-beijing.volces.com/api/v3"
	DefaultTimeout         = 120 * time.Second
	DefaultDownloadTimeout = 60 * time.Second
	MaxReferenceImages     = 1
	MaxOutputImages        = 4
	MaxImageSizeBytes      = 10 * 1024 * 1024

	ErrCodeAuthFailed            = "IMAGE_GENERATION_AUTH_FAILED"
	ErrCodeRateLimited           = "IMAGE_GENERATION_RATE_LIMITED"
	ErrCodeProviderRejected      = "IMAGE_GENERATION_PROVIDER_REJECTED"
	ErrCodeTimeout               = "IMAGE_GENERATION_TIMEOUT"
	ErrCodeRequestInvalid        = "IMAGE_GENERATION_REQUEST_INVALID"
	ErrCodeEmptyResult           = "IMAGE_GENERATION_EMPTY_RESULT"
	ErrCodeDownloadFailed        = "IMAGE_RESULT_DOWNLOAD_FAILED"
	ErrCodeInvalidFormat         = "IMAGE_RESULT_INVALID_FORMAT"
	ErrCodeCapabilityUnsupported = "IMAGE_MODEL_CAPABILITY_UNSUPPORTED"
	ErrCodeCancelled             = "IMAGE_GENERATION_CANCELLED"
)

type Adapter struct {
	httpClient *http.Client
}

func New() *Adapter {
	return &Adapter{httpClient: &http.Client{Timeout: DefaultTimeout}}
}

func NewWithClient(client *http.Client) *Adapter {
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	return &Adapter{httpClient: client}
}

func (a *Adapter) ValidateConfig(ctx context.Context, config imageprovider.ImageModelConfig) error {
	if strings.TrimSpace(config.ApiKey) == "" {
		return errors.New("ApiKey 不能为空")
	}
	if strings.TrimSpace(config.ModelName) == "" {
		return errors.New("ModelName 不能为空")
	}
	baseURL := strings.TrimSpace(config.BaseUrl)
	if baseURL == "" {
		return errors.New("BaseUrl 不能为空")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("BaseUrl 无法解析: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("BaseUrl 协议必须是 http 或 https")
	}
	if u.Host == "" {
		return errors.New("BaseUrl 缺少主机名")
	}
	return nil
}

func (a *Adapter) Capabilities(ctx context.Context, config imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	if err := a.ValidateConfig(ctx, config); err != nil {
		return imageprovider.ImageGenerationCapabilities{}, err
	}
	return imageprovider.ImageGenerationCapabilities{
		SupportsReferenceImage: true,
		SupportsMultipleImages: false,
		SupportsNegativePrompt: true,
		SupportsSeed:           true,
		SupportsAsyncOperation: false,
		SupportsCancellation:   false,
		MaxReferenceImages:     MaxReferenceImages,
		MaxOutputImages:        MaxOutputImages,
	}, nil
}

func (a *Adapter) Submit(ctx context.Context, config imageprovider.ImageModelConfig, request imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	if err := a.ValidateConfig(ctx, config); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, errors.New("Prompt 不能为空")
	}
	if len(request.ReferenceImages) > MaxReferenceImages {
		return nil, fmt.Errorf("参考图数量超过限制: 最大 %d", MaxReferenceImages)
	}

	outputCount := request.OutputCount
	if outputCount <= 0 {
		outputCount = 1
	}
	if outputCount > MaxOutputImages {
		outputCount = MaxOutputImages
	}

	result, err := a.submitGeneration(ctx, config, request, outputCount)
	if err != nil {
		return nil, err
	}
	return &imageprovider.ImageGenerationSubmission{
		Status:      "succeeded",
		OperationID: result.OperationID,
		RequestID:   result.RequestID,
		Result:      result,
	}, nil
}

func (a *Adapter) Query(ctx context.Context, config imageprovider.ImageModelConfig, operationID string) (*imageprovider.ImageGenerationResult, error) {
	return nil, errors.New("Seedream 适配器为同步模式, 不支持异步查询")
}

func (a *Adapter) Cancel(ctx context.Context, config imageprovider.ImageModelConfig, operationID string) error {
	return errors.New("Seedream 适配器为同步模式, 不支持取消操作")
}

func (a *Adapter) submitGeneration(ctx context.Context, config imageprovider.ImageModelConfig, request imageprovider.ImageGenerationRequest, outputCount int) (*imageprovider.ImageGenerationResult, error) {
	body := map[string]any{
		"model":           config.ModelName,
		"prompt":          request.Prompt,
		"response_format": "url",
		"n":               outputCount,
		"watermark":       false,
	}
	if request.NegativePrompt != "" {
		body["negative_prompt"] = request.NegativePrompt
	}
	if size := buildSize(request); size != "" {
		body["size"] = size
	}
	if request.Seed != nil {
		body["seed"] = *request.Seed
	}
	if len(request.ReferenceImages) > 0 {
		imageData, err := buildReferenceImageBody(request.ReferenceImages[0])
		if err != nil {
			return nil, err
		}
		body["image"] = imageData
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("构建请求体失败: %w", err)
	}
	endpoint := buildEndpoint(config.BaseUrl, "/images/generations")
	resp, requestID, err := a.doRequest(ctx, config, endpoint, "application/json", jsonBody)
	if err != nil {
		return nil, err
	}
	return a.parseResponse(ctx, resp, requestID, config)
}

func buildReferenceImageBody(ref imageprovider.ImageInput) (string, error) {
	imageBytes := ref.Bytes
	if len(imageBytes) == 0 && ref.Path != "" {
		data, err := os.ReadFile(ref.Path)
		if err != nil {
			return "", fmt.Errorf("读取参考图失败: %w", err)
		}
		imageBytes = data
	}
	if len(imageBytes) == 0 {
		return "", errors.New("参考图内容为空")
	}
	if len(imageBytes) > MaxImageSizeBytes {
		return "", fmt.Errorf("参考图大小超过限制: 最大 %d 字节", MaxImageSizeBytes)
	}
	mimeType := ref.MimeType
	if mimeType == "" {
		mimeType = http.DetectContentType(imageBytes)
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(imageBytes), nil
}

func (a *Adapter) doRequest(ctx context.Context, config imageprovider.ImageModelConfig, endpoint, contentType string, body []byte) (map[string]any, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, "", &ProviderError{Code: ErrCodeTimeout, Message: "请求超时", Cause: err}
		}
		return nil, "", &ProviderError{Code: ErrCodeProviderRejected, Message: fmt.Sprintf("请求失败: %v", err), Cause: err}
	}
	defer resp.Body.Close()

	requestID := resp.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = resp.Header.Get("Request-ID")
	}

	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, requestID, mapHTTPError(resp.StatusCode, parsed, raw)
	}
	return parsed, requestID, nil
}

func (a *Adapter) parseResponse(ctx context.Context, body map[string]any, requestID string, config imageprovider.ImageModelConfig) (*imageprovider.ImageGenerationResult, error) {
	result := &imageprovider.ImageGenerationResult{
		Provider:    ProviderName,
		Model:       config.ModelName,
		RequestID:   requestID,
		RawMetadata: body,
		Status:      "succeeded",
	}
	dataRaw, ok := body["data"]
	if !ok {
		result.Status = "failed"
		result.ErrorCode = ErrCodeEmptyResult
		result.ErrorMessage = "模型返回为空"
		return result, &ProviderError{Code: ErrCodeEmptyResult, Message: "模型返回为空"}
	}
	dataArr, _ := dataRaw.([]any)
	if len(dataArr) == 0 {
		result.Status = "failed"
		result.ErrorCode = ErrCodeEmptyResult
		result.ErrorMessage = "模型未返回任何图片"
		return result, &ProviderError{Code: ErrCodeEmptyResult, Message: "模型未返回任何图片"}
	}
	images := make([]imageprovider.GeneratedImage, 0, len(dataArr))
	for _, item := range dataArr {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		img, err := parseImageItem(ctx, itemMap, a.httpClient)
		if err != nil {
			result.Status = "failed"
			result.ErrorCode = errorCodeOf(err)
			result.ErrorMessage = err.Error()
			return result, err
		}
		images = append(images, img)
	}
	if len(images) == 0 {
		result.Status = "failed"
		result.ErrorCode = ErrCodeEmptyResult
		result.ErrorMessage = "未能解析到任何图片"
		return result, &ProviderError{Code: ErrCodeEmptyResult, Message: "未能解析到任何图片"}
	}
	result.Images = images
	if usageRaw, ok := body["usage"]; ok {
		if usageMap, ok := usageRaw.(map[string]any); ok {
			usage := &imageprovider.GenerationUsage{
				ImageCount: len(images),
				Raw:        usageMap,
			}
			usage.PromptTokens = asInt64(usageMap["prompt_tokens"])
			usage.CompletionTokens = asInt64(usageMap["completion_tokens"])
			usage.TotalTokens = asInt64(usageMap["total_tokens"])
			result.Usage = usage
		}
	}
	return result, nil
}

func parseImageItem(ctx context.Context, item map[string]any, client *http.Client) (imageprovider.GeneratedImage, error) {
	if urlStr, ok := item["url"].(string); ok && urlStr != "" {
		dlClient := client
		if dlClient == nil {
			dlClient = &http.Client{Timeout: DefaultDownloadTimeout}
		}
		dlCtx, cancel := context.WithTimeout(ctx, DefaultDownloadTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, urlStr, nil)
		if err != nil {
			return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeDownloadFailed, Message: fmt.Sprintf("构建下载请求失败: %v", err)}
		}
		resp, err := dlClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeTimeout, Message: "下载图片超时"}
			}
			return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeDownloadFailed, Message: fmt.Sprintf("下载图片失败: %v", err)}
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeDownloadFailed, Message: fmt.Sprintf("下载图片返回 %d", resp.StatusCode)}
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeDownloadFailed, Message: fmt.Sprintf("读取图片内容失败: %v", err)}
		}
		mime := resp.Header.Get("Content-Type")
		if mime == "" {
			mime = http.DetectContentType(data)
		}
		return buildImage(data, mime, item)
	}
	if b64, ok := item["b64_json"].(string); ok && b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeInvalidFormat, Message: fmt.Sprintf("Base64 解码失败: %v", err)}
		}
		mime := http.DetectContentType(data)
		return buildImage(data, mime, item)
	}
	return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeInvalidFormat, Message: "图片条目缺少 url 或 b64_json 字段"}
}

func buildImage(data []byte, mime string, item map[string]any) (imageprovider.GeneratedImage, error) {
	if !strings.HasPrefix(mime, "image/") {
		return imageprovider.GeneratedImage{}, &ProviderError{Code: ErrCodeInvalidFormat, Message: fmt.Sprintf("返回内容不是图片: %s", mime)}
	}
	return imageprovider.GeneratedImage{
		Bytes:    data,
		MimeType: mime,
		Metadata: item,
	}, nil
}

func buildEndpoint(baseURL, path string) string {
	b := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if b == "" {
		b = DefaultBaseURL
	}
	return b + path
}

func buildSize(request imageprovider.ImageGenerationRequest) string {
	w := request.Width
	h := request.Height
	if w <= 0 && h <= 0 {
		return ""
	}
	if w <= 0 {
		w = 1024
	}
	if h <= 0 {
		h = 1024
	}
	return fmt.Sprintf("%dx%d", w, h)
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

func mapHTTPError(status int, body map[string]any, raw []byte) *ProviderError {
	msg := truncateText(string(raw), 300)
	if e, ok := body["error"].(map[string]any); ok {
		if m, ok := e["message"].(string); ok && m != "" {
			msg = m
		}
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ProviderError{Code: ErrCodeAuthFailed, Message: fmt.Sprintf("鉴权失败 (%d): %s", status, msg)}
	case http.StatusTooManyRequests:
		return &ProviderError{Code: ErrCodeRateLimited, Message: fmt.Sprintf("请求被限流 (%d): %s", status, msg)}
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return &ProviderError{Code: ErrCodeProviderRejected, Message: fmt.Sprintf("模型拒绝请求 (%d): %s", status, msg)}
	default:
		if status >= 500 {
			return &ProviderError{Code: ErrCodeProviderRejected, Message: fmt.Sprintf("服务端错误 (%d): %s", status, msg)}
		}
		return &ProviderError{Code: ErrCodeRequestInvalid, Message: fmt.Sprintf("请求无效 (%d): %s", status, msg)}
	}
}

func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func errorCodeOf(err error) string {
	var perr *ProviderError
	if errors.As(err, &perr) {
		return perr.Code
	}
	return ErrCodeProviderRejected
}

type ProviderError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ProviderError) Unwrap() error { return e.Cause }

func (e *ProviderError) ErrorCode() string { return e.Code }

func Register(registry *imageprovider.Registry) {
	if registry == nil {
		return
	}
	registry.Register(ProviderName, New())
}
