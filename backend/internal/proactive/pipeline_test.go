package proactive

import (
	"testing"
	"time"
)

func newAllowInput() PipelineInput {
	return PipelineInput{
		CharacterID:               "char-1",
		ConversationID:            "conv-1",
		Channel:                   "web",
		Content:                   "hello from pipeline",
		RuleID:                    "rule-1",
		Priority:                  PriorityNormal,
		LastUserReplyAt:           time.Now().Add(-12 * time.Hour),
		UserActiveNow:             true,
		IntimacyScore:             0.8,
		PendingItems:              3,
		InitiativeScore:           0.7,
		QueueBackpressureLevel:    BackpressureNone,
		InterruptionRiskThreshold: 0.65,
	}
}

func TestPipelineRunAllowsWhenMotivationExceedsSuppression(t *testing.T) {
	PipelineReset()
	input := newAllowInput()

	result := RunPipeline(input, nil)

	if !result.Allowed {
		t.Fatalf("expected pipeline allowed, got reason=%s", result.Reason)
	}
	if result.Suppressed {
		t.Fatal("expected not suppressed")
	}
	if result.CorrelationID == "" {
		t.Fatal("expected correlation ID")
	}
	if result.LeaseID == "" {
		t.Fatal("expected lease ID")
	}
	if len(result.Records) == 0 {
		t.Fatal("expected at least one delivery record")
	}
	if !result.Delivered {
		t.Fatal("expected delivered true")
	}
	if result.Motivation <= result.Suppression {
		t.Fatalf("expected motivation (%d) > suppression (%d)", result.Motivation, result.Suppression)
	}
}

func TestPipelineRejectsWhenSuppressionExceedsMotivation(t *testing.T) {
	PipelineReset()
	input := PipelineInput{
		CharacterID:               "char-1",
		ConversationID:            "conv-1",
		Channel:                   "web",
		Content:                   "should be blocked",
		RuleID:                    "rule-1",
		Priority:                  PriorityNormal,
		LastUserReplyAt:           time.Now(),
		RecentSentCount:           10,
		UserActiveNow:             false,
		IntimacyScore:             0.1,
		InitiativeScore:           0.1,
		QueueBackpressureLevel:    BackpressureFull,
		InterruptionRiskThreshold: 0.65,
	}

	result := RunPipeline(input, nil)

	if result.Allowed {
		t.Fatalf("expected pipeline blocked, got reason=%s", result.Reason)
	}
	if !result.Suppressed {
		t.Fatal("expected suppressed true")
	}
}

func TestPipelineDedupPreventsDuplicate(t *testing.T) {
	PipelineReset()
	input := newAllowInput()

	first := RunPipeline(input, nil)
	if !first.Allowed {
		t.Fatalf("expected first send allowed, got reason=%s", first.Reason)
	}

	second := RunPipeline(input, nil)
	if second.Allowed {
		t.Fatalf("expected second send rejected by dedup, got reason=%s", second.Reason)
	}
}

func TestPipelineDedupAcrossChannels(t *testing.T) {
	PipelineReset()
	input := newAllowInput()

	first := RunPipeline(input, nil)
	if !first.Allowed {
		t.Fatalf("expected first send allowed, got reason=%s", first.Reason)
	}

	input.Channel = "wechat"
	second := RunPipeline(input, nil)
	if second.Allowed {
		t.Fatalf("expected cross-channel duplicate blocked, got reason=%s", second.Reason)
	}
}

func TestPipelineMultiChannelDelivery(t *testing.T) {
	PipelineReset()
	input := newAllowInput()
	input.Channel = "all"

	result := RunPipeline(input, nil)

	if !result.Allowed {
		t.Fatalf("expected pipeline allowed, got reason=%s", result.Reason)
	}

	hasWeb := false
	hasWechat := false
	hasQQ := false
	for _, ch := range result.DeliveredTo {
		if ch == "web" {
			hasWeb = true
		}
		if ch == "wechat" {
			hasWechat = true
		}
		if ch == "qq" {
			hasQQ = true
		}
	}
	if !hasWeb {
		t.Fatal("expected web channel delivered")
	}
	if !hasWechat {
		t.Fatal("expected wechat channel delivered")
	}
	if !hasQQ {
		t.Fatal("expected qq channel delivered")
	}
}

func TestPipelineSendFunctionCallback(t *testing.T) {
	PipelineReset()
	sentChannels := make(map[string]bool)

	sendFn := func(channel, content string) bool {
		sentChannels[channel] = true
		return true
	}

	input := newAllowInput()
	input.Channel = "web,wechat"

	result := RunPipeline(input, sendFn)

	if !result.Allowed {
		t.Fatalf("expected pipeline allowed, got reason=%s", result.Reason)
	}
	if !sentChannels["web"] {
		t.Fatal("expected sendFn called for web")
	}
	if !sentChannels["wechat"] {
		t.Fatal("expected sendFn called for wechat")
	}
	if sentChannels["qq"] {
		t.Fatal("expected sendFn not called for qq")
	}
}

func TestPipelineSendFunctionFailure(t *testing.T) {
	PipelineReset()
	failAllFn := func(channel, content string) bool {
		return false
	}

	input := newAllowInput()

	result := RunPipeline(input, failAllFn)

	if !result.Allowed {
		t.Fatalf("expected pipeline allowed (passed gates), got reason=%s", result.Reason)
	}
	if result.Delivered {
		t.Fatal("expected delivered false when all channels fail")
	}
	if result.Reason != "delivery_failed_all_channels" {
		t.Fatalf("expected failure reason, got %s", result.Reason)
	}
}

func TestRecordSendFeedback(t *testing.T) {
	PipelineReset()
	input := newAllowInput()

	result := RunPipeline(input, nil)
	if !result.Allowed {
		t.Fatalf("expected pipeline allowed, got reason=%s", result.Reason)
	}

	fb := RecordSendFeedback(FeedbackInput{
		CorrelationID:     result.CorrelationID,
		Channel:           "web",
		Status:            DeliveryStatusDelivered,
		CharacterID:       "char-1",
		UserReplied:       true,
		ReplyWithinWindow: true,
	})

	if !fb.Applied {
		t.Fatal("expected feedback applied")
	}
	if !fb.CooldownAdjusted {
		t.Fatal("expected cooldown adjusted on reply")
	}

	record := GlobalDedupManager.GetRecord(result.CorrelationID, "web")
	if record == nil {
		t.Fatal("expected record to exist")
	}
	if record.Status != DeliveryStatusDelivered {
		t.Fatalf("expected delivered status, got %s", record.Status)
	}
}

func TestRecordSendFeedbackRead(t *testing.T) {
	PipelineReset()
	input := newAllowInput()

	result := RunPipeline(input, nil)
	if !result.Allowed {
		t.Fatalf("expected pipeline allowed, got reason=%s", result.Reason)
	}

	RecordSendFeedback(FeedbackInput{
		CorrelationID:     result.CorrelationID,
		Channel:           "web",
		Status:            DeliveryStatusRead,
		UserReplied:       true,
		CharacterID:       "char-1",
		ReplyWithinWindow: true,
	})

	record := GlobalDedupManager.GetRecord(result.CorrelationID, "web")
	if record == nil {
		t.Fatal("expected record to exist after feedback")
	}
	if record.Status != DeliveryStatusRead {
		t.Fatalf("expected read status, got %s", record.Status)
	}
}

func TestApplyPsychologicalFeedbackUserReplied(t *testing.T) {
	CleanBudgets()
	bt := GetOrCreateBudget("char-psy-1", 10, 10*time.Minute, 30*time.Minute)
	bt.ConsecutiveUnanswered = 5

	fb := ApplyPsychologicalFeedback("char-psy-1", true)

	if !fb.Applied {
		t.Fatal("expected feedback applied")
	}
	if !fb.CooldownAdjusted {
		t.Fatal("expected cooldown adjusted")
	}
	if bt.ConsecutiveUnanswered != 0 {
		t.Fatalf("expected consecutive unanswered reset to 0, got %d", bt.ConsecutiveUnanswered)
	}
}

func TestApplyPsychologicalFeedbackNoReply(t *testing.T) {
	CleanBudgets()
	bt := GetOrCreateBudget("char-psy-2", 10, 10*time.Minute, 30*time.Minute)
	bt.ConsecutiveUnanswered = 2

	fb := ApplyPsychologicalFeedback("char-psy-2", false)

	if !fb.Applied {
		t.Fatal("expected feedback applied")
	}
	if bt.ConsecutiveUnanswered != 3 {
		t.Fatalf("expected consecutive unanswered incremented to 3, got %d", bt.ConsecutiveUnanswered)
	}
	if fb.NewUnanswered != 3 {
		t.Fatalf("expected NewUnanswered=3, got %d", fb.NewUnanswered)
	}
}

func TestApplyPsychologicalFeedbackEmptyChar(t *testing.T) {
	fb := ApplyPsychologicalFeedback("", true)
	if fb.Applied {
		t.Fatal("expected feedback not applied for empty character")
	}
}

func TestRecordSendFeedbackNonExistent(t *testing.T) {
	PipelineReset()
	fb := RecordSendFeedback(FeedbackInput{
		CorrelationID: "nonexistent",
		Channel:       "web",
		Status:        DeliveryStatusDelivered,
	})
	if fb.Applied {
		t.Fatal("expected feedback not applied for nonexistent record")
	}
}

func TestSuppressLowPriorityForUserInput(t *testing.T) {
	PipelineReset()
	GlobalLeaseManager.Reset()

	GlobalLeaseManager.AcquireLease(PriorityLow, "char-1", "conv-1", "web", "corr-low-1", 30*time.Second)
	GlobalLeaseManager.AcquireLease(PriorityNormal, "char-1", "conv-1", "web", "corr-normal-1", 30*time.Second)

	cancelled := SuppressLowPriorityForUserInput("char-1")
	if cancelled != 1 {
		t.Fatalf("expected 1 low-priority lease cancelled, got %d", cancelled)
	}

	active := GlobalLeaseManager.CountActive("char-1")
	if active != 1 {
		t.Fatalf("expected 1 active lease remaining, got %d", active)
	}
}

func TestResolveExpiredConfirmWindows(t *testing.T) {
	PipelineReset()
	result := ResolveExpiredConfirmWindows(time.Now())
	if result < 0 {
		t.Fatalf("expected non-negative resolved count, got %d", result)
	}
}

func TestPipelineResultDeliveredToChannels(t *testing.T) {
	PipelineReset()
	input := newAllowInput()
	input.Channel = "all"

	result := RunPipeline(input, nil)

	if len(result.DeliveredTo) < 3 {
		t.Fatalf("expected at least 3 delivered channels, got %d", len(result.DeliveredTo))
	}
}

func TestPipelineRecordsTrackAllChannels(t *testing.T) {
	PipelineReset()
	input := newAllowInput()
	input.Channel = "all"

	result := RunPipeline(input, nil)

	if len(result.Records) != len(result.DeliveredTo) {
		t.Fatalf("expected records count (%d) to match delivered count (%d)", len(result.Records), len(result.DeliveredTo))
	}

	for i, record := range result.Records {
		if record.Status != DeliveryStatusSent {
			t.Fatalf("expected record[%d] status sent, got %s", i, record.Status)
		}
		if record.CharacterID != "char-1" {
			t.Fatalf("expected record[%d] character char-1, got %s", i, record.CharacterID)
		}
	}
}

func TestPipelineInterruptionRisk(t *testing.T) {
	PipelineReset()
	input := newAllowInput()
	input.InterruptionRiskThreshold = 1.0

	result := RunPipeline(input, nil)
	if result.Interruption.Score < 0 {
		t.Fatalf("expected non-negative interruption score")
	}
	if result.Interruption.HardBlock {
		t.Fatalf("expected no hard block, got reasons=%v", result.Interruption.Reasons)
	}
}

func TestPipelineGetOrCreateBudget(t *testing.T) {
	CleanBudgets()
	bt1 := GetOrCreateBudget("char-budget-1", 10, 10*time.Minute, 30*time.Minute)
	if bt1 == nil {
		t.Fatal("expected non-nil budget")
	}
	bt2 := GetOrCreateBudget("char-budget-1", 5, 5*time.Minute, 15*time.Minute)
	if bt2 != bt1 {
		t.Fatal("expected same budget instance for same character")
	}
	bt3 := GetOrCreateBudget("char-budget-2", 20, 20*time.Minute, 60*time.Minute)
	if bt3 == bt1 {
		t.Fatal("expected different budget instances for different characters")
	}
}

func TestPipelineCleanBudgets(t *testing.T) {
	CleanBudgets()
	GetOrCreateBudget("char-clean-1", 10, 10*time.Minute, 30*time.Minute)
	GetOrCreateBudget("char-clean-2", 10, 10*time.Minute, 30*time.Minute)
	cleaned := CleanBudgets()
	if cleaned != 2 {
		t.Fatalf("expected 2 cleaned budgets, got %d", cleaned)
	}
}
