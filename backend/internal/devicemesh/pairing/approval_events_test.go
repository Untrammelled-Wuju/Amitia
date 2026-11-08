package pairing_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestPairingApprovalPublishesTerminalDecisionWithoutPolling(t *testing.T) {
	service, _ := pairingTestService(t)
	_, token, err := service.CreateOffer(t.Context(), "issuer", time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := service.Claim(t.Context(), signedApprovalRequest(t, token, key))
	if err != nil {
		t.Fatal(err)
	}
	if pending.Pending == nil {
		t.Fatal("claim did not create a pending approval")
	}

	observed, stopObserving := service.WatchApprovalRequests()
	defer stopObserving()
	initial, _, updates, cancel, err := service.WatchApproval(t.Context(), pending.Pending.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if initial.State != "pending" {
		t.Fatalf("initial state = %q, want pending", initial.State)
	}

	decided := make(chan error, 1)
	go func() {
		decided <- service.DecideApproval(t.Context(), pending.Pending.RequestID, pending.Pending.Revision, true)
	}()
	select {
	case <-observed:
	case <-time.After(2 * time.Second):
		t.Fatal("approval observer was not notified")
	}
	select {
	case updated := <-updates:
		if updated.State != "approved" || updated.Revision != 2 {
			t.Fatalf("updated approval = %+v", updated)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("approval waiter was not notified")
	}
	if err := <-decided; err != nil {
		t.Fatal(err)
	}

	current, _, _, cancelCurrent, err := service.WatchApproval(t.Context(), pending.Pending.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelCurrent()
	if current.State != "approved" {
		t.Fatalf("current state = %q, want approved", current.State)
	}
}
