package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type ExportRequest struct {
	Scope       dataportability.BackupScope
	Profile     dataportability.BackupProfile
	CharacterID string
	Passphrase  string
}

func (s *service) PreviewDataPortabilityExport(ctx context.Context, request ExportRequest) (map[string]interface{}, error) {
	if s.coordinator == nil {
		return nil, dataportability.ErrBackupUnavailable
	}
	if request.Profile == "" {
		request.Profile = dataportability.ProfilePortable
	}
	backupRequest := dataportability.BackupRequest{
		Scope:       request.Scope,
		Profile:     request.Profile,
		CharacterID: request.CharacterID,
		Purpose:     dataportability.PurposeMigration,
		Passphrase:  request.Passphrase,
	}
	if err := dataportability.ValidateBackupRequest(backupRequest); err != nil {
		return nil, err
	}
	components := make([]map[string]interface{}, 0)
	var estimated int64
	for _, contributor := range s.coordinator.Contributors {
		plans, err := contributor.Plan(ctx, backupRequest)
		if err != nil {
			return nil, err
		}
		for _, plan := range plans {
			estimated += plan.EstimatedSize
			components = append(components, map[string]interface{}{
				"id":            plan.ID,
				"logicalName":   plan.LogicalName,
				"kind":          plan.Kind,
				"itemCount":     plan.ItemCount,
				"required":      plan.Required,
				"sensitive":     plan.Sensitive,
				"rebuildable":   plan.Rebuildable,
				"estimatedSize": plan.EstimatedSize,
			})
		}
	}
	sort.Slice(components, func(i, j int) bool {
		return fmt.Sprint(components[i]["id"]) < fmt.Sprint(components[j]["id"])
	})
	return map[string]interface{}{
		"scope":         request.Scope,
		"profile":       request.Profile,
		"characterId":   request.CharacterID,
		"encrypted":     request.Passphrase != "",
		"components":    components,
		"estimatedSize": estimated,
	}, nil
}

func (s *service) ExportDataPortability(ctx context.Context, request ExportRequest) (map[string]interface{}, error) {
	if s.coordinator == nil {
		return nil, dataportability.ErrBackupUnavailable
	}
	if request.Profile == "" {
		request.Profile = dataportability.ProfilePortable
	}
	backupRequest := dataportability.BackupRequest{
		Scope:       request.Scope,
		Profile:     request.Profile,
		CharacterID: request.CharacterID,
		Purpose:     dataportability.PurposeMigration,
		Passphrase:  request.Passphrase,
	}
	result, err := s.coordinator.CreateBackup(ctx, backupRequest, filepath.Join(s.dataDir, "app.db"), nil)
	if err != nil {
		return nil, err
	}
	exportDir := filepath.Join(s.dataDir, "exports")
	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		return nil, err
	}
	fileName := filepath.Base(result.Path)
	exportPath := filepath.Join(exportDir, fileName)
	if err := os.Rename(result.Path, exportPath); err != nil {
		_ = os.Remove(exportPath)
		if copyErr := copyDataPortabilityFile(result.Path, exportPath); copyErr != nil {
			return nil, copyErr
		}
		_ = os.Remove(result.Path)
	}
	components := make([]map[string]interface{}, 0, len(result.Manifest.Components))
	for _, component := range result.Manifest.Components {
		components = append(components, map[string]interface{}{
			"id":          component.ID,
			"logicalName": component.LogicalName,
			"kind":        component.Kind,
			"itemCount":   component.ItemCount,
			"sensitive":   component.Sensitive,
		})
	}
	return map[string]interface{}{
		"operationId": result.BackupID,
		"file":        fileName,
		"size":        result.SizeBytes,
		"checksum":    result.Checksum,
		"scope":       result.Manifest.Scope,
		"profile":     result.Manifest.Profile,
		"characterId": result.Manifest.CharacterID,
		"encrypted":   result.Manifest.Encrypted,
		"components":  components,
		"createdAt":   result.Manifest.CreatedAt,
		"appVersion":  result.Manifest.AppVersion,
	}, nil
}

func (s *service) InspectDataPortabilityImport(ctx context.Context, uploadPath, passphrase string) (map[string]interface{}, error) {
	if s.coordinator == nil {
		return nil, dataportability.ErrBackupUnavailable
	}
	archivePath := uploadPath
	var decryptedPath string
	if dataportability.IsEncryptedFile(uploadPath) {
		if passphrase == "" {
			return nil, dataportability.ErrBackupPassphraseRequired
		}
		decryptedPath = filepath.Join(s.dataDir, "imports", ".uploads", uuid.NewString()+".amitia-backup")
		if err := dataportability.DecryptFile(uploadPath, decryptedPath, passphrase); err != nil {
			return nil, err
		}
		archivePath = decryptedPath
	}
	ticket, err := s.coordinator.StageImport(ctx, archivePath)
	if decryptedPath != "" {
		_ = os.Remove(decryptedPath)
	}
	_ = os.Remove(uploadPath)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ticketId":           ticket.ID,
		"backupId":           ticket.BackupID,
		"createdAt":          ticket.CreatedAt,
		"expiresAt":          ticket.ExpiresAt,
		"preview":            ticket.Preview,
		"requiresPassphrase": false,
	}, nil
}

func (s *service) CommitDataPortabilityImport(ctx context.Context, ticketID string, policy dataportability.CollisionPolicy, activate bool) (map[string]interface{}, error) {
	if s.coordinator == nil {
		return nil, dataportability.ErrBackupUnavailable
	}
	result, err := s.coordinator.CommitStagedImport(ctx, ticketID, policy, activate)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ticketId":          ticketID,
		"operationId":       result.Operation.ID,
		"status":            result.Operation.Status,
		"preImportBackupId": result.PreImportBackup,
		"stats":             result.Stats,
		"totalImported":     sumDataPortabilityStats(result.Stats),
		"componentCount":    len(result.Stats),
		"requiresReindex":   result.Preview != nil && result.Preview.RequiresReindex,
	}, nil
}

func (s *service) CancelDataPortabilityImport(ticketID string) error {
	if s.coordinator == nil {
		return dataportability.ErrBackupUnavailable
	}
	return s.coordinator.CancelStagedImport(ticketID)
}

func (s *service) CreateDataPortabilityBackup(ctx context.Context, passphrase string) (map[string]interface{}, error) {
	if s.coordinator == nil {
		return nil, dataportability.ErrBackupUnavailable
	}
	result, err := s.coordinator.CreateBackup(ctx, dataportability.BackupRequest{
		Scope:      dataportability.ScopeAll,
		Profile:    dataportability.ProfileFull,
		Purpose:    dataportability.PurposeUser,
		Passphrase: passphrase,
	}, filepath.Join(s.dataDir, "app.db"), nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"backupId":       result.BackupID,
		"name":           filepath.Base(result.Path),
		"size":           result.SizeBytes,
		"checksum":       result.Checksum,
		"encrypted":      result.Manifest.Encrypted,
		"createdAt":      result.Manifest.CreatedAt,
		"appVersion":     result.Manifest.AppVersion,
		"componentCount": len(result.Manifest.Components),
	}, nil
}

func (s *service) PreviewDataPortabilityRestore(ctx context.Context, name, passphrase string) (map[string]interface{}, error) {
	if s.coordinator == nil {
		return nil, dataportability.ErrBackupUnavailable
	}
	path, err := s.dataPortabilityBackupPath(name)
	if err != nil {
		return nil, err
	}
	archivePath := path
	var decryptedPath string
	if dataportability.IsEncryptedFile(path) {
		if passphrase == "" {
			return nil, dataportability.ErrBackupPassphraseRequired
		}
		decryptedPath = filepath.Join(s.dataDir, "imports", ".uploads", uuid.NewString()+".amitia-backup")
		if err := dataportability.DecryptFile(path, decryptedPath, passphrase); err != nil {
			return nil, err
		}
		archivePath = decryptedPath
	}
	manifest, err := s.coordinator.RestorePreview(ctx, archivePath)
	if err != nil {
		if decryptedPath != "" {
			_ = os.Remove(decryptedPath)
		}
		return nil, err
	}
	ticket, err := s.coordinator.CreateRestoreTicket(ctx, archivePath, manifest.BackupID, manifest)
	if decryptedPath != "" {
		_ = os.Remove(decryptedPath)
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"operationId": ticket.OperationID,
		"backupId":    ticket.BackupID,
		"createdAt":   ticket.CreatedAt,
		"manifest":    manifest,
		"encrypted":   dataportability.IsEncryptedFile(path),
	}, nil
}

func (s *service) ExecuteDataPortabilityRestore(ctx context.Context, operationID string) (map[string]interface{}, error) {
	if s.coordinator == nil {
		return nil, dataportability.ErrBackupUnavailable
	}
	ticket, err := s.coordinator.ConsumeRestoreTicket()
	if err != nil {
		return nil, err
	}
	if ticket.OperationID != operationID {
		return nil, dataportability.ErrRestoreInvalid
	}
	operation, err := s.coordinator.ExecuteRestore(ctx, ticket, filepath.Join(s.dataDir, "app.db"), nil)
	if operation == nil {
		return nil, err
	}
	result := map[string]interface{}{
		"operationId":        operation.ID,
		"backupId":           operation.BackupID,
		"status":             operation.Status,
		"preRestoreBackupId": operation.PreRestoreBackup,
		"stats":              operation.Stats,
		"requiresRestart":    true,
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *service) CancelDataPortabilityRestore() error {
	if s.coordinator == nil {
		return dataportability.ErrBackupUnavailable
	}
	return s.coordinator.CancelRestore()
}

func (s *service) dataPortabilityBackupPath(name string) (string, error) {
	clean := filepath.Base(name)
	if clean == "." || clean == "" || clean != name || strings.Contains(clean, string(filepath.Separator)+"..") {
		return "", dataportability.ErrBackupNotFound
	}
	path := filepath.Join(s.dataDir, "backups", clean)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", dataportability.ErrBackupNotFound
	}
	return path, nil
}

func (s *service) DeleteDataPortabilityBackup(name string) error {
	path, err := s.dataPortabilityBackupPath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *service) ListDataPortabilityBackups() (map[string]interface{}, error) {
	dir := filepath.Join(s.dataDir, "backups")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]interface{}{"backups": []interface{}{}}, nil
		}
		return nil, err
	}
	backups := make([]map[string]interface{}, 0)
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, map[string]interface{}{
			"name":      entry.Name(),
			"size":      info.Size(),
			"createdAt": info.ModTime().Format(time.DateTime),
			"encrypted": strings.HasSuffix(entry.Name(), ".enc"),
		})
	}
	sort.Slice(backups, func(i, j int) bool {
		return fmt.Sprint(backups[i]["createdAt"]) > fmt.Sprint(backups[j]["createdAt"])
	})
	return map[string]interface{}{"backups": backups}, nil
}

func (s *service) DataPortabilityExportPath(name string) (string, error) {
	clean := filepath.Base(name)
	if clean == "." || clean == "" || clean != name {
		return "", dataportability.ErrBackupNotFound
	}
	path := filepath.Join(s.dataDir, "exports", clean)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", dataportability.ErrBackupNotFound
	}
	return path, nil
}

func copyDataPortabilityFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := output.ReadFrom(input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func sumDataPortabilityStats(stats map[string]int64) int64 {
	var total int64
	for _, count := range stats {
		total += count
	}
	return total
}
