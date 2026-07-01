package decision

import "testing"

func TestSafetyValidateInputTextBlocked(t *testing.T) {
	gov := DefaultSafetyGovernor()
	result := gov.ValidateInputText("I want to self-harm")
	if result.Passed {
		t.Fatal("自伤内容应被阻止")
	}
	if !result.Blocked {
		t.Fatal("应标记为 Blocked")
	}
}

func TestSafetyValidateInputTextClean(t *testing.T) {
	gov := DefaultSafetyGovernor()
	result := gov.ValidateInputText("今天天气真好")
	if !result.Passed {
		t.Fatalf("干净文本应通过, 原因: %s", result.Reason)
	}
	if result.Blocked {
		t.Fatal("不应被阻止")
	}
}

func TestSafetyFilterOutputBlocks(t *testing.T) {
	gov := DefaultSafetyGovernor()
	filtered := gov.FilterOutput("some hate speech content")
	if filtered != "" {
		t.Fatalf("应返回空字符串, 实际 %s", filtered)
	}
}

func TestSafetyFilterOutputAllows(t *testing.T) {
	gov := DefaultSafetyGovernor()
	filtered := gov.FilterOutput("今天天气真好")
	if filtered == "" {
		t.Fatal("干净文本不应被过滤")
	}
}

func TestSafetyValidateOutputExpressionHighIntensity(t *testing.T) {
	gov := DefaultSafetyGovernor()
	result := gov.ValidateOutputExpression(0.95, "chat_reply")
	if result.Passed {
		t.Fatal("情绪强度超出阈值应被阻止")
	}
}

func TestSafetyValidateOutputExpressionNormal(t *testing.T) {
	gov := DefaultSafetyGovernor()
	result := gov.ValidateOutputExpression(0.3, "chat_reply")
	if !result.Passed {
		t.Fatalf("正常情绪应通过, 原因: %s", result.Reason)
	}
}

func TestSafetyIsExpressionSafeBlockedLevel(t *testing.T) {
	gov := DefaultSafetyGovernor()
	result := gov.IsExpressionSafe(0.3, 0.2, BehaviorSafetyLevelBlocked)
	if result.Passed {
		t.Fatal("Blocked 级别应被拒绝")
	}
}

func TestSafetyIsExpressionSafeHighRisk(t *testing.T) {
	gov := DefaultSafetyGovernor()
	result := gov.IsExpressionSafe(0.3, 0.95, BehaviorSafetyLevelNormal)
	if result.Passed {
		t.Fatal("风险过高应被阻止")
	}
}
