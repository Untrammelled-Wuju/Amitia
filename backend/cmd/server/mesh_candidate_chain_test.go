package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/agent"
)

func TestRealCoreCandidateChainRequiresEachAdmissionAndPreservesOriginalCanonical(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	for _, fresh := range []bool{true, false} {
		name := "existing"
		if fresh {
			name = "fresh"
		}
		t.Run(name, func(t *testing.T) {
			a, b := newThreeCoreFixture(t, name+"-chain-a", schemas), newThreeCoreFixture(t, name+"-chain-b", schemas)
			c, d := newThreeCoreFixture(t, name+"-chain-c", schemas), newThreeCoreFixture(t, name+"-chain-d", schemas)
			var original *agent.StoredCredential
			if !fresh {
				pairThreeCoreFixtures(t, a, b)
				var err error
				original, err = a.local.LoadCredential()
				if err != nil || original == nil {
					t.Fatal("missing original B canonical", err)
				}
			}
			pairThreeCoreFixtures(t, b, c)
			pairThreeCoreFixtures(t, c, d)
			if fresh {
				admitUnreadyThreeCoreCandidate(t, a, b)
			}
			assertOriginal := func() {
				t.Helper()
				current, err := a.local.LoadCredential()
				if err != nil || fresh && current != nil || !fresh && (current == nil || current.CredentialID != original.CredentialID || current.SpaceID != original.SpaceID) {
					t.Fatal("pending chain changed original canonical", err)
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			if err := a.local.FollowSuccessor(ctx); err == nil {
				t.Fatal("C admission was bypassed")
			}
			assertOriginal()
			approvals, err := c.pairing.PendingApprovals(ctx)
			if err != nil || len(approvals) != 1 || approvals[0].DeviceID != a.device.DeviceID.String() || approvals[0].PublicKey != a.device.PublicKey {
				t.Fatal("C lacks independent A identity admission", err)
			}
			if err := c.pairing.DecideApproval(ctx, approvals[0].RequestID, approvals[0].Revision, true); err != nil {
				t.Fatal(err)
			}
			if err := a.local.FollowSuccessor(ctx); err == nil {
				t.Fatal("unready C or unapproved D became canonical")
			}
			assertOriginal()
			var dApprovalsFound bool
			for !dApprovalsFound {
				_ = a.local.FollowSuccessor(ctx)
				approvals, err = d.pairing.PendingApprovals(ctx)
				if err != nil {
					t.Fatal(err)
				}
				dApprovalsFound = len(approvals) == 1 && approvals[0].DeviceID == a.device.DeviceID.String() && approvals[0].PublicKey == a.device.PublicKey
				if !dApprovalsFound {
					select {
					case <-ctx.Done():
						t.Fatal("approved C candidate could not advance to independent D admission")
					case <-time.After(20 * time.Millisecond):
					}
				}
			}
			assertOriginal()
			if err := d.pairing.DecideApproval(ctx, approvals[0].RequestID, approvals[0].Revision, true); err != nil {
				t.Fatal(err)
			}
			followApprovedThreeCoreSuccessor(t, a, d)
			current, err := a.local.LoadCredential()
			if err != nil || current == nil || current.SpaceID.String() != d.core || current.CloudBaseUrl != d.endpoint.URL || current.DeviceID != a.device.DeviceID || current.RuntimeID != a.device.RuntimeID {
				t.Fatal("chain did not produce direct A on D identity", err)
			}
			for _, intermediate := range []*threeCoreFixture{b, c} {
				credential, err := intermediate.local.LoadCredential()
				if err != nil || credential == nil || credential.CredentialID == current.CredentialID || credential.Credential == current.Credential {
					t.Fatal("chain borrowed intermediate device credential", err)
				}
			}
			policy, err := d.services.DeviceMesh.Coordination.Get(ctx, d.core, a.device.DeviceID.String())
			if err != nil || policy.Administrator {
				t.Fatal("chain inherited administrator authority", err)
			}
		})
	}
}

func admitUnreadyThreeCoreCandidate(t *testing.T, source, provider *threeCoreFixture) {
	t.Helper()
	_, token, err := provider.pairing.CreateOffer(t.Context(), provider.device.DeviceID, time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	claim := func(expected int) []byte {
		payload, err := json.Marshal(map[string]any{"endpoint": provider.endpoint, "offerToken": token, "label": source.core})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/pairing/claim", bytes.NewReader(payload))
		request.RemoteAddr = "127.0.0.1:40000"
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		source.router.ServeHTTP(recorder, request)
		if recorder.Code != expected {
			t.Fatalf("candidate scan status=%d expected=%d", recorder.Code, expected)
		}
		return recorder.Body.Bytes()
	}
	claim(http.StatusAccepted)
	approvals, err := provider.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 {
		t.Fatal("candidate scan did not require admission", err)
	}
	if err := provider.pairing.DecideApproval(t.Context(), approvals[0].RequestID, approvals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	var accepted struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(claim(http.StatusOK), &accepted); err != nil || accepted.Ticket == "" {
		t.Fatal("candidate admission did not issue ticket", err)
	}
	_, err = source.local.BindProvider(t.Context(), agent.BindingRequest{CloudBaseURL: provider.endpoint.URL, Fingerprint: provider.endpoint.Fingerprint, CoreID: provider.core, BootstrapTicket: accepted.Ticket})
	var failure *agent.BindingError
	if !errors.As(err, &failure) || failure.Code != "provider_business_not_ready" {
		t.Fatal("unready provider did not remain candidate", err)
	}
}
