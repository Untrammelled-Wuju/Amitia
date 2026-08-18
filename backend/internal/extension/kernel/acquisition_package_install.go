package kernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

// acquisitionPackageInstaller is the only package-install adapter exposed to
// capability acquisition. It deliberately re-enters Runtime's canonical
// Preview -> Confirmation -> PackageInstallSaga pipeline rather than executing
// lifecycle_manager's direct install plan.
type acquisitionPackageInstaller struct {
	container func() *Container
}

func newAcquisitionPackageInstaller(container func() *Container) acquisition.CanonicalPackageInstallPort {
	return &acquisitionPackageInstaller{container: container}
}

func (a *acquisitionPackageInstaller) runtime() (*Runtime, *Container, error) {
	if a == nil || a.container == nil {
		return nil, nil, fmt.Errorf("acquisition package installer: container unavailable")
	}
	c := a.container()
	if c == nil || c.PackageRepository == nil || c.PackageArtifactStore == nil {
		return nil, nil, fmt.Errorf("acquisition package installer: package services unavailable")
	}
	rt, err := NewRuntime(c.ExtRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("acquisition package installer: create runtime: %w", err)
	}
	rt.SetContainer(c)
	if c.Store != nil {
		rt.SetSagaRepo(NewLifecycleSagaRepository(c.Store.DB()))
	}
	return rt, c, nil
}

func (a *acquisitionPackageInstaller) InstallArtifact(ctx context.Context, artifactID, extensionID, version, expectedHash, userID string) (string, error) {
	if strings.TrimSpace(artifactID) == "" {
		return "", fmt.Errorf("acquisition package installer: artifact id required")
	}
	if strings.TrimSpace(userID) == "" {
		userID = "system:capability-acquisition"
	}
	rt, c, err := a.runtime()
	if err != nil {
		return "", err
	}
	artifact, err := c.PackageRepository.GetArtifact(ctx, artifactID)
	if err != nil {
		return "", fmt.Errorf("acquisition package installer: artifact %s unavailable: %w", artifactID, err)
	}
	if expectedHash != "" && !equalPackageHash(expectedHash, artifact.ArchiveHash) {
		return "", fmt.Errorf("acquisition package installer: artifact hash mismatch")
	}
	f, err := os.Open(artifact.ArchivePath)
	if err != nil {
		return "", fmt.Errorf("acquisition package installer: open artifact: %w", err)
	}
	defer f.Close()

	preview, err := rt.PreviewPackage(ctx, PackagePreviewRequest{
		UserID:    userID,
		ScopeType: "global",
		FileName:  filepath.Base(artifact.ArchivePath),
	}, f)
	if err != nil {
		return "", fmt.Errorf("acquisition package installer: preview: %w", err)
	}
	if !preview.Installable {
		return "", fmt.Errorf("acquisition package installer: package preview is not installable")
	}
	if extensionID != "" && preview.ExtensionID != extensionID {
		return "", fmt.Errorf("acquisition package installer: extension identity mismatch: expected %s got %s", extensionID, preview.ExtensionID)
	}
	if version != "" && preview.Version != version {
		return "", fmt.Errorf("acquisition package installer: version mismatch: expected %s got %s", version, preview.Version)
	}
	confirmations := make(map[string]bool, len(preview.RequiredConfirmations))
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	confirmation, err := rt.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID:     preview.SessionID,
		UserID:        userID,
		ScopeType:     "global",
		Confirmations: confirmations,
	})
	if err != nil {
		return "", fmt.Errorf("acquisition package installer: confirmation: %w", err)
	}
	result, err := rt.ExecutePackageInstall(ctx, PackageInstallRequest{
		SessionID:           preview.SessionID,
		UserID:              userID,
		ScopeType:           "global",
		Confirmations:       confirmations,
		ConfirmationToken:   confirmation.ConfirmationToken,
		ExpectedExtensionID: extensionID,
		IdempotencyKey:      "capability-acquisition:" + artifactID + ":" + userID,
	})
	if err != nil {
		return "", fmt.Errorf("acquisition package installer: canonical install saga: %w", err)
	}
	return result.OperationID, nil
}

func (a *acquisitionPackageInstaller) UninstallExtension(ctx context.Context, extensionID, userID string) error {
	if strings.TrimSpace(extensionID) == "" {
		return nil
	}
	if strings.TrimSpace(userID) == "" {
		userID = "system:capability-acquisition"
	}
	rt, _, err := a.runtime()
	if err != nil {
		return err
	}
	preview, err := rt.PreviewPackageUninstall(ctx, extensionID, userID, "global", "")
	if err != nil {
		return fmt.Errorf("acquisition package installer: uninstall preview: %w", err)
	}
	if !preview.Installable {
		return fmt.Errorf("acquisition package installer: uninstall preview is not installable")
	}
	confirmations := make(map[string]bool)
	for _, item := range requiredUninstallConfirmations(preview) {
		confirmations[item] = true
	}
	confirmation, err := rt.ConfirmPackageUninstall(ctx, ConfirmPackageUninstallRequest{
		ExtensionID:   extensionID,
		UserID:        userID,
		ScopeType:     "global",
		Confirmations: confirmations,
	})
	if err != nil {
		return fmt.Errorf("acquisition package installer: uninstall confirmation: %w", err)
	}
	if _, err := rt.ExecutePackageUninstall(ctx, ExecutePackageUninstallRequest{
		ExtensionID:       extensionID,
		UserID:            userID,
		ScopeType:         "global",
		ConfirmationToken: confirmation.Token,
	}); err != nil {
		return fmt.Errorf("acquisition package installer: canonical uninstall saga: %w", err)
	}
	return nil
}

func equalPackageHash(expected, actual string) bool {
	normalize := func(v string) string {
		v = strings.TrimSpace(strings.ToLower(v))
		return strings.TrimPrefix(v, "sha256:")
	}
	return normalize(expected) == normalize(actual)
}
