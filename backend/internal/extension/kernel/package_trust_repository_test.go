package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	persistencesqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func newTrustRepositoryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	statements := []string{
		`CREATE TABLE extension_publisher_keys (
			key_id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL UNIQUE, public_key BLOB NOT NULL,
			publisher_id TEXT NOT NULL, trust_source TEXT NOT NULL, trust_level TEXT NOT NULL,
			key_state TEXT NOT NULL, trusted_at TEXT NOT NULL DEFAULT '', revoked_at TEXT NOT NULL DEFAULT '',
			revocation_reason TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE extension_package_security_audit (
			event_id TEXT PRIMARY KEY, event_type TEXT NOT NULL, package_id TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '', publisher_id TEXT NOT NULL DEFAULT '', report_id TEXT NOT NULL DEFAULT '',
			staging_id TEXT NOT NULL DEFAULT '', snapshot_id TEXT NOT NULL DEFAULT '', operation_id TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT '', success INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestPackageTrustJournalPersistsPendingAuditAndMonotonicVersion(t *testing.T) {
	repository := NewPackageTrustRepository(newTrustRepositoryTestDB(t))
	ctx := context.Background()
	first, err := repository.ReservePending(ctx, trust.PolicyMutation{
		Kind: trust.PolicyMutationPublisherTrust, Actor: "user-1", Reason: "manual approval", PublisherID: "publisher-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.ReservePending(ctx, trust.PolicyMutation{
		Kind: trust.PolicyMutationBlocklist, Actor: "admin-1", Reason: "malware", PackageHash: "sha256:bad", Restrictive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || second.Version != 2 {
		t.Fatalf("unexpected versions: %d %d", first.Version, second.Version)
	}
	pending, err := repository.Pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 || pending[0].Actor != "user-1" || pending[0].Reason != "manual approval" || pending[1].PackageHash != "sha256:bad" {
		t.Fatalf("unexpected pending audit: %#v", pending)
	}
	version, err := repository.CurrentPolicyVersion(ctx)
	if err != nil || version != 2 {
		t.Fatalf("unexpected current policy version: %d err=%v", version, err)
	}
}

func TestPackageTrustJournalActivationIsIdempotent(t *testing.T) {
	repository := NewPackageTrustRepository(newTrustRepositoryTestDB(t))
	mutation, err := repository.ReservePending(context.Background(), trust.PolicyMutation{
		Kind: trust.PolicyMutationRevocation, Actor: "admin-1", Reason: "compromised", PublisherID: "publisher-1", Restrictive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkActive(context.Background(), mutation); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkActive(context.Background(), mutation); err != nil {
		t.Fatal(err)
	}
	pending, err := repository.Pending(context.Background())
	if err != nil || len(pending) != 0 {
		t.Fatalf("activation did not consume pending mutation: %#v err=%v", pending, err)
	}
}

func TestPackageTrustJournalActivatesPublisherKeyWithCAS(t *testing.T) {
	repository := NewPackageTrustRepository(newTrustRepositoryTestDB(t))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	before := PackagePublisherKeyRecord{KeyID: "key-1", Fingerprint: "sha256:key-1", PublicKey: make([]byte, 32),
		PublisherID: "publisher-1", TrustSource: "user_decision", TrustLevel: "unknown", KeyState: "active", CreatedAt: now, UpdatedAt: now}
	if err := repository.Put(context.Background(), before); err != nil {
		t.Fatal(err)
	}
	after := before
	after.TrustLevel = "user_trusted"
	after.TrustedAt = now
	after.UpdatedAt = now + "-trusted"
	oldValue, _ := json.Marshal(before)
	newValue, _ := json.Marshal(after)
	mutation, err := repository.ReservePending(context.Background(), trust.PolicyMutation{
		Kind: trust.PolicyMutationPublisherTrust, Actor: "user-1", Reason: "manual approval",
		PublisherID: before.PublisherID, KeyID: before.KeyID, OldValue: oldValue, NewValue: newValue,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkActive(context.Background(), mutation); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.GetByFingerprint(context.Background(), before.Fingerprint)
	if err != nil || stored.TrustLevel != "user_trusted" || stored.UpdatedAt != after.UpdatedAt {
		t.Fatalf("publisher key was not atomically activated: %#v err=%v", stored, err)
	}
}

func TestPackageTrustJournalDoesNotActivateOnCASConflict(t *testing.T) {
	repository := NewPackageTrustRepository(newTrustRepositoryTestDB(t))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	before := PackagePublisherKeyRecord{KeyID: "key-1", Fingerprint: "sha256:key-1", PublicKey: make([]byte, 32),
		PublisherID: "publisher-1", TrustSource: "user_decision", TrustLevel: "unknown", KeyState: "active", CreatedAt: now, UpdatedAt: now}
	if err := repository.Put(context.Background(), before); err != nil {
		t.Fatal(err)
	}
	after := before
	after.TrustLevel = "user_trusted"
	after.UpdatedAt = now + "-trusted"
	stale := before
	stale.UpdatedAt = "stale-version"
	oldValue, _ := json.Marshal(stale)
	newValue, _ := json.Marshal(after)
	mutation, err := repository.ReservePending(context.Background(), trust.PolicyMutation{
		Kind: trust.PolicyMutationPublisherTrust, Actor: "user-1", Reason: "manual approval",
		PublisherID: before.PublisherID, KeyID: before.KeyID, OldValue: oldValue, NewValue: newValue,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkActive(context.Background(), mutation); !errors.Is(err, ErrTrustMutationConflict) {
		t.Fatalf("expected CAS conflict, got %v", err)
	}
	pending, err := repository.Pending(context.Background())
	if err != nil || len(pending) != 1 {
		t.Fatalf("conflicted mutation must remain pending: %#v err=%v", pending, err)
	}
}

func TestPackageTrustRepositoryCompareAndSwapRejectsLostUpdate(t *testing.T) {
	repository := NewPackageTrustRepository(newTrustRepositoryTestDB(t))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := PackagePublisherKeyRecord{KeyID: "key-1", Fingerprint: "sha256:key-1", PublicKey: make([]byte, 32),
		PublisherID: "publisher-1", TrustSource: "user_decision", TrustLevel: "unknown", KeyState: "active", CreatedAt: now, UpdatedAt: now}
	if err := repository.Put(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	first := record
	first.TrustLevel = "user_trusted"
	first.UpdatedAt = now + "-first"
	second := record
	second.TrustLevel = "revoked"
	second.KeyState = "revoked"
	second.UpdatedAt = now + "-second"
	errorsSeen := make(chan error, 2)
	var wait sync.WaitGroup
	for _, update := range []PackagePublisherKeyRecord{first, second} {
		wait.Add(1)
		go func(candidate PackagePublisherKeyRecord) {
			defer wait.Done()
			errorsSeen <- repository.CompareAndSwap(context.Background(), record, candidate)
		}(update)
	}
	wait.Wait()
	close(errorsSeen)
	var successes, conflicts int
	for err := range errorsSeen {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrTrustMutationConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected CAS error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("unexpected CAS outcomes: success=%d conflict=%d", successes, conflicts)
	}
}

func TestPackageTrustJournalReadFailureIsNotNotFound(t *testing.T) {
	db := newTrustRepositoryTestDB(t)
	repository := NewPackageTrustRepository(db)
	if _, err := db.Exec(`DROP TABLE extension_package_security_audit`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pending(context.Background()); err == nil {
		t.Fatal("expected journal read failure")
	}
	if _, err := repository.CurrentPolicyVersion(context.Background()); err == nil {
		t.Fatal("expected policy version read failure")
	}
}

func TestPackageTrustPendingRestrictionMatchesPublisher(t *testing.T) {
	repository := NewPackageTrustRepository(newTrustRepositoryTestDB(t))
	_, err := repository.ReservePending(context.Background(), trust.PolicyMutation{
		Kind: trust.PolicyMutationPublisherTrust, Actor: "admin-1", Reason: "publisher compromised",
		PublisherID: "publisher-1", Restrictive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	reason, err := repository.PendingRestrictionReason(context.Background(), "publisher-1", "", "sha256:any")
	if err != nil || reason != "publisher compromised" {
		t.Fatalf("pending restriction was not enforced: reason=%q err=%v", reason, err)
	}
	reason, err = repository.PendingRestrictionReason(context.Background(), "publisher-2", "", "sha256:any")
	if err != nil || reason != "" {
		t.Fatalf("restriction leaked to unrelated publisher: reason=%q err=%v", reason, err)
	}
}

func TestPackageRepositoryInvalidatesMatchingUnconsumedPreviews(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := persistencesqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repository := NewPackageRepository(db)
	now := time.Now().UTC()
	artifact := PackageArtifact{ArtifactID: "artifact-1", ExtensionID: "extension-1", Version: "1.0.0",
		ArchiveHash: "sha256:archive", ManifestHash: "sha256:manifest", ContentTreeHash: "sha256:tree",
		ArtifactHash: "sha256:artifact", ArchivePath: "artifact.amitiax", SignatureStatus: "valid",
		SignerKeyID: "key-1", PublisherID: "publisher-1", TrustDecision: "user_trusted",
		CreatedAt: now.Format(time.RFC3339Nano), VerifiedAt: now.Format(time.RFC3339Nano)}
	if err := repository.PutArtifact(context.Background(), artifact); err != nil {
		t.Fatal(err)
	}
	preview := PackagePreviewSession{SessionID: "preview-1", UserID: "user-1", ScopeType: "global",
		ArtifactID: artifact.ArtifactID, ExtensionID: artifact.ExtensionID, Version: artifact.Version,
		Status: "ready", ArchiveHash: artifact.ArchiveHash, ManifestHash: artifact.ManifestHash,
		ContentTreeHash: artifact.ContentTreeHash, RiskFlagsJSON: "[]", RequiredConfirmationsJSON: "[]",
		DependencyResultJSON: "[]", PreviewResultJSON: "{}", VerificationReportJSON: "{}",
		PolicyVersion: packagePolicyVersion, VerifiedAt: now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(time.Hour).Format(time.RFC3339Nano), CreatedAt: now.Format(time.RFC3339Nano)}
	if err := repository.PutPreview(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	extensions, err := repository.InvalidateTrustPreviews(context.Background(), "publisher-1", "", "")
	if err != nil || len(extensions) != 1 || extensions[0] != "extension-1" {
		t.Fatalf("unexpected invalidation result: %v err=%v", extensions, err)
	}
	stored, err := repository.GetPreview(context.Background(), preview.SessionID, preview.UserID, preview.ScopeType, preview.ScopeID)
	if err != nil || stored.Status != "invalidated" {
		t.Fatalf("preview was not invalidated: %#v err=%v", stored, err)
	}
}
