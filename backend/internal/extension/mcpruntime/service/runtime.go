package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel"
	kernelmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	mcp "github.com/u-ai/backend/internal/extension/mcpruntime"
	mcpapi "github.com/u-ai/backend/internal/extension/mcpruntime/api"
	"github.com/u-ai/backend/internal/extension/mcpruntime/auth"
	"github.com/u-ai/backend/internal/extension/mcpruntime/canonical"
	"github.com/u-ai/backend/internal/extension/mcpruntime/dependency"
	"github.com/u-ai/backend/internal/extension/mcpruntime/discovery"
	"github.com/u-ai/backend/internal/extension/mcpruntime/features"
	"github.com/u-ai/backend/internal/extension/mcpruntime/host"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
	"gorm.io/gorm"
)

type Runtime struct {
	API         mcpapi.Services
	Connections *canonical.Manager
	Host        *host.Service
	Repository  *mcp.Repository
	Secrets     auth.SecretStore
	Discovery   *discovery.Service
	Tools       *canonical.ToolSyncer
	profile     runtimeprofile.Profile
}

func NewRuntime(
	db *gorm.DB,
	repository *mcp.Repository,
	stdio *kernelmcp.CanonicalStdioRegistry,
	remote *kernelmcp.CanonicalRemoteRegistry,
	commandResolver commandenv.Resolver,
	toolFacade *kernel.ToolFacade,
	chatService chat.Service,
	dataDir string,
	profile runtimeprofile.Profile,
	providedSecrets ...auth.SecretStore,
) (*Runtime, error) {
	if db == nil || repository == nil || stdio == nil || remote == nil || toolFacade == nil {
		return nil, fmt.Errorf("MCP runtime dependencies are unavailable")
	}
	if err := repository.EnsureOwnershipIndexes(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure MCP ownership indexes: %w", err)
	}
	secretDir := filepath.Join(dataDir, "mcp")
	var secretStore auth.SecretStore
	if len(providedSecrets) > 0 {
		secretStore = providedSecrets[0]
	}
	if secretStore == nil {
		var err error
		secretStore, err = auth.NewEncryptedFileStore(filepath.Join(secretDir, "secrets.json"), filepath.Join(secretDir, "secrets.key"))
		if err != nil {
			return nil, fmt.Errorf("initialize MCP secret store: %w", err)
		}
	}
	oauthManager := auth.NewManager(nil, secretStore, repository)
	connections := canonical.NewManager(repository, stdio, remote, secretStore, oauthManager)
	discoveryService := discovery.New(repository, connections)
	toolSyncer := canonical.NewToolSyncer(repository, toolFacade)
	featureService := features.New(repository, connections)
	interactionBroker := host.NewBroker(chatService)
	hostService := host.New(repository, connections, nil, interactionBroker, interactionBroker)
	dependencyService := dependency.New(repository, connections, discoveryService, toolSyncer, commandResolver)

	connections.RegisterReadyHandler(func(ctx context.Context, serverID string) error {
		hostService.Attach(serverID)
		if err := discoveryService.Discover(ctx, serverID); err != nil {
			return fmt.Errorf("discover MCP server %s: %w", serverID, err)
		}
		if err := toolSyncer.RegisterServer(ctx, serverID); err != nil {
			return fmt.Errorf("register MCP tools for %s: %w", serverID, err)
		}
		return nil
	})

	runtime := &Runtime{
		Connections: connections,
		Host:        hostService,
		Repository:  repository,
		Secrets:     secretStore,
		Discovery:   discoveryService,
		Tools:       toolSyncer,
		profile:     profile,
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
		return nil, fmt.Errorf("restore MCP connections: %w", err)
	}
	return runtime, nil
}

func (r *Runtime) PutContributionBinding(ctx context.Context, binding kernel.MCPBindingDescriptor) (string, error) {
	return r.putContributionBinding(ctx, binding, false)
}

func (r *Runtime) ActivateContributionBinding(ctx context.Context, binding kernel.MCPBindingDescriptor) (string, error) {
	active := r.hostsPlacement(binding.RuntimePlacement)
	serverID, err := r.putContributionBinding(ctx, binding, active)
	if err != nil {
		return "", err
	}
	if !active {
		_ = r.Repository.SetScopeEnabled(ctx, serverID, false)
		return serverID, nil
	}
	if err := r.Repository.SetScopeEnabled(ctx, serverID, true); err != nil {
		return "", err
	}
	if err := r.Connections.Connect(ctx, serverID); err != nil {
		_ = r.Repository.SetScopeEnabled(context.WithoutCancel(ctx), serverID, false)
		return "", err
	}
	return serverID, nil
}

func (r *Runtime) hostsPlacement(placement string) bool {
	placement = strings.ToLower(strings.TrimSpace(placement))
	if placement == "" || r.profile == runtimeprofile.ProfileLocal {
		return true
	}
	switch r.profile {
	case runtimeprofile.ProfileCloudCore:
		return placement == "cloud"
	case runtimeprofile.ProfileDeviceAgent:
		return placement == "device"
	default:
		return false
	}
}

func (r *Runtime) DeactivateContributionBinding(ctx context.Context, extensionID, contributionID string) error {
	server, err := r.Repository.FindServerByOwner(ctx, "package", extensionID, contributionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var firstErr error
	if err := r.Connections.Disconnect(ctx, server.ID); err != nil {
		firstErr = err
	}
	if err := r.Tools.UnregisterServer(ctx, server.ID); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := r.Repository.SetScopeEnabled(ctx, server.ID, false); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (r *Runtime) RemoveContributionBinding(ctx context.Context, extensionID, contributionID string) error {
	server, err := r.Repository.FindServerByOwner(ctx, "package", extensionID, contributionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return r.removeServer(ctx, server.ID)
}

func (r *Runtime) RemoveExtensionBindings(ctx context.Context, extensionID string) error {
	servers, err := r.Repository.ListServersByExtension(ctx, extensionID)
	if err != nil {
		return err
	}
	for _, server := range servers {
		if err := r.removeServer(ctx, server.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) putContributionBinding(ctx context.Context, binding kernel.MCPBindingDescriptor, enabled bool) (string, error) {
	extensionID := strings.TrimSpace(binding.ExtensionID)
	contributionID := strings.TrimSpace(binding.ContributionID)
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("MCP package binding requires extensionId and contributionId")
	}
	input := contributionServerInput(binding, enabled)
	existing, err := r.Repository.FindServerByOwner(ctx, "package", extensionID, contributionID)
	switch {
	case err == nil:
		if _, err := r.Repository.UpdateServer(ctx, existing.ID, input); err != nil {
			return "", err
		}
		if err := r.writeStdioEnvironment(ctx, existing.ID, binding.Env); err != nil {
			return "", err
		}
		if err := r.writeCredential(ctx, existing.ID, binding); err != nil {
			return "", err
		}
		if err := r.syncBindingCapabilities(ctx, existing.ID, binding); err != nil {
			return "", err
		}
		return existing.ID, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return "", err
	}

	identity, err := mcp.NormalizeServerIdentity(input)
	if err != nil {
		return "", err
	}
	if existing, identityErr := r.Repository.FindServerByIdentity(ctx, identity); identityErr == nil {
		return "", fmt.Errorf("MCP_SERVER_IDENTITY_CONFLICT: %s is already managed by %s", identity, existing.ID)
	} else if !errors.Is(identityErr, gorm.ErrRecordNotFound) {
		return "", identityErr
	}
	server, err := r.Repository.CreateServer(ctx, input)
	if err != nil {
		return "", err
	}
	if err := r.writeStdioEnvironment(ctx, server.ID, binding.Env); err != nil {
		_ = r.Repository.DeleteServer(context.WithoutCancel(ctx), server.ID)
		return "", err
	}
	if err := r.writeCredential(ctx, server.ID, binding); err != nil {
		_ = r.Repository.DeleteServer(context.WithoutCancel(ctx), server.ID)
		return "", err
	}
	if err := r.syncBindingCapabilities(ctx, server.ID, binding); err != nil {
		_ = r.Repository.DeleteServer(context.WithoutCancel(ctx), server.ID)
		return "", err
	}
	return server.ID, nil
}

func (r *Runtime) writeStdioEnvironment(ctx context.Context, serverID string, env map[string]string) error {
	oldReference := ""
	if record, err := r.Repository.CredentialReference(ctx, serverID, "stdio_env"); err == nil {
		oldReference = record.SecretReference
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if len(env) == 0 {
		if oldReference == "" {
			return nil
		}
		if _, err := r.Repository.DeleteCredentialReference(ctx, serverID, "stdio_env"); err != nil {
			return err
		}
		return r.Secrets.Delete(ctx, oldReference)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	reference, err := r.Secrets.Put(ctx, "mcp/"+serverID+"/stdio_env", raw)
	if err != nil {
		return err
	}
	if _, err := r.Repository.PutCredentialReference(ctx, serverID, "stdio_env", reference, "", nil); err != nil {
		_ = r.Secrets.Delete(context.WithoutCancel(ctx), reference)
		return err
	}
	if oldReference != "" && oldReference != reference {
		if err := r.Secrets.Delete(ctx, oldReference); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) writeCredential(ctx context.Context, serverID string, binding kernel.MCPBindingDescriptor) error {
	credentialType := strings.TrimSpace(binding.CredentialType)
	if credentialType == "" {
		switch strings.ToLower(strings.TrimSpace(binding.AuthType)) {
		case "bearer_token", "custom_headers":
			credentialType = strings.ToLower(strings.TrimSpace(binding.AuthType))
		}
	}
	if credentialType == "" {
		return nil
	}
	oldReference := ""
	if record, err := r.Repository.CredentialReference(ctx, serverID, credentialType); err == nil {
		oldReference = record.SecretReference
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if len(binding.Credential) == 0 {
		if oldReference == "" {
			return nil
		}
		if _, err := r.Repository.DeleteCredentialReference(ctx, serverID, credentialType); err != nil {
			return err
		}
		return r.Secrets.Delete(ctx, oldReference)
	}
	reference, err := r.Secrets.Put(ctx, "mcp/"+serverID+"/"+credentialType, binding.Credential)
	if err != nil {
		return err
	}
	if _, err := r.Repository.PutCredentialReference(ctx, serverID, credentialType, reference, "", nil); err != nil {
		_ = r.Secrets.Delete(context.WithoutCancel(ctx), reference)
		return err
	}
	if oldReference != "" && oldReference != reference {
		if err := r.Secrets.Delete(ctx, oldReference); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) syncBindingCapabilities(ctx context.Context, serverID string, binding kernel.MCPBindingDescriptor) error {
	capabilities := map[string]bool{
		"private_network": binding.AllowPrivateNetwork,
	}
	for name, enabled := range binding.Capabilities {
		capabilities[strings.ToLower(strings.TrimSpace(name))] = enabled
	}
	for name, enabled := range capabilities {
		if _, err := r.Repository.SetServerCapability(ctx, serverID, name, enabled, json.RawMessage(`{}`)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) removeServer(ctx context.Context, serverID string) error {
	_ = r.Connections.Disconnect(ctx, serverID)
	_ = r.Tools.UnregisterServer(ctx, serverID)
	references, err := r.Repository.CredentialReferences(ctx, serverID)
	if err != nil {
		return err
	}
	if err := r.Repository.DeleteServer(ctx, serverID); err != nil {
		return err
	}
	for _, reference := range references {
		if err := r.Secrets.Delete(ctx, reference); err != nil {
			return err
		}
	}
	return nil
}

func contributionServerInput(binding kernel.MCPBindingDescriptor, enabled bool) mcp.ServerInput {
	transport := strings.ToLower(strings.TrimSpace(binding.Transport))
	if transport == "" {
		if strings.TrimSpace(binding.Endpoint) != "" {
			transport = "streamable_http"
		} else {
			transport = "stdio"
		}
	}
	name := strings.TrimSpace(binding.ServerName)
	if name == "" {
		name = strings.TrimSpace(binding.ContributionID)
	}
	displayName := strings.TrimSpace(binding.DisplayName)
	if displayName == "" {
		displayName = name
	}
	authType := strings.TrimSpace(binding.AuthType)
	if authType == "" && len(binding.Env) > 0 {
		authType = "stdio_env"
	}
	return mcp.ServerInput{
		Name:                name,
		DisplayName:         displayName,
		Description:         strings.TrimSpace(binding.Description),
		Transport:           transport,
		Endpoint:            strings.TrimSpace(binding.Endpoint),
		Command:             strings.TrimSpace(binding.Command),
		Args:                append([]string(nil), binding.Args...),
		WorkDir:             strings.TrimSpace(binding.WorkDir),
		AuthType:            authType,
		Source:              "package",
		Enabled:             enabled,
		OwnerKind:           "package",
		OwnerExtensionID:    strings.TrimSpace(binding.ExtensionID),
		OwnerModuleID:       strings.TrimSpace(binding.ModuleID),
		OwnerContributionID: strings.TrimSpace(binding.ContributionID),
		RuntimePlacement:    strings.TrimSpace(binding.RuntimePlacement),
		PackageGeneration:   binding.PackageGeneration,
	}
}

var _ kernel.MCPBindingService = (*Runtime)(nil)
