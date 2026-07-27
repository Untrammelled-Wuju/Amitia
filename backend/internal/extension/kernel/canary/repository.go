package canary

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CanaryRepository struct {
	db *sql.DB
}

func NewCanaryRepository(db *sql.DB) *CanaryRepository {
	return &CanaryRepository{db: db}
}

func (r *CanaryRepository) SavePolicy(ctx context.Context, policy CanaryPolicy) error {
	stagesJSON, err := json.Marshal(policy.Stages)
	if err != nil {
		return fmt.Errorf("canary: marshal stages: %w", err)
	}
	healthJSON, err := json.Marshal(policy.HealthPolicy)
	if err != nil {
		return fmt.Errorf("canary: marshal health policy: %w", err)
	}
	abortJSON, err := json.Marshal(policy.AbortPolicy)
	if err != nil {
		return fmt.Errorf("canary: marshal abort policy: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_canary_policies
			(policy_id, extension_id, mode, stages_json, cohort_key, stable_seed,
			 min_observations, min_duration_sec, max_duration_sec,
			 health_policy_json, abort_policy_json, write_strategy, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(policy_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			mode = excluded.mode,
			stages_json = excluded.stages_json,
			cohort_key = excluded.cohort_key,
			stable_seed = excluded.stable_seed,
			min_observations = excluded.min_observations,
			min_duration_sec = excluded.min_duration_sec,
			max_duration_sec = excluded.max_duration_sec,
			health_policy_json = excluded.health_policy_json,
			abort_policy_json = excluded.abort_policy_json,
			write_strategy = excluded.write_strategy
	`,
		policy.PolicyID, policy.ExtensionID, string(policy.Mode), string(stagesJSON),
		string(policy.CohortKey), policy.StableSeed,
		policy.MinObservations, int(policy.MinDuration/time.Second), int(policy.MaxDuration/time.Second),
		string(healthJSON), string(abortJSON), policy.WriteStrategy, now,
	)
	if err != nil {
		return fmt.Errorf("canary: save policy: %w", err)
	}
	return nil
}

func (r *CanaryRepository) GetPolicy(ctx context.Context, policyID string) (*CanaryPolicy, error) {
	var policy CanaryPolicy
	var mode, stagesJSON, cohortKey, healthJSON, abortJSON string
	var minDurationSec, maxDurationSec int
	err := r.db.QueryRowContext(ctx, `
		SELECT policy_id, extension_id, mode, stages_json, cohort_key, stable_seed,
		       min_observations, min_duration_sec, max_duration_sec,
		       health_policy_json, abort_policy_json, write_strategy
		FROM extension_canary_policies WHERE policy_id = ?
	`, policyID).Scan(
		&policy.PolicyID, &policy.ExtensionID, &mode, &stagesJSON, &cohortKey, &policy.StableSeed,
		&policy.MinObservations, &minDurationSec, &maxDurationSec,
		&healthJSON, &abortJSON, &policy.WriteStrategy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("canary: policy not found: %s", policyID)
		}
		return nil, fmt.Errorf("canary: query policy: %w", err)
	}
	policy.Mode = CanaryMode(mode)
	policy.CohortKey = CohortKeyType(cohortKey)
	policy.MinDuration = time.Duration(minDurationSec) * time.Second
	policy.MaxDuration = time.Duration(maxDurationSec) * time.Second
	if err := json.Unmarshal([]byte(stagesJSON), &policy.Stages); err != nil {
		return nil, fmt.Errorf("canary: unmarshal stages: %w", err)
	}
	if err := json.Unmarshal([]byte(healthJSON), &policy.HealthPolicy); err != nil {
		return nil, fmt.Errorf("canary: unmarshal health policy: %w", err)
	}
	if err := json.Unmarshal([]byte(abortJSON), &policy.AbortPolicy); err != nil {
		return nil, fmt.Errorf("canary: unmarshal abort policy: %w", err)
	}
	return &policy, nil
}

func (r *CanaryRepository) GetPolicyByExtension(ctx context.Context, extensionID string) (*CanaryPolicy, error) {
	var policyID string
	err := r.db.QueryRowContext(ctx, `
		SELECT policy_id FROM extension_canary_policies WHERE extension_id = ? ORDER BY created_at DESC LIMIT 1
	`, extensionID).Scan(&policyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("canary: policy not found for extension: %s", extensionID)
		}
		return nil, fmt.Errorf("canary: query policy by extension: %w", err)
	}
	return r.GetPolicy(ctx, policyID)
}

func (r *CanaryRepository) SaveCanaryState(ctx context.Context, state CanaryState) error {
	now := time.Now().UTC()
	state.UpdatedAt = now
	var pausedAt, finishedAt interface{}
	if state.PausedAt != nil {
		pausedAt = *state.PausedAt
	}
	if state.FinishedAt != nil {
		finishedAt = *state.FinishedAt
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_canary_states
			(canary_id, extension_id, policy_id, old_generation, new_generation,
			 status, current_stage, started_at, updated_at, paused_at, finished_at, abort_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(canary_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			policy_id = excluded.policy_id,
			old_generation = excluded.old_generation,
			new_generation = excluded.new_generation,
			status = excluded.status,
			current_stage = excluded.current_stage,
			updated_at = excluded.updated_at,
			paused_at = excluded.paused_at,
			finished_at = excluded.finished_at,
			abort_reason = excluded.abort_reason
	`,
		state.CanaryID, state.ExtensionID, state.PolicyID, state.OldGeneration, state.NewGeneration,
		string(state.Status), state.CurrentStage, state.StartedAt, now, pausedAt, finishedAt, state.AbortReason,
	)
	if err != nil {
		return fmt.Errorf("canary: save canary state: %w", err)
	}
	return nil
}

func (r *CanaryRepository) GetCanaryState(ctx context.Context, canaryID string) (*CanaryState, error) {
	var state CanaryState
	var status string
	var pausedAt, finishedAt sql.NullTime
	var abortReason sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT canary_id, extension_id, policy_id, old_generation, new_generation,
		       status, current_stage, started_at, updated_at, paused_at, finished_at, abort_reason
		FROM extension_canary_states WHERE canary_id = ?
	`, canaryID).Scan(
		&state.CanaryID, &state.ExtensionID, &state.PolicyID, &state.OldGeneration, &state.NewGeneration,
		&status, &state.CurrentStage, &state.StartedAt, &state.UpdatedAt, &pausedAt, &finishedAt, &abortReason,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("canary: state not found: %s", canaryID)
		}
		return nil, fmt.Errorf("canary: query canary state: %w", err)
	}
	state.Status = CanaryStatus(status)
	if pausedAt.Valid {
		t := pausedAt.Time.UTC()
		state.PausedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		state.FinishedAt = &t
	}
	state.AbortReason = abortReason.String
	return &state, nil
}

func (r *CanaryRepository) GetCanaryStateByExtension(ctx context.Context, extensionID string) (*CanaryState, error) {
	var canaryID string
	err := r.db.QueryRowContext(ctx, `
		SELECT canary_id FROM extension_canary_states WHERE extension_id = ? ORDER BY updated_at DESC LIMIT 1
	`, extensionID).Scan(&canaryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("canary: state not found for extension: %s", extensionID)
		}
		return nil, fmt.Errorf("canary: query canary state by extension: %w", err)
	}
	return r.GetCanaryState(ctx, canaryID)
}

func (r *CanaryRepository) UpdateCanaryStatus(ctx context.Context, canaryID string, status CanaryStatus, abortReason string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_canary_states SET status = ?, abort_reason = ?, updated_at = ? WHERE canary_id = ?
	`, string(status), abortReason, now, canaryID)
	if err != nil {
		return fmt.Errorf("canary: update canary status: %w", err)
	}
	return nil
}

func (r *CanaryRepository) SaveAssignment(ctx context.Context, assignment CohortAssignment) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_canary_assignments
			(assignment_id, extension_id, contribution_id, cohort_type, cohort_id,
			 generation, stage_id, assigned_at, assignment_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(assignment_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			contribution_id = excluded.contribution_id,
			cohort_type = excluded.cohort_type,
			cohort_id = excluded.cohort_id,
			generation = excluded.generation,
			stage_id = excluded.stage_id,
			assigned_at = excluded.assigned_at,
			assignment_hash = excluded.assignment_hash
	`,
		assignment.AssignmentID, assignment.ExtensionID, assignment.ContributionID,
		string(assignment.CohortType), assignment.CohortID,
		assignment.Generation, assignment.StageID, assignment.AssignedAt, assignment.AssignmentHash,
	)
	if err != nil {
		return fmt.Errorf("canary: save assignment: %w", err)
	}
	return nil
}

func (r *CanaryRepository) GetAssignment(ctx context.Context, extensionID, cohortType, cohortID string) (*CohortAssignment, error) {
	var assignment CohortAssignment
	var cohortTypeStr string
	var contributionID, stageID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT assignment_id, extension_id, contribution_id, cohort_type, cohort_id,
		       generation, stage_id, assigned_at, assignment_hash
		FROM extension_canary_assignments
		WHERE extension_id = ? AND cohort_type = ? AND cohort_id = ?
		ORDER BY assigned_at DESC LIMIT 1
	`, extensionID, cohortType, cohortID).Scan(
		&assignment.AssignmentID, &assignment.ExtensionID, &contributionID, &cohortTypeStr, &assignment.CohortID,
		&assignment.Generation, &stageID, &assignment.AssignedAt, &assignment.AssignmentHash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("canary: assignment not found: %s/%s/%s", extensionID, cohortType, cohortID)
		}
		return nil, fmt.Errorf("canary: query assignment: %w", err)
	}
	assignment.CohortType = CohortKeyType(cohortTypeStr)
	assignment.ContributionID = contributionID.String
	assignment.StageID = stageID.String
	return &assignment, nil
}

func (r *CanaryRepository) SaveMetric(ctx context.Context, metric CanaryMetric) error {
	id := uuid.New().String()
	var stageID interface{}
	if metric.StageID != "" {
		stageID = metric.StageID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_canary_metrics
			(id, extension_id, generation, stage_id, metric_name, metric_value,
			 sample_count, window_start, window_end, baseline_value, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		id, metric.ExtensionID, metric.Generation, stageID, string(metric.MetricName), metric.MetricValue,
		metric.SampleCount, metric.WindowStart, metric.WindowEnd, metric.BaselineValue, string(metric.Status),
	)
	if err != nil {
		return fmt.Errorf("canary: save metric: %w", err)
	}
	return nil
}

func (r *CanaryRepository) ListMetrics(ctx context.Context, extensionID string, generation int64, windowStart, windowEnd time.Time) ([]CanaryMetric, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT extension_id, generation, stage_id, metric_name, metric_value,
		       sample_count, window_start, window_end, baseline_value, status
		FROM extension_canary_metrics
		WHERE extension_id = ? AND generation = ? AND window_start >= ? AND window_end <= ?
		ORDER BY window_start ASC
	`, extensionID, generation, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("canary: list metrics: %w", err)
	}
	defer rows.Close()

	var out []CanaryMetric
	for rows.Next() {
		var metric CanaryMetric
		var metricName, status string
		var stageID sql.NullString
		err := rows.Scan(
			&metric.ExtensionID, &metric.Generation, &stageID, &metricName, &metric.MetricValue,
			&metric.SampleCount, &metric.WindowStart, &metric.WindowEnd, &metric.BaselineValue, &status,
		)
		if err != nil {
			return nil, fmt.Errorf("canary: scan metric: %w", err)
		}
		metric.MetricName = MetricName(metricName)
		metric.Status = MetricStatus(status)
		metric.StageID = stageID.String
		out = append(out, metric)
	}
	return out, rows.Err()
}

func (r *CanaryRepository) SaveRoute(ctx context.Context, route GenerationRoute) error {
	id := uuid.New().String()
	var contributionID, stageID interface{}
	if route.ContributionID != "" {
		contributionID = route.ContributionID
	}
	if route.StageID != "" {
		stageID = route.StageID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_generation_routes
			(id, extension_id, contribution_id, cohort_type, cohort_id,
			 generation, stage_id, reason, assigned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id, cohort_type, cohort_id) DO UPDATE SET
			contribution_id = excluded.contribution_id,
			generation = excluded.generation,
			stage_id = excluded.stage_id,
			reason = excluded.reason,
			assigned_at = excluded.assigned_at
	`,
		id, route.ExtensionID, contributionID, string(route.CohortType), route.CohortID,
		route.Generation, stageID, string(route.Reason), route.AssignedAt,
	)
	if err != nil {
		return fmt.Errorf("canary: save route: %w", err)
	}
	return nil
}

func (r *CanaryRepository) GetRoute(ctx context.Context, extensionID, cohortType, cohortID string) (*GenerationRoute, error) {
	var route GenerationRoute
	var cohortTypeStr, reason string
	var contributionID, stageID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT extension_id, contribution_id, cohort_type, cohort_id,
		       generation, stage_id, reason, assigned_at
		FROM extension_generation_routes
		WHERE extension_id = ? AND cohort_type = ? AND cohort_id = ?
	`, extensionID, cohortType, cohortID).Scan(
		&route.ExtensionID, &contributionID, &cohortTypeStr, &route.CohortID,
		&route.Generation, &stageID, &reason, &route.AssignedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("canary: route not found: %s/%s/%s", extensionID, cohortType, cohortID)
		}
		return nil, fmt.Errorf("canary: query route: %w", err)
	}
	route.CohortType = CohortKeyType(cohortTypeStr)
	route.Reason = RoutingReason(reason)
	route.ContributionID = contributionID.String
	route.StageID = stageID.String
	return &route, nil
}
