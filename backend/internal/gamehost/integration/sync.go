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
// 采用两阶段 Reconcile：Phase 1 构造完整 Desired State，Phase 2 应用变更。
// 任何 Prepare 阶段错误都不会触发 Registry mutation。
func (s *GamePluginSyncService) FullSync(ctx context.Context) SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := SyncResult{}

	// Phase 1: Prepare - 纯计算阶段，构造完整 Desired State
	// 任何失败都直接返回，不执行任何 Registry 操作
	desired, err := s.buildDesiredState(ctx)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, err)
		return result
	}

	// Phase 2: Apply - 只有完整 Desired State 构建成功后才执行
	return s.applyDesiredState(ctx, desired, &result)
}

// buildDesiredState Phase 1: 构造完整的 Desired State
// 纯计算阶段，禁止调用任何 Registry 操作
func (s *GamePluginSyncService) buildDesiredState(ctx context.Context) (map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor, error) {
	plugins, err := s.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("stage=snapshot: list enabled game plugins from kernel: %w", err)
	}

	desired := make(map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor)
	for _, p := range plugins {
		if p.Extension.Domain != kerneldomain.ExtensionDomainGame {
			continue
		}
		if p.Contribution.Kind != kerneldomain.ContributionKindGamePlugin {
			continue
		}

		desc, err := s.mapper.ToDescriptor(ctx, p.Extension, p.Contribution)
		if err != nil {
			return nil, fmt.Errorf("stage=descriptor: plugin=%s/%s: %w", p.Extension.ID, p.Contribution.ID, err)
		}
		desired[desc.ID] = desc
	}

	return desired, nil
}

// applyDesiredState Phase 2: 应用 Desired State 到 Registry
// 顺序：先 Register/Update，最后处理 orphan Unregister
func (s *GamePluginSyncService) applyDesiredState(
	ctx context.Context,
	desired map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor,
	result *SyncResult,
) SyncResult {
	// 获取当前 Registry 状态
	currentPlugins, err := s.registry.List(ctx)
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, fmt.Errorf("stage=reconcile-register: list plugins from registry: %w", err))
		return *result
	}
	currentByID := make(map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor, len(currentPlugins))
	for _, cp := range currentPlugins {
		currentByID[cp.ID] = cp
	}

	// 先 Register/Update 所有 desired plugins
	for id, want := range desired {
		current, exists := currentByID[id]
		if exists && gamehostdomain.EqualPluginDescriptor(current, want) {
			result.Unchanged++
			continue
		}
		if exists {
			if err := s.registry.Unregister(ctx, id); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Errorf("stage=reconcile-update: plugin=%s: unregister: %w", id, err))
				continue
			}
		}
		if err := s.registry.Register(ctx, want); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("stage=reconcile-register: plugin=%s: register: %w", id, err))
			continue
		}
		result.Registered++
	}

	// 最后处理 orphan Unregister
	for _, current := range currentPlugins {
		if _, ok := desired[current.ID]; ok {
			continue
		}
		if err := s.registry.Unregister(ctx, current.ID); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("stage=reconcile-unregister: plugin=%s: %w", current.ID, err))
			continue
		}
		result.Unregistered++
	}

	return *result
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
