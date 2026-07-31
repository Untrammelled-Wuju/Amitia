// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) CreateEvaluation(ctx context.Context, ev *QualityEvaluation) error {
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if ev.CreatedAt == "" {
		ev.CreatedAt = now
	}
	ev.UpdatedAt = now
	return r.db.WithContext(ctx).Create(ev).Error
}

func (r *GormRepository) UpdateEvaluation(ctx context.Context, ev *QualityEvaluation) error {
	ev.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return r.db.WithContext(ctx).Save(ev).Error
}

func (r *GormRepository) GetEvaluation(ctx context.Context, evaluationID string) (*QualityEvaluation, error) {
	var ev QualityEvaluation
	if err := r.db.WithContext(ctx).Where("id = ?", evaluationID).First(&ev).Error; err != nil {
		return nil, err
	}
	return &ev, nil
}

func (r *GormRepository) GetEvaluationByInput(ctx context.Context, actionRevisionID, profileHash, engineVersion string) (*QualityEvaluation, error) {
	var ev QualityEvaluation
	err := r.db.WithContext(ctx).
		Where("action_revision_id = ? AND profile_hash = ? AND engine_version = ?", actionRevisionID, profileHash, engineVersion).
		Where("execution_status = ?", string(EvalSucceeded)).
		First(&ev).Error
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (r *GormRepository) GetActiveEvaluation(ctx context.Context, processingTaskID, actionKey string) (*QualityEvaluation, error) {
	var evaluations []*QualityEvaluation
	err := r.db.WithContext(ctx).
		Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
		Where("execution_status = ?", string(EvalSucceeded)).
		Order("created_at DESC").
		Find(&evaluations).Error
	if err != nil {
		return nil, err
	}
	if len(evaluations) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return evaluations[0], nil
}

func (r *GormRepository) ListEvaluationsByTask(ctx context.Context, processingTaskID string) ([]*QualityEvaluation, error) {
	var evaluations []*QualityEvaluation
	err := r.db.WithContext(ctx).
		Where("processing_task_id = ?", processingTaskID).
		Order("created_at DESC").
		Find(&evaluations).Error
	if err != nil {
		return nil, err
	}
	return evaluations, nil
}

func (r *GormRepository) ListEvaluationsByAction(ctx context.Context, processingActionID string) ([]*QualityEvaluation, error) {
	var evaluations []*QualityEvaluation
	err := r.db.WithContext(ctx).
		Where("processing_action_id = ?", processingActionID).
		Order("created_at DESC").
		Find(&evaluations).Error
	if err != nil {
		return nil, err
	}
	return evaluations, nil
}

func (r *GormRepository) ListPendingEvaluations(ctx context.Context) ([]*QualityEvaluation, error) {
	return r.ListEvaluationsByStatus(ctx, string(EvalPending))
}

func (r *GormRepository) ListEvaluationsByStatus(ctx context.Context, status string) ([]*QualityEvaluation, error) {
	var evaluations []*QualityEvaluation
	err := r.db.WithContext(ctx).
		Where("execution_status = ?", status).
		Order("created_at ASC").
		Find(&evaluations).Error
	if err != nil {
		return nil, err
	}
	return evaluations, nil
}

func (r *GormRepository) AcquireLease(ctx context.Context, evaluationID, executionID, workerID string, leaseDuration string) (bool, error) {
	now := time.Now().UTC()
	expiresAt := now
	if d, err := time.ParseDuration(leaseDuration); err == nil {
		expiresAt = now.Add(d)
	} else {
		expiresAt = now.Add(5 * time.Minute)
	}
	result := r.db.WithContext(ctx).
		Model(&QualityEvaluation{}).
		Where("id = ? AND (execution_id = ? OR execution_id = '' OR lease_expires_at < ?)", evaluationID, executionID, now.Format(time.RFC3339)).
		Updates(map[string]interface{}{
			"execution_id":     executionID,
			"worker_id":        workerID,
			"lease_expires_at": expiresAt.Format(time.RFC3339),
			"execution_status": string(EvalRunning),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *GormRepository) RenewLease(ctx context.Context, evaluationID, executionID string, leaseDuration string) (bool, error) {
	now := time.Now().UTC()
	expiresAt := now
	if d, err := time.ParseDuration(leaseDuration); err == nil {
		expiresAt = now.Add(d)
	} else {
		expiresAt = now.Add(5 * time.Minute)
	}
	result := r.db.WithContext(ctx).
		Model(&QualityEvaluation{}).
		Where("id = ? AND execution_id = ?", evaluationID, executionID).
		Update("lease_expires_at", expiresAt.Format(time.RFC3339))
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *GormRepository) ReleaseLease(ctx context.Context, evaluationID string) error {
	return r.db.WithContext(ctx).
		Model(&QualityEvaluation{}).
		Where("id = ?", evaluationID).
		Updates(map[string]interface{}{
			"execution_id":     "",
			"worker_id":        "",
			"lease_expires_at": "",
		}).Error
}

func (r *GormRepository) SetActiveEvaluation(ctx context.Context, processingTaskID, actionKey, evaluationID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&QualityEvaluation{}).
			Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
			Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&QualityEvaluation{}).
			Where("id = ?", evaluationID).
			Update("is_active", true).Error
	})
}

func (r *GormRepository) CreateFindings(ctx context.Context, findings []QualityFindingRecord) error {
	if len(findings) == 0 {
		return nil
	}
	for i := range findings {
		if findings[i].ID == "" {
			findings[i].ID = uuid.NewString()
		}
		if findings[i].CreatedAt == "" {
			findings[i].CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return r.db.WithContext(ctx).CreateInBatches(findings, 100).Error
}

func (r *GormRepository) ListFindings(ctx context.Context, evaluationID string) ([]QualityFindingRecord, error) {
	var records []QualityFindingRecord
	err := r.db.WithContext(ctx).
		Where("evaluation_id = ?", evaluationID).
		Order("sort_key ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (r *GormRepository) ListFindingsPaged(ctx context.Context, evaluationID string, severity string, dimension string, offset, limit int) ([]QualityFindingRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&QualityFindingRecord{}).Where("evaluation_id = ?", evaluationID)
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if dimension != "" {
		query = query.Where("dimension_key = ?", dimension)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []QualityFindingRecord
	err := query.Order("sort_key ASC").Offset(offset).Limit(limit).Find(&records).Error
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *GormRepository) CreateDimensionScores(ctx context.Context, scores []QualityDimensionScoreRecord) error {
	if len(scores) == 0 {
		return nil
	}
	for i := range scores {
		if scores[i].ID == "" {
			scores[i].ID = uuid.NewString()
		}
		if scores[i].CreatedAt == "" {
			scores[i].CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return r.db.WithContext(ctx).CreateInBatches(scores, 50).Error
}

func (r *GormRepository) ListDimensionScores(ctx context.Context, evaluationID string) ([]QualityDimensionScoreRecord, error) {
	var records []QualityDimensionScoreRecord
	err := r.db.WithContext(ctx).
		Where("evaluation_id = ?", evaluationID).
		Order("dimension_key ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (r *GormRepository) UpsertGateResult(ctx context.Context, gate *QualityGateResultRecord) error {
	if gate.ID == "" {
		gate.ID = uuid.NewString()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if gate.CreatedAt == "" {
		gate.CreatedAt = now
	}
	gate.UpdatedAt = now

	var existing QualityGateResultRecord
	err := r.db.WithContext(ctx).
		Where("processing_task_id = ? AND snapshot_hash = ?", gate.ProcessingTaskID, gate.SnapshotHash).
		First(&existing).Error
	if err == nil {
		gate.ID = existing.ID
		gate.CreatedAt = existing.CreatedAt
		return r.db.WithContext(ctx).Save(gate).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.WithContext(ctx).Create(gate).Error
}

func (r *GormRepository) GetGateResult(ctx context.Context, processingTaskID string) (*QualityGateResultRecord, error) {
	var record QualityGateResultRecord
	err := r.db.WithContext(ctx).
		Where("processing_task_id = ?", processingTaskID).
		Order("updated_at DESC").
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormRepository) DeleteGateResult(ctx context.Context, processingTaskID string) error {
	return r.db.WithContext(ctx).
		Where("processing_task_id = ?", processingTaskID).
		Delete(&QualityGateResultRecord{}).Error
}

func FindingsToRecords(findings []QualityFinding, evaluationID string) []QualityFindingRecord {
	records := make([]QualityFindingRecord, 0, len(findings))
	for i, f := range findings {
		frameIdxJSON, _ := json.Marshal(f.FrameIndexes)
		framePairsJSON, _ := json.Marshal(f.FramePairs)
		regionsJSON, _ := json.Marshal(f.Regions)

		sortKey := fmt.Sprintf("%02d_%s_%05d_%s",
			severitySortOrder(f.Severity),
			f.Dimension,
			firstFrameIndex(f.FrameIndexes, f.FramePairs),
			f.RuleCode,
		)

		records = append(records, QualityFindingRecord{
			ID:               f.ID,
			EvaluationID:     evaluationID,
			RuleCode:         f.RuleCode,
			RuleVersion:      f.RuleVersion,
			DimensionKey:     f.Dimension,
			Severity:         string(f.Severity),
			HardGate:         f.HardGate,
			FrameIndexesJSON: string(frameIdxJSON),
			FramePairsJSON:   string(framePairsJSON),
			RegionsJSON:      string(regionsJSON),
			MetricName:       f.MetricName,
			ObservedValue:    f.ObservedValue,
			ThresholdValue:   f.ThresholdValue,
			Comparison:       f.Comparison,
			Confidence:       f.Confidence,
			MessageKey:       f.MessageKey,
			Message:          f.Message,
			SuggestedAction:  f.SuggestedAction,
			EvidenceRef:      f.EvidenceRef,
			SortKey:          sortKey,
			CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		})
		_ = i
	}
	return records
}

func RecordsToFindings(records []QualityFindingRecord) []QualityFinding {
	findings := make([]QualityFinding, 0, len(records))
	for _, r := range records {
		var frameIdx []int
		var framePairs []FramePairRef
		var regions []RegionRef
		json.Unmarshal([]byte(r.FrameIndexesJSON), &frameIdx)
		json.Unmarshal([]byte(r.FramePairsJSON), &framePairs)
		json.Unmarshal([]byte(r.RegionsJSON), &regions)

		findings = append(findings, QualityFinding{
			ID:             r.ID,
			RuleCode:       r.RuleCode,
			RuleVersion:    r.RuleVersion,
			Dimension:      r.DimensionKey,
			Severity:       Severity(r.Severity),
			MessageKey:     r.MessageKey,
			Message:        r.Message,
			FrameIndexes:   frameIdx,
			FramePairs:     framePairs,
			Regions:        regions,
			MetricName:     r.MetricName,
			ObservedValue:  r.ObservedValue,
			ThresholdValue: r.ThresholdValue,
			Comparison:     r.Comparison,
			Confidence:     r.Confidence,
			HardGate:       r.HardGate,
			SuggestedAction: r.SuggestedAction,
			EvidenceRef:    r.EvidenceRef,
		})
	}
	return findings
}

func severitySortOrder(s Severity) int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityError:
		return 1
	case SeverityReview:
		return 2
	case SeverityWarning:
		return 3
	case SeverityInfo:
		return 4
	default:
		return 5
	}
}

func firstFrameIndex(indexes []int, pairs []FramePairRef) int {
	min := 99999
	for _, idx := range indexes {
		if idx < min {
			min = idx
		}
	}
	for _, p := range pairs {
		if p.From < min {
			min = p.From
		}
		if p.To < min {
			min = p.To
		}
	}
	if min == 99999 {
		return 0
	}
	return min
}
