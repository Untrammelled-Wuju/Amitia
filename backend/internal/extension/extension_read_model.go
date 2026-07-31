package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type ExtensionReadModelService struct {
	proxy   *KernelLifecycleProxy
	repo    *Repository
	counter *kernelruntime.LegacyReadCounter
}

func NewExtensionReadModelService(proxy *KernelLifecycleProxy, repo *Repository) *ExtensionReadModelService {
	return &ExtensionReadModelService{
		proxy:   proxy,
		repo:    repo,
		counter: kernelruntime.GlobalLegacyReadCounter(),
	}
}

func (s *ExtensionReadModelService) available() bool {
	return s.proxy != nil && s.proxy.ReadContainer() != nil
}

func (s *ExtensionReadModelService) authoritativeContainer() (*kernelruntime.Container, error) {
	if !s.available() {
		return nil, fmt.Errorf("readmodel: extension kernel unavailable")
	}
	container := s.proxy.ReadContainer()
	if container.InstallationRepository == nil {
		return nil, fmt.Errorf("readmodel: installation repository not injected")
	}
	return container, nil
}

func (s *ExtensionReadModelService) TryPreviewUninstall(ctx context.Context, extensionID string) (PackageUninstallPreview, bool, error) {
	container, err := s.authoritativeContainer()
	if err != nil {
		return PackageUninstallPreview{}, false, err
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidExtensionID) {
			return PackageUninstallPreview{}, false, nil
		}
		return PackageUninstallPreview{}, false, fmt.Errorf("readmodel: installation repository unavailable: %w", err)
	}
	if container.DependencyResolver == nil || container.ContributionRepository == nil || container.ModuleRepository == nil ||
		container.RuntimeSupervisor == nil || container.ScheduleRepository == nil || container.PermissionBroker == nil || container.ScopeRepository == nil {
		return PackageUninstallPreview{}, false, fmt.Errorf("readmodel: uninstall read dependencies not injected")
	}
	preview := PackageUninstallPreview{
		ExtensionID:      extensionID,
		CurrentVersion:   installation.InstalledVersion.String(),
		Enabled:          installation.EnablementState == domain.EnablementEnabled,
		Dependents:       []PackageDependencyView{},
		Grants:           []string{},
		Cleanup:          []string{},
		Preserved:        []string{},
		ArtifactArchived: true,
		ReadSource:       "kernel",
	}
	preview.Dependents, err = s.readReverseDependencies(ctx, container, extensionID)
	if err != nil {
		return PackageUninstallPreview{}, false, err
	}
	preview.ContributionSummary, preview.EventSubscriptions, err = s.readContributions(ctx, container, extensionID)
	if err != nil {
		return PackageUninstallPreview{}, false, err
	}
	preview.RuntimeImpacts, err = s.readRuntimeImpacts(ctx, container, extensionID)
	if err != nil {
		return PackageUninstallPreview{}, false, err
	}
	preview.ScheduleCount, err = s.readScheduleCount(ctx, container, extensionID)
	if err != nil {
		return PackageUninstallPreview{}, false, err
	}
	preview.Grants, err = s.readGrants(ctx, container, extensionID)
	if err != nil {
		return PackageUninstallPreview{}, false, err
	}
	preview.ConfigPresent, err = s.readConfigPresent(ctx, container, extensionID)
	if err != nil {
		return PackageUninstallPreview{}, false, err
	}
	preview.Cleanup = s.buildCleanupList(preview.ContributionSummary, preview.RuntimeImpacts, preview.Grants)
	preview.Preserved = s.buildPreservedList()
	return preview, true, nil
}

func (s *ExtensionReadModelService) TryDependencies(ctx context.Context, extensionID string) (map[string]interface{}, bool, error) {
	container, err := s.authoritativeContainer()
	if err != nil {
		return nil, false, err
	}
	if _, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID)); err != nil {
		if errors.Is(err, domain.ErrInvalidExtensionID) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("readmodel: installation repository unavailable: %w", err)
	}
	if container.DependencyResolver == nil || container.ModuleRepository == nil || container.DefinitionRepository == nil {
		return nil, false, fmt.Errorf("readmodel: dependency read repositories not injected")
	}
	direct, err := s.readDirectDependencies(ctx, container, extensionID)
	if err != nil {
		return nil, false, err
	}
	reverse, err := s.readReverseDependencies(ctx, container, extensionID)
	if err != nil {
		return nil, false, err
	}
	return map[string]interface{}{"dependencies": direct, "dependents": reverse}, true, nil
}

func (s *ExtensionReadModelService) TryListVersions(ctx context.Context, extensionID string) ([]PackageVersionView, bool, error) {
	container, err := s.authoritativeContainer()
	if err != nil {
		return nil, false, err
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidExtensionID) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("readmodel: installation repository unavailable: %w", err)
	}
	if container.PackageRepository == nil {
		return nil, false, fmt.Errorf("readmodel: package version repository unavailable, fail closed")
	}
	versionRecords, versionErr := container.PackageRepository.ListPackageVersions(ctx, extensionID)
	if versionErr != nil {
		return nil, false, fmt.Errorf("readmodel: package version repository unavailable: %w", versionErr)
	}
	if len(versionRecords) == 0 {
		return nil, false, fmt.Errorf("readmodel: no version records found for extension, fail closed")
	}
	defMap := map[string]domain.ExtensionDefinition{}
	if container.DefinitionRepository != nil {
		definitions, defErr := container.DefinitionRepository.ListExtensions(ctx)
		if defErr == nil {
			for _, def := range definitions {
				if string(def.ID) == extensionID {
					defMap[def.Version.String()] = def
				}
			}
		}
	}
	views := make([]PackageVersionView, 0)
	for _, vr := range versionRecords {
		artifact, artifactErr := container.PackageRepository.GetArtifact(ctx, vr.ArtifactID)
		if artifactErr != nil {
			return nil, false, fmt.Errorf("readmodel: version %s artifact unavailable: %w", vr.Version, artifactErr)
		}
		view := s.buildVersionViewFromRecord(vr, installation, artifact)
		if def, ok := defMap[vr.Version]; ok {
			manifestBytes, _ := json.Marshal(def)
			view.Manifest = manifestBytes
			capabilities := kernelDefinitionCapabilities(def)
			sort.Strings(capabilities)
			view.Capabilities = capabilities
			if def.Package.Signature.Status != "" {
				view.SignatureStatus = def.Package.Signature.Status
			}
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool {
		vi, ei := domain.ParseVersion(views[i].Version)
		vj, ej := domain.ParseVersion(views[j].Version)
		if ei != nil || ej != nil {
			return views[i].Version > views[j].Version
		}
		return vi.Compare(vj) > 0
	})
	return views, true, nil
}

func (s *ExtensionReadModelService) buildVersionViewFromRecord(vr kernelruntime.PackageVersionRecord, installation domain.ExtensionInstallation, artifact kernelruntime.PackageArtifact) PackageVersionView {
	active := vr.VersionState == string(kernelruntime.PackageVersionStateCurrent)
	view := PackageVersionView{
		Version:             vr.Version,
		ArtifactID:          artifact.ArtifactID,
		ArtifactHash:        artifact.ArchiveHash,
		PackageHash:         artifact.ArchiveHash,
		ArchiveHash:         artifact.ArchiveHash,
		ManifestHash:        artifact.ManifestHash,
		ContentTreeHash:     artifact.ContentTreeHash,
		Source:              "kernel",
		CompatibilityStatus: "compatible",
		InstalledAt:         installation.InstalledAt.Format(time.RFC3339),
		Active:              active,
	}
	if active {
		view.ArtifactStatus = "active"
		view.ActivationStatus = "active"
	} else {
		view.ArtifactStatus = "archived"
		view.ActivationStatus = "archived"
		view.Archived = true
	}
	return view
}

func (s *ExtensionReadModelService) TryCompareVersions(ctx context.Context, extensionID, fromVersion, toVersion string) (PackageVersionDiff, bool, error) {
	container, err := s.authoritativeContainer()
	if err != nil {
		return PackageVersionDiff{}, false, err
	}
	if _, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID)); err != nil {
		if errors.Is(err, domain.ErrInvalidExtensionID) {
			return PackageVersionDiff{}, false, nil
		}
		return PackageVersionDiff{}, false, fmt.Errorf("readmodel: installation repository unavailable: %w", err)
	}
	if container.PackageRepository == nil {
		return PackageVersionDiff{}, false, fmt.Errorf("readmodel: version comparison dependencies not injected")
	}
	if versionRecords, versionErr := container.PackageRepository.ListPackageVersions(ctx, extensionID); versionErr == nil && len(versionRecords) > 0 {
		versionExists := map[string]bool{}
		for _, vr := range versionRecords {
			versionExists[vr.Version] = true
		}
		if !versionExists[fromVersion] || !versionExists[toVersion] {
			return PackageVersionDiff{}, false, nil
		}
	}
	if _, err := domain.ParseVersion(fromVersion); err != nil {
		return PackageVersionDiff{}, false, err
	}
	if _, err := domain.ParseVersion(toVersion); err != nil {
		return PackageVersionDiff{}, false, err
	}
	comparison, err := container.PackageRepository.ComparePackageVersions(ctx, extensionID, fromVersion, toVersion)
	if err != nil {
		return PackageVersionDiff{}, false, fmt.Errorf("readmodel: package version comparison failed: %w", err)
	}
	fromDef, err := comparison.FromManifest.ToExtensionDefinition()
	if err != nil {
		return PackageVersionDiff{}, false, fmt.Errorf("readmodel: from manifest definition unavailable: %w", err)
	}
	toDef, err := comparison.ToManifest.ToExtensionDefinition()
	if err != nil {
		return PackageVersionDiff{}, false, fmt.Errorf("readmodel: to manifest definition unavailable: %w", err)
	}
	diff := s.buildVersionDiff(extensionID, fromDef, toDef)
	fromPackage := &amitiax.Package{Manifest: comparison.FromManifest, Files: comparison.FromFiles}
	toPackage := &amitiax.Package{Manifest: comparison.ToManifest, Files: comparison.ToFiles}
	applyPackageFileDiff(&diff, fromPackage, toPackage)
	return diff, true, nil
}

func (s *ExtensionReadModelService) TryExport(ctx context.Context, extensionID, version string, ownerScope ...string) (ExportedPackage, bool, error) {
	container, err := s.authoritativeContainer()
	if err != nil {
		return ExportedPackage{}, false, err
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidExtensionID) {
			return ExportedPackage{}, false, nil
		}
		return ExportedPackage{}, false, fmt.Errorf("readmodel: installation repository unavailable: %w", err)
	}
	if container.PackageRepository == nil || container.PackageArtifactStore == nil || container.PackageGenerationStore == nil || s.proxy.kernel == nil {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: export dependencies not injected")
	}
	owner, _ := installation.Metadata["ownerUserId"].(string)
	storedScopeType, _ := installation.Metadata["scopeType"].(string)
	storedScopeID, _ := installation.Metadata["scopeId"].(string)
	userID, scopeType, scopeID := owner, storedScopeType, storedScopeID
	if len(ownerScope) >= 3 {
		userID, scopeType, scopeID = ownerScope[0], ownerScope[1], ownerScope[2]
	}
	if scopeType == "" {
		scopeType = "global"
	}
	if owner != userID || storedScopeType != scopeType || storedScopeID != scopeID {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: package scope mismatch")
	}
	currentVersion := installation.InstalledVersion.String()
	if version != "" && version != currentVersion {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: export version is not current installation")
	}
	version = currentVersion
	if currentVersionRecord, verErr := container.PackageRepository.GetCurrentPackageVersion(ctx, extensionID); verErr == nil {
		if currentVersionRecord.Version != version {
			return ExportedPackage{}, false, nil
		}
	}
	if installation.Generation <= 0 || installation.PackageID == "" {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: installation identity incomplete")
	}
	currentGeneration, err := container.PackageGenerationStore.ReadCurrent(extensionID)
	if err != nil {
		return ExportedPackage{}, false, err
	}
	if err := container.PackageGenerationStore.VerifyGeneration(ctx, currentGeneration); err != nil {
		return ExportedPackage{}, false, err
	}
	if err := validateExportInstallationGeneration(installation, currentGeneration); err != nil {
		return ExportedPackage{}, false, err
	}
	artifact, err := container.PackageRepository.GetArtifact(ctx, installation.PackageID)
	if err != nil {
		return ExportedPackage{}, false, err
	}
	if err := validateExportInstallationArtifact(installation, artifact); err != nil {
		return ExportedPackage{}, false, err
	}
	pkg, err := s.proxy.kernel.VerifyStoredPackage(ctx, artifact)
	if err != nil {
		return ExportedPackage{}, false, err
	}
	preview, err := s.canonicalExportPreview(ctx, installation, artifact, userID, scopeType, scopeID)
	if err != nil {
		return ExportedPackage{}, false, err
	}
	if !preview.Installable {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: canonical export preview rejected package")
	}
	if err := validateExportPreviewIdentity(preview, artifact, installation); err != nil {
		return ExportedPackage{}, false, err
	}
	flags, err := scanPackageArchiveForExport(artifact.ArchivePath)
	if err != nil {
		return ExportedPackage{}, false, err
	}
	currentInstallation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return ExportedPackage{}, false, err
	}
	if currentInstallation.Generation != installation.Generation || currentInstallation.PackageID != installation.PackageID || currentInstallation.InstalledVersion.Compare(installation.InstalledVersion) != 0 {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: installation changed during export validation")
	}
	validatedGeneration, err := container.PackageGenerationStore.ReadCurrent(extensionID)
	if err != nil {
		return ExportedPackage{}, false, err
	}
	if err := container.PackageGenerationStore.VerifyGeneration(ctx, validatedGeneration); err != nil {
		return ExportedPackage{}, false, err
	}
	if err := validateExportInstallationGeneration(currentInstallation, validatedGeneration); err != nil {
		return ExportedPackage{}, false, err
	}
	if validatedGeneration.GenerationID != currentGeneration.GenerationID || validatedGeneration.ArtifactID != currentGeneration.ArtifactID || validatedGeneration.Version != currentGeneration.Version || validatedGeneration.TreeHash != currentGeneration.TreeHash {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: current generation changed during export validation")
	}
	currentArtifact, err := container.PackageRepository.GetArtifact(ctx, currentInstallation.PackageID)
	if err != nil {
		return ExportedPackage{}, false, err
	}
	if err := validateExportInstallationArtifact(currentInstallation, currentArtifact); err != nil {
		return ExportedPackage{}, false, err
	}
	if currentArtifact.ArchiveHash != artifact.ArchiveHash || currentArtifact.ManifestHash != artifact.ManifestHash || currentArtifact.ContentTreeHash != artifact.ContentTreeHash || currentArtifact.ArtifactHash != artifact.ArtifactHash {
		return ExportedPackage{}, false, fmt.Errorf("readmodel: artifact changed during export validation")
	}
	name := pkg.Manifest.Extension.Name.Default
	if name == "" {
		name = extensionID
	}
	now := time.Now().UTC()
	exported := ExportedPackage{
		ExportID:        uuid.NewString(),
		FileName:        safePackageFileName(name + "-" + version + ".amitiax"),
		MIME:            "application/vnd.amitia.extension+zip",
		Size:            artifact.SizeBytes,
		Hash:            artifact.ArchiveHash,
		Version:         version,
		Format:          "amitiax",
		SecretScan:      "passed",
		SignatureStatus: artifact.SignatureStatus,
		ExpiresAt:       now.Add(15 * time.Minute),
		LocalPath:       artifact.ArchivePath,
	}
	exported.TestsIncluded = flags["tests"]
	exported.ReadmeIncluded = flags["readme"]
	exported.SBOMIncluded = flags["sbom"]
	exported.ScriptsIncluded = flags["scripts"]
	if err := container.PackageRepository.PutExport(ctx, kernelruntime.PackageExportTicket{ExportID: exported.ExportID,
		UserID: userID, ExtensionID: extensionID, ArtifactID: artifact.ArtifactID,
		FileName: exported.FileName, MIMEType: exported.MIME, ExpiresAt: exported.ExpiresAt.Format(time.RFC3339Nano),
		CreatedAt: now.Format(time.RFC3339Nano)}); err != nil {
		return ExportedPackage{}, false, err
	}
	return exported, true, nil
}

func (s *ExtensionReadModelService) canonicalExportPreview(ctx context.Context, installation domain.ExtensionInstallation, artifact kernelruntime.PackageArtifact, userID, scopeType, scopeID string) (kernelruntime.InstallPreview, error) {
	archive, err := os.Open(artifact.ArchivePath)
	if err != nil {
		return kernelruntime.InstallPreview{}, err
	}
	defer archive.Close()
	devOnly, _ := installation.Metadata["devOnly"].(bool)
	developerSessionID, _ := installation.Metadata["developerSessionId"].(string)
	preview, err := s.proxy.kernel.PreviewPackage(ctx, kernelruntime.PackagePreviewRequest{UserID: userID, ScopeType: scopeType, ScopeID: scopeID, FileName: filepathBase(artifact.ArchivePath), AllowUnsignedDev: devOnly, DeveloperSessionID: developerSessionID}, archive)
	if err != nil {
		return kernelruntime.InstallPreview{}, err
	}
	if preview.SessionID == "" {
		return kernelruntime.InstallPreview{}, fmt.Errorf("readmodel: canonical export preview session missing")
	}
	cancelErr := s.proxy.ReadContainer().PackageRepository.CancelPreview(ctx, preview.SessionID, userID, scopeType, scopeID)
	if cancelErr != nil {
		return kernelruntime.InstallPreview{}, fmt.Errorf("readmodel: release canonical export preview: %w", cancelErr)
	}
	return preview, nil
}

func validateExportInstallationArtifact(installation domain.ExtensionInstallation, artifact kernelruntime.PackageArtifact) error {
	if artifact.ArtifactID != installation.PackageID || artifact.ExtensionID != string(installation.ExtensionID) || artifact.Version != installation.InstalledVersion.String() {
		return fmt.Errorf("readmodel: installation artifact identity mismatch")
	}
	checks := map[string]string{"artifactId": artifact.ArtifactID, "archiveHash": artifact.ArchiveHash, "manifestHash": artifact.ManifestHash, "contentTreeHash": artifact.ContentTreeHash, "artifactHash": artifact.ArtifactHash}
	for key, expected := range checks {
		actual, ok := installation.Metadata[key].(string)
		if !ok || actual == "" || actual != expected {
			return fmt.Errorf("readmodel: installation %s mismatch", key)
		}
	}
	return nil
}

func validateExportInstallationGeneration(installation domain.ExtensionInstallation, current kernelruntime.PackageGenerationCurrent) error {
	generationID, generationOK := installation.Metadata["generationId"].(string)
	installedTreeHash, treeOK := installation.Metadata["installedTreeHash"].(string)
	if !generationOK || generationID == "" || !treeOK || installedTreeHash == "" {
		return fmt.Errorf("readmodel: installation generation identity incomplete")
	}
	if current.ExtensionID != string(installation.ExtensionID) || current.ArtifactID != installation.PackageID || current.Version != installation.InstalledVersion.String() || current.GenerationID != generationID || current.TreeHash != installedTreeHash {
		return fmt.Errorf("readmodel: installation current generation mismatch")
	}
	return nil
}

func validateExportPreviewIdentity(preview kernelruntime.InstallPreview, artifact kernelruntime.PackageArtifact, installation domain.ExtensionInstallation) error {
	if preview.ArtifactID != artifact.ArtifactID || preview.ExtensionID != string(installation.ExtensionID) || preview.Version != installation.InstalledVersion.String() || preview.ArchiveHash != artifact.ArchiveHash || preview.ManifestHash != artifact.ManifestHash || preview.ContentTreeHash != artifact.ContentTreeHash || preview.ArtifactHash != artifact.ArtifactHash {
		return fmt.Errorf("readmodel: canonical export preview identity mismatch")
	}
	if preview.SignatureStatus != artifact.SignatureStatus || preview.TrustDecision != artifact.TrustDecision {
		return fmt.Errorf("readmodel: canonical export preview trust mismatch")
	}
	return nil
}

func filepathBase(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}

func scanPackageArchiveForExport(archivePath string) (map[string]bool, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	flags := map[string]bool{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		lower := strings.ToLower(file.Name)
		flags["tests"] = flags["tests"] || strings.HasPrefix(lower, "tests/") || strings.Contains(lower, "/tests/")
		flags["readme"] = flags["readme"] || strings.HasSuffix(lower, "/readme.md") || lower == "readme.md"
		flags["sbom"] = flags["sbom"] || strings.HasSuffix(lower, "sbom.spdx.json")
		flags["scripts"] = flags["scripts"] || strings.Contains(lower, "/scripts/") || strings.HasPrefix(lower, "scripts/")
		if file.UncompressedSize64 > uint64(DefaultPackageLimits().MaxFileBytes) {
			return nil, fmt.Errorf("readmodel: export entry exceeds limit")
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, DefaultPackageLimits().MaxFileBytes+1))
		rc.Close()
		if readErr != nil || int64(len(content)) > DefaultPackageLimits().MaxFileBytes {
			return nil, fmt.Errorf("readmodel: export entry read failed")
		}
		if secretPattern.Match(content) {
			return nil, NewExtensionError(ErrPackageSecretDetected, "导出内容包含疑似 Secret，请改用 Secret Reference", file.Name, false, nil)
		}
	}
	return flags, nil
}

func (s *ExtensionReadModelService) readReverseDependencies(ctx context.Context, container *kernelruntime.Container, extensionID string) ([]PackageDependencyView, error) {
	if container.DependencyResolver == nil {
		return nil, fmt.Errorf("readmodel: dependency resolver not injected")
	}
	subjects, err := container.DependencyResolver.AffectedBy(ctx, extensionID)
	if err != nil {
		return nil, fmt.Errorf("readmodel: query reverse dependencies: %w", err)
	}
	result := make([]PackageDependencyView, 0, len(subjects))
	for _, subject := range subjects {
		view := PackageDependencyView{
			ID:       subject.SubjectID,
			Required: subject.Required,
		}
		if inst, instErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(subject.SubjectID)); instErr == nil {
			view.Installed = true
			view.Version = inst.InstalledVersion.String()
		} else if !errors.Is(instErr, domain.ErrInvalidExtensionID) {
			return nil, fmt.Errorf("readmodel: query dependent installation: %w", instErr)
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *ExtensionReadModelService) readDirectDependencies(ctx context.Context, container *kernelruntime.Container, extensionID string) ([]PackageDependencyView, error) {
	if container.ModuleRepository == nil {
		return nil, fmt.Errorf("readmodel: module repository not injected")
	}
	if container.DefinitionRepository == nil {
		return nil, fmt.Errorf("readmodel: definition repository not injected")
	}
	modules, err := container.ModuleRepository.ListModules(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return nil, fmt.Errorf("readmodel: query modules: %w", err)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return nil, err
	}
	definition, err := container.DefinitionRepository.GetExtension(ctx, domain.ExtensionID(extensionID), installation.InstalledVersion)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	result := []PackageDependencyView{}
	appendDependency := func(dep domain.DependencyDefinition, source string) error {
		key := string(dep.Type) + ":" + dep.ID + ":" + source
		if seen[key] {
			return nil
		}
		seen[key] = true
		view := PackageDependencyView{ID: dep.ID, Type: string(dep.Type), Source: source,
			VersionConstraint: dep.Version, Required: !dep.Optional}
		if dep.Type == domain.DependencyTypeExtension {
			if inst, instErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(dep.ID)); instErr == nil {
				view.Installed = true
				view.Version = inst.InstalledVersion.String()
			} else if !errors.Is(instErr, domain.ErrInvalidExtensionID) {
				return fmt.Errorf("readmodel: query dependency installation: %w", instErr)
			}
		} else {
			view.Installed = true
		}
		result = append(result, view)
		return nil
	}
	for _, dep := range definition.Dependencies {
		if err := appendDependency(dep, "extension"); err != nil {
			return nil, err
		}
	}
	for _, mod := range modules {
		for _, dep := range mod.Dependencies {
			if err := appendDependency(dep, "module:"+string(mod.ID)); err != nil {
				return nil, err
			}
		}
		for _, contribution := range mod.Contributions {
			for _, dep := range contribution.Dependencies {
				if err := appendDependency(dep, "contribution:"+string(contribution.ID)); err != nil {
					return nil, err
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (s *ExtensionReadModelService) readContributions(ctx context.Context, container *kernelruntime.Container, extensionID string) (map[string]int, []string, error) {
	summary := map[string]int{}
	eventSubs := []string{}
	if container.ContributionRepository == nil {
		return nil, nil, fmt.Errorf("readmodel: contribution repository not injected")
	}
	contributions, err := container.ContributionRepository.ListContributions(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return nil, nil, fmt.Errorf("readmodel: query contributions: %w", err)
	}
	for _, contrib := range contributions {
		category := contributionCategory(contrib.Kind)
		summary[category]++
		if contrib.Kind == domain.ContributionKindEventSubscription {
			eventSubs = append(eventSubs, string(contrib.ID))
		}
	}
	sort.Strings(eventSubs)
	return summary, eventSubs, nil
}

func (s *ExtensionReadModelService) readRuntimeImpacts(ctx context.Context, container *kernelruntime.Container, extensionID string) ([]PackageRuntimeImpact, error) {
	if container.ModuleRepository == nil || container.RuntimeSupervisor == nil {
		return nil, fmt.Errorf("readmodel: runtime read dependencies not injected")
	}
	modules, err := container.ModuleRepository.ListModules(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return nil, fmt.Errorf("readmodel: query modules for runtime: %w", err)
	}
	result := []PackageRuntimeImpact{}
	for _, mod := range modules {
		if mod.Runtime == nil {
			continue
		}
		defID := runtime_supervisor.BuildRuntimeDefinitionID(extensionID, string(mod.ID), mod.Runtime.Type)
		snapshot := container.RuntimeSupervisor.Snapshot(ctx, defID)
		for _, inst := range snapshot.Instances {
			result = append(result, PackageRuntimeImpact{
				InstanceID:  inst.InstanceID,
				ModuleID:    string(inst.Identity.ModuleID),
				RuntimeType: string(inst.Identity.RuntimeType),
				Desired:     string(inst.Desired),
				Actual:      string(inst.Actual),
				Health:      string(inst.Health),
			})
		}
	}
	return result, nil
}

func (s *ExtensionReadModelService) readScheduleCount(ctx context.Context, container *kernelruntime.Container, extensionID string) (int64, error) {
	if container.ScheduleRepository == nil {
		return 0, fmt.Errorf("readmodel: schedule repository not injected")
	}
	defs, err := container.ScheduleRepository.ListDefinitions(ctx, extensionID)
	if err != nil {
		return 0, fmt.Errorf("readmodel: query schedules: %w", err)
	}
	return int64(len(defs)), nil
}

func (s *ExtensionReadModelService) readGrants(ctx context.Context, container *kernelruntime.Container, extensionID string) ([]string, error) {
	if container.PermissionBroker == nil {
		return nil, fmt.Errorf("readmodel: permission broker not injected")
	}
	subject := permission.SubjectForExtension(extensionID)
	grants, err := container.PermissionBroker.ListGrants(ctx, permission.PermissionGrantFilter{Subject: &subject, ActiveOnly: true})
	if err != nil {
		return nil, fmt.Errorf("readmodel: query grants: %w", err)
	}
	result := make([]string, 0, len(grants))
	for _, grant := range grants {
		result = append(result, grant.PermissionID+":"+string(grant.Decision))
	}
	sort.Strings(result)
	return result, nil
}

func (s *ExtensionReadModelService) readConfigPresent(ctx context.Context, container *kernelruntime.Container, extensionID string) (bool, error) {
	if container.ScopeRepository == nil {
		return false, fmt.Errorf("readmodel: scope repository not injected")
	}
	bindings, err := container.ScopeRepository.ListBindings(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return false, fmt.Errorf("readmodel: query scope bindings: %w", err)
	}
	return len(bindings) > 0, nil
}

func (s *ExtensionReadModelService) buildCleanupList(summary map[string]int, runtimes []PackageRuntimeImpact, grants []string) []string {
	cleanup := []string{}
	if len(runtimes) > 0 {
		cleanup = append(cleanup, "运行时实例")
	}
	categories := make([]string, 0, len(summary))
	for category := range summary {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		switch category {
		case "tool":
			cleanup = append(cleanup, "Tool 贡献")
		case "hook":
			cleanup = append(cleanup, "Hook 贡献")
		case "event_subscription":
			cleanup = append(cleanup, "事件订阅")
		case "schedule":
			cleanup = append(cleanup, "定时任务")
		case "ui":
			cleanup = append(cleanup, "UI 贡献")
		case "agent_skill":
			cleanup = append(cleanup, "Agent Skill 索引")
		case "workflow":
			cleanup = append(cleanup, "Workflow 注册")
		case "mcp_server":
			cleanup = append(cleanup, "MCP 服务")
		case "background_service":
			cleanup = append(cleanup, "后台服务")
		case "provider":
			cleanup = append(cleanup, "Provider 注册")
		}
	}
	if len(grants) > 0 {
		cleanup = append(cleanup, "权限授予")
	}
	cleanup = append(cleanup, "Capability Grant", "当前配置与缓存")
	return cleanup
}

func (s *ExtensionReadModelService) buildPreservedList() []string {
	return []string{"版本历史", "安装与操作审计", "历史运行记录", "归档 Artifact"}
}

func contributionCategory(kind domain.ContributionKind) string {
	switch kind {
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		return "ui"
	default:
		return string(kind)
	}
}

func safeKernelDirName(id string) string {
	return strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(id)
}

func (s *ExtensionReadModelService) buildVersionView(def domain.ExtensionDefinition, installation domain.ExtensionInstallation, artifact kernelruntime.PackageArtifact) PackageVersionView {
	manifestBytes, _ := json.Marshal(def)
	active := def.Version.Compare(installation.InstalledVersion) == 0
	capabilities := kernelDefinitionCapabilities(def)
	sort.Strings(capabilities)
	view := PackageVersionView{
		Version:             def.Version.String(),
		Manifest:            manifestBytes,
		ArtifactID:          artifact.ArtifactID,
		ArtifactHash:        artifact.ArtifactHash,
		PackageHash:         artifact.ArchiveHash,
		ArchiveHash:         artifact.ArchiveHash,
		ManifestHash:        artifact.ManifestHash,
		ContentTreeHash:     artifact.ContentTreeHash,
		Source:              "kernel",
		SignatureStatus:     def.Package.Signature.Status,
		CompatibilityStatus: "compatible",
		Capabilities:        capabilities,
		InstalledAt:         installation.InstalledAt.Format(time.RFC3339),
		Active:              active,
		ArtifactStatus:      "active",
		ActivationStatus:    "active",
	}
	if !active {
		view.ArtifactStatus = "archived"
		view.ActivationStatus = "archived"
		view.Archived = true
	}
	return view
}

func (s *ExtensionReadModelService) buildVersionDiff(extensionID string, fromDef, toDef domain.ExtensionDefinition) PackageVersionDiff {
	fromJSON, _ := json.Marshal(fromDef)
	toJSON, _ := json.Marshal(toDef)
	fromMigration, _ := json.Marshal(fromDef.Metadata["migration"])
	toMigration, _ := json.Marshal(toDef.Metadata["migration"])
	fromModules := kernelModuleIDs(fromDef)
	toModules := kernelModuleIDs(toDef)
	fromContribs := kernelContributionIDs(fromDef)
	toContribs := kernelContributionIDs(toDef)
	fromPerms := kernelDefinitionPermissions(fromDef)
	toPerms := kernelDefinitionPermissions(toDef)
	fromScopes := kernelDefinitionScopes(fromDef)
	toScopes := kernelDefinitionScopes(toDef)
	fromCaps := kernelDefinitionCapabilities(fromDef)
	toCaps := kernelDefinitionCapabilities(toDef)
	fromDeps := kernelDefinitionDependencyIDs(fromDef)
	toDeps := kernelDefinitionDependencyIDs(toDef)
	fromRuntimes := kernelRuntimeTypes(fromDef)
	toRuntimes := kernelRuntimeTypes(toDef)

	diff := PackageVersionDiff{
		ExtensionID: extensionID,
		FromVersion: fromDef.Version.String(),
		ToVersion:   toDef.Version.String(),
		Manifest:    jsonObjectDiff(string(fromJSON), string(toJSON)),
		Module: map[string]interface{}{
			"added":   stringSetDifference(toModules, fromModules),
			"removed": stringSetDifference(fromModules, toModules),
		},
		Contribution: map[string]interface{}{
			"added":   stringSetDifference(toContribs, fromContribs),
			"removed": stringSetDifference(fromContribs, toContribs),
		},
		Permission: map[string]interface{}{
			"added":   stringSetDifference(toPerms, fromPerms),
			"removed": stringSetDifference(fromPerms, toPerms),
		},
		Scope: map[string]interface{}{
			"added":   stringSetDifference(toScopes, fromScopes),
			"removed": stringSetDifference(fromScopes, toScopes),
		},
		Runtime: map[string]interface{}{
			"added":   stringSetDifference(toRuntimes, fromRuntimes),
			"removed": stringSetDifference(fromRuntimes, toRuntimes),
		},
		Migration: map[string]interface{}{
			"changed": string(fromMigration) != string(toMigration),
			"from":    fromDef.Metadata["migration"],
			"to":      toDef.Metadata["migration"],
		},
		Schemas:      map[string]interface{}{},
		Workflow:     map[string]interface{}{},
		Instructions: map[string]interface{}{},
		Capabilities: map[string][]string{
			"added":   stringSetDifference(toCaps, fromCaps),
			"removed": stringSetDifference(fromCaps, toCaps),
		},
		Signature: map[string]string{
			"from": fromDef.Package.Signature.Status,
			"to":   toDef.Package.Signature.Status,
		},
		Scripts: map[string][]string{"added": {}, "removed": {}},
		Files:   map[string][]string{"added": {}, "removed": {}, "changed": {}},
		Dependencies: map[string][]string{
			"added":   stringSetDifference(toDeps, fromDeps),
			"removed": stringSetDifference(fromDeps, toDeps),
		},
		Trust: map[string]string{
			"from": fromDef.Publisher.TrustLevel,
			"to":   toDef.Publisher.TrustLevel,
		},
		Risks: []PackageRisk{},
	}

	if len(diff.Capabilities["added"]) > 0 {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "CAPABILITY_ADDED", Severity: "high", Message: strings.Join(diff.Capabilities["added"], ", ")})
	}
	if moduleAdded, ok := diff.Module["added"].([]string); ok && len(moduleAdded) > 0 {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "MODULE_ADDED", Severity: "medium", Message: "新增模块: " + strings.Join(moduleAdded, ", ")})
	}
	if permAdded, ok := diff.Permission["added"].([]string); ok && len(permAdded) > 0 {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "PERMISSION_ADDED", Severity: "high", Message: "新增权限需求: " + strings.Join(permAdded, ", ")})
	}
	if fromDef.Package.Signature.Status != toDef.Package.Signature.Status {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "SIGNATURE_CHANGED", Severity: "high", Message: "签名状态发生变化"})
	}
	if fromDef.Publisher.PublisherID != toDef.Publisher.PublisherID {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "PUBLISHER_CHANGED", Severity: "high", Message: "发布者发生变化"})
	}
	if fromDef.Publisher.TrustLevel != toDef.Publisher.TrustLevel {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "TRUST_LEVEL_CHANGED", Severity: "medium", Message: "信任等级发生变化"})
	}

	return diff
}

func kernelModuleIDs(def domain.ExtensionDefinition) []string {
	result := make([]string, 0, len(def.Modules))
	for _, mod := range def.Modules {
		result = append(result, string(mod.ID))
	}
	return result
}

func kernelContributionIDs(def domain.ExtensionDefinition) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, mod := range def.Modules {
		for _, contrib := range mod.Contributions {
			id := string(contrib.Kind) + ":" + string(contrib.ID)
			if !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}
	return result
}

func kernelDefinitionPermissions(def domain.ExtensionDefinition) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, mod := range def.Modules {
		for _, contrib := range mod.Contributions {
			for _, perm := range contrib.RequiredPermissions {
				if !seen[perm] {
					seen[perm] = true
					result = append(result, perm)
				}
			}
		}
	}
	return result
}

func kernelDefinitionScopes(def domain.ExtensionDefinition) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, mod := range def.Modules {
		for _, contrib := range mod.Contributions {
			for _, scope := range contrib.RequiredScope {
				if !seen[scope] {
					seen[scope] = true
					result = append(result, scope)
				}
			}
		}
	}
	return result
}

func kernelDefinitionCapabilities(def domain.ExtensionDefinition) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, mod := range def.Modules {
		if mod.Runtime != nil {
			for cap, enabled := range mod.Runtime.Capabilities {
				if enabled && !seen[cap] {
					seen[cap] = true
					result = append(result, cap)
				}
			}
		}
	}
	return result
}

func kernelDefinitionDependencyIDs(def domain.ExtensionDefinition) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, dep := range def.Dependencies {
		if !seen[dep.ID] {
			seen[dep.ID] = true
			result = append(result, dep.ID)
		}
	}
	for _, mod := range def.Modules {
		for _, dep := range mod.Dependencies {
			if !seen[dep.ID] {
				seen[dep.ID] = true
				result = append(result, dep.ID)
			}
		}
	}
	return result
}

func kernelRuntimeTypes(def domain.ExtensionDefinition) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, mod := range def.Modules {
		if mod.Runtime != nil && mod.Runtime.Type != "" && mod.Runtime.Type != domain.RuntimeTypeBuiltin {
			key := string(mod.ID) + ":" + string(mod.Runtime.Type)
			if !seen[key] {
				seen[key] = true
				result = append(result, key)
			}
		}
	}
	return result
}

func readZipFiles(raw []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte, len(reader.File))
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[f.Name] = content
	}
	return files, nil
}

func applyPackageFileDiff(diff *PackageVersionDiff, fromPackage, toPackage *amitiax.Package) {
	fromFiles := map[string]string{}
	toFiles := map[string]string{}
	for _, file := range fromPackage.Files {
		if !file.IsDir {
			fromFiles[file.Path] = file.Hash
		}
	}
	for _, file := range toPackage.Files {
		if !file.IsDir {
			toFiles[file.Path] = file.Hash
		}
	}
	for path, hash := range fromFiles {
		toHash, exists := toFiles[path]
		if !exists {
			diff.Files["removed"] = append(diff.Files["removed"], path)
		} else if hash != toHash {
			diff.Files["changed"] = append(diff.Files["changed"], path)
		}
	}
	for path := range toFiles {
		if _, exists := fromFiles[path]; !exists {
			diff.Files["added"] = append(diff.Files["added"], path)
		}
	}
	for key := range diff.Files {
		sort.Strings(diff.Files[key])
	}
	diff.Schemas = classifiedPackageFileDiff(diff.Files, "schema", "schemas/")
	diff.Workflow = classifiedPackageFileDiff(diff.Files, "workflow", "workflows/", "/workflow")
	diff.Instructions = classifiedPackageFileDiff(diff.Files, "instructions", "instructions/", "skill.md")
	scriptChanges := classifiedPackageFileDiff(diff.Files, "scripts", "scripts/", "/scripts/")
	diff.Scripts = map[string][]string{"added": stringSliceValue(scriptChanges["added"]),
		"removed": stringSliceValue(scriptChanges["removed"]), "changed": stringSliceValue(scriptChanges["changed"])}
}

func classifiedPackageFileDiff(files map[string][]string, category string, markers ...string) map[string]interface{} {
	result := map[string]interface{}{"category": category, "added": []string{}, "removed": []string{}, "changed": []string{}}
	for _, changeType := range []string{"added", "removed", "changed"} {
		values := []string{}
		for _, filePath := range files[changeType] {
			lower := strings.ToLower(filePath)
			for _, marker := range markers {
				if strings.Contains(lower, marker) {
					values = append(values, filePath)
					break
				}
			}
		}
		result[changeType] = values
	}
	return result
}

func stringSliceValue(value interface{}) []string {
	values, _ := value.([]string)
	return values
}
