package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
)

type EmergencySecretLeaseAdapter struct {
	adapter *gamehostsecret.SecretLeaseAdapter
}

func NewEmergencySecretLeaseAdapter(adapter *gamehostsecret.SecretLeaseAdapter) *EmergencySecretLeaseAdapter {
	return &EmergencySecretLeaseAdapter{adapter: adapter}
}

func (a *EmergencySecretLeaseAdapter) RevokeRuntimeLeases(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return a.adapter.RevokeRuntimeLeases(string(runtimeID), "emergency_stop").RevokedCount, nil
}

func (a *EmergencySecretLeaseAdapter) CountRuntimeLeases(runtimeID domain.RuntimeInstanceID) int {
	return len(a.adapter.ActiveRuntimeLeases(string(runtimeID)))
}

var _ control.SecretLeaseRevoker = (*EmergencySecretLeaseAdapter)(nil)
var _ control.SecretLeaseVerifier = (*EmergencySecretLeaseAdapter)(nil)
