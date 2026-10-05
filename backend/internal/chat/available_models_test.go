package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

type availableModelService struct {
	Service
	models []ModelConfig
}

func (s availableModelService) ListModels() ([]ModelConfig, error) {
	return s.models, nil
}

func TestAvailableModelsExposeServiceWithoutCoreConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(availableModelService{models: []ModelConfig{
		{ID: 7, IsActive: 1, APIType: "openai", ModelName: "active-model", APIKey: "test-secret", BaseURL: "https://private-provider.invalid", ProviderConfigJSON: `{"private":"value"}`},
		{ID: 8, IsActive: 0, APIType: "openai", ModelName: "inactive-model"},
	}})
	r := gin.New()
	r.GET("/available", func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{})
		handler.AvailableModels(c)
	})
	result := httptest.NewRecorder()
	r.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/available", nil))
	if result.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
	for _, forbidden := range []string{"test-secret", "private-provider", "inactive-model", "providerConfig", "temperature", "baseUrl", "apiKey"} {
		if strings.Contains(result.Body.String(), forbidden) {
			t.Fatalf("configuration leaked: %s", forbidden)
		}
	}
	var response struct {
		Data []struct {
			ID      int  `json:"id"`
			Managed bool `json:"managedByCore"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil || len(response.Data) != 1 || response.Data[0].ID != 0 || !response.Data[0].Managed {
		t.Fatalf("service-only response: %s %v", result.Body.String(), err)
	}
}

func TestAvailableModelsRequireAuthenticatedActor(t *testing.T) {
	r := gin.New()
	r.GET("/available", NewHandler(availableModelService{}).AvailableModels)
	result := httptest.NewRecorder()
	r.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/available", nil))
	if result.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", result.Code)
	}
}
