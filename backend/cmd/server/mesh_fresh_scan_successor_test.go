package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/agent"
)

func TestFreshDeviceScansAlreadyBoundProviderAndRequiresDirectCoreAdmission(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, c := newThreeCoreFixture(t, "fresh-scan-a", schemas), newThreeCoreFixture(t, "fresh-scan-b", schemas), newThreeCoreFixture(t, "fresh-scan-c", schemas)
	pairThreeCoreFixtures(t, b, c)
	bPolicy, err := c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, b.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	c.request(t, b, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "selectedRole": "one", "expectedRevision": bPolicy.ModeRevision}, http.StatusOK)
	bPolicy, err = c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, b.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), c.core, b.device.DeviceID.String(), bPolicy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	if credential, err := a.local.LoadCredential(); err != nil || credential != nil {
		t.Fatal("A was already bound before scanning B", err)
	}
	_, token, err := b.pairing.CreateOffer(t.Context(), b.device.DeviceID, time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	scan := func(expected int) []byte {
		payload, err := json.Marshal(map[string]any{"endpoint": b.endpoint, "offerToken": token, "label": a.core})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/pairing/claim", bytes.NewReader(payload))
		request.RemoteAddr = "127.0.0.1:40000"
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		a.router.ServeHTTP(recorder, request)
		if recorder.Code != expected {
			t.Fatalf("fresh A scans B: status=%d expected=%d body=%s", recorder.Code, expected, recorder.Body.String())
		}
		return recorder.Body.Bytes()
	}
	scan(http.StatusAccepted)
	bApprovals, err := b.pairing.PendingApprovals(t.Context())
	if err != nil || len(bApprovals) != 1 || bApprovals[0].DeviceID != a.device.DeviceID.String() {
		t.Fatal("B did not independently approve fresh A", err)
	}
	if err := b.pairing.DecideApproval(t.Context(), bApprovals[0].RequestID, bApprovals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	var accepted struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(scan(http.StatusOK), &accepted); err != nil || accepted.Ticket == "" {
		t.Fatal("B did not issue an independent bootstrap ticket", err)
	}
	_, err = a.local.BindProvider(t.Context(), agent.BindingRequest{CloudBaseURL: b.endpoint.URL, Fingerprint: b.endpoint.Fingerprint, CoreID: b.core, BootstrapTicket: accepted.Ticket})
	var failure *agent.BindingError
	if !errors.As(err, &failure) || failure.Code != "provider_business_not_ready" {
		t.Fatal("unready B became active rather than remaining candidate", err)
	}
	if credential, err := a.local.LoadCredential(); err != nil || credential != nil {
		t.Fatal("unready B became A canonical provider", err)
	}
	if err := a.local.FollowSuccessor(t.Context()); err == nil {
		t.Fatal("B administrator permission bypassed C admission")
	}
	approvals, err := c.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 || approvals[0].DeviceID != a.device.DeviceID.String() || approvals[0].RuntimeID != a.device.RuntimeID.String() {
		t.Fatalf("C did not independently request A admission: %+v %v", approvals, err)
	}
	pending, err := a.local.LoadCredential()
	if err != nil || pending != nil {
		t.Fatal("pending C approval activated a provider", err)
	}
	if err := c.pairing.DecideApproval(t.Context(), approvals[0].RequestID, approvals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	followApprovedThreeCoreSuccessor(t, a, c)
	current, err := a.local.LoadCredential()
	bCredential, bErr := b.local.LoadCredential()
	if err != nil || bErr != nil || current == nil || bCredential == nil || current.SpaceID.String() != c.core || current.CloudBaseUrl != c.endpoint.URL || current.Fingerprint != c.endpoint.Fingerprint || current.DeviceID != a.device.DeviceID || current.RuntimeID != a.device.RuntimeID || current.Credential == bCredential.Credential || current.CredentialID == bCredential.CredentialID {
		t.Fatal("A did not receive a distinct direct C credential", err, bErr)
	}
	aPolicy, err := c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, a.device.DeviceID.String())
	if err != nil || aPolicy.Administrator || aPolicy.Coordinated || aPolicy.SelectedRole != "" {
		t.Fatalf("fresh A inherited B mode or administrator: %+v %v", aPolicy, err)
	}
	bPolicy, err = c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, b.device.DeviceID.String())
	if err != nil || !bPolicy.Administrator || !bPolicy.Coordinated {
		t.Fatal("independent A admission changed B administrator", err)
	}
	var credentials int
	if err := c.services.DeviceMesh.DB.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_runtime_credentials WHERE space_id=? AND device_id=? AND credential_id=? AND status='active'`, c.core, a.device.DeviceID.String(), current.CredentialID).Scan(&credentials); err != nil || credentials != 1 {
		t.Fatal("C does not own A direct active credential", err)
	}
}
