package extension

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type ExtensionBackupContributor struct {
	KernelDB *sql.DB
	Runtime  *kernel.Runtime
}

func NewExtensionBackupContributor(kernelDB *sql.DB, runtime *kernel.Runtime) *ExtensionBackupContributor {
	return &ExtensionBackupContributor{KernelDB: kernelDB, Runtime: runtime}
}

func (c *ExtensionBackupContributor) ID() string   { return "extension" }
func (c *ExtensionBackupContributor) Name() string { return "Extension" }

func (c *ExtensionBackupContributor) Dependencies() []string {
	return nil
}

type extensionSkillExportRecord struct {
	ExtensionID    string `json:"extension_id"`
	ContributionID string `json:"contribution_id"`
	Version        string `json:"version"`
	OwnerSpaceID   string `json:"owner_space_id"`
	ScopeType      string `json:"scope_type"`
	ScopeID        string `json:"scope_id"`
	Enabled        bool   `json:"enabled"`
	ArchiveName    string `json:"archive_name"`
	ArchiveBase64  string `json:"archive_base64"`
}

const extensionComponentID = "extension.records.v1"
const extensionLogicalName = "extension.records"

func (c *ExtensionBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if c.KernelDB == nil {
		return nil, fmt.Errorf("extension kernel database unavailable")
	}
	var count int64
	if err := c.KernelDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_contributions WHERE contribution_type = 'agent_skill'`).Scan(&count); err != nil {
		return nil, err
	}
	return []dataportability.BackupComponentPlan{{
		ID: extensionComponentID, Kind: dataportability.KindNDJSON, LogicalName: extensionLogicalName,
		Required: false, SourceOfTruth: true, ItemCount: count, EstimatedSize: count * 64 << 10,
	}}, nil
}

func (c *ExtensionBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if c.KernelDB == nil || c.Runtime == nil || c.Runtime.Container() == nil {
		return fmt.Errorf("extension kernel runtime unavailable")
	}
	component, err := out.CreateComponent(extensionComponentID, extensionLogicalName, dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer component.Close()
	rows, err := c.KernelDB.QueryContext(ctx, `
		SELECT c.extension_id, c.contribution_id, COALESCE(definition_json, ''), COALESCE(installation_json, '')
		FROM extension_contributions c
		JOIN extension_installations i ON i.extension_id = c.extension_id
		WHERE c.contribution_type = 'agent_skill'
		ORDER BY c.extension_id, c.contribution_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	container := c.Runtime.Container()
	for rows.Next() {
		var extensionID, contributionID, definitionJSON, installationJSON string
		if err := rows.Scan(&extensionID, &contributionID, &definitionJSON, &installationJSON); err != nil {
			return err
		}
		var definition struct {
			ID          string `json:"id"`
			Version     string `json:"version"`
			ExtensionID string `json:"extensionId"`
		}
		_ = json.Unmarshal([]byte(definitionJSON), &definition)
		var installation domain.ExtensionInstallation
		if err := json.Unmarshal([]byte(installationJSON), &installation); err != nil {
			return err
		}
		artifact, err := container.PackageRepository.GetArtifact(ctx, installation.PackageID)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(artifact.ArchivePath)
		if err != nil {
			return err
		}
		record := extensionSkillExportRecord{
			ExtensionID: extensionID, ContributionID: contributionID, Version: installation.InstalledVersion.String(),
			OwnerSpaceID: metadataText(installation.Metadata["ownerSpaceId"]), ScopeType: metadataText(installation.Metadata["scopeType"]),
			ScopeID: metadataText(installation.Metadata["scopeId"]), Enabled: installation.EnablementState == domain.EnablementEnabled,
			ArchiveName:   fmt.Sprintf("%s-%s.amitiax", strings.ReplaceAll(extensionID, "/", "__"), installation.InstalledVersion.String()),
			ArchiveBase64: base64.StdEncoding.EncodeToString(raw),
		}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := component.Write(data); err != nil {
			return err
		}
		if _, err := component.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (c *ExtensionBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	reader, err := in.ReadComponent(extensionComponentID)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	preview := dataportability.ImportComponentPreview{ComponentID: extensionComponentID, Kind: dataportability.KindNDJSON, LogicalName: extensionLogicalName}
	buffered := bufio.NewReader(reader)
	for {
		line, readErr := buffered.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var record extensionSkillExportRecord
			if json.Unmarshal(bytes.TrimSpace(line), &record) == nil {
				preview.ItemCount++
				var existing string
				_ = c.KernelDB.QueryRowContext(ctx, `SELECT extension_id FROM extension_installations WHERE extension_id = ?`, record.ExtensionID).Scan(&existing)
				if existing != "" {
					preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
						SourceID: record.ExtensionID, TargetID: existing, EntityType: "agent_skill", Policy: dataportability.CollisionDuplicate,
					})
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, readErr
		}
	}
	return []dataportability.ImportComponentPreview{preview}, nil
}

func (c *ExtensionBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	opts := dataportability.RestoreOptions{
		OperationID: req.OperationID, Purpose: dataportability.RestorePurposeOrdinary,
		CharacterPolicy: req.CharacterPolicy, DefaultCharacterID: req.DefaultCharacterID,
		ActivateImported: req.ActivateImported, IdentityMap: req.IdentityMap, SecretProvider: req.SecretProvider,
	}
	return c.RestoreExtensions(ctx, in, opts)
}

func (c *ExtensionBackupContributor) RestoreExtensions(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	if c.Runtime == nil || c.Runtime.Container() == nil {
		return fmt.Errorf("extension kernel runtime unavailable")
	}
	reader, err := in.ReadComponent(extensionComponentID)
	if err != nil {
		return err
	}
	defer reader.Close()
	container := c.Runtime.Container()
	buffered := bufio.NewReader(reader)
	for {
		line, readErr := buffered.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var record extensionSkillExportRecord
			if err := json.Unmarshal(bytes.TrimSpace(line), &record); err != nil {
				return err
			}
			if _, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(record.ExtensionID)); err == nil {
				switch opts.CharacterPolicy {
				case dataportability.CollisionSkip:
					if readErr != nil && readErr != io.EOF {
						return readErr
					}
					if readErr == io.EOF {
						return nil
					}
					continue
				case dataportability.CollisionReplace:
					if err := c.removePackage(ctx, record); err != nil {
						return err
					}
				default:
					return fmt.Errorf("agent skill %s already exists", record.ExtensionID)
				}
			}
			raw, err := base64.StdEncoding.DecodeString(record.ArchiveBase64)
			if err != nil {
				return err
			}
			preview, err := c.Runtime.PreviewPackage(ctx, kernel.PackagePreviewRequest{
				SpaceID: record.OwnerSpaceID, ScopeType: record.ScopeType, ScopeID: record.ScopeID,
				FileName: record.ArchiveName, AllowUnsignedLocal: true,
			}, bytes.NewReader(raw))
			if err != nil {
				return err
			}
			confirmations := map[string]bool{}
			for _, item := range preview.RequiredConfirmations {
				confirmations[item] = true
			}
			confirmation, err := c.Runtime.ConfirmPackagePreview(ctx, kernel.PackagePreviewConfirmationRequest{
				SessionID: preview.SessionID, SpaceID: record.OwnerSpaceID, ScopeType: record.ScopeType, ScopeID: record.ScopeID, Confirmations: confirmations,
			})
			if err != nil {
				return err
			}
			result, err := c.Runtime.ExecutePackageInstall(ctx, kernel.PackageInstallRequest{
				SessionID: preview.SessionID, SpaceID: record.OwnerSpaceID, ScopeType: record.ScopeType, ScopeID: record.ScopeID,
				ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "restore-agent-skill-" + record.ExtensionID + "-" + opts.OperationID,
			})
			if err != nil {
				return err
			}
			if record.Enabled || opts.ActivateImported {
				if err := c.Runtime.Enable(ctx, result.ExtensionID); err != nil {
					return err
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}
	return nil
}

func (c *ExtensionBackupContributor) removePackage(ctx context.Context, record extensionSkillExportRecord) error {
	scopeType := record.ScopeType
	if scopeType == "" {
		scopeType = "global"
	}
	preview, err := c.Runtime.PreviewPackageUninstall(ctx, record.ExtensionID, record.OwnerSpaceID, scopeType, record.ScopeID)
	if err != nil {
		return err
	}
	confirmations := map[string]bool{}
	for _, item := range kernel.RequiredUninstallConfirmations(preview) {
		confirmations[item] = true
	}
	confirmation, err := c.Runtime.ConfirmPackageUninstall(ctx, kernel.ConfirmPackageUninstallRequest{
		ExtensionID: record.ExtensionID, SpaceID: record.OwnerSpaceID, ScopeType: scopeType, ScopeID: record.ScopeID, Confirmations: confirmations,
	})
	if err != nil {
		return err
	}
	_, err = c.Runtime.ExecutePackageUninstall(ctx, kernel.ExecutePackageUninstallRequest{
		ExtensionID: record.ExtensionID, SpaceID: record.OwnerSpaceID, ScopeType: scopeType, ScopeID: record.ScopeID,
		ConfirmationToken: confirmation.Token, IdempotencyKey: "restore-replace-" + preview.PreviewHash,
	})
	return err
}

func metadataText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

var _ dataportability.ExtensionRestorePort = (*ExtensionBackupContributor)(nil)
