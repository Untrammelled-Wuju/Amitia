package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type CausalChainExportMeta struct {
	SnapshotVersion string    `json:"snapshotVersion"`
	GeneratedAt     time.Time `json:"generatedAt"`
	ReportCount     int       `json:"reportCount"`
	TotalEvents     int       `json:"totalEvents"`
	Checksum        string    `json:"checksum"`
	RetentionUntil  time.Time `json:"retentionUntil,omitempty"`
	Redacted        bool      `json:"redacted"`
}

type CausalChainExportedEvent struct {
	Index            int            `json:"index"`
	Kind             TraceEventKind `json:"kind"`
	ID               string         `json:"id,omitempty"`
	ParentID         string         `json:"parentId,omitempty"`
	Stage            TraceStage     `json:"stage,omitempty"`
	Status           string         `json:"status,omitempty"`
	Reason           string         `json:"reason,omitempty"`
	InteractionID    string         `json:"interactionId,omitempty"`
	CharacterID      string         `json:"characterId,omitempty"`
	RequestID        string         `json:"requestId,omitempty"`
	Scope            string         `json:"scope,omitempty"`
	Path             string         `json:"path,omitempty"`
	Priority         string         `json:"priority,omitempty"`
	QueueDurationMs  int64          `json:"queueDurationMs,omitempty"`
	ContextVersion   int            `json:"contextVersion,omitempty"`
	SupersededBy     string         `json:"supersededBy,omitempty"`
	CancelReason     string         `json:"cancelReason,omitempty"`
	ToolResultStatus string         `json:"toolResultStatus,omitempty"`
	DeliveryIntent   string         `json:"deliveryIntent,omitempty"`
	OutboxStatus     string         `json:"outboxStatus,omitempty"`
	LeaseStatus      string         `json:"leaseStatus,omitempty"`
	CircuitState     string         `json:"circuitState,omitempty"`
	Compensation     string         `json:"compensation,omitempty"`
	BudgetUsed       float64        `json:"budgetUsed,omitempty"`
	BudgetLimit      float64        `json:"budgetLimit,omitempty"`
	ValidationResult string         `json:"validationResult,omitempty"`
}

type CausalChainSnapshot struct {
	Meta         CausalChainExportMeta      `json:"meta"`
	Events       []CausalChainExportedEvent `json:"events"`
	Aggregations []MetricAggregation        `json:"aggregations"`
	Summary      CausalChainSummary         `json:"summary"`
}

type RedactionRule struct {
	Enabled bool
	Fields  []string
}

var DefaultRedactionRule = RedactionRule{
	Enabled: true,
	Fields:  []string{"characterId", "userId", "requestId", "interactionId"},
}

func redactField(fieldName string, rules RedactionRule) bool {
	if !rules.Enabled {
		return false
	}
	for _, f := range rules.Fields {
		if strings.EqualFold(f, fieldName) {
			return true
		}
	}
	return false
}

func redactContent(value string) string {
	if value == "" {
		return ""
	}
	redacted := "***"
	if len(value) > 6 {
		visible := value[len(value)-4:]
		redacted = "***" + visible
	}
	return redacted
}

func ExportCausalChainSnapshot(reports []RuntimeObservabilityReport, retention time.Duration, redacted bool) CausalChainSnapshot {
	if reports == nil {
		reports = []RuntimeObservabilityReport{}
	}
	events := make([]CausalChainExportedEvent, 0)
	metrics := make([]RuntimeMetric, 0)
	allRawEvents := make([]RuntimeCausalEvent, 0)
	var totalDuration time.Duration

	for _, report := range reports {
		for _, ce := range report.CausalChain {
			ee := CausalChainExportedEvent{
				Index:            len(events) + 1,
				Kind:             ce.Kind,
				ID:               ce.ID,
				ParentID:         ce.ParentID,
				Stage:            ce.Stage,
				Status:           ce.Status,
				Reason:           ce.Reason,
				InteractionID:    report.InteractionID,
				CharacterID:      report.CharacterID,
				RequestID:        report.RequestID,
				Scope:            ce.Scope,
				Path:             ce.Path,
				Priority:         ce.Priority,
				QueueDurationMs:  ce.QueueDurationMs,
				ContextVersion:   ce.ContextVersion,
				BudgetUsed:       ce.BudgetUsed,
				BudgetLimit:      ce.BudgetLimit,
				ValidationResult: ce.ValidationResult,
			}
			if ce.Kind == TraceEventSuperseded {
				ee.SupersededBy = ce.ID
			}
			if ce.Kind == TraceEventCancel {
				ee.CancelReason = ce.Reason
			}
			if ce.Kind == TraceEventTool {
				ee.ToolResultStatus = ce.Status
			}
			if ce.Kind == TraceEventDelivery {
				ee.DeliveryIntent = ce.Reason
			}
			if ce.Kind == TraceEventOutbox {
				ee.OutboxStatus = ce.Status
			}
			if ce.Kind == TraceEventLease {
				ee.LeaseStatus = ce.Status
			}
			if ce.Kind == TraceEventCircuit {
				ee.CircuitState = ce.Status
			}
			if ce.Kind == TraceEventCompensation {
				ee.Compensation = ce.Reason
			}

			if redacted {
				ee = redactExportedEvent(ee, DefaultRedactionRule)
			}
			events = append(events, ee)
		}
		allRawEvents = append(allRawEvents, report.CausalChain...)
		metrics = append(metrics, report.Metrics...)
		dur := report.CreatedAt.Sub(report.CreatedAt)
		if dur > 0 && dur < 24*time.Hour {
			totalDuration += dur
		}
	}
	summary := BuildCausalChainSummary(allRawEvents, totalDuration)
	aggregations := AggregateRuntimeMetrics(metrics)

	meta := CausalChainExportMeta{
		SnapshotVersion: "causal-chain-snapshot-v1",
		GeneratedAt:     time.Now().UTC(),
		ReportCount:     len(reports),
		TotalEvents:     len(events),
		Redacted:        redacted,
	}
	if retention > 0 {
		meta.RetentionUntil = meta.GeneratedAt.Add(retention).UTC()
	}
	meta.Checksum = causalChainChecksum(events, aggregations)

	return CausalChainSnapshot{
		Meta:         meta,
		Events:       events,
		Aggregations: aggregations,
		Summary:      summary,
	}
}

func redactExportedEvent(ee CausalChainExportedEvent, rules RedactionRule) CausalChainExportedEvent {
	if redactField("characterId", rules) {
		ee.CharacterID = redactContent(ee.CharacterID)
	}
	if redactField("userId", rules) {
		ee.RequestID = redactContent(ee.RequestID)
	}
	if redactField("requestId", rules) {
		ee.RequestID = redactContent(ee.RequestID)
	}
	if redactField("interactionId", rules) {
		ee.InteractionID = redactContent(ee.InteractionID)
		ee.ID = redactContent(ee.ID)
		ee.ParentID = redactContent(ee.ParentID)
	}
	return ee
}

func causalChainChecksum(events []CausalChainExportedEvent, aggs []MetricAggregation) string {
	parts := make([]string, 0, len(events)+len(aggs)+1)
	parts = append(parts, "causal-chain-snapshot-v1")
	for _, e := range events {
		parts = append(parts, fmt.Sprintf("%d:%s:%s:%s", e.Index, e.Kind, e.ID, e.Status))
	}
	sort.Strings(parts)
	for _, a := range aggs {
		parts = append(parts, fmt.Sprintf("%s:%d:%d", a.Name, a.Count, a.Sum))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])[:16]
}
