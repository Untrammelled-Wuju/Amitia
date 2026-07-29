package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type LegacyCounterStore struct {
	db    *sql.DB
	mu    sync.Mutex
	cache map[string]int64
}

func NewLegacyCounterStore(db *sql.DB) *LegacyCounterStore {
	return &LegacyCounterStore{
		db:    db,
		cache: make(map[string]int64),
	}
}

func (s *LegacyCounterStore) Increment(ctx context.Context, metricName string) (int64, error) {
	s.mu.Lock()
	val := s.cache[metricName] + 1
	s.cache[metricName] = val
	s.mu.Unlock()
	if s.db == nil {
		return val, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_legacy_call_counters (metric_name, count, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(metric_name) DO UPDATE SET count = excluded.count, updated_at = excluded.updated_at`,
		metricName, val, now,
	)
	if err != nil {
		return val, fmt.Errorf("legacy_counter_store: increment %s: %w", metricName, err)
	}
	return val, nil
}

func (s *LegacyCounterStore) Get(ctx context.Context, metricName string) int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	val, ok := s.cache[metricName]
	s.mu.Unlock()
	if ok {
		return val
	}
	if s.db == nil {
		return 0
	}
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT count FROM kernel_legacy_call_counters WHERE metric_name = ?`,
		metricName,
	).Scan(&count)
	if err != nil {
		return 0
	}
	s.mu.Lock()
	s.cache[metricName] = count
	s.mu.Unlock()
	return count
}

func (s *LegacyCounterStore) Set(ctx context.Context, metricName string, value int64) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.cache[metricName] = value
	s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_legacy_call_counters (metric_name, count, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(metric_name) DO UPDATE SET count = excluded.count, updated_at = excluded.updated_at`,
		metricName, value, now,
	)
	if err != nil {
		return fmt.Errorf("legacy_counter_store: set %s: %w", metricName, err)
	}
	return nil
}

func (s *LegacyCounterStore) LoadAll(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT metric_name, count FROM kernel_legacy_call_counters`)
	if err != nil {
		return fmt.Errorf("legacy_counter_store: load all: %w", err)
	}
	defer rows.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			return fmt.Errorf("legacy_counter_store: load all scan: %w", err)
		}
		s.cache[name] = count
	}
	return rows.Err()
}

func (s *LegacyCounterStore) Snapshot(ctx context.Context) map[string]int64 {
	if s == nil {
		return map[string]int64{}
	}
	s.mu.Lock()
	result := make(map[string]int64, len(s.cache))
	for k, v := range s.cache {
		result[k] = v
	}
	s.mu.Unlock()
	return result
}
