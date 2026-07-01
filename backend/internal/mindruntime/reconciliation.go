package mindruntime

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type ReconciliationTarget string

const (
	ReconciliationSQLiteQdrant         ReconciliationTarget = "sqlite_qdrant"
	ReconciliationSQLiteSurrealDB      ReconciliationTarget = "sqlite_surrealdb"
	ReconciliationInteractionRunMsg    ReconciliationTarget = "interactionrun_messages"
	ReconciliationOutboxSideEffect     ReconciliationTarget = "outbox_side_effect"
	ReconciliationLeaseDelivery        ReconciliationTarget = "lease_delivery"
	ReconciliationTombstoneDerivedData ReconciliationTarget = "tombstone_derived_data"
)

type ReconciliationStrategy string

const (
	StrategyAutoRebuild    ReconciliationStrategy = "auto_rebuild"
	StrategyReindex        ReconciliationStrategy = "reindex"
	StrategyLogicalInvalid ReconciliationStrategy = "logical_invalidate"
	StrategyReleaseLease   ReconciliationStrategy = "release_lease"
	StrategyRetry          ReconciliationStrategy = "retry"
	StrategyCompensate     ReconciliationStrategy = "compensate"
	StrategyManualConfirm  ReconciliationStrategy = "manual_confirm"
)

type ReconciliationStatus string

const (
	ReconciliationStatusIdle       ReconciliationStatus = "idle"
	ReconciliationStatusRunning    ReconciliationStatus = "running"
	ReconciliationStatusPaused     ReconciliationStatus = "paused"
	ReconciliationStatusCompleted  ReconciliationStatus = "completed"
	ReconciliationStatusCancelled  ReconciliationStatus = "cancelled"
)
type ReconciliationScan struct {
ID             string                `json:"id"`
Target         ReconciliationTarget  `json:"target"`
Strategy       ReconciliationStrategy `json:"strategy"`
Status         ReconciliationStatus  `json:"status"`
StartedAt      time.Time             `json:"startedAt"`
EndedAt        time.Time             `json:"endedAt,omitempty"`
CursorID       string                `json:"cursorId,omitempty"`
BatchSize      int                   `json:"batchSize"`
TotalScanned   int64                 `json:"totalScanned"`
DiffsFound     int64                 `json:"diffsFound"`
DiffsRepaired  int64                 `json:"diffsRepaired"`
DiffsSkipped   int64                 `json:"diffsSkipped"`
BudgetUsedMS   int64                 `json:"budgetUsedMs"`
BudgetLimitMS  int64                 `json:"budgetLimitMs"`
Diffs          []ReconciliationDiff  `json:"diffs,omitempty"`
}
type ReconciliationDiff struct {
ID            string                  `json:"id"`
ScanID        string                  `json:"scanId"`
Source        string                  `json:"source"`
Target        string                  `json:"target"`
DiffType      string                  `json:"diffType"`
SourceKey     string                  `json:"sourceKey"`
TargetKey     string                  `json:"targetKey"`
Description   string                  `json:"description"`
Severity      string                  `json:"severity"`
AutoRepairable bool                   `json:"autoRepairable"`
RepairAction  string                  `json:"repairAction,omitempty"`
Repaired      bool                    `json:"repaired"`
RepairError   string                  `json:"repairError,omitempty"`
FoundAt       time.Time               `json:"foundAt"`
RepairedAt    time.Time               `json:"repairedAt,omitempty"`
}
type ReconciliationConfig struct {
BatchSize       int           `json:"batchSize"`
PauseAfterBatch bool          `json:"pauseAfterBatch"`
BudgetLimitMS   int64         `json:"budgetLimitMs"`
MaxConcurrency  int           `json:"maxConcurrency"`
AutoRepair      bool          `json:"autoRepair"`
RetryCount      int           `json:"retryCount"`
RetryDelay      time.Duration `json:"retryDelay"`
}
func DefaultReconciliationConfig() ReconciliationConfig {
return ReconciliationConfig{
BatchSize:       50,
PauseAfterBatch: false,
BudgetLimitMS:   5000,
MaxConcurrency:  2,
AutoRepair:      false,
RetryCount:      3,
RetryDelay:      100 * time.Millisecond,
}
}
type ReconciliationEngine struct {
config   ReconciliationConfig
scans    map[string]*ReconciliationScan
	scanCtr  int64
status   ReconciliationStatus
mu       sync.RWMutex
	activeScans int32
control  chan struct{}
done     chan struct{}
}
func NewReconciliationEngine(config ReconciliationConfig) *ReconciliationEngine {
return &ReconciliationEngine{
config:  config,
scans:   make(map[string]*ReconciliationScan),
status:  ReconciliationStatusIdle,
control: make(chan struct{}, 1),
done:    make(chan struct{}, 1),
}
}
func (e *ReconciliationEngine) StartScan(target ReconciliationTarget, strategy ReconciliationStrategy, startCursor string) *ReconciliationScan {
e.mu.Lock()
defer e.mu.Unlock()
scan := &ReconciliationScan{
ID:        fmt.Sprintf("scan-%s-%d", time.Now().UTC().Format("20060102T150405.000"), atomic.AddInt64(&e.scanCtr, 1)),
Target:    target,
Strategy:  strategy,
Status:    ReconciliationStatusRunning,
StartedAt: time.Now().UTC(),
CursorID:  startCursor,
BatchSize: e.config.BatchSize,
BudgetLimitMS: e.config.BudgetLimitMS,
}
e.scans[scan.ID] = scan
e.status = ReconciliationStatusRunning
return scan
}
func (e *ReconciliationEngine) UpdateScanProgress(scanID string, scanned, diffs, repaired, skipped int64, budgetUsed int64) {
e.mu.Lock()
defer e.mu.Unlock()
scan, ok := e.scans[scanID]
if !ok {
return
}
scan.TotalScanned += scanned
scan.DiffsFound += diffs
scan.DiffsRepaired += repaired
scan.DiffsSkipped += skipped
scan.BudgetUsedMS += budgetUsed
}
func (e *ReconciliationEngine) UpdateCursor(scanID string, cursor string) {
e.mu.Lock()
defer e.mu.Unlock()
if scan, ok := e.scans[scanID]; ok {
scan.CursorID = cursor
}
}
func (e *ReconciliationEngine) AddDiff(scanID string, diff ReconciliationDiff) {
e.mu.Lock()
defer e.mu.Unlock()
scan, ok := e.scans[scanID]
if !ok {
return
}
scan.Diffs = append(scan.Diffs, diff)
}
func (e *ReconciliationEngine) CompleteScan(scanID string) {
e.mu.Lock()
defer e.mu.Unlock()
scan, ok := e.scans[scanID]
if !ok {
return
}
scan.Status = ReconciliationStatusCompleted
scan.EndedAt = time.Now().UTC()
e.status = ReconciliationStatusIdle
}
func (e *ReconciliationEngine) PauseScan(scanID string) {
e.mu.Lock()
defer e.mu.Unlock()
if scan, ok := e.scans[scanID]; ok {
scan.Status = ReconciliationStatusPaused
}
}
func (e *ReconciliationEngine) ResumeScan(scanID string) {
e.mu.Lock()
defer e.mu.Unlock()
if scan, ok := e.scans[scanID]; ok {
scan.Status = ReconciliationStatusRunning
}
}
func (e *ReconciliationEngine) CancelScan(scanID string) {
e.mu.Lock()
defer e.mu.Unlock()
if scan, ok := e.scans[scanID]; ok {
scan.Status = ReconciliationStatusCancelled
scan.EndedAt = time.Now().UTC()
}
}
func (e *ReconciliationEngine) GetScan(scanID string) *ReconciliationScan {
e.mu.RLock()
defer e.mu.RUnlock()
return e.scans[scanID]
}
func (e *ReconciliationEngine) AllScans() []*ReconciliationScan {
e.mu.RLock()
defer e.mu.RUnlock()
result := make([]*ReconciliationScan, 0, len(e.scans))
for _, scan := range e.scans {
result = append(result, scan)
}
return result
}
func (e *ReconciliationEngine) Status() ReconciliationStatus {
e.mu.RLock()
defer e.mu.RUnlock()
return e.status
}
func (e *ReconciliationEngine) IsBudgetExhausted(scanID string) bool {
e.mu.RLock()
defer e.mu.RUnlock()
scan, ok := e.scans[scanID]
if !ok {
return true
}
if scan.BudgetLimitMS <= 0 {
return false
}
return scan.BudgetUsedMS >= scan.BudgetLimitMS
}
type ConsistencyCheck struct {
ID              string                   `json:"id"`
Events          []TraceEventSummary      `json:"events"`
Evidence        []EvidenceRecord         `json:"evidence"`
Deltas          []StateDelta             `json:"deltas"`
Budget          DebugBudgetSummary           `json:"budget"`
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
EventID      string    `json:"eventId"`
Kind         string    `json:"kind"`
Status       string    `json:"status"`
DurationMs   int64     `json:"durationMs"`
CreatedAt    time.Time `json:"createdAt"`
}
type EvidenceRecord struct {
ID           string    `json:"id"`
Type         string    `json:"type"`
Source       string    `json:"source"`
ContentHash  string    `json:"contentHash"`
Validated    bool      `json:"validated"`
CreatedAt    time.Time `json:"createdAt"`
}
type StateDelta struct {
Field      string      `json:"field"`
OldValue   interface{} `json:"oldValue"`
NewValue   interface{} `json:"newValue"`
Version    int         `json:"version"`
Timestamp  time.Time   `json:"timestamp"`
}
type DebugBudgetSummary struct {
TotalBudget    float64 `json:"totalBudget"`
UsedBudget     float64 `json:"usedBudget"`
Remaining      float64 `json:"remaining"`
RejectedCount  int64   `json:"rejectedCount"`
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
Consistency          ConsistencyCheck        `json:"consistency"`
ReconciliationScans  []*ReconciliationScan   `json:"reconciliationScans"`
ReconciliationStatus ReconciliationStatus   `json:"reconciliationStatus"`
CircuitBreakers      []CircuitBreakerSnapshot `json:"circuitBreakers"`
DependencyHealth     []DependencyHealth      `json:"dependencyHealth"`
RuntimeMetrics       []RuntimeMetric         `json:"runtimeMetrics"`
QueueDepth           int                     `json:"queueDepth"`
ActiveRequests       int                     `json:"activeRequests"`
GeneratedAt          time.Time               `json:"generatedAt"`
}
type SanitizedExport struct {
ExportID    string        `json:"exportId"`
GeneratedAt time.Time     `json:"generatedAt"`
PanelData   DebugPanelData `json:"panelData"`
AuditLog    []AuditEntry  `json:"auditLog"`
Sanitized   bool          `json:"sanitized"`
}
type AuditEntry struct {
Action    string    `json:"action"`
Timestamp time.Time `json:"timestamp"`
Operator  string    `json:"operator"`
Target    string    `json:"target"`
Result    string    `json:"result"`
}
var DefaultReconciliationEngine = NewReconciliationEngine(DefaultReconciliationConfig())
func BuildDebugPanelData() DebugPanelData {
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
scans := DefaultReconciliationEngine.AllScans()
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
ReconciliationStatus: DefaultReconciliationEngine.Status(),
CircuitBreakers:      cbSnapshots,
DependencyHealth:     reports,
RuntimeMetrics:       metrics,
GeneratedAt:          now,
}
}
func BuildSanitizedExport() SanitizedExport {
now := time.Now().UTC()
return SanitizedExport{
ExportID:    "export-" + now.Format("20060102T150405"),
GeneratedAt: now,
PanelData:   BuildDebugPanelData(),
AuditLog:    make([]AuditEntry, 0),
Sanitized:   true,
}
}