package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type FinalGateMetricStatus string

const (
	MetricStatusOpen     FinalGateMetricStatus = "open"
	MetricStatusResolved FinalGateMetricStatus = "resolved"
)

type FinalGateMetricRecord struct {
	ID          string
	MetricName  string
	ResourceID  string
	ExtensionID string
	Generation  int64
	DetectedAt  time.Time
	ResolvedAt  *time.Time
	Status      FinalGateMetricStatus
	Detail      string
}

type FinalGateMetricsRepository struct {
	db *sql.DB
}

func NewFinalGateMetricsRepository(db *sql.DB) *FinalGateMetricsRepository {
	return &FinalGateMetricsRepository{db: db}
}

func (r *FinalGateMetricsRepository) SaveMetric(ctx context.Context, record *FinalGateMetricRecord) error {
	if r == nil || r.db == nil {
		return nil
	}
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.DetectedAt.IsZero() {
		record.DetectedAt = time.Now().UTC()
	}
	if record.Status == "" {
		record.Status = MetricStatusOpen
	}
	var resolvedAt interface{}
	if record.ResolvedAt != nil {
		resolvedAt = record.ResolvedAt.Format(time.RFC3339Nano)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_final_gate_metrics
		(id, metric_name, resource_id, extension_id, generation, detected_at, resolved_at, status, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.MetricName,
		record.ResourceID,
		record.ExtensionID,
		record.Generation,
		record.DetectedAt.Format(time.RFC3339Nano),
		resolvedAt,
		string(record.Status),
		record.Detail,
	)
	if err != nil {
		return fmt.Errorf("final_gate_metrics: save metric %s: %w", record.MetricName, err)
	}
	return nil
}

func (r *FinalGateMetricsRepository) ListOpenMetrics(ctx context.Context) ([]*FinalGateMetricRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, metric_name, resource_id, extension_id, generation, detected_at, resolved_at, status, detail
		FROM kernel_final_gate_metrics
		WHERE status = ?`,
		string(MetricStatusOpen),
	)
	if err != nil {
		return nil, fmt.Errorf("final_gate_metrics: list open metrics: %w", err)
	}
	defer rows.Close()
	return scanFinalGateMetricRows(rows)
}

func (r *FinalGateMetricsRepository) ListMetricsByName(ctx context.Context, metricName string) ([]*FinalGateMetricRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, metric_name, resource_id, extension_id, generation, detected_at, resolved_at, status, detail
		FROM kernel_final_gate_metrics
		WHERE metric_name = ?`,
		metricName,
	)
	if err != nil {
		return nil, fmt.Errorf("final_gate_metrics: list metrics by name %s: %w", metricName, err)
	}
	defer rows.Close()
	return scanFinalGateMetricRows(rows)
}

func (r *FinalGateMetricsRepository) ResolveMetric(ctx context.Context, id string) error {
	if r == nil || r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_final_gate_metrics
		SET status = ?, resolved_at = ?
		WHERE id = ?`,
		string(MetricStatusResolved),
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("final_gate_metrics: resolve metric %s: %w", id, err)
	}
	return nil
}

func (r *FinalGateMetricsRepository) ResolveByName(ctx context.Context, metricName string) error {
	if r == nil || r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_final_gate_metrics
		SET status = ?, resolved_at = ?
		WHERE metric_name = ? AND status = ?`,
		string(MetricStatusResolved),
		now,
		metricName,
		string(MetricStatusOpen),
	)
	if err != nil {
		return fmt.Errorf("final_gate_metrics: resolve by name %s: %w", metricName, err)
	}
	return nil
}

func (r *FinalGateMetricsRepository) CountOpenByName(ctx context.Context, metricName string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM kernel_final_gate_metrics
		WHERE metric_name = ? AND status = ?`,
		metricName,
		string(MetricStatusOpen),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("final_gate_metrics: count open by name %s: %w", metricName, err)
	}
	return count, nil
}

func (r *FinalGateMetricsRepository) CountAllOpen(ctx context.Context) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM kernel_final_gate_metrics
		WHERE status = ?`,
		string(MetricStatusOpen),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("final_gate_metrics: count all open: %w", err)
	}
	return count, nil
}

func (r *FinalGateMetricsRepository) DeleteResolved(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM kernel_final_gate_metrics
		WHERE status = ?`,
		string(MetricStatusResolved),
	)
	if err != nil {
		return fmt.Errorf("final_gate_metrics: delete resolved: %w", err)
	}
	return nil
}

func scanFinalGateMetricRows(rows *sql.Rows) ([]*FinalGateMetricRecord, error) {
	var result []*FinalGateMetricRecord
	for rows.Next() {
		var (
			record     FinalGateMetricRecord
			detectedAt string
			resolvedAt sql.NullString
			status     string
		)
		if err := rows.Scan(
			&record.ID,
			&record.MetricName,
			&record.ResourceID,
			&record.ExtensionID,
			&record.Generation,
			&detectedAt,
			&resolvedAt,
			&status,
			&record.Detail,
		); err != nil {
			return nil, fmt.Errorf("final_gate_metrics: scan row: %w", err)
		}
		if detectedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, detectedAt); err == nil {
				record.DetectedAt = t
			}
		}
		if resolvedAt.Valid && resolvedAt.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, resolvedAt.String); err == nil {
				record.ResolvedAt = &t
			}
		}
		record.Status = FinalGateMetricStatus(status)
		result = append(result, &record)
	}
	return result, rows.Err()
}
