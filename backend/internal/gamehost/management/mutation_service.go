package management

import (
	"context"
	"errors"
	"fmt"
	"log"

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

type KernelTargetReader interface {
	ListGameCenterExtensions(ctx context.Context) ([]kerneldomain.ExtensionDefinition, []kerneldomain.ExtensionInstallation, error)
	GetGameCenterExtension(ctx context.Context, extensionID string) (*kerneldomain.ExtensionDefinition, *kerneldomain.ExtensionInstallation, error)
	ListGameCenterContributions(ctx context.Context, extensionID string) ([]kerneldomain.ContributionDefinition, error)
}

type PackageMutationService struct {
	kernel   KernelMutation
	reader   KernelTargetReader
	registry PluginRegistryReader
}

type PackageMutationServiceOptions struct {
	Kernel   KernelMutation
	Reader   KernelTargetReader
	Registry PluginRegistryReader
}

func NewPackageMutationService(opts PackageMutationServiceOptions) *PackageMutationService {
	return &PackageMutationService{
		kernel:   opts.Kernel,
		reader:   opts.Reader,
		registry: opts.Registry,
	}
}

func (s *PackageMutationService) Install(ctx context.Context, req PackageInstallRequest) (*PackageMutationResult, error) {
	if s.kernel == nil {
		return nil, ErrKernelUnavailable
	}
	if req.ArchivePath == "" {
		return nil, ErrInvalidInput
	}

	installed, err := s.kernel.Install(ctx, req.ArchivePath)
	if err != nil {
		return nil, fmt.Errorf("install failed: %w", err)
	}

	hasGamePlugin := s.checkGamePluginPostInstall(ctx, installed.ID)
	if !hasGamePlugin {
		log.Printf("[game-center] installed extension %s does not contain game_plugin contribution", installed.ID)
	}

	return &PackageMutationResult{
		ExtensionID:  installed.ID,
		Operation:    "install",
		State:        "installed",
		CurrentVersion: installed.Version,
	}, nil
}

func (s *PackageMutationService) Update(ctx context.Context, req PackageUpdateRequest) (*PackageMutationResult, error) {
	if s.kernel == nil {
		return nil, ErrKernelUnavailable
	}
	if req.ExtensionID == "" {
		return nil, ErrInvalidInput
	}

	if !s.isGameCenterExtension(ctx, req.ExtensionID) {
		return nil, ErrNotGamePlugin
	}

	if req.ArchivePath == "" {
		return nil, ErrInvalidInput
	}

	installed, err := s.kernel.Update(ctx, req.ArchivePath)
	if err != nil {
		return nil, fmt.Errorf("update failed: %w", err)
	}

	return &PackageMutationResult{
		ExtensionID:  installed.ID,
		Operation:    "update",
		State:        "installed",
		CurrentVersion: installed.Version,
	}, nil
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
	if s.kernel == nil {
		return nil, ErrKernelUnavailable
	}
	if extensionID == "" {
		return nil, ErrInvalidInput
	}

	if !s.isGameCenterExtension(ctx, extensionID) {
		return nil, ErrNotGamePlugin
	}

	if err := s.kernel.Uninstall(ctx, extensionID); err != nil {
		return nil, fmt.Errorf("uninstall failed: %w", err)
	}

	return &PackageMutationResult{
		ExtensionID: extensionID,
		Operation:   "uninstall",
		State:       "uninstalled",
	}, nil
}

func (s *PackageMutationService) isGameCenterExtension(ctx context.Context, extensionID string) bool {
	if s.reader == nil {
		return false
	}
	_, inst, err := s.reader.GetGameCenterExtension(ctx, extensionID)
	if err != nil || inst == nil {
		return false
	}
	return true
}

func (s *PackageMutationService) checkGamePluginPostInstall(ctx context.Context, extensionID string) bool {
	if s.reader == nil {
		return false
	}
	contribs, err := s.reader.ListGameCenterContributions(ctx, extensionID)
	if err != nil {
		return false
	}
	for _, c := range contribs {
		if c.Kind == kerneldomain.ContributionKindGamePlugin {
			return true
		}
	}
	return false
}

type RuntimeMutationExecutor interface {
	StartRuntime(ctx context.Context, runtimeID ghdomain.RuntimeInstanceID) error
	StopRuntime(ctx context.Context, runtimeID ghdomain.RuntimeInstanceID) error
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
	if err := s.executor.StopRuntime(ctx, ghRuntimeID); err != nil {
		return nil, fmt.Errorf("restart (stop phase) failed: %w", err)
	}
	if err := s.executor.StartRuntime(ctx, ghRuntimeID); err != nil {
		return nil, fmt.Errorf("restart (start phase) failed: %w", err)
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
	ErrKernelUnavailable        = errors.New("game-center: extension kernel unavailable")
	ErrRuntimeExecutorUnavailable = errors.New("game-center: runtime executor unavailable")
	ErrInvalidInput             = errors.New("game-center: invalid input")
	ErrNotGamePlugin            = errors.New("game-center: extension is not a game_plugin")
	ErrRuntimeNotGameCenter     = errors.New("game-center: runtime does not belong to game_center")
)

type RuntimeMutationResult struct {
	RuntimeID string `json:"runtimeId"`
	Operation string `json:"operation"`
}
