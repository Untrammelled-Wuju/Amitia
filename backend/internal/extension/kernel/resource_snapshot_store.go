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
	content_hash TEXT NOT NULL DEFAULT '',
	storage_reference TEXT NOT NULL DEFAULT '',
	quarantine_path TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'quarantined',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (quarantine_id)
)`

type ResourceQuarantineState string

const (
	ResourceQuarantined ResourceQuarantineState = "quarantined"
	ResourcePurged      ResourceQuarantineState = "purged"
	ResourceRestored    ResourceQuarantineState = "restored"
)

type ResourceQuarantineEntry struct {
	QuarantineID   string
	OperationID    string
	ExtensionID    string
	ResourceID     string
	LogicalPath    string
	ContentHash    string
	StorageRef     string
	QuarantinePath string
	State          ResourceQuarantineState
	CreatedAt      string
	UpdatedAt      string
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
	_, err := s.db.ExecContext(ctx, resourceQuarantineDDL)
	return err
}

func (s *ResourceSnapshotStore) VerifyResourceSnapshotEntries(ctx context.Context, snapshotJSON string) error {
	var snapshot packageResourceSnapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return fmt.Errorf("kernel: resource snapshot corrupt: %w", err)
	}
	for _, entry := range snapshot.Entries {
		raw, err := json.Marshal(entry.Resource)
		if err != nil {
			return fmt.Errorf("kernel: marshal resource for verification: %w", err)
		}
		if entry.ResourceHash != packageSnapshotDigest(raw) {
			return fmt.Errorf("kernel: resource hash mismatch for %s", entry.Resource.ResourceID)
		}
		if entry.ContentHash != "" && !verifyResourceContentHash(entry, raw) {
			return fmt.Errorf("kernel: resource content hash mismatch for %s", entry.Resource.ResourceID)
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
	absExtRoot, _ := filepath.Abs(s.extRoot)
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
		contentHash := extractResourceStringField(resource, "contentHash")
		storageRef := resource.Reference
		if storageRef == "" {
			storageRef = resource.ResourceID
		}
		quarantineID := "resource-quarantine-" + operationID + "-" + resource.ResourceID
		quarantinePath := filepath.Join(quarantineDir, resource.ResourceID)
		if storageRef != "" {
			absStorage, _ := filepath.Abs(storageRef)
			if strings.HasPrefix(absStorage, absExtRoot+string(filepath.Separator)) {
				if _, statErr := os.Stat(absStorage); statErr == nil {
					if renameErr := os.Rename(absStorage, quarantinePath); renameErr != nil {
						if copyErr := copyResourceFile(absStorage, quarantinePath); copyErr == nil {
							os.Remove(absStorage)
						}
					}
				}
			}
		}
		entry := ResourceQuarantineEntry{
			QuarantineID:   quarantineID,
			OperationID:    operationID,
			ExtensionID:    extensionID,
			ResourceID:     resource.ResourceID,
			LogicalPath:    logicalPath,
			ContentHash:    contentHash,
			StorageRef:     storageRef,
			QuarantinePath: quarantinePath,
			State:          ResourceQuarantined,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
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
	for _, entry := range entries {
		if entry.QuarantinePath != "" && entry.StorageRef != "" {
			if _, statErr := os.Stat(entry.QuarantinePath); statErr == nil {
				destDir := filepath.Dir(entry.StorageRef)
				if mkErr := os.MkdirAll(destDir, 0755); mkErr != nil {
					return fmt.Errorf("kernel: restore quarantined resource %s: mkdir: %w", entry.ResourceID, mkErr)
				}
				if renameErr := os.Rename(entry.QuarantinePath, entry.StorageRef); renameErr != nil {
					if copyErr := copyResourceFile(entry.QuarantinePath, entry.StorageRef); copyErr != nil {
						return fmt.Errorf("kernel: restore quarantined resource %s: %w", entry.ResourceID, copyErr)
					}
					os.Remove(entry.QuarantinePath)
				}
			}
		}
		if err := s.updateQuarantineEntryState(ctx, entry.QuarantineID, ResourceRestored); err != nil {
			return err
		}
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
		 WHERE operation_id=? AND state=?`, operationID, string(ResourceQuarantined),
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("kernel: query active resource quarantine: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("kernel: %d resource(s) still in quarantine for operation %s", count, operationID)
	}
	return nil
}

func (s *ResourceSnapshotStore) ComputeResourceTreeHash(ctx context.Context, extensionID string, resources []domain.ResourceOwnership) string {
	if len(resources) == 0 {
		return ""
	}
	hasher := sha256.New()
	for _, resource := range resources {
		raw, err := json.Marshal(resource)
		if err != nil {
			continue
		}
		hasher.Write(raw)
		hasher.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func (s *ResourceSnapshotStore) persistQuarantineEntry(ctx context.Context, entry ResourceQuarantineEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO extension_package_resource_quarantine
		 (quarantine_id, operation_id, extension_id, resource_id, logical_path, content_hash,
		  storage_reference, quarantine_path, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.QuarantineID, entry.OperationID, entry.ExtensionID, entry.ResourceID,
		entry.LogicalPath, entry.ContentHash, entry.StorageRef, entry.QuarantinePath,
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
		`SELECT quarantine_id, operation_id, extension_id, resource_id, logical_path, content_hash,
		        storage_reference, quarantine_path, state, created_at, updated_at
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
			&entry.ResourceID, &entry.LogicalPath, &entry.ContentHash,
			&entry.StorageRef, &entry.QuarantinePath, &entry.State,
			&entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan quarantine entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func verifyResourceContentHash(entry packageResourceSnapshotEntry, raw []byte) bool {
	if entry.ContentHash == "" {
		return true
	}
	computed := packageSnapshotDigest(raw)
	if entry.ContentHash == computed {
		return true
	}
	if stored, ok := entry.Resource.Metadata["contentHash"]; ok {
		if storedStr, ok := stored.(string); ok && storedStr == entry.ContentHash {
			return true
		}
	}
	return false
}

func copyResourceFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
		return mkErr
	}
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}
