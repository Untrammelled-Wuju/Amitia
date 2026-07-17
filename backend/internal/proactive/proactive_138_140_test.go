package proactive

import (
	"testing"
	"time"
)

func TestOutputLeaseAcquireValid(t *testing.T) {
	GlobalLeaseManager.Reset()
	lease := GlobalLeaseManager.AcquireLease(PriorityNormal, "char-1", "conv-1", "web", "corr-1", 10*time.Second)
	if lease == nil {
		t.Fatal("expected non-nil lease")
	}
	if !GlobalLeaseManager.IsLeaseValid(lease.ID) {
		t.Fatal("expected lease to be valid")
	}
}

func TestOutputLeaseExpiredInvalid(t *testing.T) {
	GlobalLeaseManager.Reset()
	lease := GlobalLeaseManager.AcquireLease(PriorityLow, "char-1", "conv-1", "web", "corr-2", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	if GlobalLeaseManager.IsLeaseValid(lease.ID) {
		t.Fatal("expected expired lease to be invalid")
	}
}

func TestOutputLeaseCancelByUserInput(t *testing.T) {
	GlobalLeaseManager.Reset()
	GlobalLeaseManager.AcquireLease(PriorityLow, "char-1", "conv-1", "web", "corr-3", 30*time.Second)
	GlobalLeaseManager.AcquireLease(PriorityNormal, "char-1", "conv-1", "web", "corr-4", 30*time.Second)
	GlobalLeaseManager.AcquireLease(PriorityHigh, "char-1", "conv-1", "web", "corr-5", 30*time.Second)
	GlobalLeaseManager.AcquireLease(PriorityLow, "char-2", "conv-2", "web", "corr-6", 30*time.Second)

	cancelled := GlobalLeaseManager.CancelByUserInput("char-1")
	if cancelled != 1 {
		t.Fatalf("expected 1 cancelled low-priority lease for char-1, got %d", cancelled)
	}
	active := GlobalLeaseManager.GetActiveLeases("char-1")
	if len(active) != 2 {
		t.Fatalf("expected 2 active leases remaining for char-1, got %d", len(active))
	}
}

func TestOutputLeasePreempt(t *testing.T) {
	GlobalLeaseManager.Reset()
	GlobalLeaseManager.AcquireLease(PriorityLow, "char-1", "conv-1", "web", "corr-7", 30*time.Second)
	preempted := GlobalLeaseManager.PreemptLease("char-1", PriorityNormal)
	if preempted == nil {
		t.Fatal("expected preempted low-priority lease")
	}
	if preempted.Priority != PriorityLow {
		t.Fatalf("expected low priority lease, got %s", preempted.Priority)
	}
}

func TestOutputLeaseCleanExpired(t *testing.T) {
	GlobalLeaseManager.Reset()
	GlobalLeaseManager.AcquireLease(PriorityNormal, "char-1", "conv-1", "web", "corr-8", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	cleaned := GlobalLeaseManager.CleanExpired()
	if cleaned < 1 {
		t.Fatalf("expected at least 1 cleaned, got %d", cleaned)
	}
}

func TestOutputLeaseCountActive(t *testing.T) {
	GlobalLeaseManager.Reset()
	GlobalLeaseManager.AcquireLease(PriorityNormal, "char-1", "conv-1", "web", "corr-9", 30*time.Second)
	GlobalLeaseManager.AcquireLease(PriorityNormal, "char-1", "conv-1", "wechat", "corr-10", 30*time.Second)
	GlobalLeaseManager.AcquireLease(PriorityNormal, "char-2", "conv-2", "web", "corr-11", 30*time.Second)
	if GlobalLeaseManager.CountActive("char-1") != 2 {
		t.Fatalf("expected 2 active for char-1, got %d", GlobalLeaseManager.CountActive("char-1"))
	}
	if GlobalLeaseManager.CountActive("") != 3 {
		t.Fatalf("expected 3 total active, got %d", GlobalLeaseManager.CountActive(""))
	}
}

func TestOutputLeaseRelease(t *testing.T) {
	GlobalLeaseManager.Reset()
	lease := GlobalLeaseManager.AcquireLease(PriorityNormal, "char-1", "conv-1", "web", "corr-12", 10*time.Second)
	GlobalLeaseManager.ReleaseLease(lease.ID)
	if GlobalLeaseManager.IsLeaseValid(lease.ID) {
		t.Fatal("expected released lease to be invalid")
	}
}

func TestTransitionDeliveryValid(t *testing.T) {
	if !TransitionDelivery(DeliveryStatusPending, DeliveryStatusSent) {
		t.Fatal("expected pending → sent to be valid")
	}
	if !TransitionDelivery(DeliveryStatusSent, DeliveryStatusDelivered) {
		t.Fatal("expected sent → delivered to be valid")
	}
	if !TransitionDelivery(DeliveryStatusSent, DeliveryStatusUnknown) {
		t.Fatal("expected sent → unknown to be valid")
	}
	if !TransitionDelivery(DeliveryStatusUnknown, DeliveryStatusDelivered) {
		t.Fatal("expected unknown → delivered to be valid")
	}
	if !TransitionDelivery(DeliveryStatusDelivered, DeliveryStatusRead) {
		t.Fatal("expected delivered → read to be valid")
	}
}

func TestTransitionDeliveryInvalid(t *testing.T) {
	if TransitionDelivery(DeliveryStatusFailed, DeliveryStatusSent) {
		t.Fatal("expected failed → sent to be invalid")
	}
	if TransitionDelivery(DeliveryStatusRead, DeliveryStatusPending) {
		t.Fatal("expected read → pending to be invalid")
	}
	if TransitionDelivery(DeliveryStatusDelivered, DeliveryStatusPending) {
		t.Fatal("expected delivered → pending to be invalid")
	}
}

func TestDeliveryRecordTransition(t *testing.T) {
	now := time.Now()
	record := &DeliveryRecord{
		ID:            "test-1",
		CorrelationID: "corr-dedup-1",
		CharacterID:   "char-1",
		Channel:       "web",
		Status:        DeliveryStatusPending,
		LastAttemptAt: now,
		CreatedAt:     now,
	}
	err := record.TransitionTo(DeliveryStatusSent, "sent")
	if err != nil {
		t.Fatalf("unexpected transition error: %v", err)
	}
	if record.Status != DeliveryStatusSent {
		t.Fatalf("expected status sent, got %s", record.Status)
	}
	if len(record.StatusHistory) != 1 {
		t.Fatalf("expected 1 status change, got %d", len(record.StatusHistory))
	}
}

func TestDeliveryRecordIllegalTransition(t *testing.T) {
	now := time.Now()
	record := &DeliveryRecord{
		ID:            "test-2",
		CorrelationID: "corr-dedup-2",
		Status:        DeliveryStatusFailed,
		LastAttemptAt: now,
		CreatedAt:     now,
	}
	err := record.TransitionTo(DeliveryStatusSent, "retry")
	if err == nil {
		t.Fatal("expected illegal transition error")
	}
}

func TestDeliveryRecordConfirmWindow(t *testing.T) {
	record := &DeliveryRecord{
		RetryCount: 2,
	}
	window := record.ConfirmWindow()
	expectedMin := 3*time.Minute + 1*time.Minute
	if window < expectedMin {
		t.Fatalf("expected confirm window >= %v, got %v", expectedMin, window)
	}
	if window > 10*time.Minute {
		t.Fatalf("expected confirm window <= 10min, got %v", window)
	}
}

func TestDeliveryRecordMarkUnknown(t *testing.T) {
	now := time.Now()
	record := &DeliveryRecord{
		ID:            "test-3",
		CorrelationID: "corr-dedup-3",
		Status:        DeliveryStatusSent,
		LastAttemptAt: now.Add(-5 * time.Minute),
	}
	changed := record.MarkUnknown(now)
	if !changed {
		t.Fatal("expected MarkUnknown to transition")
	}
	if record.Status != DeliveryStatusUnknown {
		t.Fatalf("expected status unknown, got %s", record.Status)
	}
}

func TestDeliveryRecordMarkUnknownTooSoon(t *testing.T) {
	now := time.Now()
	record := &DeliveryRecord{
		ID:            "test-4",
		CorrelationID: "corr-dedup-4",
		Status:        DeliveryStatusSent,
		LastAttemptAt: now,
	}
	changed := record.MarkUnknown(now.Add(10 * time.Second))
	if changed {
		t.Fatal("expected MarkUnknown to not transition before confirm window")
	}
}

func TestDedupManagerDuplicateDetection(t *testing.T) {
	GlobalDedupManager.Reset()
	corrID := "corr-dedup-5"
	GlobalDedupManager.RecordDelivery(corrID, "char-1", "conv-1", "web", "hello")
	if GlobalDedupManager.IsDuplicate(corrID, "web") {
		t.Fatal("expected pending record not to be treated as duplicate")
	}
	GlobalDedupManager.MarkSent(corrID, "web")
	if !GlobalDedupManager.IsDuplicate(corrID, "web") {
		t.Fatal("expected sent record to be treated as duplicate")
	}
	if GlobalDedupManager.IsDuplicate(corrID, "wechat") {
		t.Fatal("expected different channel not to be duplicate")
	}
}

func TestDedupManagerHasSentAnyChannel(t *testing.T) {
	GlobalDedupManager.Reset()
	corrID := "corr-dedup-6"
	GlobalDedupManager.RecordDelivery(corrID, "char-1", "conv-1", "web", "hello")
	GlobalDedupManager.MarkSent(corrID, "web")
	if !GlobalDedupManager.HasSentAnyChannel(corrID) {
		t.Fatal("expected HasSentAnyChannel to return true after sending to web")
	}
}

func TestDedupManagerMarkFailed(t *testing.T) {
	GlobalDedupManager.Reset()
	corrID := "corr-dedup-7"
	GlobalDedupManager.RecordDelivery(corrID, "char-1", "conv-1", "web", "hello")
	GlobalDedupManager.MarkFailed(corrID, "web")
	record := GlobalDedupManager.GetRecord(corrID, "web")
	if record == nil {
		t.Fatal("expected record to exist")
	}
	if record.Status != DeliveryStatusFailed {
		t.Fatalf("expected failed status, got %s", record.Status)
	}
}

func TestDedupManagerMarkDeliveredAndRead(t *testing.T) {
	GlobalDedupManager.Reset()
	corrID := "corr-dedup-8"
	GlobalDedupManager.RecordDelivery(corrID, "char-1", "conv-1", "web", "hello")
	GlobalDedupManager.MarkSent(corrID, "web")
	GlobalDedupManager.MarkDelivered(corrID, "web")
	GlobalDedupManager.MarkRead(corrID, "web")
	record := GlobalDedupManager.GetRecord(corrID, "web")
	if record.Status != DeliveryStatusRead {
		t.Fatalf("expected read status, got %s", record.Status)
	}
}

func TestDedupManagerCleanExpired(t *testing.T) {
	GlobalDedupManager.Reset()
	GlobalDedupManager.ttl = 10 * time.Millisecond
	GlobalDedupManager.RecordDelivery("corr-clean-1", "char-1", "conv-1", "web", "hello")
	time.Sleep(20 * time.Millisecond)
	cleaned := GlobalDedupManager.CleanExpired()
	if cleaned < 1 {
		t.Fatalf("expected at least 1 expired record cleaned, got %d", cleaned)
	}
	GlobalDedupManager.ttl = 30 * time.Minute
}

func TestDedupManagerResolveUnknown(t *testing.T) {
	GlobalDedupManager.Reset()
	now := time.Now()
	corrID := "corr-unknown-1"
	record := GlobalDedupManager.RecordDelivery(corrID, "char-1", "conv-1", "web", "hello")
	record.LastAttemptAt = now.Add(-5 * time.Minute)
	GlobalDedupManager.MarkSent(corrID, "web")
	resolved := GlobalDedupManager.ResolveUnknown()
	if resolved < 1 {
		t.Fatalf("expected at least 1 unknown resolved, got %d", resolved)
	}
}

func TestGenerateCorrelationID(t *testing.T) {
	id1 := GenerateCorrelationID("char-1", "rule-1", "hello")
	id2 := GenerateCorrelationID("char-1", "rule-1", "hello world")
	if id1 == id2 {
		t.Fatal("expected different content to produce different correlation IDs")
	}
	if len(id1) < 10 {
		t.Fatalf("expected correlation ID length >= 10, got %d", len(id1))
	}
}

func TestDeliverableChannels(t *testing.T) {
	seen := map[string]bool{"web": true}
	channels := DeliverableChannels("all", seen)
	foundWechat := false
	foundQQ := false
	foundWeb := false
	for _, ch := range channels {
		if ch == "wechat" {
			foundWechat = true
		}
		if ch == "qq" {
			foundQQ = true
		}
		if ch == "web" {
			foundWeb = true
		}
	}
	if foundWeb {
		t.Fatal("expected web to be excluded since it was already seen")
	}
	if !foundWechat || !foundQQ {
		t.Fatal("expected wechat and qq to be included")
	}
}

func TestBackpressureEnqueueDequeue(t *testing.T) {
	processed := make(chan string, 1)
	outbox := func(item *QueueItem) bool {
		processed <- item.ID
		return true
	}

	qb := NewQueueBackpressure(10, 0.8, 0.5, outbox)
	qb.Start()
	defer qb.Stop()

	item := &QueueItem{ID: "item-1", Priority: PriorityNormal, CharacterID: "char-1"}
	if !qb.Enqueue(item) {
		t.Fatal("expected enqueue to succeed")
	}
	select {
	case id := <-processed:
		if id != item.ID {
			t.Fatalf("expected item %s to be processed, got %s", item.ID, id)
		}
	case <-time.After(time.Second):
		t.Fatal("expected item to be processed")
	}
}

func TestBackpressureLevels(t *testing.T) {
	qb := NewQueueBackpressure(10, 0.8, 0.5, func(item *QueueItem) bool { return true })
	if qb.Level() != BackpressureNone {
		t.Fatal("expected none backpressure initially")
	}
	for i := 0; i < 10; i++ {
		qb.Enqueue(&QueueItem{ID: "item", Priority: PriorityNormal})
	}
	level := qb.Level()
	if level != BackpressureFull {
		t.Fatalf("expected full backpressure when queue is full, got %s", level)
	}
}

func TestBackpressureShouldThrottle(t *testing.T) {
	qb := NewQueueBackpressure(10, 0.8, 0.5, func(item *QueueItem) bool { return true })
	if qb.ShouldThrottle(PriorityNormal) {
		t.Fatal("expected no throttle initially")
	}
	for i := 0; i < 10; i++ {
		qb.Enqueue(&QueueItem{ID: "item", Priority: PriorityLow})
	}
	if !qb.ShouldThrottle(PriorityLow) {
		t.Fatal("expected throttle low priority when full")
	}
	if qb.ShouldThrottle(PriorityCrit) {
		t.Fatal("expected no throttle for critical priority even when full")
	}
}

func TestBackpressureDropLowest(t *testing.T) {
	qb := NewQueueBackpressure(3, 0.8, 0.5, func(item *QueueItem) bool { return false })
	qb.Enqueue(&QueueItem{ID: "low-1", Priority: PriorityLow})
	qb.Enqueue(&QueueItem{ID: "normal-1", Priority: PriorityNormal})
	qb.Enqueue(&QueueItem{ID: "high-1", Priority: PriorityHigh})
	ok := qb.Enqueue(&QueueItem{ID: "normal-2", Priority: PriorityNormal})
	if !ok {
		t.Fatal("expected enqueue to succeed with drop")
	}
	if qb.Size() != 3 {
		t.Fatalf("expected queue size to remain at 3 after drop, got %d", qb.Size())
	}
}

func TestMotivationUnresolvedThreads(t *testing.T) {
	none := ScoreMotivation(MotivationInput{
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureNone,
	})
	some := ScoreMotivation(MotivationInput{
		UnresolvedThreadCount:  5,
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureNone,
	})
	many := ScoreMotivation(MotivationInput{
		UnresolvedThreadCount:  20,
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureNone,
	})
	if !(none <= some && some <= many) {
		t.Fatalf("expected unresolved threads to increase monotonic, got %d %d %d", none, some, many)
	}
}

func TestMotivationProspectiveDue(t *testing.T) {
	none := ScoreMotivation(MotivationInput{
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureNone,
	})
	some := ScoreMotivation(MotivationInput{
		ProspectiveDueCount:    3,
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureNone,
	})
	many := ScoreMotivation(MotivationInput{
		ProspectiveDueCount:    15,
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureNone,
	})
	if !(none <= some && some <= many) {
		t.Fatalf("expected prospective due to increase monotonic, got %d %d %d", none, some, many)
	}
}

func TestMotivationBackpressure(t *testing.T) {
	none := ScoreMotivation(MotivationInput{
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureNone,
	})
	med := ScoreMotivation(MotivationInput{
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureMed,
	})
	high := ScoreMotivation(MotivationInput{
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureHigh,
	})
	full := ScoreMotivation(MotivationInput{
		IntimacyScore:          0.5,
		InitiativeScore:        0.5,
		QueueBackpressureLevel: BackpressureFull,
	})
	if !(none > med && med > high && high > full) {
		t.Fatalf("expected backpressure to decrease motivation monotonic, got %d %d %d %d", none, med, high, full)
	}
}

func TestMotivationAllComponentsCapped(t *testing.T) {
	full := ScoreMotivation(MotivationInput{
		IdleDuration:           72 * time.Hour,
		IntimacyScore:          1.0,
		PendingItems:           100,
		InitiativeScore:        1.0,
		UnresolvedThreadCount:  100,
		ProspectiveDueCount:    100,
		QueueBackpressureLevel: BackpressureNone,
	})
	if full > 100 {
		t.Fatalf("expected max motivation to cap at 100, got %d", full)
	}
	empty := ScoreMotivation(MotivationInput{
		QueueBackpressureLevel: BackpressureFull,
	})
	if empty < 0 {
		t.Fatalf("expected min motivation to be >= 0, got %d", empty)
	}
}

func TestDeliveryStatusIsTerminal(t *testing.T) {
	if !DeliveryStatusDelivered.IsTerminal() {
		t.Fatal("expected delivered to be terminal")
	}
	if !DeliveryStatusRead.IsTerminal() {
		t.Fatal("expected read to be terminal")
	}
	if !DeliveryStatusFailed.IsTerminal() {
		t.Fatal("expected failed to be terminal")
	}
	if DeliveryStatusPending.IsTerminal() {
		t.Fatal("expected pending to not be terminal")
	}
	if DeliveryStatusSent.IsTerminal() {
		t.Fatal("expected sent to not be terminal")
	}
	if DeliveryStatusUnknown.IsTerminal() {
		t.Fatal("expected unknown to not be terminal")
	}
}

func TestOutputPriorityString(t *testing.T) {
	if PriorityLow.String() != "low" {
		t.Fatalf("expected low, got %s", PriorityLow.String())
	}
	if PriorityNormal.String() != "normal" {
		t.Fatalf("expected normal, got %s", PriorityNormal.String())
	}
	if PriorityHigh.String() != "high" {
		t.Fatalf("expected high, got %s", PriorityHigh.String())
	}
	if PriorityCrit.String() != "critical" {
		t.Fatalf("expected critical, got %s", PriorityCrit.String())
	}
}
