package decision

import "testing"

func TestControlExpressionFull(t *testing.T) {
	input := ExpressionControlInput{
		EmotionIntensity:   0.3,
		RiskScore:          0.1,
		RelationshipSafety: 0.8,
		StressLevel:        0.2,
	}
	config := DefaultExpressionControlConfig()
	result := ControlExpression(input, config)
	if result.Intensity != ExpressionFull {
		t.Fatalf("安全场景应完全表达, 实际 %s", result.Intensity)
	}
	if result.Suppressed {
		t.Fatal("不应被抑制")
	}
}

func TestControlExpressionSuppressedHighRisk(t *testing.T) {
	input := ExpressionControlInput{
		EmotionIntensity:   0.9,
		RiskScore:          0.9,
		RelationshipSafety: 0.2,
		StressLevel:        0.3,
	}
	config := DefaultExpressionControlConfig()
	result := ControlExpression(input, config)
	if result.Intensity != ExpressionSuppressed {
		t.Fatalf("高风险低安全应被抑制, 实际 %s", result.Intensity)
	}
	if !result.Suppressed {
		t.Fatal("应被抑制")
	}
}

func TestControlExpressionModerated(t *testing.T) {
	input := ExpressionControlInput{
		EmotionIntensity:   0.5,
		RiskScore:          0.7,
		RelationshipSafety: 0.6,
		StressLevel:        0.4,
	}
	config := DefaultExpressionControlConfig()
	result := ControlExpression(input, config)
	if result.Intensity != ExpressionModerated {
		t.Fatalf("中等风险应被调节, 实际 %s", result.Intensity)
	}
}

func TestIsExpressionSuppressed(t *testing.T) {
	input := ExpressionControlInput{
		EmotionIntensity:   0.95,
		RiskScore:          0.5,
		RelationshipSafety: 0.5,
	}
	config := DefaultExpressionControlConfig()
	if !IsExpressionSuppressed(input, config) {
		t.Fatal("高情绪强度应被抑制")
	}
}

func TestClampEmotionIntensity(t *testing.T) {
	clamped := ClampEmotionIntensity(0.95, 0.75)
	if clamped != 0.75 {
		t.Fatalf("应被限制在 0.75, 实际 %f", clamped)
	}
	normal := ClampEmotionIntensity(0.5, 0.75)
	if normal != 0.5 {
		t.Fatalf("正常值不应被修改, 实际 %f", normal)
	}
}

func TestComputeExpressionScaleFactor(t *testing.T) {
	input := ExpressionControlInput{
		EmotionIntensity:   0.9,
		RiskScore:          0.2,
		RelationshipSafety: 0.7,
		StressLevel:        0.8,
	}
	config := DefaultExpressionControlConfig()
	factor := ComputeExpressionScaleFactor(input, config)
	if factor <= 0 {
		t.Fatalf("缩放因子应大于 0, 实际 %f", factor)
	}
	if factor > 1.0 {
		t.Fatalf("缩放因子不应超过 1.0, 实际 %f", factor)
	}
}
