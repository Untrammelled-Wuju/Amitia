package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type InstallationRepository struct {
	db *sql.DB
}

func NewInstallationRepository(db *sql.DB) *InstallationRepository {
	return &InstallationRepository{db: db}
}

func (r *InstallationRepository) PutInstallation(ctx context.Context, inst domain.ExtensionInstallation) error {
	data, err := json.Marshal(inst)
	if err != nil {
		return fmt.Errorf("sqlite: marshal installation: %w", err)
	}

	id := inst.InstallationID
	if id == "" {
		id = string(inst.ExtensionID)
	}

	installed := 0
	if inst.InstallationState == domain.InstallationStateInstalled {
		installed = 1
	}
	enabled := 0
	if inst.EnablementState == domain.EnablementEnabled {
		enabled = 1
	}

	now := time.Now().UTC()
	installedAt := inst.InstalledAt
	if installedAt.IsZero() {
		installedAt = now
	}
	updatedAt := inst.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}

	packageHash := inst.PackageID

	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_installations (id, extension_id, version, package_hash, install_path, installed, enabled, generation, installed_at, updated_at, installation_json, active_snapshot_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id) DO UPDATE SET
			version = excluded.version,
			package_hash = excluded.package_hash,
			installed = excluded.installed,
			enabled = excluded.enabled,
			generation = excluded.generation,
			updated_at = excluded.updated_at,
			installation_json = excluded.installation_json,
			active_snapshot_id = excluded.active_snapshot_id
	`,
		id,
		string(inst.ExtensionID),
		inst.InstalledVersion.String(),
		packageHash,
		"",
		installed,
		enabled,
		inst.Generation,
		installedAt,
		updatedAt,
		string(data),
		inst.ActiveSnapshotID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert installation: %w", err)
	}

	return nil
}

func (r *InstallationRepository) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	ex := getExecutor(ctx, r.db)
	var data string
	err := ex.QueryRowContext(ctx, `SELECT installation_json FROM extension_installations WHERE extension_id = ?`, string(id)).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ExtensionInstallation{}, domain.ErrInvalidExtensionID
		}
		return domain.ExtensionInstallation{}, fmt.Errorf("sqlite: query installation: %w", err)
	}

	var inst domain.ExtensionInstallation
	if err := json.Unmarshal([]byte(data), &inst); err != nil {
		return domain.ExtensionInstallation{}, fmt.Errorf("sqlite: unmarshal installation: %w", err)
	}

	return inst, nil
}

func (r *InstallationRepository) ListInstallations(ctx context.Context) ([]domain.ExtensionInstallation, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT installation_json FROM extension_installations ORDER BY extension_id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list installations: %w", err)
	}
	defer rows.Close()

	var out []domain.ExtensionInstallation
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan installation: %w", err)
		}
		var inst domain.ExtensionInstallation
		if err := json.Unmarshal([]byte(data), &inst); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal installation: %w", err)
		}
		out = append(out, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate installations: %w", err)
	}

	return out, nil
}

func (r *InstallationRepository) DeleteInstallation(ctx context.Context, id domain.ExtensionID) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_installations WHERE extension_id = ?`, string(id))
	if err != nil {
		return fmt.Errorf("sqlite: delete installation: %w", err)
	}
	return nil
}

var _ domain.InstallationRepository = (*InstallationRepository)(nil)
