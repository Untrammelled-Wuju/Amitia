package imageintelligence

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/seedream"
)

type testArtifactCreator struct {
	created artifact.CreateRequest
}

func (c *testArtifactCreator) Create(_ context.Context, req artifact.CreateRequest) (artifact.Artifact, error) {
	c.created = req
	return artifact.Artifact{
		ID:        "art_generated",
		Kind:      req.Kind,
		MIMEType:  req.MIMEType,
		Filename:  req.Filename,
		SizeBytes: int64(len(req.Filename)),
	}, nil
}

func TestResourceifyImagePersistsArtifact(t *testing.T) {
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	creator := &testArtifactCreator{}
	provider := NewGenerateProvider(nil, nil, nil, creator)
	image, genErr := provider.resourceifyImage(context.Background(), "space_test", imageprovider.GeneratedImage{
		Bytes:    pngData,
		MimeType: "image/png",
		Width:    1,
		Height:   1,
	})
	if genErr != nil {
		t.Fatal(genErr)
	}
	if image.ResourceURI != "amitia://artifacts/art_generated" {
		t.Fatalf("unexpected generated resource URI: %s", image.ResourceURI)
	}
	if creator.created.OwnerSpaceID != "space_test" || creator.created.Source != artifact.SourceGenerated {
		t.Fatalf("unexpected artifact create request: %#v", creator.created)
	}
}

type fakeImageConfigService struct {
	active *imagegen.ImageGenConfig
	err    error
}

func (s *fakeImageConfigService) List() ([]imagegen.ImageGenConfig, error) {
	if s.active == nil {
		return []imagegen.ImageGenConfig{}, s.err
	}
	return []imagegen.ImageGenConfig{*s.active}, s.err
}

func (s *fakeImageConfigService) GetByID(id int) (*imagegen.ImageGenConfig, error) {
	if s.active == nil || s.active.ID != id {
		return nil, errors.New("config not found")
	}
	copy := *s.active
	copy.ApiKey = ""
	return &copy, nil
}

func (s *fakeImageConfigService) GetRuntimeByID(id int) (*imagegen.ImageGenConfig, error) {
	if s.active == nil || s.active.ID != id {
		return nil, errors.New("config not found")
	}
	copy := *s.active
	return &copy, nil
}

func (s *fakeImageConfigService) Create(*imagegen.CreateImageGenConfigRequest) (*imagegen.ImageGenConfig, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeImageConfigService) Update(int, map[string]interface{}) (*imagegen.ImageGenConfig, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeImageConfigService) Delete(int) error { return errors.New("not implemented") }

func (s *fakeImageConfigService) Activate(int) (*imagegen.ImageGenConfig, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeImageConfigService) GetActive() (*imagegen.ImageGenConfig, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.active == nil {
		return &imagegen.ImageGenConfig{}, nil
	}
	copy := *s.active
	return &copy, nil
}

func (s *fakeImageConfigService) TestConnection(context.Context, int) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeImageConfigService) ListProviders() []imagegen.ProviderInfo { return nil }

type fakeGenerationProvider struct {
	provider string
	called   *imageprovider.ImageModelConfig
}

func (p *fakeGenerationProvider) ValidateConfig(context.Context, imageprovider.ImageModelConfig) error {
	return nil
}

func (p *fakeGenerationProvider) Capabilities(context.Context, imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	return imageprovider.ImageGenerationCapabilities{SupportsReferenceImage: true, MaxReferenceImages: 1, MaxOutputImages: 4}, nil
}

func (p *fakeGenerationProvider) Submit(_ context.Context, config imageprovider.ImageModelConfig, _ imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	p.called = &config
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		return nil, err
	}
	return &imageprovider.ImageGenerationSubmission{
		Status: "succeeded",
		Result: &imageprovider.ImageGenerationResult{
			Provider: p.provider,
			Images:   []imageprovider.GeneratedImage{{Bytes: pngData, MimeType: "image/png", Width: 1, Height: 1}},
		},
	}, nil
}

func (p *fakeGenerationProvider) Query(context.Context, imageprovider.ImageModelConfig, string) (*imageprovider.ImageGenerationResult, error) {
	return nil, errors.New("not implemented")
}

func (p *fakeGenerationProvider) Cancel(context.Context, imageprovider.ImageModelConfig, string) error {
	return errors.New("not implemented")
}

func TestGenerateSelectsProviderFromActiveConfiguration(t *testing.T) {
	service := &fakeImageConfigService{active: &imagegen.ImageGenConfig{ID: 17, ApiType: "stability", ApiKey: "secret", ModelName: "stable-image-core", BaseUrl: "https://stability.example"}}
	selected := &fakeGenerationProvider{provider: "stability"}
	registry := imageprovider.NewRegistry()
	if err := registry.Register("stability", selected); err != nil {
		t.Fatal(err)
	}
	generator := NewGenerateProvider(service, registry, nil, nil)
	result, genErr := generator.Generate(context.Background(), ImageGenerateRequest{Prompt: "tree", Width: 1024, Height: 1024})
	if genErr != nil {
		t.Fatal(genErr)
	}
	if selected.called == nil || selected.called.ConfigID != 17 || selected.called.ApiType != "stability" {
		t.Fatalf("unexpected selected configuration: %#v", selected.called)
	}
	if result.Provider != "stability" || len(result.Images) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGenerateFallsBackToCloudCoreWithoutLocalKey(t *testing.T) {
	service := &fakeImageConfigService{active: &imagegen.ImageGenConfig{ID: 9, ApiType: "openai", ModelName: "dall-e-3", BaseUrl: "https://api.openai.com/v1"}}
	cloud := &fakeGenerationProvider{provider: "cloud_core"}
	registry := imageprovider.NewRegistry()
	if err := registry.Register("cloud_core", cloud); err != nil {
		t.Fatal(err)
	}
	generator := NewGenerateProvider(service, registry, nil, nil)
	result, genErr := generator.Generate(context.Background(), ImageGenerateRequest{Prompt: "tree", Width: 1024, Height: 1024})
	if genErr != nil {
		t.Fatal(genErr)
	}
	if cloud.called == nil || cloud.called.ConfigID != 0 {
		t.Fatalf("cloud bridge received unexpected config: %#v", cloud.called)
	}
	if result.Provider != "cloud_core" {
		t.Fatalf("unexpected provider: %s", result.Provider)
	}
}

func TestCapabilityRequiresActiveResolvableConfiguration(t *testing.T) {
	registry := imageprovider.NewRegistry()
	if err := registry.Register("openai", &fakeGenerationProvider{provider: "openai"}); err != nil {
		t.Fatal(err)
	}
	detector := NewCapabilityDetector(nil, &fakeImageConfigService{}, registry)
	if detector.Detect(context.Background()).Generate {
		t.Fatal("generation reported available without an active configuration")
	}
	detector = NewCapabilityDetector(nil, &fakeImageConfigService{active: &imagegen.ImageGenConfig{ID: 1, ApiType: "openai", ApiKey: "secret"}}, registry)
	if !detector.Detect(context.Background()).Generate {
		t.Fatal("generation not reported available for a registered active configuration")
	}
}

func TestProviderErrorMappingPreservesSeedreamCodes(t *testing.T) {
	generator := NewGenerateProvider(nil, nil, nil, nil)
	mapped := generator.mapProviderError("openai", &seedream.ProviderError{Code: seedream.ErrCodeAuthFailed, Message: "upstream rejected key"})
	if mapped.Code != ErrProviderAuth || mapped.HTTPStatus != 401 || mapped.Provider != "openai" {
		t.Fatalf("unexpected mapped error: %#v", mapped)
	}
}
