package imageintelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type routerTestService struct {
	request ImageGenerateRequest
}

func (s *routerTestService) Understand(context.Context, ImageUnderstandRequest) (*ImageUnderstandResult, error) {
	return nil, nil
}
func (s *routerTestService) OCR(context.Context, ImageOCRRequest) (*ImageOCRResult, error) {
	return nil, nil
}
func (s *routerTestService) Generate(_ context.Context, request ImageGenerateRequest) (*ImageGenerateResult, error) {
	s.request = request
	return &ImageGenerateResult{Provider: "test", Images: []GeneratedImage{{ResourceURI: "amitia://artifacts/test", MIMEType: "image/png", Width: 1, Height: 1, SizeBytes: 3}}}, nil
}
func (s *routerTestService) Status(context.Context) ImageIntelligenceStatus {
	return ImageIntelligenceStatus{Capabilities: ImageCapabilities{Generate: true}, Healthy: true}
}

func TestGenerateRouterUsesAuthenticatedSpaceAsOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api", func(c *gin.Context) {
		actor := &auth.ActorContext{PrincipalType: auth.PrincipalDeviceRuntime, SpaceID: runtimeidentity.SpaceID("space-a")}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	service := &routerTestService{}
	RegisterRouter(group, service)
	body, err := json.Marshal(ImageGenerateRequest{Prompt: "tree", Width: 1024, Height: 1024})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/image-intelligence/generate", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.request.OwnerSpaceID != "space-a" || service.request.Prompt != "tree" {
		t.Fatalf("unexpected request: %#v", service.request)
	}
}
