package host_api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type mockSnapshotStore struct {
	snapshots map[string]*permission.PermissionSnapshot
}

func newMockSnapshotStore() *mockSnapshotStore {
	return &mockSnapshotStore{snapshots: make(map[string]*permission.PermissionSnapshot)}
}

func (m *mockSnapshotStore) Get(_ context.Context, snapshotID string) (*permission.PermissionSnapshot, error) {
	snap, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, errors.New("not found")
	}
	return snap, nil
}

func (m *mockSnapshotStore) Put(snap *permission.PermissionSnapshot) {
	m.snapshots[snap.SnapshotID] = snap
}

func makeTestValidator() *permission.PermissionIDValidator {
	registry := permission.NewPermissionDefinitionRegistry()
	RegisterPermissionDefinitions(registry)
	return permission.NewPermissionIDValidator(registry)
}

func makeTestIdentity() runtime_supervisor.RuntimeIdentity {
	return runtime_supervisor.RuntimeIdentity{
		ExtensionID: domain.ExtensionID("ext-1"),
		ModuleID:   domain.ModuleID("mod-1"),
		Generation: 1,
	}
}

func TestSnapshotChecker_FailClosed_EmptyGrantedPerms(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-empty",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{},
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store)
	reqs := []PermissionRequirement{{Name: "clipboard.write", Resource: "clipboard"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-empty", reqs)
	if err == nil {
		t.Fatal("expected fail-closed error when snapshot has no granted permissions but reqs are non-empty")
	}
	if !strings.Contains(err.Error(), "no granted permissions") {
		t.Fatalf("expected 'no granted permissions' in error, got: %v", err)
	}
}

func TestSnapshotChecker_FailClosed_NilGrantedPerms(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-nil",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: nil,
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store)
	reqs := []PermissionRequirement{{Name: "clipboard.write", Resource: "clipboard"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-nil", reqs)
	if err == nil {
		t.Fatal("expected fail-closed error when snapshot has nil granted permissions")
	}
}

func TestSnapshotChecker_RejectActionIDs(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-action-ids",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"workflow.run", "action.copy"},
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store).WithValidator(makeTestValidator())
	reqs := []PermissionRequirement{{Name: "workflow.execute", Resource: "workflow"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-action-ids", reqs)
	if err == nil {
		t.Fatal("expected error for snapshot with action IDs")
	}
	if !strings.Contains(err.Error(), "action IDs") {
		t.Fatalf("expected 'action IDs' in error message, got: %v", err)
	}
}

func TestSnapshotChecker_PassesValidPermissions(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-valid",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write", "files.read"},
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store).WithValidator(makeTestValidator())
	reqs := []PermissionRequirement{{Name: "clipboard.write", Resource: "clipboard"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-valid", reqs)
	if err != nil {
		t.Fatalf("expected check to pass for valid snapshot, got: %v", err)
	}
}

func TestSnapshotChecker_DeniesPermissionNotInSnapshot(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-missing-perm",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write"},
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store).WithValidator(makeTestValidator())
	reqs := []PermissionRequirement{{Name: "files.write", Resource: "files"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-missing-perm", reqs)
	if err == nil {
		t.Fatal("expected error for permission not in snapshot")
	}
	if !strings.Contains(err.Error(), "not in snapshot") {
		t.Fatalf("expected 'not in snapshot' in error, got: %v", err)
	}
}

func TestSnapshotChecker_NoReqs_NoError(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-no-reqs",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{},
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store).WithValidator(makeTestValidator())

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-no-reqs", nil)
	if err != nil {
		t.Fatalf("expected no error when no reqs, got: %v", err)
	}
}

func TestSnapshotChecker_RejectExpiredSnapshot(t *testing.T) {
	store := newMockSnapshotStore()
	past := time.Now().UTC().Add(-time.Hour)
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-expired",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write"},
		CreatedAt:    time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:    &past,
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store)
	reqs := []PermissionRequirement{{Name: "clipboard.write"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-expired", reqs)
	if err == nil {
		t.Fatal("expected error for expired snapshot")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected 'expired' in error, got: %v", err)
	}
}

func TestSnapshotChecker_RejectRevokedSnapshot(t *testing.T) {
	store := newMockSnapshotStore()
	now := time.Now().UTC()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-revoked",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write"},
		CreatedAt:    now,
		RevokedAt:    &now,
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store)
	reqs := []PermissionRequirement{{Name: "clipboard.write"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-revoked", reqs)
	if err == nil {
		t.Fatal("expected error for revoked snapshot")
	}
	if !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("expected 'revoked' in error, got: %v", err)
	}
}

func TestSnapshotChecker_RejectIdentityMismatch(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-identity",
		SessionID:    "sess-1",
		ExtensionID:  "ext-other",
		ModuleID:     "mod-other",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write"},
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store)
	reqs := []PermissionRequirement{{Name: "clipboard.write"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-identity", reqs)
	if err == nil {
		t.Fatal("expected error for identity mismatch")
	}
}

func TestSnapshotChecker_EmptySnapshotID_NoError(t *testing.T) {
	store := newMockSnapshotStore()
	checker := NewBrokerPermissionSnapshotChecker(store)

	err := checker.Check(context.Background(), makeTestIdentity(), "", nil)
	if err != nil {
		t.Fatalf("expected no error for empty snapshot ID, got: %v", err)
	}
}

func TestSnapshotChecker_WithoutValidator_PassesActionIDs(t *testing.T) {
	store := newMockSnapshotStore()
	snap := &permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-no-validator",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"workflow.run"},
		CreatedAt:    time.Now().UTC(),
	}
	store.Put(snap)

	checker := NewBrokerPermissionSnapshotChecker(store)
	reqs := []PermissionRequirement{{Name: "workflow.run"}}

	err := checker.Check(context.Background(), makeTestIdentity(), "psnap-test-no-validator", reqs)
	if err != nil {
		t.Fatalf("expected check to pass without validator (action ID passes through), got: %v", err)
	}
}
