// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package imageprovider

import "context"

type ImageInput struct {
	Path     string
	Bytes    []byte
	MimeType string
}

type ImageGenerationRequest struct {
	Prompt           string
	NegativePrompt   string
	ReferenceImages  []ImageInput
	Width            int
	Height           int
	Seed             *int64
	OutputCount      int
	Metadata         map[string]string
}

type GeneratedImage struct {
	Bytes    []byte
	MimeType string
	Width    int
	Height   int
	Metadata map[string]any
}

type GenerationUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	ImageCount       int
	Raw              map[string]any
}

type ImageGenerationResult struct {
	Images        []GeneratedImage
	Provider      string
	Model         string
	RequestID     string
	RawMetadata   map[string]any
	Status        string
	OperationID   string
	Usage         *GenerationUsage
	ErrorCode     string
	ErrorMessage  string
}

type ImageGenerationCapabilities struct {
	SupportsReferenceImage bool
	SupportsMultipleImages bool
	SupportsNegativePrompt bool
	SupportsSeed           bool
	SupportsAsyncOperation bool
	SupportsCancellation   bool
	MaxReferenceImages     int
	MaxOutputImages        int
}

type ImageGenerationSubmission struct {
	Status       string
	OperationID  string
	RequestID    string
	Result       *ImageGenerationResult
}

type ImageModelConfig struct {
	Name      string
	ApiKey    string
	ModelName string
	BaseUrl   string
}

type ImageGenerationProvider interface {
	ValidateConfig(ctx context.Context, config ImageModelConfig) error
	Capabilities(ctx context.Context, config ImageModelConfig) (ImageGenerationCapabilities, error)
	Submit(ctx context.Context, config ImageModelConfig, request ImageGenerationRequest) (*ImageGenerationSubmission, error)
	Query(ctx context.Context, config ImageModelConfig, operationID string) (*ImageGenerationResult, error)
	Cancel(ctx context.Context, config ImageModelConfig, operationID string) error
}

type Registry struct {
	providers map[string]ImageGenerationProvider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]ImageGenerationProvider)}
}

func (r *Registry) Register(name string, provider ImageGenerationProvider) {
	if r.providers == nil {
		r.providers = make(map[string]ImageGenerationProvider)
	}
	r.providers[name] = provider
}

func (r *Registry) Get(name string) (ImageGenerationProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func DefaultRegistry() *Registry {
	return NewRegistry()
}
