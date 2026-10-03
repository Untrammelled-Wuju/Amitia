package main

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/workspace"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
	"github.com/u-ai/backend/pkg/resourceuri"
	"gorm.io/gorm"
)

func buildWorkspaceServices(dataDir string, resolver *resourceuri.PhysicalResolver, db *gorm.DB, safBridge workspace.SAFBridge, remoteCredentials workspace.RemoteCredentialResolver) (*workspace.Registry, *workspace.Service, *workspacegit.GitHandler, error) {
	registry := workspace.NewRegistry()

	localBackend := workspace.NewLocalBackend(dataDir)
	safBackend := workspace.NewSAFBackend(safBridge)
	if remoteCredentials == nil {
		remoteCredentials = &unavailableWorkspaceCredentialResolver{}
	}
	remoteBackend := workspace.NewRemoteBackend(remoteCredentials, workspace.DefaultRemotePolicy)
	isolationResolver := workspacegit.NewAmitiaDataRootResolver(dataDir)
	isolatedBackend := workspacegit.NewIsolatedBackend(isolationResolver)

	if err := registry.RegisterBackend(workspace.WorkspaceKindLocal, localBackend); err != nil {
		return nil, nil, nil, fmt.Errorf("register local backend: %w", err)
	}
	if err := registry.RegisterBackend(workspace.WorkspaceKindSAF, safBackend); err != nil {
		return nil, nil, nil, fmt.Errorf("register saf backend: %w", err)
	}
	if err := registry.RegisterBackend(workspace.WorkspaceKindRemote, remoteBackend); err != nil {
		return nil, nil, nil, fmt.Errorf("register remote backend: %w", err)
	}
	if err := registry.RegisterBackend(workspace.WorkspaceKindIsolated, isolatedBackend); err != nil {
		return nil, nil, nil, fmt.Errorf("register isolated backend: %w", err)
	}

	mountRepo := workspace.NewMountRepository(db)
	workspaceService := workspace.NewServiceWithRemote(registry, resolver, mountRepo, workspaceSAFGrantResolver{bridge: safBridge}, remoteCredentials)
	if err := workspaceService.LoadAndRestoreMounts(context.Background()); err != nil {
		return nil, nil, nil, fmt.Errorf("restore workspace mounts: %w", err)
	}

	var gitHandler *workspacegit.GitHandler
	if gitEngine, gitErr := workspacegit.NewCLIGitEngine(); gitErr == nil {
		gitController := workspacegit.NewGitController(workspaceService, registry, gitEngine, workspacegit.DefaultGitPolicy, isolationResolver)
		gitHandler = workspacegit.NewGitHandler(gitController)
	}

	return registry, workspaceService, gitHandler, nil
}
