// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package imageprovider

import (
	"context"
	"fmt"
	"strings"
)

type ImageInput struct {
	Path     string
	Bytes    []byte
	MimeType string
}

type ImageGenerationRequest struct {
	RequestID       string
	IdempotencyKey  string
	Mode            GenerationMode
	Prompt          string
	NegativePrompt  string
	ReferenceImages []ImageInput
	Width           int
	Height          int
	Seed            *int64
	OutputCount     int
	Metadata        map[string]string
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
	Images       []GeneratedImage
	Provider     string
	Model        string
	RequestID    string
	RawMetadata  map[string]any
	Status       string
	OperationID  string
	Usage        *GenerationUsage
	ErrorCode    string
	ErrorMessage string
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
	Status      string
	OperationID string
	RequestID   string
	Result      *ImageGenerationResult
}

type ImageModelConfig struct {
	ConfigID  int
	Name      string
	ApiType   string
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

type ExtendedProvider interface {
	ImageGenerationProvider
	ExtendedCapabilities(ctx context.Context, config ImageModelConfig) (ProviderCapabilities, error)
}

type ProviderDescriptor struct {
	Name           string
	DefaultModel   string
	DefaultBaseURL string
	SupportedModes []GenerationMode
}

type Registry struct {
	providers   map[string]ImageGenerationProvider
	descriptors map[string]ProviderDescriptor
	aliases     map[string]string
}

func NewRegistry() *Registry {
	return &Registry{
		providers:   make(map[string]ImageGenerationProvider),
		descriptors: make(map[string]ProviderDescriptor),
		aliases:     make(map[string]string),
	}
}

func (r *Registry) Register(name string, provider ImageGenerationProvider) error {
	if r.providers == nil {
		r.providers = make(map[string]ImageGenerationProvider)
	}
	if r.descriptors == nil {
		r.descriptors = make(map[string]ProviderDescriptor)
	}
	canonical := NormalizeProviderName(name)
	if _, exists := r.providers[canonical]; exists {
		return fmt.Errorf("provider %s already registered", canonical)
	}
	r.providers[canonical] = provider
	return nil
}

func (r *Registry) RegisterWithDescriptor(name string, provider ImageGenerationProvider, desc ProviderDescriptor) error {
	canonical := NormalizeProviderName(name)
	if err := r.Register(canonical, provider); err != nil {
		return err
	}
	desc.Name = canonical
	r.descriptors[canonical] = desc
	return nil
}

func (r *Registry) RegisterAlias(alias, canonical string) {
	if r.aliases == nil {
		r.aliases = make(map[string]string)
	}
	r.aliases[NormalizeProviderName(alias)] = NormalizeProviderName(canonical)
}

func (r *Registry) Resolve(name string) (ImageGenerationProvider, bool) {
	if r.providers == nil {
		return nil, false
	}
	canonical := NormalizeProviderName(name)
	if alias, ok := r.aliases[canonical]; ok {
		canonical = alias
	}
	p, ok := r.providers[canonical]
	return p, ok
}

func (r *Registry) Describe(name string) (ProviderDescriptor, bool) {
	if r.descriptors == nil {
		return ProviderDescriptor{}, false
	}
	canonical := NormalizeProviderName(name)
	if alias, ok := r.aliases[canonical]; ok {
		canonical = alias
	}
	desc, ok := r.descriptors[canonical]
	return desc, ok
}

func (r *Registry) Get(name string) (ImageGenerationProvider, bool) {
	return r.Resolve(name)
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

func NormalizeProviderName(name string) string {
	s := strings.TrimSpace(name)
	s = strings.ToLower(s)
	return s
}

var defaultAliases = map[string]string{
	"volcengine_seedream": "seedream",
	"doubao_seedream":     "seedream",
	"ark_seedream":        "seedream",
}
