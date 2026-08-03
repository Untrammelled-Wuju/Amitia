package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const resourceQuarantineDDL = `CREATE TABLE IF NOT EXISTS extension_package_resource_quarantine (
	quarantine_id TEXT NOT NULL,
	operation_id TEXT NOT NULL,
	extension_id TEXT NOT NULL,
	resource_id TEXT NOT NULL,
	logical_path TEXT NOT NULL DEFAULT '',
	original_path TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL DEFAULT '',
	storage_reference TEXT NOT NULL DEFAULT '',
	content_storage_reference TEXT NOT NULL DEFAULT '',
	namespace_hash TEXT NOT NULL DEFAULT '',
	size INTEGER NOT NULL DEFAULT 0,
	quarantine_path TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'preparing',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (quarantine_id)
)`

const resourceQuarantineIndexDDL = `CREATE INDEX IF NOT EXISTS idx_ext_pkg_resource_quarantine_ns_hash ON extension_package_resource_quarantine(extension_id, namespace_hash, state)`

type ResourceQuarantineState string

const (
	ResourceQuarantinePreparing ResourceQuarantineState = "preparing"
	ResourceQuarantined         ResourceQuarantineState = "quarantined"
	ResourcePurged              ResourceQuarantineState = "purged"
	ResourceRestored            ResourceQuarantineState = "restored"
)

type ResourceQuarantineEntry struct {
	QuarantineID            string
	OperationID             string
	ExtensionID             string
	ResourceID              string
	LogicalPath             string
	OriginalPath            string
	ContentHash             string
	StorageRef              string
	ContentStorageReference string
	NamespaceHash           string
	Size                    int64
	QuarantinePath          string
	State                   ResourceQuarantineState
	CreatedAt               string
	UpdatedAt               string
}

type ResourceSnapshotStore struct {
	db      *sql.DB
	extRoot string
}

func NewResourceSnapshotStore(db *sql.DB, extRoot string) *ResourceSnapshotStore {
	return &ResourceSnapshotStore{db: db, extRoot: extRoot}
}

func (s *ResourceSnapshotStore) EnsureSchema(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("kernel: resource snapshot store database unavailable")
	}
	if _, err := s.db.ExecContext(ctx, resourceQuarantineDDL); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, resourceQuarantineIndexDDL); err != nil {
		return err
	}
	if err := s.ensureQuarantineColumns(ctx); err != nil {
		return err
	}
	return nil
}

func (s *ResourceSnapshotStore) ensureQuarantineColumns(ctx context.Context) error {
	closures := []func(context.Context) error{
		s.ensureQuarantineSizeColumn,
		s.ensureQuarantineNamespaceHashColumn,
		s.ensureQuarantineOriginalPathColumn,
		s.ensureQuarantineContentStorageReferenceColumn,
	}
	for _, fn := range closures {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *ResourceSnapshotStore) ensureQuarantineSizeColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_resource_quarantine') WHERE name='size'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_resource_quarantine ADD COLUMN size INTEGER NOT NULL DEFAULT 0`)
	return err
}

func (s *ResourceSnapshotStore) ensureQuarantineNamespaceHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_resource_quarantine') WHERE name='namespace_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_resource_quarantine ADD COLUMN namespace_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *ResourceSnapshotStore) ensureQuarantineOriginalPathColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_resource_quarantine') WHERE name='original_path'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_resource_quarantine ADD COLUMN original_path TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *ResourceSnapshotStore) ensureQuarantineContentStorageReferenceColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_resource_quarantine') WHERE name='content_storage_reference'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_resource_quarantine ADD COLUMN content_storage_reference TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *ResourceSnapshotStore) VerifyResourceSnapshotEntries(ctx context.Context, snapshotJSON string) error {
	var snapshot packageResourceSnapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 422, fmt.Errorf("kernel: resource snapshot corrupt: %w", err))
	}
	contentStore := NewResourceContentStore(s.extRoot)
	for _, entry := range snapshot.Entries {
		if entry.ContentStorageReference == "" {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 422, fmt.Errorf("kernel: resource %s content storage reference missing", entry.Resource.ResourceID))
		}
		if entry.ContentHash == "" {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 422, fmt.Errorf("kernel: resource %s content hash missing", entry.Resource.ResourceID))
		}
		if err := contentStore.VerifyContentRef(entry.ContentStorageReference, entry.ContentHash, entry.Size); err != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 422, fmt.Errorf("kernel: resource content verification failed for %s: %w", entry.Resource.ResourceID, err))
		}
	}
	return nil
}

func (s *ResourceSnapshotStore) QuarantineNewResources(ctx context.Context, extensionID, operationID string, snapshotEntries []packageResourceSnapshotEntry, currentResources []domain.ResourceOwnership) ([]ResourceQuarantineEntry, error) {
	if s.db == nil {
		return nil, fmt.Errorf("kernel: resource snapshot store database unavailable")
	}
	if err := s.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	snapshotIDs := make(map[string]struct{}, len(snapshotEntries))
	for _, entry := range snapshotEntries {
		snapshotIDs[entry.Resource.ResourceID] = struct{}{}
	}
	quarantineDir := filepath.Join(s.extRoot, "quarantine", "resources", operationID)
	if err := os.MkdirAll(quarantineDir, 0755); err != nil {
		return nil, fmt.Errorf("kernel: create quarantine directory: %w", err)
	}
	absExtRoot, err := filepath.Abs(s.extRoot)
	if err != nil {
		return nil, fmt.Errorf("kernel: resolve ext root: %w", err)
	}
	contentStore := NewResourceContentStore(s.extRoot)
	var entries []ResourceQuarantineEntry
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, resource := range currentResources {
		if _, exists := snapshotIDs[resource.ResourceID]; exists {
			continue
		}
		logicalPath := extractResourceStringField(resource, "logicalPath")
		if logicalPath == "" {
			logicalPath = resource.Reference
		}
		originalPath := resource.Reference
		quarantineID := "resource-quarantine-" + operationID + "-" + resource.ResourceID
		quarantinePath := filepath.Join(quarantineDir, resource.ResourceID)

		if validateErr := ValidateResourcePath(originalPath, absExtRoot); validateErr != nil {
			return entries, validateErr
		}
		absOriginal, err := filepath.Abs(originalPath)
		if err != nil {
			return entries, fmt.Errorf("kernel: resolve original path for resource %s: %w", resource.ResourceID, err)
		}

		entry := ResourceQuarantineEntry{
			QuarantineID:   quarantineID,
			OperationID:    operationID,
			ExtensionID:    extensionID,
			ResourceID:     resource.ResourceID,
			LogicalPath:    logicalPath,
			OriginalPath:   originalPath,
			QuarantinePath: quarantinePath,
			State:          ResourceQuarantinePreparing,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := s.persistQuarantineEntry(ctx, entry); err != nil {
			return entries, err
		}

		if renameErr := safeRename(absOriginal, quarantinePath); renameErr != nil {
			if copyErr := copyResourceFile(absOriginal, quarantinePath); copyErr != nil {
				os.Remove(quarantinePath)
				return entries, fmt.Errorf("kernel: resource %s move to quarantine failed: %w", resource.ResourceID, copyErr)
			}
			if removeErr := os.Remove(absOriginal); removeErr != nil {
				return entries, fmt.Errorf("kernel: resource %s remove original after copy failed: %w", resource.ResourceID, removeErr)
			}
		}

		var contentHash string
		var size int64
		var contentStorageRef string
		if storeRef, hash, sz, storeErr := contentStore.StoreContent(quarantinePath); storeErr == nil {
			contentStorageRef = storeRef
			contentHash = hash
			size = sz
		} else {
			return entries, fmt.Errorf("kernel: resource %s store content failed: %w", resource.ResourceID, storeErr)
		}

		computedHash, hashErr := computeFileContentHash(quarantinePath)
		if hashErr != nil {
			return entries, NewPackageError(PackageErrCodeResourceSnapshotHashComputeFailed, 500,
				fmt.Errorf("kernel: resource %s compute content hash failed after store: %w", resource.ResourceID, hashErr))
		}
		if computedHash != contentHash {
			return entries, NewPackageError(PackageErrCodeResourceSnapshotHashMismatch, 500,
				fmt.Errorf("kernel: resource %s content hash mismatch after store: expected %s, got %s", resource.ResourceID, contentHash, computedHash))
		}

		if err := fsyncDir(quarantineDir); err != nil {
			return entries, fmt.Errorf("kernel: fsync quarantine dir after moving resource %s: %w", resource.ResourceID, err)
		}

		entry.ContentHash = contentHash
		entry.Size = size
		entry.ContentStorageReference = contentStorageRef
		entry.StorageRef = contentStorageRef
		entry.State = ResourceQuarantined
		if err := s.persistQuarantineEntry(ctx, entry); err != nil {
			return entries, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *ResourceSnapshotStore) PurgeQuarantinedResources(ctx context.Context, operationID string) error {
	if s.db == nil {
		return nil
	}
	entries, err := s.listQuarantineEntries(ctx, operationID, ResourceQuarantined)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.QuarantinePath != "" {
			if _, statErr := os.Stat(entry.QuarantinePath); statErr == nil {
				if err := os.RemoveAll(entry.QuarantinePath); err != nil {
					return fmt.Errorf("kernel: purge quarantined resource %s: %w", entry.ResourceID, err)
				}
			}
		}
		if err := s.updateQuarantineEntryState(ctx, entry.QuarantineID, ResourcePurged); err != nil {
			return err
		}
	}
	return nil
}

func (s *ResourceSnapshotStore) RestoreQuarantinedResources(ctx context.Context, operationID string) error {
	if s.db == nil {
		return nil
	}
	entries, err := s.listQuarantineEntries(ctx, operationID, ResourceQuarantined)
	if err != nil {
		return err
	}
	absExtRoot, _ := filepath.Abs(s.extRoot)
	for _, entry := range entries {
		if entry.OriginalPath == "" {
			return fmt.Errorf("kernel: resource %s original path empty, cannot restore", entry.ResourceID)
		}
		if entry.QuarantinePath == "" {
			return fmt.Errorf("kernel: resource %s quarantine path empty, cannot restore", entry.ResourceID)
		}
		if entry.ContentStorageReference == "" {
			return fmt.Errorf("kernel: resource %s content storage reference missing, cannot restore", entry.ResourceID)
		}
		if _, statErr := os.Stat(entry.QuarantinePath); statErr != nil {
			return fmt.Errorf("kernel: resource %s quarantine file missing: %w", entry.ResourceID, statErr)
		}

		if verifyErr := s.verifyQuarantineFile(entry); verifyErr != nil {
			return verifyErr
		}

		if validateErr := ValidateRestoreTargetPath(entry.OriginalPath, absExtRoot); validateErr != nil {
			return validateErr
		}

		absOriginal, err := filepath.Abs(entry.OriginalPath)
		if err != nil {
			return fmt.Errorf("kernel: resolve original path for resource %s: %w", entry.ResourceID, err)
		}

		if renameErr := safeRename(entry.QuarantinePath, absOriginal); renameErr != nil {
			if copyErr := copyResourceFile(entry.QuarantinePath, absOriginal); copyErr != nil {
				return fmt.Errorf("kernel: restore quarantined resource %s: %w", entry.ResourceID, copyErr)
			}
			if removeErr := os.Remove(entry.QuarantinePath); removeErr != nil {
				return fmt.Errorf("kernel: remove quarantine file after copy for resource %s: %w", entry.ResourceID, removeErr)
			}
		}

		if syncErr := fsyncDir(filepath.Dir(absOriginal)); syncErr != nil {
			return fmt.Errorf("kernel: fsync after restore resource %s: %w", entry.ResourceID, syncErr)
		}

		if err := s.updateQuarantineEntryState(ctx, entry.QuarantineID, ResourceRestored); err != nil {
			return err
		}
	}
	return nil
}

func (s *ResourceSnapshotStore) verifyQuarantineFile(entry ResourceQuarantineEntry) error {
	file, err := os.Open(entry.QuarantinePath)
	if err != nil {
		return fmt.Errorf("kernel: cannot open quarantine file for resource %s: %w", entry.ResourceID, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("kernel: compute hash for quarantine file %s: %w", entry.ResourceID, err)
	}
	actual := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if actual != entry.ContentHash {
		return fmt.Errorf("kernel: quarantine file hash mismatch for resource %s: expected %s, got %s", entry.ResourceID, entry.ContentHash, actual)
	}
	return nil
}

func (s *ResourceSnapshotStore) VerifyNoActiveQuarantine(ctx context.Context, operationID string) error {
	if s.db == nil {
		return nil
	}
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM extension_package_resource_quarantine
		 WHERE operation_id=? AND state IN (?, ?)`, operationID, string(ResourceQuarantinePreparing), string(ResourceQuarantined),
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("kernel: query active resource quarantine: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("kernel: %d resource(s) still in quarantine for operation %s", count, operationID)
	}
	return nil
}

type resourceTreeHashEntry struct {
	normalizedPath string
	size           int64
	contentHash    string
}

func (s *ResourceSnapshotStore) ComputeResourceTreeHash(ctx context.Context, extensionID string, resources []domain.ResourceOwnership) (string, error) {
	if len(resources) == 0 {
		return "", nil
	}
	absExtRoot, err := filepath.Abs(s.extRoot)
	if err != nil {
		return "", fmt.Errorf("kernel: resolve ext root: %w", err)
	}
	var entries []resourceTreeHashEntry
	for _, resource := range resources {
		originalPath := extractResourceStringField(resource, "originalPath")
		if originalPath == "" {
			originalPath = resource.Reference
		}
		if originalPath == "" {
			continue
		}
		if validateErr := ValidateResourcePath(originalPath, absExtRoot); validateErr != nil {
			return "", validateErr
		}
		absPath, err := filepath.Abs(originalPath)
		if err != nil {
			return "", fmt.Errorf("kernel: resolve resource path %s: %w", resource.ResourceID, err)
		}
		relPath, err := filepath.Rel(absExtRoot, absPath)
		if err != nil {
			return "", fmt.Errorf("kernel: compute relative path for %s: %w", resource.ResourceID, err)
		}
		normalizedPath := filepath.ToSlash(strings.ToLower(relPath))

		contentHash := extractResourceStringField(resource, "contentHash")
		if contentHash == "" {
			return "", fmt.Errorf("kernel: resource %s missing content hash", resource.ResourceID)
		}
		size := extractResourceInt64Field(resource, "size")

		entries = append(entries, resourceTreeHashEntry{
			normalizedPath: normalizedPath,
			size:           size,
			contentHash:    contentHash,
		})
	}
	if len(entries) != len(resources) {
		return "", fmt.Errorf("kernel: some resources missing required fields for tree hash")
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].normalizedPath < entries[j].normalizedPath
	})
	hasher := sha256.New()
	for _, entry := range entries {
		hasher.Write([]byte(entry.normalizedPath))
		hasher.Write([]byte{0})
		hasher.Write([]byte(fmt.Sprintf("%d", entry.size)))
		hasher.Write([]byte{0})
		hasher.Write([]byte(entry.contentHash))
		hasher.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *ResourceSnapshotStore) ComputeVerifiedResourceTreeHash(ctx context.Context, resources []domain.ResourceOwnership) (string, error) {
	if len(resources) == 0 {
		return "", nil
	}
	absExtRoot, err := filepath.Abs(s.extRoot)
	if err != nil {
		return "", fmt.Errorf("kernel: resolve ext root: %w", err)
	}
	var entries []resourceTreeHashEntry
	for _, resource := range resources {
		originalPath := extractResourceStringField(resource, "originalPath")
		if originalPath == "" {
			originalPath = resource.Reference
		}
		if originalPath == "" {
			continue
		}
		if validateErr := ValidateResourcePath(originalPath, absExtRoot); validateErr != nil {
			return "", validateErr
		}
		absPath, err := filepath.Abs(originalPath)
		if err != nil {
			return "", fmt.Errorf("kernel: resolve resource path %s: %w", resource.ResourceID, err)
		}
		relPath, err := filepath.Rel(absExtRoot, absPath)
		if err != nil {
			return "", fmt.Errorf("kernel: compute relative path for %s: %w", resource.ResourceID, err)
		}
		normalizedPath := filepath.ToSlash(strings.ToLower(relPath))

		info, statErr := os.Lstat(absPath)
		if statErr != nil {
			return "", fmt.Errorf("kernel: resource %s file not found: %w", resource.ResourceID, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
				fmt.Errorf("kernel: resource %s path %s is a symlink", resource.ResourceID, absPath))
		}

		file, openErr := os.Open(absPath)
		if openErr != nil {
			return "", fmt.Errorf("kernel: open resource file %s: %w", resource.ResourceID, openErr)
		}
		hasher := sha256.New()
		if _, copyErr := io.Copy(hasher, file); copyErr != nil {
			file.Close()
			return "", fmt.Errorf("kernel: hash resource file %s: %w", resource.ResourceID, copyErr)
		}
		file.Close()
		contentHash := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

		entries = append(entries, resourceTreeHashEntry{
			normalizedPath: normalizedPath,
			size:           info.Size(),
			contentHash:    contentHash,
		})
	}
	if len(entries) != len(resources) {
		return "", fmt.Errorf("kernel: some resources missing or unreadable for verified tree hash")
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].normalizedPath < entries[j].normalizedPath
	})
	hasher := sha256.New()
	for _, entry := range entries {
		hasher.Write([]byte(entry.normalizedPath))
		hasher.Write([]byte{0})
		hasher.Write([]byte(fmt.Sprintf("%d", entry.size)))
		hasher.Write([]byte{0})
		hasher.Write([]byte(entry.contentHash))
		hasher.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *ResourceSnapshotStore) persistQuarantineEntry(ctx context.Context, entry ResourceQuarantineEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO extension_package_resource_quarantine
		 (quarantine_id, operation_id, extension_id, resource_id, logical_path, original_path, content_hash,
		  storage_reference, content_storage_reference, namespace_hash, size, quarantine_path, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.QuarantineID, entry.OperationID, entry.ExtensionID, entry.ResourceID,
		entry.LogicalPath, entry.OriginalPath, entry.ContentHash, entry.StorageRef,
		entry.ContentStorageReference, entry.NamespaceHash, entry.Size, entry.QuarantinePath,
		string(entry.State), entry.CreatedAt, entry.UpdatedAt)
	return err
}

func (s *ResourceSnapshotStore) updateQuarantineEntryState(ctx context.Context, quarantineID string, state ResourceQuarantineState) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_resource_quarantine SET state=?, updated_at=? WHERE quarantine_id=?`,
		string(state), now, quarantineID)
	return err
}

func (s *ResourceSnapshotStore) listQuarantineEntries(ctx context.Context, operationID string, state ResourceQuarantineState) ([]ResourceQuarantineEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT quarantine_id, operation_id, extension_id, resource_id, logical_path, original_path, content_hash,
		        storage_reference, content_storage_reference, namespace_hash, size, quarantine_path, state, created_at, updated_at
		 FROM extension_package_resource_quarantine
		 WHERE operation_id=? AND state=?`, operationID, string(state))
	if err != nil {
		return nil, fmt.Errorf("query quarantine entries: %w", err)
	}
	defer rows.Close()
	var entries []ResourceQuarantineEntry
	for rows.Next() {
		var entry ResourceQuarantineEntry
		if err := rows.Scan(&entry.QuarantineID, &entry.OperationID, &entry.ExtensionID,
			&entry.ResourceID, &entry.LogicalPath, &entry.OriginalPath, &entry.ContentHash,
			&entry.StorageRef, &entry.ContentStorageReference, &entry.NamespaceHash,
			&entry.Size, &entry.QuarantinePath, &entry.State, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan quarantine entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *ResourceSnapshotStore) ListQuarantineEntriesByNamespaceHash(ctx context.Context, extensionID, namespaceHash string, state ResourceQuarantineState) ([]ResourceQuarantineEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT quarantine_id, operation_id, extension_id, resource_id, logical_path, original_path, content_hash,
		        storage_reference, content_storage_reference, namespace_hash, size, quarantine_path, state, created_at, updated_at
		 FROM extension_package_resource_quarantine
		 WHERE extension_id=? AND namespace_hash=? AND state=?`, extensionID, namespaceHash, string(state))
	if err != nil {
		return nil, fmt.Errorf("query quarantine entries by namespace hash: %w", err)
	}
	defer rows.Close()
	var entries []ResourceQuarantineEntry
	for rows.Next() {
		var entry ResourceQuarantineEntry
		if err := rows.Scan(&entry.QuarantineID, &entry.OperationID, &entry.ExtensionID,
			&entry.ResourceID, &entry.LogicalPath, &entry.OriginalPath, &entry.ContentHash,
			&entry.StorageRef, &entry.ContentStorageReference, &entry.NamespaceHash,
			&entry.Size, &entry.QuarantinePath, &entry.State, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan quarantine entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *ResourceSnapshotStore) RecoverPreparingQuarantine(ctx context.Context, operationID string) error {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT quarantine_id, operation_id, extension_id, resource_id, logical_path, original_path, content_hash,
		        storage_reference, content_storage_reference, namespace_hash, size, quarantine_path, state, created_at, updated_at
		 FROM extension_package_resource_quarantine
		 WHERE operation_id=? AND state=?`, operationID, string(ResourceQuarantinePreparing))
	if err != nil {
		return fmt.Errorf("query preparing quarantine entries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry ResourceQuarantineEntry
		if err := rows.Scan(&entry.QuarantineID, &entry.OperationID, &entry.ExtensionID,
			&entry.ResourceID, &entry.LogicalPath, &entry.OriginalPath, &entry.ContentHash,
			&entry.StorageRef, &entry.ContentStorageReference, &entry.NamespaceHash,
			&entry.Size, &entry.QuarantinePath, &entry.State, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return fmt.Errorf("scan preparing quarantine entry: %w", err)
		}
		if entry.OriginalPath == "" {
			continue
		}
		absOriginal, absErr := filepath.Abs(entry.OriginalPath)
		if absErr != nil {
			continue
		}
		originalExists := false
		if _, statErr := os.Stat(absOriginal); statErr == nil {
			originalExists = true
		}
		quarantineExists := false
		if _, statErr := os.Stat(entry.QuarantinePath); statErr == nil {
			quarantineExists = true
		}
		if !originalExists && quarantineExists {
			if err := s.updateQuarantineEntryState(ctx, entry.QuarantineID, ResourceQuarantined); err != nil {
				return err
			}
		} else if originalExists && !quarantineExists {
			if err := s.updateQuarantineEntryState(ctx, entry.QuarantineID, ResourceRestored); err != nil {
				return err
			}
		} else if !originalExists && !quarantineExists {
			if err := s.updateQuarantineEntryState(ctx, entry.QuarantineID, ResourcePurged); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}

func copyResourceFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	srcInfo, err := srcFile.Stat()
	if err != nil {
		srcFile.Close()
		return fmt.Errorf("kernel: stat source file: %w", err)
	}
	if !srcInfo.Mode().IsRegular() {
		srcFile.Close()
		return fmt.Errorf("kernel: source %s is not a regular file", src)
	}
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
		srcFile.Close()
		return mkErr
	}
	dstDir := filepath.Dir(dst)
	dstInfo, dirErr := os.Lstat(dstDir)
	if dirErr != nil {
		srcFile.Close()
		return fmt.Errorf("kernel: lstat destination dir: %w", dirErr)
	}
	if !dstInfo.IsDir() {
		srcFile.Close()
		return fmt.Errorf("kernel: destination dir %s is not a directory", dstDir)
	}
	if dstInfo.Mode()&os.ModeSymlink != 0 {
		srcFile.Close()
		return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
			fmt.Errorf("kernel: destination dir %s is a symlink, blocked for TOCTOU safety", dstDir))
	}
	dstFile, err := safeCreateFile(dst)
	if err != nil {
		srcFile.Close()
		return err
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		srcFile.Close()
		return err
	}
	if syncErr := dstFile.Sync(); syncErr != nil {
		srcFile.Close()
		return syncErr
	}
	srcFile.Close()
	finalInfo, finalErr := os.Lstat(dst)
	if finalErr != nil {
		return fmt.Errorf("kernel: verify written file: %w", finalErr)
	}
	if finalInfo.Mode()&os.ModeSymlink != 0 {
		os.Remove(dst)
		return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
			fmt.Errorf("kernel: written file %s became a symlink, blocked for TOCTOU safety", dst))
	}
	return nil
}

func safeCreateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscallNofollow(), 0600)
}

func safeRename(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("kernel: lstat source for rename: %w", err)
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
			fmt.Errorf("kernel: source %s is a symlink, rename blocked", src))
	}
	if _, err := os.Lstat(dst); err == nil {
		dstInfo, lErr := os.Lstat(dst)
		if lErr != nil {
			return fmt.Errorf("kernel: lstat destination for rename: %w", lErr)
		}
		if dstInfo.Mode()&os.ModeSymlink != 0 {
			return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
				fmt.Errorf("kernel: destination %s is a symlink, rename blocked", dst))
		}
	}
	if renameErr := os.Rename(src, dst); renameErr != nil {
		return renameErr
	}
	postInfo, postErr := os.Lstat(dst)
	if postErr != nil {
		return fmt.Errorf("kernel: post-rename lstat: %w", postErr)
	}
	if postInfo.Mode()&os.ModeSymlink != 0 {
		os.Remove(dst)
		return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
			fmt.Errorf("kernel: post-rename file became symlink, rolled back"))
	}
	return nil
}

func computeFileContentHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func fsyncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func ValidateResourcePath(resourcePath, extRoot string) error {
	absExtRoot, err := filepath.Abs(extRoot)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: resolve ext root: %w", err))
	}

	realExtRoot, err := filepath.EvalSymlinks(absExtRoot)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: eval symlinks for ext root: %w", err))
	}

	absResourcePath, err := filepath.Abs(resourcePath)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: resolve resource path: %w", err))
	}

	info, err := os.Lstat(absResourcePath)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: lstat resource path: %w", err))
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
			fmt.Errorf("kernel: resource path %s is a symlink, symlinks are not allowed in snapshot resources", absResourcePath))
	}

	realResourcePath, err := filepath.EvalSymlinks(absResourcePath)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: eval symlinks for resource path: %w", err))
	}

	relPath, err := filepath.Rel(realExtRoot, realResourcePath)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: compute relative path for %s: %w", resourcePath, err))
	}

	cleanRel := filepath.Clean(relPath)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) || filepath.IsAbs(cleanRel) {
		return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
			fmt.Errorf("kernel: resource path %s escapes ext root (rel: %s)", resourcePath, cleanRel))
	}

	if err := validatePathComponentsNoSymlink(absExtRoot, absResourcePath); err != nil {
		return err
	}

	return nil
}

func validatePathComponentsNoSymlink(absExtRoot, targetPath string) error {
	realExtRoot, err := filepath.EvalSymlinks(absExtRoot)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: eval symlinks for ext root: %w", err))
	}

	current := targetPath
	for {
		if len(current) < len(realExtRoot) {
			break
		}
		if current == realExtRoot || current == absExtRoot {
			break
		}
		info, err := os.Lstat(current)
		if err != nil {
			break
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
				fmt.Errorf("kernel: path component %s is a symlink, symlinks are not allowed in snapshot paths", current))
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil
}

type validatedRestorePath struct {
	AbsTargetPath string
	AbsParentDir  string
}

func findNearestExistingAncestor(path string) string {
	current := path
	for {
		if current == "" || current == "/" || current == "." {
			return ""
		}
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return current
			}
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func validateRestoreTargetPathPure(targetPath, extRoot string) (*validatedRestorePath, error) {
	absExtRoot, err := filepath.Abs(extRoot)
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: resolve ext root: %w", err))
	}

	realExtRoot, err := filepath.EvalSymlinks(absExtRoot)
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: eval symlinks for ext root: %w", err))
	}

	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: resolve target path: %w", err))
	}

	absParentDir := filepath.Dir(absTargetPath)
	absParentDir, err = filepath.Abs(absParentDir)
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: resolve parent directory: %w", err))
	}

	if err := validatePathComponentsNoSymlink(absExtRoot, absParentDir); err != nil {
		return nil, err
	}

	nearestExisting := findNearestExistingAncestor(absParentDir)
	if nearestExisting == "" {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: cannot find existing ancestor for parent dir %s", absParentDir))
	}

	realNearest, err := filepath.EvalSymlinks(nearestExisting)
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: eval symlinks for nearest existing ancestor %s: %w", nearestExisting, err))
	}

	relFromNearest, err := filepath.Rel(nearestExisting, absParentDir)
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: compute relative path from nearest ancestor: %w", err))
	}
	realParentDir := filepath.Join(realNearest, relFromNearest)

	relPath, err := filepath.Rel(realExtRoot, realParentDir)
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
			fmt.Errorf("kernel: compute relative path for parent: %w", err))
	}

	cleanRel := filepath.Clean(relPath)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) || filepath.IsAbs(cleanRel) {
		return nil, NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
			fmt.Errorf("kernel: restore target parent %s escapes ext root (rel: %s)", absParentDir, cleanRel))
	}

	return &validatedRestorePath{
		AbsTargetPath: absTargetPath,
		AbsParentDir:  absParentDir,
	}, nil
}

func createRestoreDirectoriesSafely(absParentDir string) error {
	if absParentDir == "" {
		return nil
	}
	info, err := os.Lstat(absParentDir)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("kernel: %s exists but is not a directory", absParentDir)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("kernel: stat %s: %w", absParentDir, err)
	}

	parent := filepath.Dir(absParentDir)
	if parent != absParentDir {
		if err := createRestoreDirectoriesSafely(parent); err != nil {
			return err
		}
	}

	if info, err := os.Lstat(absParentDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return NewPackageError(PackageErrCodeResourceSnapshotSymlinkForbidden, 400,
				fmt.Errorf("kernel: parent directory %s is a symlink, symlinks are not allowed for restore targets", absParentDir))
		}
	}

	if mkErr := os.Mkdir(absParentDir, 0o700); mkErr != nil {
		if !os.IsExist(mkErr) {
			return NewPackageError(PackageErrCodeResourceSnapshotPathInvalid, 400,
				fmt.Errorf("kernel: create directory %s: %w", absParentDir, mkErr))
		}
	}
	return nil
}

func ValidateRestoreTargetPath(targetPath, extRoot string) error {
	validated, err := validateRestoreTargetPathPure(targetPath, extRoot)
	if err != nil {
		return err
	}
	if err := createRestoreDirectoriesSafely(validated.AbsParentDir); err != nil {
		return err
	}
	return validatePathComponentsNoSymlink(extRoot, validated.AbsParentDir)
}
