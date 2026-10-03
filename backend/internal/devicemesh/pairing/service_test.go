package pairing_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type pairingHarness struct {
	*pairing.Service
	devices      *host_registry.Registry
	bootstrapSvc *bootstrap.Service
}

func pairingTestService(t *testing.T) (*pairingHarness, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "pairing.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := sqlite.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	if _, err := registry.EnsureDevice(ctx, host_registry.DeviceRecord{
		SpaceID: "space-a", DeviceID: "issuer", Platform: runtimeidentity.PlatformWindows,
		TrustState: host_registry.DeviceTrustTrusted,
	}); err != nil {
		t.Fatal(err)
	}
	bootstrapSvc := bootstrap.NewService(bootstrap.NewRepository(db), 300)
	svc, err := pairing.NewService(db, t.TempDir(), "space-a", registry, bootstrapSvc)
	if err != nil {
		t.Fatal(err)
	}
	return &pairingHarness{Service: svc, devices: registry, bootstrapSvc: bootstrapSvc}, db
}

func pairingTestOffer(t *testing.T, svc *pairingHarness) (*pairing.Offer, string) {
	t.Helper()
	offer, token, err := svc.CreateOffer(context.Background(), "issuer", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return offer, token
}

func pairingTestRequest(token string, deviceID runtimeidentity.DeviceID) pairing.ClaimRequest {
	return pairing.ClaimRequest{OfferToken: token, DeviceID: deviceID, RuntimeID: "runtime-a", Platform: runtimeidentity.PlatformAndroid}
}

func pairingAssertOfferActive(t *testing.T, db *sql.DB, offerID string) {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM kernel_device_pairing_offers WHERE offer_id=?`, offerID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("failed claim consumed offer: %s", status)
	}
}

func TestClaimRejectsSelfAndTrustedDuplicateWithoutConsumingOffer(t *testing.T) {
	svc, db := pairingTestService(t)
	offer, token := pairingTestOffer(t, svc)
	if _, err := svc.Claim(context.Background(), pairingTestRequest(token, "issuer")); !errors.Is(err, pairing.ErrSelfPairing) {
		t.Fatalf("expected self rejection, got %v", err)
	}
	pairingAssertOfferActive(t, db, offer.OfferID)
	if _, err := svc.devices.EnsureDevice(context.Background(), host_registry.DeviceRecord{
		SpaceID: "space-a", DeviceID: "existing", Platform: runtimeidentity.PlatformAndroid, TrustState: host_registry.DeviceTrustTrusted,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(context.Background(), pairingTestRequest(token, "existing")); !errors.Is(err, pairing.ErrAlreadyPaired) {
		t.Fatalf("expected duplicate rejection, got %v", err)
	}
	pairingAssertOfferActive(t, db, offer.OfferID)
}

func TestClaimRollsBackDeviceAndOfferWhenTicketCreationFails(t *testing.T) {
	svc, db := pairingTestService(t)
	offer, token := pairingTestOffer(t, svc)
	if _, err := db.Exec(`CREATE TRIGGER reject_test_ticket BEFORE INSERT ON kernel_device_mesh_bootstrap_tickets BEGIN SELECT RAISE(ABORT, 'ticket write failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(context.Background(), pairingTestRequest(token, "new-device")); err == nil {
		t.Fatal("claim unexpectedly succeeded")
	}
	pairingAssertOfferActive(t, db, offer.OfferID)
	device, err := svc.devices.GetDevice(context.Background(), "new-device")
	if err != nil || device != nil {
		t.Fatalf("failed claim left device behind: %v %v", device, err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_test_ticket`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(context.Background(), pairingTestRequest(token, "new-device")); err != nil {
		t.Fatalf("offer cannot be retried: %v", err)
	}
}

func TestConcurrentClaimsIssueOneTicketForDevice(t *testing.T) {
	svc, db := pairingTestService(t)
	_, first := pairingTestOffer(t, svc)
	_, second := pairingTestOffer(t, svc)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, token := range []string{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.Claim(context.Background(), pairingTestRequest(token, "new-device"))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes, rejected := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, pairing.ErrPairingPending) {
			rejected++
		} else {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("unexpected claims: success=%d rejected=%d", successes, rejected)
	}
	var tickets, consumed int
	if err := db.QueryRow(`SELECT COUNT(1) FROM kernel_device_mesh_bootstrap_tickets WHERE device_id='new-device'`).Scan(&tickets); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM kernel_device_pairing_offers WHERE status='consumed'`).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if tickets != 1 || consumed != 1 {
		t.Fatalf("duplicate state: tickets=%d offers=%d", tickets, consumed)
	}
}

func TestRevokedDeviceCanPairAgainButOldTicketIsInvalid(t *testing.T) {
	svc, db := pairingTestService(t)
	_, token := pairingTestOffer(t, svc)
	first, err := svc.Claim(context.Background(), pairingTestRequest(token, "new-device"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := svc.devices.RevokeDeviceTx(context.Background(), tx, "new-device"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeDevicePairingTx(context.Background(), tx, "new-device"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	_, token = pairingTestOffer(t, svc)
	second, err := svc.Claim(context.Background(), pairingTestRequest(token, "new-device"))
	if err != nil {
		t.Fatal(err)
	}
	if second.Ticket.TicketID == first.Ticket.TicketID {
		t.Fatal("re-pair reused old ticket")
	}
	if _, err := svc.bootstrapSvc.Validate(context.Background(), first.RawTicket); err == nil {
		t.Fatal("revoked ticket remained valid")
	}
	device, err := svc.devices.GetDevice(context.Background(), "new-device")
	if err != nil || device.TrustState != host_registry.DeviceTrustPending {
		t.Fatalf("re-pair did not reset trust: %v %v", device, err)
	}
}

func TestExpiredAndRevokedIssuerOffersAreRejected(t *testing.T) {
	svc, db := pairingTestService(t)
	offer, token := pairingTestOffer(t, svc)
	if _, err := db.Exec(`UPDATE kernel_device_pairing_offers SET expires_at=? WHERE offer_id=?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), offer.OfferID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(context.Background(), pairingTestRequest(token, "new-device")); !errors.Is(err, pairing.ErrOfferExpired) {
		t.Fatalf("expected expiry rejection: %v", err)
	}
	_, token = pairingTestOffer(t, svc)
	if err := svc.devices.RevokeDevice(context.Background(), "issuer"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(context.Background(), pairingTestRequest(token, "new-device")); !errors.Is(err, host_registry.ErrDeviceNotTrusted) {
		t.Fatalf("revoked issuer offer accepted: %v", err)
	}
}
