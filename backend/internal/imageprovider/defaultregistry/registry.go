package defaultregistry

import (
	"fmt"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/cloudbridge"
	"github.com/u-ai/backend/internal/imageprovider/seedream"
	"github.com/u-ai/backend/internal/imageprovider/stability"
	"github.com/u-ai/backend/internal/imageprovider/tongyi"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func New() (*imageprovider.Registry, error) {
	registry := imageprovider.NewRegistry()
	if err := seedream.Register(registry); err != nil {
		return nil, fmt.Errorf("failed to register seedream provider: %w", err)
	}
	for _, provider := range []struct {
		name         string
		adapterName  string
		displayName  string
		defaultModel string
		defaultURL   string
	}{
		{"openai", "openai", "OpenAI DALL-E", "dall-e-3", "https://api.openai.com/v1"},
		{"cogview", "cogview", "智谱 CogView", "cogview-3-plus", "https://open.bigmodel.cn/api/paas/v4"},
		{"siliconflow", "siliconflow", "硅基流动", "Kwai-Kolors/Kolors", "https://api.siliconflow.cn/v1"},
	} {
		if err := seedream.RegisterNamed(registry, provider.name, provider.adapterName, provider.displayName, provider.defaultModel, provider.defaultURL, []imageprovider.GenerationMode{imageprovider.ModeSingleFrame}); err != nil {
			return nil, fmt.Errorf("failed to register %s provider: %w", provider.name, err)
		}
	}
	if err := stability.Register(registry); err != nil {
		return nil, fmt.Errorf("failed to register stability provider: %w", err)
	}
	if err := tongyi.Register(registry); err != nil {
		return nil, fmt.Errorf("failed to register tongyi provider: %w", err)
	}
	if runtimeprofile.CurrentProcessProfile().IsDeviceAgent() {
		if err := registry.RegisterWithDescriptor(cloudbridge.ProviderName, cloudbridge.NewProvider(config.AppCfg.Storage.DataDir), imageprovider.ProviderDescriptor{
			Name:           cloudbridge.ProviderName,
			DisplayName:    "Cloud Core 生图代理",
			DefaultModel:   "active",
			DefaultBaseURL: "",
			SupportedModes: []imageprovider.GenerationMode{
				imageprovider.ModeSingleFrame,
				imageprovider.ModeSpriteSheet,
				imageprovider.ModeKeyframe,
			},
			DeviceOnly: true,
		}); err != nil {
			return nil, fmt.Errorf("failed to register cloud image generation bridge: %w", err)
		}
	}
	return registry, nil
}
