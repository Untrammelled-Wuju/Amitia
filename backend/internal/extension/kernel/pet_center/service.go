package pet_center

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/capabilities/behaviorcatalog"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

const managementTarget = "pet_center"

var (
	ErrKernelUnavailable        = errors.New("pet_center: extension kernel unavailable")
	ErrExtensionNotFound        = errors.New("pet_center: extension not found")
	ErrNotDesktopPetPlugin      = errors.New("pet_center: extension is not a petx")
	ErrInvalidInput             = errors.New("pet_center: invalid input")
	ErrManagementTargetMismatch = errors.New("pet_center: package management target mismatch")
	ErrPackageIdentityMismatch  = errors.New("pet_center: package identity mismatch")
	ErrBehaviorNotFound         = errors.New("pet_center: behavior not found")
	ErrBehaviorRevision         = errors.New("pet_center: behavior catalog revision changed")
	ErrBehaviorNotExecutable    = errors.New("pet_center: behavior is not executable")
	ErrBehaviorExecutorMissing  = errors.New("pet_center: behavior tool executor unavailable")
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
	preflight PackageTargetPreflight
	executor  BehaviorToolExecutor
}

type BehaviorInvocationScope struct {
	SpaceID        string
	DeviceID       string
	CharacterID    string
	ConversationID string
	Channel        string
	SessionID      string
	RequestID      string
}

type BehaviorToolExecutor func(ctx context.Context, extensionID, toolID string, input json.RawMessage, scope BehaviorInvocationScope) (json.RawMessage, error)

type PackageTargetPreflight interface {
	ValidateArchiveTarget(ctx context.Context, archivePath string, expected domain.ManagementTarget) (*PackageTargetPreview, error)
}

type PackageTargetPreview struct {
	ExtensionID      string
	ManagementTarget domain.ManagementTarget
	Installable      bool
}

func NewDesktopPetPluginManagementService(container kernelContainer, runtime kernelRuntime, preflight PackageTargetPreflight, executors ...BehaviorToolExecutor) *DesktopPetPluginManagementService {
	service := &DesktopPetPluginManagementService{
		container: container,
		runtime:   runtime,
		preflight: preflight,
	}
	if len(executors) > 0 {
		service.executor = executors[0]
	}
	return service
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
			log.Printf("pet_center: list contributions for %s: %v", inst.ExtensionID, err)
			continue
		}
		hasPetPlugin := false
		for _, c := range contribs {
			if c.EffectiveAdapter() == domain.ContributionAdapterDesktopPet {
				hasPetPlugin = true
				break
			}
		}
		if !hasPetPlugin {
			continue
		}
		for _, c := range contribs {
			if c.EffectiveAdapter() != domain.ContributionAdapterDesktopPet {
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
			if c.EffectiveAdapter() == domain.ContributionAdapterDesktopPet {
				hasPetPlugin = true
				break
			}
		}
		if !hasPetPlugin {
			continue
		}
		for _, c := range contribs {
			if c.EffectiveAdapter() != domain.ContributionAdapterDesktopPet {
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
		if c.EffectiveAdapter() == domain.ContributionAdapterDesktopPet {
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

func (s *DesktopPetPluginManagementService) GetBehaviors(ctx context.Context, pluginID string) (BehaviorCatalog, error) {
	_, catalog, err := s.findBehaviorCatalog(ctx, pluginID)
	if err != nil {
		return BehaviorCatalog{}, err
	}
	return catalog, nil
}

func (s *DesktopPetPluginManagementService) InvokeBehavior(
	ctx context.Context,
	pluginID, behaviorID, expectedRevision string,
	input json.RawMessage,
	scope BehaviorInvocationScope,
) (BehaviorInvocationResult, error) {
	if s == nil || s.container == nil {
		return BehaviorInvocationResult{}, ErrKernelUnavailable
	}
	if s.executor == nil {
		return BehaviorInvocationResult{}, ErrBehaviorExecutorMissing
	}
	expectedRevision = strings.TrimSpace(expectedRevision)
	if expectedRevision == "" {
		return BehaviorInvocationResult{}, fmt.Errorf("%w: expected revision is required", ErrInvalidInput)
	}
	behaviorID = strings.TrimSpace(behaviorID)
	inst, catalog, err := s.findBehaviorCatalog(ctx, pluginID)
	if err != nil {
		return BehaviorInvocationResult{}, err
	}
	if catalog.CatalogRevision != expectedRevision {
		return BehaviorInvocationResult{}, fmt.Errorf("%w: expected=%s current=%s", ErrBehaviorRevision, expectedRevision, catalog.CatalogRevision)
	}
	behavior, ok := catalog.Find(behaviorID)
	if !ok {
		return BehaviorInvocationResult{}, fmt.Errorf("%w: %s", ErrBehaviorNotFound, behaviorID)
	}
	if behavior.Available != nil && !*behavior.Available {
		return BehaviorInvocationResult{}, fmt.Errorf("%w: %s is unavailable", ErrBehaviorNotFound, behavior.ID)
	}
	if strings.TrimSpace(behavior.ToolID) == "" {
		return BehaviorInvocationResult{}, fmt.Errorf("%w: behavior %s has no toolId", ErrBehaviorNotExecutable, behavior.ID)
	}
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	if !json.Valid(input) {
		return BehaviorInvocationResult{}, fmt.Errorf("%w: behavior arguments are not valid JSON", ErrInvalidInput)
	}
	var arguments map[string]any
	if err := json.Unmarshal(input, &arguments); err != nil || arguments == nil {
		if err == nil {
			err = errors.New("arguments must be an object")
		}
		return BehaviorInvocationResult{}, fmt.Errorf("%w: behavior arguments must be an object: %v", ErrInvalidInput, err)
	}
	result, err := s.executor(ctx, string(inst.ExtensionID), behavior.ToolID, input, scope)
	if err != nil {
		return BehaviorInvocationResult{}, fmt.Errorf("invoke pet behavior %s using tool %s: %w", behavior.ID, behavior.ToolID, err)
	}
	if len(result) == 0 {
		result = json.RawMessage(`null`)
	}
	if !json.Valid(result) {
		return BehaviorInvocationResult{}, fmt.Errorf("invoke pet behavior %s using tool %s returned invalid JSON", behavior.ID, behavior.ToolID)
	}
	return BehaviorInvocationResult{
		ExtensionID:     string(inst.ExtensionID),
		PluginID:        pluginID,
		BehaviorID:      behavior.ID,
		CatalogRevision: catalog.CatalogRevision,
		ToolID:          behavior.ToolID,
		Result:          append(json.RawMessage(nil), result...),
	}, nil
}

func (s *DesktopPetPluginManagementService) findBehaviorCatalog(ctx context.Context, pluginID string) (domain.ExtensionInstallation, BehaviorCatalog, error) {
	if s == nil || s.container == nil {
		return domain.ExtensionInstallation{}, BehaviorCatalog{}, ErrKernelUnavailable
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return domain.ExtensionInstallation{}, BehaviorCatalog{}, ErrInvalidInput
	}
	installations, err := s.container.ListInstallations(ctx)
	if err != nil {
		return domain.ExtensionInstallation{}, BehaviorCatalog{}, fmt.Errorf("list installations: %w", err)
	}
	for _, inst := range installations {
		contributions, err := s.container.ListContributions(ctx, inst.ExtensionID)
		if err != nil {
			continue
		}
		for _, contribution := range contributions {
			if contribution.EffectiveAdapter() != domain.ContributionAdapterDesktopPet || string(contribution.ID) != pluginID {
				continue
			}
			catalog, err := behaviorcatalog.ParseDefinition(contribution.Metadata)
			if errors.Is(err, behaviorcatalog.ErrMissing) {
				return inst, BehaviorCatalog{ExtensionID: string(inst.ExtensionID), PluginID: pluginID, CatalogRevision: "empty", Behaviors: []behaviorcatalog.Behavior{}}, nil
			}
			if err != nil {
				return domain.ExtensionInstallation{}, BehaviorCatalog{}, err
			}
			return inst, BehaviorCatalog{ExtensionID: string(inst.ExtensionID), PluginID: pluginID, CatalogRevision: catalog.Revision, Behaviors: catalog.Behaviors}, nil
		}
	}
	return domain.ExtensionInstallation{}, BehaviorCatalog{}, ErrExtensionNotFound
}

func (s *DesktopPetPluginManagementService) Install(ctx context.Context, archivePath string) (*InstallResult, error) {
	if s.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	if archivePath == "" {
		return nil, ErrInvalidInput
	}
	if s.preflight == nil {
		return nil, ErrKernelUnavailable
	}

	preview, err := s.preflight.ValidateArchiveTarget(ctx, archivePath, domain.ManagementTargetDesktopPetCenter)
	if err != nil {
		return nil, err
	}

	installed, err := s.runtime.Install(ctx, archivePath)
	if err != nil {
		return nil, err
	}

	if installed.ID != preview.ExtensionID {
		return nil, ErrPackageIdentityMismatch
	}

	if err := s.requireDesktopPetPlugin(ctx, installed.ID); err != nil {
		return nil, err
	}

	return &InstallResult{
		ExtensionID:  installed.ID,
		Version:      installed.Version,
		InstallState: string(PluginInstallStateInstalled),
	}, nil
}

func (s *DesktopPetPluginManagementService) Update(ctx context.Context, extensionID string, archivePath string) (*InstallResult, error) {
	if s.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	if extensionID == "" {
		return nil, ErrInvalidInput
	}
	if archivePath == "" {
		return nil, ErrInvalidInput
	}
	if s.preflight == nil {
		return nil, ErrKernelUnavailable
	}

	if err := s.requireDesktopPetPlugin(ctx, extensionID); err != nil {
		return nil, err
	}

	preview, err := s.preflight.ValidateArchiveTarget(ctx, archivePath, domain.ManagementTargetDesktopPetCenter)
	if err != nil {
		return nil, err
	}

	if preview.ExtensionID != extensionID {
		return nil, ErrPackageIdentityMismatch
	}

	installed, err := s.runtime.Update(ctx, archivePath)
	if err != nil {
		return nil, err
	}

	if installed.ID != extensionID {
		return nil, ErrPackageIdentityMismatch
	}

	if err := s.requireDesktopPetPlugin(ctx, installed.ID); err != nil {
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
	if err := s.requireDesktopPetPlugin(ctx, extensionID); err != nil {
		return nil, err
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
	if err := s.requireDesktopPetPlugin(ctx, extensionID); err != nil {
		return nil, err
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
	if err := s.requireDesktopPetPlugin(ctx, extensionID); err != nil {
		return nil, err
	}
	if err := s.runtime.Uninstall(ctx, extensionID); err != nil {
		return nil, err
	}
	return &MutationResult{ExtensionID: extensionID, Success: true}, nil
}

func (s *DesktopPetPluginManagementService) requireDesktopPetPlugin(ctx context.Context, extID string) error {
	if s.container == nil {
		return ErrKernelUnavailable
	}
	contribs, err := s.container.ListContributions(ctx, domain.ExtensionID(extID))
	if err != nil {
		return err
	}
	for _, c := range contribs {
		if c.EffectiveAdapter() == domain.ContributionAdapterDesktopPet {
			return nil
		}
	}
	return ErrNotDesktopPetPlugin
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
