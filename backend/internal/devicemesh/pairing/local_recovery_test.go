package pairing_test

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
)

func TestLocalOwnerRecoveryRestoresOnlyCoreAndAudits(t *testing.T) {
	svc, db := pairingTestService(t)
	if err := svc.devices.RevokeDevice(t.Context(), "issuer"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.devices.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "space-a", DeviceID: "remote", TrustState: host_registry.DeviceTrustRevoked}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateOffer(t.Context(), "issuer", time.Minute); err == nil {
		t.Fatal("revoked issuer created offer")
	}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true, AuthMethod: "desktop_session", SpaceID: "space-a", DeviceID: "issuer", Permissions: auth.OwnerDevicePermissions()}
	ctx := auth.WithActor(context.Background(), actor)
	if err := svc.RecoverLocalDevice(ctx, "issuer"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateOffer(ctx, "issuer", time.Minute); err != nil {
		t.Fatal(err)
	}
	remote, err := svc.devices.GetDevice(ctx, "remote")
	if err != nil || remote.TrustState != host_registry.DeviceTrustRevoked {
		t.Fatal("remote revocation changed", err)
	}
	if err := svc.RecoverLocalDevice(ctx, "issuer"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_security_audit_outbox WHERE json_extract(payload_json,'$.eventType')='device_mesh.local_core_trust_recovered'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		rows, _ := db.Query(`SELECT payload_json FROM kernel_security_audit_outbox`)
		defer rows.Close()
		for rows.Next() {
			var payload string
			rows.Scan(&payload)
			t.Log(payload)
		}
		t.Fatalf("recovery audit count %d", count)
	}
}

func TestLocalRecoveryRejectsOtherAuthorities(t *testing.T) {
	svc, _ := pairingTestService(t)
	if err := svc.devices.RevokeDevice(t.Context(), "issuer"); err != nil {
		t.Fatal(err)
	}
	valid := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true, AuthMethod: "local_token", SpaceID: "space-a", DeviceID: "issuer", Permissions: auth.OwnerDevicePermissions()}
	for _, change := range []func(*auth.ActorContext){
		func(a *auth.ActorContext) { a.PrincipalType = auth.PrincipalTrustedDevice },
		func(a *auth.ActorContext) { a.IsLocalTrusted = false },
		func(a *auth.ActorContext) { a.AuthMethod = "device_credential" },
		func(a *auth.ActorContext) { a.DeviceID = "remote" },
		func(a *auth.ActorContext) { a.SpaceID = "foreign" },
		func(a *auth.ActorContext) { a.Permissions = nil },
	} {
		actor := valid.Clone()
		change(actor)
		if err := svc.RecoverLocalDevice(auth.WithActor(t.Context(), actor), "issuer"); err == nil {
			t.Fatalf("unauthorized recovery: %+v", actor)
		}
	}
	if err := svc.RecoverLocalDevice(t.Context(), "issuer"); err == nil {
		t.Fatal("anonymous recovery accepted")
	}
	if err := svc.devices.RequireTrustedDevice(t.Context(), "space-a", "issuer"); err == nil {
		t.Fatal("rejected request restored trust")
	}
}

func TestLocalRecoveryRollsBackWithoutAudit(t *testing.T) {
	svc, db := pairingTestService(t)
	if err := svc.devices.RevokeDevice(t.Context(), "issuer"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE kernel_security_audit_outbox`); err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true, AuthMethod: "local_token", SpaceID: "space-a", DeviceID: "issuer", Permissions: auth.OwnerDevicePermissions()}
	if err := svc.RecoverLocalDevice(auth.WithActor(t.Context(), actor), "issuer"); err == nil {
		t.Fatal("recovery without audit succeeded")
	}
	if err := svc.devices.RequireTrustedDevice(t.Context(), "space-a", "issuer"); err == nil {
		t.Fatal("failed recovery committed trust")
	}
}
