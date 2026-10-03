package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel"
	kernelmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/auth"
	mcpcanonical "github.com/u-ai/backend/internal/mcp/canonical"
	"github.com/u-ai/backend/internal/mcp/dependency"
	"github.com/u-ai/backend/internal/mcp/discovery"
	"github.com/u-ai/backend/internal/mcp/features"
	"github.com/u-ai/backend/internal/mcp/host"
	"github.com/u-ai/backend/internal/mcpapi"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
)

// MCPCompatibilityRuntime keeps the existing desktop/mobile HTTP contract while
// delegating connection ownership and tool registration to the Extension Kernel.
// It deliberately does not construct the removed legacy MCP manager.
type MCPCompatibilityRuntime struct {
	API         mcpapi.Services
	Connections *mcpcanonical.Manager
	Host        *host.Service
}

func buildMCPCompatibilityRuntime(
	appContext *app.AppContext,
	repository *mcp.Repository,
	stdio *kernelmcp.CanonicalStdioRegistry,
	remote *kernelmcp.CanonicalRemoteRegistry,
	commandResolver commandenv.Resolver,
	toolFacade *kernel.ToolFacade,
	chatService chat.Service,
	dataDir string,
) (*MCPCompatibilityRuntime, error) {
	if appContext == nil || appContext.DB == nil {
		return nil, fmt.Errorf("MCP compatibility runtime: app context unavailable")
	}
	if repository == nil || stdio == nil || remote == nil || toolFacade == nil {
		return nil, fmt.Errorf("MCP compatibility runtime: canonical dependencies unavailable")
	}

	secretDir := filepath.Join(dataDir, "mcp")
	secretStore, err := auth.NewEncryptedFileStore(
		filepath.Join(secretDir, "secrets.json"),
		filepath.Join(secretDir, "secrets.key"),
	)
	if err != nil {
		return nil, fmt.Errorf("MCP compatibility runtime: secret store: %w", err)
	}
	oauthManager := auth.NewManager(nil, secretStore, repository)
	connections := mcpcanonical.NewManager(repository, stdio, remote, secretStore, oauthManager)
	discoveryService := discovery.New(repository, connections)
	toolSyncer := mcpcanonical.NewToolSyncer(repository, toolFacade)
	featureService := features.New(repository, connections)
	interactionBroker := host.NewBroker(chatService)
	hostService := host.New(repository, connections, nil, interactionBroker, interactionBroker)
	dependencyService := dependency.New(repository, connections, discoveryService, toolSyncer, commandResolver)

	connections.RegisterReadyHandler(func(ctx context.Context, serverID string) {
		hostService.Attach(serverID)
		if err := discoveryService.Discover(ctx, serverID); err != nil {
			log.Warn("canonical MCP discovery failed for ", serverID, ": ", err)
			return
		}
		if err := toolSyncer.RegisterServer(ctx, serverID); err != nil {
			log.Warn("canonical MCP tool sync failed for ", serverID, ": ", err)
		}
	})

	runtime := &MCPCompatibilityRuntime{
		Connections: connections,
		Host:        hostService,
		API: mcpapi.Services{
			Repository:   repository,
			Connections:  connections,
			Auth:         oauthManager,
			Discovery:    discoveryService,
			Tools:        toolSyncer,
			Secrets:      secretStore,
			Features:     featureService,
			Dependencies: dependencyService,
			Interactions: interactionBroker,
		},
	}
	if err := connections.Restore(context.Background()); err != nil {
		return nil, fmt.Errorf("MCP compatibility runtime: restore: %w", err)
	}
	return runtime, nil
}
