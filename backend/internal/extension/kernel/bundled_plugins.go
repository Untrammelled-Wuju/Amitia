package kernel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func ProvisionBundledPlugins(ctx context.Context, runtime *Runtime, root, spaceID string) error {
	if runtime == nil || runtime.container == nil {
		return errors.New("kernel: bundled plugin provisioning requires an attached container")
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("kernel: inspect bundled plugin directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("kernel: bundled plugin path is not a directory: %s", root)
	}
	paths := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".amitiax") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("kernel: scan bundled plugins: %w", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := provisionBundledPlugin(ctx, runtime, path, spaceID); err != nil {
			return fmt.Errorf("kernel: provision bundled plugin %s: %w", path, err)
		}
	}
	return nil
}

func provisionBundledPlugin(ctx context.Context, runtime *Runtime, path, spaceID string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open package: %w", err)
	}
	preview, previewErr := runtime.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID:   spaceID,
		ScopeType: "global",
		FileName:  filepath.Base(path),
		Source: PackageSourceProvenance{
			URI:        "bundled:" + filepath.Base(path),
			CatalogID:  "bundled",
			SourceType: "bundled",
			Publisher:  "com.amitia",
			TrustLevel: "official",
		},
	}, file)
	closeErr := file.Close()
	if previewErr != nil {
		return fmt.Errorf("preview package: %w", previewErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close package: %w", closeErr)
	}
	defaultInstall, _ := preview.Manifest.Extension.Metadata["system.defaultInstall"].(bool)
	if !defaultInstall {
		return nil
	}
	if !preview.Installable {
		issues := make([]string, 0, len(preview.Issues))
		for _, issue := range preview.Issues {
			detail := strings.TrimSpace(issue.Code)
			if message := strings.TrimSpace(issue.Message); message != "" {
				detail = strings.TrimSpace(detail + ": " + message)
			}
			if path := strings.TrimSpace(issue.Path); path != "" {
				detail = strings.TrimSpace(detail + " [" + path + "]")
			}
			if detail != "" {
				issues = append(issues, detail)
			}
		}
		if len(issues) == 0 {
			return errors.New("preview package is not installable")
		}
		return fmt.Errorf("preview package is not installable: %s", strings.Join(issues, "; "))
	}
	defaultEnabled, _ := preview.Manifest.Extension.Metadata["system.defaultEnabled"].(bool)
	extensionID := domain.ExtensionID(preview.ExtensionID)
	bundledVersion, err := domain.ParseVersion(preview.Version)
	if err != nil {
		return fmt.Errorf("parse bundled package version %q: %w", preview.Version, err)
	}
	installed, installedErr := runtime.container.InstallationRepository.GetInstallation(ctx, extensionID)
	exists := installedErr == nil
	if installedErr != nil && !errors.Is(installedErr, domain.ErrInvalidExtensionID) {
		return fmt.Errorf("read installation: %w", installedErr)
	}
	if exists && installed.InstalledVersion.Compare(bundledVersion) >= 0 {
		if defaultEnabled && !installed.IsUserDisabled() && installed.EnablementState != domain.EnablementEnabled {
			if err := runtime.Enable(ctx, preview.ExtensionID); err != nil {
				return fmt.Errorf("enable already installed package: %w", err)
			}
		}
		return nil
	}
	confirmations := make(map[string]bool, len(preview.RequiredConfirmations))
	for _, confirmation := range preview.RequiredConfirmations {
		if value := strings.TrimSpace(confirmation); value != "" {
			confirmations[value] = true
		}
	}
	confirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID:     preview.SessionID,
		SpaceID:       spaceID,
		ScopeType:     "global",
		Confirmations: confirmations,
	})
	if err != nil {
		return fmt.Errorf("confirm package preview: %w", err)
	}
	request := PackageInstallRequest{
		SessionID:           preview.SessionID,
		SpaceID:             spaceID,
		ScopeType:           "global",
		Confirmations:       confirmations,
		ConfirmationToken:   confirmation.ConfirmationToken,
		ExpectedExtensionID: preview.ExtensionID,
		IdempotencyKey:      fmt.Sprintf("bundled:%s:%s:%s", preview.ExtensionID, preview.Version, preview.ArchiveHash),
	}
	if exists {
		_, err = runtime.ExecutePackageUpdate(ctx, request)
	} else {
		_, err = runtime.ExecutePackageInstall(ctx, request)
	}
	if err != nil {
		return fmt.Errorf("execute package transaction: %w", err)
	}
	if !defaultEnabled {
		return nil
	}
	after, err := runtime.container.InstallationRepository.GetInstallation(ctx, extensionID)
	if err != nil {
		return fmt.Errorf("read installation after package transaction: %w", err)
	}
	if !after.IsUserDisabled() && after.EnablementState != domain.EnablementEnabled {
		if err := runtime.Enable(ctx, preview.ExtensionID); err != nil {
			return fmt.Errorf("enable package: %w", err)
		}
	}
	return nil
}
