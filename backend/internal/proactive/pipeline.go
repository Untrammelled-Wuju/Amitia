package proactive

import (
	"log"
	"sync"
	"time"
)

type PipelineInput struct {
	CharacterID               string
	ConversationID            string
	Channel                   string
	Content                   string
	RuleID                    string
	Priority                  OutputPriority
	LastUserReplyAt           time.Time
	RecentSentCount           int
	UserActiveNow             bool
	IntimacyScore             float64
	InitiativeScore           float64
	PendingItems              int
	UnresolvedThreadCount     int
	ProspectiveDueCount       int
	QueueBackpressureLevel    BackpressureLevel
	InterruptionRiskThreshold float64
}

type PipelineResult struct {
	Allowed       bool
	Suppressed    bool
	Delivered     bool
	Motivation    int
	Suppression   int
	CorrelationID string
	LeaseID       string
	Records       []*DeliveryRecord
	DeliveredTo   []string
	Reason        string
	Interruption  InterruptionRiskResult
}

type FeedbackInput struct {
	CorrelationID     string
	Channel           string
	Status            DeliveryStatus
	UserReplied       bool
	CharacterID       string
	ReplyWithinWindow bool
}

type FeedbackResult struct {
	Applied          bool
	CooldownAdjusted bool
	BudgetUpdated    bool
	NewUnanswered    int
}

var (
	psychFeedbackMu    sync.Mutex
	characterBudgets   = make(map[string]*BudgetTracker)
	characterBudgetsMu sync.Mutex
)

func GetOrCreateBudget(characterID string, dailyLimit int, cooldown, baseInterval time.Duration) *BudgetTracker {
	characterBudgetsMu.Lock()
	defer characterBudgetsMu.Unlock()
	if bt, ok := characterBudgets[characterID]; ok {
		return bt
	}
	bt := NewBudgetTracker(dailyLimit, cooldown, baseInterval)
	characterBudgets[characterID] = bt
	return bt
}

func CleanBudgets() int {
	characterBudgetsMu.Lock()
	defer characterBudgetsMu.Unlock()
	count := len(characterBudgets)
	characterBudgets = make(map[string]*BudgetTracker)
	return count
}

func RunPipeline(input PipelineInput, sendFn func(channel, content string) bool) PipelineResult {
	result := PipelineResult{}

	now := time.Now()
	idleDuration := time.Duration(0)
	if !input.LastUserReplyAt.IsZero() {
		idleDuration = now.Sub(input.LastUserReplyAt)
	}

	us := UnifiedState{
		Energy:    0.5,
		Fatigue:   0.3,
		Busy:      false,
		Replyable: true,
	}

	motivation := ScoreMotivation(MotivationInput{
		IdleDuration:           idleDuration,
		IntimacyScore:          input.IntimacyScore,
		PendingItems:           input.PendingItems,
		InitiativeScore:        input.InitiativeScore,
		UnresolvedThreadCount:  input.UnresolvedThreadCount,
		ProspectiveDueCount:    input.ProspectiveDueCount,
		QueueBackpressureLevel: input.QueueBackpressureLevel,
	})

	suppression := ScoreSuppression(SuppressionInput{
		UnifiedState:    us,
		LastUserReplyAt: input.LastUserReplyAt,
		RecentSentCount: input.RecentSentCount,
		UserActiveNow:   input.UserActiveNow,
	})

	result.Motivation = motivation
	result.Suppression = suppression

	if motivation <= suppression {
		result.Suppressed = true
		result.Reason = "suppression_exceeds_motivation"
		return result
	}

	interruption := ScoreInterruptionRisk(InterruptionRiskInput{
		Now:                    now,
		Channel:                input.Channel,
		Availability:           InterruptionAvailabilityUnknown,
		IdleDuration:           idleDuration,
		AvailabilityConfidence: 0.5,
		ConsecutiveUnanswered:  0,
		LastOutputAt:           time.Time{},
	})
	result.Interruption = interruption

	threshold := input.InterruptionRiskThreshold
	if threshold <= 0 {
		threshold = 0.65
	}
	if !InterruptionRiskAllowsSend(interruption, threshold) {
		result.Suppressed = true
		result.Reason = "interruption_risk"
		return result
	}

	correlationID := GenerateCorrelationID(input.CharacterID, input.RuleID, input.Content)
	result.CorrelationID = correlationID

	if GlobalDedupManager.IsDuplicate(correlationID, input.Channel) {
		result.Suppressed = true
		result.Reason = "dedup_duplicate"
		return result
	}

	if GlobalDedupManager.HasSentAnyChannel(correlationID) {
		result.Suppressed = true
		result.Reason = "dedup_any_channel"
		return result
	}

	priority := input.Priority
	if priority == 0 {
		priority = PriorityNormal
	}

	lease := GlobalLeaseManager.AcquireLease(priority, input.CharacterID, input.ConversationID, input.Channel, correlationID, 2*time.Minute)
	result.LeaseID = lease.ID

	seen := make(map[string]bool)
	channels := DeliverableChannels(input.Channel, seen)
	if len(channels) == 0 {
		channels = []string{input.Channel}
		if input.Channel == "" {
			channels = []string{"web"}
		}
	}

	delivered := false
	for _, ch := range channels {
		record := GlobalDedupManager.RecordDelivery(correlationID, input.CharacterID, input.ConversationID, ch, input.Content)
		result.Records = append(result.Records, record)

		if sendFn != nil {
			if sendFn(ch, input.Content) {
				GlobalDedupManager.MarkSent(correlationID, ch)
				delivered = true
				result.DeliveredTo = append(result.DeliveredTo, ch)
			} else {
				GlobalDedupManager.MarkFailed(correlationID, ch)
			}
		} else {
			GlobalDedupManager.MarkSent(correlationID, ch)
			delivered = true
			result.DeliveredTo = append(result.DeliveredTo, ch)
		}
	}

	GlobalLeaseManager.ReleaseLease(lease.ID)

	result.Allowed = true
	result.Delivered = delivered
	if !delivered {
		result.Reason = "delivery_failed_all_channels"
	} else {
		result.Reason = "ok"
	}
	return result
}

func SuppressLowPriorityForUserInput(characterID string) int {
	cancelled := GlobalLeaseManager.CancelByUserInput(characterID)
	if cancelled > 0 {
		log.Printf("[Pipeline] user input suppressed %d low-priority leases (char=%s)", cancelled, characterID)
	}
	return cancelled
}

func RecordSendFeedback(input FeedbackInput) FeedbackResult {
	result := FeedbackResult{}

	if input.Channel == "" {
		input.Channel = "web"
	}

	record := GlobalDedupManager.GetRecord(input.CorrelationID, input.Channel)
	if record == nil {
		return result
	}

	psychFeedbackMu.Lock()
	defer psychFeedbackMu.Unlock()

	switch input.Status {
	case DeliveryStatusDelivered:
		GlobalDedupManager.MarkDelivered(input.CorrelationID, input.Channel)
		result.Applied = true
	case DeliveryStatusRead:
		GlobalDedupManager.MarkDelivered(input.CorrelationID, input.Channel)
		GlobalDedupManager.MarkRead(input.CorrelationID, input.Channel)
		result.Applied = true
	case DeliveryStatusFailed:
		GlobalDedupManager.MarkFailed(input.CorrelationID, input.Channel)
		result.Applied = true
	}

	if input.CharacterID != "" {
		bt := GetOrCreateBudget(input.CharacterID, 10, 10*time.Minute, 30*time.Minute)
		if input.UserReplied && input.ReplyWithinWindow {
			bt.OnUserReply()
			result.CooldownAdjusted = true
			result.BudgetUpdated = true
		}
	}

	return result
}

func ApplyPsychologicalFeedback(characterID string, userReplied bool) FeedbackResult {
	result := FeedbackResult{}

	if characterID == "" {
		return result
	}

	bt := GetOrCreateBudget(characterID, 10, 10*time.Minute, 30*time.Minute)

	if userReplied {
		bt.OnUserReply()
		result.CooldownAdjusted = true
		if bt.ConsecutiveUnanswered == 0 {
			result.NewUnanswered = 0
		}
	} else {
		bt.MarkUnanswered()
		result.CooldownAdjusted = true
		result.NewUnanswered = bt.ConsecutiveUnanswered
	}

	result.Applied = true
	result.BudgetUpdated = true

	log.Printf("[Pipeline] psychological feedback char=%s replied=%v unanswered=%d", characterID, userReplied, bt.ConsecutiveUnanswered)
	return result
}

func ResolveExpiredConfirmWindows(now time.Time) int {
	resolved := GlobalDedupManager.ResolveUnknown()

	resolved += GlobalLeaseManager.CleanExpired()
	resolved += GlobalDedupManager.CleanExpired()

	if resolved > 0 {
		log.Printf("[Pipeline] expired confirmation windows resolved: %d", resolved)
	}
	return resolved
}

func PipelineReset() {
	GlobalDedupManager.Reset()
	GlobalLeaseManager.Reset()
	CleanBudgets()
}
