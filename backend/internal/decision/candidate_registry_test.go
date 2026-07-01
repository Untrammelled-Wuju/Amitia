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
	r.Register(def)
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
	r.Register(CandidateActionDef{ID: "a1", Type: CandidateActionChat})
	r.Register(CandidateActionDef{ID: "a2", Type: CandidateActionChat})
	r.Register(CandidateActionDef{ID: "a3", Type: CandidateActionProactive})
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
