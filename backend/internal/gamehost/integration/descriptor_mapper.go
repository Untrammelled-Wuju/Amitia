package integration

import (
	"context"
	"fmt"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	gamehostdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

// GamePluginContributionMapper 负责将Kernel的game_plugin Contribution转换为GameHost的PluginDescriptor
type GamePluginContributionMapper interface {
	ToDescriptor(
		ctx context.Context,
		extension kerneldomain.ExtensionDefinition,
		contribution kerneldomain.ContributionDefinition,
	) (gamehostdomain.PluginDescriptor, error)
}

// DefaultGamePluginContributionMapper 默认实现
type DefaultGamePluginContributionMapper struct{}

// NewDefaultGamePluginContributionMapper 创建默认Mapper实例
func NewDefaultGamePluginContributionMapper() *DefaultGamePluginContributionMapper {
	return &DefaultGamePluginContributionMapper{}
}

// ToDescriptor 实现转换逻辑
func (m *DefaultGamePluginContributionMapper) ToDescriptor(
	ctx context.Context,
	extension kerneldomain.ExtensionDefinition,
	contribution kerneldomain.ContributionDefinition,
) (gamehostdomain.PluginDescriptor, error) {
	// 构造稳定的PluginID: <extensionID>/<contributionID>
	pluginID := gamehostdomain.PluginID(fmt.Sprintf("%s/%s", extension.ID, contribution.ID))

	// 优先使用contribution的显示名称，fallback到扩展名称
	name := contribution.Name.Default
	if name == "" {
		name = extension.Name.Default
	}

	// G38: ProtocolVersion 单一来源 protocol.ProtocolVersion (G13)
	protocolVersion := protocol.ProtocolVersion
	gameSpec, specErr := protocol.ParseGamePluginSpec(contribution.Definition)
	if specErr != nil {
		return gamehostdomain.PluginDescriptor{}, fmt.Errorf("parse game plugin spec: %w", specErr)
	}
	if err := gameSpec.Validate(); err != nil {
		return gamehostdomain.PluginDescriptor{}, fmt.Errorf("validate game plugin spec: %w", err)
	}

	// 转换能力列表，从Contribution的Definition map中读取capabilities，如果没有则返回空
	capabilities := make([]gamehostdomain.Capability, 0)
	if caps, ok := contribution.Definition["capabilities"].([]interface{}); ok {
		for _, cap := range caps {
			if capStr, ok := cap.(string); ok {
				capabilities = append(capabilities, gamehostdomain.Capability(capStr))
			}
		}
	}

	// 转换元数据，仅保留非敏感字段，复制底层map避免共享，同时转换Value为string
	metadata := make(map[string]string)
	if contribution.Metadata != nil {
		for k, v := range contribution.Metadata {
			// 过滤敏感字段
			if isKernelPrivateMetadataKey(k) {
				continue
			}
			metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	metadata["gameProtocolVersion"] = gameSpec.GameProtocolVersion
	metadata["runtimeModuleId"] = gameSpec.RuntimeModuleID
	if gameSpec.GameID != "" {
		metadata["gameId"] = gameSpec.GameID
	}
	if gameSpec.GameFamily != "" {
		metadata["gameFamily"] = gameSpec.GameFamily
	}

	descriptor := gamehostdomain.PluginDescriptor{
		ID:              pluginID,
		ExtensionID:     string(extension.ID),
		Name:            name,
		Version:         fmt.Sprintf("%v", extension.Version),
		ProtocolVersion: protocolVersion,
		Capabilities:    capabilities,
		Metadata:        metadata,
	}

	// 执行最终校验
	if err := descriptor.Validate(); err != nil {
		return gamehostdomain.PluginDescriptor{}, fmt.Errorf("mapped plugin descriptor validation failed: %w", err)
	}

	return descriptor, nil
}

// isKernelPrivateMetadataKey 判断是否为Kernel私有敏感字段
func isKernelPrivateMetadataKey(key string) bool {
	privateKeys := map[string]struct{}{
		"signature":      {},
		"secret":         {},
		"token":          {},
		"install_path":   {},
		"db_record_id":   {},
		"internal_state": {},
	}
	_, ok := privateKeys[key]
	return ok
}
