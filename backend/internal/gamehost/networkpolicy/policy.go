package networkpolicy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

var (
	ErrModeRequired        = errors.New("gamehost network policy: explicit mode required")
	ErrPermissionRequired  = errors.New("gamehost network policy: network permission required")
	ErrUnsupportedMode     = errors.New("gamehost network policy: unsupported mode")
	ErrPlatformUnsupported = errors.New("gamehost network policy: platform sandbox unsupported")
)

// Build converts the public Game Plugin network contract into the exact trusted
// service sandbox policy used at process launch. Package preflight and runtime
// graph construction both call this function so install-time support checks
// cannot drift from the production execution boundary.
func Build(spec *gameprotocol.PluginNetworkPolicy, permissions []string) (trusted_service.ServiceNetworkPolicy, error) {
	if spec == nil || strings.TrimSpace(spec.Mode) == "" {
		return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("%w: explicit game plugin network.mode is required", ErrModeRequired)
	}
	mode := strings.ToLower(strings.TrimSpace(spec.Mode))
	policy := trusted_service.ServiceNetworkPolicy{Mode: mode, Enforce: true}
	switch mode {
	case "none":
	case "loopback":
		policy.AllowOutbound = true
		policy.LoopbackOnly = true
	case "restricted":
		if !containsPermission(permissions, "service.network.request") {
			return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("%w: restricted outbound network requires service.network.request", ErrPermissionRequired)
		}
		policy.RequireProxy = true
		policy.AllowedDomains = append([]string(nil), spec.AllowedDomains...)
		policy.AllowedIPs = append([]string(nil), spec.AllowedIPs...)
		policy.AllowedPorts = append([]int(nil), spec.AllowedPorts...)
	case "unrestricted":
		if !containsPermission(permissions, "service.network.request") {
			return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("%w: unrestricted outbound network requires service.network.request", ErrPermissionRequired)
		}
		policy.AllowOutbound = true
	default:
		return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("%w: %q", ErrUnsupportedMode, mode)
	}
	if err := trusted_service.ValidateNetworkPolicySupport(policy); err != nil {
		return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("%w: game plugin network mode %q is unavailable on this host: %v", ErrPlatformUnsupported, mode, err)
	}
	return policy, nil
}

func containsPermission(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
