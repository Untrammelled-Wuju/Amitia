package mindruntime

import (
	"strings"
	"time"
)

type TraceEventKind string

const (
	TraceEventRequest      TraceEventKind = "request"
	TraceEventInteraction  TraceEventKind = "interaction"
	TraceEventCompensation TraceEventKind = "compensation"
	TraceEventValidation   TraceEventKind = "validation"
	TraceEventFrame        TraceEventKind = "frame"
	TraceEventTool         TraceEventKind = "tool"
	TraceEventDelivery     TraceEventKind = "delivery"
	TraceEventOutbox       TraceEventKind = "outbox"
	TraceEventLease        TraceEventKind = "lease"
	TraceEventSuperseded   TraceEventKind = "superseded"
	TraceEventCancel       TraceEventKind = "cancel"
	TraceEventCircuit      TraceEventKind = "circuit_breaker"
)

type RuntimeMetricName string

const (
	RuntimeMetricLatencyMillis    RuntimeMetricName = "latency_millis"
	RuntimeMetricQueueMillis      RuntimeMetricName = "queue_millis"
	RuntimeMetricTraceFrameCount  RuntimeMetricName = "trace_frame_count"
	RuntimeMetricDiagnosticCount  RuntimeMetricName = "diagnostic_count"
	RuntimeMetricModelCallCount   RuntimeMetricName = "model_call_count"
	RuntimeMetricQueueDepth       RuntimeMetricName = "queue_depth"
	RuntimeMetricConflictCount    RuntimeMetricName = "conflict_count"
	RuntimeMetricBudgetRejected   RuntimeMetricName = "budget_rejected"
	RuntimeMetricDegraded         RuntimeMetricName = "degraded"
	RuntimeMetricLeaseCollision   RuntimeMetricName = "lease_collision"
	RuntimeMetricUnknownDelivery  RuntimeMetricName = "unknown_delivery"
	RuntimeMetricConsistencyDiffs RuntimeMetricName = "consistency_diffs"
)

type RuntimeObservabilityInput struct {
	Snapshot             RuntimeSnapshot
	RequestID            string
	EventID              string
	DeliveryID           string
	ToolID               string
	Path                 string
	Priority             string
	Scope                string
	InteractionStatus    string
	QueueDuration        time.Duration
	TotalDuration        time.Duration
	CancellationReason   string
	SupersededBy         string
	ToolStatus           string
	DeliveryStatus       string
	DeliveryIntent       string
	OutboxStatus         string
	LeaseStatus          string
	CircuitBreakerStatus string
	ModelCallCount       int
	QueueDepth           int
	ConflictCount        int
	BudgetRejected       bool
	BudgetUsed           float64
	BudgetLimit          float64
	ContextVersion       int
	BeliefParsing        string
	Evaluation           string
	CandidateCount       int
	ArbitrationResult    string
	PromptInfo           string
	ValidationResult     string
	CompensationEvent    string
	RedactSensitive      bool
	DegradationReason    string
	ConsistencyDiffs     int
}

type RuntimeObservabilityReport struct {
	SnapshotID        string               `json:"snapshotId"`
	RequestID         string               `json:"requestId,omitempty"`
	EventID           string               `json:"eventId,omitempty"`
	UserID            string               `json:"userId,omitempty"`
	CharacterID       string               `json:"characterId,omitempty"`
	InteractionID     string               `json:"interactionId,omitempty"`
	Scope             string               `json:"scope,omitempty"`
	StateVersion      int                  `json:"stateVersion"`
	Path              string               `json:"path,omitempty"`
	Priority          string               `json:"priority,omitempty"`
	InteractionStatus string               `json:"interactionStatus,omitempty"`
	CreatedAt         time.Time            `json:"createdAt"`
	CausalChain       []RuntimeCausalEvent `json:"causalChain"`
	Metrics           []RuntimeMetric      `json:"metrics"`
	Diagnostics       []RuntimeDiagnostic  `json:"diagnostics,omitempty"`
	Redacted          bool                 `json:"redacted"`
	RetentionUntil    time.Time            `json:"retentionUntil,omitempty"`
}

type RuntimeCausalEvent struct {
	Index            int            `json:"index"`
	Kind             TraceEventKind `json:"kind"`
	ID               string         `json:"id,omitempty"`
	ParentID         string         `json:"parentId,omitempty"`
	Stage            TraceStage     `json:"stage,omitempty"`
	Status           string         `json:"status,omitempty"`
	Reason           string         `json:"reason,omitempty"`
	Scope            string         `json:"scope,omitempty"`
	Path             string         `json:"path,omitempty"`
	Priority         string         `json:"priority,omitempty"`
	QueueDurationMs  int64          `json:"queueDurationMs,omitempty"`
	ContextVersion   int            `json:"contextVersion,omitempty"`
	BudgetUsed       float64        `json:"budgetUsed,omitempty"`
	BudgetLimit      float64        `json:"budgetLimit,omitempty"`
	CandidateCount   int            `json:"candidateCount,omitempty"`
	ValidationResult string         `json:"validationResult,omitempty"`
}

type RuntimeMetric struct {
	Name  RuntimeMetricName `json:"name"`
	Value int64             `json:"value"`
}

func BuildRuntimeObservabilityReport(input RuntimeObservabilityInput) RuntimeObservabilityReport {
	snapshot := input.Snapshot
	report := RuntimeObservabilityReport{
		SnapshotID:        snapshot.ID,
		RequestID:         strings.TrimSpace(input.RequestID),
		EventID:           strings.TrimSpace(input.EventID),
		UserID:            snapshot.UserID,
		CharacterID:       snapshot.CharacterID,
		InteractionID:     snapshot.InteractionID,
		Scope:             strings.TrimSpace(input.Scope),
		StateVersion:      snapshot.StateVersion,
		Path:              strings.TrimSpace(input.Path),
		Priority:          strings.TrimSpace(input.Priority),
		InteractionStatus: strings.TrimSpace(input.InteractionStatus),
		CreatedAt:         snapshot.CreatedAt,
		Diagnostics:       append([]RuntimeDiagnostic{}, snapshot.Diagnostics...),
		Redacted:          input.RedactSensitive,
	}
	report.CausalChain = buildRuntimeCausalChain(input)
	report.Metrics = buildRuntimeMetrics(input)
	return report
}

func buildRuntimeCausalChain(input RuntimeObservabilityInput) []RuntimeCausalEvent {
	snapshot := input.Snapshot
	events := make([]RuntimeCausalEvent, 0, len(snapshot.Trace)+10)
	lastID := strings.TrimSpace(input.RequestID)
	if lastID != "" {
		events = append(events, RuntimeCausalEvent{
			Kind: TraceEventRequest, ID: lastID, Status: input.Path,
			Scope: strings.TrimSpace(input.Scope), Path: strings.TrimSpace(input.Path),
			Priority: strings.TrimSpace(input.Priority), QueueDurationMs: durationMillis(input.QueueDuration),
			ContextVersion: input.ContextVersion,
		})
	}
	if snapshot.InteractionID != "" {
		parentID := lastID
		events = append(events, RuntimeCausalEvent{
			Kind: TraceEventInteraction, ID: snapshot.InteractionID, ParentID: parentID,
			Status: strings.TrimSpace(input.InteractionStatus),
			Scope: strings.TrimSpace(input.Scope), ContextVersion: input.ContextVersion,
			BudgetUsed: input.BudgetUsed, BudgetLimit: input.BudgetLimit,
			CandidateCount: input.CandidateCount, ValidationResult: strings.TrimSpace(input.ValidationResult),
		})
		lastID = snapshot.InteractionID
	}
	for _, frame := range snapshot.Trace {
		id := strings.TrimSpace(frame.Reference.ID)
		events = append(events, RuntimeCausalEvent{Kind: TraceEventFrame, ID: id, ParentID: lastID, Stage: frame.Stage, Status: strings.TrimSpace(frame.Reference.Version)})
		if id != "" {
			lastID = id
		}
	}
	if input.ToolID != "" || input.ToolStatus != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventTool, ID: strings.TrimSpace(input.ToolID), ParentID: lastID, Status: strings.TrimSpace(input.ToolStatus)})
	}
	if input.DeliveryID != "" || input.DeliveryStatus != "" {
		events = append(events, RuntimeCausalEvent{
			Kind: TraceEventDelivery, ID: strings.TrimSpace(input.DeliveryID),
			ParentID: lastID, Status: strings.TrimSpace(input.DeliveryStatus),
			Reason: strings.TrimSpace(input.DeliveryIntent),
		})
	}
	if input.OutboxStatus != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventOutbox, ParentID: lastID, Status: strings.TrimSpace(input.OutboxStatus)})
	}
	if input.LeaseStatus != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventLease, ParentID: lastID, Status: strings.TrimSpace(input.LeaseStatus)})
	}
	if input.SupersededBy != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventSuperseded, ID: strings.TrimSpace(input.SupersededBy), ParentID: snapshot.InteractionID})
	}
	if input.CancellationReason != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventCancel, ParentID: snapshot.InteractionID, Reason: strings.TrimSpace(input.CancellationReason)})
	}
	if input.CircuitBreakerStatus != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventCircuit, ParentID: lastID, Status: strings.TrimSpace(input.CircuitBreakerStatus)})
	}
	if strings.TrimSpace(input.CompensationEvent) != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventCompensation, ParentID: lastID, Reason: strings.TrimSpace(input.CompensationEvent)})
	}
	if strings.TrimSpace(input.ValidationResult) != "" {
		events = append(events, RuntimeCausalEvent{Kind: TraceEventValidation, ParentID: snapshot.InteractionID, Status: strings.TrimSpace(input.ValidationResult)})
	}
	for i := range events {
		events[i].Index = i + 1
	}
	return events
}

func buildRuntimeMetrics(input RuntimeObservabilityInput) []RuntimeMetric {
	metrics := []RuntimeMetric{
		{Name: RuntimeMetricLatencyMillis, Value: durationMillis(input.TotalDuration)},
		{Name: RuntimeMetricQueueMillis, Value: durationMillis(input.QueueDuration)},
		{Name: RuntimeMetricTraceFrameCount, Value: int64(len(input.Snapshot.Trace))},
		{Name: RuntimeMetricDiagnosticCount, Value: int64(len(input.Snapshot.Diagnostics))},
	}
	if input.BudgetRejected {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricBudgetRejected, Value: 1})
	}
	if input.BudgetUsed > 0 || input.BudgetLimit > 0 {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricBudgetRejected, Value: budgetRejectedCountMetric(input)})
	}
	if input.ModelCallCount > 0 {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricModelCallCount, Value: int64(input.ModelCallCount)})
	}
	if input.QueueDepth > 0 {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricQueueDepth, Value: int64(input.QueueDepth)})
	}
	if input.ConflictCount > 0 {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricConflictCount, Value: int64(input.ConflictCount)})
	}
	if input.CandidateCount > 0 {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricModelCallCount, Value: int64(input.CandidateCount)})
	}
	if strings.TrimSpace(input.CompensationEvent) != "" {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricDegraded, Value: 1})
	}
	if strings.TrimSpace(input.ValidationResult) != "" && !strings.EqualFold(strings.TrimSpace(input.ValidationResult), "ok") {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricConsistencyDiffs, Value: 1})
	}
	if strings.TrimSpace(input.DegradationReason) != "" {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricDegraded, Value: 1})
	}
	if strings.EqualFold(strings.TrimSpace(input.LeaseStatus), "collision") {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricLeaseCollision, Value: 1})
	}
	if strings.EqualFold(strings.TrimSpace(input.DeliveryStatus), "unknown") {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricUnknownDelivery, Value: 1})
	}
	if input.ConsistencyDiffs > 0 {
		metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricConsistencyDiffs, Value: int64(input.ConsistencyDiffs)})
	}
	return metrics
}

func durationMillis(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	return value.Milliseconds()
}

func budgetRejectedCountMetric(input RuntimeObservabilityInput) int64 {
	if input.BudgetLimit > 0 && input.BudgetUsed > input.BudgetLimit {
		ratio := input.BudgetUsed / input.BudgetLimit
		if ratio > 2.0 {
			return int64(2)
		}
		if ratio > 1.0 {
			return int64(1)
		}
	}
	return 0
}