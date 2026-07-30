package kernel

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type PackageVersionState string

const (
	PackageVersionStatePending           PackageVersionState = "pending"
	PackageVersionStateCurrent           PackageVersionState = "current"
	PackageVersionStateRetained          PackageVersionState = "retained"
	PackageVersionStateRollbackAvailable PackageVersionState = "rollback_available"
	PackageVersionStateRemoved           PackageVersionState = "removed"
	PackageVersionStateBlocked           PackageVersionState = "blocked"
	PackageVersionStateCorrupted         PackageVersionState = "corrupted"
)

type PackageVersionRecord struct {
	VersionID            string
	ExtensionID          string
	Version              string
	ArtifactID           string
	InstallOperationID   string
	UninstallOperationID string
	InstalledAt          string
	UninstalledAt        string
	IsActive             bool
	VersionState         string
	RetainedUntil        string
	InstalledPath        string
	InstalledTreeHash    string
	ArchiveHash          string
	ManifestHash         string
	ContentTreeHash      string
	GenerationID         string
}

type packageVersionExec interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type packageVersionRowScanner interface {
	Scan(dest ...interface{}) error
}

func packageVersionStateOf(record PackageVersionRecord) string {
	if record.VersionState != "" {
		return record.VersionState
	}
	if record.IsActive {
		return string(PackageVersionStateCurrent)
	}
	return string(PackageVersionStatePending)
}

const packageVersionSelectColumns = `version_id, extension_id, version, artifact_id,
	install_operation_id, uninstall_operation_id, installed_at, uninstalled_at,
	version_state, retained_until, installed_path, installed_tree_hash, archive_hash,
	manifest_hash, content_tree_hash, generation_id`

func scanPackageVersionRecord(scanner packageVersionRowScanner, record *PackageVersionRecord) error {
	if err := scanner.Scan(
		&record.VersionID, &record.ExtensionID, &record.Version, &record.ArtifactID,
		&record.InstallOperationID, &record.UninstallOperationID, &record.InstalledAt, &record.UninstalledAt,
		&record.VersionState, &record.RetainedUntil, &record.InstalledPath, &record.InstalledTreeHash, &record.ArchiveHash,
		&record.ManifestHash, &record.ContentTreeHash, &record.GenerationID); err != nil {
		return err
	}
	record.IsActive = record.VersionState == string(PackageVersionStateCurrent)
	return nil
}

func (r *PackageRepository) putPackageVersion(ctx context.Context, exec packageVersionExec, record PackageVersionRecord) error {
	versionState := packageVersionStateOf(record)
	createdAt := record.InstalledAt
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	retainedUntil := record.RetainedUntil
	_, err := exec.ExecContext(ctx, `INSERT INTO package_versions (
		version_id, extension_id, version, artifact_id, generation_id,
		manifest_hash, content_tree_hash, version_state, retained_until, installed_at, created_at,
		install_operation_id, uninstall_operation_id, uninstalled_at,
		installed_path, installed_tree_hash, archive_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(extension_id, version) DO UPDATE SET
		artifact_id=excluded.artifact_id,
		manifest_hash=excluded.manifest_hash,
		content_tree_hash=excluded.content_tree_hash,
		version_state=excluded.version_state,
		retained_until=excluded.retained_until,
		installed_at=excluded.installed_at,
		install_operation_id=excluded.install_operation_id,
		uninstall_operation_id=excluded.uninstall_operation_id,
		uninstalled_at=excluded.uninstalled_at,
		installed_path=excluded.installed_path,
		installed_tree_hash=excluded.installed_tree_hash,
		archive_hash=excluded.archive_hash,
		generation_id=excluded.generation_id`,
		record.VersionID, record.ExtensionID, record.Version, record.ArtifactID, record.GenerationID,
		record.ManifestHash, record.ContentTreeHash, versionState, retainedUntil, record.InstalledAt, createdAt,
		record.InstallOperationID, record.UninstallOperationID, record.UninstalledAt,
		record.InstalledPath, record.InstalledTreeHash, record.ArchiveHash)
	return err
}

func (r *PackageRepository) PutPackageVersion(ctx context.Context, record PackageVersionRecord) error {
	return r.putPackageVersion(ctx, r.db, record)
}

func (r *PackageRepository) PutPackageVersionTx(ctx context.Context, tx *sql.Tx, record PackageVersionRecord) error {
	if tx == nil {
		return errors.New("kernel: package version write requires transaction")
	}
	return r.putPackageVersion(ctx, tx, record)
}

func (r *PackageRepository) GetCurrentPackageVersionIDTx(ctx context.Context, tx *sql.Tx, extensionID string) (string, error) {
	if tx == nil {
		return "", errors.New("kernel: current package version lookup requires transaction")
	}
	var versionID string
	err := tx.QueryRowContext(ctx, `SELECT version_id FROM package_versions WHERE extension_id = ? AND version_state = ? ORDER BY installed_at DESC LIMIT 1`,
		extensionID, string(PackageVersionStateCurrent)).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return versionID, err
}

func (r *PackageRepository) ActivatePackageVersionTx(ctx context.Context, tx *sql.Tx, extensionID, newVersionID, oldVersionID string) error {
	if tx == nil {
		return errors.New("kernel: activate package version requires transaction")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if oldVersionID != "" && oldVersionID != newVersionID {
		if _, err := tx.ExecContext(ctx, `UPDATE package_versions SET version_state = ?, retained_until = ? WHERE extension_id = ? AND version_id = ?`,
			string(PackageVersionStateRetained), now, extensionID, oldVersionID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE package_versions SET version_state = ?, retained_until = '' WHERE extension_id = ? AND version_id = ?`,
		string(PackageVersionStateCurrent), extensionID, newVersionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE extension_installations SET current_version_id = ? WHERE extension_id = ?`,
		newVersionID, extensionID); err != nil {
		return err
	}
	return nil
}

func (r *PackageRepository) GetPackageVersion(ctx context.Context, extensionID, version string) (PackageVersionRecord, error) {
	var record PackageVersionRecord
	err := r.db.QueryRowContext(ctx, `SELECT `+packageVersionSelectColumns+`
		FROM package_versions WHERE extension_id = ? AND version = ?`,
		extensionID, version).Scan(
		&record.VersionID, &record.ExtensionID, &record.Version, &record.ArtifactID,
		&record.InstallOperationID, &record.UninstallOperationID, &record.InstalledAt, &record.UninstalledAt,
		&record.VersionState, &record.RetainedUntil, &record.InstalledPath, &record.InstalledTreeHash, &record.ArchiveHash,
		&record.ManifestHash, &record.ContentTreeHash, &record.GenerationID)
	if err != nil {
		return PackageVersionRecord{}, err
	}
	record.IsActive = record.VersionState == string(PackageVersionStateCurrent)
	return record, nil
}

func (r *PackageRepository) GetPackageVersionByID(ctx context.Context, extensionID, versionID string) (PackageVersionRecord, error) {
	var record PackageVersionRecord
	err := r.db.QueryRowContext(ctx, `SELECT `+packageVersionSelectColumns+`
		FROM package_versions WHERE extension_id = ? AND version_id = ?`,
		extensionID, versionID).Scan(
		&record.VersionID, &record.ExtensionID, &record.Version, &record.ArtifactID,
		&record.InstallOperationID, &record.UninstallOperationID, &record.InstalledAt, &record.UninstalledAt,
		&record.VersionState, &record.RetainedUntil, &record.InstalledPath, &record.InstalledTreeHash, &record.ArchiveHash,
		&record.ManifestHash, &record.ContentTreeHash, &record.GenerationID)
	if err != nil {
		return PackageVersionRecord{}, err
	}
	record.IsActive = record.VersionState == string(PackageVersionStateCurrent)
	return record, nil
}

func (r *PackageRepository) ListPackageVersions(ctx context.Context, extensionID string) ([]PackageVersionRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+packageVersionSelectColumns+`
		FROM package_versions WHERE extension_id = ? ORDER BY installed_at DESC`,
		extensionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []PackageVersionRecord
	for rows.Next() {
		var record PackageVersionRecord
		if err := scanPackageVersionRecord(rows, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *PackageRepository) ListActivePackageVersions(ctx context.Context) ([]PackageVersionRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+packageVersionSelectColumns+`
		FROM package_versions WHERE version_state = ? ORDER BY installed_at DESC`,
		string(PackageVersionStateCurrent))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []PackageVersionRecord
	for rows.Next() {
		var record PackageVersionRecord
		if err := scanPackageVersionRecord(rows, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *PackageRepository) DeactivatePackageVersion(ctx context.Context, extensionID, version, uninstallOperationID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `UPDATE package_versions SET version_state = ?, uninstall_operation_id = ?, uninstalled_at = ?, retained_until = ? WHERE extension_id = ? AND version = ?`,
		string(PackageVersionStateRetained), uninstallOperationID, now, now, extensionID, version)
	return err
}

func (r *PackageRepository) DeactivatePackageVersionTx(ctx context.Context, tx *sql.Tx, extensionID, version, uninstallOperationID string) error {
	if tx == nil {
		return errors.New("kernel: deactivate package version requires transaction")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `UPDATE package_versions SET version_state = ?, uninstall_operation_id = ?, uninstalled_at = ?, retained_until = ? WHERE extension_id = ? AND version = ?`,
		string(PackageVersionStateRetained), uninstallOperationID, now, now, extensionID, version)
	return err
}

func (r *PackageRepository) GetLatestPackageVersion(ctx context.Context, extensionID string) (PackageVersionRecord, error) {
	var record PackageVersionRecord
	err := r.db.QueryRowContext(ctx, `SELECT `+packageVersionSelectColumns+`
		FROM package_versions WHERE extension_id = ? ORDER BY installed_at DESC, created_at DESC LIMIT 1`,
		extensionID).Scan(
		&record.VersionID, &record.ExtensionID, &record.Version, &record.ArtifactID,
		&record.InstallOperationID, &record.UninstallOperationID, &record.InstalledAt, &record.UninstalledAt,
		&record.VersionState, &record.RetainedUntil, &record.InstalledPath, &record.InstalledTreeHash, &record.ArchiveHash,
		&record.ManifestHash, &record.ContentTreeHash, &record.GenerationID)
	if err != nil {
		return PackageVersionRecord{}, err
	}
	record.IsActive = record.VersionState == string(PackageVersionStateCurrent)
	return record, nil
}

func (r *PackageRepository) GetCurrentPackageVersion(ctx context.Context, extensionID string) (PackageVersionRecord, error) {
	var record PackageVersionRecord
	err := r.db.QueryRowContext(ctx, `SELECT `+packageVersionSelectColumns+`
		FROM package_versions WHERE extension_id = ? AND version_state = ? ORDER BY installed_at DESC LIMIT 1`,
		extensionID, string(PackageVersionStateCurrent)).Scan(
		&record.VersionID, &record.ExtensionID, &record.Version, &record.ArtifactID,
		&record.InstallOperationID, &record.UninstallOperationID, &record.InstalledAt, &record.UninstalledAt,
		&record.VersionState, &record.RetainedUntil, &record.InstalledPath, &record.InstalledTreeHash, &record.ArchiveHash,
		&record.ManifestHash, &record.ContentTreeHash, &record.GenerationID)
	if err != nil {
		return PackageVersionRecord{}, err
	}
	record.IsActive = record.VersionState == string(PackageVersionStateCurrent)
	return record, nil
}
