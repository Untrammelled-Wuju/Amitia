package personality

import "testing"

func TestCompilerCompileBasic(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	cp := c.Compile("char-1", map[string]interface{}{
		"identity": "测试角色",
	})
	if cp.CharacterID != "char-1" {
		t.Errorf("expected char-1, got %s", cp.CharacterID)
	}
	if cp.CognitiveSens != 0.5 {
		t.Errorf("expected default sensitivity 0.5, got %f", cp.CognitiveSens)
	}
}

func TestCompilerCompileFullConfig(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	cp := c.Compile("char-2", map[string]interface{}{
		"identity":             "测试角色",
		"coreBoundary":         "不讨论政治",
		"cognitiveSensitivity": 0.8,
		"warmth":               0.7,
		"directness":           0.3,
	})
	if cp.CognitiveSens != 0.8 {
		t.Errorf("expected 0.8, got %f", cp.CognitiveSens)
	}
	if cp.BehaviorBias["warmth"] != 0.7 {
		t.Errorf("expected warmth 0.7, got %f", cp.BehaviorBias["warmth"])
	}
	if cp.ImmutableCore["coreBoundary"] != "不讨论政治" {
		t.Errorf("expected coreBoundary '不讨论政治', got %s", cp.ImmutableCore["coreBoundary"])
	}
}

func TestCompilerCompileEmptyConfig(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	cp := c.Compile("char-3", map[string]interface{}{})
	if len(cp.Diagnostics) == 0 {
		t.Error("expected diagnostics for empty config")
	}
}

func TestCompilerSliderNormalization_0to100(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	cp80 := c.Compile("char-80", map[string]interface{}{
		"warmth":     float64(80),
		"directness": float64(10),
	})
	cp90 := c.Compile("char-90", map[string]interface{}{
		"warmth":     float64(90),
		"directness": float64(10),
	})

	warmth80 := cp80.BehaviorBias["warmth"]
	warmth90 := cp90.BehaviorBias["warmth"]
	if warmth80 >= warmth90 {
		t.Fatalf("warmth 80(%.4f) should be < warmth 90(%.4f)", warmth80, warmth90)
	}
	if warmth80 < 0.79 || warmth80 > 0.81 {
		t.Fatalf("warmth 80 expected ~0.8, got %.4f", warmth80)
	}
	if warmth90 < 0.89 || warmth90 > 0.91 {
		t.Fatalf("warmth 90 expected ~0.9, got %.4f", warmth90)
	}

	direct10 := cp80.BehaviorBias["directness"]
	if direct10 < 0.09 || direct10 > 0.11 {
		t.Fatalf("directness 10 expected ~0.1, got %.4f", direct10)
	}

	cp01 := c.Compile("char-01", map[string]interface{}{
		"warmth": float64(0.7),
	})
	if cp01.BehaviorBias["warmth"] < 0.69 || cp01.BehaviorBias["warmth"] > 0.71 {
		t.Fatalf("warmth 0.7 should stay ~0.7, got %.4f", cp01.BehaviorBias["warmth"])
	}
}

func TestCompilerSliderClampEdgeValues(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())

	cpMinus := c.Compile("char-minus", map[string]interface{}{
		"warmth": float64(-10),
	})
	if cpMinus.BehaviorBias["warmth"] != 0 {
		t.Fatalf("warmth -10 should clamp to 0, got %.4f", cpMinus.BehaviorBias["warmth"])
	}

	cpOver := c.Compile("char-over", map[string]interface{}{
		"warmth": float64(150),
	})
	if cpOver.BehaviorBias["warmth"] != 1.0 {
		t.Fatalf("warmth 150 should normalize to 1.0, got %.4f", cpOver.BehaviorBias["warmth"])
	}
}

func TestCompilerAllFrontendFieldsPresent(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	raw := map[string]interface{}{
		"familiarity":            float64(78),
		"formality":              float64(22),
		"customerServiceAvoidance": float64(92),
		"directness":             float64(75),
		"verbosity":              float64(32),
		"structureLevel":         float64(40),
		"shortSentence":          float64(85),
		"toneWords":              float64(45),
		"warmth":                 float64(58),
		"emotionalExpression":    float64(45),
		"comfortLevel":           float64(55),
		"preachingAvoidance":     float64(88),
		"rationality":            float64(62),
		"humor":                  float64(35),
		"teasing":                float64(30),
		"initiative":             float64(50),
		"patience":               float64(60),
		"companionship":          float64(55),
		"boundary":               float64(85),
		"dependencyAvoidance":    float64(85),
		"execution":              float64(75),
		"explanationDepth":       float64(55),
		"judgment":               float64(75),
		"clarification":          float64(35),
		"intimacyExpression":     float64(25),
		"flirtiness":             float64(0),
		"romanticTone":           float64(0),
		"suggestivenessAvoidance": float64(100),
		"intimacyBoundary":       float64(90),
		"identity":               "测试角色",
		"coreBoundary":           "边界测试",
		"personality":            "性格描述测试",
	}
	cp := c.Compile("char-all", raw)

	behaviorFields := []string{
		"familiarity", "customerServiceAvoidance",
		"directness", "structureLevel",
		"warmth", "emotionalExpression", "comfortLevel", "preachingAvoidance",
		"rationality", "humor", "teasing", "initiative", "patience",
		"companionship", "boundary", "dependencyAvoidance",
		"execution", "judgment", "clarification",
		"intimacyExpression", "flirtiness", "romanticTone", "suggestivenessAvoidance", "intimacyBoundary",
		"affection", "conflictAvoidance", "explanationDepth",
	}
	for _, f := range behaviorFields {
		v, ok := cp.BehaviorBias[f]
		if !ok {
			t.Errorf("BehaviorBias missing field: %s", f)
		}
		if v < 0 || v > 1 {
			t.Errorf("BehaviorBias[%s] out of range [0,1]: %.4f", f, v)
		}
	}

	exprFields := []string{"verbosity", "formality", "emotionalExpression", "emotionality", "shortSentence", "toneWords"}
	for _, f := range exprFields {
		v, ok := cp.ExpressionStyle[f]
		if !ok {
			t.Errorf("ExpressionStyle missing field: %s", f)
		}
		if v < 0 || v > 1 {
			t.Errorf("ExpressionStyle[%s] out of range [0,1]: %.4f", f, v)
		}
	}

	if cp.ImmutableCore["identity"] != "测试角色" {
		t.Errorf("identity mismatch: %s", cp.ImmutableCore["identity"])
	}
	if cp.ImmutableCore["coreBoundary"] != "边界测试" {
		t.Errorf("coreBoundary mismatch: %s", cp.ImmutableCore["coreBoundary"])
	}
	if cp.ImmutableCore["personalityDesc"] != "性格描述测试" {
		t.Errorf("personalityDesc mismatch: %s", cp.ImmutableCore["personalityDesc"])
	}
}

func TestCompilerEmotionalExpressionMapped(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	raw := map[string]interface{}{
		"emotionalExpression": float64(80),
	}
	cp := c.Compile("char-emo", raw)
	if cp.BehaviorBias["emotionalExpression"] < 0.79 || cp.BehaviorBias["emotionalExpression"] > 0.81 {
		t.Errorf("emotionalExpression 80 not normalized correctly: %.4f", cp.BehaviorBias["emotionalExpression"])
	}
	if cp.ExpressionStyle["emotionalExpression"] < 0.79 || cp.ExpressionStyle["emotionalExpression"] > 0.81 {
		t.Errorf("ExpressionStyle emotionalExpression 80 not normalized correctly: %.4f", cp.ExpressionStyle["emotionalExpression"])
	}
}

func TestCompilerFrontendDefaultConfigRoundTrip(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	raw := map[string]interface{}{
		"familiarity":            float64(78),
		"formality":              float64(22),
		"customerServiceAvoidance": float64(92),
		"directness":             float64(75),
		"verbosity":              float64(32),
		"structureLevel":         float64(40),
		"shortSentence":          float64(85),
		"toneWords":              float64(45),
		"warmth":                 float64(58),
		"emotionalExpression":    float64(45),
		"comfortLevel":           float64(55),
		"preachingAvoidance":     float64(88),
		"rationality":            float64(62),
		"humor":                  float64(35),
		"teasing":                float64(30),
		"initiative":             float64(50),
		"patience":               float64(60),
		"companionship":          float64(55),
		"boundary":               float64(85),
		"dependencyAvoidance":    float64(85),
		"execution":              float64(75),
		"explanationDepth":       float64(55),
		"judgment":               float64(75),
		"clarification":          float64(35),
		"intimacyExpression":     float64(25),
		"flirtiness":             float64(0),
		"romanticTone":           float64(0),
		"suggestivenessAvoidance": float64(100),
		"intimacyBoundary":       float64(90),
	}
	cp := c.Compile("char-default", raw)

	tests := []struct {
		name     string
		key      string
		source   float64
		expected float64
	}{
		{"familiarity_bias", "familiarity", 78, 0.78},
		{"warmth_bias", "warmth", 58, 0.58},
		{"initiative_bias", "initiative", 50, 0.50},
		{"intimacyBoundary_bias", "intimacyBoundary", 90, 0.90},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cp.BehaviorBias[tc.key]
			if tc.key == "formality" || tc.key == "verbosity" || tc.key == "emotionalExpression" {
				got = cp.ExpressionStyle[tc.key]
			}
			if got < tc.expected-0.01 || got > tc.expected+0.01 {
				t.Errorf("%s: expected ~%.2f, got %.4f", tc.name, tc.expected, got)
			}
		})
	}
}

