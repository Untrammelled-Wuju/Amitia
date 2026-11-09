package agent

import (
	"testing"
	"time"
)

func TestActiveCredentialForCenter(t *testing.T) {
	now := time.Now()
	active := &StoredCredential{
		Credential: "secret",
		SpaceID:    "core-a",
		ExpiresAt:  now.Add(time.Hour),
	}
	if !activeCredentialForCenter(active, "core-a", now) {
		t.Fatal("active credential was not recognized")
	}
	if activeCredentialForCenter(active, "core-b", now) {
		t.Fatal("credential for another center was accepted")
	}
	expired := *active
	expired.ExpiresAt = now.Add(-time.Second)
	if activeCredentialForCenter(&expired, "core-a", now) {
		t.Fatal("expired credential was accepted")
	}
	emptySecret := *active
	emptySecret.Credential = ""
	if activeCredentialForCenter(&emptySecret, "core-a", now) {
		t.Fatal("empty credential was accepted")
	}
}
