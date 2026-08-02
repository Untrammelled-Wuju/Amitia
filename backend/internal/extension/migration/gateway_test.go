package migration

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGatewayCreateSnapshot(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	snap, err := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if snap.SnapshotID == "" {
		t.Errorf("expected snapshot id")
	}
	if snap.IntegrityHash == "" {
		t.Errorf("expected integrity hash")
	}
}

func TestGatewayRegisterAndGetEntity(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	snap, _ := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	record := LegacyEntityRecord{
		EntityType: LegacyEntitySkill,
		LegacyID:   "skill1",
		RawData:    json.RawMessage(`{"id":"skill1"}`),
	}
	if err := gw.RegisterEntity(context.Background(), snap.SnapshotID, record); err != nil {
		t.Fatalf("RegisterEntity: %v", err)
	}
	got, err := gw.GetEntity(context.Background(), snap.SnapshotID, LegacyEntitySkill, "skill1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.LegacyID != "skill1" {
		t.Errorf("expected skill1, got %s", got.LegacyID)
	}
}

func TestGatewayListEntities(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	snap, _ := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	for i := 0; i < 5; i++ {
		_ = gw.RegisterEntity(context.Background(), snap.SnapshotID, LegacyEntityRecord{
			EntityType: LegacyEntityPlugin,
			LegacyID:   "plugin" + string(rune('0'+i)),
		})
	}
	page, err := gw.ListEntities(context.Background(), snap.SnapshotID, LegacyEntityQuery{
		EntityType: LegacyEntityPlugin,
		Limit:      3,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(page.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(page.Items))
	}
	if !page.HasMore {
		t.Errorf("expected has more")
	}
	if page.Total != 5 {
		t.Errorf("expected total 5, got %d", page.Total)
	}
}

func TestGatewaySnapshotMissing(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	_, err := gw.GetEntity(context.Background(), "nonexistent", LegacyEntitySkill, "x")
	if err != ErrSnapshotNotFound {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestGatewayArtifactMissing(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	snap, _ := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	_, err := gw.ReadArtifact(context.Background(), snap.SnapshotID, "/path/x")
	if err != ErrArtifactMissing {
		t.Errorf("expected ErrArtifactMissing, got %v", err)
	}
}

func TestValidatorMissingFields(t *testing.T) {
	v := NewDefaultValidator()
	conflicts := v.Validate(context.Background(), MigrationEntity{})
	if len(conflicts) == 0 {
		t.Errorf("expected conflicts for empty entity")
	}
	found := false
	for _, c := range conflicts {
		if c.Type == "missing_canonical_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing_canonical_id conflict")
	}
}

func TestConflictDetectorDuplicates(t *testing.T) {
	d := NewDefaultConflictDetector()
	entities := []MigrationEntity{
		{CanonicalID: "c1", LegacySource: LegacySourceReference{LegacyID: "l1"}},
		{CanonicalID: "c1", LegacySource: LegacySourceReference{LegacyID: "l2"}},
	}
	conflicts := d.Detect(context.Background(), entities)
	if len(conflicts) == 0 {
		t.Errorf("expected duplicate conflict")
	}
}

func TestPlanner(t *testing.T) {
	v := NewDefaultValidator()
	d := NewDefaultConflictDetector()
	p := NewDefaultPlanner(v, d)
	entities := []MigrationEntity{
		{
			CanonicalID:  "c1",
			EntityType:   "extension",
			LegacySource: LegacySourceReference{LegacyID: "l1"},
			Definition:   json.RawMessage(`{}`),
		},
	}
	plan, err := p.Plan(context.Background(), "snap1", entities, nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.PlanID == "" {
		t.Errorf("expected plan id")
	}
	if plan.SnapshotID != "snap1" {
		t.Errorf("expected snap1, got %s", plan.SnapshotID)
	}
}

func TestMigrationServiceRun(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	snap, _ := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	v := NewDefaultValidator()
	d := NewDefaultConflictDetector()
	p := NewDefaultPlanner(v, d)
	reports := NewInMemoryReportStore()
	svc := NewMigrationService(gw, v, d, p, reports)
	entities := []MigrationEntity{
		{
			CanonicalID:  "c1",
			EntityType:   "extension",
			LegacySource: LegacySourceReference{LegacyID: "l1"},
			Definition:   json.RawMessage(`{}`),
		},
		{
			CanonicalID:  "",
			EntityType:   "extension",
			LegacySource: LegacySourceReference{LegacyID: "l2"},
		},
	}
	report, err := svc.Run(context.Background(), snap.SnapshotID, entities)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.TotalEntities != 2 {
		t.Errorf("expected total 2, got %d", report.TotalEntities)
	}
	if report.Converted != 1 {
		t.Errorf("expected converted 1, got %d", report.Converted)
	}
	if report.Failed != 1 {
		t.Errorf("expected failed 1, got %d", report.Failed)
	}
}

func TestSnapshotIntegrityHash(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	snap1, _ := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{Labels: map[string]string{"a": "1"}})
	snap2, _ := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{Labels: map[string]string{"a": "2"}})
	if snap1.IntegrityHash == snap2.IntegrityHash {
		t.Errorf("expected different hashes for different snapshots")
	}
}

func TestListSnapshots(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	_, _ = gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	_, _ = gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	snaps, err := gw.ListSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snaps))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	gw := NewDefaultGateway(NewInMemorySnapshotStore(), NewInMemoryEntityStore(), NewInMemoryArtifactStore())
	snap, _ := gw.CreateSnapshot(context.Background(), LegacySnapshotRequest{})
	if err := gw.DeleteSnapshot(context.Background(), snap.SnapshotID); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}
	snaps, _ := gw.ListSnapshots(context.Background())
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots after delete, got %d", len(snaps))
	}
}
