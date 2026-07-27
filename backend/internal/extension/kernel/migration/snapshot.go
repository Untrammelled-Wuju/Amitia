package migration

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
	"time"
)

type SnapshotManager struct {
	baseDir string
	db      *sql.DB
}

func NewSnapshotManager(baseDir string, db *sql.DB) *SnapshotManager {
	return &SnapshotManager{
		baseDir: baseDir,
		db:      db,
	}
}

func (m *SnapshotManager) CreateSnapshot(ctx context.Context, extensionID, operationID string, generation int64, entries []SnapshotEntry) (*SnapshotManifest, error) {
	snapshotID := fmt.Sprintf("snap-%s-%d", extensionID, time.Now().UnixNano())
	snapDir := m.snapshotDir(m.baseDir, snapshotID)

	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}

	var totalBytes int64
	createdEntries := make([]SnapshotEntry, 0, len(entries))

	for i := range entries {
		entry := entries[i]
		entry.SnapshotID = snapshotID
		entry.EntryID = fmt.Sprintf("%s-entry-%d", snapshotID, i)

		relPath := m.relPathFromSource(entry.SourcePath)
		entry.SnapPath = filepath.Join(snapDir, relPath)

		switch entry.Type {
		case SnapshotEntryFile:
			size, hash, err := m.copyFile(entry.SourcePath, entry.SnapPath)
			if err != nil {
				return nil, fmt.Errorf("copy file %s: %w", entry.SourcePath, err)
			}
			entry.SizeBytes = size
			entry.Hash = hash
		case SnapshotEntryDirectory:
			size, hash, err := m.copyDir(entry.SourcePath, entry.SnapPath)
			if err != nil {
				return nil, fmt.Errorf("copy dir %s: %w", entry.SourcePath, err)
			}
			entry.SizeBytes = size
			entry.Hash = hash
		case SnapshotEntrySQLite:
			size, hash, err := m.copyFile(entry.SourcePath, entry.SnapPath)
			if err != nil {
				return nil, fmt.Errorf("copy sqlite %s: %w", entry.SourcePath, err)
			}
			entry.SizeBytes = size
			entry.Hash = hash
			entry.WALHandled = true
		default:
			return nil, fmt.Errorf("unknown snapshot entry type: %s", entry.Type)
		}

		totalBytes += entry.SizeBytes
		createdEntries = append(createdEntries, entry)
	}

	manifest := &SnapshotManifest{
		SnapshotID:      snapshotID,
		ExtensionID:     extensionID,
		OperationID:     operationID,
		Generation:      generation,
		Entries:         createdEntries,
		TotalBytes:      totalBytes,
		ManifestHash:    m.computeManifestHash(createdEntries),
		CreatedAt:       time.Now(),
		RetentionPolicy: "until_override",
	}

	if err := m.saveManifest(ctx, manifest); err != nil {
		return nil, fmt.Errorf("save manifest: %w", err)
	}

	return manifest, nil
}

func (m *SnapshotManager) RestoreSnapshot(ctx context.Context, snapshotID string) error {
	manifest, err := m.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return err
	}

	for _, entry := range manifest.Entries {
		switch entry.Type {
		case SnapshotEntryFile, SnapshotEntrySQLite:
			if err := os.MkdirAll(filepath.Dir(entry.SourcePath), 0o755); err != nil {
				return fmt.Errorf("create source dir for %s: %w", entry.SourcePath, err)
			}
			if _, _, err := m.copyFile(entry.SnapPath, entry.SourcePath); err != nil {
				return fmt.Errorf("restore file %s: %w", entry.SourcePath, err)
			}
		case SnapshotEntryDirectory:
			if err := os.MkdirAll(filepath.Dir(entry.SourcePath), 0o755); err != nil {
				return fmt.Errorf("create source dir for %s: %w", entry.SourcePath, err)
			}
			if _, _, err := m.copyDir(entry.SnapPath, entry.SourcePath); err != nil {
				return fmt.Errorf("restore dir %s: %w", entry.SourcePath, err)
			}
		}
	}

	return nil
}

func (m *SnapshotManager) VerifySnapshot(ctx context.Context, snapshotID string) (*ValidationResult, error) {
	manifest, err := m.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	result := &ValidationResult{
		Passed:   true,
		Errors:   []string{},
		Warnings: []string{},
	}

	for _, entry := range manifest.Entries {
		switch entry.Type {
		case SnapshotEntryFile, SnapshotEntrySQLite:
			info, err := os.Stat(entry.SnapPath)
			if err != nil {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("entry %s: snap file missing: %v", entry.EntryID, err))
				continue
			}
			if info.Size() != entry.SizeBytes {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("entry %s: size mismatch (expected %d, got %d)", entry.EntryID, entry.SizeBytes, info.Size()))
			}
			hash, err := m.hashFile(entry.SnapPath)
			if err != nil {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("entry %s: hash compute failed: %v", entry.EntryID, err))
				continue
			}
			if hash != entry.Hash {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("entry %s: hash mismatch (expected %s, got %s)", entry.EntryID, entry.Hash, hash))
			}
		case SnapshotEntryDirectory:
			info, err := os.Stat(entry.SnapPath)
			if err != nil {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("entry %s: snap dir missing: %v", entry.EntryID, err))
				continue
			}
			if !info.IsDir() {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("entry %s: expected directory, got file", entry.EntryID))
			}
		}
	}

	return result, nil
}

func (m *SnapshotManager) EstimateSpace(ctx context.Context, extensionID string, domains []DataDomain) (*SpaceEstimate, error) {
	var currentDataBytes int64

	for _, domain := range domains {
		size, err := m.pathSize(domain.Storage)
		if err != nil {
			return nil, fmt.Errorf("estimate size for domain %s: %w", domain.Domain, err)
		}
		currentDataBytes += size
	}

	snapshotBytes := currentDataBytes
	temporaryBytes := currentDataBytes / 2
	safetyMarginBytes := currentDataBytes / 10
	totalRequired := snapshotBytes + temporaryBytes + safetyMarginBytes

	est := &SpaceEstimate{
		CurrentDataBytes:   currentDataBytes,
		TargetStagingBytes: currentDataBytes,
		SnapshotBytes:      snapshotBytes,
		TemporaryBytes:     temporaryBytes,
		SafetyMarginBytes:  safetyMarginBytes,
		TotalRequired:      totalRequired,
		AvailableBytes:     0,
		Sufficient:         true,
	}

	return est, nil
}

func (m *SnapshotManager) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	snapDir := m.snapshotDir(m.baseDir, snapshotID)
	if err := os.RemoveAll(snapDir); err != nil {
		return fmt.Errorf("remove snapshot dir: %w", err)
	}

	_, err := m.db.ExecContext(ctx, `DELETE FROM extension_snapshot_entries WHERE snapshot_id = ?`, snapshotID)
	if err != nil {
		return fmt.Errorf("delete snapshot entries: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `DELETE FROM extension_data_snapshots WHERE snapshot_id = ?`, snapshotID)
	if err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}

	return nil
}

func (m *SnapshotManager) GetSnapshot(ctx context.Context, snapshotID string) (*SnapshotManifest, error) {
	var (
		extID           string
		opID            sql.NullString
		generation      int64
		totalBytes      int64
		manifestHash    string
		retentionPolicy string
		createdAt       time.Time
	)

	err := m.db.QueryRowContext(ctx, `
		SELECT extension_id, operation_id, generation, total_bytes, manifest_hash, retention_policy, created_at
		FROM extension_data_snapshots
		WHERE snapshot_id = ?
	`, snapshotID).Scan(&extID, &opID, &generation, &totalBytes, &manifestHash, &retentionPolicy, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("query snapshot %s: %w", snapshotID, err)
	}

	manifest := &SnapshotManifest{
		SnapshotID:      snapshotID,
		ExtensionID:     extID,
		OperationID:     opID.String,
		Generation:      generation,
		TotalBytes:      totalBytes,
		ManifestHash:    manifestHash,
		CreatedAt:       createdAt,
		RetentionPolicy: retentionPolicy,
	}

	rows, err := m.db.QueryContext(ctx, `
		SELECT entry_id, entry_type, source_path, snap_path, size_bytes, hash, page_count, wal_handled
		FROM extension_snapshot_entries
		WHERE snapshot_id = ?
	`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("query snapshot entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			entryID    string
			entryType  string
			sourcePath string
			snapPath   string
			sizeBytes  int64
			hash       string
			pageCount  int
			walHandled int
		)
		if err := rows.Scan(&entryID, &entryType, &sourcePath, &snapPath, &sizeBytes, &hash, &pageCount, &walHandled); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		manifest.Entries = append(manifest.Entries, SnapshotEntry{
			EntryID:    entryID,
			SnapshotID: snapshotID,
			Type:       SnapshotEntryType(entryType),
			SourcePath: sourcePath,
			SnapPath:   snapPath,
			SizeBytes:  sizeBytes,
			Hash:       hash,
			PageCount:  pageCount,
			WALHandled: walHandled != 0,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entries: %w", err)
	}

	return manifest, nil
}

func (m *SnapshotManager) copyFile(src, dst string) (int64, string, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, "", fmt.Errorf("create dst dir: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return 0, "", fmt.Errorf("open src: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, "", fmt.Errorf("create dst: %w", err)
	}
	defer dstFile.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(dstFile, hasher)

	size, err := io.Copy(writer, srcFile)
	if err != nil {
		return 0, "", fmt.Errorf("copy data: %w", err)
	}

	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (m *SnapshotManager) copyDir(src, dst string) (int64, string, error) {
	var totalSize int64
	hasher := sha256.New()

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		size, fileHash, err := m.copyFile(path, targetPath)
		if err != nil {
			return err
		}
		totalSize += size
		hasher.Write([]byte(fileHash))
		return nil
	})

	if err != nil {
		return 0, "", err
	}

	return totalSize, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (m *SnapshotManager) hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (m *SnapshotManager) computeManifestHash(entries []SnapshotEntry) string {
	data, err := json.Marshal(entries)
	if err != nil {
		return ""
	}
	hasher := sha256.New()
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}

func (m *SnapshotManager) snapshotDir(baseDir, snapshotID string) string {
	return filepath.Join(baseDir, snapshotID)
}

func (m *SnapshotManager) saveManifest(ctx context.Context, manifest *SnapshotManifest) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO extension_data_snapshots (snapshot_id, extension_id, operation_id, generation, total_bytes, manifest_hash, retention_policy, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, manifest.SnapshotID, manifest.ExtensionID, manifest.OperationID, manifest.Generation, manifest.TotalBytes, manifest.ManifestHash, manifest.RetentionPolicy, manifest.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}

	for _, entry := range manifest.Entries {
		walHandled := 0
		if entry.WALHandled {
			walHandled = 1
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO extension_snapshot_entries (entry_id, snapshot_id, entry_type, source_path, snap_path, size_bytes, hash, page_count, wal_handled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, entry.EntryID, entry.SnapshotID, string(entry.Type), entry.SourcePath, entry.SnapPath, entry.SizeBytes, entry.Hash, entry.PageCount, walHandled)
		if err != nil {
			return fmt.Errorf("insert entry %s: %w", entry.EntryID, err)
		}
	}

	return tx.Commit()
}

func (m *SnapshotManager) pathSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}
	var total int64
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func (m *SnapshotManager) relPathFromSource(sourcePath string) string {
	relPath := filepath.Clean(sourcePath)
	if vol := filepath.VolumeName(relPath); vol != "" {
		relPath = relPath[len(vol):]
	}
	for len(relPath) > 0 && (relPath[0] == '/' || relPath[0] == '\\') {
		relPath = relPath[1:]
	}
	if relPath == "" || relPath == "." {
		relPath = "data"
	}
	return relPath
}
