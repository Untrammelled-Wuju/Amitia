package stability

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/imageprovider"
)

func TestSubmitUsesStabilityMultipartProtocol(t *testing.T) {
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/stable-image/generate/core" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization: %s", r.Header.Get("Authorization"))
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("prompt") != "draw a tree" || r.FormValue("aspect_ratio") != "16:9" || r.FormValue("output_format") != "png" {
			t.Fatalf("unexpected form: %#v", r.MultipartForm.Value)
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(pngData); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	adapter := New()
	submission, err := adapter.Submit(context.Background(), imageprovider.ImageModelConfig{
		ApiKey:    "test-key",
		ModelName: DefaultModel,
		BaseUrl:   server.URL,
	}, imageprovider.ImageGenerationRequest{
		Prompt: "draw a tree",
		Width:  1280,
		Height: 720,
	})
	if err != nil {
		t.Fatal(err)
	}
	if submission.Result == nil || len(submission.Result.Images) != 1 {
		t.Fatalf("unexpected submission: %#v", submission)
	}
	if submission.Result.Provider != ProviderName || submission.Result.Images[0].MimeType != "image/png" {
		t.Fatalf("unexpected result: %#v", submission.Result)
	}
}

func TestSubmitPreservesStabilityErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := io.WriteString(w, `{"errors":["prompt rejected"]}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	_, err := New().Submit(context.Background(), imageprovider.ImageModelConfig{
		ApiKey:  "test-key",
		BaseUrl: server.URL,
	}, imageprovider.ImageGenerationRequest{Prompt: "x", Width: 1024, Height: 1024})
	if err == nil || !strings.Contains(err.Error(), "prompt rejected") {
		t.Fatalf("expected original provider error, got %v", err)
	}
}
