package integration

import (
	"context"
	"fmt"
	"sync"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	contracts "github.com/u-ai/backend/internal/gamehost/contracts"
	gamehostdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

// KernelContributionSource 定义从Kernel获取game_plugin的接口，避免直接依赖Kernel实现
type KernelContributionSource interface {
	// ListEnabledGamePlugins 获取所有已启用的game_plugin列表
	ListEnabledGamePlugins(ctx context.Context) ([]KernelGamePlugin, error)
}

// KernelGamePlugin 封装Kernel中的扩展和对应的game_plugin贡献
type KernelGamePlugin struct {
	Extension    kerneldomain.ExtensionDefinition
	Contribution kerneldomain.ContributionDefinition
}

// SyncResult 同步结果统计
type SyncResult struct {
	Registered   int
	Unregistered int
	Unchanged    int
	Failed       int
	Errors       []error
}

// Add 合并两个SyncResult
func (r *SyncResult) Add(other SyncResult) {
	r.Registered += other.Registered
	r.Unregistered += other.Unregistered
	r.Unchanged += other.Unchanged
	r.Failed += other.Failed
	r.Errors = append(r.Errors, other.Errors...)
}

// HasError 判断是否有错误
func (r SyncResult) HasError() bool {
	return r.Failed > 0 || len(r.Errors) > 0
}

// GamePluginSyncService Kernel到GameHost的同步服务
type GamePluginSyncService struct {
	registry contracts.PluginRegistry
	mapper   GamePluginContributionMapper
	source   KernelContributionSource
	mu       sync.Mutex
}

// NewGamePluginSyncService 创建同步服务实例
func NewGamePluginSyncService(
	reg contracts.PluginRegistry,
	mapper GamePluginContributionMapper,
	source KernelContributionSource,
) (*GamePluginSyncService, error) {
	if reg == nil {
		return nil, fmt.Errorf("plugin registry is required")
	}
	if mapper == nil {
		return nil, fmt.Errorf("contribution mapper is required")
	}
	if source == nil {
		return nil, fmt.Errorf("kernel contribution source is required")
	}
	return &GamePluginSyncService{
		registry: reg,
		mapper:   mapper,
		source:   source,
	}, nil
}

// FullSync 全量同步，将Kernel中的启用game_plugin同步到Registry，幂等
func (s *GamePluginSyncService) FullSync(ctx context.Context) SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := SyncResult{}

	// 1. 从Kernel获取所有启制的game_plugin
	plugins, err := s.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Errorf("failed to list enabled game plugins from kernel: %w", err))
		return result
	}

	// 过滤：仅保留game domain的game_plugin
	validPlugins := make([]KernelGamePlugin, 0, len(plugins))
	existingPlugins := make(map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor)
	for _, p := range plugins {
		if p.Extension.Domain != kerneldomain.ExtensionDomainGame {
			continue
		}
		if p.Contribution.Kind != kerneldomain.ContributionKindGamePlugin {
			continue
		}
		validPlugins = append(validPlugins, p)

		// 先尝试转换Descriptor，失败的记录错误
		desc, err := s.mapper.ToDescriptor(ctx, p.Extension, p.Contribution)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to map game plugin %s/%s: %w", p.Extension.ID, p.Contribution.ID, err))
			continue
		}
		existingPlugins[desc.ID] = desc
	}

	// 2. 获取当前Registry中的所有插件
	currentPlugins, err := s.registry.List(ctx)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Errorf("failed to list plugins from registry: %w", err))
		return result
	}

	// 3. 计算需要注销的插件：在Registry但不在Kernel当前列表中的
	toBeUnregistered := make([]gamehostdomain.PluginID, 0)
	for _, current := range currentPlugins {
		if _, ok := existingPlugins[current.ID]; !ok {
			toBeUnregistered = append(toBeUnregistered, current.ID)
		}
	}

	// 4. 执行注销
	for _, id := range toBeUnregistered {
		if err := s.registry.Unregister(ctx, id); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to unregister orphan plugin %s: %w", id, err))
			continue
		}
		result.Unregistered++
	}

	// 5. 同步存在的插件
	for id, desired := range existingPlugins {
		// 检查是否已经存在
		current, err := s.registry.Get(ctx, id)
		if err != nil {
			// 不存在，直接注册
			if err := s.registry.Register(ctx, desired); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Errorf("failed to register plugin %s: %w", id, err))
				continue
			}
			result.Registered++
			continue
		}

		// 存在，比较Descriptor是否一致
		if gamehostdomain.EqualPluginDescriptor(current, desired) {
			result.Unchanged++
			continue
		}

		// 不一致，先注销旧的在注册新的
		if err := s.registry.Unregister(ctx, id); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to unregister outdated plugin %s: %w", id, err))
			continue
		}
		if err := s.registry.Register(ctx, desired); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to register updated plugin %s: %w", id, err))
			// 尝试回滚，重新注册旧的
			if err := s.registry.Register(ctx, current); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to rollback plugin %s: %w", id, err))
			}
			continue
		}
		result.Registered++
		result.Unregistered++
	}

	return result
}

// SyncExtension 同步单个Extension的game_plugin，保证原子性
func (s *GamePluginSyncService) SyncExtension(ctx context.Context, extensionID string) SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := SyncResult{}
	plugins, err := s.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Errorf("failed to list game plugins for extension %s: %w", extensionID, err))
		return result
	}

	desired := make(map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor)
	for _, p := range plugins {
		if string(p.Extension.ID) != extensionID || p.Extension.Domain != kerneldomain.ExtensionDomainGame || p.Contribution.Kind != kerneldomain.ContributionKindGamePlugin {
			continue
		}
		desc, mapErr := s.mapper.ToDescriptor(ctx, p.Extension, p.Contribution)
		if mapErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to map game plugin %s/%s: %w", p.Extension.ID, p.Contribution.ID, mapErr))
			return result
		}
		desired[desc.ID] = desc
	}

	original, err := s.registry.ListByExtension(ctx, extensionID)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Errorf("failed to list registered plugins for extension %s: %w", extensionID, err))
		return result
	}
	originalByID := make(map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor, len(original))
	for _, desc := range original {
		originalByID[desc.ID] = desc
	}

	rollback := func(cause error) {
		current, listErr := s.registry.ListByExtension(ctx, extensionID)
		if listErr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("rollback list extension %s: %w", extensionID, listErr))
			return
		}
		for _, desc := range current {
			if unregisterErr := s.registry.Unregister(ctx, desc.ID); unregisterErr != nil {
				result.Errors = append(result.Errors, fmt.Errorf("rollback unregister plugin %s: %w", desc.ID, unregisterErr))
			}
		}
		for _, desc := range original {
			if registerErr := s.registry.Register(ctx, desc); registerErr != nil {
				result.Errors = append(result.Errors, fmt.Errorf("rollback restore plugin %s: %w", desc.ID, registerErr))
			}
		}
		result.Failed++
		result.Errors = append(result.Errors, cause)
	}

	// Remove registrations that no longer exist in the enabled Kernel view first.
	for _, current := range original {
		if _, keep := desired[current.ID]; keep {
			continue
		}
		if err := s.registry.Unregister(ctx, current.ID); err != nil {
			rollback(fmt.Errorf("failed to unregister stale plugin %s: %w", current.ID, err))
			return result
		}
		result.Unregistered++
	}

	for id, want := range desired {
		current, exists := originalByID[id]
		if exists && gamehostdomain.EqualPluginDescriptor(current, want) {
			result.Unchanged++
			continue
		}
		if exists {
			if err := s.registry.Unregister(ctx, id); err != nil {
				rollback(fmt.Errorf("failed to unregister outdated plugin %s: %w", id, err))
				return result
			}
			result.Unregistered++
		}
		if err := s.registry.Register(ctx, want); err != nil {
			rollback(fmt.Errorf("failed to register plugin %s: %w", id, err))
			return result
		}
		result.Registered++
	}

	return result
}

// UnregisterExtension 注销指定Extension的所有game_plugin
func (s *GamePluginSyncService) UnregisterExtension(ctx context.Context, extensionID string) SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := SyncResult{}

	// 获取该Extension的所有已注册插件
	plugins, err := s.registry.ListByExtension(ctx, extensionID)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Errorf("failed to list plugins for extension %s: %w", extensionID, err))
		return result
	}
	for _, p := range plugins {
		if err := s.registry.Unregister(ctx, p.ID); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to unregister plugin %s: %w", p.ID, err))
			continue
		}
		result.Unregistered++
	}

	return result
}
