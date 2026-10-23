package dataportability

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (c *Coordinator) RestorePreview(ctx context.Context, archivePath string) (*BackupManifest, error) {
	verification, err := VerifyArchive(archivePath)
	if err != nil {
		return nil, err
	}
	if verification == nil || !verification.PackageIntegrity || verification.ComponentsVerified == 0 || verification.ComponentsFailed != 0 {
		return nil, ErrRestoreIntegrityFailed
	}
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
	if manifest.SchemaFingerprint != "" && !compatibleSchemaFingerprint(manifest.SchemaFingerprint, c.SchemaFinger) {
		return nil, ErrRestoreSchemaUnsupported
	}
	_ = ctx
	return manifest, nil
}

func compatibleSchemaFingerprint(source, current string) bool {
	if source == current {
		return true
	}
	var sourceMigrations, currentMigrations int
	if _, err := fmt.Sscanf(source, "mig-%d-", &sourceMigrations); err == nil {
		if _, err := fmt.Sscanf(current, "migrations-v1-%d", &currentMigrations); err == nil {
			return sourceMigrations == currentMigrations
		}
	}
	return false
}

func (c *Coordinator) RestoreVerify(ctx context.Context, archivePath string) error {
	_, err := c.RestorePreview(ctx, archivePath)
	return err
}

func (c *Coordinator) CreateRestoreTicket(ctx context.Context, archivePath, backupID string, manifest *BackupManifest) (*DataRestoreTicket, error) {
	if !validOperationID(backupID) {
		return nil, ErrRestoreInvalid
	}
	previewManifest, err := c.RestorePreview(ctx, archivePath)
	if err != nil {
		return nil, err
	}
	if manifest != nil && manifest.BackupID != "" && manifest.BackupID != previewManifest.BackupID {
		return nil, ErrRestoreInvalid
	}
	stagedPath := filepath.Join(c.DataDir, "imports", ".restore", backupID+".amitia-backup")
	if err := copyArchiveFile(archivePath, stagedPath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreStagingFailed, err)
	}
	hash, err := fileSHA256(stagedPath)
	if err != nil {
		_ = os.Remove(stagedPath)
		return nil, err
	}
	ticket := &DataRestoreTicket{
		OperationID:                "restore-" + uuid.NewString(),
		BackupID:                   backupID,
		ArchivePath:                stagedPath,
		ArchiveSHA256:              hash,
		ManifestHash:               hash,
		SourceSchemaFingerprint:    previewManifest.SchemaFingerprint,
		ExpectedCurrentFingerprint: c.SchemaFinger,
		CreatedAt:                  time.Now().UTC().Format(time.RFC3339),
	}
	if err := c.Staging.WriteRestoreTicket(ticket); err != nil {
		_ = os.Remove(stagedPath)
		return nil, err
	}
	return ticket, nil
}

func (c *Coordinator) ExecuteRestore(ctx context.Context, ticket *DataRestoreTicket, _ string, _ SnapshotRunner) (*RestoreOperation, error) {
	if ticket == nil || !validOperationID(ticket.BackupID) || ticket.ArchivePath == "" {
		return nil, ErrRestoreInvalid
	}
	hash, err := fileSHA256(ticket.ArchivePath)
	if err != nil || hash != ticket.ArchiveSHA256 {
		return nil, ErrRestoreIntegrityFailed
	}
	manifest, err := c.RestorePreview(ctx, ticket.ArchivePath)
	if err != nil {
		return nil, err
	}
	op := &RestoreOperation{
		ID:        ticket.OperationID,
		BackupID:  ticket.BackupID,
		Status:    "preparing",
		Stats:     make(map[string]int64),
		CreatedAt: nowRFC3339(),
	}
	c.mu.Lock()
	c.restoreOps[op.ID] = op
	c.mu.Unlock()
	snapshot, err := c.CreateBackup(ctx, BackupRequest{
		Scope:   ScopeAll,
		Profile: ProfileFull,
		Purpose: PurposePreRestore,
	}, "", nil)
	if err != nil {
		op.Status = "failed"
		op.Error = err.Error()
		op.UpdatedAt = nowRFC3339()
		return op, fmt.Errorf("%w: %v", ErrRestorePreBackupFailed, err)
	}
	op.PreRestoreBackup = snapshot.BackupID
	ticket.PreRestoreBackupID = snapshot.BackupID
	if err := c.Staging.WriteRestoreTicket(ticket); err != nil {
		op.Status = "failed"
		op.Error = err.Error()
		op.UpdatedAt = nowRFC3339()
		return op, err
	}
	op.Status = "restoring"
	_, importOp, err := c.executeImportWithOperation(ctx, ticket.ArchivePath, ImportRequest{
		CollisionPolicy:  CollisionReplace,
		CharacterPolicy:  CollisionReplace,
		ActivateImported: true,
		Purpose:          PurposePreRestore,
	})
	if err != nil {
		op.Status = "rollback"
		op.Error = err.Error()
		_, _, rollbackErr := c.executeImportWithOperation(ctx, snapshot.Path, ImportRequest{
			CollisionPolicy:  CollisionReplace,
			CharacterPolicy:  CollisionReplace,
			ActivateImported: true,
			Purpose:          PurposePreRestore,
		})
		if rollbackErr != nil {
			op.Status = "rollback_failed"
			op.Error = fmt.Errorf("%w: %v", ErrRestoreRollbackFailed, rollbackErr).Error()
			op.UpdatedAt = nowRFC3339()
			return op, ErrRestoreRollbackFailed
		}
		op.Status = "rolled_back"
		op.UpdatedAt = nowRFC3339()
		return op, err
	}
	if importOp != nil {
		op.Stats = importOp.Stats
	}
	for _, component := range manifest.Components {
		op.Stats[component.ID] = component.ItemCount
	}
	op.Status = "completed"
	op.UpdatedAt = nowRFC3339()
	_ = c.Staging.ClearRestoreTicket()
	_ = os.Remove(ticket.ArchivePath)
	return op, nil
}

func (c *Coordinator) HasPendingRestore() bool {
	return c.Staging.HasRestoreTicket()
}

func (c *Coordinator) ConsumeRestoreTicket() (*DataRestoreTicket, error) {
	return c.Staging.ReadRestoreTicket()
}

func (c *Coordinator) CancelRestore() error {
	ticket, err := c.Staging.ReadRestoreTicket()
	if err == nil && ticket.ArchivePath != "" && strings.HasPrefix(filepath.Clean(ticket.ArchivePath), filepath.Join(c.DataDir, "imports", ".restore")) {
		_ = os.Remove(ticket.ArchivePath)
	}
	return c.Staging.ClearRestoreTicket()
}
