package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type IndexOutboxHealth struct {
	Status                   string    `json:"status"`
	PendingUpserts           int64     `json:"pendingUpserts"`
	PendingDeletes           int64     `json:"pendingDeletes"`
	RetryingUpserts          int64     `json:"retryingUpserts"`
	RetryingDeletes          int64     `json:"retryingDeletes"`
	ExhaustedUpserts         int64     `json:"exhaustedUpserts"`
	ExhaustedDeletes         int64     `json:"exhaustedDeletes"`
	ActiveLeases             int64     `json:"activeLeases"`
	OverdueOperations        int64     `json:"overdueOperations"`
	OldestOutstandingSeconds int64     `json:"oldestOutstandingSeconds"`
	ObservedAt               time.Time `json:"observedAt"`
}

func (r *Repository) GetIndexOutboxHealth(ctx context.Context, tenantID string, now time.Time) (IndexOutboxHealth, error) {
	result := IndexOutboxHealth{Status: "healthy", ObservedAt: now.UTC()}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return result, fmt.Errorf("tenant scope required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	maxRetries := r.settings.OutboxMaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultRuntimeSettings().OutboxMaxRetries
	}
	cutoff := now.Add(-15 * time.Minute)
	err := r.db.WithContext(ctx).Raw(`
        SELECT
          COALESCE(SUM(CASE WHEN operation = 'upsert' AND status = 'pending' THEN 1 ELSE 0 END), 0) AS pending_upserts,
          COALESCE(SUM(CASE WHEN operation = 'delete' AND status = 'pending' THEN 1 ELSE 0 END), 0) AS pending_deletes,
          COALESCE(SUM(CASE WHEN operation = 'upsert' AND status = 'failed' AND retry_count < ? THEN 1 ELSE 0 END), 0) AS retrying_upserts,
          COALESCE(SUM(CASE WHEN operation = 'delete' AND status = 'failed' AND retry_count < ? THEN 1 ELSE 0 END), 0) AS retrying_deletes,
          COALESCE(SUM(CASE WHEN operation = 'upsert' AND status = 'failed' AND retry_count >= ? THEN 1 ELSE 0 END), 0) AS exhausted_upserts,
          COALESCE(SUM(CASE WHEN operation = 'delete' AND status = 'failed' AND retry_count >= ? THEN 1 ELSE 0 END), 0) AS exhausted_deletes,
          COALESCE(SUM(CASE WHEN lease_until > ? THEN 1 ELSE 0 END), 0) AS active_leases,
          COALESCE(SUM(CASE WHEN retry_count < ? AND created_at <= ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (lease_until IS NULL OR lease_until <= ?) THEN 1 ELSE 0 END), 0) AS overdue_operations
        FROM knowledge_index_outbox
        WHERE tenant_id = ? AND operation IN ('upsert', 'delete') AND status IN ('pending', 'failed')`,
		maxRetries, maxRetries, maxRetries, maxRetries, now, maxRetries, cutoff, now, now, tenantID,
	).Scan(&result).Error
	if err != nil {
		return result, err
	}
	result.ObservedAt = now.UTC()
	result.Status = "healthy"
	var oldest IndexOutbox
	err = tenantScope(r.db.WithContext(ctx).Model(&IndexOutbox{}), tenantID).
		Where("operation IN ? AND status IN ?", []string{"upsert", "delete"}, []string{"pending", "failed"}).
		Order("created_at ASC, id ASC").First(&oldest).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return result, err
	}
	if err == nil && now.After(oldest.CreatedAt) {
		result.OldestOutstandingSeconds = int64(now.Sub(oldest.CreatedAt).Seconds())
	}
	switch {
	case result.ExhaustedDeletes > 0:
		result.Status = "blocked"
	case result.ExhaustedUpserts > 0 || result.RetryingDeletes > 0 || result.RetryingUpserts > 0 || result.OverdueOperations > 0:
		result.Status = "degraded"
	}
	return result, nil
}

func (s *service) GetIndexOutboxHealth(ctx context.Context, tenantID string) (IndexOutboxHealth, error) {
	return s.repo.GetIndexOutboxHealth(ctx, tenantID, time.Now().UTC())
}
