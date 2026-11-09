package kernel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/packageext"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type BundledPluginIssue struct {
	Path  string
	Error string
}

func ProvisionBundledPlugins(ctx context.Context, runtime *Runtime, root, spaceID string) ([]BundledPluginIssue, error) {
	if runtime == nil || runtime.container == nil {
		return nil, errors.New("kernel: bundled plugin provisioning requires an attached container")
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return []BundledPluginIssue{{Path: root, Error: fmt.Sprintf("inspect bundled plugin directory: %v", err)}}, nil
	}
	if !info.IsDir() {
		return []BundledPluginIssue{{Path: root, Error: "bundled plugin path is not a directory"}}, nil
	}
	paths := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !packageext.IsArchivePath(entry.Name()) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return []BundledPluginIssue{{Path: root, Error: fmt.Sprintf("scan bundled plugins: %v", err)}}, nil
	}
	paths = selectLatestBundledPluginPaths(paths)
	sort.Strings(paths)
	issues := make([]BundledPluginIssue, 0)
	for _, path := range paths {
		if err := provisionBundledPlugin(ctx, runtime, path, spaceID); err != nil {
			issues = append(issues, BundledPluginIssue{Path: path, Error: err.Error()})
		}
	}
	return issues, nil
}

var bundledPluginVersionSuffix = regexp.MustCompile(`^(.*)-(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?)$`)

func selectLatestBundledPluginPaths(paths []string) []string {
	selected := make(map[string]string, len(paths))
	versions := make(map[string]domain.SemanticVersion, len(paths))
	order := make([]string, 0, len(paths))
	for _, path := range paths {
		archiveName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		match := bundledPluginVersionSuffix.FindStringSubmatch(archiveName)
		if match == nil {
			key := "path:" + path
			selected[key] = path
			order = append(order, key)
			continue
		}
		version, err := domain.ParseVersion(match[2])
		if err != nil {
			key := "path:" + path
			selected[key] = path
			order = append(order, key)
			continue
		}
		key := "package:" + filepath.Dir(path) + "\x00" + match[1]
		current, exists := selected[key]
		if !exists {
			selected[key] = path
			versions[key] = version
			order = append(order, key)
			continue
		}
		if version.Compare(versions[key]) > 0 || version.Compare(versions[key]) == 0 && path < current {
			selected[key] = path
			versions[key] = version
		}
	}
	result := make([]string, 0, len(selected))
	for _, key := range order {
		if path, exists := selected[key]; exists {
			result = append(result, path)
			delete(selected, key)
		}
	}
	return result
}

func provisionBundledPlugin(ctx context.Context, runtime *Runtime, path, spaceID string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open package: %w", err)
	}
	preview, previewErr := runtime.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID:        spaceID,
		ScopeType:      "global",
		FileName:       filepath.Base(path),
		TrustedBundled: true,
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
	defaultInstall, _ := preview.Manifest.Plugin.Metadata["system.defaultInstall"].(bool)
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
	defaultEnabled, _ := preview.Manifest.Plugin.Metadata["system.defaultEnabled"].(bool)
	extensionID := domain.ExtensionID(preview.ExtensionID)
	if err := reconcileBundledPluginReadModel(ctx, runtime, extensionID, spaceID, defaultEnabled); err != nil {
		return fmt.Errorf("reconcile installation read model: %w", err)
	}
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
		IdempotencyKey:      fmt.Sprintf("bundled:%s:%s:%s:%s", preview.ExtensionID, preview.Version, preview.SessionID, preview.ArchiveHash),
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

func reconcileBundledPluginReadModel(ctx context.Context, runtime *Runtime, extensionID domain.ExtensionID, spaceID string, defaultEnabled bool) error {
	if runtime == nil || runtime.container == nil || runtime.container.PackageGenerationStore == nil {
		return nil
	}
	current, err := runtime.container.PackageGenerationStore.ReadCurrent(string(extensionID))
	if errors.Is(err, ErrPackageGenerationNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	versions, err := runtime.container.PackageRepository.ListPackageVersions(ctx, string(extensionID))
	if err != nil {
		return err
	}
	if verifyErr := runtime.container.PackageGenerationStore.VerifyGeneration(ctx, current); verifyErr != nil {
		repaired, repairErr := recoverBundledPluginGeneration(ctx, runtime, extensionID, current, versions)
		if repairErr != nil {
			return repairErr
		}
		current = repaired
	}
	var version PackageVersionRecord
	found := false
	for _, item := range versions {
		if item.GenerationID == current.GenerationID && item.ArtifactID == current.ArtifactID && item.Version == current.Version {
			version = item
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	installed, installedErr := runtime.container.InstallationRepository.GetInstallation(ctx, extensionID)
	if installedErr != nil && !errors.Is(installedErr, domain.ErrInvalidExtensionID) {
		return installedErr
	}
	parsedVersion, err := domain.ParseVersion(current.Version)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if installedErr != nil {
		installed = domain.ExtensionInstallation{
			InstallationID:    "installation-" + uuid.NewString(),
			ExtensionID:       extensionID,
			InstalledAt:       now,
			Generation:        1,
			InstallationState: domain.InstallationStateInstalled,
			EnablementState:   domain.EnablementDisabled,
			Metadata:          map[string]any{},
		}
		if defaultEnabled {
			installed.EnablementState = domain.EnablementEnabled
		}
	}
	if installed.Metadata == nil {
		installed.Metadata = map[string]any{}
	}
	installed.InstalledVersion = parsedVersion
	installed.PackageID = current.ArtifactID
	installed.UpdatedAt = now
	installed.Metadata["ownerSpaceId"] = spaceID
	installed.Metadata["scopeType"] = "global"
	installed.Metadata["scopeId"] = ""
	installed.Metadata["generationId"] = current.GenerationID
	installed.Metadata["currentPath"] = version.InstalledPath
	installed.Metadata["installedPath"] = version.InstalledPath
	installed.Metadata["installedTreeHash"] = version.InstalledTreeHash
	installed.Metadata["artifactId"] = current.ArtifactID
	installed.Metadata["archiveHash"] = version.ArchiveHash
	installed.Metadata["manifestHash"] = version.ManifestHash
	installed.Metadata["contentTreeHash"] = version.ContentTreeHash
	installed.Metadata["currentVersionId"] = version.VersionID
	installed.Metadata["lastOperationId"] = current.OperationID
	installed.Metadata["fencingToken"] = float64(current.FencingToken)
	if runtime.container.TransactionManager == nil {
		return runtime.container.InstallationRepository.PutInstallation(ctx, installed)
	}
	return runtime.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := runtime.container.InstallationRepository.PutInstallation(txCtx, installed); err != nil {
			return err
		}
		tx, ok := kernelsqlite.TxFromContext(txCtx)
		if !ok {
			return errors.New("kernel: installation reconciliation transaction unavailable")
		}
		return runtime.container.PackageRepository.ActivatePackageVersionTx(
			txCtx,
			tx,
			PackageWriteGuard{},
			string(extensionID),
			version.VersionID,
			current.GenerationID,
		)
	})
}

func recoverBundledPluginGeneration(ctx context.Context, runtime *Runtime, extensionID domain.ExtensionID, current PackageGenerationCurrent, versions []PackageVersionRecord) (PackageGenerationCurrent, error) {
	for _, version := range versions {
		if strings.TrimSpace(version.InstalledPath) == "" || strings.TrimSpace(version.InstalledTreeHash) == "" || strings.TrimSpace(version.InstallOperationID) == "" {
			continue
		}
		info, err := os.Stat(version.InstalledPath)
		if err != nil || !info.IsDir() {
			continue
		}
		treeHash, err := runtime.container.PackageGenerationStore.ComputeGenerationTreeHash(ctx, string(extensionID), version.GenerationID)
		if err != nil || !equalTreeHash(treeHash, version.InstalledTreeHash) {
			continue
		}
		fencingToken := current.FencingToken
		if fencingToken < 1 {
			fencingToken = 1
		}
		next := PackageGenerationCurrent{
			ExtensionID:  string(extensionID),
			GenerationID: version.GenerationID,
			Version:      version.Version,
			ArtifactID:   version.ArtifactID,
			TreeHash:     version.InstalledTreeHash,
			OperationID:  version.InstallOperationID,
			FencingToken: fencingToken,
		}
		if err := runtime.container.PackageGenerationStore.SwitchCurrent(string(extensionID), current.GenerationID, next); err != nil {
			latest, readErr := runtime.container.PackageGenerationStore.ReadCurrent(string(extensionID))
			if readErr == nil && runtime.container.PackageGenerationStore.VerifyGeneration(ctx, latest) == nil {
				return latest, nil
			}
			return PackageGenerationCurrent{}, err
		}
		return runtime.container.PackageGenerationStore.ReadCurrent(string(extensionID))
	}
	return PackageGenerationCurrent{}, ErrPackageGenerationNotFound
}
