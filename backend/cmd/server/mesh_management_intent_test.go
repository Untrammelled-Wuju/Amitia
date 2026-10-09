package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestDeviceManagementRealTLSRequiresOriginalAuthorityAcrossPolicyABA(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, other := newThreeCoreFixture(t, "management-a", schemas), newThreeCoreFixture(t, "management-b", schemas), newThreeCoreFixture(t, "management-other", schemas)
	pairThreeCoreFixtures(t, a, b)
	pairThreeCoreFixtures(t, other, b)
	readHeaders := func() map[string]string {
		var state struct {
			CoreID string              `json:"coreId"`
			Policy coordination.Policy `json:"policy"`
		}
		if err := json.Unmarshal(b.request(t, a, http.MethodGet, "/api/device-mesh/v1/coordination/me", nil, 200), &state); err != nil || state.CoreID != b.core || state.Policy.DeviceID != a.device.DeviceID.String() {
			t.Fatal("management snapshot invalid", err)
		}
		return map[string]string{security.ExpectedCoreHeader: state.CoreID, security.ExpectedConfigurationPolicyHeader: fmt.Sprintf("%d:%d:%d", state.Policy.ProviderEpoch, state.Policy.ModeRevision, state.Policy.PermissionRevision)}
	}
	original := readHeaders()
	grantPath := "/api/device-mesh/v1/business/devices/" + a.device.DeviceID.String() + "/grants"
	payload := map[string]any{"callerId": other.device.DeviceID.String(), "capability": "ai.chat", "allowed": true, "expectedRevision": 0, "expectedCoreId": b.core}
	b.request(t, a, http.MethodPut, grantPath, payload, 409, map[string]string{})
	var granted struct {
		Grant coordination.CapabilityGrant `json:"grant"`
	}
	if err := json.Unmarshal(b.request(t, a, http.MethodPut, grantPath, payload, 200, original), &granted); err != nil {
		t.Fatal(err)
	}
	if granted.Grant.Revision != 1 || !granted.Grant.Allowed || granted.Grant.TargetID != a.device.DeviceID.String() {
		t.Fatalf("invalid actual grant acknowledgement: %+v", granted)
	}
	payload["capability"] = "task.execute"
	b.request(t, a, http.MethodPut, grantPath, payload, 409, original)
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "selectedRole": "one", "expectedRevision": policy.ModeRevision}, 200)
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	policy, err = b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	administrator := readHeaders()
	pending := newThreeCoreFixture(t, "management-pending", schemas)
	_, token, err := b.pairing.CreateOffer(t.Context(), b.device.DeviceID, time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	claimPayload, err := json.Marshal(map[string]any{"endpoint": b.endpoint, "offerToken": token, "label": "pending"})
	if err != nil {
		t.Fatal(err)
	}
	claim := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/pairing/claim", bytes.NewReader(claimPayload))
	claim.RemoteAddr = "127.0.0.1:40000"
	claim.Header.Set("Content-Type", "application/json")
	claimResult := httptest.NewRecorder()
	pending.router.ServeHTTP(claimResult, claim)
	if claimResult.Code != http.StatusAccepted {
		t.Fatalf("pending claim status %d", claimResult.Code)
	}
	approvals, err := b.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 {
		t.Fatal("expected actual pending approval", err)
	}
	approval := approvals[0]
	adminPath := "/api/device-mesh/v1/devices/" + a.device.DeviceID.String() + "/administrator"
	b.request(t, a, http.MethodPut, adminPath, map[string]any{"grant": false, "expectedRevision": policy.PermissionRevision}, 200, administrator)
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	policy, err = b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, adminPath, map[string]any{"grant": false, "expectedRevision": policy.PermissionRevision}, 409, administrator)
	approvalPath := "/api/device-mesh/v1/pairing/approvals/" + approval.RequestID
	b.request(t, a, http.MethodPut, approvalPath, map[string]any{"allow": true, "expectedRevision": approval.Revision}, 409, administrator)
	approvals, err = b.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 || approvals[0].Revision != approval.Revision {
		t.Fatal("stale actor approved a pending device", err)
	}
	b.request(t, a, http.MethodPut, approvalPath, map[string]any{"allow": true, "expectedRevision": approval.Revision}, 200)
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": false, "expectedRevision": policy.ModeRevision}, 200)
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision}, 409, original)
	revokePath := "/api/device-mesh/v1/devices/" + a.device.DeviceID.String()
	b.request(t, a, http.MethodDelete, revokePath, nil, 409, original)
	if err := b.services.DeviceMesh.DeviceReg.RequireTrustedDevice(t.Context(), runtimeidentity.SpaceID(b.core), a.device.DeviceID); err != nil {
		t.Fatal("stale revoke changed actual trust", err)
	}
	foreign := readHeaders()
	foreign[security.ExpectedCoreHeader] = "foreign-core"
	b.request(t, a, http.MethodPut, grantPath, payload, 409, foreign)
	b.request(t, a, http.MethodPost, revokePath+"/runtimes/unknown/probe", nil, 409, map[string]string{})
	b.request(t, a, http.MethodPost, revokePath+"/runtimes/unknown/probe", nil, 409, original)
	b.request(t, a, http.MethodDelete, revokePath, nil, 200)
	if err := b.services.DeviceMesh.DeviceReg.RequireTrustedDevice(t.Context(), runtimeidentity.SpaceID(b.core), a.device.DeviceID); err == nil {
		t.Fatal("acknowledged self revoke did not revoke actual device")
	}
}
