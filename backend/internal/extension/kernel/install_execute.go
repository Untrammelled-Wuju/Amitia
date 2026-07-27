package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/contribution"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func (r *Runtime) ExecuteInstall(ctx context.Context, archivePath string) (KernelInstallResult, error) {
	result := KernelInstallResult{}

	if r.container == nil {
		return result, errors.New("kernel: container not attached")
	}

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		return result, fmt.Errorf("kernel: read archive: %w", err)
	}

	source := package_security.PackageSource{
		SourceType:  package_security.SourceLocalFile,
		LocalPath:   archivePath,
		DisplayName: filepath.Base(archivePath),
	}
	secReport, err := r.container.PackageSecurity.Inspect(ctx, raw, source)
	if err != nil {
		return result, fmt.Errorf("kernel: security inspect: %w", err)
	}
	if !secReport.Passed {
		return result, fmt.Errorf("kernel: security check failed: %d blocking issue(s)", len(secReport.BlockingIssues))
	}

	hasher := r.container.PackageSecurity.GetHasher()
	packageHash := hasher.HashArchive(raw)
	packageHashHex := strings.TrimPrefix(packageHash, "sha256:")

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		return result, fmt.Errorf("kernel: open archive: %w", err)
	}

	manifest := pkg.Manifest
	report := manifest.Validate()
	if report.HasErrors() {
		data, _ := json.Marshal(report.Errors)
		return result, fmt.Errorf("kernel: manifest validation failed: %s", data)
	}

	def, err := manifest.ToExtensionDefinition()
	if err != nil {
		return result, fmt.Errorf("kernel: build definition: %w", err)
	}

	safeID := safeDirectoryName(manifest.Extension.ID)
	version := manifest.Extension.Version

	staging, err := os.MkdirTemp(r.root, ".install-"+safeID+"-")
	if err != nil {
		return result, fmt.Errorf("kernel: create staging: %w", err)
	}

	var installedDir string
	var artifactPath string
	var success bool
	defer func() {
		os.RemoveAll(staging)
		if !success {
			if installedDir != "" {
				os.RemoveAll(installedDir)
			}
			if artifactPath != "" {
				os.Remove(artifactPath)
			}
		}
	}()

	if err := amitiax.WritePackageToDir(pkg, archivePath, staging); err != nil {
		return result, fmt.Errorf("kernel: extract to staging: %w", err)
	}

	contentTreeHash := package_security.ComputeDirHash(staging, hasher)

	def.Package = domain.PackageReference{
		PackageID:       packageHashHex,
		ManifestVersion: manifest.ManifestVersion,
		ArchiveHash:     packageHash,
		ContentTreeHash: contentTreeHash,
	}

	defData, err := json.Marshal(def)
	if err != nil {
		return result, fmt.Errorf("kernel: marshal definition: %w", err)
	}
	defHash := sha256.Sum256(defData)
	defHashHex := hex.EncodeToString(defHash[:])

	artifactDir := filepath.Join(r.root, "artifacts", safeID, version)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return result, fmt.Errorf("kernel: create artifact dir: %w", err)
	}
	artifactPath = filepath.Join(artifactDir, packageHashHex+".amitiax")
	if err := copyFile(archivePath, artifactPath); err != nil {
		return result, fmt.Errorf("kernel: write artifact: %w", err)
	}

	installedDir = filepath.Join(r.root, "installed", safeID, version, defHashHex)
	if err := os.MkdirAll(installedDir, 0o755); err != nil {
		return result, fmt.Errorf("kernel: create install dir: %w", err)
	}
	if err := copyDirContents(staging, installedDir); err != nil {
		return result, fmt.Errorf("kernel: copy to install dir: %w", err)
	}

	now := time.Now().UTC()
	inst := domain.ExtensionInstallation{
		InstallationID:    "inst-" + safeID + "-" + version,
		ExtensionID:       def.ID,
		InstalledVersion:  def.Version,
		PackageID:         packageHashHex,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementDisabled,
		InstalledAt:       now,
		UpdatedAt:         now,
		Generation:        1,
	}

	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.DefinitionRepository.PutExtension(txCtx, def); err != nil {
			return fmt.Errorf("kernel: put definition: %w", err)
		}
		if err := r.container.InstallationRepository.PutInstallation(txCtx, inst); err != nil {
			return fmt.Errorf("kernel: put installation: %w", err)
		}
		for _, mod := range def.Modules {
			if err := r.container.ModuleRepository.PutModule(txCtx, mod); err != nil {
				return fmt.Errorf("kernel: put module %s: %w", mod.ID, err)
			}
		}
		for _, mod := range def.Modules {
			for _, contrib := range mod.Contributions {
				if err := r.container.ContributionRepository.PutContribution(txCtx, contrib); err != nil {
					return fmt.Errorf("kernel: put contribution %s: %w", contrib.ID, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	for _, mod := range def.Modules {
		for _, contrib := range mod.Contributions {
			bc := &contribution.BaseContribution{
				ID:        string(contrib.ID),
				Type:      contribution.ContributionType(contrib.Kind),
				Extension: string(contrib.ExtensionID),
				Module:    string(contrib.ModuleID),
				Enabled:   false,
			}
			_ = r.container.ContributionRegistry.Register(bc)
		}
	}

	for _, mod := range def.Modules {
		for _, contrib := range mod.Contributions {
			if !isUIContributionKind(string(contrib.Kind)) {
				continue
			}
			defData, mErr := json.Marshal(contrib.Definition)
			if mErr != nil {
				continue
			}
			var uiDef ui_contribution.UIContributionDefinition
			if uErr := json.Unmarshal(defData, &uiDef); uErr != nil {
				continue
			}
			if uiDef.ContributionID == "" {
				uiDef.ContributionID = ui_contribution.ContributionID(contrib.ID)
			}
			if uiDef.ExtensionID == "" {
				uiDef.ExtensionID = ui_contribution.ExtensionID(contrib.ExtensionID)
			}
			if uiDef.ModuleID == "" {
				uiDef.ModuleID = ui_contribution.ModuleID(contrib.ModuleID)
			}
			_ = r.container.UIHost.RegisterContribution(&uiDef)
			if r.container.UIContributionRepo != nil {
				_ = r.container.UIContributionRepo.PutContribution(ctx, &uiDef)
			}
		}
	}

	item := installedFromManifest(manifest, installedDir, now)
	r.mu.Lock()
	r.installed[item.ID] = item
	r.mu.Unlock()

	success = true
	result = KernelInstallResult{
		ExtensionID:     string(def.ID),
		Version:         version,
		InstallationID:  inst.InstallationID,
		PackageHash:     packageHash,
		ContentTreeHash: contentTreeHash,
		ArtifactPath:    artifactPath,
		InstallPath:     installedDir,
		DefinitionHash:  defHashHex,
		InstalledAt:     now,
	}
	return result, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			if err := copyDirContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return err
			}
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func isUIContributionKind(kind string) bool {
	return strings.HasPrefix(kind, "ui_")
}
