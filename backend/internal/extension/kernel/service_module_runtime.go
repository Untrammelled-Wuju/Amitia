package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/native_companion"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/runtimeport"
)

func (r *Runtime) registerServiceModuleDefinitions(ctx context.Context, extID domain.ExtensionID, version domain.SemanticVersion, modules []domain.ModuleDefinition) error {
	if r.container == nil {
		return nil
	}
	return r.container.registerServiceModuleDefinitions(ctx, extID, version, modules)
}

func (c *Container) registerServiceModuleDefinitions(ctx context.Context, extID domain.ExtensionID, version domain.SemanticVersion, modules []domain.ModuleDefinition) error {
	if c == nil || c.ServiceDefinitions == nil || c.NodeEnvironmentResolver == nil {
		return nil
	}
	hasServiceModule := false
	for _, module := range modules {
		if module.Runtime != nil && module.Runtime.Type == domain.RuntimeTypeService {
			hasServiceModule = true
			break
		}
	}
	if !hasServiceModule {
		return nil
	}
	definition, err := c.DefinitionRepository.GetExtension(ctx, extID, version)
	if err != nil {
		return fmt.Errorf("get extension definition: %w", err)
	}
	bundlePath := resolveExtensionBundlePath(c.ExtRoot, string(extID))
	if bundlePath == "" {
		return fmt.Errorf("service bundle unavailable for %s", extID)
	}
	nodeEnv, err := c.NodeEnvironmentResolver.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("resolve managed node: %w", err)
	}
	for _, module := range modules {
		if module.Runtime == nil || module.Runtime.Type != domain.RuntimeTypeService {
			continue
		}
		pluginDataPath := filepath.Join(c.ExtRoot, "plugin-data", safePluginDataID(string(extID)))
		serviceDefinition, buildErr := buildServiceModuleRuntimeDefinition(definition, module, bundlePath, pluginDataPath, nodeEnv.NodeBinary)
		if buildErr != nil {
			return fmt.Errorf("build service definition %s: %w", module.ID, buildErr)
		}
		c.applyChannelSandboxPolicy(module, serviceDefinition)
		if err := c.registerServiceRuntimeDefinition(serviceDefinition); err != nil {
			return fmt.Errorf("register service definition %s: %w", module.ID, err)
		}
	}
	return nil
}

func (c *Container) registerServiceRuntimeDefinition(definition *trusted_service.ServiceRuntimeDefinition) error {
	if c == nil || definition == nil || c.ServiceDefinitions == nil {
		return nil
	}
	if c.TrustedServiceSupervisor != nil {
		if _, err := c.TrustedServiceSupervisor.GetDefinition(definition.ServiceID); err == nil {
			if err := c.TrustedServiceSupervisor.Unregister(definition.ServiceID); err != nil {
				return err
			}
		}
		if err := c.TrustedServiceSupervisor.Register(definition); err != nil {
			return err
		}
	}
	c.ServiceDefinitions.Register(definition)
	return nil
}

func buildServiceModuleRuntimeDefinition(extension domain.ExtensionDefinition, module domain.ModuleDefinition, bundlePath, pluginDataPath, nodePath string) (*trusted_service.ServiceRuntimeDefinition, error) {
	if module.Runtime == nil || module.Runtime.Type != domain.RuntimeTypeService {
		return nil, fmt.Errorf("module is not a service runtime")
	}
	moduleID := strings.TrimSpace(string(module.ID))
	entryPoint := strings.TrimSpace(module.Runtime.EntryPoint)
	if moduleID == "" || entryPoint == "" {
		return nil, fmt.Errorf("service module id and entry point are required")
	}
	moduleRoot := filepath.Join(bundlePath, "modules", moduleID)
	entryPath := filepath.Join(moduleRoot, filepath.FromSlash(entryPoint))
	if _, err := os.Stat(entryPath); err != nil {
		return nil, fmt.Errorf("service entrypoint unavailable: %w", err)
	}
	nodeHash, err := sha256Path(nodePath)
	if err != nil {
		return nil, fmt.Errorf("hash managed node: %w", err)
	}
	companions, err := native_companion.Resolve(moduleRoot, module.Runtime.NativeCompanions, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, fmt.Errorf("resolve native companions: %w", err)
	}
	companionJSON, err := json.Marshal(companions)
	if err != nil {
		return nil, fmt.Errorf("encode native companions: %w", err)
	}
	dependencies := make([]trusted_service.LibraryDep, 0, len(companions))
	for _, companion := range companions {
		dependencies = append(dependencies, trusted_service.LibraryDep{
			Name:     companion.ID,
			Path:     companion.Path,
			Sha256:   companion.SHA256,
			Required: true,
		})
	}
	envTemplate := map[string]string{
		"AMITIA_NATIVE_COMPANIONS_VERSION": "1",
		"AMITIA_NATIVE_COMPANIONS":         string(companionJSON),
		"AMITIA_HOST_ACCESS_VERSION":       "1",
		"AMITIA_PLUGIN_DATA_DIR":           pluginDataPath,
		"AMITIA_HOST_FILESYSTEM_MODE":      "none",
		"AMITIA_HOST_FILESYSTEM_ROOTS":     "",
		"AMITIA_HOST_NETWORK_MODE":         "none",
		"AMITIA_HOST_PROCESS_MODE":         "none",
		"AMITIA_HOST_PROCESS_LIMIT":        "0",
	}
	if module.Runtime.HostAccess.Filesystem.Mode != "" {
		envTemplate["AMITIA_HOST_FILESYSTEM_MODE"] = module.Runtime.HostAccess.Filesystem.Mode
	}
	rootMap := map[string]string{}
	filesystemRoots := make([]string, 0, len(module.Runtime.HostAccess.Filesystem.Roots))
	for _, root := range module.Runtime.HostAccess.Filesystem.Roots {
		switch strings.ToLower(strings.TrimSpace(root)) {
		case "package":
			rootMap["package"] = bundlePath
			filesystemRoots = append(filesystemRoots, bundlePath)
		case "plugin_data":
			rootMap["plugin_data"] = pluginDataPath
			filesystemRoots = append(filesystemRoots, pluginDataPath)
		}
	}
	envTemplate["AMITIA_HOST_FILESYSTEM_ROOTS"] = strings.Join(filesystemRoots, string(os.PathListSeparator))
	if encodedRoots, marshalErr := json.Marshal(rootMap); marshalErr == nil {
		envTemplate["AMITIA_HOST_FILESYSTEM_ROOTS_JSON"] = string(encodedRoots)
	}
	if module.Runtime.HostAccess.Network.Mode != "" {
		envTemplate["AMITIA_HOST_NETWORK_MODE"] = module.Runtime.HostAccess.Network.Mode
	}
	if module.Runtime.HostAccess.Process.Mode != "" {
		envTemplate["AMITIA_HOST_PROCESS_MODE"] = module.Runtime.HostAccess.Process.Mode
	}
	if module.Runtime.HostAccess.Process.MaxProcesses > 0 {
		envTemplate["AMITIA_HOST_PROCESS_LIMIT"] = strconv.Itoa(module.Runtime.HostAccess.Process.MaxProcesses)
	}
	for key, value := range dynamicChannelPortEnv(module) {
		envTemplate[key] = value
	}
	serviceTrust := strings.ToLower(strings.TrimSpace(extension.Publisher.TrustLevel))
	switch serviceTrust {
	case string(trusted_service.TrustLevelOfficial):
	case "user_trusted":
		serviceTrust = string(trusted_service.TrustLevelTrusted)
	case string(trusted_service.TrustLevelTrusted):
	default:
		return nil, fmt.Errorf("publisher trust %q is insufficient for a service runtime", extension.Publisher.TrustLevel)
	}
	if err := os.MkdirAll(pluginDataPath, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin data directory: %w", err)
	}
	networkMode := strings.ToLower(strings.TrimSpace(module.Runtime.HostAccess.Network.Mode))
	if networkMode == "" {
		networkMode = "none"
	}
	maxSubprocesses := module.Runtime.MaxSubprocesses
	if strings.EqualFold(strings.TrimSpace(module.Runtime.HostAccess.Process.Mode), "direct") && module.Runtime.HostAccess.Process.MaxProcesses > 0 {
		maxSubprocesses = module.Runtime.HostAccess.Process.MaxProcesses
	}
	return &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   string(runtime_supervisor.BuildRuntimeDefinitionID(string(extension.ID), moduleID, domain.RuntimeTypeService)),
		ExtensionID: string(extension.ID),
		ModuleID:    moduleID,
		Name:        module.Name.Default,
		Description: module.Description.Default,
		Publisher:   extension.Publisher.PublisherID,
		TrustLevel:  serviceTrust,
		Executables: []trusted_service.PlatformExecutable{{
			Platform: trusted_service.CurrentPlatform(),
			Path:     nodePath,
			Sha256:   nodeHash,
			ArgsTemplate: []string{
				entryPath,
			},
			Signature: trusted_service.BinarySignature{
				Algorithm: "managed-node",
				Value:     nodeHash,
				Trusted:   true,
			},
			Dependencies: dependencies,
			EnvTemplate:  envTemplate,
		}},
		Protocol:       "plain",
		InstancePolicy: "singleton",
		HealthCheck: trusted_service.ServiceHealthCheck{
			Type:                "process",
			Interval:            10 * time.Second,
			Timeout:             5 * time.Second,
			GracePeriod:         5 * time.Second,
			MaxConsecutiveFails: 3,
		},
		Recovery: trusted_service.ServiceRecoveryPolicy{
			MaxRestarts:          3,
			RestartDelay:         time.Second,
			BackoffMultiplier:    2,
			MaxRestartDelay:      30 * time.Second,
			QuarantineOnFail:     true,
			RecoveryDecisionMode: trusted_service.RecoveryDecisionAutoRestart,
		},
		Shutdown: trusted_service.ServiceShutdownPolicy{
			GracePeriod:     5 * time.Second,
			KillTimeout:     5 * time.Second,
			CleanupChildren: true,
			RemoveTempDir:   true,
		},
		Limits: trusted_service.ServiceResourceLimits{
			MaxMemoryMB:     module.Runtime.Memory / (1024 * 1024),
			MaxSubprocesses: maxSubprocesses,
		},
		Network: trusted_service.ServiceNetworkPolicy{
			Mode: networkMode,
		},
		DefinitionVersion: 1,
	}, nil
}

func safePluginDataID(value string) string {
	return strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(value)
}

func dynamicChannelPortEnv(module domain.ModuleDefinition) map[string]string {
	if module.Provider == nil || module.Provider.Metadata == nil {
		return nil
	}
	metadata := module.Provider.Metadata
	channelID := strings.TrimSpace(channelStringValue(metadata["channelId"]))
	if channelID == "" {
		channelID = strings.TrimSpace(channelStringValue(metadata["id"]))
	}
	if labels, ok := metadata["labels"].(map[string]any); ok {
		if channelID == "" {
			channelID = strings.TrimSpace(channelStringValue(labels["channelId"]))
		}
	}
	transport := channelMetadataMap(metadata, "transport")
	if transport == nil {
		transport = channelMetadataMap(metadata, "sidecar")
	}
	portEnv := ""
	if transport != nil {
		portEnv = strings.TrimSpace(channelStringValue(transport["portEnv"]))
	}
	if portEnv == "" {
		switch channelID {
		case "qq":
			portEnv = "QQ_SIDECAR_PORT"
		case "wechat":
			portEnv = "SIDECAR_PORT"
		case "wechat_ipad860":
			portEnv = "AMITIA_WECHAT_IPAD860_PORT"
		}
	}
	if channelID == "" || portEnv == "" {
		return nil
	}
	port, err := runtimeport.AllocateLoopback()
	if err != nil {
		return nil
	}
	env := map[string]string{portEnv: strconv.Itoa(port)}
	runtimeport.SetChannelEndpoint(channelID, fmt.Sprintf("http://127.0.0.1:%d", port))
	if channelID == "wechat_ipad860" {
		if enginePort, engineErr := runtimeport.AllocateLoopback(); engineErr == nil {
			env["AMITIA_WECHAT_IPAD860_ENGINE_PORT"] = strconv.Itoa(enginePort)
		}
	}
	return env
}

func channelMetadataMap(metadata map[string]any, key string) map[string]any {
	value, _ := metadata[key].(map[string]any)
	return value
}

func channelStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func sha256Path(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (c *Container) applyChannelSandboxPolicy(module domain.ModuleDefinition, definition *trusted_service.ServiceRuntimeDefinition) {
	if c == nil || definition == nil {
		return
	}
	channelID := moduleChannelID(module)
	if c.ChannelSandbox == nil || channelID == "" {
		return
	}
	policy, ok := c.ChannelSandbox.ChannelSandboxPolicy(channelID)
	if !ok || !policy.Enabled || strings.TrimSpace(policy.Root) == "" {
		return
	}
	definition.Network.Enforce = true
	if policy.AllowNetwork {
		definition.Network.Mode = "unrestricted"
		definition.Network.AllowOutbound = true
		definition.Network.AllowInbound = false
		definition.Network.LoopbackOnly = false
	} else {
		definition.Network.Mode = "loopback"
		definition.Network.AllowOutbound = false
		definition.Network.AllowInbound = true
		definition.Network.LoopbackOnly = true
	}
	definition.SandboxWritableRoot = policy.Root
}

func moduleChannelID(module domain.ModuleDefinition) string {
	if module.Provider == nil {
		return ""
	}
	if channelID := strings.TrimSpace(module.Provider.Labels["channelId"]); channelID != "" {
		return channelID
	}
	metadata := module.Provider.Metadata
	if metadata == nil {
		return ""
	}
	if channelID := strings.TrimSpace(channelStringValue(metadata["channelId"])); channelID != "" {
		return channelID
	}
	if labels, ok := metadata["labels"].(map[string]any); ok {
		if channelID := strings.TrimSpace(channelStringValue(labels["channelId"])); channelID != "" {
			return channelID
		}
	}
	if raw, ok := metadata["labels"]; ok && raw != nil {
		if encoded, err := json.Marshal(raw); err == nil {
			var parsed map[string]any
			if json.Unmarshal(encoded, &parsed) == nil {
				return strings.TrimSpace(channelStringValue(parsed["channelId"]))
			}
		}
	}
	return ""
}
