package pairing_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestSuccessorRequiresIndependentAdmissionPreservesModeAndClearsAdministrator(t *testing.T) {
	service, db := pairingTestService(t)
	offer, token, err := service.CreateSuccessorOffer(t.Context(), "issuer", "phone-a", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Claim(t.Context(), pairingTestRequest(token, "another-device")); err == nil {
		t.Fatal("successor offer accepted another device")
	}
	pairingAssertOfferActive(t, db, offer.OfferID)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	pending, err := service.Claim(t.Context(), signedApprovalRequest(t, token, key))
	if err != nil || pending.Pending == nil || pending.Ticket != nil {
		t.Fatalf("admission bypassed: %v", err)
	}
	var policies int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_coordination WHERE device_id='phone-a'`).Scan(&policies); err != nil || policies != 0 {
		t.Fatal("mode granted before admission")
	}
	if err := service.DecideApproval(t.Context(), pending.Pending.RequestID, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kernel_device_coordination(space_id,device_id,coordinated,administrator,selected_role) VALUES('space-a','phone-a',0,1,'old-core-role')`); err != nil {
		t.Fatal(err)
	}
	accepted, err := service.Claim(t.Context(), signedApprovalRequest(t, token, key))
	if err != nil || accepted.Ticket == nil {
		t.Fatalf("approved device not admitted: %v", err)
	}
	var coordinated, admin bool
	var role string
	if err := db.QueryRow(`SELECT coordinated,administrator,selected_role FROM kernel_device_coordination WHERE space_id='space-a' AND device_id='phone-a'`).Scan(&coordinated, &admin, &role); err != nil {
		t.Fatal(err)
	}
	if !coordinated || admin || role != "" {
		t.Fatalf("invalid successor policy: coordinated=%v administrator=%v role=%s", coordinated, admin, role)
	}
}
