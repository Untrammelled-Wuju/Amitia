package dataportability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	amitialog "log"
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
		if err := os.Remove(stagedPath); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return nil, err
	}
	return ticket, nil
}

func (c *Coordinator) CommitStagedImport(ctx context.Context, id string, policy CollisionPolicy, activate bool) (*StagedImportResult, error) {
	release, err := c.BeginRestoreOperation()
	if err != nil {
		return nil, err
	}
	defer release()
	ticket, err := c.readImportTicket(id)
	if err != nil {
		return nil, err
	}
	expiresAt, err := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		return nil, errors.Join(ErrImportTicketExpired, c.cleanupImportTicket(ticket))
	}
	hash, err := fileSHA256(ticket.ArchivePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrImportTicketChanged, err)
	}
	if hash != ticket.ArchiveSHA256 {
		return nil, ErrImportTicketChanged
	}
	if policy != CollisionDuplicate && policy != CollisionSkip && policy != CollisionReplace {
		return nil, ErrImportCollisionPolicyInvalid
	}
	if ticket.Preview == nil || ticket.Preview.Manifest == nil {
		return nil, ErrImportPackageInvalid
	}
	snapshot, err := c.CreateBackup(ctx, BackupRequest{
		Scope:          ScopeAll,
		Profile:        ProfileFull,
		Purpose:        PurposePreImport,
		IncludeSecrets: true,
	}, "", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrImportTransactionFailed, err)
	}
	_, operation, err := c.executeImportWithOperation(ctx, ticket.ArchivePath, ImportRequest{
		CollisionPolicy:  policy,
		CharacterPolicy:  policy,
		ActivateImported: activate,
		Purpose:          PurposeUser,
	})
	if err != nil {
		rollbackErr := c.rollbackImport(ctx, snapshot.Path)
		if rollbackErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrRestoreRollbackFailed, errors.Join(err, rollbackErr))
		}
		return nil, err
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
	if err := c.removeImportTicket(id); err != nil {
		return nil, fmt.Errorf("%w: cleanup staged import: %w", ErrImportTransactionFailed, err)
	}
	return &StagedImportResult{
		Operation:       operation,
		PreImportBackup: snapshot.BackupID,
		Preview:         ticket.Preview,
		Stats:           stats,
	}, nil
}

func (c *Coordinator) CancelStagedImport(id string) error {
	release, err := c.BeginRestoreOperation()
	if err != nil {
		return err
	}
	defer release()
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
	return c.cleanupImportTicket(ticket)
}

func (c *Coordinator) cleanupImportTicket(ticket *ImportTicket) error {
	if ticket == nil || !validOperationID(ticket.ID) {
		return ErrImportTicketNotFound
	}
	expectedPath := filepath.Join(c.DataDir, "imports", ".staged", ticket.ID+".amitia-backup")
	var pathErr error
	if ticket.ArchivePath != expectedPath {
		pathErr = fmt.Errorf("%w: invalid staged archive path %q", ErrImportTicketChanged, ticket.ArchivePath)
	} else {
		pathErr = removeOptionalFile(ticket.ArchivePath)
	}
	ticketPath := filepath.Join(c.DataDir, "imports", ".tickets", ticket.ID+".json")
	return errors.Join(pathErr, removeOptionalFile(ticketPath))
}

func (c *Coordinator) CleanupTransientImports() error {
	importsDir := filepath.Join(c.DataDir, "imports")
	var cleanupErr error
	for _, name := range []string{".tickets", ".staged", ".staging", ".uploads"} {
		dir := filepath.Join(importsDir, name)
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		for _, entry := range entries {
			cleanupErr = errors.Join(cleanupErr, os.RemoveAll(filepath.Join(dir, entry.Name())))
		}
	}
	return cleanupErr
}

func removeOptionalFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
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
		if err := os.Remove(destination); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return err
	}
	if err := out.Close(); err != nil {
		if err := os.Remove(destination); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return err
	}
	return nil
}
