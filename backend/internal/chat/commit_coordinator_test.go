package chat

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/psyche"
	"gorm.io/gorm"
)

func setupCommitCoordinatorTest(t *testing.T, withOutbox bool) (*gorm.DB, *service, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "commit.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &relationshipStateRecord{}, &relationshipEventRecord{}, &interaction.InteractionRecordModel{}); err != nil {
		t.Fatal(err)
	}
	store := psyche.NewSQLitePsycheStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if withOutbox {
		if err := interaction.NewSQLiteOutboxStore(db).InitSchema(); err != nil {
			t.Fatal(err)
		}
	}
	convID := "conv-commit"
	if err := db.Create(&Conversation{ID: convID, CharacterID: "char-commit", Channel: "web", Source: "system"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Message{ID: "user-commit", ConversationID: convID, Role: "user", Content: "hello", MsgType: "text", Source: "system", Status: "processing", RequestID: "req-commit"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&interaction.InteractionRecordModel{
		ID:             "interaction-commit",
		UserID:         "user:web",
		CharacterID:    "char-commit",
		ConversationID: convID,
		Channel:        "web",
		PeerID:         "peer-commit",
		RequestID:      "req-commit",
		Status:         string(interaction.InteractionStatusContextReady),
		StatusVersion:  2,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	return db, &service{db: db, psycheStore: store}, convID
}

func runtimeForCommitTest() *interaction.RuntimeAssembly {
	return &interaction.RuntimeAssembly{
		Path: interaction.PathTypeDeep,
		Safety: interaction.RuntimeSafetyDecision{
			Level: "standard",
		},
		Delivery: interaction.RuntimeDeliveryIntent{
			Channel:      "web",
			RequiresText: true,
		},
		Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll},
		Context: interaction.ContextSnapshot{
			Version:     "context-test",
			AssembledAt: time.Now(),
		},
	}
}

func TestCommitInteractionPersistsMessagesStateRelationshipAndOutboxAtomically(t *testing.T) {
	db, svc, convID := setupCommitCoordinatorTest(t, true)
	req := &ProcessMessageRequest{
		CharacterID:           "char-commit",
		ConversationID:        convID,
		Channel:               "web",
		Source:                "system",
		PeerID:                "peer-commit",
		RequestID:             "req-commit",
		InteractionID:         "interaction-commit",
		ExpectedStatusVersion: 2,
		Runtime:               runtimeForCommitTest(),
	}
	result, err := svc.commitInteraction(messageCommitPlan{
		Request:       req,
		Conversation:  convID,
		Character:     "char-commit",
		CharacterName: "Amitia",
		UserMessageID: "user-commit",
		Reply:         "ok",
		Lines:         []string{"ok"},
		Source:        "system",
		Runtime:       req.Runtime,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.MessageIDs) != 1 {
		t.Fatalf("expected one assistant message, got %d", len(result.MessageIDs))
	}
	if len(result.Events) != 3 {
		t.Fatalf("expected three outbox records, got %d", len(result.Events))
	}
	var assistantCount int64
	if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ?", convID, "assistant").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount != 1 {
		t.Fatalf("expected assistant message, got %d", assistantCount)
	}
	var status string
	if err := db.Model(&Message{}).Select("status").Where("id = ?", "user-commit").Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "sent" {
		t.Fatalf("expected user status sent, got %s", status)
	}
	state, err := svc.psycheStore.LoadState("char-commit")
	if err != nil {
		t.Fatal(err)
	}
	if state.StateVersion != 2 || state.Energy >= 0.7 {
		t.Fatalf("unexpected psyche state: version=%d energy=%f", state.StateVersion, state.Energy)
	}
	var relationship relationshipStateRecord
	if err := db.Where("character_id = ? AND relation_type = ?", "char-commit", "peer:peer-commit").Take(&relationship).Error; err != nil {
		t.Fatal(err)
	}
	var relationData map[string]float64
	if err := json.Unmarshal([]byte(relationship.RelationData), &relationData); err != nil {
		t.Fatal(err)
	}
	if relationData["familiarity"] <= 0 {
		t.Fatalf("expected relationship familiarity to increase: %#v", relationData)
	}
	var outboxCount int64
	if err := db.Model(&interaction.OutboxRecordModel{}).Where("aggregate_id = ?", "interaction-commit").Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if outboxCount != 3 {
		t.Fatalf("expected 3 outbox records, got %d", outboxCount)
	}
}

func TestCommitInteractionRollsBackWhenOutboxCommitFails(t *testing.T) {
	db, svc, convID := setupCommitCoordinatorTest(t, false)
	req := &ProcessMessageRequest{
		CharacterID:           "char-commit",
		ConversationID:        convID,
		Channel:               "web",
		Source:                "system",
		PeerID:                "peer-commit",
		RequestID:             "req-commit",
		InteractionID:         "interaction-commit",
		ExpectedStatusVersion: 2,
		Runtime:               runtimeForCommitTest(),
	}
	_, err := svc.commitInteraction(messageCommitPlan{
		Request:       req,
		Conversation:  convID,
		Character:     "char-commit",
		CharacterName: "Amitia",
		UserMessageID: "user-commit",
		Reply:         "ok",
		Lines:         []string{"ok"},
		Source:        "system",
		Runtime:       req.Runtime,
	})
	if err == nil {
		t.Fatal("expected outbox commit failure")
	}
	var assistantCount int64
	if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ?", convID, "assistant").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount != 0 {
		t.Fatalf("assistant message was not rolled back: %d", assistantCount)
	}
	var status string
	if err := db.Model(&Message{}).Select("status").Where("id = ?", "user-commit").Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "processing" {
		t.Fatalf("user status was not rolled back, got %s", status)
	}
	var psycheCount int64
	if err := db.Table("psyche_states").Where("character_id = ?", "char-commit").Count(&psycheCount).Error; err != nil {
		t.Fatal(err)
	}
	if psycheCount != 0 {
		t.Fatalf("psyche state was not rolled back: %d", psycheCount)
	}
	var relationshipCount int64
	if err := db.Model(&relationshipStateRecord{}).Where("character_id = ?", "char-commit").Count(&relationshipCount).Error; err != nil {
		t.Fatal(err)
	}
	if relationshipCount != 0 {
		t.Fatalf("relationship state was not rolled back: %d", relationshipCount)
	}
}

func TestCommitInteractionRejectsStaleInteractionRecord(t *testing.T) {
	cases := []struct {
		name   string
		update map[string]interface{}
	}{
		{
			name: "version_conflict",
			update: map[string]interface{}{
				"status_version": int64(3),
			},
		},
		{
			name: "cancel_requested",
			update: map[string]interface{}{
				"cancel_requested_at": time.Now(),
				"status_version":      int64(3),
			},
		},
		{
			name: "superseded",
			update: map[string]interface{}{
				"status":           string(interaction.InteractionStatusSuperseded),
				"superseded_by_id": "interaction-new",
				"status_version":   int64(3),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, svc, convID := setupCommitCoordinatorTest(t, true)
			if err := db.Model(&interaction.InteractionRecordModel{}).Where("id = ?", "interaction-commit").Updates(tc.update).Error; err != nil {
				t.Fatal(err)
			}
			req := &ProcessMessageRequest{
				CharacterID:           "char-commit",
				ConversationID:        convID,
				Channel:               "web",
				Source:                "system",
				PeerID:                "peer-commit",
				RequestID:             "req-commit",
				InteractionID:         "interaction-commit",
				ExpectedStatusVersion: 2,
				Runtime:               runtimeForCommitTest(),
			}
			_, err := svc.commitInteraction(messageCommitPlan{
				Request:       req,
				Conversation:  convID,
				Character:     "char-commit",
				CharacterName: "Amitia",
				UserMessageID: "user-commit",
				Reply:         "stale",
				Lines:         []string{"stale"},
				Source:        "system",
				Runtime:       req.Runtime,
			})
			if err == nil {
				t.Fatal("expected stale interaction commit to fail")
			}
			assertNoCommitSideEffects(t, db, convID)
		})
	}
}

func assertNoCommitSideEffects(t *testing.T, db *gorm.DB, convID string) {
	t.Helper()
	var assistantCount int64
	if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ?", convID, "assistant").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount != 0 {
		t.Fatalf("assistant message was committed: %d", assistantCount)
	}
	var status string
	if err := db.Model(&Message{}).Select("status").Where("id = ?", "user-commit").Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "processing" {
		t.Fatalf("user status changed, got %s", status)
	}
	var psycheCount int64
	if err := db.Table("psyche_states").Where("character_id = ?", "char-commit").Count(&psycheCount).Error; err != nil {
		t.Fatal(err)
	}
	if psycheCount != 0 {
		t.Fatalf("psyche state was committed: %d", psycheCount)
	}
	var relationshipCount int64
	if err := db.Model(&relationshipStateRecord{}).Where("character_id = ?", "char-commit").Count(&relationshipCount).Error; err != nil {
		t.Fatal(err)
	}
	if relationshipCount != 0 {
		t.Fatalf("relationship state was committed: %d", relationshipCount)
	}
	var outboxCount int64
	if err := db.Model(&interaction.OutboxRecordModel{}).Where("aggregate_id = ?", "interaction-commit").Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if outboxCount != 0 {
		t.Fatalf("outbox records were committed: %d", outboxCount)
	}
}
