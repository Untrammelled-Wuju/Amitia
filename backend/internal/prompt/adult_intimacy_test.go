package prompt

import "testing"

func TestIsHardStop(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"停", true},
		{"不要了", true},
		{"今天太累了", true},
		{"stop", true},
		{"no more", true},
		{"求你了停下", true},
		{"你好", false},
		{"今天天气不错", false},
		{"我想你", false},
	}
	for _, tt := range tests {
		got := IsHardStop(tt.input)
		if got != tt.want {
			t.Errorf("IsHardStop(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsAdultRejection(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"不要", true},
		{"别这样", true},
		{"not now", true},
		{"有点不舒服", true},
		{"太快了", true},
		{"你好", false},
		{"嗯", false},
	}
	for _, tt := range tests {
		got := IsAdultRejection(tt.input)
		if got != tt.want {
			t.Errorf("IsAdultRejection(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsBlockedEmotion(t *testing.T) {
	if !IsBlockedEmotion("HURT_GRIEVANCE") {
		t.Error("HURT_GRIEVANCE should be blocked")
	}
	if !IsBlockedEmotion("ANGRY_ATTACK") {
		t.Error("ANGRY_ATTACK should be blocked")
	}
	if !IsBlockedEmotion("COLD_DETACHED") {
		t.Error("COLD_DETACHED should be blocked")
	}
	if !IsBlockedEmotion("FEARFUL_OBEDIENT") {
		t.Error("FEARFUL_OBEDIENT should be blocked")
	}
	if IsBlockedEmotion("SWEET_ATTACHMENT") {
		t.Error("SWEET_ATTACHMENT should not be blocked")
	}
	if IsBlockedEmotion("CALM_RATIONAL") {
		t.Error("CALM_RATIONAL should not be blocked")
	}
}

func TestIntimacyGate(t *testing.T) {
	if gate := IntimacyGate("停", "CALM_RATIONAL"); gate != "hard_stop" {
		t.Errorf("hard_stop gate expected, got %q", gate)
	}
	if gate := IntimacyGate("你好", "HURT_GRIEVANCE"); gate != "blocked_emotion" {
		t.Errorf("blocked_emotion gate expected, got %q", gate)
	}
	if gate := IntimacyGate("不要", "CALM_RATIONAL"); gate != "rejection" {
		t.Errorf("rejection gate expected, got %q", gate)
	}
	if gate := IntimacyGate("你好", "SWEET_ATTACHMENT"); gate != "" {
		t.Errorf("empty gate expected, got %q", gate)
	}
}

func TestBuildAdultIntimacyDefaultSection(t *testing.T) {
	result := BuildAdultIntimacyDefaultSection("tsundere")
	if result == "" {
		t.Fatal("expected non-empty section")
	}
	if contains(result, "成人模式已开启") {
		t.Error("section should not contain '成人模式已开启'")
	}
	if !contains(result, "成年伴侣亲密表达能力") {
		t.Error("section should contain '成年伴侣亲密表达能力'")
	}
	if !contains(result, "安全门禁") {
		t.Error("section should contain '安全门禁'")
	}
	if !contains(result, "傲娇在亲密时") {
		t.Error("section should contain tsundere expression text")
	}
}

func TestBuildIntimacyDowngradeSection(t *testing.T) {
	result := BuildIntimacyDowngradeSection()
	if result == "" {
		t.Fatal("expected non-empty downgrade section")
	}
	if !contains(result, "正常陪伴") {
		t.Error("downgrade section should mention 正常陪伴")
	}
}

func TestGetAdultExpression(t *testing.T) {
	result := getAdultExpression("nonexistent")
	if result == "" {
		t.Error("expected fallback text for unknown personality")
	}
	result = getAdultExpression("tsundere")
	if !contains(result, "傲娇") {
		t.Error("expected tsundere expression text")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
