package extension_center

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKernelCardProviderReturnsEmptyWithoutRepositories(t *testing.T) {
	provider := NewKernelCardProvider(nil, nil)
	cards, err := provider.ListCards(context.Background())
	if err != nil {
		t.Fatalf("ListCards returned error: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("expected empty cards, got %d", len(cards))
	}
}

type failingCardProvider struct{}

func (failingCardProvider) ListCards(context.Context) ([]ExtensionCard, error) {
	return nil, errors.New("test failure")
}

func TestExtensionCenterHandlerReturnsEmptyViewOnProviderError(t *testing.T) {
	handler := NewHTTPHandler(NewCenterService(failingCardProvider{}))
	mux := http.NewServeMux()
	handler.Register(mux)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/extension-center/view", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var view CenterView
	if err := json.NewDecoder(recorder.Body).Decode(&view); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(view.Installed)+len(view.Discover)+len(view.Updates)+len(view.NeedsAction) != 0 {
		t.Fatalf("expected empty view")
	}
}
