package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/permission"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
)

const ServiceSecretUsePermission = "service.secret.use"

type SecretPermissionDecision = gamehostsecret.SecretPermissionDecision

type SecretPermissionGate interface {
	CheckSecretUse(
		ctx context.Context,
		extensionID string,
		pluginID string,
		runtimeID string,
		serviceID string,
		ref string,
	) (SecretPermissionDecision, error)
}

type EffectiveSecretPermissionGate struct {
	adapter *permission.EffectivePermissionAdapter
}

func NewEffectiveSecretPermissionGate(
	adapter *permission.EffectivePermissionAdapter,
) (*EffectiveSecretPermissionGate, error) {
	if adapter == nil {
		return nil, fmt.Errorf("effective permission adapter is required")
	}
	return &EffectiveSecretPermissionGate{adapter: adapter}, nil
}

func (g *EffectiveSecretPermissionGate) CheckSecretUse(
	ctx context.Context,
	extensionID string,
	pluginID string,
	runtimeID string,
	serviceID string,
	ref string,
) (SecretPermissionDecision, error) {
	if extensionID == "" {
		return SecretPermissionDecision{
			Allowed: false,
			Reason:  "extension id is required",
		}, fmt.Errorf("extension id is required")
	}
	if runtimeID == "" {
		return SecretPermissionDecision{
			Allowed: false,
			Reason:  "runtime id is required",
		}, fmt.Errorf("runtime id is required")
	}
	if serviceID == "" {
		return SecretPermissionDecision{
			Allowed: false,
			Reason:  "service id is required",
		}, fmt.Errorf("service id is required")
	}

	result := g.adapter.CheckServicePermission(ctx, runtimeID, pluginID, serviceID, ServiceSecretUsePermission)

	switch result.Decision {
	case permission.DecisionAllowed:
		return SecretPermissionDecision{
			Allowed: true,
			Reason:  "",
		}, nil
	case permission.DecisionRequireApproval:
		return SecretPermissionDecision{
			Allowed: false,
			Reason:  string(result.Reason),
		}, fmt.Errorf("permission requires approval: %s", result.Reason)
	default:
		return SecretPermissionDecision{
			Allowed: false,
			Reason:  string(result.Reason),
		}, fmt.Errorf("permission denied: %s - %s", result.Reason, result.Detail)
	}
}
