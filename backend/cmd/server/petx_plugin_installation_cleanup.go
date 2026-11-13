package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/integration"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/gorm"
)

const petxPluginReleaseSourceType = "petx_plugin"

type petxReleaseOwnershipReader interface {
	GetPetIdentity(petID string) (*release.PetIdentityData, error)
	GetRelease(releaseID string) (*release.ReleaseData, error)
}

type petxInstallationUninstaller interface {
	Uninstall(ctx context.Context, req coordinator.UninstallRequest) (*coordinator.UninstallResult, error)
}

type petxInstallationCleanup struct {
	installations installation.Repository
	releases      petxReleaseOwnershipReader
	uninstaller   petxInstallationUninstaller
	spaceID       string
	logger        *log.Logger
}

func newPetxInstallationCleanup(installations installation.Repository, releases petxReleaseOwnershipReader, uninstaller petxInstallationUninstaller, spaceID string) *petxInstallationCleanup {
	return &petxInstallationCleanup{
		installations: installations,
		releases:      releases,
		uninstaller:   uninstaller,
		spaceID:       strings.TrimSpace(spaceID),
		logger:        log.Default(),
	}
}

func (c *petxInstallationCleanup) CleanupOwnedInstallations(ctx context.Context, req integration.OwnedInstallationCleanupRequest) error {
	extensionID := strings.TrimSpace(req.ExtensionID)
	if extensionID == "" {
		return fmt.Errorf("cleanup owned installations: extension id is empty")
	}
	spaceID := strings.TrimSpace(req.SpaceID)
	if spaceID == "" {
		spaceID = c.spaceID
	}
	if spaceID == "" {
		return fmt.Errorf("cleanup owned installations for %s: space id is empty", extensionID)
	}
	owned, err := c.installations.ListInstallationsBySpace(spaceID)
	if err != nil {
		return fmt.Errorf("cleanup owned installations for %s: list installations: %w", extensionID, err)
	}
	var cleanupErrs []error
	for _, ins := range owned {
		if ins == nil || ins.IsUninstalled() {
			continue
		}
		matched, matchErr := c.ownedByExtension(ins, extensionID)
		if matchErr != nil {
			cleanupErrs = append(cleanupErrs, matchErr)
			continue
		}
		if !matched {
			continue
		}
		result, uninstallErr := c.uninstaller.Uninstall(ctx, coordinator.UninstallRequest{
			DeviceCtx:      device.DeviceContext{SpaceID: spaceID, DeviceID: ins.DeviceID},
			InstallationID: ins.ID,
			IdempotencyKey: fmt.Sprintf("plugin-uninstall:%s:%s", extensionID, ins.ID),
			Metadata:       map[string]string{"reason": "plugin_uninstalled", "extensionId": extensionID},
		})
		if uninstallErr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("cleanup installation %s owned by %s: %w", ins.ID, extensionID, uninstallErr))
			continue
		}
		if result == nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("cleanup installation %s owned by %s: missing uninstall receipt", ins.ID, extensionID))
			continue
		}
		switch result.Status {
		case operation.OpStatusFailedTerminal, operation.OpStatusFailedRetryable, operation.OpStatusCancelled:
			cleanupErrs = append(cleanupErrs, fmt.Errorf("cleanup installation %s owned by %s: operation %s status %s: %s", ins.ID, extensionID, result.OperationID, result.Status, result.ErrorMessage))
			continue
		}
		c.logger.Printf("petx cleanup: uninstall requested installation=%s extension=%s operation=%s", ins.ID, extensionID, result.OperationID)
	}
	return errors.Join(cleanupErrs...)
}

func (c *petxInstallationCleanup) ownedByExtension(ins *installation.Installation, extensionID string) (bool, error) {
	petID := strings.TrimSpace(ins.PetID)
	if petID == "" {
		return false, nil
	}
	identity, err := c.releases.GetPetIdentity(petID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("resolve pet identity %s for installation %s: %w", petID, ins.ID, err)
	}
	if identity == nil || strings.TrimSpace(identity.UpstreamPetID) != extensionID {
		return false, nil
	}
	releaseID := strings.TrimSpace(ins.CurrentReleaseID)
	if releaseID == "" {
		return false, fmt.Errorf("installation %s owned by %s has no current release", ins.ID, extensionID)
	}
	releaseData, err := c.releases.GetRelease(releaseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, fmt.Errorf("installation %s owned by %s references missing release %s: %w", ins.ID, extensionID, releaseID, err)
		}
		return false, fmt.Errorf("resolve release %s for installation %s: %w", releaseID, ins.ID, err)
	}
	if releaseData == nil {
		return false, fmt.Errorf("installation %s owned by %s references empty release %s", ins.ID, extensionID, releaseID)
	}
	if strings.TrimSpace(releaseData.SourceType) != petxPluginReleaseSourceType {
		return false, nil
	}
	if strings.TrimSpace(releaseData.SourceProcessingTask) != extensionID {
		return false, nil
	}
	return true, nil
}
