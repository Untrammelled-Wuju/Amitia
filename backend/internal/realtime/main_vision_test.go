package realtime

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

func TestMainVisionRealtimeUsesSelectedProtocol(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/messages" {
			t.Errorf("explicit protocol ignored: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"type":"text","text":"screen recognized"}]}`)
	}))
	defer server.Close()
	svc := mainVisionService{cfg: &vision.VisionConfig{Protocol: "anthropic_messages", ApiType: "openai", ApiKey: "test-key", ModelName: "main-model", BaseUrl: server.URL, FromMainModel: true}}
	analyzer := NewConfiguredVisualAnalyzer(svc)
	if !analyzer.Available() {
		t.Fatal("main vision should be available")
	}
	for _, source := range []VisualSourceType{VisualSourceScreen, VisualSourceCamera} {
		text, err := analyzer.Analyze(context.Background(), VisualFrame{Data: []byte("image"), MIME: "image/png", Source: source})
		if err != nil || text != "screen recognized" {
			t.Fatalf("realtime recognition failed: %q %v", text, err)
		}
	}
	if calls != 2 {
		t.Fatalf("frame calls=%d", calls)
	}
}
