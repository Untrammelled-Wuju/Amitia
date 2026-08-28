package worker

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/cloudbridge"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type generationProviderResolution struct {
	ConfigID       int
	ProviderName   string
	ModelName      string
	ConfigRevision string
	Provider       imageprovider.ImageGenerationProvider
	ModelConfig    imageprovider.ImageModelConfig
}

func (w *Worker) resolveGenerationProvider(ctx context.Context, task *desktoppet.GenerationTask) (*generationProviderResolution, string, string) {
	if runtimeprofile.CurrentProcessProfile().IsDeviceAgent() {
		provider, ok := w.registry.Resolve(cloudbridge.ProviderName)
		if !ok || provider == nil {
			return nil, desktoppet.ErrCodeImageModelUnavailable, "云端生图桥不可用"
		}
		cloudProvider, ok := provider.(*cloudbridge.Provider)
		if !ok {
			return nil, desktoppet.ErrCodeImageModelUnavailable, "云端生图桥类型无效"
		}
		metadata, err := cloudProvider.DescribeConfig(ctx, task.ModelConfigID)
		if err != nil {
			return nil, desktoppet.ErrCodeImageModelUnavailable, "读取云端生图模型失败: " + err.Error()
		}
		if !metadata.Enabled {
			return nil, desktoppet.ErrCodeImageModelDisabled, "生图模型已禁用"
		}
		if !metadata.HasAPIKey {
			return nil, desktoppet.ErrCodeImageModelCredentialMissing, "生图模型缺少 API 凭据"
		}
		return &generationProviderResolution{
			ConfigID:       task.ModelConfigID,
			ProviderName:   metadata.Provider,
			ModelName:      metadata.Model,
			ConfigRevision: metadata.Revision,
			Provider:       provider,
			ModelConfig: imageprovider.ImageModelConfig{
				ConfigID:  task.ModelConfigID,
				Name:      metadata.Name,
				ApiType:   cloudbridge.ProviderName,
				ModelName: metadata.Model,
			},
		}, "", ""
	}

	cfg, err := w.repo.GetImageGenConfigByID(task.ModelConfigID)
	if err != nil || cfg == nil {
		return nil, desktoppet.ErrCodeImageModelNotFound, "生图模型配置不存在"
	}
	if cfg.Enabled != 1 {
		return nil, desktoppet.ErrCodeImageModelDisabled, "生图模型已禁用"
	}
	if strings.TrimSpace(cfg.ApiKey) == "" {
		return nil, desktoppet.ErrCodeImageModelCredentialMissing, "生图模型缺少 API 凭据"
	}
	providerName := imageprovider.NormalizeProviderName(cfg.ApiType)
	if providerName == "" {
		providerName = "seedream"
	}
	provider, ok := w.registry.Resolve(providerName)
	if !ok || provider == nil {
		return nil, desktoppet.ErrCodeImageModelUnavailable, "生图提供者不可用: " + providerName
	}
	return &generationProviderResolution{
		ConfigID:       cfg.ID,
		ProviderName:   providerName,
		ModelName:      cfg.ModelName,
		ConfigRevision: cfg.UpdatedAt,
		Provider:       provider,
		ModelConfig: imageprovider.ImageModelConfig{
			ConfigID:  cfg.ID,
			Name:      cfg.Name,
			ApiType:   cfg.ApiType,
			ApiKey:    cfg.ApiKey,
			ModelName: cfg.ModelName,
			BaseUrl:   cfg.BaseUrl,
		},
	}, "", ""
}

func validateResolvedProvider(ctx context.Context, resolved *generationProviderResolution) (imageprovider.ImageGenerationCapabilities, error) {
	if resolved == nil || resolved.Provider == nil {
		return imageprovider.ImageGenerationCapabilities{}, fmt.Errorf("generation provider resolution is empty")
	}
	if err := resolved.Provider.ValidateConfig(ctx, resolved.ModelConfig); err != nil {
		return imageprovider.ImageGenerationCapabilities{}, err
	}
	return resolved.Provider.Capabilities(ctx, resolved.ModelConfig)
}
