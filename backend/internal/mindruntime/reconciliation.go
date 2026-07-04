package mindruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
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
	ReconciliationStatusIdle      ReconciliationStatus = "idle"
	ReconciliationStatusRunning   ReconciliationStatus = "running"
	ReconciliationStatusPaused    ReconciliationStatus = "paused"
	ReconciliationStatusCompleted ReconciliationStatus = "completed"
	ReconciliationStatusCancelled ReconciliationStatus = "cancelled"
)

type ReconciliationScan struct {
	ID            string                 `json:"id"`
	Target        ReconciliationTarget   `json:"target"`
	Strategy      ReconciliationStrategy `json:"strategy"`
	Status        ReconciliationStatus   `json:"status"`
	StartedAt     time.Time              `json:"startedAt"`
	EndedAt       time.Time              `json:"endedAt,omitempty"`
	CursorID      string                 `json:"cursorId,omitempty"`
	BatchSize     int                    `json:"batchSize"`
	TotalScanned  int64                  `json:"totalScanned"`
	DiffsFound    int64                  `json:"diffsFound"`
	DiffsRepaired int64                  `json:"diffsRepaired"`
	DiffsSkipped  int64                  `json:"diffsSkipped"`
	BudgetUsedMS  int64                  `json:"budgetUsedMs"`
	BudgetLimitMS int64                  `json:"budgetLimitMs"`
	Diffs         []ReconciliationDiff   `json:"diffs,omitempty"`
}
type ReconciliationDiff struct {
	ID             string    `json:"id"`
	ScanID         string    `json:"scanId"`
	Source         string    `json:"source"`
	Target         string    `json:"target"`
	DiffType       string    `json:"diffType"`
	SourceKey      string    `json:"sourceKey"`
	TargetKey      string    `json:"targetKey"`
	Description    string    `json:"description"`
	Severity       string    `json:"severity"`
	AutoRepairable bool      `json:"autoRepairable"`
	RepairAction   string    `json:"repairAction,omitempty"`
	Repaired       bool      `json:"repaired"`
	RepairError    string    `json:"repairError,omitempty"`
	FoundAt        time.Time `json:"foundAt"`
	RepairedAt     time.Time `json:"repairedAt,omitempty"`
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
type ReconciliationCheckRequest struct {
	ScanID    string                 `json:"scanId"`
	Target    ReconciliationTarget   `json:"target"`
	Strategy  ReconciliationStrategy `json:"strategy"`
	CursorID  string                 `json:"cursorId,omitempty"`
	BatchSize int                    `json:"batchSize"`
	StartedAt time.Time              `json:"startedAt"`
}
type ReconciliationChecker interface {
	CheckReconciliation(context.Context, ReconciliationCheckRequest) ([]ReconciliationDiff, error)
}
type ReconciliationCheckerFunc func(context.Context, ReconciliationCheckRequest) ([]ReconciliationDiff, error)

func (f ReconciliationCheckerFunc) CheckReconciliation(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationDiff, error) {
	return f(ctx, req)
}

type ReconciliationEntity struct {
	Store       string
	Kind        string
	Key         string
	Version     string
	Status      string
	Hash        string
	Deleted     bool
	LeasedUntil time.Time
	Fields      map[string]string
	References  map[string]string
}

type ReconciliationStateSource interface {
	ListReconciliationEntities(context.Context, ReconciliationCheckRequest) ([]ReconciliationEntity, error)
}

type ReconciliationStateSourceFunc func(context.Context, ReconciliationCheckRequest) ([]ReconciliationEntity, error)

func (f ReconciliationStateSourceFunc) ListReconciliationEntities(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
	return f(ctx, req)
}

type ReconciliationWorkerTarget struct {
	Target   ReconciliationTarget
	Strategy ReconciliationStrategy
	Cursor   string
}

type RuntimeReconciliationChecker struct {
	Source ReconciliationStateSource
	Target ReconciliationStateSource
	Now    func() time.Time
}

func NewRuntimeReconciliationChecker(source ReconciliationStateSource, target ReconciliationStateSource) *RuntimeReconciliationChecker {
	return &RuntimeReconciliationChecker{Source: source, Target: target}
}

func (c *RuntimeReconciliationChecker) CheckReconciliation(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationDiff, error) {
	if c == nil || c.Source == nil || c.Target == nil {
		return nil, errors.New("runtime reconciliation checker requires source and target state sources")
	}
	sourceEntities, err := c.Source.ListReconciliationEntities(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list source reconciliation entities: %w", err)
	}
	targetEntities, err := c.Target.ListReconciliationEntities(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list target reconciliation entities: %w", err)
	}
	now := time.Now().UTC()
	if c.Now != nil {
		now = c.Now().UTC()
	}
	return CompareReconciliationEntities(req, sourceEntities, targetEntities, now), nil
}

type GormReconciliationSource struct {
	DB     *gorm.DB
	Store  string
	Tables []GormReconciliationTable
}

type GormReconciliationTable struct {
	Table             string
	Kind              string
	KeyColumns        []string
	VersionColumn     string
	StatusColumn      string
	DeletedColumn     string
	LeasedUntilColumn string
	HashColumns       []string
	FieldColumns      []string
	ReferenceColumns  map[string]string
}

func (s GormReconciliationSource) ListReconciliationEntities(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
	if s.DB == nil {
		return nil, errors.New("gorm reconciliation source requires db")
	}
	entities := make([]ReconciliationEntity, 0)
	for _, table := range s.Tables {
		if strings.TrimSpace(table.Table) == "" {
			return nil, errors.New("gorm reconciliation table name is required")
		}
		rows := make([]map[string]interface{}, 0)
		query := s.DB.WithContext(ctx).Table(table.Table)
		if req.BatchSize > 0 {
			query = query.Limit(req.BatchSize)
		}
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			entity := table.entityFromRow(s.Store, row)
			if entity.Key == "" {
				continue
			}
			entities = append(entities, entity)
		}
	}
	sort.Slice(entities, func(i, j int) bool {
		return reconciliationEntityIdentity(entities[i]) < reconciliationEntityIdentity(entities[j])
	})
	return entities, nil
}

func (t GormReconciliationTable) entityFromRow(store string, row map[string]interface{}) ReconciliationEntity {
	fields := make(map[string]string)
	for _, column := range t.FieldColumns {
		if value, ok := reconciliationRowValue(row, column); ok {
			fields[column] = reconciliationValueString(value)
		}
	}
	references := make(map[string]string)
	for name, column := range t.ReferenceColumns {
		if value, ok := reconciliationRowValue(row, column); ok {
			references[name] = reconciliationValueString(value)
		}
	}
	keyParts := make([]string, 0, len(t.KeyColumns))
	for _, column := range t.KeyColumns {
		value, _ := reconciliationRowValue(row, column)
		keyParts = append(keyParts, reconciliationValueString(value))
	}
	statusValue, _ := reconciliationRowValue(row, t.StatusColumn)
	versionValue, _ := reconciliationRowValue(row, t.VersionColumn)
	deletedValue, _ := reconciliationRowValue(row, t.DeletedColumn)
	leasedUntilValue, _ := reconciliationRowValue(row, t.LeasedUntilColumn)
	status := reconciliationValueString(statusValue)
	version := reconciliationValueString(versionValue)
	deleted := reconciliationBool(deletedValue)
	leasedUntil := reconciliationTime(leasedUntilValue)
	hashColumns := t.HashColumns
	if len(hashColumns) == 0 {
		hashColumns = t.FieldColumns
	}
	return ReconciliationEntity{
		Store:       store,
		Kind:        firstNonEmpty(t.Kind, t.Table),
		Key:         strings.Join(keyParts, ":"),
		Version:     version,
		Status:      status,
		Hash:        reconciliationRowHash(row, hashColumns),
		Deleted:     deleted,
		LeasedUntil: leasedUntil,
		Fields:      fields,
		References:  references,
	}
}

func CompareReconciliationEntities(req ReconciliationCheckRequest, sources []ReconciliationEntity, targets []ReconciliationEntity, now time.Time) []ReconciliationDiff {
	sourceByKey := make(map[string]ReconciliationEntity, len(sources))
	targetByKey := make(map[string]ReconciliationEntity, len(targets))
	for _, entity := range sources {
		sourceByKey[reconciliationCompareKey(entity)] = normalizeReconciliationEntity(entity)
	}
	for _, entity := range targets {
		targetByKey[reconciliationCompareKey(entity)] = normalizeReconciliationEntity(entity)
	}
	diffs := make([]ReconciliationDiff, 0)
	for key, source := range sourceByKey {
		target, ok := targetByKey[key]
		if !ok {
			diffs = append(diffs, newReconciliationDiff(req, source, ReconciliationEntity{}, "missing_target", "critical", true, string(req.Strategy), "source entity has no matching target entity", now))
			continue
		}
		if source.Deleted && !target.Deleted {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "tombstone_target_present", "critical", true, string(StrategyLogicalInvalid), "source tombstone is deleted but target data is still present", now))
		}
		if source.Version != "" && target.Version != "" && source.Version != target.Version {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "version_mismatch", "critical", true, string(StrategyReindex), "source and target versions differ", now))
		}
		if source.Hash != "" && target.Hash != "" && source.Hash != target.Hash {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "hash_mismatch", "warning", true, string(StrategyReindex), "source and target content hashes differ", now))
		}
		if source.Status != "" && target.Status != "" && source.Status != target.Status {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "status_mismatch", "warning", true, string(StrategyCompensate), "source and target statuses differ", now))
		}
		if !source.LeasedUntil.IsZero() && source.LeasedUntil.Before(now) {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "expired_source_lease", "critical", true, string(StrategyReleaseLease), "source lease expired without being released", now))
		}
		if !target.LeasedUntil.IsZero() && target.LeasedUntil.Before(now) {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "expired_target_lease", "critical", true, string(StrategyReleaseLease), "target lease expired without being released", now))
		}
		diffs = append(diffs, referenceDiffs(req, source, target, now)...)
	}
	for key, target := range targetByKey {
		if _, ok := sourceByKey[key]; !ok {
			diffs = append(diffs, newReconciliationDiff(req, ReconciliationEntity{}, target, "orphan_target", "critical", true, string(StrategyLogicalInvalid), "target entity has no matching source entity", now))
		}
	}
	sort.Slice(diffs, func(i, j int) bool {
		left := diffs[i].DiffType + ":" + diffs[i].SourceKey + ":" + diffs[i].TargetKey
		right := diffs[j].DiffType + ":" + diffs[j].SourceKey + ":" + diffs[j].TargetKey
		return left < right
	})
	return diffs
}

func referenceDiffs(req ReconciliationCheckRequest, source ReconciliationEntity, target ReconciliationEntity, now time.Time) []ReconciliationDiff {
	diffs := make([]ReconciliationDiff, 0)
	for name, sourceRef := range source.References {
		targetRef := target.References[name]
		if sourceRef != "" && targetRef == "" {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "missing_reference", "warning", true, string(StrategyCompensate), "target is missing reference "+name, now))
			continue
		}
		if sourceRef != "" && targetRef != "" && sourceRef != targetRef {
			diffs = append(diffs, newReconciliationDiff(req, source, target, "reference_mismatch", "warning", true, string(StrategyCompensate), "reference "+name+" differs between source and target", now))
		}
	}
	return diffs
}

func newReconciliationDiff(req ReconciliationCheckRequest, source ReconciliationEntity, target ReconciliationEntity, diffType string, severity string, autoRepairable bool, repairAction string, description string, now time.Time) ReconciliationDiff {
	return ReconciliationDiff{
		Source:         firstNonEmpty(source.Store, source.Kind),
		Target:         firstNonEmpty(target.Store, target.Kind, string(req.Target)),
		DiffType:       diffType,
		SourceKey:      reconciliationEntityIdentity(source),
		TargetKey:      reconciliationEntityIdentity(target),
		Description:    description,
		Severity:       severity,
		AutoRepairable: autoRepairable,
		RepairAction:   repairAction,
		FoundAt:        now,
	}
}

func normalizeReconciliationEntity(entity ReconciliationEntity) ReconciliationEntity {
	entity.Store = strings.TrimSpace(entity.Store)
	entity.Kind = strings.TrimSpace(entity.Kind)
	entity.Key = strings.TrimSpace(entity.Key)
	entity.Version = strings.TrimSpace(entity.Version)
	entity.Status = strings.TrimSpace(entity.Status)
	entity.Hash = strings.TrimSpace(entity.Hash)
	if entity.Fields == nil {
		entity.Fields = make(map[string]string)
	}
	if entity.References == nil {
		entity.References = make(map[string]string)
	}
	return entity
}

func reconciliationCompareKey(entity ReconciliationEntity) string {
	entity = normalizeReconciliationEntity(entity)
	return entity.Kind + ":" + entity.Key
}

func reconciliationEntityIdentity(entity ReconciliationEntity) string {
	entity = normalizeReconciliationEntity(entity)
	if entity.Kind == "" && entity.Key == "" {
		return ""
	}
	return entity.Kind + ":" + entity.Key
}

func reconciliationValueString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case []byte:
		return strings.TrimSpace(string(v))
	case time.Time:
		if v.IsZero() {
			return ""
		}
		return v.UTC().Format(time.RFC3339Nano)
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func reconciliationBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case []byte:
		return reconciliationBool(string(v))
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "deleted", "completed":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func reconciliationTime(value interface{}) time.Time {
	switch v := value.(type) {
	case time.Time:
		return v.UTC()
	case []byte:
		return parseReconciliationTime(string(v))
	case string:
		return parseReconciliationTime(v)
	default:
		return time.Time{}
	}
}

func parseReconciliationTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func reconciliationRowHash(row map[string]interface{}, columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	values := make(map[string]string, len(columns))
	for _, column := range columns {
		value, _ := reconciliationRowValue(row, column)
		values[column] = reconciliationValueString(value)
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:])
}

func reconciliationRowValue(row map[string]interface{}, column string) (interface{}, bool) {
	if column == "" {
		return nil, false
	}
	if value, ok := row[column]; ok {
		return value, true
	}
	normalizedColumn := normalizeReconciliationColumn(column)
	for key, value := range row {
		if normalizeReconciliationColumn(key) == normalizedColumn {
			return value, true
		}
	}
	return nil, false
}

func normalizeReconciliationColumn(column string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(column)))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
	config      ReconciliationConfig
	scans       map[string]*ReconciliationScan
	checkers    map[ReconciliationTarget]ReconciliationChecker
	scanCtr     int64
	status      ReconciliationStatus
	mu          sync.RWMutex
	activeScans int32
	control     chan struct{}
	done        chan struct{}
}

func NewReconciliationEngine(config ReconciliationConfig) *ReconciliationEngine {
	return &ReconciliationEngine{
		config:   config,
		scans:    make(map[string]*ReconciliationScan),
		checkers: make(map[ReconciliationTarget]ReconciliationChecker),
		status:   ReconciliationStatusIdle,
		control:  make(chan struct{}, 1),
		done:     make(chan struct{}, 1),
	}
}
func (e *ReconciliationEngine) RegisterChecker(target ReconciliationTarget, checker ReconciliationChecker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if checker == nil {
		delete(e.checkers, target)
		return
	}
	e.checkers[target] = checker
}
func (e *ReconciliationEngine) RegisteredTargets() []ReconciliationTarget {
	e.mu.RLock()
	defer e.mu.RUnlock()
	targets := make([]ReconciliationTarget, 0, len(e.checkers))
	for target := range e.checkers {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i] < targets[j]
	})
	return targets
}
func (e *ReconciliationEngine) RunScan(ctx context.Context, target ReconciliationTarget, strategy ReconciliationStrategy, startCursor string) (*ReconciliationScan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	scan := e.StartScan(target, strategy, startCursor)
	e.mu.RLock()
	checker := e.checkers[target]
	e.mu.RUnlock()
	if checker == nil {
		e.FailScan(scan.ID, errors.New("reconciliation checker not registered"))
		return scan, errors.New("reconciliation checker not registered")
	}
	started := time.Now()
	diffs, err := checker.CheckReconciliation(ctx, ReconciliationCheckRequest{
		ScanID:    scan.ID,
		Target:    target,
		Strategy:  strategy,
		CursorID:  startCursor,
		BatchSize: scan.BatchSize,
		StartedAt: scan.StartedAt,
	})
	budgetUsed := time.Since(started).Milliseconds()
	if err != nil {
		e.FailScan(scan.ID, err)
		return scan, err
	}
	for i := range diffs {
		if diffs[i].ID == "" {
			diffs[i].ID = fmt.Sprintf("diff-%s-%d", scan.ID, i+1)
		}
		diffs[i].ScanID = scan.ID
		if diffs[i].FoundAt.IsZero() {
			diffs[i].FoundAt = time.Now().UTC()
		}
		e.AddDiff(scan.ID, diffs[i])
	}
	e.UpdateScanProgress(scan.ID, int64(len(diffs)), int64(len(diffs)), 0, 0, budgetUsed)
	e.CompleteScan(scan.ID)
	return e.GetScan(scan.ID), nil
}
func (e *ReconciliationEngine) StartScan(target ReconciliationTarget, strategy ReconciliationStrategy, startCursor string) *ReconciliationScan {
	e.mu.Lock()
	defer e.mu.Unlock()
	scan := &ReconciliationScan{
		ID:            fmt.Sprintf("scan-%s-%d", time.Now().UTC().Format("20060102T150405.000"), atomic.AddInt64(&e.scanCtr, 1)),
		Target:        target,
		Strategy:      strategy,
		Status:        ReconciliationStatusRunning,
		StartedAt:     time.Now().UTC(),
		CursorID:      startCursor,
		BatchSize:     e.config.BatchSize,
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
func (e *ReconciliationEngine) FailScan(scanID string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	scan, ok := e.scans[scanID]
	if !ok {
		return
	}
	scan.Status = ReconciliationStatusCancelled
	scan.EndedAt = time.Now().UTC()
	if err != nil {
		scan.DiffsSkipped++
		scan.Diffs = append(scan.Diffs, ReconciliationDiff{
			ID:          fmt.Sprintf("diff-%s-error", scanID),
			ScanID:      scanID,
			Source:      "reconciliation",
			Target:      string(scan.Target),
			DiffType:    "scan_error",
			Description: err.Error(),
			Severity:    "critical",
			FoundAt:     time.Now().UTC(),
		})
	}
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

func (e *ReconciliationEngine) RunWorker(ctx context.Context, interval time.Duration, targets []ReconciliationWorkerTarget) {
	if e == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if interval <= 0 {
		interval = time.Minute
	}
	run := func() {
		for _, target := range targets {
			if target.Target == "" {
				continue
			}
			strategy := target.Strategy
			if strategy == "" {
				strategy = StrategyManualConfirm
			}
			_, _ = e.RunScan(ctx, target.Target, strategy, target.Cursor)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func DefaultReconciliationWorkerTargets() []ReconciliationWorkerTarget {
	return []ReconciliationWorkerTarget{
		{Target: ReconciliationTombstoneDerivedData, Strategy: StrategyLogicalInvalid},
		{Target: ReconciliationLeaseDelivery, Strategy: StrategyReleaseLease},
		{Target: ReconciliationOutboxSideEffect, Strategy: StrategyCompensate},
		{Target: ReconciliationInteractionRunMsg, Strategy: StrategyCompensate},
	}
}

func RegisterDefaultRuntimeReconciliationCheckers(engine *ReconciliationEngine, db *gorm.DB) error {
	if engine == nil {
		return errors.New("reconciliation engine is nil")
	}
	if db == nil {
		return errors.New("reconciliation db is nil")
	}
	engine.RegisterChecker(ReconciliationTombstoneDerivedData, NewTombstoneDerivedDataReconciliationChecker(db))
	registerGormReconciliationChecker(engine, db, ReconciliationLeaseDelivery, leaseDeliverySource(db), deliveryIntentSource(db))
	registerGormReconciliationChecker(engine, db, ReconciliationOutboxSideEffect, outboxSideEffectSource(db), outboxChannelReceiptSource(db))
	registerGormReconciliationChecker(engine, db, ReconciliationInteractionRunMsg, interactionRunSource(db), interactionMessageSource(db))
	return nil
}

func registerGormReconciliationChecker(engine *ReconciliationEngine, db *gorm.DB, target ReconciliationTarget, source GormReconciliationSource, targetSource GormReconciliationSource) {
	source.Tables = existingReconciliationTables(db, source.Tables)
	targetSource.Tables = existingReconciliationTables(db, targetSource.Tables)
	if len(source.Tables) == 0 || len(targetSource.Tables) == 0 {
		return
	}
	engine.RegisterChecker(target, NewRuntimeReconciliationChecker(source, targetSource))
}

func existingReconciliationTables(db *gorm.DB, tables []GormReconciliationTable) []GormReconciliationTable {
	existing := make([]GormReconciliationTable, 0, len(tables))
	for _, table := range tables {
		if strings.TrimSpace(table.Table) != "" && db.Migrator().HasTable(table.Table) {
			existing = append(existing, table)
		}
	}
	return existing
}

func leaseDeliverySource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:             "outbox_records",
				Kind:              "outbox",
				KeyColumns:        []string{"id"},
				StatusColumn:      "status",
				VersionColumn:     "retry_count",
				LeasedUntilColumn: "leased_until",
				HashColumns:       []string{"aggregate_id", "event_type", "status", "retry_count", "last_error"},
				FieldColumns:      []string{"aggregate_id", "event_type", "status", "last_error"},
			},
		},
	}
}

func outboxSideEffectSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "outbox_records",
				Kind:          "outbox_side_effect",
				KeyColumns:    []string{"aggregate_id", "event_type"},
				StatusColumn:  "status",
				VersionColumn: "retry_count",
				HashColumns:   []string{"payload", "status", "retry_count"},
				FieldColumns:  []string{"aggregate_id", "event_type", "status", "last_error"},
			},
		},
	}
}

func interactionRunSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "interaction_records",
				Kind:          "interaction",
				KeyColumns:    []string{"request_id"},
				StatusColumn:  "status",
				VersionColumn: "status_version",
				HashColumns:   []string{"status", "status_version", "result_ref", "error_code", "error_message"},
				FieldColumns:  []string{"user_id", "character_id", "conversation_id", "request_id", "status", "result_ref"},
			},
		},
	}
}

func interactionMessageSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "messages",
				Kind:          "interaction",
				KeyColumns:    []string{"request_id"},
				StatusColumn:  "status",
				VersionColumn: "updated_at",
				HashColumns:   []string{"content", "status", "updated_at"},
				FieldColumns:  []string{"conversation_id", "character_id", "request_id", "status"},
			},
		},
	}
}

type TombstoneDerivedDataReconciliationChecker struct {
	DB               *gorm.DB
	ExpectedStorages []string
	Now              func() time.Time
}

func NewTombstoneDerivedDataReconciliationChecker(db *gorm.DB) *TombstoneDerivedDataReconciliationChecker {
	return &TombstoneDerivedDataReconciliationChecker{
		DB:               db,
		ExpectedStorages: []string{"qdrant", "surrealdb", "cache", "summaries", "reflections", "traces"},
	}
}

func (c *TombstoneDerivedDataReconciliationChecker) CheckReconciliation(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationDiff, error) {
	if c == nil || c.DB == nil {
		return nil, errors.New("tombstone reconciliation checker requires db")
	}
	if !c.DB.Migrator().HasTable("deletion_tombstones") {
		return nil, nil
	}
	now := time.Now().UTC()
	if c.Now != nil {
		now = c.Now().UTC()
	}
	var tombstones []DeletionTombstoneModel
	query := c.DB.WithContext(ctx).Table("deletion_tombstones").Order("requested_at ASC")
	if req.BatchSize > 0 {
		query = query.Limit(req.BatchSize)
	}
	if err := query.Find(&tombstones).Error; err != nil {
		return nil, err
	}
	diffs := make([]ReconciliationDiff, 0)
	for _, tombstone := range tombstones {
		diffs = append(diffs, c.missingOutboxDiffs(ctx, req, tombstone, now)...)
		diffs = append(diffs, c.missingRecalculationDiffs(ctx, req, tombstone, now)...)
	}
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].DiffType+diffs[i].SourceKey+diffs[i].TargetKey < diffs[j].DiffType+diffs[j].SourceKey+diffs[j].TargetKey
	})
	return diffs, nil
}

func (c *TombstoneDerivedDataReconciliationChecker) missingOutboxDiffs(ctx context.Context, req ReconciliationCheckRequest, tombstone DeletionTombstoneModel, now time.Time) []ReconciliationDiff {
	if !c.DB.Migrator().HasTable("data_lifecycle_outbox_cleanup_items") {
		return []ReconciliationDiff{newReconciliationDiff(req, tombstoneEntity(tombstone), ReconciliationEntity{}, "missing_cleanup_table", "critical", true, string(StrategyLogicalInvalid), "cleanup outbox table is missing for tombstone derived data", now)}
	}
	diffs := make([]ReconciliationDiff, 0)
	for _, storage := range c.ExpectedStorages {
		var count int64
		err := c.DB.WithContext(ctx).Table("data_lifecycle_outbox_cleanup_items").
			Where("target_id = ? AND storage = ?", tombstone.TargetID, storage).
			Count(&count).Error
		if err != nil {
			diffs = append(diffs, newReconciliationDiff(req, tombstoneEntity(tombstone), ReconciliationEntity{Store: "sqlite", Kind: "cleanup", Key: tombstone.TargetID + ":" + storage}, "cleanup_query_error", "critical", false, "", err.Error(), now))
			continue
		}
		if count == 0 {
			diffs = append(diffs, newReconciliationDiff(req, tombstoneEntity(tombstone), ReconciliationEntity{Store: "sqlite", Kind: "cleanup", Key: tombstone.TargetID + ":" + storage}, "missing_cleanup_item", "critical", true, string(StrategyLogicalInvalid), "tombstone is missing cleanup item for "+storage, now))
		}
	}
	return diffs
}

func (c *TombstoneDerivedDataReconciliationChecker) missingRecalculationDiffs(ctx context.Context, req ReconciliationCheckRequest, tombstone DeletionTombstoneModel, now time.Time) []ReconciliationDiff {
	if !c.DB.Migrator().HasTable("data_lifecycle_recalculation_tasks") {
		return nil
	}
	zones := expectedRecalculationZones(DeletionScope(tombstone.Scope))
	diffs := make([]ReconciliationDiff, 0)
	for _, zone := range zones {
		var count int64
		err := c.DB.WithContext(ctx).Table("data_lifecycle_recalculation_tasks").
			Where("target_id = ? AND affected_zone = ?", tombstone.TargetID, zone).
			Count(&count).Error
		if err != nil {
			diffs = append(diffs, newReconciliationDiff(req, tombstoneEntity(tombstone), ReconciliationEntity{Store: "sqlite", Kind: "recalculation", Key: tombstone.TargetID + ":" + zone}, "recalculation_query_error", "warning", false, "", err.Error(), now))
			continue
		}
		if count == 0 {
			diffs = append(diffs, newReconciliationDiff(req, tombstoneEntity(tombstone), ReconciliationEntity{Store: "sqlite", Kind: "recalculation", Key: tombstone.TargetID + ":" + zone}, "missing_recalculation_task", "warning", true, string(StrategyRetry), "tombstone is missing recalculation task for "+zone, now))
		}
	}
	return diffs
}

func expectedRecalculationZones(scope DeletionScope) []string {
	switch scope {
	case DeletionScopeAll:
		return []string{"belief", "relationship", "memory"}
	case DeletionScopeBelief:
		return []string{"belief"}
	case DeletionScopeRelation:
		return []string{"relationship"}
	case DeletionScopeMemory:
		return []string{"memory"}
	default:
		return nil
	}
}

func tombstoneEntity(tombstone DeletionTombstoneModel) ReconciliationEntity {
	return ReconciliationEntity{
		Store:  "sqlite",
		Kind:   "tombstone",
		Key:    tombstone.TargetID,
		Status: tombstone.Status,
		Fields: map[string]string{
			"target_type": tombstone.TargetType,
			"scope":       tombstone.Scope,
		},
	}
}

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
