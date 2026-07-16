package mindruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

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
