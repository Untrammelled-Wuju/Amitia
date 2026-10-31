package localmodel

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

type ProviderFieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ProviderField struct {
	Key         string                `json:"key"`
	Label       string                `json:"label"`
	Type        string                `json:"type"`
	Required    bool                  `json:"required"`
	Secret      bool                  `json:"secret,omitempty"`
	Placeholder string                `json:"placeholder,omitempty"`
	Help        string                `json:"help,omitempty"`
	Default     any                   `json:"default,omitempty"`
	Options     []ProviderFieldOption `json:"options,omitempty"`
}

type ProviderDescriptor struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	Protocol           string                `json:"protocol"`
	DefaultProtocol    string                `json:"defaultProtocol"`
	SupportedProtocols []string              `json:"supportedProtocols"`
	RuntimeKind        string                `json:"runtimeKind"`
	ModelFormats       []string              `json:"modelFormats,omitempty"`
	ConfigFields       []ProviderField       `json:"configFields"`
	Capabilities       map[string]bool       `json:"capabilities"`
	DocsURL            string                `json:"docsUrl,omitempty"`
	DefaultModel       string                `json:"defaultModel,omitempty"`
	Availability       func() (bool, string) `json:"-"`
}

var providerDescriptors = map[string]ProviderDescriptor{}

func RegisterWithDescriptor(provider string, factory CreateInference, descriptor ProviderDescriptor) error {
	if strings.TrimSpace(descriptor.ID) == "" {
		descriptor.ID = provider
	}
	if descriptor.ID != provider {
		return errors.New("local model provider descriptor id mismatch")
	}
	if err := Register(provider, factory); err != nil {
		return err
	}
	providerDescriptors[provider] = descriptor
	return nil
}

func Descriptor(provider string) (ProviderDescriptor, bool) {
	descriptor, ok := providerDescriptors[strings.TrimSpace(provider)]
	return descriptor, ok
}

func Descriptors() []ProviderDescriptor {
	result := make([]ProviderDescriptor, 0, len(providerDescriptors))
	for _, descriptor := range providerDescriptors {
		result = append(result, descriptor)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func IsEmbedded(provider string) bool {
	descriptor, ok := Descriptor(provider)
	return ok && descriptor.RuntimeKind == "embedded"
}

func ProviderAvailable(provider string) (bool, string) {
	descriptor, ok := Descriptor(provider)
	if !ok {
		return false, "运行后端未注册"
	}
	if GetGlobalRuntimeHost() == nil {
		return false, "运行时宿主尚未就绪"
	}
	if descriptor.Availability != nil {
		return descriptor.Availability()
	}
	return true, ""
}

func Probe(ctx context.Context, params CreateLocalModelParams, prompt string) (LocalModelHealth, string, error) {
	backend, err := Create(params)
	if err != nil {
		return LocalModelHealth{State: "failed", LastError: err.Error()}, "", err
	}
	defer backend.Unload(context.Background())
	if err := backend.Load(ctx); err != nil {
		return backend.Health(ctx), "", err
	}
	health := backend.Health(ctx)
	if health.State != "ready" {
		if health.LastError == "" {
			health.LastError = "本地模型未就绪"
		}
		return health, "", errors.New(health.LastError)
	}
	probePrompt := strings.TrimSpace(prompt)
	if probePrompt == "" {
		probePrompt = "只回复 OK"
	}
	result, err := backend.Generate(ctx, LocalModelRequest{
		Messages:     []LocalModelMessage{{Role: "user", Parts: []LocalModelContent{{Type: "text", Text: probePrompt}}}},
		MaxNewTokens: 8,
		Temperature:  0,
	}, nil)
	if err != nil {
		return backend.Health(ctx), "", err
	}
	return backend.Health(ctx), result.Text, nil
}

func RunProbe(ctx context.Context, params CreateLocalModelParams, prompt string) (LocalModelHealth, string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return Probe(probeCtx, params, prompt)
}
