package canary

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS extension_canary_policies (
			policy_id TEXT PRIMARY KEY,
			extension_id TEXT NOT NULL,
			mode TEXT NOT NULL,
			stages_json TEXT NOT NULL,
			cohort_key TEXT NOT NULL DEFAULT 'character',
			stable_seed TEXT NOT NULL,
			min_observations INTEGER NOT NULL DEFAULT 10,
			min_duration_sec INTEGER NOT NULL DEFAULT 60,
			max_duration_sec INTEGER NOT NULL DEFAULT 3600,
			health_policy_json TEXT,
			abort_policy_json TEXT,
			write_strategy TEXT NOT NULL DEFAULT 'old_only',
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS extension_canary_states (
			canary_id TEXT PRIMARY KEY,
			extension_id TEXT NOT NULL,
			policy_id TEXT NOT NULL,
			old_generation INTEGER NOT NULL,
			new_generation INTEGER NOT NULL,
			status TEXT NOT NULL,
			current_stage INTEGER NOT NULL DEFAULT 0,
			started_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			paused_at DATETIME,
			finished_at DATETIME,
			abort_reason TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS extension_canary_assignments (
			assignment_id TEXT PRIMARY KEY,
			extension_id TEXT NOT NULL,
			contribution_id TEXT,
			cohort_type TEXT NOT NULL,
			cohort_id TEXT NOT NULL,
			generation INTEGER NOT NULL,
			stage_id TEXT,
			assigned_at DATETIME NOT NULL,
			assignment_hash TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS extension_canary_metrics (
			id TEXT PRIMARY KEY,
			extension_id TEXT NOT NULL,
			generation INTEGER NOT NULL,
			stage_id TEXT,
			metric_name TEXT NOT NULL,
			metric_value REAL NOT NULL DEFAULT 0,
			sample_count INTEGER NOT NULL DEFAULT 0,
			window_start DATETIME NOT NULL,
			window_end DATETIME NOT NULL,
			baseline_value REAL NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'normal'
		)`,
		`CREATE TABLE IF NOT EXISTS extension_generation_routes (
			id TEXT PRIMARY KEY,
			extension_id TEXT NOT NULL,
			contribution_id TEXT,
			cohort_type TEXT NOT NULL,
			cohort_id TEXT NOT NULL,
			generation INTEGER NOT NULL,
			stage_id TEXT,
			reason TEXT NOT NULL DEFAULT 'fallback',
			assigned_at DATETIME NOT NULL,
			UNIQUE(extension_id, cohort_type, cohort_id)
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("failed to execute schema statement: %v", err)
		}
	}

	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func makeTestHealthPolicy() CanaryHealthPolicy {
	return CanaryHealthPolicy{
		MaximumErrorRate:         0.1,
		MaximumRelativeErrorRate: 0.5,
		MaximumP95Latency:        500 * time.Millisecond,
		MaximumLatencyRegression: 0.5,
		MaximumCrashCount:        5,
		MaximumTimeoutRate:       0.1,
		MaximumInvalidResultRate: 0.1,
		RequiredHealthChecks:     []string{},
	}
}

func makeTestCanaryPolicy() *CanaryPolicy {
	return &CanaryPolicy{
		PolicyID:    "test-policy-1",
		ExtensionID: "ext-1",
		Mode:        CanaryModeCanary,
		Stages: []CanaryStage{
			{
				StageID:        "stage-1",
				Mode:           StageModeCanary,
				Percentage:     10,
				AutoAdvance:    true,
				MinDuration:    1 * time.Second,
				MinInvocations: 1,
			},
		},
		CohortKey:       CohortKeyCharacter,
		StableSeed:      "test-seed",
		MinObservations: 10,
		MinDuration:     60 * time.Second,
		MaxDuration:     3600 * time.Second,
		HealthPolicy:    makeTestHealthPolicy(),
		AbortPolicy: CanaryAbortPolicy{
			AbortOnErrorRateExceeded: true,
			AbortOnCrashExceeded:     true,
		},
		WriteStrategy: "dual_write",
	}
}

func makeTestCanaryState() *CanaryState {
	return &CanaryState{
		CanaryID:      "canary-1",
		ExtensionID:   "ext-1",
		PolicyID:      "test-policy-1",
		OldGeneration: 1,
		NewGeneration: 2,
		Status:        CanaryStatusCreated,
		CurrentStage:  0,
	}
}

func TestCohortResolver(t *testing.T) {
	resolver := NewCohortResolver()

	t.Run("stable hash for same input", func(t *testing.T) {
		h1 := resolver.StableHash("seed-1", CohortKeyCharacter, "char-1")
		h2 := resolver.StableHash("seed-1", CohortKeyCharacter, "char-1")
		if h1 != h2 {
			t.Fatalf("expected same hash for same input, got %d and %d", h1, h2)
		}
	})

	t.Run("different hash for different input", func(t *testing.T) {
		h1 := resolver.StableHash("seed-1", CohortKeyCharacter, "char-1")
		h2 := resolver.StableHash("seed-1", CohortKeyCharacter, "char-2")
		if h1 == h2 {
			t.Fatalf("expected different hash for different input, got same value %d", h1)
		}

		h3 := resolver.StableHash("seed-2", CohortKeyCharacter, "char-1")
		if h1 == h3 {
			t.Fatalf("expected different hash for different seed, got same value %d", h1)
		}
	})

	t.Run("percentage routing 10 percent canary", func(t *testing.T) {
		policy := makeTestCanaryPolicy()
		policy.Mode = CanaryModeCanary
		policy.CohortKey = CohortKeyCharacter
		policy.StableSeed = "percentage-seed"
		policy.Stages = []CanaryStage{
			{
				StageID:    "stage-canary",
				Mode:       StageModeCanary,
				Percentage: 10,
			},
		}

		ctx := context.Background()
		const (
			oldGen   int64 = 1
			newGen   int64 = 2
			total          = 1000
			oldGenID       = "ext-canary"
		)
		var stableCount, canaryCount int
		for i := 0; i < total; i++ {
			invCtx := InvocationContext{
				ExtensionID:  oldGenID,
				CharacterID:  fmt.Sprintf("char-%d", i),
				InvocationID: fmt.Sprintf("inv-%d", i),
			}
			gen, _, err := resolver.ResolveCohort(ctx, policy, invCtx, oldGen, newGen)
			if err != nil {
				t.Fatalf("ResolveCohort failed at index %d: %v", i, err)
			}
			switch gen {
			case oldGen:
				stableCount++
			case newGen:
				canaryCount++
			default:
				t.Fatalf("unexpected generation %d at index %d", gen, i)
			}
		}

		if canaryCount == 0 {
			t.Fatalf("expected at least one entity routed to canary cohort")
		}
		if stableCount == 0 {
			t.Fatalf("expected at least one entity routed to stable cohort")
		}

		canaryRatio := float64(canaryCount) / float64(total)
		if canaryRatio < 0.05 || canaryRatio > 0.15 {
			t.Fatalf("canary ratio %.3f out of expected range [0.05, 0.15] for 10%% target", canaryRatio)
		}
	})
}

func TestHealthEvaluator(t *testing.T) {
	evaluator := NewHealthEvaluator()
	ctx := context.Background()

	t.Run("healthy metrics pass evaluation", func(t *testing.T) {
		policy := makeTestHealthPolicy()
		current := map[MetricName]float64{
			MetricErrorRate:     0.01,
			MetricRuntimeCrash:  0,
			MetricTimeout:       0.01,
			MetricInvalidResult: 0.01,
		}
		baseline := map[MetricName]float64{}

		eval := evaluator.Evaluate(ctx, &policy, current, baseline)
		if eval.ShouldAbort {
			t.Fatalf("expected no abort for healthy metrics, got reason: %s", eval.AbortReason)
		}
		if eval.Overall != "healthy" {
			t.Errorf("expected overall healthy, got %s", eval.Overall)
		}
	})

	t.Run("high error rate triggers abort", func(t *testing.T) {
		policy := makeTestHealthPolicy()
		policy.MaximumErrorRate = 0.1
		current := map[MetricName]float64{
			MetricErrorRate: 0.5,
		}
		baseline := map[MetricName]float64{}

		eval := evaluator.Evaluate(ctx, &policy, current, baseline)
		if !eval.ShouldAbort {
			t.Fatalf("expected abort due to high error rate")
		}
		if eval.AbortReason != "error_rate_exceeded_absolute" {
			t.Errorf("expected abort reason error_rate_exceeded_absolute, got %s", eval.AbortReason)
		}
	})

	t.Run("crash count exceeded triggers abort", func(t *testing.T) {
		policy := makeTestHealthPolicy()
		policy.MaximumCrashCount = 5
		current := map[MetricName]float64{
			MetricRuntimeCrash: 10,
		}
		baseline := map[MetricName]float64{}

		eval := evaluator.Evaluate(ctx, &policy, current, baseline)
		if !eval.ShouldAbort {
			t.Fatalf("expected abort due to crash count exceeded")
		}
		if eval.AbortReason != "crash_count_exceeded" {
			t.Errorf("expected abort reason crash_count_exceeded, got %s", eval.AbortReason)
		}
	})

	t.Run("timeout rate exceeded triggers abort", func(t *testing.T) {
		policy := makeTestHealthPolicy()
		policy.MaximumTimeoutRate = 0.1
		current := map[MetricName]float64{
			MetricTimeout: 0.5,
		}
		baseline := map[MetricName]float64{}

		eval := evaluator.Evaluate(ctx, &policy, current, baseline)
		if !eval.ShouldAbort {
			t.Fatalf("expected abort due to timeout rate exceeded")
		}
		if eval.AbortReason != "timeout_rate_exceeded" {
			t.Errorf("expected abort reason timeout_rate_exceeded, got %s", eval.AbortReason)
		}
	})

	t.Run("invalid result rate exceeded triggers abort", func(t *testing.T) {
		policy := makeTestHealthPolicy()
		policy.MaximumInvalidResultRate = 0.1
		current := map[MetricName]float64{
			MetricInvalidResult: 0.5,
		}
		baseline := map[MetricName]float64{}

		eval := evaluator.Evaluate(ctx, &policy, current, baseline)
		if !eval.ShouldAbort {
			t.Fatalf("expected abort due to invalid result rate exceeded")
		}
		if eval.AbortReason != "invalid_result_rate_exceeded" {
			t.Errorf("expected abort reason invalid_result_rate_exceeded, got %s", eval.AbortReason)
		}
	})
}

func TestHealthMetricsCollector(t *testing.T) {
	ctx := context.Background()

	t.Run("record and get metrics", func(t *testing.T) {
		collector := NewHealthMetricsCollector()
		now := time.Now().UTC()
		metric := CanaryMetric{
			ExtensionID: "ext-record",
			Generation:  1,
			StageID:     "stage-1",
			MetricName:  MetricErrorRate,
			MetricValue: 0.05,
			SampleCount: 100,
			WindowStart: now.Add(-1 * time.Minute),
			WindowEnd:   now,
			Status:      MetricStatusNormal,
		}
		if err := collector.RecordMetric(ctx, metric); err != nil {
			t.Fatalf("RecordMetric failed: %v", err)
		}

		got, err := collector.GetMetrics(ctx, "ext-record", 1, now.Add(-2*time.Minute), now.Add(1*time.Minute))
		if err != nil {
			t.Fatalf("GetMetrics failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(got))
		}
		if got[0].MetricValue != 0.05 {
			t.Errorf("expected metric value 0.05, got %f", got[0].MetricValue)
		}
		if got[0].MetricName != MetricErrorRate {
			t.Errorf("expected metric name %s, got %s", MetricErrorRate, got[0].MetricName)
		}
	})

	t.Run("aggregate metrics error rate and p95 latency", func(t *testing.T) {
		collector := NewHealthMetricsCollector()
		now := time.Now().UTC()
		metrics := []CanaryMetric{
			{
				ExtensionID: "ext-agg",
				Generation:  1,
				MetricName:  MetricErrorRate,
				MetricValue: 0.1,
				WindowStart: now.Add(-1 * time.Minute),
				WindowEnd:   now,
			},
			{
				ExtensionID: "ext-agg",
				Generation:  1,
				MetricName:  MetricErrorRate,
				MetricValue: 0.3,
				WindowStart: now.Add(-1 * time.Minute),
				WindowEnd:   now,
			},
			{
				ExtensionID: "ext-agg",
				Generation:  1,
				MetricName:  MetricP95Latency,
				MetricValue: 10,
				WindowStart: now.Add(-1 * time.Minute),
				WindowEnd:   now,
			},
			{
				ExtensionID: "ext-agg",
				Generation:  1,
				MetricName:  MetricP95Latency,
				MetricValue: 20,
				WindowStart: now.Add(-1 * time.Minute),
				WindowEnd:   now,
			},
			{
				ExtensionID: "ext-agg",
				Generation:  1,
				MetricName:  MetricP95Latency,
				MetricValue: 30,
				WindowStart: now.Add(-1 * time.Minute),
				WindowEnd:   now,
			},
		}

		aggregated := collector.AggregateMetrics(ctx, metrics)

		errRate, ok := aggregated[MetricErrorRate]
		if !ok {
			t.Fatalf("expected error rate in aggregated metrics")
		}
		if errRate != 0.2 {
			t.Errorf("expected aggregated error rate 0.2, got %f", errRate)
		}

		p95, ok := aggregated[MetricP95Latency]
		if !ok {
			t.Fatalf("expected p95 latency in aggregated metrics")
		}
		if p95 != 30 {
			t.Errorf("expected aggregated p95 latency 30, got %f", p95)
		}
	})

	t.Run("collect baseline", func(t *testing.T) {
		collector := NewHealthMetricsCollector()
		now := time.Now().UTC()
		metric := CanaryMetric{
			ExtensionID: "ext-baseline",
			Generation:  1,
			MetricName:  MetricErrorRate,
			MetricValue: 0.05,
			SampleCount: 50,
			WindowStart: now.Add(-30 * time.Second),
			WindowEnd:   now,
			Status:      MetricStatusBaseline,
		}
		if err := collector.RecordMetric(ctx, metric); err != nil {
			t.Fatalf("RecordMetric failed: %v", err)
		}

		baseline, err := collector.CollectBaseline(ctx, "ext-baseline", 1, 5*time.Minute)
		if err != nil {
			t.Fatalf("CollectBaseline failed: %v", err)
		}
		errRate, ok := baseline[MetricErrorRate]
		if !ok {
			t.Fatalf("expected error rate in baseline")
		}
		if errRate != 0.05 {
			t.Errorf("expected baseline error rate 0.05, got %f", errRate)
		}
	})
}

func TestDualWriteManager(t *testing.T) {
	manager := NewDualWriteManager()
	ctx := context.Background()

	t.Run("consistency check same data", func(t *testing.T) {
		data, err := json.Marshal(map[string]interface{}{"result": "ok", "count": 1})
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		result, err := manager.CheckConsistency(ctx, data, data)
		if err != nil {
			t.Fatalf("expected no error for same data, got %v", err)
		}
		if !result.Consistent {
			t.Fatalf("expected consistent result for same data")
		}
		if len(result.DifferFields) != 0 {
			t.Errorf("expected no differ fields for same data, got %v", result.DifferFields)
		}
	})

	t.Run("consistency check different data", func(t *testing.T) {
		oldData, err := json.Marshal(map[string]interface{}{"result": "ok", "count": 1})
		if err != nil {
			t.Fatalf("marshal oldData failed: %v", err)
		}
		newData, err := json.Marshal(map[string]interface{}{"result": "ok", "count": 2})
		if err != nil {
			t.Fatalf("marshal newData failed: %v", err)
		}
		result, err := manager.CheckConsistency(ctx, oldData, newData)
		if err == nil {
			t.Fatalf("expected error for different data, got nil")
		}
		if result.Consistent {
			t.Fatalf("expected inconsistent result for different data")
		}
		if len(result.DifferFields) == 0 {
			t.Errorf("expected differ fields for different data, got none")
		}

		found := false
		for _, f := range result.DifferFields {
			if f == "count" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected differ fields to contain 'count', got %v", result.DifferFields)
		}
	})

	t.Run("dual write both writers succeed", func(t *testing.T) {
		policy := DualWritePolicy{
			RequiredIdempotent:  true,
			RecordBothSides:     true,
			ValidateConsistency: true,
			ExternalSideEffect:  false,
		}
		data, err := json.Marshal(map[string]interface{}{"result": "ok"})
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		oldWriter := func(ctx context.Context) (json.RawMessage, error) {
			return data, nil
		}
		newWriter := func(ctx context.Context) (json.RawMessage, error) {
			return data, nil
		}

		result, err := manager.ExecuteDualWrite(ctx, policy, oldWriter, newWriter)
		if err != nil {
			t.Fatalf("expected no error when both writers succeed, got %v", err)
		}
		if !result.Consistent {
			t.Fatalf("expected consistent result when both writers return same data")
		}
		if len(result.Errors) != 0 {
			t.Errorf("expected no errors, got %v", result.Errors)
		}
	})

	t.Run("dual write one writer fails", func(t *testing.T) {
		policy := DualWritePolicy{
			RequiredIdempotent:  true,
			RecordBothSides:     true,
			ValidateConsistency: true,
			ExternalSideEffect:  false,
		}
		data, err := json.Marshal(map[string]interface{}{"result": "ok"})
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		oldWriter := func(ctx context.Context) (json.RawMessage, error) {
			return data, nil
		}
		newWriter := func(ctx context.Context) (json.RawMessage, error) {
			return nil, fmt.Errorf("new writer error")
		}

		result, err := manager.ExecuteDualWrite(ctx, policy, oldWriter, newWriter)
		if err == nil {
			t.Fatalf("expected error when one writer fails, got nil")
		}
		if len(result.Errors) == 0 {
			t.Errorf("expected errors recorded in result when writer fails")
		}
	})
}

func TestStageManager(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCanaryRepository(db)
	manager := NewStageManager(repo)
	ctx := context.Background()

	t.Run("start canary initializes and persists state", func(t *testing.T) {
		state := makeTestCanaryState()
		state.CanaryID = "canary-start-1"
		state.Status = CanaryStatusCreated
		policy := makeTestCanaryPolicy()

		if err := manager.StartCanary(ctx, state, policy); err != nil {
			t.Fatalf("StartCanary failed: %v", err)
		}
		if state.StartedAt.IsZero() {
			t.Fatalf("expected StartedAt to be set after StartCanary")
		}
		if state.UpdatedAt.IsZero() {
			t.Errorf("expected UpdatedAt to be set after StartCanary")
		}
		if state.Status != CanaryStatusCreated {
			t.Errorf("expected status created, got %s", state.Status)
		}

		persisted, err := repo.GetCanaryState(ctx, state.CanaryID)
		if err != nil {
			t.Fatalf("GetCanaryState failed: %v", err)
		}
		if persisted.Status != CanaryStatusCreated {
			t.Errorf("expected persisted status created, got %s", persisted.Status)
		}
		if persisted.StartedAt.IsZero() {
			t.Errorf("expected persisted StartedAt to be set")
		}
	})

	t.Run("pause canary transitions to paused", func(t *testing.T) {
		state := makeTestCanaryState()
		state.CanaryID = "canary-pause-1"
		policy := makeTestCanaryPolicy()

		if err := manager.StartCanary(ctx, state, policy); err != nil {
			t.Fatalf("StartCanary failed: %v", err)
		}
		if err := manager.PauseCanary(ctx, state); err != nil {
			t.Fatalf("PauseCanary failed: %v", err)
		}
		if state.Status != CanaryStatusPaused {
			t.Fatalf("expected status paused, got %s", state.Status)
		}
		if state.PausedAt == nil {
			t.Errorf("expected PausedAt to be set after pause")
		}

		persisted, err := repo.GetCanaryState(ctx, state.CanaryID)
		if err != nil {
			t.Fatalf("GetCanaryState failed: %v", err)
		}
		if persisted.Status != CanaryStatusPaused {
			t.Errorf("expected persisted status paused, got %s", persisted.Status)
		}
	})

	t.Run("resume canary recovers from paused", func(t *testing.T) {
		state := makeTestCanaryState()
		state.CanaryID = "canary-resume-1"
		policy := makeTestCanaryPolicy()

		if err := manager.StartCanary(ctx, state, policy); err != nil {
			t.Fatalf("StartCanary failed: %v", err)
		}
		if err := manager.PauseCanary(ctx, state); err != nil {
			t.Fatalf("PauseCanary failed: %v", err)
		}
		if err := manager.ResumeCanary(ctx, state); err != nil {
			t.Fatalf("ResumeCanary failed: %v", err)
		}
		if state.Status != CanaryStatusCanary {
			t.Fatalf("expected status canary after resume, got %s", state.Status)
		}
		if state.PausedAt != nil {
			t.Errorf("expected PausedAt to be nil after resume")
		}

		persisted, err := repo.GetCanaryState(ctx, state.CanaryID)
		if err != nil {
			t.Fatalf("GetCanaryState failed: %v", err)
		}
		if persisted.Status != CanaryStatusCanary {
			t.Errorf("expected persisted status canary, got %s", persisted.Status)
		}
	})

	t.Run("abort canary transitions to aborted", func(t *testing.T) {
		state := makeTestCanaryState()
		state.CanaryID = "canary-abort-1"
		policy := makeTestCanaryPolicy()

		if err := manager.StartCanary(ctx, state, policy); err != nil {
			t.Fatalf("StartCanary failed: %v", err)
		}
		if err := manager.AbortCanary(ctx, state, "test abort"); err != nil {
			t.Fatalf("AbortCanary failed: %v", err)
		}
		if state.Status != CanaryStatusAborted {
			t.Fatalf("expected status aborted, got %s", state.Status)
		}
		if state.AbortReason != "test abort" {
			t.Errorf("expected abort reason 'test abort', got %s", state.AbortReason)
		}
		if state.FinishedAt == nil {
			t.Errorf("expected FinishedAt to be set after abort")
		}

		persisted, err := repo.GetCanaryState(ctx, state.CanaryID)
		if err != nil {
			t.Fatalf("GetCanaryState failed: %v", err)
		}
		if persisted.Status != CanaryStatusAborted {
			t.Errorf("expected persisted status aborted, got %s", persisted.Status)
		}
		if persisted.AbortReason != "test abort" {
			t.Errorf("expected persisted abort reason 'test abort', got %s", persisted.AbortReason)
		}
	})

	t.Run("commit canary transitions to completed", func(t *testing.T) {
		state := makeTestCanaryState()
		state.CanaryID = "canary-commit-1"
		policy := makeTestCanaryPolicy()

		if err := manager.StartCanary(ctx, state, policy); err != nil {
			t.Fatalf("StartCanary failed: %v", err)
		}
		if err := manager.CommitCanary(ctx, state); err != nil {
			t.Fatalf("CommitCanary failed: %v", err)
		}
		if state.Status != CanaryStatusCompleted {
			t.Fatalf("expected status completed, got %s", state.Status)
		}
		if state.FinishedAt == nil {
			t.Errorf("expected FinishedAt to be set after commit")
		}

		persisted, err := repo.GetCanaryState(ctx, state.CanaryID)
		if err != nil {
			t.Fatalf("GetCanaryState failed: %v", err)
		}
		if persisted.Status != CanaryStatusCompleted {
			t.Errorf("expected persisted status completed, got %s", persisted.Status)
		}
		if persisted.FinishedAt == nil {
			t.Errorf("expected persisted FinishedAt to be set")
		}
	})

	t.Run("check auto abort returns true when unhealthy", func(t *testing.T) {
		state := makeTestCanaryState()
		state.CanaryID = "canary-autoabort-1"
		policy := makeTestCanaryPolicy()

		healthEval := HealthEvaluation{
			ShouldAbort: true,
			AbortReason: "error_rate_exceeded_absolute",
		}
		result, err := manager.CheckAutoAbort(ctx, state, policy, healthEval)
		if err != nil {
			t.Fatalf("CheckAutoAbort failed: %v", err)
		}
		if !result.ShouldAbort {
			t.Fatalf("expected shouldAbort true when health eval fails")
		}
		if result.Trigger != "health_check" {
			t.Errorf("expected trigger health_check, got %s", result.Trigger)
		}
		if result.Reason != "error_rate_exceeded_absolute" {
			t.Errorf("expected reason error_rate_exceeded_absolute, got %s", result.Reason)
		}
	})

	t.Run("check auto abort returns false when healthy", func(t *testing.T) {
		state := makeTestCanaryState()
		state.CanaryID = "canary-autoabort-2"
		policy := makeTestCanaryPolicy()

		healthEval := HealthEvaluation{
			ShouldAbort: false,
		}
		result, err := manager.CheckAutoAbort(ctx, state, policy, healthEval)
		if err != nil {
			t.Fatalf("CheckAutoAbort failed: %v", err)
		}
		if result.ShouldAbort {
			t.Fatalf("expected shouldAbort false when health eval passes")
		}
	})
}
