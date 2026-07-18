package temporal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func newRelationshipTimeTestRepository(t *testing.T, now time.Time) (*RelationshipTimeRepository, *gorm.DB, *FakeClock) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE relationship_states (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
relation_type TEXT NOT NULL DEFAULT '',
relation_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT '',
user_id TEXT NOT NULL DEFAULT 'default',
channel TEXT NOT NULL DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	runner := migration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]migration.Migration{migration.TemporalRelationshipTimeMigration()}); err != nil {
		t.Fatal(err)
	}
	clock := NewFakeClock(now)
	return NewRelationshipTimeRepository(db, clock), db, clock
}

func TestRelationshipTimeMigrationCreatesSchemaAndIndexes(t *testing.T) {
	_, db, _ := newRelationshipTimeTestRepository(t, time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC))
	tables := []string{
		"temporal_global_presence_states",
		"temporal_relationship_presence_states",
		"temporal_cadence_samples",
		"temporal_reunion_episodes",
		"temporal_interaction_receipts",
		"temporal_effect_ledger",
	}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
	indexes := []string{
		"idx_temporal_relationship_presence_scope",
		"idx_temporal_cadence_interaction",
		"idx_temporal_reunion_idempotency",
		"idx_temporal_receipt_request",
		"idx_temporal_receipt_interaction",
		"idx_temporal_effect_key",
	}
	for _, index := range indexes {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("missing index %s", index)
		}
	}
}

func TestRelationshipTimeMigrationCopiesLatestCanonicalRelationship(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE relationship_states (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
relation_type TEXT NOT NULL DEFAULT '',
relation_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT '',
user_id TEXT NOT NULL DEFAULT 'default',
channel TEXT NOT NULL DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		id        string
		data      string
		updatedAt string
	}{
		{id: "old-web", data: `{"version":"old"}`, updatedAt: "2026-07-10T00:00:00Z"},
		{id: "latest-wechat", data: `{"version":"latest"}`, updatedAt: "2026-07-18T00:00:00Z"},
	}
	for _, row := range rows {
		if err := db.Exec("INSERT INTO relationship_states (id, user_id, character_id, relation_type, relation_data, channel, created_at, updated_at) VALUES (?, 'user-a', 'character-a', 'friend', ?, ?, '2026-07-01T00:00:00Z', ?)", row.id, row.data, row.id, row.updatedAt).Error; err != nil {
			t.Fatal(err)
		}
	}
	runner := migration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]migration.Migration{migration.TemporalRelationshipTimeMigration()}); err != nil {
		t.Fatal(err)
	}
	var canonical struct {
		RelationData string `gorm:"column:relation_data"`
		Channel      string `gorm:"column:channel"`
		RelationType string `gorm:"column:relation_type"`
	}
	if err := db.Table("relationship_states").Where("user_id = 'user-a' AND character_id = 'character-a' AND channel = '*' AND relation_type = 'user_character'").First(&canonical).Error; err != nil {
		t.Fatal(err)
	}
	if canonical.RelationData != `{"version":"latest"}` || canonical.Channel != CanonicalRelationshipChannel || canonical.RelationType != CanonicalRelationshipType {
		t.Fatalf("unexpected canonical relationship %#v", canonical)
	}
	if err := db.Exec("INSERT INTO relationship_states (id, user_id, character_id, relation_type, relation_data, channel) VALUES ('duplicate', 'user-a', 'character-a', 'user_character', '{}', '*')").Error; err == nil {
		t.Fatal("canonical unique index accepted duplicate scope")
	}
	var legacyCount int64
	if err := db.Table("relationship_states").Where("channel != '*' OR relation_type != 'user_character'").Count(&legacyCount).Error; err != nil {
		t.Fatal(err)
	}
	if legacyCount != 2 {
		t.Fatalf("legacy relationship rows were not preserved: %d", legacyCount)
	}
}

func TestSaveObservedPresenceDoesNotCommitInteraction(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 123456789, time.UTC)
	repository, _, _ := newRelationshipTimeTestRepository(t, now)
	if err := repository.WithTransaction(ctx, func(tx *RelationshipTimeRepository) error {
		return tx.SaveObservedPresence(ctx, ObservePresenceInput{UserID: "user-a", CharacterID: "character-a", Channel: "web", ObservedAt: now})
	}); err != nil {
		t.Fatal(err)
	}
	global, err := repository.GetGlobalPresence(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if global.LastObservedUserActivityAtUTC != now.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected observed time %s", global.LastObservedUserActivityAtUTC)
	}
	if global.LastCommittedUserInteractionAtUTC != "" || global.InteractionCount != 0 {
		t.Fatalf("prepare mutated committed presence: %#v", global)
	}
	relationship, err := repository.GetRelationshipPresence(ctx, "user-a", "character-a")
	if err != nil {
		t.Fatal(err)
	}
	if relationship.LastCommittedUserInteractionAtUTC != "" || relationship.InteractionCount != 0 {
		t.Fatalf("prepare mutated relationship commit state: %#v", relationship)
	}
}

func TestReceiptAndReunionEpisodeAreIdempotent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	repository, _, _ := newRelationshipTimeTestRepository(t, now)
	receipt := &InteractionReceipt{ID: "receipt-a", UserID: "user-a", RequestID: "request-a", InteractionID: "interaction-a", Status: InteractionReceiptObserved}
	created, err := repository.CreateReceipt(ctx, receipt)
	if err != nil || !created {
		t.Fatalf("first receipt create failed: %v %v", created, err)
	}
	duplicate := *receipt
	duplicate.ID = "receipt-b"
	duplicate.InteractionID = "interaction-b"
	created, err = repository.CreateReceipt(ctx, &duplicate)
	if err != nil || created {
		t.Fatalf("duplicate receipt was created: %v %v", created, err)
	}
	episode := &ReunionEpisode{UserID: "user-a", CharacterID: "character-a", Status: ReunionStatePending, IdempotencyKey: "same-gap", PolicyJSON: "{}"}
	first, created, err := repository.CreateOrGetReunionEpisode(ctx, episode)
	if err != nil || !created {
		t.Fatalf("first episode create failed: %v %v", created, err)
	}
	second, created, err := repository.CreateOrGetReunionEpisode(ctx, &ReunionEpisode{UserID: "user-a", CharacterID: "character-a", Status: ReunionStatePending, IdempotencyKey: "same-gap", PolicyJSON: "{}"})
	if err != nil || created || first.ID != second.ID {
		t.Fatalf("episode idempotency failed: %#v %#v %v %v", first, second, created, err)
	}
}

func TestReunionClaimExclusionReleaseAndTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	repository, _, clock := newRelationshipTimeTestRepository(t, now)
	episode, _, err := repository.CreateOrGetReunionEpisode(ctx, &ReunionEpisode{UserID: "user-a", CharacterID: "character-a", Status: ReunionStatePending, IdempotencyKey: "gap-a", PolicyJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	claimed, acquired, err := repository.ClaimReunionEpisode(ctx, episode.ID, "interaction-a")
	if err != nil || !acquired || claimed.Status != ReunionStateClaimed {
		t.Fatalf("initial claim failed: %#v %v %v", claimed, acquired, err)
	}
	_, acquired, err = repository.ClaimReunionEpisode(ctx, episode.ID, "interaction-b")
	if err != nil || acquired {
		t.Fatalf("competing claim acquired: %v %v", acquired, err)
	}
	if err := repository.ReleaseClaim(ctx, "interaction-a", "failed"); err != nil {
		t.Fatal(err)
	}
	_, acquired, err = repository.ClaimReunionEpisode(ctx, episode.ID, "interaction-b")
	if err != nil || !acquired {
		t.Fatalf("released claim not reusable: %v %v", acquired, err)
	}
	clock.Advance(ReunionClaimTTL + time.Nanosecond)
	_, acquired, err = repository.ClaimReunionEpisode(ctx, episode.ID, "interaction-c")
	if err != nil || !acquired {
		t.Fatalf("expired claim not reusable: %v %v", acquired, err)
	}
}

func TestFinalizeInteractionIsAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	repository, db, _ := newRelationshipTimeTestRepository(t, now)
	if err := repository.SaveObservedPresence(ctx, ObservePresenceInput{UserID: "user-a", CharacterID: "character-a", Channel: "web", ObservedAt: now}); err != nil {
		t.Fatal(err)
	}
	episode, _, err := repository.CreateOrGetReunionEpisode(ctx, &ReunionEpisode{UserID: "user-a", CharacterID: "character-a", Status: ReunionStatePending, IdempotencyKey: "gap-a", PolicyJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if _, acquired, err := repository.ClaimReunionEpisode(ctx, episode.ID, "interaction-a"); err != nil || !acquired {
		t.Fatalf("claim failed: %v %v", acquired, err)
	}
	relationship, err := repository.GetRelationshipPresence(ctx, "user-a", "character-a")
	if err != nil {
		t.Fatal(err)
	}
	relationship.ActiveReunionEpisodeID = episode.ID
	if err := repository.SaveRelationshipPresence(ctx, relationship); err != nil {
		t.Fatal(err)
	}
	receipt := &InteractionReceipt{ID: "receipt-a", UserID: "user-a", CharacterID: "character-a", RequestID: "request-a", InteractionID: "interaction-a", ReunionEpisodeID: episode.ID, Status: InteractionReceiptObserved}
	if _, err := repository.CreateReceipt(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	input := FinalizeInteractionInput{
		UserID:             "user-a",
		CharacterID:        "character-a",
		InteractionID:      "interaction-a",
		CommittedAt:        now,
		ReunionEpisodeID:   episode.ID,
		ReacclimationTurns: 2,
		CadenceSample: &CadenceSample{
			ID: "sample-a", UserID: "user-a", CharacterID: "character-a", InteractionID: "interaction-a", SampleKind: "relationship", Included: true, CurrentInteractionAtUTC: now.Format(time.RFC3339Nano), CreatedAtUTC: now.Format(time.RFC3339Nano),
		},
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return repository.FinalizeInteractionTx(ctx, tx, input) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return repository.FinalizeInteractionTx(ctx, tx, input) }); err != nil {
		t.Fatal(err)
	}
	global, err := repository.GetGlobalPresence(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if global.InteractionCount != 1 || global.SessionCount != 1 {
		t.Fatalf("finalize was not idempotent: %#v", global)
	}
	storedEpisode, err := repository.GetReunionEpisode(ctx, episode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedEpisode.Status != ReunionStateHandled || storedEpisode.HandledInteractionID != "interaction-a" {
		t.Fatalf("episode not finalized: %#v", storedEpisode)
	}
	entries, err := repository.ListEffectLedger(ctx, "user-a", "character-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("effect ledger count = %d", len(entries))
	}
}

func TestCadenceSamplesKeepLatestSixty(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	repository, _, _ := newRelationshipTimeTestRepository(t, now)
	for index := 0; index < 65; index++ {
		at := now.Add(time.Duration(index) * time.Hour)
		sample := &CadenceSample{
			ID: fmt.Sprintf("sample-%02d", index), UserID: "user-a", CharacterID: "character-a", InteractionID: fmt.Sprintf("interaction-%02d", index), SampleKind: "relationship", Included: true, CurrentInteractionAtUTC: FormatRelationshipTime(at), CreatedAtUTC: FormatRelationshipTime(at),
		}
		if _, err := repository.AddCadenceSample(ctx, sample); err != nil {
			t.Fatal(err)
		}
	}
	samples, err := repository.ListCadenceSamples(ctx, "user-a", "character-a", "relationship", 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != MaximumCadenceSamples || samples[0].InteractionID != "interaction-64" || samples[len(samples)-1].InteractionID != "interaction-05" {
		t.Fatalf("unexpected retained cadence range: %d %s %s", len(samples), samples[0].InteractionID, samples[len(samples)-1].InteractionID)
	}
}
