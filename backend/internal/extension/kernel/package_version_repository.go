package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
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

type PackageArtifactChange struct {
	Path    string `json:"path"`
	OldHash string `json:"oldHash,omitempty"`
	NewHash string `json:"newHash,omitempty"`
	Change  string `json:"change"`
}

type PackageManifestChange struct {
	Field     string `json:"field"`
	FromValue string `json:"fromValue,omitempty"`
	ToValue   string `json:"toValue,omitempty"`
	Changed   bool   `json:"changed"`
}

type PackageVersionComparison struct {
	ExtensionID          string                  `json:"extensionId"`
	FromVersion          string                  `json:"fromVersion"`
	ToVersion            string                  `json:"toVersion"`
	FromArtifactID       string                  `json:"fromArtifactId"`
	ToArtifactID         string                  `json:"toArtifactId"`
	FromArchiveHash      string                  `json:"fromArchiveHash"`
	ToArchiveHash        string                  `json:"toArchiveHash"`
	FromManifestHash     string                  `json:"fromManifestHash"`
	ToManifestHash       string                  `json:"toManifestHash"`
	FromContentTreeHash  string                  `json:"fromContentTreeHash"`
	ToContentTreeHash    string                  `json:"toContentTreeHash"`
	FromSignatureStatus  string                  `json:"fromSignatureStatus"`
	ToSignatureStatus    string                  `json:"toSignatureStatus"`
	ArtifactChanges      []PackageArtifactChange `json:"artifactChanges"`
	ManifestChanges      []PackageManifestChange `json:"manifestChanges"`
	FromManifest         manifest_v2.Manifest    `json:"-"`
	ToManifest           manifest_v2.Manifest    `json:"-"`
	FromFiles            []amitiax.FileEntry     `json:"-"`
	ToFiles              []amitiax.FileEntry     `json:"-"`
}

func (r *PackageRepository) ComparePackageVersions(ctx context.Context, extensionID, fromVersion, toVersion string) (*PackageVersionComparison, error) {
	fromRecord, err := r.GetPackageVersion(ctx, extensionID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("kernel: from version record unavailable: %w", err)
	}
	toRecord, err := r.GetPackageVersion(ctx, extensionID, toVersion)
	if err != nil {
		return nil, fmt.Errorf("kernel: to version record unavailable: %w", err)
	}
	fromArtifact, err := r.GetArtifact(ctx, fromRecord.ArtifactID)
	if err != nil {
		return nil, fmt.Errorf("kernel: from artifact unavailable: %w", err)
	}
	toArtifact, err := r.GetArtifact(ctx, toRecord.ArtifactID)
	if err != nil {
		return nil, fmt.Errorf("kernel: to artifact unavailable: %w", err)
	}
	fromPackage, err := amitiax.OpenArchive(fromArtifact.ArchivePath)
	if err != nil {
		return nil, fmt.Errorf("kernel: from package archive parse failed: %w", err)
	}
	toPackage, err := amitiax.OpenArchive(toArtifact.ArchivePath)
	if err != nil {
		return nil, fmt.Errorf("kernel: to package archive parse failed: %w", err)
	}
	comparison := &PackageVersionComparison{
		ExtensionID:         extensionID,
		FromVersion:         fromVersion,
		ToVersion:           toVersion,
		FromArtifactID:      fromRecord.ArtifactID,
		ToArtifactID:        toRecord.ArtifactID,
		FromArchiveHash:     fromArtifact.ArchiveHash,
		ToArchiveHash:       toArtifact.ArchiveHash,
		FromManifestHash:    fromArtifact.ManifestHash,
		ToManifestHash:      toArtifact.ManifestHash,
		FromContentTreeHash: fromArtifact.ContentTreeHash,
		ToContentTreeHash:   toArtifact.ContentTreeHash,
		FromSignatureStatus: fromArtifact.SignatureStatus,
		ToSignatureStatus:   toArtifact.SignatureStatus,
		FromManifest:        fromPackage.Manifest,
		ToManifest:          toPackage.Manifest,
		FromFiles:           fromPackage.Files,
		ToFiles:             toPackage.Files,
	}
	comparison.ArtifactChanges = computePackageArtifactFileChanges(fromPackage.Files, toPackage.Files)
	comparison.ManifestChanges = computePackageManifestFieldChanges(fromPackage.Manifest, toPackage.Manifest)
	return comparison, nil
}

func computePackageArtifactFileChanges(fromFiles, toFiles []amitiax.FileEntry) []PackageArtifactChange {
	fromMap := map[string]string{}
	toMap := map[string]string{}
	for _, file := range fromFiles {
		if file.IsDir {
			continue
		}
		fromMap[file.Path] = file.Hash
	}
	for _, file := range toFiles {
		if file.IsDir {
			continue
		}
		toMap[file.Path] = file.Hash
	}
	changes := make([]PackageArtifactChange, 0, len(fromMap)+len(toMap))
	for path, newHash := range toMap {
		oldHash, exists := fromMap[path]
		if !exists {
			changes = append(changes, PackageArtifactChange{Path: path, NewHash: newHash, Change: "added"})
		} else if oldHash != newHash {
			changes = append(changes, PackageArtifactChange{Path: path, OldHash: oldHash, NewHash: newHash, Change: "changed"})
		}
	}
	for path, oldHash := range fromMap {
		if _, exists := toMap[path]; !exists {
			changes = append(changes, PackageArtifactChange{Path: path, OldHash: oldHash, Change: "removed"})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Change != changes[j].Change {
			return changes[i].Change < changes[j].Change
		}
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func computePackageManifestFieldChanges(fromManifest, toManifest manifest_v2.Manifest) []PackageManifestChange {
	fromRaw, _ := json.Marshal(fromManifest)
	toRaw, _ := json.Marshal(toManifest)
	var fromMap, toMap map[string]interface{}
	if err := json.Unmarshal(fromRaw, &fromMap); err != nil {
		fromMap = map[string]interface{}{}
	}
	if err := json.Unmarshal(toRaw, &toMap); err != nil {
		toMap = map[string]interface{}{}
	}
	changes := make([]PackageManifestChange, 0, len(fromMap)+len(toMap))
	fromCanonical := func(value interface{}) string {
		raw, _ := json.Marshal(value)
		return string(raw)
	}
	keys := map[string]bool{}
	for key := range fromMap {
		keys[key] = true
	}
	for key := range toMap {
		keys[key] = true
	}
	sortedKeys := make([]string, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		fromValue, fromOK := fromMap[key]
		toValue, toOK := toMap[key]
		fromCanonicalValue := fromCanonical(fromValue)
		toCanonicalValue := fromCanonical(toValue)
		if !fromOK && toOK {
			changes = append(changes, PackageManifestChange{Field: key, ToValue: toCanonicalValue, Changed: true})
		} else if fromOK && !toOK {
			changes = append(changes, PackageManifestChange{Field: key, FromValue: fromCanonicalValue, Changed: true})
		} else if fromCanonicalValue != toCanonicalValue {
			changes = append(changes, PackageManifestChange{Field: key, FromValue: fromCanonicalValue, ToValue: toCanonicalValue, Changed: true})
		}
	}
	return changes
}

func (r *PackageRepository) MarkPackageVersionRollbackAvailable(ctx context.Context, extensionID, version string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE package_versions SET version_state = ? WHERE extension_id = ? AND version = ? AND version_state = ?`,
		string(PackageVersionStateRollbackAvailable), extensionID, version, string(PackageVersionStateRetained))
	if err != nil {
		return fmt.Errorf("kernel: mark package version rollback available failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("kernel: mark package version rollback available inspect failed: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("kernel: package version %s not in retained state for rollback available", version)
	}
	return nil
}

func (r *PackageRepository) MarkPackageVersionRollbackAvailableTx(ctx context.Context, tx *sql.Tx, extensionID, version string) error {
	if tx == nil {
		return errors.New("kernel: mark package version rollback available requires transaction")
	}
	result, err := tx.ExecContext(ctx, `UPDATE package_versions SET version_state = ? WHERE extension_id = ? AND version = ? AND version_state = ?`,
		string(PackageVersionStateRollbackAvailable), extensionID, version, string(PackageVersionStateRetained))
	if err != nil {
		return fmt.Errorf("kernel: mark package version rollback available failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("kernel: mark package version rollback available inspect failed: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("kernel: package version %s not in retained state for rollback available", version)
	}
	return nil
}

func (r *PackageRepository) BlockPackageVersion(ctx context.Context, extensionID, version string, reason string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE package_versions SET version_state = ? WHERE extension_id = ? AND version = ?`,
		string(PackageVersionStateBlocked), extensionID, version)
	if err != nil {
		return fmt.Errorf("kernel: block package version failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("kernel: block package version inspect failed: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("kernel: package version %s not found for block: %s", version, reason)
	}
	return nil
}

func (r *PackageRepository) CorruptPackageVersion(ctx context.Context, extensionID, version string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE package_versions SET version_state = ? WHERE extension_id = ? AND version = ?`,
		string(PackageVersionStateCorrupted), extensionID, version)
	if err != nil {
		return fmt.Errorf("kernel: corrupt package version failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("kernel: corrupt package version inspect failed: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("kernel: package version %s not found for corrupt", version)
	}
	return nil
}

func (r *PackageRepository) RemovePackageVersion(ctx context.Context, extensionID, version string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE package_versions SET version_state = ? WHERE extension_id = ? AND version = ?`,
		string(PackageVersionStateRemoved), extensionID, version)
	if err != nil {
		return fmt.Errorf("kernel: remove package version failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("kernel: remove package version inspect failed: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("kernel: package version %s not found for remove", version)
	}
	return nil
}
