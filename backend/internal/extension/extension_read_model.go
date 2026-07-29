package extension

import (
	"context"
	"sort"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type ExtensionReadModelService struct {
	proxy   *KernelLifecycleProxy
	repo    *Repository
	counter *kernelruntime.LegacyReadCounter
}

func NewExtensionReadModelService(proxy *KernelLifecycleProxy, repo *Repository) *ExtensionReadModelService {
	return &ExtensionReadModelService{
		proxy:   proxy,
		repo:    repo,
		counter: kernelruntime.GlobalLegacyReadCounter(),
	}
}

func (s *ExtensionReadModelService) available() bool {
	return s.proxy != nil && s.proxy.ReadContainer() != nil
}

func (s *ExtensionReadModelService) TryPreviewUninstall(ctx context.Context, extensionID string) (PackageUninstallPreview, bool, error) {
	if !s.available() {
		return PackageUninstallPreview{}, false, nil
	}
	container := s.proxy.ReadContainer()
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return PackageUninstallPreview{}, false, nil
	}
	preview := PackageUninstallPreview{
		ExtensionID:      extensionID,
		CurrentVersion:   installation.InstalledVersion.String(),
		Enabled:          installation.EnablementState == domain.EnablementEnabled,
		Dependents:       []PackageDependencyView{},
		Grants:           []string{},
		Cleanup:          []string{},
		Preserved:        []string{},
		ArtifactArchived: true,
		ReadSource:       "kernel",
	}
	preview.Dependents = s.readReverseDependencies(ctx, container, extensionID)
	contribSummary, eventSubs := s.readContributions(ctx, container, extensionID)
	preview.ContributionSummary = contribSummary
	preview.EventSubscriptions = eventSubs
	preview.RuntimeImpacts = s.readRuntimeImpacts(ctx, container, extensionID)
	preview.ScheduleCount = s.readScheduleCount(ctx, container, extensionID)
	preview.Grants = s.readGrants(ctx, container, extensionID)
	preview.ConfigPresent = s.readConfigPresent(ctx, container, extensionID)
	preview.Cleanup = s.buildCleanupList(preview.ContributionSummary, preview.RuntimeImpacts, preview.Grants)
	preview.Preserved = s.buildPreservedList()
	return preview, true, nil
}

func (s *ExtensionReadModelService) TryDependencies(ctx context.Context, extensionID string) (map[string]interface{}, bool, error) {
	if !s.available() {
		return nil, false, nil
	}
	container := s.proxy.ReadContainer()
	if _, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID)); err != nil {
		return nil, false, nil
	}
	direct := s.readDirectDependencies(ctx, container, extensionID)
	reverse := s.readReverseDependencies(ctx, container, extensionID)
	return map[string]interface{}{"dependencies": direct, "dependents": reverse}, true, nil
}

func (s *ExtensionReadModelService) readReverseDependencies(ctx context.Context, container *kernelruntime.Container, extensionID string) []PackageDependencyView {
	if container.DependencyResolver == nil {
		return []PackageDependencyView{}
	}
	subjects, err := container.DependencyResolver.AffectedBy(ctx, extensionID)
	if err != nil {
		return []PackageDependencyView{}
	}
	result := make([]PackageDependencyView, 0, len(subjects))
	for _, subject := range subjects {
		view := PackageDependencyView{
			ID:       subject.SubjectID,
			Required: subject.Required,
		}
		if inst, instErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(subject.SubjectID)); instErr == nil {
			view.Installed = true
			view.Version = inst.InstalledVersion.String()
		}
		result = append(result, view)
	}
	return result
}

func (s *ExtensionReadModelService) readDirectDependencies(ctx context.Context, container *kernelruntime.Container, extensionID string) []PackageDependencyView {
	if container.ModuleRepository == nil {
		return []PackageDependencyView{}
	}
	modules, err := container.ModuleRepository.ListModules(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return []PackageDependencyView{}
	}
	seen := map[string]bool{}
	result := []PackageDependencyView{}
	for _, mod := range modules {
		for _, dep := range mod.Dependencies {
			if seen[dep.ID] {
				continue
			}
			seen[dep.ID] = true
			view := PackageDependencyView{
				ID:                dep.ID,
				VersionConstraint: dep.Version,
				Required:          !dep.Optional,
			}
			if dep.Type == domain.DependencyTypeExtension {
				if inst, instErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(dep.ID)); instErr == nil {
					view.Installed = true
					view.Version = inst.InstalledVersion.String()
				}
			} else {
				view.Installed = true
			}
			result = append(result, view)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *ExtensionReadModelService) readContributions(ctx context.Context, container *kernelruntime.Container, extensionID string) (map[string]int, []string) {
	summary := map[string]int{}
	eventSubs := []string{}
	if container.ContributionRepository == nil {
		return summary, eventSubs
	}
	contributions, err := container.ContributionRepository.ListContributions(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return summary, eventSubs
	}
	for _, contrib := range contributions {
		category := contributionCategory(contrib.Kind)
		summary[category]++
		if contrib.Kind == domain.ContributionKindEventSubscription {
			eventSubs = append(eventSubs, string(contrib.ID))
		}
	}
	sort.Strings(eventSubs)
	return summary, eventSubs
}

func (s *ExtensionReadModelService) readRuntimeImpacts(ctx context.Context, container *kernelruntime.Container, extensionID string) []PackageRuntimeImpact {
	if container.ModuleRepository == nil || container.RuntimeSupervisor == nil {
		return []PackageRuntimeImpact{}
	}
	modules, err := container.ModuleRepository.ListModules(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return []PackageRuntimeImpact{}
	}
	result := []PackageRuntimeImpact{}
	for _, mod := range modules {
		if mod.Runtime == nil {
			continue
		}
		defID := runtime_supervisor.BuildRuntimeDefinitionID(extensionID, string(mod.ID), mod.Runtime.Type)
		snapshot := container.RuntimeSupervisor.Snapshot(ctx, defID)
		for _, inst := range snapshot.Instances {
			result = append(result, PackageRuntimeImpact{
				InstanceID:  inst.InstanceID,
				ModuleID:    string(inst.Identity.ModuleID),
				RuntimeType: string(inst.Identity.RuntimeType),
				Desired:     string(inst.Desired),
				Actual:      string(inst.Actual),
				Health:      string(inst.Health),
			})
		}
	}
	return result
}

func (s *ExtensionReadModelService) readScheduleCount(ctx context.Context, container *kernelruntime.Container, extensionID string) int64 {
	if container.ScheduleRepository == nil {
		return 0
	}
	defs, err := container.ScheduleRepository.ListDefinitions(ctx, extensionID)
	if err != nil {
		return 0
	}
	return int64(len(defs))
}

func (s *ExtensionReadModelService) readGrants(ctx context.Context, container *kernelruntime.Container, extensionID string) []string {
	if container.PermissionBroker == nil {
		return []string{}
	}
	subject := permission.SubjectForExtension(extensionID)
	grants, err := container.PermissionBroker.ListGrants(ctx, permission.PermissionGrantFilter{Subject: &subject, ActiveOnly: true})
	if err != nil {
		return []string{}
	}
	result := make([]string, 0, len(grants))
	for _, grant := range grants {
		result = append(result, grant.PermissionID+":"+string(grant.Decision))
	}
	sort.Strings(result)
	return result
}

func (s *ExtensionReadModelService) readConfigPresent(ctx context.Context, container *kernelruntime.Container, extensionID string) bool {
	if container.ScopeRepository == nil {
		return false
	}
	bindings, err := container.ScopeRepository.ListBindings(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return false
	}
	return len(bindings) > 0
}

func (s *ExtensionReadModelService) buildCleanupList(summary map[string]int, runtimes []PackageRuntimeImpact, grants []string) []string {
	cleanup := []string{}
	if len(runtimes) > 0 {
		cleanup = append(cleanup, "运行时实例")
	}
	categories := make([]string, 0, len(summary))
	for category := range summary {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		switch category {
		case "tool":
			cleanup = append(cleanup, "Tool 贡献")
		case "hook":
			cleanup = append(cleanup, "Hook 贡献")
		case "event_subscription":
			cleanup = append(cleanup, "事件订阅")
		case "schedule":
			cleanup = append(cleanup, "定时任务")
		case "ui":
			cleanup = append(cleanup, "UI 贡献")
		case "agent_skill":
			cleanup = append(cleanup, "Agent Skill 索引")
		case "workflow":
			cleanup = append(cleanup, "Workflow 注册")
		case "mcp_server":
			cleanup = append(cleanup, "MCP 服务")
		case "background_service":
			cleanup = append(cleanup, "后台服务")
		case "provider":
			cleanup = append(cleanup, "Provider 注册")
		}
	}
	if len(grants) > 0 {
		cleanup = append(cleanup, "权限授予")
	}
	cleanup = append(cleanup, "Capability Grant", "当前配置与缓存")
	return cleanup
}

func (s *ExtensionReadModelService) buildPreservedList() []string {
	return []string{"版本历史", "安装与操作审计", "历史运行记录", "归档 Artifact"}
}

func contributionCategory(kind domain.ContributionKind) string {
	switch kind {
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		return "ui"
	default:
		return string(kind)
	}
}
