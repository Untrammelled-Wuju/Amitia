package emote

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/delivery"
	"gorm.io/gorm"
)

func setupEmoteTest(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "emote.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&Emote{}, &Group{}, &GroupItem{}, &CharacterBinding{}, &CharacterSettings{}, &SendRecord{}, &chat.Conversation{}, &chat.Message{}, &delivery.DeliveryIntentModel{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("CREATE TABLE characters (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("INSERT INTO characters (id) VALUES ('c1'), ('c2')").Error; err != nil {
		t.Fatal(err)
	}
	config.AppCfg = &config.Config{Storage: config.StorageConfig{DataDir: t.TempDir()}}
	return NewService(db, delivery.NewSQLiteDeliveryStore(db)), db
}

func pngBytes(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 12, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, c)
		}
	}
	var body bytes.Buffer
	if err := png.Encode(&body, img); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func uploadHeader(t *testing.T, filename, mime string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err = req.ParseMultipartForm(2 << 20); err != nil {
		t.Fatal(err)
	}
	header := req.MultipartForm.File["file"][0]
	header.Header.Set("Content-Type", mime)
	return header
}

func addTestEmote(t *testing.T, service *Service, item Emote, characters ...string) {
	t.Helper()
	if item.Keywords == "" {
		item.Keywords = "[]"
	}
	if item.CreatedAt == "" {
		item.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
		item.UpdatedAt = item.CreatedAt
	}
	if err := service.repo.Create(&item, nil, characters); err != nil {
		t.Fatal(err)
	}
}

func TestRoleScopeAIDisableAndGroupsDoNotAffectPermission(t *testing.T) {
	service, db := setupEmoteTest(t)
	all := Emote{ID: "all", Name: "全角色", Meaning: "开心", Enabled: 1, AIEnabled: 1, RoleScope: RoleScopeAll}
	selected := Emote{ID: "selected", Name: "指定", Meaning: "开心", Enabled: 1, AIEnabled: 1, RoleScope: RoleScopeSelected}
	disabled := Emote{ID: "disabled", Name: "禁用", Meaning: "开心", Enabled: 1, AIEnabled: 0, RoleScope: RoleScopeAll}
	addTestEmote(t, service, all)
	addTestEmote(t, service, selected, "c1")
	addTestEmote(t, service, disabled)
	group := Group{ID: "g1", Name: "任意分组", CreatedAt: time.Now().Format("2006-01-02 15:04:05"), UpdatedAt: time.Now().Format("2006-01-02 15:04:05")}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.repo.AddToGroup(group.ID, []string{selected.ID}); err != nil {
		t.Fatal(err)
	}
	if !service.repo.CanCharacterUse(&all, "c2") || !service.repo.CanCharacterUse(&selected, "c1") || service.repo.CanCharacterUse(&selected, "c2") || service.repo.CanCharacterUse(&disabled, "c1") {
		t.Fatal("角色范围或 AI 禁用过滤结果不符合预期")
	}
	if !service.repo.CanCharacterUse(&selected, "c1") {
		t.Fatal("分组不应影响角色权限")
	}
}

func TestSemanticTextProbabilityAndWeightedSelection(t *testing.T) {
	item := &Emote{Name: "微笑", Meaning: "开心", Keywords: `["友好"]`}
	text := SemanticText(item)
	if strings.Contains(text, "分组") || !strings.Contains(text, "开心") {
		t.Fatalf("语义文本不正确: %s", text)
	}
	if got := FinalProbability(0.2, 0.3, 2); got != 0.3 {
		t.Fatalf("概率上界错误: %v", got)
	}
	if got := FinalProbability(-1, 0.3, 1); got != 0 {
		t.Fatalf("概率下界错误: %v", got)
	}
	candidates := []DecisionCandidate{{Emote: Emote{ID: "a"}, Score: 0.4}, {Emote: Emote{ID: "b"}, Score: 0.8}}
	first, ok := WeightedSelect(candidates, func() float64 { return 0 })
	if !ok || first.Emote.ID != "a" {
		t.Fatal("Top-K 加权选择低区间错误")
	}
	second, ok := WeightedSelect(candidates, func() float64 { return 0.99 })
	if !ok || second.Emote.ID != "b" {
		t.Fatal("Top-K 加权选择高区间错误")
	}
	if _, ok = WeightedSelect([]DecisionCandidate{{Score: MinimumSimilarity - 0.01}}, func() float64 { return 0 }); ok {
		t.Fatal("低相似度候选应取消")
	}
}

func TestRandomMissSkipsSearchAndDecisionIsIdempotent(t *testing.T) {
	service, db := setupEmoteTest(t)
	assetDir := filepath.Join(config.AppCfg.Storage.DataDir, "emotes", "e1")
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id) VALUES ('conv', 'c1', 'web', '')").Error; err != nil {
		t.Fatal(err)
	}
	addTestEmote(t, service, Emote{ID: "e1", Name: "开心", Meaning: "开心", Enabled: 1, AIEnabled: 1, RoleScope: RoleScopeAll, FilePath: "/emote-assets/e1/original.png", FallbackPath: "/emote-assets/e1/fallback.png"})
	if err := db.Create(&CharacterSettings{CharacterID: "c1", Enabled: 1, BaseProbability: 0.1, MaxProbability: 0.3, MaxPerHour: 5}).Error; err != nil {
		t.Fatal(err)
	}
	called := 0
	decision := NewDecisionService(service)
	decision.random = func() float64 { return 0.99 }
	decision.search = func(string, string, int) ([]DecisionCandidate, error) {
		called++
		return nil, nil
	}
	event := &chat.MessagePlanningEvent{ConversationID: "conv", CharacterID: "c1", RequestID: "r1", Lines: []string{"好的"}, Reply: "好的", Source: "manual"}
	plan := decision.Plan(event)
	if plan == nil || plan.Persist == nil {
		t.Fatal("应生成一次无表情决策计划")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return plan.Persist(tx, nil) }); err != nil {
		t.Fatal(err)
	}
	if duplicate := decision.Plan(event); duplicate != nil {
		t.Fatal("相同回复不应重复决策")
	}
	if called != 0 {
		t.Fatalf("随机未命中不应检索，实际调用 %d 次", called)
	}
	var count int64
	db.Model(&SendRecord{}).Where("response_id = ?", "r1").Count(&count)
	if count != 1 {
		t.Fatalf("一次回复应只决策一次，实际记录 %d", count)
	}
	_ = assetDir
}

func TestRandomHitSearchesAndSelectsOnlyOncePerResponse(t *testing.T) {
	service, db := setupEmoteTest(t)
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id) VALUES ('conv-hit', 'c1', 'web', '')").Error; err != nil {
		t.Fatal(err)
	}
	item := Emote{ID: "e-hit", Name: "开心", Meaning: "开心", Enabled: 1, AIEnabled: 1, RoleScope: RoleScopeAll, FilePath: "/emote-assets/e-hit/original.png", FallbackPath: "/emote-assets/e-hit/fallback.png"}
	addTestEmote(t, service, item)
	assetDir := filepath.Join(config.AppCfg.Storage.DataDir, "emotes", item.ID)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "original.png"), pngBytes(t, color.RGBA{R: 40, G: 180, B: 80, A: 255}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&CharacterSettings{CharacterID: "c1", Enabled: 1, BaseProbability: 1, MaxProbability: 1, MaxPerHour: 5, MinReplyGap: 0, SameEmoteCooldownMinutes: 0}).Error; err != nil {
		t.Fatal(err)
	}
	searchCalls := 0
	randomCalls := 0
	values := []float64{0, 0, 0.1, 0}
	decision := NewDecisionService(service)
	decision.random = func() float64 {
		value := values[randomCalls]
		randomCalls++
		return value
	}
	decision.search = func(string, string, int) ([]DecisionCandidate, error) {
		searchCalls++
		return []DecisionCandidate{{Emote: item, Score: 0.9}}, nil
	}
	event := &chat.MessagePlanningEvent{ConversationID: "conv-hit", CharacterID: "c1", RequestID: "response-hit", Lines: []string{"第一条", "第二条"}, Reply: "第一条\n第二条", Source: "manual"}
	plan := decision.Plan(event)
	if plan == nil || plan.Emote == nil || plan.Emote.EmoteID != item.ID {
		t.Fatalf("命中后应只选择一张表情: %#v", plan)
	}
	if searchCalls != 1 || randomCalls != 4 {
		t.Fatalf("一次回复应只检索和选择一次，search=%d random=%d", searchCalls, randomCalls)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return plan.Persist(tx, &chat.Message{ID: "emote-message"}) }); err != nil {
		t.Fatal(err)
	}
	if duplicate := decision.Plan(event); duplicate != nil {
		t.Fatal("同一 responseGroupId 不应再次决策")
	}
	if searchCalls != 1 {
		t.Fatalf("重复请求不应再次语义检索，实际 %d", searchCalls)
	}
	var count int64
	db.Model(&SendRecord{}).Where("response_id = ?", event.RequestID).Count(&count)
	if count != 1 {
		t.Fatalf("一次回复只允许一条表情决策记录，实际 %d", count)
	}
}

func TestEmoteOnlyRequiresCharacterSetting(t *testing.T) {
	service, db := setupEmoteTest(t)
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id) VALUES ('conv-only', 'c1', 'web', '')").Error; err != nil {
		t.Fatal(err)
	}
	item := Emote{ID: "e-only", Name: "挥手", Meaning: "打招呼", Enabled: 1, AIEnabled: 1, RoleScope: RoleScopeAll, FilePath: "/emote-assets/e-only/original.png", FallbackPath: "/emote-assets/e-only/fallback.png"}
	addTestEmote(t, service, item)
	assetDir := filepath.Join(config.AppCfg.Storage.DataDir, "emotes", item.ID)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "original.png"), pngBytes(t, color.RGBA{R: 80, G: 120, B: 220, A: 255}), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := CharacterSettings{CharacterID: "c1", Enabled: 1, BaseProbability: 1, MaxProbability: 1, MaxPerHour: 5, AllowEmoteOnly: 0}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatal(err)
	}
	decision := NewDecisionService(service)
	decision.random = func() float64 { return 0 }
	decision.search = func(string, string, int) ([]DecisionCandidate, error) {
		return []DecisionCandidate{{Emote: item, Score: 0.9}}, nil
	}
	blocked := decision.Plan(&chat.MessagePlanningEvent{ConversationID: "conv-only", CharacterID: "c1", RequestID: "only-off", UserMessage: "打个招呼", Lines: nil, Reply: "", Source: "manual"})
	if blocked == nil || blocked.Emote != nil {
		t.Fatalf("allow_emote_only 关闭时不得生成纯表情: %#v", blocked)
	}
	if err := db.Model(&CharacterSettings{}).Where("character_id = ?", "c1").Update("allow_emote_only", 1).Error; err != nil {
		t.Fatal(err)
	}
	allowed := decision.Plan(&chat.MessagePlanningEvent{ConversationID: "conv-only", CharacterID: "c1", RequestID: "only-on", UserMessage: "打个招呼", Lines: nil, Reply: "", Source: "manual"})
	if allowed == nil || allowed.Emote == nil || allowed.SendMode != SendModeEmoteOnly || allowed.InsertAfter != 0 {
		t.Fatalf("allow_emote_only 开启后应生成合法纯表情计划: %#v", allowed)
	}
}

func TestMessagePlanPlacementRules(t *testing.T) {
	placement := func(values ...float64) *DecisionService {
		index := 0
		return &DecisionService{random: func() float64 {
			value := values[index]
			index++
			return value
		}}
	}
	insertAfter, mode := placement(0.249, 0).selectPlacement([]string{"第一条", "第二条"}, "manual", "第一条\n第二条")
	if insertAfter != 1 || mode != SendModeBetweenTextMessages {
		t.Fatalf("普通回复 25%% 权重内应允许穿插: %d %s", insertAfter, mode)
	}
	insertAfter, mode = placement(0.25).selectPlacement([]string{"第一条", "第二条"}, "manual", "第一条\n第二条")
	if insertAfter != 2 || mode != SendModeAfterAllText {
		t.Fatalf("普通回复 75%% 权重应后置: %d %s", insertAfter, mode)
	}
	insertAfter, mode = placement(0.299, 0.99).selectPlacement([]string{"第一条", "第二条", "第三条"}, "proactive", "第一条\n第二条\n第三条")
	if insertAfter != 2 || mode != SendModeBetweenTextMessages {
		t.Fatalf("主动推送 30%% 权重内应允许穿插: %d %s", insertAfter, mode)
	}
	insertAfter, mode = placement(0.30).selectPlacement([]string{"第一条", "第二条"}, "proactive", "第一条\n第二条")
	if insertAfter != 2 || mode != SendModeAfterAllText {
		t.Fatalf("主动推送 70%% 权重应后置: %d %s", insertAfter, mode)
	}
	randomCalls := 0
	single := &DecisionService{random: func() float64 { randomCalls++; return 0 }}
	insertAfter, mode = single.selectPlacement([]string{"唯一一条"}, "manual", "唯一一条")
	if insertAfter != 1 || mode != SendModeAfterAllText || randomCalls != 0 {
		t.Fatalf("单条回复只能后置且不应随机位置: %d %s calls=%d", insertAfter, mode, randomCalls)
	}
	unsafeCases := []struct {
		name   string
		lines  []string
		source string
		reply  string
	}{
		{name: "steps", lines: []string{"1. 第一步", "2. 第二步"}, source: "manual", reply: "1. 第一步\n2. 第二步"},
		{name: "code", lines: []string{"```go", "fmt.Println()", "```"}, source: "manual", reply: "```go\nfmt.Println()\n```"},
		{name: "markdown_table", lines: []string{"| 项目 | 值 |", "| --- | --- |"}, source: "manual", reply: "| 项目 | 值 |\n| --- | --- |"},
		{name: "long_explanation", lines: []string{"第一段", "第二段"}, source: "manual", reply: strings.Repeat("知识解释", 61)},
		{name: "tool_result", lines: []string{"第一条", "第二条"}, source: "tool", reply: "第一条\n第二条"},
		{name: "safety_sensitive", lines: []string{"请立即报警", "并联系急救"}, source: "manual", reply: "请立即报警并联系急救"},
	}
	for _, tc := range unsafeCases {
		t.Run(tc.name, func(t *testing.T) {
			decision := &DecisionService{random: func() float64 { t.Fatal("不安全场景不应随机穿插位置"); return 0 }}
			gotAfter, gotMode := decision.selectPlacement(tc.lines, tc.source, tc.reply)
			if gotAfter != len(tc.lines) || gotMode != SendModeAfterAllText {
				t.Fatalf("不安全场景只能后置: %d %s", gotAfter, gotMode)
			}
		})
	}
	if DefaultSettings("c1").AllowEmoteOnly != 0 {
		t.Fatal("emote_only 必须默认关闭")
	}
}

func TestLimitsCooldownAndQdrantFallback(t *testing.T) {
	service, db := setupEmoteTest(t)
	now := time.Now().Format("2006-01-02 15:04:05")
	addTestEmote(t, service, Emote{ID: "e1", Name: "微笑", Meaning: "开心 问候", Keywords: `["友好"]`, Enabled: 1, AIEnabled: 1, RoleScope: RoleScopeAll})
	record := SendRecord{ID: "s1", EmoteID: stringPointer("e1"), CharacterID: "c1", ConversationID: "conv", TriggerType: TriggerAIRandom, TriggerHit: 1, Status: "sent", CreatedAt: now}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	decision := NewDecisionService(service)
	if !decision.hourlyLimitReached("c1", 1) {
		t.Fatal("每小时限制未生效")
	}
	filtered := decision.filterCandidates([]DecisionCandidate{{Emote: Emote{ID: "e1", Enabled: 1, AIEnabled: 1, RoleScope: RoleScopeAll}, Score: 0.9}}, "c1", "web", 30)
	if len(filtered) != 0 {
		t.Fatal("同表情冷却未生效")
	}
	if err := db.Create(&chat.Message{ID: "m1", ConversationID: "conv", Role: "assistant", Content: "一", MsgType: "text", CreatedAt: time.Now().Add(time.Second).Format("2006-01-02 15:04:05")}).Error; err != nil {
		t.Fatal(err)
	}
	if !decision.replyGapBlocked("conv", "c1", 2) {
		t.Fatal("最小回复间隔未生效")
	}
	items := service.semantic.searchText("开心", "c1", 5)
	if len(items) != 1 || items[0].Emote.ID != "e1" {
		t.Fatal("Qdrant 不可用时的文本降级未生效")
	}
}

func TestDeleteGroupPreservesEmoteAndDuplicateUpload(t *testing.T) {
	service, db := setupEmoteTest(t)
	data := pngBytes(t, color.RGBA{R: 180, G: 20, B: 40, A: 255})
	first := service.Import(uploadHeader(t, "first.png", "image/png", data), ImportConfig{Name: "第一张"})
	if first.Status != "success" {
		t.Fatalf("首次导入失败: %#v", first)
	}
	duplicate := service.Import(uploadHeader(t, "second.png", "image/png", data), ImportConfig{Name: "第二张"})
	if duplicate.Status != "duplicate" || duplicate.DuplicateEmoteID != first.EmoteID {
		t.Fatalf("重复文件检测失败: %#v", duplicate)
	}
	group, err := service.CreateGroup("测试分组")
	if err != nil {
		t.Fatal(err)
	}
	if err = service.AddToGroup(group.ID, []string{first.EmoteID}); err != nil {
		t.Fatal(err)
	}
	if err = service.DeleteGroup(group.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&Emote{}).Where("id = ?", first.EmoteID).Count(&count)
	if count != 1 {
		t.Fatal("删除分组不应删除表情")
	}
}

func TestUploadValidation(t *testing.T) {
	service, _ := setupEmoteTest(t)
	bad := service.Import(uploadHeader(t, "bad.png", "image/png", []byte("not-an-image")), ImportConfig{})
	if bad.Status != "failed" || bad.ErrorCode != "unsupported_format" {
		t.Fatalf("非法格式未拒绝: %#v", bad)
	}
	mismatch := service.Import(uploadHeader(t, "bad.jpg", "image/jpeg", pngBytes(t, color.Black)), ImportConfig{})
	if mismatch.Status != "failed" || mismatch.ErrorCode != "unsupported_format" {
		t.Fatalf("扩展名伪装未拒绝: %#v", mismatch)
	}
}

func stringPointer(value string) *string { return &value }
