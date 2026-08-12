package dataportability

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const batchSize = 1000

type BackupResult struct {
	BackupID  string `json:"backupId"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	Checksum  string `json:"checksum"`
	Manifest  *BackupManifest `json:"manifest"`
}

func GenerateBackupID() string {
	return fmt.Sprintf("backup-%s", time.Now().UTC().Format("20060102T150405.000000000"))
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
			continue
		}
		for _, plan := range plans {
			compWriter, err := writer.CreateComponent(plan.ID, plan.LogicalName, plan.Kind)
			if err != nil {
				return nil, c.failBackup(opID, err)
			}
			if err := ct.Export(ctx, req, &componentWriterAdapter{w: compWriter, plan: plan, writer: writer}); err != nil {
				compWriter.Close()
				return nil, c.failBackup(opID, err)
			}
			compWriter.Close()
		}
	}

	if req.Profile == ProfileFull && dbPath != "" {
		snapComp, err := c.snapshotSQLite(runner, writer)
		if err != nil {
			return nil, c.failBackup(opID, err)
		}
		manifest.Components = append(manifest.Components, *snapComp)
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

func (c *Coordinator) snapshotSQLite(runner SnapshotRunner, writer *ArchiveWriter) (*BackupComponentManifest, error) {
	runner.CheckpointWAL()
	runner.RunIntegrityCheck()
	runner.RunForeignKeyCheck()

	dbFiles := runner.GetSQLiteFiles()
	totalSize := int64(0)
	for _, f := range dbFiles {
		info, err := os.Stat(f.Path)
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	comp := &BackupComponentManifest{
		ID:             "sqlite-main",
		Kind:           string(KindSQLite),
		LogicalName:    "database/sqlite",
		Path:           "database/sqlite",
		Required:       true,
		SourceOfTruth:  true,
		Rebuildable:    false,
		SizeBytes:      totalSize,
		SHA256:         "",
	}

	for _, f := range dbFiles {
		if err := writer.CopyFile("database/sqlite", f.Name, f.Path); err != nil {
			return nil, err
		}
	}

	return comp, nil
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

func randomComponentID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
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
