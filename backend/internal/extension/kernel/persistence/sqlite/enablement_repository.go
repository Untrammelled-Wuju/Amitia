package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/enablement"
)

type StateStoreRepository struct {
	db *sql.DB
}

func NewStateStore(db *sql.DB) *StateStoreRepository {
	return &StateStoreRepository{db: db}
}

func (r *StateStoreRepository) Get(ctx context.Context, subject enablement.StateSubject) (enablement.SubjectState, error) {
	ex := getExecutor(ctx, r.db)
	var data string
	err := ex.QueryRowContext(ctx, `SELECT state_json FROM extension_enablement_overrides WHERE subject_kind = ? AND subject_id = ?`, string(subject.Kind), subject.ID).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return enablement.SubjectState{}, enablement.ErrSubjectNotFound
		}
		return enablement.SubjectState{}, fmt.Errorf("sqlite: query enablement state: %w", err)
	}

	var state enablement.SubjectState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return enablement.SubjectState{}, fmt.Errorf("sqlite: unmarshal enablement state: %w", err)
	}

	return state, nil
}

func (r *StateStoreRepository) SetEnablement(ctx context.Context, subject enablement.StateSubject, state enablement.EnablementState) error {
	return r.upsertState(ctx, subject, func(s *enablement.SubjectState) {
		s.Enablement = state
	})
}

func (r *StateStoreRepository) SetDesiredRuntime(ctx context.Context, subject enablement.StateSubject, state enablement.DesiredRuntimeState) error {
	return r.upsertState(ctx, subject, func(s *enablement.SubjectState) {
		s.DesiredRuntime = state
	})
}

func (r *StateStoreRepository) SetState(ctx context.Context, state enablement.SubjectState) error {
	subject := state.Subject
	state.UpdatedAt = time.Now().UTC()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("sqlite: marshal enablement state: %w", err)
	}

	id := string(subject.Kind) + "/" + subject.ID
	now := time.Now().UTC()

	depReady := boolToInt(state.DependencyReady)
	platSupported := boolToInt(state.PlatformSupported)
	migRequired := boolToInt(state.MigrationRequired)
	var parentEnabled sql.NullInt32
	if state.ParentEnabled != nil {
		parentEnabled = sql.NullInt32{Int32: int32(boolToInt(*state.ParentEnabled)), Valid: true}
	}

	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_enablement_overrides (
			id, subject_kind, subject_id, parent_id, owner_id,
			enablement_state, desired_runtime, installation_state, definition_state,
			actual_runtime, health, circuit, scope_state, permission_state,
			dependency_ready, platform_supported, migration_required, parent_enabled,
			state_json, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(subject_kind, subject_id) DO UPDATE SET
			parent_id = excluded.parent_id,
			owner_id = excluded.owner_id,
			enablement_state = excluded.enablement_state,
			desired_runtime = excluded.desired_runtime,
			installation_state = excluded.installation_state,
			definition_state = excluded.definition_state,
			actual_runtime = excluded.actual_runtime,
			health = excluded.health,
			circuit = excluded.circuit,
			scope_state = excluded.scope_state,
			permission_state = excluded.permission_state,
			dependency_ready = excluded.dependency_ready,
			platform_supported = excluded.platform_supported,
			migration_required = excluded.migration_required,
			parent_enabled = excluded.parent_enabled,
			state_json = excluded.state_json,
			updated_at = excluded.updated_at
	`,
		id,
		string(subject.Kind),
		subject.ID,
		subject.ParentID,
		subject.OwnerID,
		string(state.Enablement),
		string(state.DesiredRuntime),
		string(state.Installation),
		string(state.Definition),
		string(state.ActualRuntime),
		string(state.Health),
		string(state.Circuit),
		string(state.Scope),
		string(state.Permission),
		depReady,
		platSupported,
		migRequired,
		parentEnabled,
		string(data),
		now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert enablement state: %w", err)
	}

	return nil
}

func (r *StateStoreRepository) List(ctx context.Context, kind enablement.StateSubjectKind) ([]enablement.SubjectState, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT state_json FROM extension_enablement_overrides WHERE subject_kind = ? ORDER BY subject_id`, string(kind))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list enablement states: %w", err)
	}
	defer rows.Close()

	var out []enablement.SubjectState
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan enablement state: %w", err)
		}
		var state enablement.SubjectState
		if err := json.Unmarshal([]byte(data), &state); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal enablement state: %w", err)
		}
		out = append(out, state)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate enablement states: %w", err)
	}

	return out, nil
}

func (r *StateStoreRepository) Delete(ctx context.Context, subject enablement.StateSubject) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_enablement_overrides WHERE subject_kind = ? AND subject_id = ?`, string(subject.Kind), subject.ID)
	if err != nil {
		return fmt.Errorf("sqlite: delete enablement state: %w", err)
	}
	return nil
}

func (r *StateStoreRepository) upsertState(ctx context.Context, subject enablement.StateSubject, mutate func(*enablement.SubjectState)) error {
	ex := getExecutor(ctx, r.db)

	var state enablement.SubjectState
	var data string
	err := ex.QueryRowContext(ctx, `SELECT state_json FROM extension_enablement_overrides WHERE subject_kind = ? AND subject_id = ?`, string(subject.Kind), subject.ID).Scan(&data)
	if err == nil {
		if jsonErr := json.Unmarshal([]byte(data), &state); jsonErr != nil {
			return fmt.Errorf("sqlite: unmarshal enablement state: %w", jsonErr)
		}
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("sqlite: query enablement state: %w", err)
	}

	state.Subject = subject
	mutate(&state)

	return r.SetState(ctx, state)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var _ enablement.StateStore = (*StateStoreRepository)(nil)
