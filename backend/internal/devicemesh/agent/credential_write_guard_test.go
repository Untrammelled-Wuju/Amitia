package agent

import (
	"context"
	"testing"
	"time"
)

func TestCredentialWriteAndUnpairAreSerializedAcrossStoreInstances(t *testing.T) {
	root := t.TempDir()
	writer := NewCredentialStore(root)
	unpair := NewCredentialStore(root)
	credential := &StoredCredential{CredentialID: "id", Credential: "token", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", ExpiresAt: time.Now().Add(time.Hour)}
	if err := writer.SaveCredential(credential); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	written := make(chan error, 1)
	go func() {
		written <- writer.WithActiveCredential(t.Context(), credential, func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	deleted := make(chan error, 1)
	go func() { deleted <- unpair.DeleteCredential() }()
	select {
	case err := <-deleted:
		t.Fatalf("unpair raced active owner commit: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	if err := writer.WithActiveCredential(t.Context(), credential, func(context.Context) error { t.Fatal("late write invoked"); return nil }); err == nil {
		t.Fatal("old Core wrote after unpair")
	}
}

func TestCredentialGuardRejectsTransitionAndReplacement(t *testing.T) {
	store := NewCredentialStore(t.TempDir())
	credential := &StoredCredential{CredentialID: "id", Credential: "token", SpaceID: "core", ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.SaveCredential(credential); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(&StoredCredential{SpaceID: "new-core"}); err != nil {
		t.Fatal(err)
	}
	write := func(context.Context) error { t.Fatal("obsolete write invoked"); return nil }
	if err := store.WithActiveCredential(t.Context(), credential, write); err == nil {
		t.Fatal("old write accepted during transition")
	}
	if err := store.DeleteCandidate(); err != nil {
		t.Fatal(err)
	}
	replacement := *credential
	replacement.SpaceID = "new-core"
	if err := store.SaveCredential(&replacement); err != nil {
		t.Fatal(err)
	}
	if err := store.WithActiveCredential(t.Context(), credential, write); err == nil {
		t.Fatal("old credential accepted after replacement")
	}
}
