package dataportability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const importTicketTTL = 30 * time.Minute

type ImportTicket struct {
	ID            string               `json:"id"`
	ArchivePath   string               `json:"archivePath"`
	ArchiveSHA256 string               `json:"archiveSha256"`
	BackupID      string               `json:"backupId"`
	CreatedAt     string               `json:"createdAt"`
	ExpiresAt     string               `json:"expiresAt"`
	Preview       *ImportPreviewResult `json:"preview"`
}

type StagedImportResult struct {
	Operation       *ImportOperation     `json:"operation"`
	PreImportBackup string               `json:"preImportBackupId"`
	Preview         *ImportPreviewResult `json:"preview"`
	Stats           map[string]int64     `json:"stats"`
}

func (c *Coordinator) StageImport(ctx context.Context, archivePath string) (*ImportTicket, error) {
	verification, err := VerifyArchive(archivePath)
	if err != nil {
		return nil, err
	}
	if verification == nil || !verification.PackageIntegrity || verification.ComponentsVerified == 0 || verification.ComponentsFailed != 0 {
		return nil, ErrBackupVerifyFailed
	}
	preview, err := c.PreviewImport(ctx, archivePath)
	if err != nil {
		return nil, err
	}
	if preview == nil || !preview.CanImport || preview.Manifest == nil {
		return nil, ErrImportPackageInvalid
	}
	hash, err := fileSHA256(archivePath)
	if err != nil {
		return nil, err
	}
	id := "import-" + uuid.NewString()
	stagedPath := filepath.Join(c.DataDir, "imports", ".staged", id+".amitia-backup")
	if err := copyArchiveFile(archivePath, stagedPath); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	ticket := &ImportTicket{
		ID:            id,
		ArchivePath:   stagedPath,
		ArchiveSHA256: hash,
		BackupID:      preview.Manifest.BackupID,
		CreatedAt:     now.Format(time.RFC3339),
		ExpiresAt:     now.Add(importTicketTTL).Format(time.RFC3339),
		Preview:       preview,
	}
	if err := c.writeImportTicket(ticket); err != nil {
		_ = os.Remove(stagedPath)
		return nil, err
	}
	return ticket, nil
}

func (c *Coordinator) CommitStagedImport(ctx context.Context, id string, policy CollisionPolicy, activate bool) (*StagedImportResult, error) {
	ticket, err := c.readImportTicket(id)
	if err != nil {
		return nil, err
	}
	expiresAt, err := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		return nil, ErrImportTicketExpired
	}
	hash, err := fileSHA256(ticket.ArchivePath)
	if err != nil || hash != ticket.ArchiveSHA256 {
		return nil, ErrImportTicketChanged
	}
	if policy != CollisionDuplicate && policy != CollisionSkip && policy != CollisionReplace {
		return nil, ErrImportCollisionPolicyInvalid
	}
	if ticket.Preview == nil || ticket.Preview.Manifest == nil {
		return nil, ErrImportPackageInvalid
	}
	snapshot, err := c.CreateBackup(ctx, BackupRequest{
		Scope:   ScopeAll,
		Profile: ProfileFull,
		Purpose: PurposePreImport,
	}, "", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImportTransactionFailed, err)
	}
	_, operation, err := c.executeImportWithOperation(ctx, ticket.ArchivePath, ImportRequest{
		CollisionPolicy:  policy,
		CharacterPolicy:  policy,
		ActivateImported: activate,
		Purpose:          PurposeUser,
	})
	if err != nil {
		originalError := err.Error()
		if operation != nil && operation.Error != "" {
			originalError = operation.Error
		}
		_, _, rollbackErr := c.executeImportWithOperation(ctx, snapshot.Path, ImportRequest{
			CollisionPolicy:  CollisionReplace,
			CharacterPolicy:  CollisionReplace,
			ActivateImported: true,
			Purpose:          PurposePreRestore,
		})
		if rollbackErr != nil {
			return nil, fmt.Errorf("%w: import=%s rollback=%v", ErrRestoreRollbackFailed, originalError, rollbackErr)
		}
		return nil, fmt.Errorf("%w: %s", err, originalError)
	}
	stats := make(map[string]int64)
	if ticket.Preview != nil {
		for _, component := range ticket.Preview.Components {
			stats[component.ComponentID] = component.ItemCount
		}
	}
	if operation != nil {
		operation.Stats = stats
	}
	_ = c.removeImportTicket(id)
	return &StagedImportResult{
		Operation:       operation,
		PreImportBackup: snapshot.BackupID,
		Preview:         ticket.Preview,
		Stats:           stats,
	}, nil
}

func (c *Coordinator) CancelStagedImport(id string) error {
	return c.removeImportTicket(id)
}

func (c *Coordinator) readImportTicket(id string) (*ImportTicket, error) {
	if !validOperationID(id) {
		return nil, ErrImportTicketNotFound
	}
	data, err := os.ReadFile(filepath.Join(c.DataDir, "imports", ".tickets", id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrImportTicketNotFound
		}
		return nil, err
	}
	var ticket ImportTicket
	if err := json.Unmarshal(data, &ticket); err != nil {
		return nil, ErrImportPackageInvalid
	}
	return &ticket, nil
}

func (c *Coordinator) writeImportTicket(ticket *ImportTicket) error {
	dir := filepath.Join(c.DataDir, "imports", ".tickets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(ticket)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ticket.ID+".json"), data, 0o600)
}

func (c *Coordinator) removeImportTicket(id string) error {
	if !validOperationID(id) {
		return ErrImportTicketNotFound
	}
	ticket, err := c.readImportTicket(id)
	if err != nil {
		return err
	}
	_ = os.Remove(ticket.ArchivePath)
	return os.Remove(filepath.Join(c.DataDir, "imports", ".tickets", id+".json"))
}

func validOperationID(id string) bool {
	if id == "" || id != filepath.Base(id) || strings.Contains(id, "..") {
		return false
	}
	return true
}

func copyArchiveFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(destination)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return nil
}
