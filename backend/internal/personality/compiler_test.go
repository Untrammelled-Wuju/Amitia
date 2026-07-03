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
