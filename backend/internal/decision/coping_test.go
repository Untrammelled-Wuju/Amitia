package decision

import "testing"

func TestSelectCopingStrategyHighStressWithSupport(t *testing.T) {
	input := CopingInput{
		Stress:           0.9,
		AvailableSupport: 0.7,
	}
	result := SelectCopingStrategy(input)
	if result.PrimaryStrategy != CopingSeekSupport {
		t.Fatalf("高压有支持应选择寻求支持, 实际 %s", result.PrimaryStrategy)
	}
	if result.Confidence < 0.5 {
		t.Fatalf("置信度应较高, 实际 %f", result.Confidence)
	}
}

func TestSelectCopingStrategyHighStressNoSupport(t *testing.T) {
	input := CopingInput{
		Stress:           0.9,
		AvailableSupport: 0.1,
	}
	result := SelectCopingStrategy(input)
	if result.PrimaryStrategy != CopingAcceptance {
		t.Fatalf("高压无支持应选择接受, 实际 %s", result.PrimaryStrategy)
	}
}

func TestSelectCopingStrategyHighNegativeHighLoad(t *testing.T) {
	input := CopingInput{
		NegativeEmotion: 0.8,
		CognitiveLoad:   0.8,
	}
	result := SelectCopingStrategy(input)
	if result.PrimaryStrategy != CopingDistraction {
		t.Fatalf("高负情绪高负荷应选择分散注意力, 实际 %s", result.PrimaryStrategy)
	}
}

func TestSelectCopingStrategyHighNegativeAvailableCapacity(t *testing.T) {
	input := CopingInput{
		NegativeEmotion: 0.8,
		CognitiveLoad:   0.3,
	}
	result := SelectCopingStrategy(input)
	if result.PrimaryStrategy != CopingReappraisal {
		t.Fatalf("高负情绪有认知余量应选择重新评价, 实际 %s", result.PrimaryStrategy)
	}
}

func TestSelectCopingStrategyPositiveEngaged(t *testing.T) {
	input := CopingInput{
		PositiveEmotion: 0.7,
	}
	result := SelectCopingStrategy(input)
	if result.PrimaryStrategy != CopingProblemSolving {
		t.Fatalf("正情绪应选择问题解决, 实际 %s", result.PrimaryStrategy)
	}
}

func TestSelectCopingStrategyDefault(t *testing.T) {
	input := CopingInput{}
	result := SelectCopingStrategy(input)
	if result.PrimaryStrategy == "" {
		t.Fatal("应有默认策略")
	}
	if result.Confidence <= 0 {
		t.Fatal("置信度应大于 0")
	}
}

func TestCopingStrategyBoostReappraisal(t *testing.T) {
	boost := CopingStrategyBoost(CopingReappraisal, "chat_reply")
	if boost != 0.15 {
		t.Fatalf("重新评价对 chat_reply 应有 0.15 提升, 实际 %f", boost)
	}
}

func TestCopingStrategyBoostSuppression(t *testing.T) {
	boost := CopingStrategyBoost(CopingSuppression, "wait_observe")
	if boost != 0.20 {
		t.Fatalf("抑制对 wait_observe 应有 0.20 提升, 实际 %f", boost)
	}
}
