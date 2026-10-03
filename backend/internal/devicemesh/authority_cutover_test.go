package devicemesh_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func meshAuthorityDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCloudRuntimeRejectsSplitIdentityAuthority(t *testing.T) {
	business, kernel := meshAuthorityDB(t), meshAuthorityDB(t)
	if _, err := devicemesh.NewCloudRuntime(business, host_registry.NewRegistry(kernel)); err == nil {
		t.Fatal("split database authority accepted")
	}
}

func TestLegacyAuthorityCutoverPreservesRevocationAcrossRestart(t *testing.T) {
	business, kernel := meshAuthorityDB(t), meshAuthorityDB(t)
	ctx := context.Background()
	ticket, raw, err := bootstrap.NewService(bootstrap.NewRepository(business), 300).Issue(ctx, "space-a", "device-a", "runtime-a", runtimeidentity.PlatformAndroid)
	if err != nil {
		t.Fatal(err)
	}
	if err := devicemesh.ImportLegacyState(ctx, business, kernel); err != nil {
		t.Fatal(err)
	}
	canonical := bootstrap.NewService(bootstrap.NewRepository(kernel), 300)
	if _, err := canonical.Validate(ctx, raw); err != nil {
		t.Fatalf("legacy ticket lost during cutover: %v", err)
	}
	if _, err := kernel.Exec(`UPDATE kernel_device_mesh_bootstrap_tickets SET status='revoked' WHERE ticket_id=?`, ticket.TicketID); err != nil {
		t.Fatal(err)
	}
	if err := devicemesh.ImportLegacyState(ctx, business, kernel); err != nil {
		t.Fatal(err)
	}
	if _, err := canonical.Validate(ctx, raw); err == nil {
		t.Fatal("restart restored a revoked ticket from legacy data")
	}
}

func TestLegacyAuthorityConflictRollsBackEntireCutover(t *testing.T) {
	business, kernel := meshAuthorityDB(t), meshAuthorityDB(t)
	ctx := context.Background()
	ticket, _, err := bootstrap.NewService(bootstrap.NewRepository(business), 300).Issue(ctx, "space-a", "device-a", "runtime-a", runtimeidentity.PlatformAndroid)
	if err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.NewRepository(kernel).Create(ctx, ticket); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Exec(`UPDATE kernel_device_mesh_bootstrap_tickets SET device_id='different-device' WHERE ticket_id=?`, ticket.TicketID); err != nil {
		t.Fatal(err)
	}
	if err := devicemesh.ImportLegacyState(ctx, business, kernel); err == nil {
		t.Fatal("conflicting identity overwritten")
	}
	var markers int
	if err := kernel.QueryRow(`SELECT COUNT(1) FROM kernel_device_mesh_cutovers`).Scan(&markers); err != nil {
		t.Fatal(err)
	}
	if markers != 0 {
		t.Fatal("failed cutover marked completed")
	}
}

func TestPairingExchangeAndRegistryUseOneAuthority(t *testing.T) {
	db := meshAuthorityDB(t)
	ctx := context.Background()
	registry := host_registry.NewRegistry(db)
	if _, err := registry.EnsureDevice(ctx, host_registry.DeviceRecord{
		SpaceID: "space-a", DeviceID: "issuer", Platform: runtimeidentity.PlatformWindows, TrustState: host_registry.DeviceTrustTrusted,
	}); err != nil {
		t.Fatal(err)
	}
	runtime, err := devicemesh.NewCloudRuntime(db, registry)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := pairing.NewService(db, t.TempDir(), "space-a", registry, runtime.BootstrapSvc)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := svc.CreateOffer(ctx, "issuer", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := svc.Claim(ctx, pairing.ClaimRequest{OfferToken: token, DeviceID: "device-a", RuntimeID: "runtime-a", Platform: runtimeidentity.PlatformAndroid})
	if err != nil {
		t.Fatal(err)
	}
	_, rawCredential, _, err := runtime.BootstrapSvc.Exchange(ctx, claim.RawTicket, "device-a", "runtime-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.CredentialSvc.Validate(ctx, rawCredential); err != nil {
		t.Fatal(err)
	}
	if err := registry.RequireTrustedDevice(ctx, "space-a", "device-a"); err != nil {
		t.Fatalf("credential exchange did not update canonical trust: %v", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := registry.RevokeDeviceTx(ctx, tx, "device-a"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.CredentialSvc.RevokeAllForDeviceTx(ctx, tx, "space-a", "device-a"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeDevicePairingTx(ctx, tx, "device-a"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := credential.NewService(credential.NewRepository(db), 300).Validate(ctx, rawCredential); err == nil {
		t.Fatal("revoked credential still valid")
	}
}
