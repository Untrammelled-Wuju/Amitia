package imageintelligence

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/u-ai/backend/internal/vision"
)

type mainVisionService struct {
	vision.Service
	cfg *vision.VisionConfig
}

func (s mainVisionService) GetActive() (*vision.VisionConfig, error) { return s.cfg, nil }

func TestMainVisionUnderstandAndOCR(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/chat/completions" {
			t.Errorf("wrong main model protocol: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"identified text"}}]}`)
	}))
	defer server.Close()
	svc := mainVisionService{cfg: &vision.VisionConfig{Protocol: "openai_chat", ApiType: "openai", ApiKey: "test-key", ModelName: "main-model", BaseUrl: server.URL, FromMainModel: true}}
	summary := ImageInputSummary{MIME: "image/png"}
	understood, understandErr := NewUnderstandProvider(svc).Understand(context.Background(), ImageUnderstandRequest{}, []byte("image"), summary)
	if understandErr != nil || understood.Text != "identified text" || understood.Model != "main-model" {
		t.Fatalf("image understanding failed: %v %v", understood, understandErr)
	}
	ocr, ocrErr := NewOCRProvider(svc).OCR(context.Background(), ImageOCRRequest{}, []byte("image"), summary)
	if ocrErr != nil || ocr.Text != "identified text" || requests != 2 {
		t.Fatalf("OCR failed: %v %v, calls=%d", ocr, ocrErr, requests)
	}
}
