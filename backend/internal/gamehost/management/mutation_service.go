package management

import (
	"context"
	"errors"
	"fmt"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
)

type KernelMutation interface {
	Install(ctx context.Context, archivePath string) (KernelInstalledExtension, error)
	Update(ctx context.Context, archivePath string) (KernelInstalledExtension, error)
	Enable(ctx context.Context, extensionID string) error
	Disable(ctx context.Context, extensionID string) error
	Uninstall(ctx context.Context, extensionID string) error
}

type KernelInstalledExtension struct {
	ID      string
	Name    string
	Version string
}

type PackageTargetPreflight interface {
	ValidateArchiveTarget(ctx context.Context, archivePath string, expected kerneldomain.ManagementTarget) (*PackageTargetPreview, error)
}

type PackageTargetPreview struct {
	ExtensionID      string
	ManagementTarget kerneldomain.ManagementTarget
	Installable      bool
}

type KernelTargetReader interface {
	ListGameCenterExtensions(ctx context.Context) ([]kerneldomain.ExtensionDefinition, []kerneldomain.ExtensionInstallation, error)
	GetGameCenterExtension(ctx context.Context, extensionID string) (*kerneldomain.ExtensionDefinition, *kerneldomain.ExtensionInstallation, error)
	ListGameCenterContributions(ctx context.Context, extensionID string) ([]kerneldomain.ContributionDefinition, error)
}

type PackageUpgradeCoordinator interface {
	ExecuteUpgrade(ctx context.Context, extensionID, archivePath string) error
}

type PackageMutationService struct {
	kernel             KernelMutation
	reader             KernelTargetReader
	registry           PluginRegistryReader
	upgradeCoordinator PackageUpgradeCoordinator
	preflight          PackageTargetPreflight
}

type PackageMutationServiceOptions struct {
	Kernel             KernelMutation
	Reader             KernelTargetReader
	Registry           PluginRegistryReader
	UpgradeCoordinator PackageUpgradeCoordinator
	Preflight          PackageTargetPreflight
}

func NewPackageMutationService(opts PackageMutationServiceOptions) *PackageMutationService {
	return &PackageMutationService{
		kernel:             opts.Kernel,
		reader:             opts.Reader,
		registry:           opts.Registry,
		upgradeCoordinator: opts.UpgradeCoordinator,
		preflight:          opts.Preflight,
	}
}

func (s *PackageMutationService) Install(ctx context.Context, req PackageInstallRequest) (*PackageMutationResult, error) {
	_ = ctx
	_ = req
	return nil, ErrPackageLifecycleRequired
}

func (s *PackageMutationService) preflightArchive(ctx context.Context, archivePath string, expected kerneldomain.ManagementTarget) (*PackageTargetPreview, error) {
	if s.preflight == nil {
		return nil, ErrKernelUnavailable
	}
	preview, err := s.preflight.ValidateArchiveTarget(ctx, archivePath, expected)
	if err != nil {
		return nil, err
	}
	return preview, nil
}

func (s *PackageMutationService) Update(ctx context.Context, req PackageUpdateRequest) (*PackageMutationResult, error) {
	_ = ctx
	_ = req
	return nil, ErrPackageLifecycleRequired
}

func (s *PackageMutationService) Enable(ctx context.Context, extensionID string) (*PackageMutationResult, error) {
	if s.kernel == nil {
		return nil, ErrKernelUnavailable
	}
	if extensionID == "" {
		return nil, ErrInvalidInput
	}

	if !s.isGameCenterExtension(ctx, extensionID) {
		return nil, ErrNotGamePlugin
	}

	if err := s.kernel.Enable(ctx, extensionID); err != nil {
		return nil, fmt.Errorf("enable failed: %w", err)
	}

	return &PackageMutationResult{
		ExtensionID: extensionID,
		Operation:   "enable",
		State:       "enabled",
	}, nil
}

func (s *PackageMutationService) Disable(ctx context.Context, extensionID string) (*PackageMutationResult, error) {
	if s.kernel == nil {
		return nil, ErrKernelUnavailable
	}
	if extensionID == "" {
		return nil, ErrInvalidInput
	}

	if !s.isGameCenterExtension(ctx, extensionID) {
		return nil, ErrNotGamePlugin
	}

	if err := s.kernel.Disable(ctx, extensionID); err != nil {
		return nil, fmt.Errorf("disable failed: %w", err)
	}

	return &PackageMutationResult{
		ExtensionID: extensionID,
		Operation:   "disable",
		State:       "disabled",
	}, nil
}

func (s *PackageMutationService) Uninstall(ctx context.Context, extensionID string) (*PackageMutationResult, error) {
	_ = ctx
	_ = extensionID
	return nil, ErrPackageLifecycleRequired
}

func (s *PackageMutationService) requireGameCenterExtension(ctx context.Context, extensionID string) error {
	if s.reader == nil {
		return ErrKernelUnavailable
	}
	_, inst, err := s.reader.GetGameCenterExtension(ctx, extensionID)
	if err != nil {
		return err
	}
	if inst == nil {
		return ErrNotGamePlugin
	}
	return nil
}

func (s *PackageMutationService) isGameCenterExtension(ctx context.Context, extensionID string) bool {
	if s.reader == nil {
		return false
	}
	_, inst, _ := s.reader.GetGameCenterExtension(ctx, extensionID)
	return inst != nil
}

type RuntimeMutationExecutor interface {
	StartRuntime(ctx context.Context, runtimeID ghdomain.RuntimeInstanceID) error
	StopRuntime(ctx context.Context, runtimeID ghdomain.RuntimeInstanceID) error
	RestartRuntime(ctx context.Context, runtimeID ghdomain.RuntimeInstanceID, reason string) error
}

type RuntimeLister interface {
	ListRuntimes() []*ghruntime.RuntimeInstanceRef
	GetRuntime(runtimeID string) (*ghruntime.RuntimeInstanceRef, error)
}

type RuntimeMutationService struct {
	executor       RuntimeMutationExecutor
	runtimeLister  RuntimeLister
	pluginRegistry PluginRegistryReader
}

type RuntimeMutationServiceOptions struct {
	Executor       RuntimeMutationExecutor
	RuntimeLister  RuntimeLister
	PluginRegistry PluginRegistryReader
}

func NewRuntimeMutationService(opts RuntimeMutationServiceOptions) *RuntimeMutationService {
	return &RuntimeMutationService{
		executor:       opts.Executor,
		runtimeLister:  opts.RuntimeLister,
		pluginRegistry: opts.PluginRegistry,
	}
}

func (s *RuntimeMutationService) Start(ctx context.Context, runtimeID string) (*RuntimeMutationResult, error) {
	if s.executor == nil {
		return nil, ErrRuntimeExecutorUnavailable
	}
	if runtimeID == "" {
		return nil, ErrInvalidInput
	}

	if !s.validateRuntimeBelongsToGameCenter(ctx, runtimeID) {
		return nil, ErrRuntimeNotGameCenter
	}

	ghRuntimeID := ghdomain.RuntimeInstanceID(runtimeID)
	if err := s.executor.StartRuntime(ctx, ghRuntimeID); err != nil {
		return nil, fmt.Errorf("start runtime failed: %w", err)
	}

	return &RuntimeMutationResult{
		RuntimeID: runtimeID,
		Operation: "start",
	}, nil
}

func (s *RuntimeMutationService) Stop(ctx context.Context, runtimeID string) (*RuntimeMutationResult, error) {
	if s.executor == nil {
		return nil, ErrRuntimeExecutorUnavailable
	}
	if runtimeID == "" {
		return nil, ErrInvalidInput
	}

	if !s.validateRuntimeBelongsToGameCenter(ctx, runtimeID) {
		return nil, ErrRuntimeNotGameCenter
	}

	ghRuntimeID := ghdomain.RuntimeInstanceID(runtimeID)
	if err := s.executor.StopRuntime(ctx, ghRuntimeID); err != nil {
		return nil, fmt.Errorf("stop runtime failed: %w", err)
	}

	return &RuntimeMutationResult{
		RuntimeID: runtimeID,
		Operation: "stop",
	}, nil
}

func (s *RuntimeMutationService) Restart(ctx context.Context, runtimeID string) (*RuntimeMutationResult, error) {
	if s.executor == nil {
		return nil, ErrRuntimeExecutorUnavailable
	}
	if runtimeID == "" {
		return nil, ErrInvalidInput
	}

	if !s.validateRuntimeBelongsToGameCenter(ctx, runtimeID) {
		return nil, ErrRuntimeNotGameCenter
	}

	ghRuntimeID := ghdomain.RuntimeInstanceID(runtimeID)
	if err := s.executor.RestartRuntime(ctx, ghRuntimeID, "game-center restart"); err != nil {
		return nil, fmt.Errorf("restart runtime failed: %w", err)
	}

	return &RuntimeMutationResult{
		RuntimeID: runtimeID,
		Operation: "restart",
	}, nil
}

func (s *RuntimeMutationService) validateRuntimeBelongsToGameCenter(ctx context.Context, runtimeID string) bool {
	if s.runtimeLister == nil || s.pluginRegistry == nil {
		return false
	}
	rt, err := s.runtimeLister.GetRuntime(runtimeID)
	if err != nil || rt == nil {
		return false
	}
	plugin, err := s.pluginRegistry.Get(ctx, string(rt.PluginID))
	if err != nil {
		return false
	}
	return plugin.ExtensionID != ""
}

var (
	ErrKernelUnavailable          = errors.New("game-center: extension kernel unavailable")
	ErrRuntimeExecutorUnavailable = errors.New("game-center: runtime executor unavailable")
	ErrInvalidInput               = errors.New("game-center: invalid input")
	ErrNotGamePlugin              = errors.New("game-center: extension is not a game_plugin")
	ErrRuntimeNotGameCenter       = errors.New("game-center: runtime does not belong to game_center")
	ErrManagementTargetMismatch   = errors.New("game-center: package management target mismatch")
	ErrPackageIdentityMismatch    = errors.New("game-center: package identity mismatch")
	ErrPackageLifecycleRequired   = errors.New("game-center: package writes must use the extension package preview/confirmation lifecycle")
)

type RuntimeMutationResult struct {
	RuntimeID string `json:"runtimeId"`
	Operation string `json:"operation"`
}
