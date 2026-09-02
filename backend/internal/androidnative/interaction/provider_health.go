package interaction

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type ProviderHealthState string

const (
	ProviderStateSupported          ProviderHealthState = "SUPPORTED"
	ProviderStateUnavailable        ProviderHealthState = "UNAVAILABLE"
	ProviderStatePermissionRequired ProviderHealthState = "PERMISSION_REQUIRED"
	ProviderStateStarting           ProviderHealthState = "STARTING"
	ProviderStateReady              ProviderHealthState = "READY"
	ProviderStateDegraded           ProviderHealthState = "DEGRADED"
	ProviderStateFailed             ProviderHealthState = "FAILED"
)

type ProviderCapabilityHealth struct {
	State         ProviderHealthState `json:"state"`
	Reason        string              `json:"reason,omitempty"`
	LastProbeAt   time.Time           `json:"lastProbeAt"`
	LastSuccessAt *time.Time          `json:"lastSuccessAt,omitempty"`
	Provider      string              `json:"provider"`
	Permission    string              `json:"permission,omitempty"`
	Recoverable   bool                `json:"recoverable"`
}

type ProviderHealthProbe interface {
	ProbeHealth(ctx context.Context) ProviderCapabilityHealth
}

func newProviderHealth(provider string, state ProviderHealthState, reason, permission string, recoverable bool) ProviderCapabilityHealth {
	now := time.Now().UTC()
	health := ProviderCapabilityHealth{
		State:       state,
		Reason:      reason,
		LastProbeAt: now,
		Provider:    provider,
		Permission:  permission,
		Recoverable: recoverable,
	}
	if state == ProviderStateReady || state == ProviderStateDegraded || state == ProviderStateSupported {
		health.LastSuccessAt = &now
	}
	return health
}

func providerReady(health ProviderCapabilityHealth) bool {
	return health.State == ProviderStateReady || health.State == ProviderStateDegraded
}

func providerUsable(health ProviderCapabilityHealth) bool {
	return providerReady(health) || health.State == ProviderStateSupported
}

func healthFromBridgeResponse(provider string, result map[string]any, err error) ProviderCapabilityHealth {
	if err != nil {
		return newProviderHealth(provider, ProviderStateFailed, err.Error(), "", true)
	}
	stateText, _ := result["state"].(string)
	reason, _ := result["reason"].(string)
	permission, _ := result["permissionState"].(string)
	if permission == "" {
		permission, _ = result["authorizationState"].(string)
	}
	userActionRequired, _ := result["userActionRequired"].(bool)
	connected, hasConnected := result["connected"].(bool)

	lower := strings.ToLower(stateText)
	switch {
	case userActionRequired || strings.Contains(lower, "permission") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "authorization_required"):
		return newProviderHealth(provider, ProviderStatePermissionRequired, reason, permission, true)
	case hasConnected && connected:
		return newProviderHealth(provider, ProviderStateReady, reason, permission, true)
	case hasConnected && !connected:
		return newProviderHealth(provider, ProviderStateUnavailable, "provider is not connected", permission, true)
	case strings.Contains(lower, "ready") || strings.Contains(lower, "authorized") || strings.Contains(lower, "connected") || strings.Contains(lower, "granted"):
		return newProviderHealth(provider, ProviderStateReady, reason, permission, true)
	case strings.Contains(lower, "starting") || strings.Contains(lower, "binding") || strings.Contains(lower, "pending") || strings.Contains(lower, "enabled_not_connected"):
		return newProviderHealth(provider, ProviderStateStarting, reason, permission, true)
	case strings.Contains(lower, "degraded") || strings.Contains(lower, "offline") || strings.Contains(lower, "ambiguous"):
		return newProviderHealth(provider, ProviderStateDegraded, reason, permission, true)
	case lower == "" && len(result) > 0:
		return newProviderHealth(provider, ProviderStateSupported, reason, permission, true)
	default:
		if reason == "" && stateText != "" {
			reason = stateText
		}
		return newProviderHealth(provider, ProviderStateUnavailable, reason, permission, true)
	}
}

func probeNativeStatus(ctx context.Context, bridge nativebridge.Bridge, provider, operation string) ProviderCapabilityHealth {
	if bridge == nil {
		return newProviderHealth(provider, ProviderStateUnavailable, "native bridge is not connected", "", true)
	}
	switch bridge.Health(ctx) {
	case nativebridge.HealthUnhealthy:
		return newProviderHealth(provider, ProviderStateFailed, "native bridge is unhealthy", "", true)
	case nativebridge.HealthUnknown:
		// Continue with the provider-specific status call; a relay may become ready
		// before the coarse bridge health is refreshed.
	}
	resp, err := bridge.Execute(ctx, nativebridge.Request{
		ProtocolVersion: 1,
		Operation:       operation,
		Platform:        "android",
		Payload:         map[string]any{},
	})
	if err != nil {
		return newProviderHealth(provider, ProviderStateFailed, err.Error(), "", true)
	}
	if resp.Error != nil {
		return newProviderHealth(provider, ProviderStateFailed, fmt.Sprintf("%s: %s", resp.Error.Code, resp.Error.Message), "", true)
	}
	if resp.Status != "" && !strings.EqualFold(resp.Status, "success") {
		return newProviderHealth(provider, ProviderStateFailed, "provider status request failed", "", true)
	}
	return healthFromBridgeResponse(provider, resp.Result, nil)
}

func probeProviderHealth(ctx context.Context, providerName string, provider any) ProviderCapabilityHealth {
	if provider == nil {
		return newProviderHealth(providerName, ProviderStateUnavailable, "provider not configured", "", true)
	}
	if probe, ok := provider.(ProviderHealthProbe); ok {
		health := probe.ProbeHealth(ctx)
		if health.Provider == "" {
			health.Provider = providerName
		}
		return health
	}
	// Third-party/test adapters may not expose a probe yet. Keep the adapter
	// usable for compatibility but distinguish this from a proven READY state.
	return newProviderHealth(providerName, ProviderStateSupported, "provider does not expose a health probe", "", true)
}
