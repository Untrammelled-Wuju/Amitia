package update

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type GenerationStateRepository struct {
	db *sql.DB
}

func NewGenerationStateRepository(db *sql.DB) *GenerationStateRepository {
	return &GenerationStateRepository{db: db}
}

func (r *GenerationStateRepository) SaveGeneration(ctx context.Context, gen *Generation) error {
	if r == nil || r.db == nil {
		return nil
	}
	activatedAt := ""
	if gen.ActivatedAt != nil {
		activatedAt = gen.ActivatedAt.Format(time.RFC3339Nano)
	}
	stoppedAt := ""
	if gen.StoppedAt != nil {
		stoppedAt = gen.StoppedAt.Format(time.RFC3339Nano)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_generation_states
		(generation_id, extension_id, version, generation_num, state, definition_hash,
		 created_at, activated_at, stopped_at, invocations)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		gen.GenerationID,
		gen.ExtensionID,
		gen.Version,
		gen.Generation,
		string(gen.State),
		gen.DefinitionHash,
		gen.CreatedAt.Format(time.RFC3339Nano),
		activatedAt,
		stoppedAt,
		gen.Invocations,
	)
	if err != nil {
		return fmt.Errorf("generation-state-repo: save %s: %w", gen.GenerationID, err)
	}
	return nil
}

func (r *GenerationStateRepository) UpdateGenerationState(ctx context.Context, generationID string, state string) error {
	if r == nil || r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_generation_states
		SET state = ?, activated_at = CASE WHEN ? = 'active' THEN ? ELSE activated_at END,
		    stopped_at = CASE WHEN ? = 'stopped' THEN ? ELSE stopped_at END
		WHERE generation_id = ?`,
		state, state, now, state, now, generationID,
	)
	if err != nil {
		return fmt.Errorf("generation-state-repo: update state %s: %w", generationID, err)
	}
	return nil
}

func (r *GenerationStateRepository) SetActiveGeneration(ctx context.Context, extensionID, generationID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("generation-state-repo: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		UPDATE kernel_generation_states
		SET state = 'draining'
		WHERE extension_id = ? AND state = 'active'`,
		extensionID,
	)
	if err != nil {
		return fmt.Errorf("generation-state-repo: drain old active: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `
		UPDATE kernel_generation_states
		SET state = 'active', activated_at = ?
		WHERE generation_id = ? AND extension_id = ?`,
		now, generationID, extensionID,
	)
	if err != nil {
		return fmt.Errorf("generation-state-repo: set active %s: %w", generationID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("generation-state-repo: commit set active: %w", err)
	}
	return nil
}

func (r *GenerationStateRepository) GetGeneration(ctx context.Context, generationID string) (*Generation, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT generation_id, extension_id, version, generation_num, state, definition_hash,
		       created_at, activated_at, stopped_at, invocations
		FROM kernel_generation_states
		WHERE generation_id = ?`, generationID)
	return scanGenerationRow(row)
}

func (r *GenerationStateRepository) ListGenerationsByExtension(ctx context.Context, extensionID string) ([]Generation, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT generation_id, extension_id, version, generation_num, state, definition_hash,
		       created_at, activated_at, stopped_at, invocations
		FROM kernel_generation_states
		WHERE extension_id = ?
		ORDER BY generation_num`, extensionID)
	if err != nil {
		return nil, fmt.Errorf("generation-state-repo: list by extension %s: %w", extensionID, err)
	}
	defer rows.Close()
	return scanGenerationRows(rows)
}

func (r *GenerationStateRepository) ListAllGenerations(ctx context.Context) ([]Generation, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT generation_id, extension_id, version, generation_num, state, definition_hash,
		       created_at, activated_at, stopped_at, invocations
		FROM kernel_generation_states
		ORDER BY extension_id, generation_num`)
	if err != nil {
		return nil, fmt.Errorf("generation-state-repo: list all: %w", err)
	}
	defer rows.Close()
	return scanGenerationRows(rows)
}

func (r *GenerationStateRepository) DeleteGeneration(ctx context.Context, generationID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM kernel_generation_states WHERE generation_id = ?`, generationID)
	if err != nil {
		return fmt.Errorf("generation-state-repo: delete %s: %w", generationID, err)
	}
	return nil
}

func (r *GenerationStateRepository) LoadAll(ctx context.Context) ([]Generation, error) {
	return r.ListAllGenerations(ctx)
}

func (r *GenerationStateRepository) UpdateInvocations(ctx context.Context, generationID string, invocations int) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_generation_states SET invocations = ? WHERE generation_id = ?`,
		invocations, generationID,
	)
	if err != nil {
		return fmt.Errorf("generation-state-repo: update invocations %s: %w", generationID, err)
	}
	return nil
}

func scanGenerationRow(row *sql.Row) (*Generation, error) {
	var (
		gen          Generation
		stateStr     string
		createdAtStr string
		activatedAt  sql.NullString
		stoppedAt    sql.NullString
	)
	if err := row.Scan(
		&gen.GenerationID,
		&gen.ExtensionID,
		&gen.Version,
		&gen.Generation,
		&stateStr,
		&gen.DefinitionHash,
		&createdAtStr,
		&activatedAt,
		&stoppedAt,
		&gen.Invocations,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("generation-state-repo: scan row: %w", err)
	}
	gen.State = GenerationState(stateStr)
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			gen.CreatedAt = t
		}
	}
	if activatedAt.Valid && activatedAt.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, activatedAt.String); err == nil {
			gen.ActivatedAt = &t
		}
	}
	if stoppedAt.Valid && stoppedAt.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, stoppedAt.String); err == nil {
			gen.StoppedAt = &t
		}
	}
	return &gen, nil
}

func scanGenerationRows(rows *sql.Rows) ([]Generation, error) {
	var out []Generation
	for rows.Next() {
		var (
			gen          Generation
			stateStr     string
			createdAtStr string
			activatedAt  sql.NullString
			stoppedAt    sql.NullString
		)
		if err := rows.Scan(
			&gen.GenerationID,
			&gen.ExtensionID,
			&gen.Version,
			&gen.Generation,
			&stateStr,
			&gen.DefinitionHash,
			&createdAtStr,
			&activatedAt,
			&stoppedAt,
			&gen.Invocations,
		); err != nil {
			return nil, fmt.Errorf("generation-state-repo: scan row: %w", err)
		}
		gen.State = GenerationState(stateStr)
		if createdAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
				gen.CreatedAt = t
			}
		}
		if activatedAt.Valid && activatedAt.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, activatedAt.String); err == nil {
				gen.ActivatedAt = &t
			}
		}
		if stoppedAt.Valid && stoppedAt.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, stoppedAt.String); err == nil {
				gen.StoppedAt = &t
			}
		}
		out = append(out, gen)
	}
	return out, rows.Err()
}
