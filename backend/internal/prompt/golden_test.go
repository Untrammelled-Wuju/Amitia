package prompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/config"
)

func goldenPath(name string) string {
	return filepath.Join("testdata", "golden", name)
}

func ensureGoldenDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("testdata", "golden")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create golden dir: %v", err)
	}
	return dir
}

func writeGolden(t *testing.T, name string, data []byte) {
	t.Helper()
	ensureGoldenDir(t)
	path := goldenPath(name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write golden file: %v", err)
	}
}

func readGolden(t *testing.T, name string) ([]byte, bool) {
	t.Helper()
	path := goldenPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

func goldenAssertOrUpdate(t *testing.T, name string, data []byte) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		writeGolden(t, name, data)
		return
	}
	expected, ok := readGolden(t, name)
	if !ok {
		writeGolden(t, name, data)
		return
	}
	if string(expected) != string(data) {
		t.Errorf("golden mismatch for %s", name)
	}
}

func allFlagsEnabled() config.PromptFeatureFlags {
	return config.PromptFeatureFlags{
		TextlibRawEnabled:      true,
		PersonalityRawEnabled:  true,
		EmotionFusionEnabled:   true,
		IntimacyDefaultEnabled: true,
		MemoryRawEnabled:       true,
		ReplySanitizerEnabled:  true,
		ProactiveRawEnabled:    true,
	}
}

func allFlagsDisabled() config.PromptFeatureFlags {
	return config.PromptFeatureFlags{}
}

func containsSub(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsSection(gw GwIR, id string) bool {
	for _, s := range gw.Sections {
		if s.ID == id && s.Enabled {
			return true
		}
	}
	return false
}

func initGoldenTestFlags(t *testing.T, flags config.PromptFeatureFlags) func() {
	t.Helper()
	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}
	old := config.AppCfg.Prompt
	config.AppCfg.Prompt = flags
	return func() { config.AppCfg.Prompt = old }
}

func TestGoldenCompileIRSnapshot(t *testing.T) {
	sections := []Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 120, Source: "system", Sensitivity: SensitivityInternal, Content: "You are a helpful companion. Stay safe and respectful. Do not role-play harmful scenarios."},
		{Type: SectionTypeIdentity, Priority: 90, TokenBudget: 80, Source: "character", Sensitivity: SensitivityInternal, Content: "Name: Amitia. Identity: AI companion. Speaking style: warm and direct."},
		{Type: SectionTypeBehaviorPlan, Priority: 80, TokenBudget: 60, Source: "decision", Sensitivity: SensitivityInternal, Content: "Be warm. Use humor when appropriate. Stay concise."},
		{Type: SectionTypeMemory, Priority: 40, TokenBudget: 50, Source: "memory", Sensitivity: SensitivityUserData, Content: "User likes tea. User mentioned jogging on weekends."},
		{Type: SectionTypeHistory, Priority: 20, TokenBudget: 40, Source: "history", Sensitivity: SensitivityUserData, Trimmable: true, Content: "User: Hello\nAmitia: Hi there!"},
	}

	ir := CompileIR(sections, CompileOptions{
		DropEmptySections: true,
	})

	snapshot := SnapshotIR(ir)

	goldenName := "compile_ir_snapshot.json"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	expected, ok := readGolden(t, goldenName)
	if !ok {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	actual, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal actual: %v", err)
	}

	var expectedIR, actualIR IR
	if err := json.Unmarshal(expected, &expectedIR); err != nil {
		t.Fatalf("failed to unmarshal golden: %v", err)
	}
	if err := json.Unmarshal(actual, &actualIR); err != nil {
		t.Fatalf("failed to unmarshal actual: %v", err)
	}

	if expectedIR.Version != actualIR.Version {
		t.Fatalf("golden mismatch version:\nexpected=%s\nactual=%s", expectedIR.Version, actualIR.Version)
	}
	if len(expectedIR.Sections) != len(actualIR.Sections) {
		t.Fatalf("golden mismatch section count:\nexpected=%d\nactual=%d", len(expectedIR.Sections), len(actualIR.Sections))
	}
	for i := range expectedIR.Sections {
		if expectedIR.Sections[i].Type != actualIR.Sections[i].Type {
			t.Fatalf("golden mismatch section[%d] type:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Type, actualIR.Sections[i].Type)
		}
		if expectedIR.Sections[i].Content != actualIR.Sections[i].Content {
			t.Fatalf("golden mismatch section[%d] content:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Content, actualIR.Sections[i].Content)
		}
	}
}

func TestGoldenBudgetApply(t *testing.T) {
	sections := []Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 20, Source: "system", Sensitivity: SensitivityInternal, Content: "You are a helpful assistant. Follow safety rules. Be respectful. Do not share personal data."},
		{Type: SectionTypeCurrentInput, Priority: 90, TokenBudget: 15, Source: "message", Sensitivity: SensitivityUserData, Content: "What is the weather like today in Beijing?"},
		{Type: SectionTypeMemory, Priority: 40, TokenBudget: 12, Source: "memory", Sensitivity: SensitivityUserData, Trimmable: true, Content: "User likes tea. User lives in Beijing. User jogs every morning."},
		{Type: SectionTypeHistory, Priority: 20, TokenBudget: 10, Source: "history", Sensitivity: SensitivityUserData, Trimmable: true, Content: "User: Hi\nAssistant: Hello! User: How are you today?"},
	}

	ir := CompileIR(sections, CompileOptions{DropEmptySections: true})

	budgeted := ApplyBudget(ir, BudgetPolicy{
		MaxPromptTokens: 30,
		SectionLimits: map[SectionType]SectionBudget{
			SectionTypeSystem:       {MaxTokens: 15, MinTokens: 4, Priority: 100},
			SectionTypeCurrentInput: {MaxTokens: 12, MinTokens: 4, Priority: 90},
			SectionTypeMemory:       {MaxTokens: 6, MinTokens: 0, Priority: 40, TrimReason: "low_priority_memory_trimmed"},
			SectionTypeHistory:      {MaxTokens: 4, MinTokens: 0, Priority: 20, TrimReason: "old_history_trimmed"},
		},
	})

	goldenName := "budget_apply.json"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		data, err := json.MarshalIndent(budgeted, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	expected, ok := readGolden(t, goldenName)
	if !ok {
		data, err := json.MarshalIndent(budgeted, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	actual, err := json.MarshalIndent(budgeted, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal actual: %v", err)
	}

	var expectedIR, actualIR IR
	if err := json.Unmarshal(expected, &expectedIR); err != nil {
		t.Fatalf("failed to unmarshal golden: %v", err)
	}
	if err := json.Unmarshal(actual, &actualIR); err != nil {
		t.Fatalf("failed to unmarshal actual: %v", err)
	}

	if len(expectedIR.Sections) != len(actualIR.Sections) {
		t.Fatalf("golden mismatch section count:\nexpected=%d\nactual=%d", len(expectedIR.Sections), len(actualIR.Sections))
	}
	for i := range expectedIR.Sections {
		if expectedIR.Sections[i].Type != actualIR.Sections[i].Type {
			t.Fatalf("golden mismatch section[%d] type:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Type, actualIR.Sections[i].Type)
		}
		if expectedIR.Sections[i].Content != actualIR.Sections[i].Content {
			t.Fatalf("golden mismatch section[%d] content:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Content, actualIR.Sections[i].Content)
		}
	}
	if len(expectedIR.Audit.TrimRecords) != len(actualIR.Audit.TrimRecords) {
		t.Fatalf("golden mismatch trim records:\nexpected=%d\nactual=%d", len(expectedIR.Audit.TrimRecords), len(actualIR.Audit.TrimRecords))
	}
}

func TestGoldenRenderIR(t *testing.T) {
	sections := []Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 40, Source: "system", Sensitivity: SensitivityInternal, Content: "You are a companion AI. Be safe, kind, and helpful."},
		{Type: SectionTypeIdentity, Priority: 90, TokenBudget: 30, Source: "character", Sensitivity: SensitivityInternal, Content: "Name: Amitia"},
		{Type: SectionTypeBehaviorPlan, Priority: 80, TokenBudget: 30, Source: "decision", Sensitivity: SensitivityInternal, Content: "Be warm and concise."},
		{Type: SectionTypeMemory, Priority: 40, TokenBudget: 20, Source: "memory", Sensitivity: SensitivityUserData, DataOnly: true, Content: "User likes tea."},
	}

	ir := CompileIR(sections, CompileOptions{DropEmptySections: true})
	rendered := RenderIR(ir)

	goldenName := "render_ir.txt"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		writeGolden(t, goldenName, []byte(rendered))
		return
	}

	expected, ok := readGolden(t, goldenName)
	if !ok {
		writeGolden(t, goldenName, []byte(rendered))
		return
	}

	if string(expected) != rendered {
		t.Fatalf("golden mismatch render:\nexpected:\n%s\nactual:\n%s", string(expected), rendered)
	}
}

func TestGolden_NormalChatSections(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		CharacterConfig:  "测试角色配置",
		CurrentUserInput: "用户输入内容",
	})

	required := []string{"platform_policy", "base_identity", "app_contract", "cognitive_contract", "anti_flattery_contract", "technical_task_contract", "character_contract", "current_user_message"}
	for _, id := range required {
		if !containsSection(ir, id) {
			t.Errorf("期望包含 section %s，实际未找到", id)
		}
	}

	data, _ := json.MarshalIndent(ir, "", "  ")
	goldenAssertOrUpdate(t, "normal_chat.json", data)
}

func TestGolden_PersonalitySection(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		PersonalityRaw:   "人格原始提示词内容",
		CurrentUserInput: "你好",
	})

	if !containsSection(ir, "personality_raw") {
		t.Error("flags全部开启时应包含 personality_raw section")
	}
}

func TestGolden_PersonalitySectionDisabled(t *testing.T) {
	flags := allFlagsEnabled()
	flags.PersonalityRawEnabled = false
	cleanup := initGoldenTestFlags(t, flags)
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		PersonalityRaw:   "人格原始提示词内容",
		CurrentUserInput: "你好",
	})

	if containsSection(ir, "personality_raw") {
		t.Error("PersonalityRawEnabled=false 时不应包含 personality_raw section")
	}
}

func TestGolden_EmotionFusionSection(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		EmotionFusionRaw: "情绪融合提示词",
		CurrentUserInput: "你好",
	})

	if !containsSection(ir, "emotion_fusion_raw") {
		t.Error("flags全部开启时应包含 emotion_fusion_raw section")
	}
}

func TestGolden_EmotionFusionSectionDisabled(t *testing.T) {
	flags := allFlagsEnabled()
	flags.EmotionFusionEnabled = false
	cleanup := initGoldenTestFlags(t, flags)
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		EmotionFusionRaw: "情绪融合提示词",
		CurrentUserInput: "你好",
	})

	if containsSection(ir, "emotion_fusion_raw") {
		t.Error("EmotionFusionEnabled=false 时不应包含 emotion_fusion_raw section")
	}
}

func TestGolden_AdultIntimacySection(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		AdultIntimacyRaw: "成人亲密边界提示词",
		CurrentUserInput: "你好",
	})

	if !containsSection(ir, "adult_intimacy_raw") {
		t.Error("flags全部开启时应包含 adult_intimacy_raw section")
	}
}

func TestGolden_AdultIntimacySectionDisabled(t *testing.T) {
	flags := allFlagsEnabled()
	flags.IntimacyDefaultEnabled = false
	cleanup := initGoldenTestFlags(t, flags)
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		AdultIntimacyRaw: "成人亲密边界提示词",
		CurrentUserInput: "你好",
	})

	if containsSection(ir, "adult_intimacy_raw") {
		t.Error("IntimacyDefaultEnabled=false 时不应包含 adult_intimacy_raw section")
	}
}

func TestGolden_MemoryInjectSection(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		MemoryInjectRaw:  "记忆注入原始文本",
		CurrentUserInput: "你好",
	})

	if !containsSection(ir, "memory_inject_raw") {
		t.Error("flags全部开启时应包含 memory_inject_raw section")
	}
}

func TestGolden_MemoryInjectSectionDisabled(t *testing.T) {
	flags := allFlagsEnabled()
	flags.MemoryRawEnabled = false
	cleanup := initGoldenTestFlags(t, flags)
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		MemoryInjectRaw:  "记忆注入原始文本",
		CurrentUserInput: "你好",
	})

	if containsSection(ir, "memory_inject_raw") {
		t.Error("MemoryRawEnabled=false 时不应包含 memory_inject_raw section")
	}
}

func TestGolden_OutputShapeAndAntiRepeat(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		OutputShapeRaw:   "输出清洗提示词",
		AntiRepeatRaw:    "防复读提示词",
		CurrentUserInput: "你好",
	})

	if !containsSection(ir, "output_shape_raw") {
		t.Error("应包含 output_shape_raw section")
	}
	if !containsSection(ir, "anti_repeat_raw") {
		t.Error("应包含 anti_repeat_raw section")
	}
}

func TestGolden_OutputShapeDisabled(t *testing.T) {
	flags := allFlagsEnabled()
	flags.ReplySanitizerEnabled = false
	cleanup := initGoldenTestFlags(t, flags)
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		OutputShapeRaw:   "输出清洗提示词",
		AntiRepeatRaw:    "防复读提示词",
		CurrentUserInput: "你好",
	})

	if containsSection(ir, "output_shape_raw") {
		t.Error("ReplySanitizerEnabled=false 时不应包含 output_shape_raw")
	}
	if containsSection(ir, "anti_repeat_raw") {
		t.Error("ReplySanitizerEnabled=false 时不应包含 anti_repeat_raw")
	}
}

func TestGolden_ChannelShortSection(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		ChannelShortRaw:  "微信短句提示词",
		CurrentUserInput: "你好",
	})

	if !containsSection(ir, "channel_short_raw") {
		t.Error("应包含 channel_short_raw section")
	}
}

func TestGolden_ChannelShortSectionDisabled(t *testing.T) {
	flags := allFlagsEnabled()
	flags.TextlibRawEnabled = false
	cleanup := initGoldenTestFlags(t, flags)
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		ChannelShortRaw:  "微信短句提示词",
		CurrentUserInput: "你好",
	})

	if containsSection(ir, "channel_short_raw") {
		t.Error("TextlibRawEnabled=false 时不应包含 channel_short_raw")
	}
}

func TestGolden_ProactiveSections(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:         "测试BaseIdentity",
		ProactiveRaw:         "主动消息原始提示词",
		ProactivePersonality: "主动消息人格",
		ProactiveScene:       "主动消息场景",
		ProactiveTimeContext: "主动消息时间上下文",
		CurrentUserInput:     "你好",
	})

	required := []string{"proactive_raw", "proactive_personality", "proactive_scene", "proactive_time_context"}
	for _, id := range required {
		if !containsSection(ir, id) {
			t.Errorf("期望包含 proactive section %s，实际未找到", id)
		}
	}
}

func TestGolden_ProactiveSectionsDisabled(t *testing.T) {
	flags := allFlagsEnabled()
	flags.ProactiveRawEnabled = false
	cleanup := initGoldenTestFlags(t, flags)
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:         "测试BaseIdentity",
		ProactiveRaw:         "主动消息原始提示词",
		ProactivePersonality: "主动消息人格",
		ProactiveScene:       "主动消息场景",
		ProactiveTimeContext: "主动消息时间上下文",
		CurrentUserInput:     "你好",
	})

	proactiveIDs := []string{"proactive_raw", "proactive_personality", "proactive_scene", "proactive_time_context", "proactive_relationship", "proactive_emotion", "proactive_memory", "proactive_recent_context"}
	for _, id := range proactiveIDs {
		if containsSection(ir, id) {
			t.Errorf("ProactiveRawEnabled=false 时不应包含 %s", id)
		}
	}
}

func TestGolden_AllFlagsEnabledBuild(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:         "测试BaseIdentity",
		CharacterConfig:      "测试角色",
		PersonalityRaw:       "人格",
		EmotionFusionRaw:     "情绪",
		AdultIntimacyRaw:     "成人",
		MemoryInjectRaw:      "记忆注入",
		MemoryExtractRaw:     "记忆抽取",
		OutputShapeRaw:       "输出清洗",
		AntiRepeatRaw:        "防复读",
		ProactiveRaw:         "主动消息",
		ProactivePersonality: "主动人格",
		ProactiveScene:       "主动场景",
		ProactiveTimeContext: "主动时间",
		ChannelShortRaw:      "短句",
		CurrentUserInput:     "测试",
	})

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("渲染后消息为空")
	}
	if msgs[0].Role != "system" {
		t.Fatalf("第一条消息应为system, 实际为 %s", msgs[0].Role)
	}

	v := NewValidator()
	if err := v.ValidateMessages(msgs); err != nil {
		t.Fatalf("校验messages失败: %v", err)
	}

	data, _ := json.MarshalIndent(msgs, "", "  ")
	goldenAssertOrUpdate(t, "all_flags_render.json", data)
}

func TestGolden_AllFlagsDisabledBuild(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsDisabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:         "测试BaseIdentity",
		CharacterConfig:      "测试角色",
		PersonalityRaw:       "人格",
		EmotionFusionRaw:     "情绪",
		AdultIntimacyRaw:     "成人",
		MemoryInjectRaw:      "记忆注入",
		MemoryExtractRaw:     "记忆抽取",
		OutputShapeRaw:       "输出清洗",
		AntiRepeatRaw:        "防复读",
		ProactiveRaw:         "主动消息",
		ProactivePersonality: "主动人格",
		ProactiveScene:       "主动场景",
		ProactiveTimeContext: "主动时间",
		ChannelShortRaw:      "短句",
		CurrentUserInput:     "测试",
	})

	newSectionIDs := []string{"personality_raw", "emotion_fusion_raw", "adult_intimacy_raw", "memory_inject_raw", "memory_extract_raw", "output_shape_raw", "anti_repeat_raw", "proactive_raw", "proactive_personality", "proactive_scene", "proactive_time_context", "proactive_relationship", "proactive_emotion", "proactive_memory", "proactive_recent_context", "channel_short_raw"}
	for _, id := range newSectionIDs {
		if containsSection(ir, id) {
			t.Errorf("全部关闭后不应包含 %s", id)
		}
	}

	coreIDs := []string{"platform_policy", "base_identity", "app_contract", "cognitive_contract", "anti_flattery_contract", "technical_task_contract", "current_user_message"}
	for _, id := range coreIDs {
		if !containsSection(ir, id) {
			t.Errorf("核心 section %s 应始终存在", id)
		}
	}

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("全部关闭后渲染失败: %v", err)
	}

	v := NewValidator()
	if err := v.ValidateMessages(msgs); err != nil {
		t.Fatalf("全部关闭后校验messages失败: %v", err)
	}

	data, _ := json.MarshalIndent(msgs, "", "  ")
	goldenAssertOrUpdate(t, "all_flags_off_render.json", data)
}

func TestGolden_ReplySanitizerBlocksMetadata(t *testing.T) {
	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		ProactiveRaw:     "主动消息",
		ProactiveScene:   "主动场景",
		OutputShapeRaw:   "输出清洗",
		CurrentUserInput: "你好",
	})

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	for _, m := range msgs {
		content := m.Content
		forbidden := []string{"系统提醒", "任务：", "主动消息：", "任务:", "主动消息:"}
		for _, fb := range forbidden {
			if containsSub(content, fb) {
				t.Errorf("prompt中不应包含元信息 %q", fb)
			}
		}
	}
}

func TestGolden_ProactiveSceneSystemPlacement(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		ProactiveScene:   "这是主动消息场景描述",
		CurrentUserInput: "你好",
	})

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	systemContent := msgs[0].Content
	if !containsSub(systemContent, "这是主动消息场景描述") {
		t.Error("ProactiveScene 应渲染到 system 消息中")
	}
}

func TestGolden_ProactivePersonalityCharacterPlacement(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:         "测试BaseIdentity",
		ProactivePersonality: "主动消息人格指令",
		CurrentUserInput:     "你好",
	})

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	found := false
	for i := 1; i < len(msgs); i++ {
		if containsSub(msgs[i].Content, "主动消息人格指令") {
			found = true
			break
		}
	}
	if !found {
		t.Error("ProactivePersonality 应渲染到 character 消息中")
	}
}

func TestGolden_AllFlagsOffOldChatLink(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsDisabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:     "测试BaseIdentity",
		CharacterConfig:  "角色",
		CurrentUserInput: "你好用户",
		History:          "User: 你好\nAmitia: 你好呀",
		ProfileContext:   "用户画像",
		MemoryContext:    "记忆",
	})

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("旧链路渲染失败: %v", err)
	}

	v := NewValidator()
	if err := v.ValidateIR(ir); err != nil {
		t.Fatalf("旧链路ValidateIR失败: %v", err)
	}
	if err := v.ValidateMessages(msgs); err != nil {
		t.Fatalf("旧链路ValidateMessages失败: %v", err)
	}

	t.Logf("旧链路渲染成功，消息数=%d", len(msgs))
	for i, m := range msgs {
		t.Logf("消息%d: role=%s content_len=%d", i, m.Role, len(m.Content))
	}
}

func TestGolden_ProactiveTaskInstructionNotInCurrentUserMessage(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:             "测试BaseIdentity",
		ProactiveTaskInstruction: "这是一条主动消息任务指令，不应出现在用户消息中",
		CurrentUserInput:         "你好",
	})

	if !containsSection(ir, "proactive_task_instruction") {
		t.Fatal("期望包含 proactive_task_instruction section")
	}

	currentUserSection := getSectionContent(ir, "current_user_message")
	if currentUserSection != "(proactive)" {
		t.Errorf("current_user_message 内容应为 (proactive)，实际为: %s", currentUserSection)
	}

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	if len(msgs) == 0 {
		t.Fatal("渲染后消息为空")
	}

	lastMsg := msgs[len(msgs)-1]
	if containsSub(lastMsg.Content, "这是一条主动消息任务指令") {
		t.Error("ProactiveTaskInstruction 文本不应出现在 current_user_message 中")
	}

	systemMsg := msgs[0].Content
	if !containsSub(systemMsg, "这是一条主动消息任务指令") {
		t.Error("ProactiveTaskInstruction 文本应出现在 system 消息中")
	}

	for _, m := range msgs {
		if m.Role == "user" && containsSub(m.Content, "<current_user_message>") {
			cmContent := m.Content
			if containsSub(cmContent, "这是一条主动消息任务指令") {
				t.Error("ProactiveTaskInstruction 文本泄漏到了 <current_user_message> 块中")
			}
		}
	}
}

func TestGolden_ProactiveContextInjection(t *testing.T) {
	cleanup := initGoldenTestFlags(t, allFlagsEnabled())
	defer cleanup()

	b := NewBuilder()
	ir := b.Build(BuildRequest{
		BaseIdentity:          "测试BaseIdentity",
		ProactiveRelationship: "关系类型：CLOSE_FRIEND\n关系数据：亲密好友",
		ProactiveEmotion:      "情绪：{\"joy\":0.8}\n心情：{\"calm\":0.6}\n压力：20\n精力：80",
		ProactiveMemory:       "最近记忆：上次一起看了电影",
		CurrentUserInput:      "你好",
	})

	required := []string{"proactive_relationship", "proactive_emotion", "proactive_memory"}
	for _, id := range required {
		if !containsSection(ir, id) {
			t.Errorf("期望包含 %s section", id)
		}
	}

	r := NewRenderer()
	msgs, err := r.Render(ir)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	if len(msgs) < 3 {
		t.Fatalf("期望至少3条消息，实际 %d", len(msgs))
	}

	contextMsg := msgs[1]
	if !containsSub(contextMsg.Content, "关系类型") {
		t.Error("proactive_relationship 内容未注入到渲染输出")
	}
	if !containsSub(contextMsg.Content, "关系类型：CLOSE_FRIEND") || !containsSub(contextMsg.Content, "亲密好友") {
		t.Error("proactive_relationship 的关系数据未完整注入")
	}
	if !containsSub(contextMsg.Content, "情绪") {
		t.Error("proactive_emotion 内容未注入到渲染输出")
	}
	if !containsSub(contextMsg.Content, "joy") {
		t.Error("proactive_emotion 的情绪数据未完整注入")
	}
	if !containsSub(contextMsg.Content, "上次一起看了电影") {
		t.Error("proactive_memory 的记忆内容未注入到渲染输出")
	}
}

func getSectionContent(ir GwIR, id string) string {
	for _, s := range ir.Sections {
		if s.ID == id {
			return s.Content
		}
	}
	return ""
}
