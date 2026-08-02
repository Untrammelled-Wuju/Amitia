//go:build legacy_migration

package extension

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type packageImportSessionRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	UserID      string `gorm:"column:user_id"`
	ScopeType   string `gorm:"column:scope_type"`
	ScopeID     string `gorm:"column:scope_id"`
	Format      string `gorm:"column:format"`
	PackageHash string `gorm:"column:package_hash"`
	Status      string `gorm:"column:status"`
	PreviewJSON string `gorm:"column:preview_json"`
	PackageBlob []byte `gorm:"column:package_blob"`
	FileName    string `gorm:"column:file_name"`
	ExpiresAt   string `gorm:"column:expires_at"`
	ConsumedAt  string `gorm:"column:consumed_at"`
	CreatedAt   string `gorm:"column:created_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
}

func (packageImportSessionRecord) TableName() string { return "extension_package_import_sessions" }

type packageOperationRecord struct {
	ID                string `gorm:"column:id;primaryKey"`
	ExtensionID       string `gorm:"column:extension_id"`
	ExtensionVersion  string `gorm:"column:extension_version"`
	Operation         string `gorm:"column:operation"`
	Source            string `gorm:"column:source"`
	PackageHash       string `gorm:"column:package_hash"`
	SignatureStatus   string `gorm:"column:signature_status"`
	SignerFingerprint string `gorm:"column:signer_fingerprint"`
	PreviousVersion   string `gorm:"column:previous_version"`
	TargetVersion     string `gorm:"column:target_version"`
	UserID            string `gorm:"column:user_id"`
	ScopeType         string `gorm:"column:scope_type"`
	ScopeID           string `gorm:"column:scope_id"`
	Status            string `gorm:"column:status"`
	ErrorCode         string `gorm:"column:error_code"`
	TraceID           string `gorm:"column:trace_id"`
	CreatedAt         string `gorm:"column:created_at"`
	CompletedAt       string `gorm:"column:completed_at"`
}

func (packageOperationRecord) TableName() string { return "extension_package_installations" }

type packageSignerRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	Fingerprint string `gorm:"column:fingerprint"`
	PublicKey   string `gorm:"column:public_key"`
	Algorithm   string `gorm:"column:algorithm"`
	DisplayName string `gorm:"column:display_name"`
	Trusted     int    `gorm:"column:trusted"`
	TrustedAt   string `gorm:"column:trusted_at"`
	RevokedAt   string `gorm:"column:revoked_at"`
	CreatedAt   string `gorm:"column:created_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
}

func (packageSignerRecord) TableName() string { return "extension_package_signers" }

type packageDependencyRecord struct {
	ExtensionID      string `gorm:"column:extension_id"`
	ExtensionVersion string `gorm:"column:extension_version"`
	DependencyID     string `gorm:"column:dependency_id"`
	Constraint       string `gorm:"column:version_constraint"`
	Required         int    `gorm:"column:required"`
	CreatedAt        string `gorm:"column:created_at"`
}

func (packageDependencyRecord) TableName() string { return "extension_version_dependencies" }

func (packageVersionRecord) TableName() string { return "extension_versions" }

func (packageArtifactRecord) TableName() string { return "extension_artifacts" }

func (record packageArtifactRecord) base() extensionArtifactRecord {
	return extensionArtifactRecord{ID: record.ID, ArtifactID: record.ArtifactID, ExtensionID: record.ExtensionID, ExtensionVersion: record.ExtensionVersion, Source: record.Source, SessionID: record.SessionID, Revision: record.Revision, ManifestJSON: record.ManifestJSON, WorkflowJSON: record.WorkflowJSON, SchemasJSON: record.SchemasJSON, CompiledWorkflowJSON: record.CompiledWorkflowJSON, TestsJSON: record.TestsJSON, ReadmeText: record.ReadmeText, Checksum: record.Checksum, SizeBytes: record.SizeBytes, CreatedAt: record.CreatedAt, ArchivedAt: record.ArchivedAt}
}

type packageExportRecord struct {
	ID           string `gorm:"column:id;primaryKey"`
	UserID       string `gorm:"column:user_id"`
	ExtensionID  string `gorm:"column:extension_id"`
	FileName     string `gorm:"column:file_name"`
	MIME         string `gorm:"column:mime"`
	PackageHash  string `gorm:"column:package_hash"`
	ContentBlob  []byte `gorm:"column:content_blob"`
	ExpiresAt    string `gorm:"column:expires_at"`
	CreatedAt    string `gorm:"column:created_at"`
	DownloadedAt string `gorm:"column:downloaded_at"`
}

func (packageExportRecord) TableName() string { return "extension_package_exports" }

func (r *Repository) CreatePackageImportSession(ctx context.Context, userID, scopeType, scopeID, fileName string, parsed parsedExtensionPackage, preview PackageImportPreview) error {
	rawPreview, err := json.Marshal(preview)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := packageImportSessionRecord{ID: preview.SessionID, UserID: userID, ScopeType: scopeType, ScopeID: scopeID, Format: string(parsed.Format), PackageHash: parsed.PackageHash, Status: "previewed", PreviewJSON: string(rawPreview), PackageBlob: parsed.Raw, FileName: fileName, ExpiresAt: preview.ExpiresAt.UTC().Format(time.RFC3339Nano), CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *Repository) AcquirePackageImportSession(ctx context.Context, sessionID, userID, scopeType, scopeID string) (packageImportSessionRecord, error) {
	var record packageImportSessionRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND scope_type = ? AND scope_id = ?", sessionID, userID, scopeType, scopeID).First(&record).Error; err != nil {
			return err
		}
		if record.Status != "previewed" || record.ConsumedAt != "" {
			return NewExtensionError(ErrPackageImportSessionConsumed, "导入会话已使用", sessionID, false, nil)
		}
		expires, _ := time.Parse(time.RFC3339Nano, record.ExpiresAt)
		if expires.IsZero() || time.Now().After(expires) {
			_ = tx.Model(&record).Updates(map[string]interface{}{"status": "expired", "package_blob": []byte{}, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
			return NewExtensionError(ErrPackageImportSessionExpired, "导入会话已过期", sessionID, false, nil)
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		return tx.Model(&record).Updates(map[string]interface{}{"status": "installing", "consumed_at": now, "updated_at": now}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, NewExtensionError(ErrPackageImportSessionExpired, "导入会话不存在或作用域不匹配", sessionID, false, nil)
	}
	return record, err
}

func (r *Repository) FinishPackageImportSession(ctx context.Context, id, status string) error {
	return r.db.WithContext(ctx).Model(&packageImportSessionRecord{}).Where("id = ?", id).Updates(map[string]interface{}{"status": status, "package_blob": []byte{}, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
}

func (r *Repository) CleanupPackageSessions(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.db.WithContext(ctx).Model(&packageImportSessionRecord{}).Where("expires_at < ? AND status NOT IN ?", now, []string{"installed", "expired"}).Updates(map[string]interface{}{"status": "expired", "package_blob": []byte{}, "updated_at": now}).Error
}

func (r *Repository) PackageSignerTrusted(ctx context.Context, fingerprint string) (bool, error) {
	if fingerprint == "" {
		return false, nil
	}
	var record packageSignerRecord
	err := r.db.WithContext(ctx).Where("fingerprint = ?", fingerprint).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return record.Trusted == 1 && record.RevokedAt == "", err
}

func (r *Repository) SavePackageSigner(ctx context.Context, view PackageSignatureView, publicKey string) error {
	if view.Fingerprint == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := packageSignerRecord{ID: uuid.NewString(), Fingerprint: view.Fingerprint, PublicKey: publicKey, Algorithm: view.Algorithm, DisplayName: view.DisplayName, CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "fingerprint"}}, DoUpdates: clause.AssignmentColumns([]string{"public_key", "algorithm", "display_name", "updated_at"})}).Create(&record).Error
}

func (r *Repository) SetPackageSignerTrust(ctx context.Context, fingerprint string, trusted bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updates := map[string]interface{}{"trusted": boolNumber(trusted), "updated_at": now}
	if trusted {
		updates["trusted_at"] = now
		updates["revoked_at"] = ""
	} else {
		updates["revoked_at"] = now
	}
	result := r.db.WithContext(ctx).Model(&packageSignerRecord{}).Where("fingerprint = ?", fingerprint).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return NewExtensionError(ErrPackageSignatureInvalid, "签名者不存在", fingerprint, false, nil)
	}
	return nil
}

func (r *Repository) ListPackageSigners(ctx context.Context) ([]PackageSignerView, error) {
	var records []packageSignerRecord
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]PackageSignerView, 0, len(records))
	for _, record := range records {
		result = append(result, PackageSignerView{Fingerprint: record.Fingerprint, Algorithm: record.Algorithm, DisplayName: record.DisplayName, Trusted: record.Trusted == 1 && record.RevokedAt == "", TrustedAt: record.TrustedAt, RevokedAt: record.RevokedAt})
	}
	return result, nil
}

func (r *Repository) CreatePackageOperation(ctx context.Context, record packageOperationRecord) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *Repository) SetPackageOperationStatus(ctx context.Context, id, status string) error {
	return r.db.WithContext(ctx).Model(&packageOperationRecord{}).Where("id = ?", id).Updates(map[string]interface{}{"status": status, "error_code": ""}).Error
}

func (r *Repository) UpdatePackageOperationDetails(ctx context.Context, id string, operation PackageOperation, preview PackageImportPreview, previousVersion string) error {
	updates := map[string]interface{}{
		"operation":          string(operation),
		"extension_id":       preview.ID,
		"extension_version":  preview.Version,
		"source":             preview.Source,
		"package_hash":       preview.PackageHash,
		"signature_status":   string(preview.Signature.Status),
		"signer_fingerprint": preview.Signature.Fingerprint,
		"target_version":     preview.Version,
	}
	if previousVersion != "" {
		updates["previous_version"] = previousVersion
	}
	return r.db.WithContext(ctx).Model(&packageOperationRecord{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) FinishPackageOperation(ctx context.Context, id, status, errorCode string) error {
	return r.db.WithContext(ctx).Model(&packageOperationRecord{}).Where("id = ?", id).Updates(map[string]interface{}{"status": status, "error_code": errorCode, "completed_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
}

func (r *Repository) ListPackageOperations(ctx context.Context, userID string, limit int) ([]PackageOperationView, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	var records []packageOperationRecord
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]PackageOperationView, 0, len(records))
	for _, record := range records {
		result = append(result, packageOperationView(record))
	}
	return result, nil
}

func (r *Repository) GetPackageOperation(ctx context.Context, userID, id string) (PackageOperationView, error) {
	var record packageOperationRecord
	if err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&record).Error; err != nil {
		return PackageOperationView{}, err
	}
	return packageOperationView(record), nil
}

func packageOperationView(record packageOperationRecord) PackageOperationView {
	return PackageOperationView{ID: record.ID, Operation: PackageOperation(record.Operation), ExtensionID: record.ExtensionID, PreviousVersion: record.PreviousVersion, TargetVersion: record.TargetVersion, Source: record.Source, PackageHash: record.PackageHash, SignatureStatus: record.SignatureStatus, SignerFingerprint: record.SignerFingerprint, ScopeType: record.ScopeType, ScopeID: record.ScopeID, Status: record.Status, ErrorCode: record.ErrorCode, TraceID: record.TraceID, CreatedAt: record.CreatedAt, CompletedAt: record.CompletedAt}
}

func (r *Repository) GetPackageExtension(ctx context.Context, id, userID, scopeType, scopeID string) (extensionRecord, error) {
	var record extensionRecord
	query := r.db.WithContext(ctx).Where("extension_id = ? AND archived_at = ''", id)
	if err := query.First(&record).Error; err != nil {
		return record, err
	}
	var ownership struct {
		OwnerUserID string `gorm:"column:owner_user_id"`
		ScopeType   string `gorm:"column:scope_type"`
		ScopeID     string `gorm:"column:scope_id"`
	}
	if err := r.db.WithContext(ctx).Table("extensions").Select("owner_user_id", "scope_type", "scope_id").Where("extension_id = ?", id).Take(&ownership).Error; err != nil {
		return record, err
	}
	if ownership.OwnerUserID == "" {
		var agent agentSkillMetadataRecord
		if r.db.WithContext(ctx).Where("extension_id = ?", id).First(&agent).Error == nil {
			ownership.OwnerUserID, ownership.ScopeType, ownership.ScopeID = agent.UserID, agent.ScopeType, agent.ScopeID
		} else {
			_ = r.db.WithContext(ctx).Table("extension_artifacts ea").Select("ws.user_id AS owner_user_id", "CASE WHEN ws.character_id = '' THEN 'global' ELSE 'character' END AS scope_type", "ws.character_id AS scope_id").Joins("JOIN extension_workshop_sessions ws ON ws.id = ea.session_id").Where("ea.extension_id = ? AND ea.extension_version = ?", id, record.CurrentVersion).Take(&ownership).Error
		}
	}
	if ownership.OwnerUserID == "" || ownership.OwnerUserID != userID || ownership.ScopeType != scopeType || ownership.ScopeID != scopeID {
		return record, NewExtensionError(ErrSkillPermissionDenied, "扩展作用域不匹配", id, false, nil)
	}
	return record, nil
}

func (r *Repository) ListPackageVersions(ctx context.Context, extensionID, userID, scopeType, scopeID string) ([]PackageVersionView, error) {
	current, err := r.GetPackageExtension(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	var records []packageVersionRecord
	if err := r.db.WithContext(ctx).Where("extension_id = ?", extensionID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]PackageVersionView, 0, len(records))
	for _, record := range records {
		if record.ArtifactID == "" {
			var artifact packageArtifactRecord
			if r.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", extensionID, record.Version).First(&artifact).Error == nil {
				record.ArtifactID, record.ArtifactHash = artifact.ArtifactID, artifact.Checksum
				if record.PackageHash == "" {
					record.PackageHash = artifact.Checksum
				}
			}
		}
		var capabilities []string
		_ = json.Unmarshal([]byte(record.CapabilitiesJSON), &capabilities)
		result = append(result, PackageVersionView{Version: record.Version, Manifest: redactJSON(json.RawMessage(record.ManifestJSON)), ArtifactID: record.ArtifactID, ArtifactHash: record.ArtifactHash, PackageHash: record.PackageHash, Source: record.Source, SignatureStatus: record.SignatureStatus, SignerFingerprint: record.SignerFingerprint, CompatibilityStatus: record.CompatibilityStatus, Capabilities: capabilities, InstalledAt: record.CreatedAt, InstalledBy: record.InstalledBy, Active: current.CurrentVersion == record.Version, ValidationStatus: record.ValidationStatus, TestStatus: record.TestStatus, ArtifactStatus: record.ArtifactStatus, ActivationStatus: record.ActivationStatus, OperationID: record.OperationID, FailureCode: record.FailureCode, Archived: record.ArchivedAt != ""})
	}
	return result, nil
}

func (r *Repository) GetPackageVersion(ctx context.Context, extensionID, version string) (packageVersionRecord, error) {
	var record packageVersionRecord
	err := r.db.WithContext(ctx).Where("extension_id = ? AND version = ?", extensionID, version).First(&record).Error
	return record, err
}

func (r *Repository) SavePackageExport(ctx context.Context, userID string, exported ExportedPackage, extensionID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := packageExportRecord{ID: exported.ExportID, UserID: userID, ExtensionID: extensionID, FileName: exported.FileName, MIME: exported.MIME, PackageHash: exported.Hash, ContentBlob: exported.Content, ExpiresAt: exported.ExpiresAt.UTC().Format(time.RFC3339Nano), CreatedAt: now}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *Repository) GetPackageExport(ctx context.Context, userID, extensionID, id string) (ExportedPackage, error) {
	var record packageExportRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ? AND extension_id = ?", id, userID, extensionID).First(&record).Error; err != nil {
		return ExportedPackage{}, err
	}
	expires, _ := time.Parse(time.RFC3339Nano, record.ExpiresAt)
	if expires.IsZero() || time.Now().After(expires) {
		_ = r.db.WithContext(ctx).Delete(&record).Error
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "导出文件已过期", id, false, nil)
	}
	_ = r.db.WithContext(ctx).Model(&record).Update("downloaded_at", time.Now().UTC().Format(time.RFC3339Nano)).Error
	return ExportedPackage{ExportID: record.ID, FileName: record.FileName, MIME: record.MIME, Size: int64(len(record.ContentBlob)), Hash: record.PackageHash, ExpiresAt: expires, Content: record.ContentBlob}, nil
}

func (r *Repository) ReversePackageDependencies(ctx context.Context, dependencyID string) ([]PackageDependencyView, error) {
	var rows []packageDependencyRecord
	if err := r.db.WithContext(ctx).Where("dependency_id = ?", dependencyID).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]PackageDependencyView, 0, len(rows))
	for _, row := range rows {
		var current extensionRecord
		if r.db.WithContext(ctx).Where("extension_id = ? AND current_version = ? AND archived_at = ''", row.ExtensionID, row.ExtensionVersion).First(&current).Error == nil {
			result = append(result, PackageDependencyView{ID: row.ExtensionID, VersionConstraint: row.Constraint, Required: row.Required == 1, Installed: true, Version: row.ExtensionVersion})
		}
	}
	return result, nil
}
