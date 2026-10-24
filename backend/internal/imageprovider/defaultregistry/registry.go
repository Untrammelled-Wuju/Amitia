package defaultregistry

import (
	"fmt"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/cloudbridge"
	"github.com/u-ai/backend/internal/imageprovider/seedream"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func New() (*imageprovider.Registry, error) {
	registry := imageprovider.NewRegistry()
	for alias, canonical := range map[string]string{
		"volcengine_seedream": "seedream",
		"doubao_seedream":     "seedream",
		"ark_seedream":        "seedream",
	} {
		registry.RegisterAlias(alias, canonical)
	}
	if err := seedream.Register(registry); err != nil {
		return nil, fmt.Errorf("failed to register seedream provider: %w", err)
	}
	if runtimeprofile.CurrentProcessProfile().IsDeviceAgent() {
		if err := registry.Register(cloudbridge.ProviderName, cloudbridge.NewProvider(config.AppCfg.Storage.DataDir)); err != nil {
			return nil, fmt.Errorf("failed to register cloud image generation bridge: %w", err)
		}
	}
	return registry, nil
}
