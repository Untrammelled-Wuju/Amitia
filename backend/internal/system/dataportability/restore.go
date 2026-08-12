package dataportability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (c *Coordinator) RestorePreview(ctx context.Context, archivePath string) (*BackupManifest, error) {
	reader, err := NewArchiveReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		return nil, ErrRestoreInvalid
	}

	if manifest.Format != FormatName {
		return nil, ErrRestoreFormatUnsupported
	}

	if manifest.FormatVersion > FormatVersion {
		return nil, ErrRestoreFormatUnsupported
	}

	return manifest, nil
}

func (c *Coordinator) RestoreVerify(ctx context.Context, archivePath string) error {
	reader, err := NewArchiveReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	for _, comp := range manifest.Components {
		if !comp.Required {
			continue
		}
		path := ""
		switch comp.Kind {
		case string(KindSQLite):
			path = fmt.Sprintf("database/%s/app.db", "sqlite")
		case string(KindDataset):
			path = fmt.Sprintf("datasets/%s.ndjson", comp.ID)
		case string(KindManifest):
			path = "manifest.json"
		default:
			continue
		}
		if _, ok := reader.files[path]; !ok {
			if comp.Required {
				return ErrBackupComponentFailed
			}
		}
	}

	_ = ctx
	return nil
}

func (c *Coordinator) CreateRestoreTicket(ctx context.Context, archivePath, backupID string, manifest *BackupManifest) (*DataRestoreTicket, error) {
	staging, err := c.Staging.CreateStaging()
	if err != nil {
		return nil, ErrRestoreStagingFailed
	}

	hash, err := fileSHA256(archivePath)
	if err != nil {
		return nil, err
	}

	ticket := &DataRestoreTicket{
		OperationID:                backupID,
		BackupID:                   backupID,
		StagingPath:                staging,
		ManifestHash:               hash,
		SourceSchemaFingerprint:    manifest.SchemaFingerprint,
		ExpectedCurrentFingerprint: c.SchemaFinger,
		CreatedAt:                  time.Now().UTC().Format(time.RFC3339),
	}

	if err := c.Staging.WriteRestoreTicket(ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (c *Coordinator) ExecuteRestore(ctx context.Context, ticket *DataRestoreTicket, dbPath string, runner SnapshotRunner) error {
	backupCurrent := filepath.Join(c.DataDir, "backups", fmt.Sprintf("prerestore-%s.db", ticket.OperationID))
	if err := runner.BackupTo(backupCurrent); err != nil {
		return ErrRestorePreBackupFailed
	}

	reader, err := NewArchiveReader(filepath.Join(c.DataDir, "backups", ticket.BackupID+".amitia-backup"))
	if err != nil {
		return err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	hasSQLite := false
	for _, comp := range manifest.Components {
		if comp.Kind == string(KindSQLite) {
			hasSQLite = true
			break
		}
	}

	if hasSQLite {
		if err := RestoreSQLiteSnapshot(reader, dbPath, stagingFromTicket(ticket)); err != nil {
			return ErrRestoreAtomicReplaceFailed
		}
	}

	if manifest.SchemaFingerprint != c.SchemaFinger && len(manifest.SchemaFingerprint) > 0 {
		if err := runner.Migrate(); err != nil {
			return ErrRestoreMigrationFailed
		}
	}

	c.Staging.ClearRestoreTicket()

	_ = ctx
	return nil
}

func stagingFromTicket(ticket *DataRestoreTicket) string {
	return ticket.StagingPath
}

func RestoreSQLiteSnapshot(reader *ArchiveReader, dbPath, staging string) error {
	dbDir := filepath.Dir(dbPath)
	restoreDir := filepath.Join(dbDir, ".restore-tmp")
	if err := os.MkdirAll(restoreDir, 0o700); err != nil {
		return err
	}
	defer os.RemoveAll(restoreDir)

	mainFile, ok := reader.files["database/sqlite/app.db"]
	if !ok {
		return ErrRestoreAtomicReplaceFailed
	}

	mainData, err := mainFile.Open()
	if err != nil {
		return err
	}
	defer mainData.Close()

	tmpMain := filepath.Join(restoreDir, "app.db")
	w, err := os.OpenFile(tmpMain, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	if _, err := readAllRestore(mainData, w); err != nil {
		w.Close()
		return err
	}
	w.Close()

	if err := atomicReplace(tmpMain, dbPath); err != nil {
		return err
	}

	return nil
}

func readAllRestore(src io.Reader, dst io.Writer) (int64, error) {
	buf := make([]byte, 32*1024)
	return io.CopyBuffer(dst, src, buf)
}

func atomicReplace(src, dst string) error {
	backup := dst + ".old"
	os.Remove(backup)
	os.Rename(dst, backup)
	if err := os.Rename(src, dst); err != nil {
		os.Rename(backup, dst)
		return err
	}
	os.Remove(backup)
	os.Remove(dst + "-wal")
	os.Remove(dst + "-shm")
	return nil
}

func (c *Coordinator) HasPendingRestore() bool {
	return c.Staging.HasRestoreTicket()
}

func (c *Coordinator) ConsumeRestoreTicket() (*DataRestoreTicket, error) {
	return c.Staging.ReadRestoreTicket()
}

func (c *Coordinator) CancelRestore() error {
	return c.Staging.ClearRestoreTicket()
}

func (r *ArchiveReader) ReadFileContent(path string) ([]byte, error) {
	_ = r
	_ = path
	return nil, nil
}

func readJSONFromFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
