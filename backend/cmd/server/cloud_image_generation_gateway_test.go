package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/cloudbridge"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type gatewayConfigRepository struct {
	config imagegen.ImageGenConfig
}

func (r *gatewayConfigRepository) List() ([]imagegen.ImageGenConfig, error) {
	return []imagegen.ImageGenConfig{r.config}, nil
}
func (r *gatewayConfigRepository) GetByID(id int) (*imagegen.ImageGenConfig, error) {
	if id != r.config.ID {
		return nil, errors.New("config not found")
	}
	copy := r.config
	return &copy, nil
}
func (r *gatewayConfigRepository) Create(*imagegen.ImageGenConfig) error {
	return errors.New("not implemented")
}
func (r *gatewayConfigRepository) Update(int, map[string]interface{}) error {
	return errors.New("not implemented")
}
func (r *gatewayConfigRepository) Delete(int) error {
	return errors.New("not implemented")
}
func (r *gatewayConfigRepository) Activate(int) error {
	return errors.New("not implemented")
}
func (r *gatewayConfigRepository) GetActive() (*imagegen.ImageGenConfig, error) {
	copy := r.config
	return &copy, nil
}

type gatewayProvider struct {
	config imageprovider.ImageModelConfig
}

func (p *gatewayProvider) ValidateConfig(context.Context, imageprovider.ImageModelConfig) error {
	return nil
}
func (p *gatewayProvider) Capabilities(context.Context, imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	return imageprovider.ImageGenerationCapabilities{MaxOutputImages: 1}, nil
}
func (p *gatewayProvider) Submit(_ context.Context, config imageprovider.ImageModelConfig, _ imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	p.config = config
	return &imageprovider.ImageGenerationSubmission{
		Status: "succeeded",
		Result: &imageprovider.ImageGenerationResult{
			Provider: "gateway",
			Images:   []imageprovider.GeneratedImage{{Bytes: []byte{1, 2, 3}, MimeType: "image/png"}},
		},
	}, nil
}
func (p *gatewayProvider) Query(context.Context, imageprovider.ImageModelConfig, string) (*imageprovider.ImageGenerationResult, error) {
	return nil, errors.New("not implemented")
}
func (p *gatewayProvider) Cancel(context.Context, imageprovider.ImageModelConfig, string) error {
	return errors.New("not implemented")
}

func TestCloudImageGenerationGatewayUsesCloudActiveConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api", func(c *gin.Context) {
		actor := &auth.ActorContext{
			PrincipalType: auth.PrincipalDeviceRuntime,
			SpaceID:       runtimeidentity.SpaceID("space"),
			DeviceID:      runtimeidentity.DeviceID("device"),
		}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	provider := &gatewayProvider{}
	registry := imageprovider.NewRegistry()
	if err := registry.Register("gateway", provider); err != nil {
		t.Fatal(err)
	}
	repo := &gatewayConfigRepository{config: imagegen.ImageGenConfig{ID: 4, Name: "cloud", ApiType: "gateway", ApiKey: "secret", ModelName: "model", BaseUrl: "https://provider.example", Enabled: 1}}
	services := &AppServices{
		RuntimeProfile: runtimeprofile.ProfileCloudCore,
		ImageGen:       imagegen.NewService(repo, registry),
		ImageProviders: registry,
	}
	registerCloudImageGenerationGateway(group, services)
	body, err := json.Marshal(cloudbridge.Request{
		Operation: cloudbridge.OperationSubmit,
		ConfigID:  0,
		GenerationRequest: &imageprovider.ImageGenerationRequest{
			Prompt: "tree",
			Width:  1024,
			Height: 1024,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api"+cloudbridge.EndpointPath, strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if provider.config.ConfigID != 4 || provider.config.ApiKey != "secret" {
		t.Fatalf("unexpected provider config: %#v", provider.config)
	}
	var result cloudbridge.Response
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Submission == nil || result.Submission.Result == nil || len(result.Submission.Result.Images) != 1 {
		t.Fatalf("unexpected response: %#v", result)
	}
}
