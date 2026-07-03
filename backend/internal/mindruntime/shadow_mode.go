package mindruntime

import (
	"sort"
	"time"
)

type ShadowPhase string

const (
	ShadowPhaseInteraction    ShadowPhase = "interaction"
	ShadowPhaseStateVersion   ShadowPhase = "state_version"
	ShadowPhasePsyche         ShadowPhase = "psyche"
	ShadowPhaseBelief         ShadowPhase = "belief"
	ShadowPhaseBDI            ShadowPhase = "bdi"
	ShadowPhaseDelivery       ShadowPhase = "delivery"
	ShadowPhaseReconciliation ShadowPhase = "reconciliation"
	ShadowPhaseProactive      ShadowPhase = "proactive"
	ShadowPhaseReflection     ShadowPhase = "reflection"
)

type ShadowModeStatus string

const (
	ShadowModeOff    ShadowModeStatus = "off"
	ShadowModeShadow ShadowModeStatus = "shadow"
	ShadowModeGray   ShadowModeStatus = "gray"
	ShadowModeFull   ShadowModeStatus = "full"
)

type ShadowDecision struct {
	ID               string        `json:"id"`
	Phase            ShadowPhase   `json:"phase"`
	ComputedState    string        `json:"computedState"`
	ComputedDecision string        `json:"computedDecision"`
	CollaborationRef string        `json:"collaborationRef,omitempty"`
	SentToAuthority  bool          `json:"sentToAuthority"`
	GeneratedAt      time.Time     `json:"generatedAt"`
	Metrics          ShadowMetrics `json:"metrics"`
}

type ShadowMetrics struct {
	LatencyMs           int64   `json:"latencyMs"`
	ErrorCount          int     `json:"errorCount"`
	CancelCount         int     `json:"cancelCount"`
	QueueDepth          int     `json:"queueDepth"`
	DeliveryStatus      string  `json:"deliveryStatus"`
	SafetyScore         float64 `json:"safetyScore"`
	ConsistencyDiffs    int     `json:"consistencyDiffs"`
	UnknownBacklog      int     `json:"unknownBacklog"`
	DuplicateDeliveries int     `json:"duplicateDeliveries"`
	QueueAgeMs          int64   `json:"queueAgeMs"`
}

type ShadowComparatorResult struct {
	OldReplyFeatures map[string]interface{} `json:"oldReplyFeatures"`
	NewReplyFeatures map[string]interface{} `json:"newReplyFeatures"`
	LatencyDiff      int64                  `json:"latencyDiff"`
	ErrorRateChange  float64                `json:"errorRateChange"`
	CancelRateChange float64                `json:"cancelRateChange"`
	QueueDepthChange int                    `json:"queueDepthChange"`
	DeliveryChange   string                 `json:"deliveryChange"`
	SafetyDiff       float64                `json:"safetyDiff"`
	ConsistencyDiff  int                    `json:"consistencyDiff"`
	Decision         string                 `json:"decision"`
	ComparedAt       time.Time              `json:"comparedAt"`
}

type AutoRollbackThresholds struct {
	MaxErrorRate           float64       `json:"maxErrorRate"`
	MaxP95Latency          time.Duration `json:"maxP95Latency"`
	MaxDuplicateDeliveries int           `json:"maxDuplicateDeliveries"`
	MaxUnknownBacklog      int           `json:"maxUnknownBacklog"`
	MaxConsistencyDiffs    int           `json:"maxConsistencyDiffs"`
	MaxPostCancelSubmit    int           `json:"maxPostCancelSubmit"`
	MaxQueueAge            time.Duration `json:"maxQueueAge"`
}

type ShadowRollbackEvent struct {
	ID              string           `json:"id"`
	Phase           ShadowPhase      `json:"phase"`
	FromStatus      ShadowModeStatus `json:"fromStatus"`
	ToStatus        ShadowModeStatus `json:"toStatus"`
	TriggerReason   string           `json:"triggerReason"`
	TriggerMetric   string           `json:"triggerMetric"`
	TriggerValue    float64          `json:"triggerValue"`
	Threshold       float64          `json:"threshold"`
	RolledBackAt    time.Time        `json:"rolledBackAt"`
	PreservedEvents []string         `json:"preservedEvents"`
}

type ShadowState struct {
	CurrentPhase    ShadowPhase              `json:"currentPhase"`
	Status          ShadowModeStatus         `json:"status"`
	ActiveSince     time.Time                `json:"activeSince"`
	Decisions       []ShadowDecision         `json:"decisions"`
	Comparisons     []ShadowComparatorResult `json:"comparisons"`
	Rollbacks       []ShadowRollbackEvent    `json:"rollbacks"`
	Thresholds      AutoRollbackThresholds   `json:"thresholds"`
	MetricsSnapshot ShadowMetrics            `json:"metricsSnapshot"`
	PhasesCompleted []ShadowPhase            `json:"phasesCompleted"`
}

func DefaultAutoRollbackThresholds() AutoRollbackThresholds {
	return AutoRollbackThresholds{
		MaxErrorRate:           0.10,
		MaxP95Latency:          5 * time.Second,
		MaxDuplicateDeliveries: 3,
		MaxUnknownBacklog:      10,
		MaxConsistencyDiffs:    5,
		MaxPostCancelSubmit:    2,
		MaxQueueAge:            30 * time.Second,
	}
}

func NewShadowState() ShadowState {
	return ShadowState{
		CurrentPhase:    ShadowPhaseInteraction,
		Status:          ShadowModeOff,
		ActiveSince:     time.Time{},
		Decisions:       make([]ShadowDecision, 0),
		Comparisons:     make([]ShadowComparatorResult, 0),
		Rollbacks:       make([]ShadowRollbackEvent, 0),
		Thresholds:      DefaultAutoRollbackThresholds(),
		PhasesCompleted: make([]ShadowPhase, 0),
	}
}

func ComputeShadowDecision(input RuntimeObservabilityInput, phase ShadowPhase) ShadowDecision {
	metrics := buildShadowMetrics(input)

	decision := ShadowDecision{
		ID:               shadowDecisionID(input, phase),
		Phase:            phase,
		ComputedState:    computeShadowState(input.Snapshot),
		ComputedDecision: computeShadowAction(input),
		SentToAuthority:  false,
		GeneratedAt:      time.Now().UTC(),
		Metrics:          metrics,
	}

	return decision
}

func CompareShadowResults(oldMetrics, newMetrics ShadowMetrics) ShadowComparatorResult {
	result := ShadowComparatorResult{
		OldReplyFeatures: map[string]interface{}{
			"latencyMs":      oldMetrics.LatencyMs,
			"errorCount":     oldMetrics.ErrorCount,
			"cancelCount":    oldMetrics.CancelCount,
			"queueDepth":     oldMetrics.QueueDepth,
			"deliveryStatus": oldMetrics.DeliveryStatus,
			"safetyScore":    oldMetrics.SafetyScore,
		},
		NewReplyFeatures: map[string]interface{}{
			"latencyMs":      newMetrics.LatencyMs,
			"errorCount":     newMetrics.ErrorCount,
			"cancelCount":    newMetrics.CancelCount,
			"queueDepth":     newMetrics.QueueDepth,
			"deliveryStatus": newMetrics.DeliveryStatus,
			"safetyScore":    newMetrics.SafetyScore,
		},
		LatencyDiff:      newMetrics.LatencyMs - oldMetrics.LatencyMs,
		ErrorRateChange:  safeDiv(float64(newMetrics.ErrorCount-oldMetrics.ErrorCount), float64(oldMetrics.ErrorCount+1)),
		CancelRateChange: safeDiv(float64(newMetrics.CancelCount-oldMetrics.CancelCount), float64(oldMetrics.CancelCount+1)),
		QueueDepthChange: newMetrics.QueueDepth - oldMetrics.QueueDepth,
		SafetyDiff:       newMetrics.SafetyScore - oldMetrics.SafetyScore,
		ConsistencyDiff:  newMetrics.ConsistencyDiffs - oldMetrics.ConsistencyDiffs,
		ComparedAt:       time.Now().UTC(),
	}

	if newMetrics.DeliveryStatus == oldMetrics.DeliveryStatus {
		result.DeliveryChange = "unchanged"
	} else {
		result.DeliveryChange = oldMetrics.DeliveryStatus + "->" + newMetrics.DeliveryStatus
	}

	result.Decision = evaluateShadowComparison(result)
	return result
}

func CheckAutoRollback(state ShadowState, currentMetrics ShadowMetrics, thresholds AutoRollbackThresholds) (bool, ShadowRollbackEvent) {
	triggers := make([]string, 0)

	if safeDiv(float64(currentMetrics.ErrorCount), float64(state.MetricsSnapshot.ErrorCount+1)) > thresholds.MaxErrorRate {
		triggers = append(triggers, "error_rate")
	}

	if time.Duration(currentMetrics.LatencyMs)*time.Millisecond > thresholds.MaxP95Latency {
		triggers = append(triggers, "p95_latency")
	}

	if currentMetrics.DuplicateDeliveries > thresholds.MaxDuplicateDeliveries {
		triggers = append(triggers, "duplicate_deliveries")
	}

	if currentMetrics.UnknownBacklog > thresholds.MaxUnknownBacklog {
		triggers = append(triggers, "unknown_backlog")
	}

	if currentMetrics.ConsistencyDiffs > thresholds.MaxConsistencyDiffs {
		triggers = append(triggers, "consistency_diffs")
	}

	if currentMetrics.CancelCount > thresholds.MaxPostCancelSubmit {
		triggers = append(triggers, "post_cancel_submit")
	}

	if time.Duration(currentMetrics.QueueAgeMs)*time.Millisecond > thresholds.MaxQueueAge {
		triggers = append(triggers, "queue_age")
	}

	if len(triggers) == 0 {
		return false, ShadowRollbackEvent{}
	}

	reason := triggers[0]
	metric := triggers[0]
	value := float64(currentMetrics.ErrorCount)

	event := ShadowRollbackEvent{
		ID:              shadowRollbackID(state.CurrentPhase, state.Status, ShadowModeShadow),
		Phase:           state.CurrentPhase,
		FromStatus:      state.Status,
		ToStatus:        ShadowModeShadow,
		TriggerReason:   reason,
		TriggerMetric:   metric,
		TriggerValue:    value,
		Threshold:       thresholds.MaxErrorRate,
		RolledBackAt:    time.Now().UTC(),
		PreservedEvents: preserveDecisionIDs(state.Decisions),
	}

	return true, event
}

func AdvanceShadowPhase(state ShadowState) (ShadowState, bool) {
	phases := []ShadowPhase{
		ShadowPhaseInteraction,
		ShadowPhaseStateVersion,
		ShadowPhasePsyche,
		ShadowPhaseBelief,
		ShadowPhaseBDI,
		ShadowPhaseDelivery,
		ShadowPhaseReconciliation,
		ShadowPhaseProactive,
		ShadowPhaseReflection,
	}

	currentIdx := -1
	for i, p := range phases {
		if p == state.CurrentPhase {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 || currentIdx >= len(phases)-1 {
		return state, false
	}

	nextPhase := phases[currentIdx+1]
	newState := state
	newState.PhasesCompleted = append([]ShadowPhase{}, state.PhasesCompleted...)
	newState.PhasesCompleted = append(newState.PhasesCompleted, state.CurrentPhase)
	newState.CurrentPhase = nextPhase

	return newState, true
}

func IsShadowPhaseComplete(state ShadowState) bool {
	return state.CurrentPhase == ShadowPhaseReflection
}

func AllShadowPhases() []ShadowPhase {
	return []ShadowPhase{
		ShadowPhaseInteraction,
		ShadowPhaseStateVersion,
		ShadowPhasePsyche,
		ShadowPhaseBelief,
		ShadowPhaseBDI,
		ShadowPhaseDelivery,
		ShadowPhaseReconciliation,
		ShadowPhaseProactive,
		ShadowPhaseReflection,
	}
}

func buildShadowMetrics(input RuntimeObservabilityInput) ShadowMetrics {
	return ShadowMetrics{
		LatencyMs:           durationMillis(input.TotalDuration),
		ErrorCount:          countDiagnosticSeverity(input.Snapshot.Diagnostics, DiagnosticSeverityWarning),
		CancelCount:         boolToInt(input.CancellationReason != ""),
		QueueDepth:          input.QueueDepth,
		DeliveryStatus:      input.DeliveryStatus,
		SafetyScore:         0.95,
		ConsistencyDiffs:    input.ConsistencyDiffs,
		UnknownBacklog:      boolToInt(stringsEqualFold(input.DeliveryStatus, "unknown")),
		DuplicateDeliveries: boolToInt(input.OutboxStatus == "duplicate"),
		QueueAgeMs:          durationMillis(input.QueueDuration),
	}
}

func computeShadowState(snapshot RuntimeSnapshot) string {
	if snapshot.StateVersion > 0 {
		return "state-v" + intToStr(snapshot.StateVersion)
	}
	return "state-init"
}

func computeShadowAction(input RuntimeObservabilityInput) string {
	if input.CancellationReason != "" {
		return "cancel"
	}
	if input.BudgetRejected {
		return "budget_rejected"
	}
	if input.ToolID != "" {
		return "tool:" + input.ToolID
	}
	return "process"
}

func evaluateShadowComparison(result ShadowComparatorResult) string {
	issues := 0
	if result.ErrorRateChange > 0.05 {
		issues++
	}
	if result.LatencyDiff > 1000 {
		issues++
	}
	if result.CancelRateChange > 0.05 {
		issues++
	}
	if result.QueueDepthChange > 50 {
		issues++
	}
	if result.SafetyDiff < -0.05 {
		issues++
	}
	if result.ConsistencyDiff > 3 {
		issues++
	}
	if issues == 0 {
		return "promote"
	}
	if issues <= 2 {
		return "hold"
	}
	return "rollback"
}

func shadowDecisionID(input RuntimeObservabilityInput, phase ShadowPhase) string {
	ts := time.Now().UTC().Format("20060102T150405")
	return "shadow-" + string(phase) + "-" + input.Snapshot.ID[:8] + "-" + ts
}

func shadowRollbackID(phase ShadowPhase, from, to ShadowModeStatus) string {
	ts := time.Now().UTC().Format("20060102T150405")
	return "shadow-rollback-" + string(phase) + "-" + string(from) + "-" + string(to) + "-" + ts
}

func preserveDecisionIDs(decisions []ShadowDecision) []string {
	ids := make([]string, 0, len(decisions))
	for _, d := range decisions {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func countDiagnosticSeverity(diags []RuntimeDiagnostic, sev DiagnosticSeverity) int {
	count := 0
	for _, d := range diags {
		if d.Severity == sev {
			count++
		}
	}
	return count
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
