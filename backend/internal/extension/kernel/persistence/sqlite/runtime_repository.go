package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type RuntimeRepository struct {
	db *sql.DB
}

func NewRuntimeRepository(db *sql.DB) *RuntimeRepository {
	return &RuntimeRepository{db: db}
}

func (r *RuntimeRepository) PutInstance(ctx context.Context, instance domain.RuntimeInstance) error {
	metadataJSON := ""
	if instance.Metadata != nil {
		data, err := json.Marshal(instance.Metadata)
		if err != nil {
			return fmt.Errorf("sqlite: marshal instance metadata: %w", err)
		}
		metadataJSON = string(data)
	}

	var startedAt sql.NullTime
	if instance.StartedAt != nil {
		startedAt = sql.NullTime{Time: instance.StartedAt.UTC(), Valid: true}
	}
	var stoppedAt sql.NullTime
	if instance.StoppedAt != nil {
		stoppedAt = sql.NullTime{Time: instance.StoppedAt.UTC(), Valid: true}
	}

	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_runtime_instances (
			instance_id, extension_id, module_id, runtime_type, generation,
			desired_state, actual_state, health, circuit,
			started_at, stopped_at, pid, metadata_json, runtime_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(instance_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			module_id = excluded.module_id,
			runtime_type = excluded.runtime_type,
			generation = excluded.generation,
			desired_state = excluded.desired_state,
			actual_state = excluded.actual_state,
			health = excluded.health,
			circuit = excluded.circuit,
			started_at = excluded.started_at,
			stopped_at = excluded.stopped_at,
			pid = excluded.pid,
			metadata_json = excluded.metadata_json,
			runtime_id = excluded.runtime_id
	`,
		instance.InstanceID,
		string(instance.ExtensionID),
		string(instance.ModuleID),
		string(instance.RuntimeType),
		instance.Generation,
		instance.DesiredState,
		instance.ActualState,
		instance.Health,
		instance.Circuit,
		startedAt,
		stoppedAt,
		0,
		metadataJSON,
		string(instance.RuntimeID),
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert runtime instance: %w", err)
	}

	return nil
}

func (r *RuntimeRepository) GetInstance(ctx context.Context, instanceID string) (domain.RuntimeInstance, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT instance_id, extension_id, module_id, runtime_type, generation,
			desired_state, actual_state, health, circuit,
			started_at, stopped_at, metadata_json, runtime_id
		FROM extension_runtime_instances
		WHERE instance_id = ?
	`, instanceID)

	var inst domain.RuntimeInstance
	var extensionID, moduleID, runtimeType string
	var metadataJSON string
	var runtimeID string
	var startedAt, stoppedAt sql.NullTime

	err := row.Scan(
		&inst.InstanceID,
		&extensionID,
		&moduleID,
		&runtimeType,
		&inst.Generation,
		&inst.DesiredState,
		&inst.ActualState,
		&inst.Health,
		&inst.Circuit,
		&startedAt,
		&stoppedAt,
		&metadataJSON,
		&runtimeID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.RuntimeInstance{}, domain.ErrInvalidExtensionID
		}
		return domain.RuntimeInstance{}, fmt.Errorf("sqlite: query runtime instance: %w", err)
	}

	inst.ExtensionID = domain.ExtensionID(extensionID)
	inst.ModuleID = domain.ModuleID(moduleID)
	inst.RuntimeType = domain.RuntimeType(runtimeType)
	inst.RuntimeID = domain.RuntimeID(runtimeID)

	if startedAt.Valid {
		t := startedAt.Time.UTC()
		inst.StartedAt = &t
	}
	if stoppedAt.Valid {
		t := stoppedAt.Time.UTC()
		inst.StoppedAt = &t
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &inst.Metadata); err != nil {
			return domain.RuntimeInstance{}, fmt.Errorf("sqlite: unmarshal instance metadata: %w", err)
		}
	}

	return inst, nil
}

func (r *RuntimeRepository) ListInstances(ctx context.Context, extensionID domain.ExtensionID) ([]domain.RuntimeInstance, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT instance_id, extension_id, module_id, runtime_type, generation,
			desired_state, actual_state, health, circuit,
			started_at, stopped_at, metadata_json, runtime_id
		FROM extension_runtime_instances
		WHERE extension_id = ?
		ORDER BY instance_id
	`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list runtime instances: %w", err)
	}
	defer rows.Close()

	var out []domain.RuntimeInstance
	for rows.Next() {
		var inst domain.RuntimeInstance
		var extID, modID, rtType string
		var metadataJSON string
		var rtID string
		var startedAt, stoppedAt sql.NullTime

		err := rows.Scan(
			&inst.InstanceID,
			&extID,
			&modID,
			&rtType,
			&inst.Generation,
			&inst.DesiredState,
			&inst.ActualState,
			&inst.Health,
			&inst.Circuit,
			&startedAt,
			&stoppedAt,
			&metadataJSON,
			&rtID,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan runtime instance: %w", err)
		}

		inst.ExtensionID = domain.ExtensionID(extID)
		inst.ModuleID = domain.ModuleID(modID)
		inst.RuntimeType = domain.RuntimeType(rtType)
		inst.RuntimeID = domain.RuntimeID(rtID)

		if startedAt.Valid {
			t := startedAt.Time.UTC()
			inst.StartedAt = &t
		}
		if stoppedAt.Valid {
			t := stoppedAt.Time.UTC()
			inst.StoppedAt = &t
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &inst.Metadata); err != nil {
				return nil, fmt.Errorf("sqlite: unmarshal instance metadata: %w", err)
			}
		}

		out = append(out, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate runtime instances: %w", err)
	}

	return out, nil
}

func (r *RuntimeRepository) DeleteInstance(ctx context.Context, instanceID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_runtime_instances WHERE instance_id = ?`, instanceID)
	if err != nil {
		return fmt.Errorf("sqlite: delete runtime instance: %w", err)
	}
	return nil
}

var _ domain.RuntimeRepository = (*RuntimeRepository)(nil)
