package dataportability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const batchSize = 1000

type BackupResult struct {
	BackupID  string          `json:"backupId"`
	Path      string          `json:"path"`
	SizeBytes int64           `json:"sizeBytes"`
	Checksum  string          `json:"checksum"`
	Manifest  *BackupManifest `json:"manifest"`
}

func GenerateBackupID() string {
	return "backup-" + uuid.NewString()
}

func (c *Coordinator) CreateBackup(ctx context.Context, req BackupRequest, dbPath string, runner SnapshotRunner) (*BackupResult, error) {
	opID := GenerateBackupID()
	op := &BackupOperation{
		ID:        opID,
		Status:    "planning",
		Profile:   string(req.Profile),
		Scope:     string(req.Scope),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	c.mu.Lock()
	c.backupOps[opID] = op
	c.mu.Unlock()

	backupDir := filepath.Join(c.DataDir, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return nil, c.failBackup(opID, err)
	}

	if err := c.CheckDiskSpace(req, backupDir); err != nil {
		return nil, c.failBackup(opID, err)
	}

	op.Status = "running"

	manifest := &BackupManifest{
		Format:            FormatName,
		FormatVersion:     FormatVersion,
		BackupID:          opID,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		AppVersion:        c.AppVersion,
		SchemaFingerprint: c.SchemaFinger,
		Profile:           string(req.Profile),
		Scope:             string(req.Scope),
		CharacterID:       req.CharacterID,
		SourcePlatform:    c.Platform,
		Encrypted:         false,
	}

	pkgPath := filepath.Join(backupDir, opID+".amitia-backup")
	writer := NewArchiveWriter(pkgPath)
	defer writer.Close()

	for _, ct := range c.Contributors {
		plans, err := ct.Plan(ctx, req)
		if err != nil {
			return nil, c.failBackup(opID, fmt.Errorf("contributor %s plan failed: %w", ct.ID(), err))
		}
		for _, plan := range plans {
			compWriter, err := writer.CreateComponent(plan.ID, plan.LogicalName, plan.Kind)
			if err != nil {
				return nil, c.failBackup(opID, err)
			}
			hash := sha256.New()
			counting := &countingWriter{w: io.MultiWriter(compWriter, hash)}
			adapter := &componentWriterAdapter{w: counting, plan: plan, writer: writer}
			if err := ct.Export(ctx, req, adapter); err != nil {
				_ = compWriter.Close()
				return nil, c.failBackup(opID, err)
			}
			if err := compWriter.Close(); err != nil {
				return nil, c.failBackup(opID, err)
			}
			manifest.Components = append(manifest.Components, BackupComponentManifest{
				ID:            plan.ID,
				Kind:          string(plan.Kind),
				LogicalName:   plan.LogicalName,
				Path:          fmt.Sprintf("datasets/%s.ndjson", plan.ID),
				SizeBytes:     counting.n,
				ItemCount:     plan.ItemCount,
				SHA256:        hex.EncodeToString(hash.Sum(nil)),
				Required:      plan.Required,
				SourceOfTruth: plan.SourceOfTruth,
				Rebuildable:   plan.Rebuildable,
				Sensitive:     plan.Sensitive,
			})
		}
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, c.failBackup(opID, err)
	}
	if err := writer.WriteManifest(manifestData); err != nil {
		return nil, c.failBackup(opID, err)
	}

	if err := writer.Finalize(); err != nil {
		return nil, c.failBackup(opID, err)
	}

	op.Status = "completed"
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	info, err := os.Stat(pkgPath)
	if err != nil {
		return nil, err
	}

	checksum, err := fileSHA256(pkgPath)
	if err != nil {
		return nil, err
	}

	return &BackupResult{
		BackupID:  opID,
		Path:      pkgPath,
		SizeBytes: info.Size(),
		Checksum:  checksum,
		Manifest:  manifest,
	}, nil
}

func (c *Coordinator) failBackup(opID string, err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if op, ok := c.backupOps[opID]; ok {
		op.Status = "failed"
		op.Error = err.Error()
		op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return err
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

type componentWriterAdapter struct {
	w      io.Writer
	plan   BackupComponentPlan
	writer *ArchiveWriter
	count  int64
}

func (a *componentWriterAdapter) CreateComponent(id, logicalName string, kind ComponentKind) (io.WriteCloser, error) {
	return &nopWriteCloser{a.w}, nil
}

func (a *componentWriterAdapter) WriteJSON(id string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = a.w.Write(data)
	return err
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type SnapshotRunner interface {
	CheckpointWAL()
	RunIntegrityCheck()
	RunForeignKeyCheck()
	GetSQLiteFiles() []SQLiteFileInfo
	BackupTo(destPath string) error
	Migrate() error
}

type SQLiteFileInfo struct {
	Name string
	Path string
}
