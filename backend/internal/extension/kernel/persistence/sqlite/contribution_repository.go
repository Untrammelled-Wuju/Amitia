package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ContributionRepository interface {
	PutContribution(ctx context.Context, contrib domain.ContributionDefinition) error
	GetContribution(ctx context.Context, extensionID domain.ExtensionID, contributionID domain.ContributionID) (domain.ContributionDefinition, error)
	ListContributions(ctx context.Context, extensionID domain.ExtensionID) ([]domain.ContributionDefinition, error)
	ListContributionsByModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) ([]domain.ContributionDefinition, error)
	DeleteContributions(ctx context.Context, extensionID domain.ExtensionID) error
	DeleteContributionsByModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) error
}

type SQLiteContributionRepository struct {
	db *sql.DB
}

func NewContributionRepository(db *sql.DB) *SQLiteContributionRepository {
	return &SQLiteContributionRepository{db: db}
}

func (r *SQLiteContributionRepository) PutContribution(ctx context.Context, contrib domain.ContributionDefinition) error {
	data, err := json.Marshal(contrib)
	if err != nil {
		return fmt.Errorf("sqlite: marshal contribution: %w", err)
	}

	id := contributionKey(contrib.ExtensionID, contrib.ID)
	now := time.Now().UTC()

	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_contributions (id, extension_id, module_id, contribution_id, contribution_type, definition_json, enabled_override, registered, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			module_id = excluded.module_id,
			contribution_type = excluded.contribution_type,
			definition_json = excluded.definition_json,
			enabled_override = excluded.enabled_override
	`,
		id,
		string(contrib.ExtensionID),
		string(contrib.ModuleID),
		string(contrib.ID),
		string(contrib.Kind),
		string(data),
		nil,
		0,
		now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert contribution: %w", err)
	}

	return nil
}

func (r *SQLiteContributionRepository) GetContribution(ctx context.Context, extensionID domain.ExtensionID, contributionID domain.ContributionID) (domain.ContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	key := contributionKey(extensionID, contributionID)

	var data string
	err := ex.QueryRowContext(ctx, `SELECT definition_json FROM extension_contributions WHERE id = ?`, key).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ContributionDefinition{}, domain.ErrInvalidExtensionID
		}
		return domain.ContributionDefinition{}, fmt.Errorf("sqlite: query contribution: %w", err)
	}

	var contrib domain.ContributionDefinition
	if err := json.Unmarshal([]byte(data), &contrib); err != nil {
		return domain.ContributionDefinition{}, fmt.Errorf("sqlite: unmarshal contribution: %w", err)
	}

	return contrib, nil
}

func (r *SQLiteContributionRepository) ListContributions(ctx context.Context, extensionID domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_contributions WHERE extension_id = ? ORDER BY contribution_id`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list contributions: %w", err)
	}
	defer rows.Close()

	var out []domain.ContributionDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan contribution: %w", err)
		}
		var contrib domain.ContributionDefinition
		if err := json.Unmarshal([]byte(data), &contrib); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal contribution: %w", err)
		}
		out = append(out, contrib)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate contributions: %w", err)
	}

	return out, nil
}

func (r *SQLiteContributionRepository) ListContributionsByModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) ([]domain.ContributionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_contributions WHERE extension_id = ? AND module_id = ? ORDER BY contribution_id`, string(extensionID), string(moduleID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list contributions by module: %w", err)
	}
	defer rows.Close()

	var out []domain.ContributionDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan contribution: %w", err)
		}
		var contrib domain.ContributionDefinition
		if err := json.Unmarshal([]byte(data), &contrib); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal contribution: %w", err)
		}
		out = append(out, contrib)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate contributions: %w", err)
	}

	return out, nil
}

func (r *SQLiteContributionRepository) DeleteContributions(ctx context.Context, extensionID domain.ExtensionID) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_contributions WHERE extension_id = ?`, string(extensionID))
	if err != nil {
		return fmt.Errorf("sqlite: delete contributions: %w", err)
	}
	return nil
}

func (r *SQLiteContributionRepository) DeleteContributionsByModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_contributions WHERE extension_id = ? AND module_id = ?`, string(extensionID), string(moduleID))
	if err != nil {
		return fmt.Errorf("sqlite: delete contributions by module: %w", err)
	}
	return nil
}

func contributionKey(extensionID domain.ExtensionID, contributionID domain.ContributionID) string {
	return string(extensionID) + "/" + string(contributionID)
}

var _ ContributionRepository = (*SQLiteContributionRepository)(nil)
