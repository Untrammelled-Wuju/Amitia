package mindruntime

import (
	"context"
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
)

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
