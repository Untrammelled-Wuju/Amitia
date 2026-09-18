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
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/native_companion"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func (r *Runtime) registerServiceModuleDefinitions(ctx context.Context, extID domain.ExtensionID, version domain.SemanticVersion, modules []domain.ModuleDefinition) error {
	if r.container == nil || r.container.ServiceDefinitions == nil || r.container.NodeEnvironmentResolver == nil {
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
	definition, err := r.container.DefinitionRepository.GetExtension(ctx, extID, version)
	if err != nil {
		return fmt.Errorf("get extension definition: %w", err)
	}
	bundlePath := resolveExtensionBundlePath(r.container.ExtRoot, string(extID))
	if bundlePath == "" {
		return nil
	}
	nodeEnv, err := r.container.NodeEnvironmentResolver.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("resolve managed node: %w", err)
	}
	for _, module := range modules {
		if module.Runtime == nil || module.Runtime.Type != domain.RuntimeTypeService {
			continue
		}
		serviceDefinition, buildErr := buildServiceModuleRuntimeDefinition(definition, module, bundlePath, nodeEnv.NodeBinary)
		if buildErr != nil {
			return fmt.Errorf("build service definition %s: %w", module.ID, buildErr)
		}
		if err := r.container.registerServiceRuntimeDefinition(serviceDefinition); err != nil {
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

func buildServiceModuleRuntimeDefinition(extension domain.ExtensionDefinition, module domain.ModuleDefinition, bundlePath, nodePath string) (*trusted_service.ServiceRuntimeDefinition, error) {
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
	serviceTrust := strings.ToLower(strings.TrimSpace(extension.Publisher.TrustLevel))
	switch serviceTrust {
	case string(trusted_service.TrustLevelOfficial):
	case "user_trusted":
		serviceTrust = string(trusted_service.TrustLevelTrusted)
	case string(trusted_service.TrustLevelTrusted):
	default:
		return nil, fmt.Errorf("publisher trust %q is insufficient for a service runtime", extension.Publisher.TrustLevel)
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
			EnvTemplate: map[string]string{
				"AMITIA_NATIVE_COMPANIONS_VERSION": "1",
				"AMITIA_NATIVE_COMPANIONS":         string(companionJSON),
			},
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
			MaxSubprocesses: module.Runtime.MaxSubprocesses,
		},
		Network: trusted_service.ServiceNetworkPolicy{
			Mode:         "loopback",
			LoopbackOnly: true,
		},
		DefinitionVersion: 1,
	}, nil
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
