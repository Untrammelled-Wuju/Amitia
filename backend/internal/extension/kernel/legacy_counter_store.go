package kernel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

type LegacyCounterStore struct {
	db    *sql.DB
	mu    sync.Mutex
	cache map[string]int64
}

const (
	legacyZeroWindowStartedMetric = "legacy_zero_window_started_unix_nano"
	legacyZeroWindowReadMetric    = "legacy_zero_window_read_baseline"
	legacyZeroWindowWriteMetric   = "legacy_zero_window_write_baseline"
)

type LegacyZeroWindowProof struct {
	StartedAt     time.Time        `json:"startedAt"`
	ObservedAt    time.Time        `json:"observedAt"`
	ReadBaseline  int64            `json:"readBaseline"`
	ReadCurrent   int64            `json:"readCurrent"`
	WriteBaseline int64            `json:"writeBaseline"`
	WriteCurrent  int64            `json:"writeCurrent"`
	Duration      time.Duration    `json:"duration"`
	ZeroRead      bool             `json:"zeroRead"`
	ZeroWrite     bool             `json:"zeroWrite"`
	Passed        bool             `json:"passed"`
	Metrics       map[string]int64 `json:"metrics"`
}

func NewLegacyCounterStore(db *sql.DB) *LegacyCounterStore {
	return &LegacyCounterStore{
		db:    db,
		cache: make(map[string]int64),
	}
}

func (s *LegacyCounterStore) Increment(ctx context.Context, metricName string) (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("legacy_counter_store: unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	val, loaded := s.cache[metricName]
	if !loaded && s.db != nil {
		err := s.db.QueryRowContext(ctx, `SELECT count FROM kernel_legacy_call_counters WHERE metric_name = ?`, metricName).Scan(&val)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("legacy_counter_store: load %s before increment: %w", metricName, err)
		}
	}
	val++
	if s.db == nil {
		s.cache[metricName] = val
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
	s.cache[metricName] = val
	return val, nil
}

func (s *LegacyCounterStore) Get(ctx context.Context, metricName string) int64 {
	value, _ := s.GetValue(ctx, metricName)
	return value
}

func (s *LegacyCounterStore) GetValue(ctx context.Context, metricName string) (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("legacy_counter_store: unavailable")
	}
	s.mu.Lock()
	val, ok := s.cache[metricName]
	s.mu.Unlock()
	if ok {
		return val, nil
	}
	if s.db == nil {
		return 0, nil
	}
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT count FROM kernel_legacy_call_counters WHERE metric_name = ?`,
		metricName,
	).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("legacy_counter_store: get %s: %w", metricName, err)
	}
	s.mu.Lock()
	s.cache[metricName] = count
	s.mu.Unlock()
	return count, nil
}

func (s *LegacyCounterStore) persistedValue(ctx context.Context, metricName string) (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("legacy_counter_store: unavailable")
	}
	if s.db == nil {
		return s.GetValue(ctx, metricName)
	}
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT count FROM kernel_legacy_call_counters WHERE metric_name = ?`, metricName).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		count = 0
	} else if err != nil {
		return 0, fmt.Errorf("legacy_counter_store: persisted get %s: %w", metricName, err)
	}
	s.mu.Lock()
	s.cache[metricName] = count
	s.mu.Unlock()
	return count, nil
}

func (s *LegacyCounterStore) Set(ctx context.Context, metricName string, value int64) error {
	if s == nil {
		return fmt.Errorf("legacy_counter_store: unavailable")
	}
	if s.db != nil {
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
	}
	s.mu.Lock()
	s.cache[metricName] = value
	s.mu.Unlock()
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

func (s *LegacyCounterStore) BeginZeroWindow(ctx context.Context) error {
	reads, err := s.persistedValue(ctx, "legacy_package_read_calls")
	if err != nil {
		return err
	}
	writes, err := s.persistedValue(ctx, "legacy_package_write_calls")
	if err != nil {
		return err
	}
	started := time.Now().UTC().UnixNano()
	if err := s.Set(ctx, legacyZeroWindowReadMetric, reads); err != nil {
		return err
	}
	if err := s.Set(ctx, legacyZeroWindowWriteMetric, writes); err != nil {
		return err
	}
	return s.Set(ctx, legacyZeroWindowStartedMetric, started)
}

func (s *LegacyCounterStore) ZeroWindowProof(ctx context.Context, minimum time.Duration) (LegacyZeroWindowProof, error) {
	startedNano, err := s.persistedValue(ctx, legacyZeroWindowStartedMetric)
	if err != nil {
		return LegacyZeroWindowProof{}, err
	}
	if startedNano <= 0 {
		return LegacyZeroWindowProof{}, fmt.Errorf("legacy_counter_store: zero window not started")
	}
	readBaseline, err := s.persistedValue(ctx, legacyZeroWindowReadMetric)
	if err != nil {
		return LegacyZeroWindowProof{}, err
	}
	writeBaseline, err := s.persistedValue(ctx, legacyZeroWindowWriteMetric)
	if err != nil {
		return LegacyZeroWindowProof{}, err
	}
	readCurrent, err := s.persistedValue(ctx, "legacy_package_read_calls")
	if err != nil {
		return LegacyZeroWindowProof{}, err
	}
	writeCurrent, err := s.persistedValue(ctx, "legacy_package_write_calls")
	if err != nil {
		return LegacyZeroWindowProof{}, err
	}
	observed := time.Now().UTC()
	started := time.Unix(0, startedNano).UTC()
	duration := observed.Sub(started)
	proof := LegacyZeroWindowProof{StartedAt: started, ObservedAt: observed, ReadBaseline: readBaseline, ReadCurrent: readCurrent, WriteBaseline: writeBaseline, WriteCurrent: writeCurrent, Duration: duration, ZeroRead: readBaseline == readCurrent, ZeroWrite: writeBaseline == writeCurrent, Metrics: s.Snapshot(ctx)}
	proof.Passed = proof.ZeroRead && proof.ZeroWrite && duration >= minimum
	if duration < minimum {
		return proof, fmt.Errorf("legacy_counter_store: zero window duration %s is less than %s", duration, minimum)
	}
	return proof, nil
}
