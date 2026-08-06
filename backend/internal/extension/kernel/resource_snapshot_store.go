package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
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
		if entry.RestoreStrategy == "uri_reference" {
			continue
		}
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

		if moveErr := copyFileAndRemoveByHandle(absExtRoot, absOriginal, absExtRoot, quarantinePath); moveErr != nil {
			return entries, fmt.Errorf("kernel: resource %s move to quarantine failed: %w", resource.ResourceID, moveErr)
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

		if err := func() error {
			validated, prepareErr := prepareRestoreTargetPath(entry.OriginalPath, absExtRoot)
			if prepareErr != nil {
				return prepareErr
			}
			defer validated.Close()
			if restoreErr := restoreQuarantinedFileSafely(absExtRoot, entry, validated); restoreErr != nil {
				return fmt.Errorf("kernel: restore quarantined resource %s: %w", entry.ResourceID, restoreErr)
			}
			return nil
		}(); err != nil {
			return err
		}

		if syncErr := fsyncDir(filepath.Dir(entry.OriginalPath)); syncErr != nil {
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
		absPath, err := filepath.Abs(originalPath)
		if err != nil {
			return "", fmt.Errorf("kernel: resolve resource path %s: %w", resource.ResourceID, err)
		}
		relPath, err := filepath.Rel(absExtRoot, absPath)
		if err != nil {
			return "", fmt.Errorf("kernel: compute relative path for %s: %w", resource.ResourceID, err)
		}
		normalizedPath := filepath.ToSlash(strings.ToLower(relPath))
		cleanRel := filepath.Clean(normalizedPath)
		if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+"/") || filepath.IsAbs(cleanRel) {
			return "", fmt.Errorf("kernel: resource path %s escapes ext root", resource.ResourceID)
		}

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

func restoreQuarantinedFileSafely(sourceRoot string, entry ResourceQuarantineEntry, validated *validatedRestorePath) error {
	return publishRestoreFileNoReplace(sourceRoot, entry.QuarantinePath, validated, entry.ContentHash)
}

func copyFileAndRemove(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("kernel: stat source file: %w", err)
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("kernel: source %s is not a regular file", src)
	}
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscallNofollow(), 0600)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	if syncErr := dstFile.Sync(); syncErr != nil {
		return syncErr
	}
	return os.Remove(src)
}

func copyFileAndRemoveByHandle(sourceRoot string, sourcePath string, targetRoot string, targetPath string) error {
	sourceParent, sourceName, err := openPlatformFileParent(sourceRoot, sourcePath)
	if err != nil {
		return err
	}
	defer sourceParent.close()
	targetParent, targetName, err := openPlatformFileParent(targetRoot, targetPath)
	if err != nil {
		return err
	}
	defer targetParent.close()
	source, _, err := sourceParent.openRegularFile(sourceName)
	if err != nil {
		return err
	}
	defer source.Close()
	temp, err := targetParent.createTempFile(".amitia-quarantine-")
	if err != nil {
		return err
	}
	cleanup := func() { _ = temp.File.Close(); _ = targetParent.removeChild(temp.Name) }
	if _, err := io.Copy(temp.File, source); err != nil {
		cleanup()
		return err
	}
	if err := temp.File.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := targetParent.publishTempNoReplace(temp, targetName); err != nil {
		cleanup()
		return err
	}
	if err := targetParent.removeChild(temp.Name); err != nil {
		_ = temp.File.Close()
		return err
	}
	if err := temp.File.Close(); err != nil {
		return err
	}
	if err := targetParent.sync(); err != nil {
		return err
	}
	if err := sourceParent.removeChild(sourceName); err != nil {
		return err
	}
	return sourceParent.sync()
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
		component := filepath.Base(current)
		if err := validatePlatformPathComponent(component); err != nil {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil
}

type validatedPlatformPathComponent struct {
	Path     string
	Identity platformPathIdentity
}

type validatedRestorePath struct {
	AbsoluteRoot string
	RealRoot     string

	AbsTargetPath string
	AbsParentDir  string
	TargetName    string

	ExistingAncestor string
	RealAncestor     string

	MissingComponents []string

	RootInfo     os.FileInfo
	AncestorInfo os.FileInfo

	PlatformComponents []validatedPlatformPathComponent
	DirectoryChain     []*platformRestoreDirectory
	ParentDirectory    *platformRestoreDirectory
}

func (validated *validatedRestorePath) Close() error {
	if validated == nil {
		return nil
	}
	var closeErrors []error
	for index := len(validated.DirectoryChain) - 1; index >= 0; index-- {
		if directory := validated.DirectoryChain[index]; directory != nil {
			if err := directory.close(); err != nil {
				closeErrors = append(closeErrors, err)
			}
		}
	}
	validated.DirectoryChain = nil
	validated.ParentDirectory = nil
	return errors.Join(closeErrors...)
}

func pathIsWithinRoot(root string, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	relative = filepath.Clean(relative)

	if relative == "." {
		return true
	}

	if relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(relative) {
		return false
	}

	return true
}

func captureRestoreParentPlatformChain(absoluteRoot string, absParentDir string) ([]validatedPlatformPathComponent, []string, error) {
	if !pathIsWithinRoot(absoluteRoot, absParentDir) {
		return nil, nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: restore parent %s escapes root %s", absParentDir, absoluteRoot),
		)
	}

	rootIdentity, err := capturePlatformPathIdentity(absoluteRoot, true)
	if err != nil {
		return nil, nil, fmt.Errorf("kernel: validate platform restore root %s: %w", absoluteRoot, err)
	}

	components := []validatedPlatformPathComponent{
		{
			Path:     absoluteRoot,
			Identity: rootIdentity,
		},
	}

	relative, err := filepath.Rel(absoluteRoot, absParentDir)
	if err != nil {
		return nil, nil, fmt.Errorf("kernel: compute restore parent relative path: %w", err)
	}

	relative = filepath.Clean(relative)

	if relative == "." {
		return components, nil, nil
	}

	names := strings.Split(relative, string(filepath.Separator))

	current := absoluteRoot

	for index, name := range names {
		if err := validatePlatformPathComponent(name); err != nil {
			return nil, nil, err
		}

		next := filepath.Join(current, name)

		_, statErr := os.Lstat(next)

		if os.IsNotExist(statErr) {
			return components, append([]string(nil), names[index:]...), nil
		}

		if statErr != nil {
			return nil, nil, NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: inspect restore path component %s: %w", next, statErr),
			)
		}

		identity, err := capturePlatformPathIdentity(next, true)
		if err != nil {
			return nil, nil, fmt.Errorf("kernel: validate full restore path component %s: %w", next, err)
		}

		components = append(components, validatedPlatformPathComponent{
			Path:     next,
			Identity: identity,
		})

		current = next
	}

	return components, nil, nil
}

func validateRestoreTargetPathPure(targetPath string, extRoot string) (*validatedRestorePath, error) {
	absoluteRoot, err := filepath.Abs(extRoot)
	if err != nil {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: resolve ext root: %w", err),
		)
	}

	absoluteRoot = filepath.Clean(absoluteRoot)

	realRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: eval ext root: %w", err),
		)
	}

	realRoot = filepath.Clean(realRoot)

	rootInfo, err := os.Stat(realRoot)
	if err != nil {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: stat real ext root: %w", err),
		)
	}

	if !rootInfo.IsDir() {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: ext root %s is not a directory", realRoot),
		)
	}

	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: resolve restore target: %w", err),
		)
	}

	absTargetPath = filepath.Clean(absTargetPath)

	if !pathIsWithinRoot(absoluteRoot, absTargetPath) {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: restore target %s escapes root %s", absTargetPath, absoluteRoot),
		)
	}

	if absTargetPath == absoluteRoot {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: restore target cannot equal extension root"),
		)
	}

	targetBase := filepath.Base(absTargetPath)

	if err := validatePlatformPathComponent(targetBase); err != nil {
		return nil, err
	}

	absParentDir := filepath.Clean(filepath.Dir(absTargetPath))

	if !pathIsWithinRoot(absoluteRoot, absParentDir) {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: restore parent %s escapes root %s", absParentDir, absoluteRoot),
		)
	}

	platformComponents, missingComponents, err := captureRestoreParentPlatformChain(absoluteRoot, absParentDir)
	if err != nil {
		return nil, err
	}

	if len(platformComponents) == 0 {
		return nil, NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore parent platform proof is empty"),
		)
	}

	existingAncestor := platformComponents[len(platformComponents)-1].Path

	realAncestor, err := filepath.EvalSymlinks(existingAncestor)
	if err != nil {
		return nil, NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: eval existing ancestor %s: %w", existingAncestor, err),
		)
	}

	realAncestor = filepath.Clean(realAncestor)

	if !pathIsWithinRoot(realRoot, realAncestor) {
		return nil, NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf("kernel: real ancestor %s escapes real root %s", realAncestor, realRoot),
		)
	}

	ancestorInfo, err := os.Stat(realAncestor)
	if err != nil {
		return nil, NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: stat real ancestor: %w", err),
		)
	}

	if _, targetErr := os.Lstat(absTargetPath); targetErr == nil {
		if err := validateExistingPathNoReparse(absTargetPath, false); err != nil {
			return nil, NewPackageError(
				PackageErrCodeResourceRestoreReparsePointForbidden,
				400,
				err,
			)
		}
	} else if !os.IsNotExist(targetErr) {
		return nil, NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: lstat restore target %s: %w", absTargetPath, targetErr),
		)
	}

	return &validatedRestorePath{
		AbsoluteRoot:      absoluteRoot,
		RealRoot:          realRoot,
		AbsTargetPath:     absTargetPath,
		AbsParentDir:      absParentDir,
		ExistingAncestor:  existingAncestor,
		RealAncestor:      realAncestor,
		MissingComponents: append([]string(nil), missingComponents...),
		RootInfo:          rootInfo,
		AncestorInfo:      ancestorInfo,
		PlatformComponents: append(
			[]validatedPlatformPathComponent(nil),
			platformComponents...,
		),
	}, nil
}

func revalidatePlatformPathComponents(components []validatedPlatformPathComponent) error {
	if len(components) == 0 {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: platform path component proof missing"),
		)
	}

	for index, component := range components {
		if strings.TrimSpace(component.Path) == "" {
			return NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: platform path proof component %d has empty path", index),
			)
		}

		if err := validatePlatformPathIdentity(component.Path, component.Identity, true); err != nil {
			return fmt.Errorf("kernel: platform path component %s changed: %w", component.Path, err)
		}
	}

	return nil
}

func revalidateRestorePathProof(validated *validatedRestorePath, requireParent bool) error {
	if validated == nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore path proof missing"),
		)
	}

	if err := revalidatePlatformPathComponents(validated.PlatformComponents); err != nil {
		return err
	}

	currentRealRoot, err := filepath.EvalSymlinks(validated.AbsoluteRoot)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: ext root changed after validation: %w", err),
		)
	}

	currentRealRoot = filepath.Clean(currentRealRoot)

	if currentRealRoot != validated.RealRoot {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: ext root real path changed: expected=%s actual=%s", validated.RealRoot, currentRealRoot),
		)
	}

	currentRootInfo, err := os.Stat(currentRealRoot)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: stat current real root: %w", err),
		)
	}

	if validated.RootInfo == nil || !os.SameFile(validated.RootInfo, currentRootInfo) {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: extension root filesystem identity changed"),
		)
	}

	ancestorLstat, err := os.Lstat(validated.ExistingAncestor)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: existing ancestor disappeared: %w", err),
		)
	}

	if ancestorLstat.Mode()&os.ModeSymlink != 0 || !ancestorLstat.IsDir() {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: existing ancestor changed type"),
		)
	}

	currentRealAncestor, err := filepath.EvalSymlinks(validated.ExistingAncestor)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: existing ancestor changed: %w", err),
		)
	}

	currentRealAncestor = filepath.Clean(currentRealAncestor)

	if currentRealAncestor != validated.RealAncestor {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: real ancestor changed: expected=%s actual=%s", validated.RealAncestor, currentRealAncestor),
		)
	}

	currentAncestorInfo, err := os.Stat(currentRealAncestor)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			err,
		)
	}

	if validated.AncestorInfo == nil || !os.SameFile(validated.AncestorInfo, currentAncestorInfo) {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: existing ancestor filesystem identity changed"),
		)
	}

	current := validated.ExistingAncestor

	for _, component := range validated.MissingComponents {
		if err := validatePlatformPathComponent(component); err != nil {
			return err
		}

		current = filepath.Join(current, component)

		info, statErr := os.Lstat(current)

		if os.IsNotExist(statErr) {
			if requireParent {
				return NewPackageError(
					PackageErrCodeResourceRestorePathRace,
					409,
					fmt.Errorf("kernel: required restore directory %s disappeared", current),
				)
			}

			break
		}

		if statErr != nil {
			return NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: lstat restore component %s: %w", current, statErr),
			)
		}

		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: restore component %s changed type", current),
			)
		}

		realCurrent, err := filepath.EvalSymlinks(current)
		if err != nil {
			return NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: eval restore component %s: %w", current, err),
			)
		}

		if !pathIsWithinRoot(validated.RealRoot, realCurrent) {
			return NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: restore component %s escaped real root", current),
			)
		}
	}

	if requireParent && filepath.Clean(current) != validated.AbsParentDir {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restored parent chain ended at %s, expected %s", current, validated.AbsParentDir),
		)
	}

	return nil
}

func createRestoreDirectoriesSafely(validated *validatedRestorePath) error {
	if validated == nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore path proof missing"),
		)
	}

	root, err := openPlatformRestoreRoot(validated.AbsoluteRoot)
	if err != nil {
		return NewPackageError(PackageErrCodeResourceRestorePathRace, 409, fmt.Errorf("kernel: open restore root handle: %w", err))
	}
	validated.DirectoryChain = append(validated.DirectoryChain, root)

	if len(validated.PlatformComponents) == 0 {
		_ = root.close()
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore root platform identity missing"),
		)
	}

	expectedRootIdentity := validated.PlatformComponents[0].Identity
	if !root.identity().same(expectedRootIdentity) {
		_ = root.close()
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore root identity changed between validation and handle acquisition"),
		)
	}

	current := root
	relativeParent, err := filepath.Rel(validated.AbsoluteRoot, validated.AbsParentDir)
	if err != nil {
		return err
	}

	if relativeParent == "." {
		validated.ParentDirectory = current
		return nil
	}

	missing := make(map[string]bool, len(validated.MissingComponents))
	for _, component := range validated.MissingComponents {
		missing[component] = true
	}

	currentPath := validated.AbsoluteRoot
	existingIndex := 1

	for _, component := range strings.Split(filepath.Clean(relativeParent), string(filepath.Separator)) {
		currentPath = filepath.Join(currentPath, component)

		child, created, err := current.openOrCreateChildDirectory(component)
		if err != nil {
			return NewPackageError(PackageErrCodeResourceRestorePathRace, 409, fmt.Errorf("kernel: open or create restore directory %s relative to validated handle: %w", component, err))
		}

		if missing[component] {
			if !created {
				_ = child.close()
				return NewPackageError(
					PackageErrCodeResourceRestorePathRace,
					409,
					fmt.Errorf("kernel: missing restore directory %s appeared concurrently", component),
				)
			}
			validated.PlatformComponents = append(validated.PlatformComponents, validatedPlatformPathComponent{
				Path:     currentPath,
				Identity: child.identity(),
			})
			validated.DirectoryChain = append(validated.DirectoryChain, child)
			current = child
			continue
		}

		if existingIndex >= len(validated.PlatformComponents) {
			_ = child.close()
			return NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: validated directory identity missing for %s", component),
			)
		}

		expected := validated.PlatformComponents[existingIndex]
		existingIndex++

		if !child.identity().same(expected.Identity) {
			_ = child.close()
			return NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: validated directory identity changed: %s", expected.Path),
			)
		}

		validated.DirectoryChain = append(validated.DirectoryChain, child)
		current = child
	}

	validated.ParentDirectory = current
	return nil
}

func prepareRestoreTargetPath(targetPath string, extRoot string) (*validatedRestorePath, error) {
	validated, err := validateRestoreTargetPathPure(targetPath, extRoot)
	if err != nil {
		return nil, err
	}

	if err := createRestoreDirectoriesSafely(validated); err != nil {
		_ = validated.Close()
		return nil, err
	}
	validated.TargetName = filepath.Base(validated.AbsTargetPath)

	return validated, nil
}

func ValidateRestoreTargetPath(targetPath string, extRoot string) error {
	validated, err := prepareRestoreTargetPath(targetPath, extRoot)
	if err != nil {
		return err
	}
	return validated.Close()
}

func inspectRestoreTarget(validated *validatedRestorePath, expectedHash string) (bool, error) {
	if validated == nil || validated.ParentDirectory == nil {
		return false, NewPackageError(PackageErrCodeResourceRestorePathRace, 409, fmt.Errorf("kernel: restore parent handle missing"))
	}
	target, _, err := validated.ParentDirectory.openRegularFile(validated.TargetName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer target.Close()
	actualHash, err := computeOpenFileContentHash(target)
	if err != nil {
		return false, NewPackageError(
			PackageErrCodeResourceSnapshotHashComputeFailed,
			500,
			fmt.Errorf("kernel: hash existing restore target: %w", err),
		)
	}

	if actualHash != expectedHash {
		return false, NewPackageError(
			PackageErrCodeResourceRestoreTargetChanged,
			409,
			fmt.Errorf("kernel: restore target %s already exists with different content: expected=%s actual=%s", validated.AbsTargetPath, expectedHash, actualHash),
		)
	}

	return true, nil
}

func computeOpenFileContentHash(file *os.File) (string, error) {
	if file == nil {
		return "", fmt.Errorf("kernel: file handle missing")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func captureRegularFilePlatformIdentity(path string) (platformPathIdentity, error) {
	return capturePlatformPathIdentity(path, false)
}

func createPreparedRestoreTemp(validated *validatedRestorePath, source io.Reader, expectedHash string) (string, platformPathIdentity, error) {
	if err := revalidateRestorePathProof(validated, true); err != nil {
		return "", platformPathIdentity{}, err
	}

	temp, err := validated.ParentDirectory.createTempFile(".amitia-restore-")
	if err != nil {
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			500,
			fmt.Errorf("kernel: create restore temp in %s: %w", validated.AbsParentDir, err),
		)
	}
	tempFile := temp.File

	tempPath := filepath.Join(validated.AbsParentDir, temp.Name)

	cleanup := func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}

	if filepath.Clean(filepath.Dir(tempPath)) != validated.AbsParentDir {
		cleanup()
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore temp escaped validated parent"),
		)
	}

	hasher := sha256.New()

	_, err = io.Copy(io.MultiWriter(tempFile, hasher), source)

	if err != nil {
		cleanup()
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			500,
			fmt.Errorf("kernel: write restore temp: %w", err),
		)
	}

	actualHash := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	if actualHash != expectedHash {
		cleanup()
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceSnapshotHashMismatch,
			409,
			fmt.Errorf("kernel: restore temp hash mismatch: expected=%s actual=%s", expectedHash, actualHash),
		)
	}

	if err := tempFile.Sync(); err != nil {
		cleanup()
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			500,
			fmt.Errorf("kernel: sync restore temp: %w", err),
		)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			500,
			fmt.Errorf("kernel: close restore temp: %w", err),
		)
	}

	tempInfo, err := os.Lstat(tempPath)
	if err != nil || !tempInfo.Mode().IsRegular() || tempInfo.Mode()&os.ModeSymlink != 0 {
		_ = os.Remove(tempPath)
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore temp changed after close: %v", err),
		)
	}

	tempIdentity, err := captureRegularFilePlatformIdentity(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return "", platformPathIdentity{}, NewPackageError(
			PackageErrCodeResourceRestoreReparsePointForbidden,
			400,
			fmt.Errorf("kernel: restore temp %s failed platform verification: %w", tempPath, err),
		)
	}

	if err := revalidateRestorePathProof(validated, true); err != nil {
		_ = os.Remove(tempPath)
		return "", platformPathIdentity{}, err
	}

	return tempPath, tempIdentity, nil
}

func publishPreparedRestoreTempNoReplace(validated *validatedRestorePath, tempPath string, tempIdentity platformPathIdentity, expectedHash string) error {
	if validated != nil && validated.ParentDirectory != nil {
		tempFile, identity, err := validated.ParentDirectory.openRegularFile(filepath.Base(tempPath))
		if err != nil {
			return err
		}
		if !identity.same(tempIdentity) {
			_ = tempFile.Close()
			return NewPackageError(PackageErrCodeResourceRestorePathRace, 409, fmt.Errorf("kernel: restore temp identity changed before publish"))
		}
		return publishPreparedRestoreTempHandleNoReplace(validated, &preparedRestoreTemp{Name: filepath.Base(tempPath), File: tempFile, Identity: identity}, expectedHash)
	}
	if validated == nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore path proof missing"),
		)
	}

	if filepath.Clean(filepath.Dir(tempPath)) != validated.AbsParentDir {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore temp is outside validated parent"),
		)
	}

	removePreparedTemp := true

	defer func() {
		if !removePreparedTemp {
			return
		}

		if err := validatePlatformPathIdentity(tempPath, tempIdentity, false); err == nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err := validatePlatformPathIdentity(tempPath, tempIdentity, false); err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore temp identity changed before publish: %w", err),
		)
	}

	tempHash, err := computeFileContentHash(tempPath)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceSnapshotHashComputeFailed,
			500,
			fmt.Errorf("kernel: hash restore temp before publish: %w", err),
		)
	}

	if tempHash != expectedHash {
		return NewPackageError(
			PackageErrCodeResourceSnapshotHashMismatch,
			409,
			fmt.Errorf("kernel: restore temp changed before publish"),
		)
	}

	alreadyPresent, err := inspectRestoreTarget(validated, expectedHash)
	if err != nil {
		return err
	}

	if alreadyPresent {
		return nil
	}

	if err := revalidateRestorePathProof(validated, true); err != nil {
		return err
	}

	linkErr := fmt.Errorf("kernel: legacy path publish disabled")

	if linkErr != nil {
		if os.IsExist(linkErr) {
			alreadyPresent, inspectErr := inspectRestoreTarget(validated, expectedHash)
			if inspectErr != nil {
				return inspectErr
			}

			if alreadyPresent {
				return nil
			}

			return NewPackageError(
				PackageErrCodeResourceRestoreTargetChanged,
				409,
				fmt.Errorf("kernel: restore target appeared during no-replace publish"),
			)
		}

		return NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			500,
			fmt.Errorf("kernel: no-replace hard link %s -> %s: %w", tempPath, validated.AbsTargetPath, linkErr),
		)
	}

	targetIdentity, err := captureRegularFilePlatformIdentity(validated.AbsTargetPath)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestoreReparsePointForbidden,
			400,
			fmt.Errorf("kernel: published target failed platform verification: %w", err),
		)
	}

	if !tempIdentity.same(targetIdentity) {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: published target does not share temp filesystem identity"),
		)
	}

	targetInfo, err := os.Lstat(validated.AbsTargetPath)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: published restore target disappeared: %w", err),
		)
	}

	tempInfo, err := os.Lstat(tempPath)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore temp disappeared after link: %w", err),
		)
	}

	if targetInfo.Mode()&os.ModeSymlink != 0 || !targetInfo.Mode().IsRegular() {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: published restore target changed type"),
		)
	}

	if !os.SameFile(tempInfo, targetInfo) {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: published target is not the prepared temp inode"),
		)
	}

	targetHash, err := computeFileContentHash(validated.AbsTargetPath)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceSnapshotHashComputeFailed,
			500,
			fmt.Errorf("kernel: hash published restore target: %w", err),
		)
	}

	if targetHash != expectedHash {
		return NewPackageError(
			PackageErrCodeResourceSnapshotHashMismatch,
			409,
			fmt.Errorf("kernel: published restore target hash mismatch"),
		)
	}

	if err := fsyncDir(validated.AbsParentDir); err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			409,
			fmt.Errorf("kernel: fsync restore parent after publish: %w", err),
		)
	}

	removePreparedTemp = false

	if err := os.Remove(tempPath); err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			409,
			fmt.Errorf("kernel: remove restore temp after publish: %w", err),
		)
	}

	if err := fsyncDir(validated.AbsParentDir); err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			409,
			fmt.Errorf("kernel: fsync restore parent after temp removal: %w", err),
		)
	}

	if err := revalidateRestorePathProof(validated, true); err != nil {
		return err
	}

	alreadyPresent, err = inspectRestoreTarget(validated, expectedHash)
	if err != nil {
		return err
	}

	if !alreadyPresent {
		return NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			409,
			fmt.Errorf("kernel: restore target missing after successful publish"),
		)
	}

	return nil
}

func publishRestoreBytesNoReplace(validated *validatedRestorePath, data []byte, expectedHash string) error {
	temp, err := createPreparedRestoreTempHandle(validated, bytes.NewReader(data), expectedHash)
	if err != nil {
		return err
	}
	return publishPreparedRestoreTempHandleNoReplace(validated, temp, expectedHash)
}

func publishRestoreFileNoReplace(sourceRoot string, sourcePath string, validated *validatedRestorePath, expectedHash string) error {
	sourceParent, sourceName, err := openPlatformFileParent(sourceRoot, sourcePath)
	if err != nil {
		return fmt.Errorf("kernel: open restore source parent: %w", err)
	}
	defer sourceParent.close()
	source, _, err := sourceParent.openRegularFile(sourceName)
	if err != nil {
		return fmt.Errorf("kernel: open restore source: %w", err)
	}
	defer source.Close()
	temp, err := createPreparedRestoreTempHandle(validated, source, expectedHash)
	if err != nil {
		return err
	}
	if err := publishPreparedRestoreTempHandleNoReplace(validated, temp, expectedHash); err != nil {
		return err
	}
	if err := sourceParent.removeChild(sourceName); err != nil {
		return fmt.Errorf("kernel: remove restored source: %w", err)
	}
	return sourceParent.sync()
}

func createPreparedRestoreTempHandle(validated *validatedRestorePath, source io.Reader, expectedHash string) (*preparedRestoreTemp, error) {
	if validated == nil || validated.ParentDirectory == nil {
		return nil, NewPackageError(PackageErrCodeResourceRestorePathRace, 409, fmt.Errorf("kernel: restore parent handle missing"))
	}
	temp, err := validated.ParentDirectory.createTempFile(".amitia-restore-")
	if err != nil {
		return nil, NewPackageError(PackageErrCodeResourceRestoreIncomplete, 500, fmt.Errorf("kernel: create restore temp: %w", err))
	}
	cleanup := func() { _ = temp.File.Close(); _ = validated.ParentDirectory.removeChild(temp.Name) }
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temp.File, hasher), source); err != nil {
		cleanup()
		return nil, err
	}
	if "sha256:"+hex.EncodeToString(hasher.Sum(nil)) != expectedHash {
		cleanup()
		return nil, NewPackageError(PackageErrCodeResourceSnapshotHashMismatch, 409, fmt.Errorf("kernel: restore temp hash mismatch"))
	}
	if err := temp.File.Sync(); err != nil {
		cleanup()
		return nil, err
	}
	if _, err := temp.File.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, err
	}
	return temp, nil
}

func publishPreparedRestoreTempHandleNoReplace(validated *validatedRestorePath, temp *preparedRestoreTemp, expectedHash string) error {
	if validated == nil || validated.ParentDirectory == nil || temp == nil || temp.File == nil {
		return NewPackageError(PackageErrCodeResourceRestorePathRace, 409, fmt.Errorf("kernel: restore publish handles missing"))
	}
	cleanup := func() { _ = temp.File.Close(); _ = validated.ParentDirectory.removeChild(temp.Name) }
	tempHash, err := computeOpenFileContentHash(temp.File)
	if err != nil {
		cleanup()
		return err
	}
	if tempHash != expectedHash {
		cleanup()
		return NewPackageError(PackageErrCodeResourceSnapshotHashMismatch, 409, fmt.Errorf("kernel: restore temp changed before publish"))
	}
	alreadyPresent, err := inspectRestoreTarget(validated, expectedHash)
	if err != nil {
		cleanup()
		return err
	}
	if !alreadyPresent {
		if err := validated.ParentDirectory.publishTempNoReplace(temp, validated.TargetName); err != nil {
			if again, inspectErr := inspectRestoreTarget(validated, expectedHash); inspectErr == nil && again {
				cleanup()
				return nil
			}
			cleanup()
			return err
		}
	} else {
		cleanup()
		return nil
	}
	target, targetIdentity, err := validated.ParentDirectory.openRegularFile(validated.TargetName)
	if err != nil {
		cleanup()
		return err
	}
	defer target.Close()
	if !temp.Identity.same(targetIdentity) {
		cleanup()
		return NewPackageError(PackageErrCodeResourceRestorePathRace, 409, fmt.Errorf("kernel: published target does not share temp identity"))
	}
	targetHash, err := computeOpenFileContentHash(target)
	if err != nil || targetHash != expectedHash {
		cleanup()
		return NewPackageError(PackageErrCodeResourceSnapshotHashMismatch, 409, fmt.Errorf("kernel: published restore target hash mismatch"))
	}
	if err := validated.ParentDirectory.removeChild(temp.Name); err != nil {
		_ = temp.File.Close()
		return err
	}
	if err := temp.File.Close(); err != nil {
		return err
	}
	return validated.ParentDirectory.sync()
}

func publishRestoreFileNoReplaceLegacy(sourcePath string, validated *validatedRestorePath, expectedHash string) error {
	if validated == nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: restore path proof missing"),
		)
	}

	sourceIdentity, err := captureRegularFilePlatformIdentity(sourcePath)
	if err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestoreReparsePointForbidden,
			400,
			fmt.Errorf("kernel: quarantine source %s failed platform verification: %w", sourcePath, err),
		)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("kernel: open restore source: %w", err)
	}

	sourceFileInfo, err := source.Stat()
	if err != nil {
		_ = source.Close()
		return fmt.Errorf("kernel: stat restore source %s: %w", sourcePath, err)
	}

	if sourceFileInfo.Mode()&os.ModeSymlink != 0 || !sourceFileInfo.Mode().IsRegular() {
		_ = source.Close()
		return fmt.Errorf("kernel: restore source %s is not a regular file", sourcePath)
	}

	if err := validatePlatformPathIdentity(sourcePath, sourceIdentity, false); err != nil {
		_ = source.Close()
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: quarantine source changed after open: %w", err),
		)
	}

	tempPath, tempIdentity, tempErr := createPreparedRestoreTemp(validated, source, expectedHash)

	closeErr := source.Close()

	if tempErr != nil {
		return tempErr
	}

	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}

	if err := publishPreparedRestoreTempNoReplace(validated, tempPath, tempIdentity, expectedHash); err != nil {
		return err
	}

	if err := validatePlatformPathIdentity(sourcePath, sourceIdentity, false); err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf("kernel: quarantine source changed before cleanup: %w", err),
		)
	}

	if err := os.Remove(sourcePath); err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			409,
			fmt.Errorf("kernel: target published but source cleanup failed: %w", err),
		)
	}

	if err := fsyncDir(filepath.Dir(sourcePath)); err != nil {
		return NewPackageError(
			PackageErrCodeResourceRestoreIncomplete,
			409,
			fmt.Errorf("kernel: fsync source parent after cleanup: %w", err),
		)
	}

	return nil
}
