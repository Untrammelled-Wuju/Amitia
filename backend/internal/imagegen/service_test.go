package imagegen

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/imageprovider"
)

type testRepository struct {
	config ImageGenConfig
}

func (r *testRepository) List() ([]ImageGenConfig, error) { return []ImageGenConfig{r.config}, nil }
func (r *testRepository) GetByID(id int) (*ImageGenConfig, error) {
	if id != r.config.ID {
		return nil, errors.New("config not found")
	}
	copy := r.config
	return &copy, nil
}
func (r *testRepository) Create(*ImageGenConfig) error { return errors.New("not implemented") }
func (r *testRepository) Update(int, map[string]interface{}) error {
	return errors.New("not implemented")
}
func (r *testRepository) Delete(int) error { return errors.New("not implemented") }
func (r *testRepository) Activate(int) error {
	return errors.New("not implemented")
}
func (r *testRepository) GetActive() (*ImageGenConfig, error) {
	copy := r.config
	return &copy, nil
}

type probeProvider struct {
	config imageprovider.ImageModelConfig
}

func (p *probeProvider) ValidateConfig(context.Context, imageprovider.ImageModelConfig) error {
	return nil
}
func (p *probeProvider) Capabilities(context.Context, imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	return imageprovider.ImageGenerationCapabilities{MaxOutputImages: 1}, nil
}
func (p *probeProvider) Submit(context.Context, imageprovider.ImageModelConfig, imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	return &imageprovider.ImageGenerationSubmission{Status: "succeeded", Result: &imageprovider.ImageGenerationResult{Images: []imageprovider.GeneratedImage{{Bytes: []byte{1}, MimeType: "image/png"}}}}, nil
}
func (p *probeProvider) Query(context.Context, imageprovider.ImageModelConfig, string) (*imageprovider.ImageGenerationResult, error) {
	return nil, errors.New("not implemented")
}
func (p *probeProvider) Cancel(context.Context, imageprovider.ImageModelConfig, string) error {
	return errors.New("not implemented")
}
func (p *probeProvider) Probe(ctx context.Context, config imageprovider.ImageModelConfig) (*imageprovider.ImageGenerationSubmission, error) {
	p.config = config
	return p.Submit(ctx, config, imageprovider.ImageGenerationRequest{})
}

func TestTestConnectionUsesConfiguredProvider(t *testing.T) {
	repo := &testRepository{config: ImageGenConfig{ID: 8, Name: "tongyi", ApiType: "tongyi", ApiKey: "secret", ModelName: "wanx", BaseUrl: "https://dashscope.example"}}
	provider := &probeProvider{}
	registry := imageprovider.NewRegistry()
	if err := registry.Register("tongyi", provider); err != nil {
		t.Fatal(err)
	}
	result, err := NewService(repo, registry).TestConnection(context.Background(), 8)
	if err != nil {
		t.Fatal(err)
	}
	if result["success"] != true || result["provider"] != "tongyi" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if provider.config.ConfigID != 8 || provider.config.ApiType != "tongyi" {
		t.Fatalf("unexpected provider config: %#v", provider.config)
	}
}
