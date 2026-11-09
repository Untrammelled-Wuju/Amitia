package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	currentGenerationID := strings.TrimSpace(stringMetadata(inst.Metadata, "generationId"))
	currentArtifactID := strings.TrimSpace(stringMetadata(inst.Metadata, "artifactId"))
	currentVersionID := strings.TrimSpace(stringMetadata(inst.Metadata, "currentVersionId"))

	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_installations (id, extension_id, version, package_hash, install_path, installed, enabled, generation, installed_at, updated_at, installation_json, active_snapshot_id, current_generation_id, current_artifact_id, current_version_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id) DO UPDATE SET
			version = excluded.version,
			package_hash = excluded.package_hash,
			installed = excluded.installed,
			enabled = excluded.enabled,
			generation = excluded.generation,
			updated_at = excluded.updated_at,
			installation_json = excluded.installation_json,
			active_snapshot_id = excluded.active_snapshot_id,
			current_generation_id = excluded.current_generation_id,
			current_artifact_id = excluded.current_artifact_id,
			current_version_id = excluded.current_version_id
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
		currentGenerationID,
		currentArtifactID,
		currentVersionID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert installation: %w", err)
	}

	return nil
}

func (r *InstallationRepository) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	ex := getExecutor(ctx, r.db)
	var data, versionID, generationID, artifactID, currentVersion, recordGenerationID, recordArtifactID, installedPath, installedTreeHash, installOperationID string
	err := ex.QueryRowContext(ctx, `
		SELECT ei.installation_json,
		       COALESCE(ei.current_version_id, ''),
		       COALESCE(ei.current_generation_id, ''),
		       COALESCE(ei.current_artifact_id, ''),
		       COALESCE(pv.version, ''),
		       COALESCE(pv.generation_id, ''),
		       COALESCE(pv.artifact_id, ''),
		       COALESCE(pv.installed_path, ''),
		       COALESCE(pv.installed_tree_hash, ''),
		       COALESCE(pv.install_operation_id, '')
		FROM extension_installations ei
		LEFT JOIN package_versions pv ON pv.version_id = ei.current_version_id
		WHERE ei.extension_id = ?
	`, string(id)).Scan(&data, &versionID, &generationID, &artifactID, &currentVersion, &recordGenerationID, &recordArtifactID, &installedPath, &installedTreeHash, &installOperationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ExtensionInstallation{}, domain.ErrInvalidExtensionID
		}
		return domain.ExtensionInstallation{}, fmt.Errorf("sqlite: query installation: %w", err)
	}

	return decodeInstallation(data, versionID, generationID, artifactID, currentVersion, recordGenerationID, recordArtifactID, installedPath, installedTreeHash, installOperationID)
}

func (r *InstallationRepository) ListInstallations(ctx context.Context) ([]domain.ExtensionInstallation, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT ei.installation_json,
		       COALESCE(ei.current_version_id, ''),
		       COALESCE(ei.current_generation_id, ''),
		       COALESCE(ei.current_artifact_id, ''),
		       COALESCE(pv.version, ''),
		       COALESCE(pv.generation_id, ''),
		       COALESCE(pv.artifact_id, ''),
		       COALESCE(pv.installed_path, ''),
		       COALESCE(pv.installed_tree_hash, ''),
		       COALESCE(pv.install_operation_id, '')
		FROM extension_installations ei
		LEFT JOIN package_versions pv ON pv.version_id = ei.current_version_id
		ORDER BY ei.extension_id
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list installations: %w", err)
	}
	defer rows.Close()

	var out []domain.ExtensionInstallation
	for rows.Next() {
		var data, versionID, generationID, artifactID, currentVersion, recordGenerationID, recordArtifactID, installedPath, installedTreeHash, installOperationID string
		if err := rows.Scan(&data, &versionID, &generationID, &artifactID, &currentVersion, &recordGenerationID, &recordArtifactID, &installedPath, &installedTreeHash, &installOperationID); err != nil {
			return nil, fmt.Errorf("sqlite: scan installation: %w", err)
		}
		inst, err := decodeInstallation(data, versionID, generationID, artifactID, currentVersion, recordGenerationID, recordArtifactID, installedPath, installedTreeHash, installOperationID)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate installations: %w", err)
	}

	return out, nil
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}

func decodeInstallation(data, versionID, generationID, artifactID, currentVersion, recordGenerationID, recordArtifactID, installedPath, installedTreeHash, installOperationID string) (domain.ExtensionInstallation, error) {
	var inst domain.ExtensionInstallation
	if err := json.Unmarshal([]byte(data), &inst); err != nil {
		return domain.ExtensionInstallation{}, fmt.Errorf("sqlite: unmarshal installation: %w", err)
	}
	if versionID == "" || generationID == "" || recordGenerationID != generationID ||
		recordArtifactID == "" || currentVersion == "" ||
		(artifactID != "" && recordArtifactID != artifactID) {
		return inst, nil
	}
	if info, statErr := os.Stat(installedPath); statErr != nil || !info.IsDir() {
		return inst, nil
	}
	artifactID = recordArtifactID
	version, err := domain.ParseVersion(currentVersion)
	if err != nil {
		return inst, nil
	}
	inst.InstalledVersion = version
	inst.PackageID = artifactID
	if inst.Metadata == nil {
		inst.Metadata = map[string]any{}
	}
	inst.Metadata["currentVersionId"] = versionID
	inst.Metadata["generationId"] = generationID
	inst.Metadata["currentPath"] = installedPath
	inst.Metadata["installedPath"] = installedPath
	inst.Metadata["artifactId"] = artifactID
	inst.Metadata["installedTreeHash"] = installedTreeHash
	inst.Metadata["lastOperationId"] = installOperationID
	return inst, nil
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
