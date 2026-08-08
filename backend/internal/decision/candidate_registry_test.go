package decision

import "testing"

func TestDefaultCandidateRegistryHasAllTypes(t *testing.T) {
	r := DefaultCandidateRegistry()
	all := r.All()
	if len(all) < 6 {
		t.Fatalf("默认注册表至少应有 6 种候选行为, 实际 %d", len(all))
	}
	chatActions := r.ByType(CandidateActionChat)
	if len(chatActions) == 0 {
		t.Fatal("应包含 chat 类型候选")
	}
	proactiveActions := r.ByType(CandidateActionProactive)
	if len(proactiveActions) == 0 {
		t.Fatal("应包含 proactive 类型候选")
	}
	toolActions := r.ByType(CandidateActionToolCall)
	if len(toolActions) == 0 {
		t.Fatal("应包含 tool_call 类型候选")
	}
}

func TestCandidateRegistryRegisterAndGet(t *testing.T) {
	r := NewCandidateRegistry()
	def := CandidateActionDef{
		ID:        "custom_action",
		Type:      CandidateActionDefault,
		Label:     "自定义",
		BaseScore: 0.42,
	}
	if err := r.Register(def); err != nil {
		t.Fatal(err)
	}
	got, ok := r.Get("custom_action")
	if !ok {
		t.Fatal("自定义候选应可获取")
	}
	if got.BaseScore != 0.42 {
		t.Fatalf("BaseScore 不匹配: %f", got.BaseScore)
	}
}

func TestCandidateRegistryByType(t *testing.T) {
	r := NewCandidateRegistry()
	if err := r.Register(CandidateActionDef{ID: "a1", Type: CandidateActionChat}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(CandidateActionDef{ID: "a2", Type: CandidateActionChat}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(CandidateActionDef{ID: "a3", Type: CandidateActionProactive}); err != nil {
		t.Fatal(err)
	}
	chat := r.ByType(CandidateActionChat)
	if len(chat) != 2 {
		t.Fatalf("chat 类型应有 2 个, 实际 %d", len(chat))
	}
	proactive := r.ByType(CandidateActionProactive)
	if len(proactive) != 1 {
		t.Fatalf("proactive 类型应有 1 个, 实际 %d", len(proactive))
	}
	empty := r.ByType("nonexistent")
	if len(empty) != 0 {
		t.Fatalf("不存在的类型应返回空列表, 实际 %d", len(empty))
	}
}

func TestCandidateRegistryAllExcept(t *testing.T) {
	r := DefaultCandidateRegistry()
	all := r.All()
	totalBefore := len(all)
	excluded := r.AllExcept([]string{"chat_reply", "ask_clarify"})
	if len(excluded) != totalBefore-2 {
		t.Fatalf("排除 2 个后应有 %d, 实际 %d", totalBefore-2, len(excluded))
	}
	for _, def := range excluded {
		if def.ID == "chat_reply" || def.ID == "ask_clarify" {
			t.Fatalf("被排除的候选不应出现: %s", def.ID)
		}
	}
}

func TestCandidateRegistryStableOrdering(t *testing.T) {
	r := NewCandidateRegistry()
	if err := r.Register(CandidateActionDef{ID: "z_action", Type: CandidateActionChat}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(CandidateActionDef{ID: "a_action", Type: CandidateActionChat}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(CandidateActionDef{ID: "m_action", Type: CandidateActionChat}); err != nil {
		t.Fatal(err)
	}
	all := r.All()
	for i := 1; i < len(all); i++ {
		if all[i].ID < all[i-1].ID {
			t.Fatalf("Registry 应稳定按 ID ASC 排序, 位置 %d: %s 在 %s 之前", i, all[i].ID, all[i-1].ID)
		}
	}
}

func TestCandidateRegistryDefensiveCopy(t *testing.T) {
	r := NewCandidateRegistry()
	def := CandidateActionDef{
		ID:       "defensive_test",
		Type:     CandidateActionChat,
		Preconds: []string{"boundary_crossed"},
	}
	if err := r.Register(def); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Get("defensive_test")
	if len(got.Preconds) != 1 {
		t.Fatal("Preconds 应有 1 个")
	}
	got.Preconds[0] = "corrupted"
	got2, _ := r.Get("defensive_test")
	if got2.Preconds[0] != "boundary_crossed" {
		t.Fatalf("Registry 内部数据被污染, 期望 boundary_crossed, 实际 %s", got2.Preconds[0])
	}
}

func TestCandidateRegistryRejectsEmptyID(t *testing.T) {
	r := NewCandidateRegistry()
	err := r.Register(CandidateActionDef{Type: CandidateActionChat})
	if err == nil {
		t.Fatal("空 ID 应被拒绝")
	}
}

func TestCandidateRegistryRejectsEmptyType(t *testing.T) {
	r := NewCandidateRegistry()
	err := r.Register(CandidateActionDef{ID: "no_type"})
	if err == nil {
		t.Fatal("空 Type 应被拒绝")
	}
}

func TestCandidateRegistryRejectsDuplicate(t *testing.T) {
	r := NewCandidateRegistry()
	if err := r.Register(CandidateActionDef{ID: "dup_action", Type: CandidateActionChat}); err != nil {
		t.Fatal(err)
	}
	err := r.Register(CandidateActionDef{ID: "dup_action", Type: CandidateActionChat})
	if err == nil {
		t.Fatal("重复注册应返回 error")
	}
	if r.Len() != 1 {
		t.Fatalf("重复注册不应增加数量, 实际 %d", r.Len())
	}
}

func TestCandidateRegistryLen(t *testing.T) {
	r := NewCandidateRegistry()
	if r.Len() != 0 {
		t.Fatal("新 Registry 长度应为 0")
	}
	_ = r.Register(CandidateActionDef{ID: "g1", Type: CandidateActionChat})
	_ = r.Register(CandidateActionDef{ID: "g2", Type: CandidateActionChat})
	if r.Len() != 2 {
		t.Fatalf("长度应为 2, 实际 %d", r.Len())
	}
}
