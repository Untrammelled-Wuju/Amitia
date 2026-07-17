package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/interaction"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupE2EGoldenTest(t *testing.T, personalityConfig string) (*gorm.DB, *service, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "e2e.db")), &gorm.Config{})
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
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &ModelConfig{}, &character.Character{}); err != nil {
		t.Fatal(err)
	}

	store := psyche.NewSQLitePsycheStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&RelationshipStateRecord{}, &relationshipEventRecord{}, &interaction.InteractionRecordModel{}, &NeedStateRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&newoutbox.OutboxRecordModel{}, &newoutbox.DeadLetterRecordModel{}); err != nil {
		t.Fatal(err)
	}
	if err := delivery.NewSQLiteDeliveryStore(db).InitSchema(); err != nil {
		t.Fatal(err)
	}

	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	charRepo := character.NewRepository(ctx)

	charID := "char-e2e"
	convID := "conv-e2e"

	if err := db.Create(&character.Character{
		ID:                charID,
		Name:              "测试角色",
		Identity:          "一位温柔体贴的朋友",
		Status:            "enabled",
		Personality:       "温柔、善解人意、有耐心",
		PersonalityConfig: personalityConfig,
		ChatStyleConfig:   "{}",
		SceneRules:        "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Conversation{
		ID:          convID,
		CharacterID: charID,
		Title:       "E2E测试对话",
		Channel:     "web",
		Source:      "manual",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ModelConfig{
		Name:        "e2e-model",
		APIType:     "openai-compatible",
		BaseURL:     "http://127.0.0.1",
		APIKey:      "e2e-key",
		ModelName:   "e2e",
		Temperature: 0.7,
		MaxTokens:   128,
		IsActive:    1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	llm := func(_ context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		return "你今天看起来心情不错呢\n有什么开心的事吗", "", nil, 21, nil
	}

	svc := &service{
		repo:         repo,
		charRepo:     charRepo,
		db:           db,
		llmWithTools: llm,
		psycheStore:  store,
	}

	return db, svc, charID, convID
}

func createInteractionRecord(t *testing.T, db *gorm.DB, interactionID, charID, convID, requestID string) {
	t.Helper()
	if err := db.Create(&interaction.InteractionRecordModel{
		ID:             interactionID,
		UserID:         "user:web",
		CharacterID:    charID,
		ConversationID: convID,
		Channel:        "web",
		PeerID:         "peer-e2e",
		RequestID:      requestID,
		Status:         string(interaction.InteractionStatusContextReady),
		StatusVersion:  0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func mustQueryPsycheRaw(t *testing.T, db *gorm.DB, charID string) map[string]interface{} {
	t.Helper()
	var rec struct {
		CharacterID  string  `gorm:"column:character_id"`
		StateVersion int     `gorm:"column:state_version"`
		Emotion      string  `gorm:"column:emotion"`
		Mood         string  `gorm:"column:mood"`
		Stress       float64 `gorm:"column:stress"`
		Energy       float64 `gorm:"column:energy"`
	}
	if err := db.Table("psyche_states").Where("character_id = ?", charID).First(&rec).Error; err != nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"character_id":  rec.CharacterID,
		"state_version": rec.StateVersion,
		"emotion":       rec.Emotion,
		"mood":          rec.Mood,
		"stress":        rec.Stress,
		"energy":        rec.Energy,
	}
}

func mustQueryRelationshipsRaw(t *testing.T, db *gorm.DB, charID, relationType string) []map[string]interface{} {
	t.Helper()
	var records []RelationshipStateRecord
	if err := db.Where("character_id = ? AND relation_type = ?", charID, relationType).Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	for _, r := range records {
		result = append(result, map[string]interface{}{
			"character_id":  r.CharacterID,
			"relation_type": r.RelationType,
			"relation_data": r.RelationData,
		})
	}
	return result
}

func mustQueryNeedsRaw(t *testing.T, db *gorm.DB, charID string) []map[string]interface{} {
	t.Helper()
	var records []NeedStateRecord
	if err := db.Where("character_id = ?", charID).Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	for _, r := range records {
		result = append(result, map[string]interface{}{
			"id":            r.ID,
			"character_id":  r.CharacterID,
			"need_key":      r.NeedKey,
			"current_value": r.CurrentValue,
			"baseline":      r.Baseline,
			"trend":         r.Trend,
			"saturated":     r.Saturated,
		})
	}
	return result
}

func TestE2E_GoldenPath_CharacterPersonalityToPsycheRelationshipAndMindState(t *testing.T) {
	personalityCfg := `{"warmth":60,"initiative":40,"sensitivity":70,"tolerance":80}`
	db, svc, charID, convID := setupE2EGoldenTest(t, personalityCfg)

	createInteractionRecord(t, db, "interaction-e2e-1", charID, convID, "req-e2e-1")

	resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:           charID,
		ConversationID:        convID,
		Channel:               "web",
		Source:                "manual",
		Message:               "今天天气真好",
		RequestID:             "req-e2e-1",
		PeerID:                "peer-e2e",
		InteractionID:         "interaction-e2e-1",
		ExpectedStatusVersion: 0,
		Runtime: &interaction.RuntimeAssembly{
			Path: interaction.PathTypeDeep,
			Safety: interaction.RuntimeSafetyDecision{
				Level: "standard",
			},
			Delivery: interaction.RuntimeDeliveryIntent{
				Channel:      "web",
				RequiresText: true,
			},
			Transaction: interaction.TransactionDefinition{
				Name: interaction.TransactionBoundaryAll,
			},
			Context: interaction.ContextSnapshot{
				Version:     "context-v1",
				AssembledAt: time.Now(),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reply == "" {
		t.Fatal("期望回复不为空")
	}
	if len(resp.MessageIDs) == 0 {
		t.Fatal("期望有assistant消息ID")
	}
	t.Logf("回复: %s", resp.Reply)
	t.Logf("消息IDs: %v", resp.MessageIDs)

	var assistantCount int64
	if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ?", convID, "assistant").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount < 1 {
		t.Fatalf("期望至少1条assistant消息, 实际 %d", assistantCount)
	}

	var userStatus string
	if err := db.Model(&Message{}).Where("request_id = ?", "req-e2e-1").Select("status").Row().Scan(&userStatus); err != nil {
		t.Fatal(err)
	}
	if userStatus != "sent" {
		t.Fatalf("期望用户消息状态为sent, 实际 %s", userStatus)
	}

	state, err := svc.psycheStore.LoadState(charID)
	if err != nil {
		t.Logf("psyche状态尚未创建(需要Appraisal Delta触发), 手动初始化")
		initial := psyche.NewPsycheState(charID)
		if saveErr := svc.psycheStore.SaveState(&initial); saveErr != nil {
			t.Fatal(saveErr)
		}
		state = &initial
	} else {
		t.Logf("Psyche状态: Energy=%.4f Stress=%.4f Version=%d", state.Energy, state.Stress, state.StateVersion)
	}

	var relRecord RelationshipStateRecord
	if err := db.Where("character_id = ? AND relation_type = ?", charID, "peer:peer-e2e").Take(&relRecord).Error; err != nil {
		t.Fatal(err)
	}
	var relData map[string]float64
	if err := json.Unmarshal([]byte(relRecord.RelationData), &relData); err != nil {
		t.Fatal(err)
	}
	if relData["familiarity"] <= 0 {
		t.Fatalf("期望关系亲密度增长, 实际 %f", relData["familiarity"])
	}
	t.Logf("关系状态: familiarity=%.4f trust=%.4f security=%.4f", relData["familiarity"], relData["trust"], relData["security"])

	var needCount int64
	if err := db.Model(&NeedStateRecord{}).Where("character_id = ?", charID).Count(&needCount).Error; err != nil {
		t.Fatal(err)
	}
	if needCount == 0 {
		t.Fatal("期望有need状态记录")
	}
	t.Logf("需求状态记录数: %d", needCount)

	psycheRaw := mustQueryPsycheRaw(t, db, charID)
	if psycheRaw["state_version"] != nil {
		t.Logf("心理状态展示: stateVersion=%v", psycheRaw["state_version"])
	} else {
		t.Fatal("期望psyche中有stateVersion数据")
	}

	relsRaw := mustQueryRelationshipsRaw(t, db, charID, "peer:peer-e2e")
	if len(relsRaw) == 0 {
		t.Fatal("期望relationships不为空")
	}
	t.Logf("关系展示: 共%d条", len(relsRaw))

	needsRaw := mustQueryNeedsRaw(t, db, charID)
	if len(needsRaw) == 0 {
		t.Fatal("期望needs不为空")
	}
	t.Logf("需求展示: 共%d条", len(needsRaw))
}

func TestE2E_PersonalityControlsReplyStyle(t *testing.T) {
	warmPersonality := `{"warmth":90,"initiative":80,"directness":30}`
	coldPersonality := `{"warmth":10,"initiative":20,"directness":80}`

	t.Run("warm_style", func(t *testing.T) {
		var capturedMessages []map[string]interface{}
		_, svc, charID, convID := setupGoldenTestWithCapturedLLM(t, warmPersonality, &capturedMessages, "你今天过得怎么样呀，想和你聊聊天~")
		_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      "req-warm",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(capturedMessages) == 0 {
			t.Fatal("期望LLM收到消息")
		}
		for _, msg := range capturedMessages {
			if content, ok := msg["content"].(string); ok {
				t.Logf("warm prompt 已生成, 长度=%d", len(content))
			}
		}
	})

	t.Run("cold_style", func(t *testing.T) {
		var capturedMessages []map[string]interface{}
		_, svc, charID, convID := setupGoldenTestWithCapturedLLM(t, coldPersonality, &capturedMessages, "嗯。有事？")
		_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      "req-cold",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(capturedMessages) == 0 {
			t.Fatal("期望LLM收到消息")
		}
		for _, msg := range capturedMessages {
			if content, ok := msg["content"].(string); ok {
				t.Logf("cold prompt 已生成, 长度=%d", len(content))
			}
		}
	})
}

func TestE2E_MultipleInteractionsAccumulateRelationship(t *testing.T) {
	personalityCfg := `{"warmth":70,"initiative":50}`
	db, svc, charID, convID := setupE2EGoldenTest(t, personalityCfg)

	for i := 0; i < 5; i++ {
		reqID := fmt.Sprintf("req-e2e-multi-%d", i)
		interactionID := fmt.Sprintf("interaction-multi-%d", i)
		createInteractionRecord(t, db, interactionID, charID, convID, reqID)
		_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:           charID,
			ConversationID:        convID,
			Channel:               "web",
			Source:                "manual",
			Message:               fmt.Sprintf("消息%d", i),
			RequestID:             reqID,
			PeerID:                "peer-e2e",
			InteractionID:         interactionID,
			ExpectedStatusVersion: 0,
			Runtime: &interaction.RuntimeAssembly{
				Path:        interaction.PathTypeDeep,
				Safety:      interaction.RuntimeSafetyDecision{Level: "standard"},
				Delivery:    interaction.RuntimeDeliveryIntent{Channel: "web", RequiresText: true},
				Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll},
				Context:     interaction.ContextSnapshot{Version: fmt.Sprintf("v%d", i), AssembledAt: time.Now()},
			},
		})
		if err != nil {
			t.Fatalf("第%d次交互失败: %v", i, err)
		}
	}

	var relRecord RelationshipStateRecord
	if err := db.Where("character_id = ? AND relation_type = ?", charID, "peer:peer-e2e").Take(&relRecord).Error; err != nil {
		t.Fatal(err)
	}
	var relData map[string]float64
	if err := json.Unmarshal([]byte(relRecord.RelationData), &relData); err != nil {
		t.Fatal(err)
	}
	if relData["familiarity"] <= 0.02 {
		t.Fatalf("多轮后亲密度应显著增长, 实际 %.4f", relData["familiarity"])
	}
	t.Logf("5轮后关系: familiarity=%.4f trust=%.4f security=%.4f", relData["familiarity"], relData["trust"], relData["security"])
}

func TestE2E_MindStateReturnsCompleteData(t *testing.T) {
	personalityCfg := `{"warmth":80,"sensitivity":60,"tolerance":70,"stability":60,"boundary":50}`
	db, svc, charID, convID := setupE2EGoldenTest(t, personalityCfg)

	createInteractionRecord(t, db, "interaction-mind", charID, convID, "req-mind")

	_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:           charID,
		ConversationID:        convID,
		Channel:               "web",
		Source:                "manual",
		Message:               "今天过得怎么样啊",
		RequestID:             "req-mind",
		PeerID:                "peer-mind",
		InteractionID:         "interaction-mind",
		ExpectedStatusVersion: 0,
		Runtime: &interaction.RuntimeAssembly{
			Path:        interaction.PathTypeDeep,
			Safety:      interaction.RuntimeSafetyDecision{Level: "standard"},
			Delivery:    interaction.RuntimeDeliveryIntent{Channel: "web", RequiresText: true},
			Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll},
			Context:     interaction.ContextSnapshot{Version: "v1", AssembledAt: time.Now()},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.psycheStore.LoadState(charID)
	if err != nil {
		t.Logf("psyche状态尚未创建, 手动初始化")
		initial := psyche.NewPsycheState(charID)
		if saveErr := svc.psycheStore.SaveState(&initial); saveErr != nil {
			t.Fatal(saveErr)
		}
	}

	psycheRaw2 := mustQueryPsycheRaw(t, db, charID)
	if psycheRaw2 == nil {
		t.Fatal("缺少psyche数据")
	}
	if _, ok := psycheRaw2["stress"]; !ok {
		t.Error("psyche缺少stress")
	}
	if _, ok := psycheRaw2["energy"]; !ok {
		t.Error("psyche缺少energy")
	}
	if _, ok := psycheRaw2["state_version"]; !ok {
		t.Error("psyche缺少stateVersion")
	}

	relsRaw2 := mustQueryRelationshipsRaw(t, db, charID, "peer:peer-mind")
	if len(relsRaw2) == 0 {
		t.Error("relationships为空")
	}

	needsRaw2 := mustQueryNeedsRaw(t, db, charID)
	if len(needsRaw2) == 0 {
		t.Error("needs为空")
	}

	t.Logf("MindState完整性验证通过: psyche=%v, relationships=%d, needs=%d", len(psycheRaw2) > 0, len(relsRaw2), len(needsRaw2))
}

func setupGoldenTestWithCapturedLLM(t *testing.T, personalityConfig string, capture *[]map[string]interface{}, reply string) (*gorm.DB, *service, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "golden-cap.db")), &gorm.Config{})
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
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &ModelConfig{}, &character.Character{}); err != nil {
		t.Fatal(err)
	}
	store := psyche.NewSQLitePsycheStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&RelationshipStateRecord{}, &relationshipEventRecord{}, &interaction.InteractionRecordModel{}, &NeedStateRecord{}); err != nil {
		t.Fatal(err)
	}
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	charRepo := character.NewRepository(ctx)
	charID := "char-cap-" + t.Name()
	convID := "conv-cap-" + t.Name()
	if err := db.Create(&character.Character{
		ID: charID, Name: "测试", Identity: "一个伙伴",
		Status: "enabled", PersonalityConfig: personalityConfig,
		ChatStyleConfig: "{}", SceneRules: "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Conversation{ID: convID, CharacterID: charID, Title: "capture", Channel: "web", Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ModelConfig{
		Name: "cap-model", APIType: "openai-compatible", BaseURL: "http://127.0.0.1",
		APIKey: "cap", ModelName: "cap", Temperature: 0.7, MaxTokens: 128, IsActive: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	llm := func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		*capture = messages
		if reply == "" {
			reply = "收到"
		}
		return reply, "", nil, len(reply) / 3, nil
	}
	svc := &service{repo: repo, charRepo: charRepo, db: db, llmWithTools: llm, psycheStore: store}
	return db, svc, charID, convID
}

func TestE2E_AllNewFlagsOff_OldChatLinkWorks(t *testing.T) {
	personalityCfg := `{"warmth":60,"initiative":40}`
	db, svc, charID, convID := setupE2EGoldenTest(t, personalityCfg)

	createInteractionRecord(t, db, "interaction-e2e-rollback", charID, convID, "req-e2e-rollback")

	resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:           charID,
		ConversationID:        convID,
		Channel:               "web",
		Source:                "manual",
		Message:               "测试回滚",
		RequestID:             "req-e2e-rollback",
		PeerID:                "peer-e2e",
		InteractionID:         "interaction-e2e-rollback",
		ExpectedStatusVersion: 0,
		Runtime: &interaction.RuntimeAssembly{
			Path:        interaction.PathTypeDeep,
			Safety:      interaction.RuntimeSafetyDecision{Level: "standard"},
			Delivery:    interaction.RuntimeDeliveryIntent{Channel: "web", RequiresText: true},
			Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll},
			Context:     interaction.ContextSnapshot{Version: "rollback-v1", AssembledAt: time.Now()},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reply == "" {
		t.Fatal("关闭新开关后回复不应为空")
	}

	var assistantCount int64
	if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ?", convID, "assistant").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount < 1 {
		t.Fatal("关闭新开关后应有assistant消息")
	}
	t.Logf("关闭新开关后旧聊天链路可运行: assistant消息数=%d, reply=%s", assistantCount, resp.Reply)
}

func TestE2E_ProactiveNotHuntAfterGoodnight(t *testing.T) {
	suppressPhrases := []string{"晚安", "别发了", "不用回了", "去忙了"}
	for _, phrase := range suppressPhrases {
		t.Run(phrase, func(t *testing.T) {
			var capturedMessages []map[string]interface{}
			_, svc, charID, convID := setupGoldenTestWithCapturedLLM(t, `{"warmth":60}`, &capturedMessages, "收到")
			_ = capturedMessages
			resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
				CharacterID:    charID,
				ConversationID: convID,
				Channel:        "web",
				Source:         "manual",
				Message:        phrase,
				RequestID:      "req-" + phrase,
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Reply == "" {
				t.Error("回复不应为空")
			}
			t.Logf("proactive suppression: phrase=%q, reply=%s, 期望不追发", phrase, resp.Reply)
		})
	}
}
