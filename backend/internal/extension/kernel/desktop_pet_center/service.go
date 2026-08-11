package desktop_pet_center

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

const managementTarget = "desktop_pet_center"

var (
	ErrKernelUnavailable   = errors.New("desktop_pet_center: extension kernel unavailable")
	ErrExtensionNotFound   = errors.New("desktop_pet_center: extension not found")
	ErrNotDesktopPetPlugin = errors.New("desktop_pet_center: extension is not a desktop_pet_plugin")
	ErrInvalidInput        = errors.New("desktop_pet_center: invalid input")
)

type kernelContainer interface {
	ListInstallations(ctx context.Context) ([]domain.ExtensionInstallation, error)
	GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error)
	ListModules(ctx context.Context, extID domain.ExtensionID) ([]domain.ModuleDefinition, error)
	ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error)
	ListGrants(ctx context.Context, extID domain.ExtensionID) ([]sqlite.PermissionGrant, error)
}

type kernelRuntime interface {
	Install(ctx context.Context, archivePath string) (kernelInstalledExtension, error)
	Update(ctx context.Context, archivePath string) (kernelInstalledExtension, error)
	Enable(ctx context.Context, extensionID string) error
	Disable(ctx context.Context, extensionID string) error
	Uninstall(ctx context.Context, extensionID string) error
}

type kernelInstalledExtension struct {
	ID      string
	Name    string
	Version string
}

type DesktopPetPluginManagementService struct {
	container kernelContainer
	runtime   kernelRuntime
}

func NewDesktopPetPluginManagementService(container kernelContainer, runtime kernelRuntime) *DesktopPetPluginManagementService {
	return &DesktopPetPluginManagementService{
		container: container,
		runtime:   runtime,
	}
}

func (s *DesktopPetPluginManagementService) List(ctx context.Context, page, pageSize int, search string) (*ListResponse, error) {
	if s.container == nil {
		return nil, ErrKernelUnavailable
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	installations, err := s.container.ListInstallations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installations: %w", err)
	}
	matches := make([]DesktopPetPluginSummary, 0)
	for _, inst := range installations {
		contribs, err := s.container.ListContributions(ctx, inst.ExtensionID)
		if err != nil {
			log.Printf("desktop_pet_center: list contributions for %s: %v", inst.ExtensionID, err)
			continue
		}
		hasPetPlugin := false
		for _, c := range contribs {
			if c.Kind == domain.ContributionKindDesktopPetPlugin {
				hasPetPlugin = true
				break
			}
		}
		if !hasPetPlugin {
			continue
		}
		for _, c := range contribs {
			if c.Kind != domain.ContributionKindDesktopPetPlugin {
				continue
			}
			if search != "" && !matchesPluginSearch(c, inst, search) {
				continue
			}
			summary := s.buildSummary(inst, c)
			matches = append(matches, summary)
		}
	}
	total := len(matches)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &ListResponse{
		Plugins:  matches[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *DesktopPetPluginManagementService) Get(ctx context.Context, pluginID string) (*DesktopPetPluginDetail, error) {
	if s.container == nil {
		return nil, ErrKernelUnavailable
	}
	if pluginID == "" {
		return nil, ErrInvalidInput
	}
	installations, err := s.container.ListInstallations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installations: %w", err)
	}
	for _, inst := range installations {
		contribs, err := s.container.ListContributions(ctx, inst.ExtensionID)
		if err != nil {
			continue
		}
		hasPetPlugin := false
		for _, c := range contribs {
			if c.Kind == domain.ContributionKindDesktopPetPlugin {
				hasPetPlugin = true
				break
			}
		}
		if !hasPetPlugin {
			continue
		}
		for _, c := range contribs {
			if c.Kind != domain.ContributionKindDesktopPetPlugin {
				continue
			}
			if string(c.ID) != pluginID {
				continue
			}
			return s.buildDetail(ctx, inst, c)
		}
	}
	return nil, ErrExtensionNotFound
}

func (s *DesktopPetPluginManagementService) GetByExtensionID(ctx context.Context, extID string) ([]DesktopPetPluginDetail, error) {
	if s.container == nil {
		return nil, ErrKernelUnavailable
	}
	inst, err := s.container.GetInstallation(ctx, domain.ExtensionID(extID))
	if err != nil {
		return nil, ErrExtensionNotFound
	}
	contribs, err := s.container.ListContributions(ctx, inst.ExtensionID)
	if err != nil {
		return nil, fmt.Errorf("list contributions: %w", err)
	}
	found := false
	var results []DesktopPetPluginDetail
	for _, c := range contribs {
		if c.Kind == domain.ContributionKindDesktopPetPlugin {
			found = true
			detail, err := s.buildDetail(ctx, inst, c)
			if err != nil {
				continue
			}
			results = append(results, *detail)
		}
	}
	if !found {
		return nil, ErrNotDesktopPetPlugin
	}
	return results, nil
}

func (s *DesktopPetPluginManagementService) Install(ctx context.Context, archivePath string) (*InstallResult, error) {
	if s.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	if archivePath == "" {
		return nil, ErrInvalidInput
	}
	installed, err := s.runtime.Install(ctx, archivePath)
	if err != nil {
		return nil, err
	}
	return &InstallResult{
		ExtensionID:  installed.ID,
		Version:      installed.Version,
		InstallState: string(PluginInstallStateInstalled),
	}, nil
}

func (s *DesktopPetPluginManagementService) Update(ctx context.Context, archivePath string) (*InstallResult, error) {
	if s.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	if archivePath == "" {
		return nil, ErrInvalidInput
	}
	installed, err := s.runtime.Update(ctx, archivePath)
	if err != nil {
		return nil, err
	}
	return &InstallResult{
		ExtensionID:  installed.ID,
		Version:      installed.Version,
		InstallState: string(PluginInstallStateInstalled),
	}, nil
}

func (s *DesktopPetPluginManagementService) Enable(ctx context.Context, extensionID string) (*MutationResult, error) {
	if s.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	if extensionID == "" {
		return nil, ErrInvalidInput
	}
	if !s.hasDesktopPetPlugin(ctx, extensionID) {
		return nil, ErrNotDesktopPetPlugin
	}
	if err := s.runtime.Enable(ctx, extensionID); err != nil {
		return nil, err
	}
	return &MutationResult{ExtensionID: extensionID, Success: true}, nil
}

func (s *DesktopPetPluginManagementService) Disable(ctx context.Context, extensionID string) (*MutationResult, error) {
	if s.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	if extensionID == "" {
		return nil, ErrInvalidInput
	}
	if !s.hasDesktopPetPlugin(ctx, extensionID) {
		return nil, ErrNotDesktopPetPlugin
	}
	if err := s.runtime.Disable(ctx, extensionID); err != nil {
		return nil, err
	}
	return &MutationResult{ExtensionID: extensionID, Success: true}, nil
}

func (s *DesktopPetPluginManagementService) Uninstall(ctx context.Context, extensionID string) (*MutationResult, error) {
	if s.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	if extensionID == "" {
		return nil, ErrInvalidInput
	}
	if !s.hasDesktopPetPlugin(ctx, extensionID) {
		return nil, ErrNotDesktopPetPlugin
	}
	if err := s.runtime.Uninstall(ctx, extensionID); err != nil {
		return nil, err
	}
	return &MutationResult{ExtensionID: extensionID, Success: true}, nil
}

func (s *DesktopPetPluginManagementService) hasDesktopPetPlugin(ctx context.Context, extID string) bool {
	if s.container == nil {
		return false
	}
	contribs, err := s.container.ListContributions(ctx, domain.ExtensionID(extID))
	if err != nil {
		return false
	}
	for _, c := range contribs {
		if c.Kind == domain.ContributionKindDesktopPetPlugin {
			return true
		}
	}
	return false
}

func (s *DesktopPetPluginManagementService) buildSummary(inst domain.ExtensionInstallation, c domain.ContributionDefinition) DesktopPetPluginSummary {
	enabled := inst.EnablementState == domain.EnablementEnabled
	installState := mapInstallState(inst.InstallationState)
	return DesktopPetPluginSummary{
		ExtensionID:      string(inst.ExtensionID),
		PluginID:         string(c.ID),
		Name:             c.Name.Default,
		Description:      c.Description.Default,
		Version:          inst.InstalledVersion.String(),
		Enabled:          enabled,
		InstallState:     installState,
		ManagementTarget: managementTarget,
		Publisher:        string(inst.ExtensionID),
	}
}

func (s *DesktopPetPluginManagementService) buildDetail(ctx context.Context, inst domain.ExtensionInstallation, c domain.ContributionDefinition) (*DesktopPetPluginDetail, error) {
	enabled := inst.EnablementState == domain.EnablementEnabled
	installState := mapInstallState(inst.InstallationState)
	grants, _ := s.container.ListGrants(ctx, inst.ExtensionID)
	grantNames := make([]string, 0, len(grants))
	for _, g := range grants {
		grantNames = append(grantNames, g.PermissionName)
	}
	return &DesktopPetPluginDetail{
		ExtensionID:         string(inst.ExtensionID),
		PluginID:            string(c.ID),
		Name:                c.Name.Default,
		Description:         c.Description.Default,
		Version:             inst.InstalledVersion.String(),
		Enabled:             enabled,
		InstallState:        installState,
		ManagementTarget:    managementTarget,
		Publisher:           string(inst.ExtensionID),
		RequiredPermissions: c.RequiredPermissions,
		PermissionSummary: &PermissionSummary{
			Declared: c.RequiredPermissions,
			Granted:  grantNames,
		},
		PackageVersion: inst.InstalledVersion.String(),
		InstalledAt:    &inst.InstalledAt,
		UpdatedAt:      &inst.UpdatedAt,
	}, nil
}

func mapInstallState(state domain.InstallationState) PluginInstallState {
	switch state {
	case domain.InstallationStateInstalled:
		return PluginInstallStateInstalled
	case domain.InstallationStateInstalling:
		return PluginInstallStateInstalling
	case domain.InstallationStateFailed, domain.InstallationStateUninstallFailed:
		return PluginInstallStateFailed
	case domain.InstallationStateUninstalling:
		return PluginInstallStateUninstalling
	default:
		return PluginInstallState(state)
	}
}

func matchesPluginSearch(c domain.ContributionDefinition, inst domain.ExtensionInstallation, search string) bool {
	needle := strings.ToLower(search)
	haystacks := []string{
		string(inst.ExtensionID),
		string(c.ID),
		c.Name.Default,
		c.Description.Default,
	}
	for _, h := range haystacks {
		if strings.Contains(strings.ToLower(h), needle) {
			return true
		}
	}
	return false
}

var _ = time.Now
