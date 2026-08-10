package mindruntime

import (
	"time"

	"gorm.io/gorm"
)

type ConsistencyCheck struct {
	ID              string                   `json:"id"`
	Events          []TraceEventSummary      `json:"events"`
	Evidence        []EvidenceRecord         `json:"evidence"`
	Deltas          []StateDelta             `json:"deltas"`
	Budget          DebugBudgetSummary       `json:"budget"`
	Candidates      []CandidateRecord        `json:"candidates"`
	Queues          []QueueSnapshot          `json:"queues"`
	Cancellations   []CancellationRecord     `json:"cancellations"`
	Deliveries      []DeliveryRecord         `json:"deliveries"`
	ToolResults     []ToolResultRecord       `json:"toolResults"`
	CircuitBreakers []CircuitBreakerSnapshot `json:"circuitBreakers"`
	Version         VersionSnapshot          `json:"version"`
	CheckedAt       time.Time                `json:"checkedAt"`
}
type TraceEventSummary struct {
	EventID    string    `json:"eventId"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	DurationMs int64     `json:"durationMs"`
	CreatedAt  time.Time `json:"createdAt"`
}
type EvidenceRecord struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Source      string    `json:"source"`
	ContentHash string    `json:"contentHash"`
	Validated   bool      `json:"validated"`
	CreatedAt   time.Time `json:"createdAt"`
}
type StateDelta struct {
	Field     string      `json:"field"`
	OldValue  interface{} `json:"oldValue"`
	NewValue  interface{} `json:"newValue"`
	Version   int         `json:"version"`
	Timestamp time.Time   `json:"timestamp"`
}
type DebugBudgetSummary struct {
	TotalBudget   float64 `json:"totalBudget"`
	UsedBudget    float64 `json:"usedBudget"`
	Remaining     float64 `json:"remaining"`
	RejectedCount int64   `json:"rejectedCount"`
}
type CandidateRecord struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Score       float64   `json:"score"`
	Selected    bool      `json:"selected"`
	SubmittedAt time.Time `json:"submittedAt"`
}
type QueueSnapshot struct {
	Name       string `json:"name"`
	Depth      int    `json:"depth"`
	Processing int    `json:"processing"`
	Waiting    int    `json:"waiting"`
}
type CancellationRecord struct {
	RequestID   string    `json:"requestId"`
	Reason      string    `json:"reason"`
	Source      string    `json:"source"`
	CancelledAt time.Time `json:"cancelledAt"`
}
type DeliveryRecord struct {
	ID          string    `json:"id"`
	Channel     string    `json:"channel"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"lastAttempt"`
}
type ToolResultRecord struct {
	ToolName    string      `json:"toolName"`
	Status      string      `json:"status"`
	Result      interface{} `json:"result"`
	DurationMs  int64       `json:"durationMs"`
	Compensated bool        `json:"compensated"`
}
type CircuitBreakerSnapshot struct {
	Name        string    `json:"name"`
	State       string    `json:"state"`
	Failures    int       `json:"failures"`
	TotalCalls  int64     `json:"totalCalls"`
	LastFailure time.Time `json:"lastFailure,omitempty"`
}
type VersionSnapshot struct {
	AppVersion    string `json:"appVersion"`
	SchemaVersion int    `json:"schemaVersion"`
	GitCommit     string `json:"gitCommit"`
	BuildTime     string `json:"buildTime"`
}
type DebugPanelData struct {
	Consistency          ConsistencyCheck         `json:"consistency"`
	ReconciliationScans  []*ReconciliationScan    `json:"reconciliationScans"`
	ReconciliationStatus ReconciliationStatus     `json:"reconciliationStatus"`
	CircuitBreakers      []CircuitBreakerSnapshot `json:"circuitBreakers"`
	DependencyHealth     []DependencyHealth       `json:"dependencyHealth"`
	RuntimeMetrics       []RuntimeMetric          `json:"runtimeMetrics"`
	QueueDepth           int                      `json:"queueDepth"`
	ActiveRequests       int                      `json:"activeRequests"`
	GeneratedAt          time.Time                `json:"generatedAt"`
}
type SanitizedExport struct {
	ExportID    string         `json:"exportId"`
	GeneratedAt time.Time      `json:"generatedAt"`
	PanelData   DebugPanelData `json:"panelData"`
	AuditLog    []AuditEntry   `json:"auditLog"`
	Sanitized   bool           `json:"sanitized"`
}
type AuditEntry struct {
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
	Operator  string    `json:"operator"`
	Target    string    `json:"target"`
	Result    string    `json:"result"`
}

var DefaultReconciliationEngine = NewReconciliationEngine(DefaultReconciliationConfig())

func BuildDebugPanelData(reconciliationEngine *ReconciliationEngine) DebugPanelData {
	now := time.Now().UTC()
	reports := DefaultCircuitBreakerRegistry.AllHealthReports()
	cbSnapshots := make([]CircuitBreakerSnapshot, 0)
	for _, report := range reports {
		if report.CircuitBreaker != nil {
			cbSnapshots = append(cbSnapshots, CircuitBreakerSnapshot{
				Name:        report.Name,
				State:       string(report.CircuitBreaker.Status()),
				Failures:    report.CircuitBreaker.Failures,
				TotalCalls:  report.CircuitBreaker.TotalCalls,
				LastFailure: report.CircuitBreaker.LastFailure,
			})
		}
	}
	var scans []*ReconciliationScan
	var status ReconciliationStatus
	if reconciliationEngine != nil {
		scans = reconciliationEngine.AllScans()
		status = reconciliationEngine.Status()
	}
	metrics := make([]RuntimeMetric, 0)
	metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricQueueDepth, Value: 0})
	metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricConsistencyDiffs, Value: 0})
	metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricLeaseCollision, Value: 0})
	metrics = append(metrics, RuntimeMetric{Name: RuntimeMetricUnknownDelivery, Value: 0})
	return DebugPanelData{
		Consistency: ConsistencyCheck{
			ID:        "debug-" + now.Format("20060102T150405"),
			CheckedAt: now,
			Events:    make([]TraceEventSummary, 0),
			Evidence:  make([]EvidenceRecord, 0),
			Deltas:    make([]StateDelta, 0),
			Budget: DebugBudgetSummary{
				TotalBudget:   100.0,
				UsedBudget:    0.0,
				Remaining:     100.0,
				RejectedCount: 0,
			},
			Candidates:      make([]CandidateRecord, 0),
			Queues:          make([]QueueSnapshot, 0),
			Cancellations:   make([]CancellationRecord, 0),
			Deliveries:      make([]DeliveryRecord, 0),
			ToolResults:     make([]ToolResultRecord, 0),
			CircuitBreakers: cbSnapshots,
			Version: VersionSnapshot{
				AppVersion:    "1.0.0",
				SchemaVersion: 1,
			},
		},
		ReconciliationScans:  scans,
		ReconciliationStatus: status,
		CircuitBreakers:      cbSnapshots,
		DependencyHealth:     reports,
		RuntimeMetrics:       metrics,
		GeneratedAt:          now,
	}
}
func BuildSanitizedExport(reconciliationEngine *ReconciliationEngine) SanitizedExport {
	now := time.Now().UTC()
	return SanitizedExport{
		ExportID:    "export-" + now.Format("20060102T150405"),
		GeneratedAt: now,
		PanelData:   BuildDebugPanelData(reconciliationEngine),
		AuditLog:    make([]AuditEntry, 0),
		Sanitized:   true,
	}
}

func deliveryIntentSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "delivery_intents",
				Kind:          "delivery_intent",
				KeyColumns:    []string{"request_id"},
				StatusColumn:  "status",
				VersionColumn: "retry_count",
				HashColumns:   []string{"status", "channel", "retry_count", "last_error"},
				FieldColumns:  []string{"request_id", "channel", "status", "last_error"},
			},
		},
	}
}

func outboxChannelReceiptSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "channel_receipt",
		Tables: []GormReconciliationTable{
			{
				Table:         "delivery_intents",
				Kind:          "delivery_receipt",
				KeyColumns:    []string{"request_id"},
				StatusColumn:  "status",
				VersionColumn: "retry_count",
				HashColumns:   []string{"status", "channel", "retry_count"},
				FieldColumns:  []string{"request_id", "channel", "status"},
			},
		},
	}
}
