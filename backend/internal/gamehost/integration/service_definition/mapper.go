package service_definition

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type DefinitionMapper struct{}

func NewDefinitionMapper() *DefinitionMapper {
	return &DefinitionMapper{}
}

func (m *DefinitionMapper) MapToDefinition(view ServiceRuntimeView) (*trusted_service.ServiceRuntimeDefinition, error) {
	if view.ExtensionID == "" {
		return nil, NewServiceDefinitionError(ErrDefinitionMappingFailed, "extension id must not be empty")
	}
	if view.ModuleID == "" {
		return nil, NewServiceDefinitionError(ErrDefinitionMappingFailed, "module id must not be empty")
	}
	if !IsValidServiceRuntimeType(view.RuntimeType) {
		return nil, NewServiceDefinitionErrorWithCause(ErrUnsupportedServiceKind,
			"unsupported service runtime type",
			NewServiceDefinitionError(ErrUnsupportedServiceKind, view.RuntimeType))
	}
	if view.EntryPoint == "" {
		return nil, NewServiceDefinitionError(ErrDefinitionMappingFailed, "entry point must not be empty for process service")
	}

	definitionID := view.ToDefinitionID()
	envCopy := cloneStringMap(view.Env)
	executablePath := view.ExecutablePath
	if executablePath == "" {
		executablePath = view.EntryPoint
	}
	integrityValue := view.IntegrityValue
	if integrityValue == "" && view.ExecutableSHA256 != "" {
		integrityValue = "sha256:" + view.ExecutableSHA256
	}

	trustLevel := authoritativeServiceTrustLevel(view.PublisherTrust)
	signatureTrusted := trustLevel.AllowedForService()

	return &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   definitionID,
		ExtensionID: view.ExtensionID,
		ModuleID:    view.ModuleID,
		Name:        view.Name,
		Description: view.Description,
		Publisher:   view.PublisherID,
		TrustLevel:  string(trustLevel),
		Executables: []trusted_service.PlatformExecutable{
			{
				Platform:     trusted_service.CurrentPlatform(),
				Path:         executablePath,
				Sha256:       view.ExecutableSHA256,
				Entry:        view.EntryPoint,
				ArgsTemplate: append([]string(nil), view.Arguments...),
				EnvTemplate:  envCopy,
				Signature: trusted_service.BinarySignature{
					Algorithm: "sha256-integrity",
					Value:     integrityValue,
					Signer:    view.PublisherID,
					Trusted:   signatureTrusted,
				},
				Dependencies: append([]trusted_service.LibraryDep(nil), view.Dependencies...),
			},
		},
		Protocol:       resolveProtocol(view),
		InstancePolicy: "single",
		HealthCheck: trusted_service.ServiceHealthCheck{
			Type:                "heartbeat",
			Interval:            30 * time.Second,
			Timeout:             10 * time.Second,
			GracePeriod:         5 * time.Second,
			MaxConsecutiveFails: 3,
		},
		Recovery: trusted_service.ServiceRecoveryPolicy{
			MaxRestarts:          0,
			RestartDelay:         1 * time.Second,
			BackoffMultiplier:    2.0,
			MaxRestartDelay:      30 * time.Second,
			QuarantineOnFail:     false,
			RecoveryDecisionMode: trusted_service.RecoveryDecisionExternal,
		},
		Shutdown: trusted_service.ServiceShutdownPolicy{
			GracePeriod:     10 * time.Second,
			KillTimeout:     30 * time.Second,
			CleanupChildren: true,
			RemoveTempDir:   false,
		},
		// Do not publish nominal limits that the current platform supervisor does
		// not enforce. Community plugins are separately blocked unless the process
		// manager reports complete CPU/memory/filesystem/network isolation.
		Limits:              trusted_service.ServiceResourceLimits{},
		Network:             resolveNetworkPolicy(view.Network),
		SandboxReadOnlyRoot: view.SandboxReadOnlyRoot,
		ManifestHash:        computeManifestHash(view),
		DefinitionVersion:   2,
		AutoStart:           false,
		AllowedNamespaces:   []string{},
	}, nil
}

func (m *DefinitionMapper) MapToViews(views []ServiceRuntimeView) ([]*trusted_service.ServiceRuntimeDefinition, []error) {
	var defs []*trusted_service.ServiceRuntimeDefinition
	var errs []error
	for _, view := range views {
		def, err := m.MapToDefinition(view)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		defs = append(defs, def)
	}
	return defs, errs
}

func computeManifestHash(view ServiceRuntimeView) string {
	h := sha256.New()
	h.Write([]byte(view.ExtensionID))
	h.Write([]byte(view.ModuleID))
	h.Write([]byte(view.RuntimeType))
	h.Write([]byte(view.EntryPoint))
	h.Write([]byte(view.ExecutablePath))
	h.Write([]byte(view.ExecutableSHA256))
	h.Write([]byte(view.IntegrityValue))
	h.Write([]byte(view.SandboxReadOnlyRoot))
	for _, dep := range view.Dependencies {
		h.Write([]byte(dep.Name))
		h.Write([]byte(dep.Path))
		h.Write([]byte(dep.Sha256))
		if dep.Required {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	}
	h.Write([]byte(view.Name))
	h.Write([]byte(view.PublisherID))
	h.Write([]byte(view.PublisherTrust))
	h.Write([]byte(view.Network.Mode))
	for _, arg := range view.Arguments {
		h.Write([]byte(arg))
	}
	if view.Network.Enforce {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if view.Network.AllowInbound {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if view.Network.AllowOutbound {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if view.Network.LoopbackOnly {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if view.Network.RequireProxy {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if view.Network.AuditAll {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	for _, domain := range view.Network.AllowedDomains {
		h.Write([]byte(domain))
	}
	for _, port := range view.Network.AllowedPorts {
		h.Write([]byte(fmt.Sprintf("%d", port)))
	}
	envKeys := make([]string, 0, len(view.Env))
	for k := range view.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		h.Write([]byte(k))
		h.Write([]byte(view.Env[k]))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func authoritativeServiceTrustLevel(raw string) trusted_service.TrustLevel {
	switch raw {
	case "official":
		return trusted_service.TrustLevelOfficial
	case "trusted", "user_trusted":
		return trusted_service.TrustLevelTrusted
	case "community":
		return trusted_service.TrustLevelCommunity
	default:
		return trusted_service.TrustLevelUnknown
	}
}

func resolveNetworkPolicy(policy trusted_service.ServiceNetworkPolicy) trusted_service.ServiceNetworkPolicy {
	if policy.Mode != "" || policy.Enforce || policy.AllowInbound || policy.AllowOutbound || policy.LoopbackOnly || policy.RequireProxy || len(policy.AllowedDomains) > 0 || len(policy.AllowedPorts) > 0 {
		policy.AllowedDomains = append([]string(nil), policy.AllowedDomains...)
		policy.AllowedPorts = append([]int(nil), policy.AllowedPorts...)
		return policy
	}
	// Missing network policy is deny-by-default. A plugin must explicitly request
	// loopback or unrestricted access; unsupported policies never degrade open.
	return trusted_service.ServiceNetworkPolicy{Mode: "none", Enforce: true}
}

func CanonicalizeEnv(env map[string]string) []string {
	if env == nil {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, k := range keys {
		result = append(result, fmt.Sprintf("%s=%s", k, env[k]))
	}
	return result
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

const TrustedServiceProtocol = "amitia-trusted-service/1"
const GameHostProtocol = "amitia-game-host/1"

func resolveProtocol(view ServiceRuntimeView) string {
	if view.Metadata != nil {
		if proto, ok := view.Metadata["protocol"]; ok && proto != "" {
			return proto
		}
	}
	return TrustedServiceProtocol
}
