package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type UIContributionRecord struct {
	ContributionID  string
	ExtensionID     string
	ModuleID        string
	Kind            string
	SlotID          string
	ContractVersion int
	DefinitionJSON  string
	EnabledOverride sql.NullInt64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SQLiteUIContributionRepository struct {
	db *sql.DB
}

func NewSQLiteUIContributionRepository(db *sql.DB) *SQLiteUIContributionRepository {
	return &SQLiteUIContributionRepository{db: db}
}

func (r *SQLiteUIContributionRepository) PutContribution(ctx context.Context, def *ui_contribution.UIContributionDefinition) error {
	if def == nil {
		return fmt.Errorf("sqlite: nil ui contribution definition")
	}
	data, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("sqlite: marshal ui contribution: %w", err)
	}
	now := time.Now().UTC()
	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_ui_contributions (
			contribution_id, extension_id, module_id, kind, slot_id,
			contract_version, definition_json, enabled_override, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(contribution_id) DO UPDATE SET
			extension_id = excluded.extension_id,
			module_id = excluded.module_id,
			kind = excluded.kind,
			slot_id = excluded.slot_id,
			contract_version = excluded.contract_version,
			definition_json = excluded.definition_json,
			enabled_override = excluded.enabled_override,
			updated_at = excluded.updated_at
	`,
		string(def.ContributionID),
		string(def.ExtensionID),
		string(def.ModuleID),
		string(def.Kind),
		def.Slot.SlotID,
		def.ContractVersion,
		string(data),
		nil,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert ui contribution: %w", err)
	}
	return nil
}

func (r *SQLiteUIContributionRepository) GetContribution(ctx context.Context, contributionID string) (*ui_contribution.UIContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	var data string
	err := ex.QueryRowContext(ctx, `SELECT definition_json FROM extension_ui_contributions WHERE contribution_id = ?`, contributionID).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: ui contribution %s not found: %w", contributionID, err)
		}
		return nil, fmt.Errorf("sqlite: query ui contribution: %w", err)
	}
	var def ui_contribution.UIContributionDefinition
	if err := json.Unmarshal([]byte(data), &def); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal ui contribution: %w", err)
	}
	return &def, nil
}

func (r *SQLiteUIContributionRepository) ListByExtension(ctx context.Context, extensionID string) ([]*ui_contribution.UIContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_ui_contributions WHERE extension_id = ? ORDER BY contribution_id`, extensionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list ui contributions by extension: %w", err)
	}
	defer rows.Close()
	var out []*ui_contribution.UIContributionDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan ui contribution: %w", err)
		}
		var def ui_contribution.UIContributionDefinition
		if err := json.Unmarshal([]byte(data), &def); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal ui contribution: %w", err)
		}
		out = append(out, &def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate ui contributions: %w", err)
	}
	return out, nil
}

func (r *SQLiteUIContributionRepository) ListBySlot(ctx context.Context, slotID string) ([]*ui_contribution.UIContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_ui_contributions WHERE slot_id = ? ORDER BY contribution_id`, slotID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list ui contributions by slot: %w", err)
	}
	defer rows.Close()
	var out []*ui_contribution.UIContributionDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan ui contribution: %w", err)
		}
		var def ui_contribution.UIContributionDefinition
		if err := json.Unmarshal([]byte(data), &def); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal ui contribution: %w", err)
		}
		out = append(out, &def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate ui contributions: %w", err)
	}
	return out, nil
}

func (r *SQLiteUIContributionRepository) ListAll(ctx context.Context) ([]*ui_contribution.UIContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_ui_contributions ORDER BY contribution_id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list all ui contributions: %w", err)
	}
	defer rows.Close()
	var out []*ui_contribution.UIContributionDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan ui contribution: %w", err)
		}
		var def ui_contribution.UIContributionDefinition
		if err := json.Unmarshal([]byte(data), &def); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal ui contribution: %w", err)
		}
		out = append(out, &def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate ui contributions: %w", err)
	}
	return out, nil
}

func (r *SQLiteUIContributionRepository) DeleteContribution(ctx context.Context, contributionID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_ui_contributions WHERE contribution_id = ?`, contributionID)
	if err != nil {
		return fmt.Errorf("sqlite: delete ui contribution: %w", err)
	}
	return nil
}

func (r *SQLiteUIContributionRepository) DeleteByExtension(ctx context.Context, extensionID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_ui_contributions WHERE extension_id = ?`, extensionID)
	if err != nil {
		return fmt.Errorf("sqlite: delete ui contributions by extension: %w", err)
	}
	return nil
}
