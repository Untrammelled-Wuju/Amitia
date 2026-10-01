package artifact

import (
	"strings"
	"testing"
	"time"
)

func TestMediaTicketSignerIssueAndValidate(t *testing.T) {
	signer := newMediaTicketSigner()
	signer.now = func() time.Time { return time.Unix(1000, 0).UTC() }
	ticket, expiresAt, err := signer.Issue("art_test", "user:test")
	if err != nil {
		t.Fatal(err)
	}
	if !expiresAt.After(signer.now()) {
		t.Fatalf("expiration must be in the future: %s", expiresAt)
	}
	owner, err := signer.Validate(ticket, "art_test")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "user:test" {
		t.Fatalf("unexpected owner %q", owner)
	}
	if _, err := signer.Validate(ticket+"x", "art_test"); err == nil {
		t.Fatal("tampered ticket must be rejected")
	}
	if _, err := signer.Validate(ticket, "art_other"); err == nil {
		t.Fatal("ticket for another artifact must be rejected")
	}
}

func TestMediaTicketSignerRejectsExpiredTicket(t *testing.T) {
	signer := newMediaTicketSigner()
	current := time.Unix(1000, 0).UTC()
	signer.now = func() time.Time { return current }
	ticket, _, err := signer.Issue("art_test", "user:test")
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(2 * time.Hour)
	if _, err := signer.Validate(ticket, "art_test"); err == nil || !strings.Contains(err.Error(), "invalid media ticket") {
		t.Fatalf("expired ticket must be rejected, got %v", err)
	}
}

func TestContentDispositionSupportsUnicodeFilename(t *testing.T) {
	value := contentDisposition("attachment", "资料 2026.pdf")
	if !strings.HasPrefix(value, "attachment;") {
		t.Fatalf("unexpected disposition %q", value)
	}
	if !strings.Contains(strings.ToLower(value), "filename*=utf-8''") {
		t.Fatalf("unicode filename must use RFC 5987 encoding: %q", value)
	}
}
