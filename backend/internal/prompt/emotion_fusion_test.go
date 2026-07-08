package prompt

import (
	"strings"
	"testing"
)

func TestBuildEmotionFusionRawSection_HappyPath(t *testing.T) {
	input := EmotionFusionInput{
		PrimaryLabel:     "SWEET_ATTACHMENT",
		Aff:              0.8,
		Sec:              0.6,
		Aro:              0.7,
		Dom:              0.3,
		PersonalityLabel: "Amitia",
		CoreConflict:     "嘴硬但心软",
		Catchphrases:     []string{"嗯", "哼"},
		SpeakingStyle:    "自然口语化",
	}
	result := BuildEmotionFusionRawSection(input)
	if result == "" {
		t.Fatal("emotion fusion section should not be empty")
	}
	if !strings.Contains(result, "行为优先级") {
		t.Error("should contain priority section")
	}
	if !strings.Contains(result, "人格基底") {
		t.Error("should contain personality section")
	}
	if !strings.Contains(result, "动态情绪") {
		t.Error("should contain emotion section")
	}
	if !strings.Contains(result, "甜蜜依恋") {
		t.Error("should contain emotion label")
	}
	if !strings.Contains(result, "融合执行策略") {
		t.Error("should contain fusion strategy")
	}
	if !strings.Contains(result, "绝对禁止清单") {
		t.Error("should contain prohibition section")
	}
}

func TestBuildEmotionFusionRawSection_DifferentLabelsProduceDifferentOutput(t *testing.T) {
	base := EmotionFusionInput{
		Aff:              0.5,
		Sec:              0.5,
		Aro:              0.5,
		Dom:              0.0,
		PersonalityLabel: "Amitia",
		CoreConflict:     "嘴硬但心软",
		Catchphrases:     []string{"嗯"},
		SpeakingStyle:    "自然口语化",
	}

	sweet := base
	sweet.PrimaryLabel = "SWEET_ATTACHMENT"
	sweetResult := BuildEmotionFusionRawSection(sweet)

	angry := base
	angry.PrimaryLabel = "ANGRY_ATTACK"
	angryResult := BuildEmotionFusionRawSection(angry)

	if sweetResult == angryResult {
		t.Error("different emotion labels should produce different output")
	}
	if !strings.Contains(sweetResult, "甜蜜依恋") {
		t.Error("sweet attachment should contain its label")
	}
	if !strings.Contains(angryResult, "愤怒反击") {
		t.Error("angry attack should contain its label")
	}
}

func TestBuildEmotionFusionRawSection_EmptyLabelDefaultsToCalm(t *testing.T) {
	input := EmotionFusionInput{
		PrimaryLabel:     "",
		Aff:              0.5,
		Sec:              0.5,
		Aro:              0.5,
		Dom:              0.0,
		PersonalityLabel: "Amitia",
	}
	result := BuildEmotionFusionRawSection(input)
	if !strings.Contains(result, "平静理性") {
		t.Error("empty label should default to CALM_RATIONAL")
	}
}

func TestEmotionProhibitionsDifferByLabel(t *testing.T) {
	sweet := getEmotionProhibitionsCN("SWEET_ATTACHMENT")
	angry := getEmotionProhibitionsCN("ANGRY_ATTACK")
	cold := getEmotionProhibitionsCN("COLD_DETACHED")

	if len(sweet) == 0 || len(angry) == 0 || len(cold) == 0 {
		t.Fatal("prohibitions should not be empty")
	}
	if strings.Join(sweet, "|") == strings.Join(angry, "|") {
		t.Error("sweet and angry prohibitions should differ")
	}
}

func TestMergeProhibitionsCN_RespectsPersonalityFirst(t *testing.T) {
	personality := []string{"人格禁止项A", "人格禁止项B"}
	emotion := []string{"情绪禁止项X"}
	merged := mergeProhibitionsCN(personality, emotion, false)
	if len(merged) < 3 {
		t.Error("merged should contain both personality and emotion prohibitions")
	}
	foundA := false
	for _, p := range merged {
		if p == "人格禁止项A" {
			foundA = true
		}
	}
	if !foundA {
		t.Error("personality prohibition should be preserved in merge")
	}
}

func TestMergeProhibitionsCN_ApologyRemovesRestrictiveProhibitions(t *testing.T) {
	personality := []string{"禁止道歉", "禁止示弱", "正常禁止项"}
	emotion := []string{"委婉道歉"}
	merged := mergeProhibitionsCN(personality, emotion, true)
	for _, p := range merged {
		if strings.Contains(p, "道歉") || strings.Contains(p, "示弱") {
			t.Errorf("apology mode should remove restrictive prohibitions, but found: %s", p)
		}
	}
}

func TestToDisplay100_RangeConversion(t *testing.T) {
	if v := toDisplay100(-1.0); v != 0 {
		t.Errorf("expected 0, got %d", v)
	}
	if v := toDisplay100(0.0); v != 50 {
		t.Errorf("expected 50, got %d", v)
	}
	if v := toDisplay100(1.0); v != 100 {
		t.Errorf("expected 100, got %d", v)
	}
}

func TestLabelZH_AllLabelsHaveMapping(t *testing.T) {
	labels := []string{
		"SWEET_ATTACHMENT", "SHY_HEARTBEAT", "TSUNDERE",
		"HURT_GRIEVANCE", "ANGRY_ATTACK", "COLD_DETACHED",
		"FEARFUL_OBEDIENT", "QUIET_FOND", "CALM_RATIONAL",
	}
	for _, l := range labels {
		zh := labelZH(l)
		if zh == l {
			t.Errorf("label %s should have a Chinese mapping", l)
		}
	}
}

func TestBuildEmotionFusionRawSection_EmptyCoreConflictSkipsDefault(t *testing.T) {
	input := EmotionFusionInput{
		PrimaryLabel:     "SWEET_ATTACHMENT",
		Aff:              0.8,
		Sec:              0.6,
		Aro:              0.7,
		Dom:              0.3,
		PersonalityLabel: "Tsundere",
		CoreConflict:     "",
		Catchphrases:     []string{"哼", "笨蛋"},
		SpeakingStyle:    "傲娇自然口语",
	}
	result := BuildEmotionFusionRawSection(input)
	if strings.Contains(result, "嘴硬但心软") {
		t.Error("empty core conflict should not produce default '嘴硬但心软'")
	}
	if !strings.Contains(result, "哼") || !strings.Contains(result, "笨蛋") {
		t.Error("should contain actual catchphrases when provided")
	}
	if !strings.Contains(result, "融合执行策略") {
		t.Error("should still contain fusion strategy section")
	}
}

func TestBuildEmotionFusionRawSection_EmptyCatchphrasesSkipsDefault(t *testing.T) {
	input := EmotionFusionInput{
		PrimaryLabel:     "CALM_RATIONAL",
		Aff:              0.0,
		Sec:              0.0,
		Aro:              0.0,
		Dom:              0.5,
		PersonalityLabel: "Professional",
		CoreConflict:     "",
		Catchphrases:     nil,
		SpeakingStyle:    "专业简洁",
	}
	result := BuildEmotionFusionRawSection(input)
	if strings.Contains(result, "嗯") || strings.Contains(result, "哼") || strings.Contains(result, "切") {
		t.Error("empty catchphrases should not produce default '嗯/哼/切'")
	}
	if !strings.Contains(result, "人格基底") {
		t.Error("should contain personality section")
	}
}

func TestBuildEmotionFusionRawSection_NoCoreConflictNoPersonalitySkipLine(t *testing.T) {
	input := EmotionFusionInput{
		PrimaryLabel:     "QUIET_FOND",
		Aff:              0.5,
		Sec:              0.4,
		Aro:              0.2,
		Dom:              0.1,
		PersonalityLabel: "Quiet",
		CoreConflict:     "",
		Catchphrases:     nil,
		SpeakingStyle:    "",
	}
	result := BuildEmotionFusionRawSection(input)
	if strings.Contains(result, "嘴硬但心软") {
		t.Error("empty coreConflict should skip core constraint, not default to '嘴硬但心软'")
	}
	if !strings.Contains(result, "Quiet") {
		t.Error("should contain personality label even without coreConflict/catchphrases/speakingStyle")
	}
}
