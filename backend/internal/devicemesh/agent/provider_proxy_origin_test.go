package agent

import "testing"

func TestProviderProxyOnlyPreservesOriginForRealtimeAuthorityEndpoints(t *testing.T) {
	for _, path := range []string{"/api/device-mesh/v1/business/realtime/tickets", "/api/device-mesh/v1/business/realtime/session", "/api/device-mesh/v1/business/realtime/invitations/aa464513-bdb4-4306-9bde-0c7c0397b7bf/accept"} {
		if !ownedRealtimeOriginPath(path) {
			t.Fatalf("realtime Origin lost: %s", path)
		}
	}
	for _, path := range []string{"/api/system/config", "/api/device-mesh/v1/business/realtime/invitations", "/api/device-mesh/v1/business/realtime/invitations/not-a-uuid/accept", "/api/device-mesh/v1/business/realtime/invitations/aa464513-bdb4-4306-9bde-0c7c0397b7bf/other/accept"} {
		if ownedRealtimeOriginPath(path) {
			t.Fatalf("unrelated endpoint preserved Origin: %s", path)
		}
	}
}
