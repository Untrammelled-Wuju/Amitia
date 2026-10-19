package chat

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/runtimegate"
	"github.com/u-ai/backend/internal/interaction"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/psyche"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
	"time"
)

func setupCommitCoordinatorTest(t *testing.T, withOutbox bool) (*gorm.DB, *service, string) {
	t.Helper()
	runtimegate.Set(runtimegate.EmotionExtensionID, true)
	t.Cleanup(func() { runtimegate.Set(runtimegate.EmotionExtensionID, false) })
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
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &RelationshipStateRecord{}, &relationshipEventRecord{}, &interaction.InteractionRecordModel{}, &NeedStateRecord{}); err != nil {
		t.Fatal(err)
	}
	store := psyche.NewSQLitePsycheStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if withOutbox {
		if err := db.AutoMigrate(&newoutbox.OutboxRecordModel{}, &newoutbox.DeadLetterRecordModel{}); err != nil {
			t.Fatal(err)
		}
		if err := delivery.NewSQLiteDeliveryStore(db).InitSchema(); err != nil {
			t.Fatal(err)
		}
	}
	convID := "conv-commit"
	if err := db.Create(&Conversation{ID: convID, SpaceID: normalizeConversationOwner(""), Channel: "web", Source: "system"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Message{ID: "user-commit", ConversationID: convID, Role: "user", Content: "hello", MsgType: "text", Source: "system", Status: "processing", RequestID: "req-commit"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&interaction.InteractionRecordModel{
		ID:             "interaction-commit",
		SpaceID:        "user:web",
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

func ownedStateTestPlan() OwnedStatePlan {
	scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "core", ResourceOwnerID: "device", RoleOwnerID: "device", RoleID: "char-commit", RequestID: "req-commit", TurnID: "turn-commit"}
	return OwnedStatePlan{Scope: scope, SpaceID: "user:web", Character: scope.RoleID, Conversation: "conv-commit", InteractionID: scope.TurnID, Channel: "web", Source: "manual", Reply: "ok", MessageID: "req-commit/assistant", Runtime: &interaction.RuntimeAssembly{Appraisal: &interaction.AppraisalResult{EventType: "praise", RelationshipDelta: 0.2, NeedDeltas: map[string]float64{"connection": 0.1}}, Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll}}}
}

func TestOwnedStateCommitsPsycheRelationshipNeedsAndOutboxAtomically(t *testing.T) {
	db, svc, _ := setupCommitCoordinatorTest(t, true)
	plan := ownedStateTestPlan()
	ctx := coordination.WithScope(t.Context(), plan.Scope)
	for i := 0; i < 2; i++ {
		if err := svc.CommitOwnedState(ctx, plan); err != nil {
			t.Fatal(err)
		}
	}
	state, err := svc.psycheStore.LoadState(plan.Character)
	if err != nil {
		t.Fatal(err)
	}
	if state.Energy >= 1 || state.StateVersion < 1 {
		t.Fatalf("missing interaction psyche update: %+v", state)
	}
	var psycheEvents int64
	db.Table("psyche_events").Count(&psycheEvents)
	if psycheEvents != 1 {
		t.Fatalf("replayed psyche: %d", psycheEvents)
	}
	var relationship RelationshipStateRecord
	if err := db.Where("character_id=? AND space_id=? AND channel=?", plan.Character, plan.SpaceID, "*").Take(&relationship).Error; err != nil {
		t.Fatal(err)
	}
	var values map[string]float64
	if err := json.Unmarshal([]byte(relationship.RelationData), &values); err != nil {
		t.Fatal(err)
	}
	if values["familiarity"] <= 0 {
		t.Fatal(values)
	}
	var needs []NeedStateRecord
	if err := db.Find(&needs).Error; err != nil {
		t.Fatal(err)
	}
	if len(needs) != 7 {
		t.Fatalf("needs=%d", len(needs))
	}
	for _, need := range needs {
		if need.NeedKey == "connection" && need.CurrentValue != 0.6 {
			t.Fatalf("first appraisal lost: %+v", need)
		}
	}
	var events int64
	db.Model(&newoutbox.OutboxRecordModel{}).Count(&events)
	if events != 3 {
		t.Fatalf("outbox=%d", events)
	}
	plan.Reply = "changed"
	if !errors.Is(svc.CommitOwnedState(ctx, plan), coordination.ErrRequestConflict) {
		t.Fatal("conflicting receipt accepted")
	}
}

func TestOwnedCommitSuppressesReplyPersistence(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	f.model.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		return "handled", "", nil, 1, nil
	}
	request := f.request("suppressed")
	ctx := business.WithIngress(t.Context(), business.Ingress{Source: "plugin", Channel: "web", SuppressReplyPersistence: true})
	response, err := f.engine.Run(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !response.Saved || response.MemoryStatus != "skipped" {
		t.Fatalf("%+v", response)
	}
	if messages := f.messages(t); len(messages) != 0 {
		t.Fatalf("suppressed turn persisted messages: %+v", messages)
	}
}

func TestOwnedStateRollsBackWhenOutboxCommitFails(t *testing.T) {
	db, svc, _ := setupCommitCoordinatorTest(t, false)
	plan := ownedStateTestPlan()
	if err := svc.CommitOwnedState(coordination.WithScope(t.Context(), plan.Scope), plan); err == nil {
		t.Fatal("missing outbox accepted")
	}
	for _, table := range []string{"psyche_states", "psyche_events", "relationship_states", "relationship_events", "need_states"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s leaked %d records", table, count)
		}
	}
}

func TestOwnedStateRejectsStaleAuthority(t *testing.T) {
	for _, test := range []string{"revoked", "role_changed", "cancelled"} {
		t.Run(test, func(t *testing.T) {
			db, svc, _ := setupCommitCoordinatorTest(t, true)
			plan := ownedStateTestPlan()
			ctx := coordination.WithAdditionalGuard(coordination.WithScope(t.Context(), plan.Scope), func(context.Context) error { return coordination.ErrScopeExpired })
			if test == "role_changed" {
				plan.Character = "other"
			}
			if test == "cancelled" {
				current, cancel := context.WithCancel(ctx)
				cancel()
				ctx = current
			}
			if err := svc.CommitOwnedState(ctx, plan); err == nil {
				t.Fatal("stale authority accepted")
			}
			var count int64
			db.Table("relationship_events").Count(&count)
			if count != 0 {
				t.Fatal("stale effects persisted")
			}
		})
	}
}
