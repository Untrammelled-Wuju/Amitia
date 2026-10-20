package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func phase37Actor(device, session string) *auth.ActorContext {
	return &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       runtimeidentity.SpaceID("tenant-A"),
		DeviceID:      runtimeidentity.DeviceID(device),
		SessionID:     session,
		Permissions:   []string{auth.PermSystemAdmin},
	}
}

func TestPhase37DeviceIdentityAcrossSessions(t *testing.T) {
	first, ok := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	if !ok || !recoveryPrincipalIsVerified(first) {
		t.Fatal("missing versioned stable device principal")
	}
	relogged, ok := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-2"))
	if !ok || first != relogged {
		t.Fatal("session rotation bypasses stable principal")
	}
	other, ok := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-1"))
	if !ok || other == first {
		t.Fatal("different devices have identical principal")
	}
	local := phase37Actor("device-A", "desktop-session")
	local.PrincipalType = auth.PrincipalLocalUI
	local.IsLocalTrusted = true
	localIdentity, ok := recoveryDevicePrincipalHash(local)
	if !ok || localIdentity != first {
		t.Fatal("same device bypasses identity via authentication method")
	}
	for _, actor := range []*auth.ActorContext{
		nil,
		{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "tenant-A", Permissions: []string{auth.PermSystemAdmin}},
		{PrincipalType: auth.PrincipalTrustedDevice, DeviceID: "device-A", Permissions: []string{auth.PermSystemAdmin}},
		{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "tenant-A", DeviceID: "device-A"},
		{PrincipalType: auth.PrincipalLocalUI, SpaceID: "tenant-A", DeviceID: "device-A", Permissions: []string{auth.PermSystemAdmin}},
		{PrincipalType: auth.PrincipalSystemWorker, SpaceID: "tenant-A", DeviceID: "device-A", Permissions: []string{auth.PermSystemAdmin}},
	} {
		if hash, ok := recoveryDevicePrincipalHash(actor); ok || hash != "" {
			t.Fatalf("unverified actor accepted: %#v", actor)
		}
	}
}

func TestPhase37LegacyIdentityFailsClosedButCanRevoke(t *testing.T) {
	now := time.Date(2026, 10, 16, 21, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "old-session"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "current-session"))
	request, err := repo.createRecoveryReview(context.Background(), "tenant-A", auditID, recheckID, "phase37-idempotency-1", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.transitionRecoveryReview(context.Background(), "tenant-A", request.Request.ID, requester, "approve", now.Add(2*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("same device approved itself: %v", err)
	}
	if err = repo.db.Exec("UPDATE knowledge_vector_recovery_reviews SET requested_by_hash = ? WHERE id = ?", strings.Repeat("a", 64), request.Request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = repo.transitionRecoveryReview(context.Background(), "tenant-A", request.Request.ID, reviewer, "approve", now.Add(3*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("legacy identity approved: %v", err)
	}
	plan, err := repo.recoveryReviewPlan(context.Background(), "tenant-A", request.Request.ID, now.Add(4*time.Second))
	if err != nil || plan.EvidenceStatus != "identity_unverified" || plan.ManualReviewReady || plan.Executable {
		t.Fatalf("legacy identity eligible: %+v %v", plan, err)
	}
	revoked, err := repo.transitionRecoveryReview(context.Background(), "tenant-A", request.Request.ID, reviewer, "revoke", now.Add(5*time.Second))
	if err != nil || revoked.Request.Status != "revoked" || revoked.Executable {
		t.Fatalf("legacy revoke: %+v %v", revoked, err)
	}
	var events int64
	if err := repo.db.Table("knowledge_vector_recovery_review_events").Where("tenant_id = ? AND review_id = ?", "tenant-A", request.Request.ID).Count(&events).Error; err != nil || events != 2 {
		t.Fatalf("unexpected approval or missing revoke audit: %d %v", events, err)
	}
}
