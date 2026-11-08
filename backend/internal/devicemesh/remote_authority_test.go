package devicemesh

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestOfflineAuthorityTargetCanCloseForProviderSwitch(t *testing.T) {
	runtime := &Runtime{
		Hub:                server.NewConnectionHub(),
		PendingInvocations: capability.NewPendingInvocationManager(),
	}
	err := runtime.fenceRemoteAuthority(t.Context(), "space", "target", 1, []string{"offline-source"})
	if !errors.Is(err, coordination.ErrAuthoritySourceOffline) || !errors.Is(err, ErrDeviceOffline) {
		t.Fatalf("offline authority fence was not classified: %v", err)
	}
}
