package temporal

import (
	"testing"
	"time"
)

func TestPrepareInboundSecondScopeSameTurnDegrades(t *testing.T) {
	now := time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC)
	repo, _, clock := newRelationshipTimeTestRepository(t, now)
	device := NewRelationshipTimeCoordinator(repo, clock).ForOwner("device")
	core := NewRelationshipTimeCoordinator(repo, clock).ForOwner("core")

	deviceSpace := RelationshipOwnerSpace("space", "device")
	coreSpace := RelationshipOwnerSpace("space", "core")
	turn := PrepareInboundInput{
		SpaceID:       deviceSpace,
		CharacterID:   "role",
		RequestID:     "request-1",
		InteractionID: "turn-1",
		Message:       "你好",
	}
	first, err := device.PrepareInbound(t.Context(), turn)
	if err != nil {
		t.Fatalf("first scope: %v", err)
	}
	if first.Policy == nil {
		t.Fatal("first scope lost its policy")
	}

	turn.SpaceID = coreSpace
	second, err := core.PrepareInbound(t.Context(), turn)
	if err != nil {
		t.Fatalf("second scope returned error instead of degrading: %v", err)
	}
	if second.Version != RelationshipTimeVersion || second.SpaceID != coreSpace || second.CharacterID != "role" {
		t.Fatalf("second scope context: %+v", second)
	}
	if second.Policy != nil {
		t.Fatalf("second scope must not own the turn policy: %+v", second.Policy)
	}

	var count int64
	if err := repo.db.Model(&InteractionReceipt{}).Where("interaction_id = ?", "turn-1").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("turn kept %d receipts, want exactly 1", count)
	}
}

func TestPrepareInboundSameTurnDifferentRequestDegrades(t *testing.T) {
	now := time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC)
	repo, _, clock := newRelationshipTimeTestRepository(t, now)
	device := NewRelationshipTimeCoordinator(repo, clock).ForOwner("device")
	core := NewRelationshipTimeCoordinator(repo, clock).ForOwner("core")

	if _, err := device.PrepareInbound(t.Context(), PrepareInboundInput{
		SpaceID:       RelationshipOwnerSpace("space", "device"),
		CharacterID:   "role",
		RequestID:     "request-1",
		InteractionID: "turn-1",
		Message:       "你好",
	}); err != nil {
		t.Fatal(err)
	}
	second, err := core.PrepareInbound(t.Context(), PrepareInboundInput{
		SpaceID:       RelationshipOwnerSpace("space", "core"),
		CharacterID:   "role",
		RequestID:     "request-2",
		InteractionID: "turn-1",
		Message:       "你好",
	})
	if err != nil {
		t.Fatalf("second scope returned error instead of degrading: %v", err)
	}
	if second.Policy != nil {
		t.Fatalf("second scope must not own the turn policy: %+v", second.Policy)
	}
}
