// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package seedream

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/imageprovider"
)

func validConfig(baseURL string) imageprovider.ImageModelConfig {
	return imageprovider.ImageModelConfig{
		Name:      "test",
		ApiKey:    "sk-test",
		ModelName: DefaultModel,
		BaseUrl:   baseURL,
	}
}

func makeTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestAdapter_ValidateConfig_Success(t *testing.T) {
	a := New()
	cases := []string{
		"https://ark.cn-beijing.volces.com/api/v3",
		"http://localhost:8080",
		"https://example.com/",
	}
	for _, baseURL := range cases {
		t.Run(baseURL, func(t *testing.T) {
			if err := a.ValidateConfig(context.Background(), validConfig(baseURL)); err != nil {
				t.Fatalf("ValidateConfig(%q): %v", baseURL, err)
			}
		})
	}
}

func TestAdapter_ValidateConfig_Failures(t *testing.T) {
	a := New()
	cases := []struct {
		name   string
		config imageprovider.ImageModelConfig
		errMsg string
	}{
		{"empty_api_key", imageprovider.ImageModelConfig{ApiKey: "", ModelName: "m", BaseUrl: "https://x.com"}, "ApiKey"},
		{"whitespace_api_key", imageprovider.ImageModelConfig{ApiKey: "   ", ModelName: "m", BaseUrl: "https://x.com"}, "ApiKey"},
		{"empty_model_name", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "", BaseUrl: "https://x.com"}, "ModelName"},
		{"empty_base_url", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: ""}, "BaseUrl"},
		{"whitespace_base_url", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "   "}, "BaseUrl"},
		{"invalid_scheme_ftp", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "ftp://x.com"}, "协议"},
		{"invalid_scheme_file", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "file:///x"}, "协议"},
		{"no_scheme", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "x.com"}, "协议"},
		{"empty_host", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "https://"}, "主机名"},
		{"invalid_url", imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "://missing-scheme"}, "BaseUrl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := a.ValidateConfig(context.Background(), tc.config)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errMsg)
			}
			if !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tc.errMsg)
			}
		})
	}
}

func TestAdapter_Capabilities_ReturnsExpected(t *testing.T) {
	a := New()
	caps, err := a.Capabilities(context.Background(), validConfig("https://x.com"))
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if !caps.SupportsReferenceImage {
		t.Error("SupportsReferenceImage should be true")
	}
	if caps.SupportsMultipleImages {
		t.Error("SupportsMultipleImages should be false (single reference)")
	}
	if !caps.SupportsNegativePrompt {
		t.Error("SupportsNegativePrompt should be true")
	}
	if !caps.SupportsSeed {
		t.Error("SupportsSeed should be true")
	}
	if caps.SupportsAsyncOperation {
		t.Error("SupportsAsyncOperation should be false (sync provider)")
	}
	if caps.SupportsCancellation {
		t.Error("SupportsCancellation should be false")
	}
	if caps.MaxReferenceImages != MaxReferenceImages {
		t.Fatalf("MaxReferenceImages = %d, want %d", caps.MaxReferenceImages, MaxReferenceImages)
	}
	if caps.MaxOutputImages != MaxOutputImages {
		t.Fatalf("MaxOutputImages = %d, want %d", caps.MaxOutputImages, MaxOutputImages)
	}
}

func TestAdapter_Capabilities_FailsWhenConfigInvalid(t *testing.T) {
	a := New()
	_, err := a.Capabilities(context.Background(), imageprovider.ImageModelConfig{ApiKey: "", ModelName: "", BaseUrl: ""})
	if err == nil {
		t.Fatal("expected error when config invalid")
	}
}

func TestAdapter_Query_NotSupported(t *testing.T) {
	a := New()
	_, err := a.Query(context.Background(), validConfig("https://x.com"), "op-1")
	if err == nil {
		t.Fatal("expected error for Query on sync provider")
	}
	if !strings.Contains(err.Error(), "同步模式") {
		t.Fatalf("error should mention sync mode: %v", err)
	}
}

func TestAdapter_Cancel_NotSupported(t *testing.T) {
	a := New()
	err := a.Cancel(context.Background(), validConfig("https://x.com"), "op-1")
	if err == nil {
		t.Fatal("expected error for Cancel on sync provider")
	}
	if !strings.Contains(err.Error(), "同步模式") {
		t.Fatalf("error should mention sync mode: %v", err)
	}
}

func TestAdapter_Submit_GenerationWithB64ReturnsImage(t *testing.T) {
	pngBytes := makeTestPNG(t)
	b64 := base64.StdEncoding.EncodeToString(pngBytes)

	body := map[string]any{
		"data": []any{
			map[string]any{"b64_json": b64},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}
	srv := newMockSeedreamServer(t, http.StatusOK, body)
	defer srv.Close()

	a := NewWithClient(srv.Client())
	cfg := validConfig(srv.URL)
	cfg.ModelName = "doubao-seedream-5-0"

	submission, err := a.Submit(context.Background(), cfg, imageprovider.ImageGenerationRequest{
		Prompt:      "test prompt",
		OutputCount: 1,
		Width:       512,
		Height:      512,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submission == nil || submission.Result == nil {
		t.Fatal("expected non-nil submission result")
	}
	if submission.Status != "succeeded" {
		t.Fatalf("submission.Status = %q, want succeeded", submission.Status)
	}
	result := submission.Result
	if result.Status != "succeeded" {
		t.Fatalf("result.Status = %q, want succeeded", result.Status)
	}
	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(result.Images))
	}
	if !bytes.Equal(result.Images[0].Bytes, pngBytes) {
		t.Fatalf("image bytes mismatch: got %d bytes, want %d", len(result.Images[0].Bytes), len(pngBytes))
	}
	if result.Usage == nil {
		t.Fatal("expected usage")
	}
	if result.Usage.PromptTokens != 10 || result.Usage.TotalTokens != 15 || result.Usage.ImageCount != 1 {
		t.Fatalf("usage = %+v, want prompt=10 total=15 imageCount=1", result.Usage)
	}
	if result.Provider != ProviderName {
		t.Fatalf("Provider = %q, want %q", result.Provider, ProviderName)
	}
	if result.Model != cfg.ModelName {
		t.Fatalf("Model = %q, want %q", result.Model, cfg.ModelName)
	}
}

func TestAdapter_Submit_EditsWithReferenceImageBytes(t *testing.T) {
	pngBytes := makeTestPNG(t)
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	expectedImageData := "data:image/png;base64," + b64

	var capturedContentType string
	var capturedPath string
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContentType = r.Header.Get("Content-Type")
		capturedPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &capturedBody)
		writeJSON(t, w, http.StatusOK, map[string]any{
			"data": []any{map[string]any{"b64_json": b64}},
		})
	}))
	defer srv.Close()

	a := NewWithClient(srv.Client())
	cfg := validConfig(srv.URL)
	cfg.ModelName = "doubao-seedream-5-0"

	submission, err := a.Submit(context.Background(), cfg, imageprovider.ImageGenerationRequest{
		Prompt:          "edit prompt",
		OutputCount:     1,
		ReferenceImages: []imageprovider.ImageInput{{Bytes: pngBytes, MimeType: "image/png"}},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submission == nil || submission.Result == nil || len(submission.Result.Images) != 1 {
		t.Fatalf("unexpected submission: %+v", submission)
	}
	if capturedContentType != "application/json" {
		t.Fatalf("expected application/json content-type, got %q", capturedContentType)
	}
	if capturedPath != "/images/generations" {
		t.Fatalf("expected endpoint /images/generations, got %q", capturedPath)
	}
	if capturedBody["model"] != "doubao-seedream-5-0" {
		t.Fatalf("model = %v", capturedBody["model"])
	}
	if capturedBody["prompt"] != "edit prompt" {
		t.Fatalf("prompt = %v", capturedBody["prompt"])
	}
	if capturedBody["watermark"] != false {
		t.Fatalf("watermark = %v", capturedBody["watermark"])
	}
	imageField, ok := capturedBody["image"].(string)
	if !ok {
		t.Fatalf("image field missing or not string: %v", capturedBody["image"])
	}
	if imageField != expectedImageData {
		t.Fatalf("image field mismatch:\ngot:  %s\nwant: %s", imageField, expectedImageData)
	}
}

func TestAdapter_Submit_WithURLImageDownloads(t *testing.T) {
	pngBytes := makeTestPNG(t)
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer dlSrv.Close()

	srv := newMockSeedreamServer(t, http.StatusOK, map[string]any{
		"data": []any{map[string]any{"url": dlSrv.URL + "/image.png"}},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	cfg := validConfig(srv.URL)

	submission, err := a.Submit(context.Background(), cfg, imageprovider.ImageGenerationRequest{
		Prompt:      "prompt",
		OutputCount: 1,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(submission.Result.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(submission.Result.Images))
	}
	if !bytes.Equal(submission.Result.Images[0].Bytes, pngBytes) {
		t.Fatalf("downloaded bytes mismatch")
	}
	if !strings.HasPrefix(submission.Result.Images[0].MimeType, "image/png") {
		t.Fatalf("mimeType = %q", submission.Result.Images[0].MimeType)
	}
}

func TestAdapter_Submit_EmptyResultReturnsError(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusOK, map[string]any{
		"data": []any{},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	assertProviderErrorCode(t, err, ErrCodeEmptyResult)
}

func TestAdapter_Submit_MissingDataFieldReturnsEmptyResult(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusOK, map[string]any{
		"usage": map[string]any{},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	if err == nil {
		t.Fatal("expected error when data field missing")
	}
	assertProviderErrorCode(t, err, ErrCodeEmptyResult)
}

func TestAdapter_Submit_AuthFailed(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusUnauthorized, map[string]any{
		"error": map[string]any{"message": "invalid api key"},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
	assertProviderErrorCode(t, err, ErrCodeAuthFailed)
	if !strings.Contains(err.Error(), "鉴权失败") {
		t.Fatalf("error should contain 鉴权失败: %v", err)
	}
}

func TestAdapter_Submit_ForbiddenAlsoAuthFailed(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusForbidden, map[string]any{
		"error": map[string]any{"message": "no permission"},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	assertProviderErrorCode(t, err, ErrCodeAuthFailed)
}

func TestAdapter_Submit_RateLimited(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusTooManyRequests, map[string]any{
		"error": map[string]any{"message": "rate limited"},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	assertProviderErrorCode(t, err, ErrCodeRateLimited)
}

func TestAdapter_Submit_ProviderRejected(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusBadRequest, map[string]any{
		"error": map[string]any{"message": "bad prompt"},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	assertProviderErrorCode(t, err, ErrCodeProviderRejected)
}

func TestAdapter_Submit_ServerErrorRejected(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusInternalServerError, map[string]any{
		"error": map[string]any{"message": "internal"},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	assertProviderErrorCode(t, err, ErrCodeProviderRejected)
}

func TestAdapter_Submit_UnsupportedImageFormat(t *testing.T) {
	badBytes := []byte("not-an-image-data")
	b64 := base64.StdEncoding.EncodeToString(badBytes)
	srv := newMockSeedreamServer(t, http.StatusOK, map[string]any{
		"data": []any{map[string]any{"b64_json": b64}},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	assertProviderErrorCode(t, err, ErrCodeInvalidFormat)
}

func TestAdapter_Submit_MissingUrlAndB64ReturnsInvalidFormat(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusOK, map[string]any{
		"data": []any{map[string]any{"foo": "bar"}},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	assertProviderErrorCode(t, err, ErrCodeInvalidFormat)
}

func TestAdapter_Submit_InvalidBase64ReturnsInvalidFormat(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusOK, map[string]any{
		"data": []any{map[string]any{"b64_json": "!!!not base64!!!"}},
	})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	assertProviderErrorCode(t, err, ErrCodeInvalidFormat)
}

func TestAdapter_Submit_EmptyPromptRejected(t *testing.T) {
	a := New()
	_, err := a.Submit(context.Background(), validConfig("https://x.com"), imageprovider.ImageGenerationRequest{
		Prompt: "   ", OutputCount: 1,
	})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if !strings.Contains(err.Error(), "Prompt") {
		t.Fatalf("error should mention Prompt: %v", err)
	}
}

func TestAdapter_Submit_TooManyReferenceImagesRejected(t *testing.T) {
	a := New()
	png := makeTestPNG(t)
	refs := []imageprovider.ImageInput{
		{Bytes: png, MimeType: "image/png"},
		{Bytes: png, MimeType: "image/png"},
	}
	_, err := a.Submit(context.Background(), validConfig("https://x.com"), imageprovider.ImageGenerationRequest{
		Prompt:          "p",
		OutputCount:     1,
		ReferenceImages: refs,
	})
	if err == nil {
		t.Fatal("expected error for too many references")
	}
	if !strings.Contains(err.Error(), "参考图数量") {
		t.Fatalf("error should mention 参考图数量: %v", err)
	}
}

func TestAdapter_Submit_ReferenceImageBytesEmptyRejected(t *testing.T) {
	srv := newMockSeedreamServer(t, http.StatusOK, map[string]any{"data": []any{}})
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt:          "p",
		OutputCount:     1,
		ReferenceImages: []imageprovider.ImageInput{{Bytes: nil, MimeType: "image/png"}},
	})
	if err == nil {
		t.Fatal("expected error for empty reference bytes")
	}
	if !strings.Contains(err.Error(), "参考图内容为空") {
		t.Fatalf("error should mention empty reference: %v", err)
	}
}

func TestAdapter_Submit_ReferenceImageTooLargeRejected(t *testing.T) {
	a := New()
	big := make([]byte, MaxImageSizeBytes+1)
	for i := range big {
		big[i] = 0x89
	}
	_, err := a.Submit(context.Background(), validConfig("https://x.com"), imageprovider.ImageGenerationRequest{
		Prompt:          "p",
		OutputCount:     1,
		ReferenceImages: []imageprovider.ImageInput{{Bytes: big, MimeType: "image/png"}},
	})
	if err == nil {
		t.Fatal("expected error for oversized reference")
	}
	if !strings.Contains(err.Error(), "参考图大小超过限制") {
		t.Fatalf("error should mention size limit: %v", err)
	}
}

func TestAdapter_Submit_ValidateConfigFailureShortCircuits(t *testing.T) {
	a := New()
	_, err := a.Submit(context.Background(), imageprovider.ImageModelConfig{ApiKey: "", ModelName: "", BaseUrl: ""}, imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAdapter_Submit_OutputCountClampedToMax(t *testing.T) {
	var capturedN string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "\"n\":") {
				start := strings.Index(string(body), "\"n\":")
				capturedN = string(body[start : start+20])
			}
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"data": []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString(makeTestPNG(t))}},
		})
	}))
	defer srv.Close()

	a := NewWithClient(srv.Client())
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt:      "p",
		OutputCount: 100,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !strings.Contains(capturedN, "4") {
		t.Fatalf("expected n clamped to %d, captured body snippet: %q", MaxOutputImages, capturedN)
	}
}

func TestAdapter_Submit_ContextTimeoutReturnsTimeoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		writeJSON(t, w, http.StatusOK, map[string]any{"data": []any{}})
	}))
	defer srv.Close()

	a := NewWithClient(&http.Client{Timeout: 50 * time.Millisecond})
	_, err := a.Submit(context.Background(), validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	assertProviderErrorCode(t, err, ErrCodeTimeout)
}

func TestAdapter_Submit_AsyncContextCancelledReturnsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		writeJSON(t, w, http.StatusOK, map[string]any{"data": []any{}})
	}))
	defer srv.Close()

	a := NewWithClient(srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := a.Submit(ctx, validConfig(srv.URL), imageprovider.ImageGenerationRequest{
		Prompt: "p", OutputCount: 1,
	})
	if err == nil {
		t.Fatal("expected error due to context deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		var pe *ProviderError
		if !errors.As(err, &pe) || (pe.Code != ErrCodeTimeout && pe.Code != ErrCodeProviderRejected) {
			t.Fatalf("expected ProviderError (timeout or rejected), got %T: %v", err, err)
		}
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := imageprovider.NewRegistry()
	if _, ok := r.Get(ProviderName); ok {
		t.Fatal("expected provider not registered initially")
	}
	Register(r)
	p, ok := r.Get(ProviderName)
	if !ok {
		t.Fatal("expected provider registered after Register")
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	names := r.Names()
	if len(names) != 1 || names[0] != ProviderName {
		t.Fatalf("Names = %v, want [%s]", names, ProviderName)
	}
}

func TestRegistry_RegisterNilIgnored(t *testing.T) {
	r := imageprovider.NewRegistry()
	Register(nil)
	if _, ok := r.Get(ProviderName); ok {
		t.Fatal("nil registry should not register provider")
	}
}

func TestProviderError_ErrorAndCode(t *testing.T) {
	cause := errors.New("root cause")
	pe := &ProviderError{Code: ErrCodeTimeout, Message: "msg", Cause: cause}
	if pe.Code != ErrCodeTimeout {
		t.Fatalf("Code = %q", pe.Code)
	}
	if !errors.Is(pe, cause) {
		t.Fatalf("expected errors.Is to unwrap cause")
	}
	if !strings.Contains(pe.Error(), ErrCodeTimeout) || !strings.Contains(pe.Error(), "msg") {
		t.Fatalf("Error() = %q", pe.Error())
	}

	peNoCause := &ProviderError{Code: ErrCodeAuthFailed, Message: "no cause"}
	if errors.Unwrap(peNoCause) != nil {
		t.Fatal("expected nil unwrap when no cause")
	}
	if !strings.Contains(peNoCause.Error(), ErrCodeAuthFailed) {
		t.Fatalf("Error() should contain code: %q", peNoCause.Error())
	}
}

func TestBuildEndpoint_DefaultAndCustom(t *testing.T) {
	if got := buildEndpoint("", "/x"); got != DefaultBaseURL+"/x" {
		t.Fatalf("buildEndpoint empty = %q, want %q", got, DefaultBaseURL+"/x")
	}
	if got := buildEndpoint("https://x.com/", "/y"); got != "https://x.com/y" {
		t.Fatalf("buildEndpoint trailing slash = %q, want https://x.com/y", got)
	}
	if got := buildEndpoint("  https://x.com  ", "/z"); got != "https://x.com/z" {
		t.Fatalf("buildEndpoint with whitespace = %q", got)
	}
}

func TestBuildSize_Formats(t *testing.T) {
	cases := []struct {
		w, h     int
		expected string
	}{
		{0, 0, ""},
		{512, 512, "512x512"},
		{0, 768, "1024x768"},
		{768, 0, "768x1024"},
	}
	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			req := imageprovider.ImageGenerationRequest{Width: tc.w, Height: tc.h}
			if got := buildSize(req); got != tc.expected {
				t.Fatalf("buildSize(%d,%d) = %q, want %q", tc.w, tc.h, got, tc.expected)
			}
		})
	}
}

func TestMapHTTPError_AllStatuses(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{http.StatusUnauthorized, ErrCodeAuthFailed},
		{http.StatusForbidden, ErrCodeAuthFailed},
		{http.StatusTooManyRequests, ErrCodeRateLimited},
		{http.StatusBadRequest, ErrCodeProviderRejected},
		{http.StatusUnprocessableEntity, ErrCodeProviderRejected},
		{http.StatusInternalServerError, ErrCodeProviderRejected},
		{http.StatusBadGateway, ErrCodeProviderRejected},
		{http.StatusNotImplemented, ErrCodeProviderRejected},
		{http.StatusGone, ErrCodeRequestInvalid},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			pe := mapHTTPError(tc.status, nil, []byte("raw body"))
			if pe == nil {
				t.Fatal("expected non-nil ProviderError")
			}
			if pe.Code != tc.code {
				t.Fatalf("status %d code = %q, want %q", tc.status, pe.Code, tc.code)
			}
		})
	}
}

func TestMapHTTPError_ExtractsErrorMessage(t *testing.T) {
	body := map[string]any{
		"error": map[string]any{"message": "explicit error message"},
	}
	pe := mapHTTPError(http.StatusBadRequest, body, []byte("raw"))
	if !strings.Contains(pe.Message, "explicit error message") {
		t.Fatalf("expected message extracted, got %q", pe.Message)
	}
}

func TestTruncateText(t *testing.T) {
	short := "abc"
	if got := truncateText(short, 10); got != short {
		t.Fatalf("truncateText short = %q", got)
	}
	long := strings.Repeat("a", 20)
	got := truncateText(long, 5)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ... suffix, got %q", got)
	}
	if len([]rune(got)) != 8 {
		t.Fatalf("truncated length = %d, want 8", len([]rune(got)))
	}
}

func TestAsInt64(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want int64
	}{
		{"float64", float64(42), 42},
		{"int", 42, 42},
		{"int64", int64(42), 42},
		{"nil", nil, 0},
		{"string", "abc", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := asInt64(tc.v); got != tc.want {
				t.Fatalf("asInt64(%v) = %d, want %d", tc.v, got, tc.want)
			}
		})
	}
}

func newMockSeedreamServer(t *testing.T, status int, body map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, status, body)
	}))
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", "req-mock-1")
	w.WriteHeader(status)
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	_, _ = w.Write(data)
}

func assertProviderErrorCode(t *testing.T, err error, expected string) {
	t.Helper()
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ProviderError, got %T: %v", err, err)
	}
	if pe.Code != expected {
		t.Fatalf("ProviderError.Code = %q, want %q (msg=%q)", pe.Code, expected, pe.Message)
	}
}

var _ = io.EOF
