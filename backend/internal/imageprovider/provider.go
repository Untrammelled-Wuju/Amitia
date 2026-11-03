// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package imageprovider

import (
	"context"
	"fmt"
	"strings"
)

type ImageInput struct {
	Path     string `json:"path,omitempty"`
	Bytes    []byte `json:"bytes,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type ImageGenerationRequest struct {
	RequestID       string            `json:"requestId,omitempty"`
	IdempotencyKey  string            `json:"idempotencyKey,omitempty"`
	Mode            GenerationMode    `json:"mode,omitempty"`
	Prompt          string            `json:"prompt"`
	NegativePrompt  string            `json:"negativePrompt,omitempty"`
	ReferenceImages []ImageInput      `json:"referenceImages,omitempty"`
	Width           int               `json:"width,omitempty"`
	Height          int               `json:"height,omitempty"`
	Seed            *int64            `json:"seed,omitempty"`
	OutputCount     int               `json:"outputCount,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type GeneratedImage struct {
	Bytes    []byte         `json:"bytes,omitempty"`
	MimeType string         `json:"mimeType,omitempty"`
	Width    int            `json:"width,omitempty"`
	Height   int            `json:"height,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type GenerationUsage struct {
	PromptTokens     int64          `json:"promptTokens,omitempty"`
	CompletionTokens int64          `json:"completionTokens,omitempty"`
	TotalTokens      int64          `json:"totalTokens,omitempty"`
	ImageCount       int            `json:"imageCount,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
}

type ImageGenerationResult struct {
	Images       []GeneratedImage `json:"images,omitempty"`
	Provider     string           `json:"provider,omitempty"`
	Model        string           `json:"model,omitempty"`
	RequestID    string           `json:"requestId,omitempty"`
	RawMetadata  map[string]any   `json:"rawMetadata,omitempty"`
	Status       string           `json:"status,omitempty"`
	OperationID  string           `json:"operationId,omitempty"`
	Usage        *GenerationUsage `json:"usage,omitempty"`
	ErrorCode    string           `json:"errorCode,omitempty"`
	ErrorMessage string           `json:"errorMessage,omitempty"`
}

type ImageGenerationCapabilities struct {
	SupportsReferenceImage bool `json:"supportsReferenceImage"`
	SupportsMultipleImages bool `json:"supportsMultipleImages"`
	SupportsNegativePrompt bool `json:"supportsNegativePrompt"`
	SupportsSeed           bool `json:"supportsSeed"`
	SupportsAsyncOperation bool `json:"supportsAsyncOperation"`
	SupportsCancellation   bool `json:"supportsCancellation"`
	MaxReferenceImages     int  `json:"maxReferenceImages"`
	MaxOutputImages        int  `json:"maxOutputImages"`
}

type ImageGenerationSubmission struct {
	Status      string                 `json:"status"`
	OperationID string                 `json:"operationId,omitempty"`
	RequestID   string                 `json:"requestId,omitempty"`
	Result      *ImageGenerationResult `json:"result,omitempty"`
}

type ImageModelConfig struct {
	ConfigID  int    `json:"configId"`
	Name      string `json:"name"`
	ApiType   string `json:"apiType"`
	ApiKey    string `json:"apiKey"`
	ModelName string `json:"modelName"`
	BaseUrl   string `json:"baseUrl"`
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

type ConnectionProbe interface {
	Probe(ctx context.Context, config ImageModelConfig) (*ImageGenerationSubmission, error)
}

type ProviderDescriptor struct {
	Name           string
	DisplayName    string
	DefaultModel   string
	DefaultBaseURL string
	SupportedModes []GenerationMode
	LocalOnly      bool
	DeviceOnly     bool
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

func (r *Registry) Select(apiType, apiKey string) (string, ImageGenerationProvider, bool) {
	name := NormalizeProviderName(apiType)
	if strings.TrimSpace(apiKey) == "" {
		if provider, ok := r.Resolve("cloud_core"); ok {
			return "cloud_core", provider, true
		}
	}
	provider, ok := r.Resolve(name)
	return name, provider, ok
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

func (r *Registry) Descriptors() []ProviderDescriptor {
	descriptors := make([]ProviderDescriptor, 0, len(r.descriptors))
	for _, descriptor := range r.descriptors {
		descriptors = append(descriptors, descriptor)
	}
	return descriptors
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
