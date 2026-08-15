package dataportability

import (
	"context"
	"fmt"
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

func (c *Coordinator) ExecuteRestore(ctx context.Context, ticket *DataRestoreTicket, _ string, _ SnapshotRunner) error {
	archivePath := filepath.Join(c.DataDir, "backups", ticket.BackupID+".amitia-backup")
	reader, err := NewArchiveReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	if manifest.SchemaFingerprint != c.SchemaFinger && len(manifest.SchemaFingerprint) > 0 {
		return ErrRestoreMigrationFailed
	}

	br := &archiveBackupReader{r: reader}
	sortedContributors := c.sortedContributorsForImport()

	opID := generateOpID()
	op := &ImportOperation{
		ID:        opID,
		Status:    "restoring",
		Scope:     string(manifest.Scope),
		CreatedAt: nowRFC3339(),
		Stats:     make(map[string]int64),
	}
	c.mu.Lock()
	c.importOps[opID] = op
	c.mu.Unlock()

	idMap := NewImportIdentityMap()
	req := ImportRequest{
		OperationID:      opID,
		StagingPath:      ticket.StagingPath,
		Manifest:         manifest,
		CharacterPolicy:  CollisionReplace,
		ActivateImported: true,
		IdentityMap:      idMap,
		Purpose:          PurposePreRestore,
	}

	op.Status = "importing"
	for _, ct := range sortedContributors {
		if err := ct.Import(ctx, req, br); err != nil {
			op.Status = "failed"
			op.Error = err.Error()
			return fmt.Errorf("canonical restore contributor %s failed: %w", ct.ID(), err)
		}
		op.Stats[ct.ID()]++
	}

	op.Status = "completed"
	op.UpdatedAt = nowRFC3339()

	c.Staging.ClearRestoreTicket()

	_ = ctx
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

