package kernel

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type canonicalSkillInstallBridge struct {
	runtime *Runtime
}

func newCanonicalSkillInstallBridge() *canonicalSkillInstallBridge {
	return &canonicalSkillInstallBridge{}
}

func (b *canonicalSkillInstallBridge) BindRuntime(runtime *Runtime) {
	if b != nil {
		b.runtime = runtime
	}
}

func (b *canonicalSkillInstallBridge) ImportSkill(ctx context.Context, sourceURI string, skillName string, hash string) (string, error) {
	spaceID := ""
	if b != nil && b.runtime != nil && b.runtime.container != nil {
		spaceID = b.runtime.container.runtimeIdentitySpaceID()
	}
	return b.ImportSkillForSpace(ctx, spaceID, sourceURI, skillName, hash)
}

func (b *canonicalSkillInstallBridge) ImportSkillForSpace(ctx context.Context, spaceID, sourceURI, skillName, hash string) (string, error) {
	if b == nil || b.runtime == nil {
		return "", fmt.Errorf("canonical skill installer unavailable")
	}
	files, rootName, err := readAgentSkillSource(ctx, sourceURI)
	if err != nil {
		return "", err
	}
	built, err := BuildLocalAgentSkillPackage(ctx, spaceID, rootName, files)
	if err != nil {
		return "", err
	}
	if hash != "" {
		sum := sha256.Sum256(built.Archive)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), hash) {
			return "", fmt.Errorf("agent skill package hash mismatch")
		}
	}
	preview, err := b.runtime.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID: spaceID, ScopeType: "global", FileName: built.FileName, AllowUnsignedLocal: true,
	}, bytes.NewReader(built.Archive))
	if err != nil {
		return "", err
	}
	confirmations := map[string]bool{}
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	confirmation, err := b.runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID: preview.SessionID, SpaceID: spaceID, ScopeType: "global", Confirmations: confirmations,
	})
	if err != nil {
		return "", err
	}
	result, err := b.runtime.ExecutePackageInstall(ctx, PackageInstallRequest{
		SessionID: preview.SessionID, SpaceID: spaceID, ScopeType: "global",
		ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "acquisition-skill-" + preview.SessionID,
	})
	if err != nil {
		return "", err
	}
	if err := b.runtime.Enable(ctx, result.ExtensionID); err != nil {
		return "", err
	}
	return result.ExtensionID, nil
}

func (b *canonicalSkillInstallBridge) RemoveSkill(ctx context.Context, skillID string) error {
	return b.RemoveSkillForSpace(ctx, "", skillID)
}

func (b *canonicalSkillInstallBridge) RemoveSkillForSpace(ctx context.Context, spaceID, skillID string) error {
	if b == nil || b.runtime == nil {
		return fmt.Errorf("canonical skill installer unavailable")
	}
	extensionID, scopeType, scopeID, err := b.resolveSkillTarget(ctx, spaceID, skillID)
	if err != nil {
		return err
	}
	preview, err := b.runtime.PreviewPackageUninstall(ctx, extensionID, spaceID, scopeType, scopeID)
	if err != nil {
		return err
	}
	confirmations := map[string]bool{}
	for _, item := range RequiredUninstallConfirmations(preview) {
		confirmations[item] = true
	}
	confirmation, err := b.runtime.ConfirmPackageUninstall(ctx, ConfirmPackageUninstallRequest{
		ExtensionID: extensionID, SpaceID: spaceID, ScopeType: scopeType, ScopeID: scopeID, Confirmations: confirmations,
	})
	if err != nil {
		return err
	}
	_, err = b.runtime.ExecutePackageUninstall(ctx, ExecutePackageUninstallRequest{
		ExtensionID: extensionID, SpaceID: spaceID, ScopeType: scopeType, ScopeID: scopeID,
		ConfirmationToken: confirmation.Token, IdempotencyKey: "acquisition-skill-remove-" + preview.PreviewHash,
	})
	return err
}

func (b *canonicalSkillInstallBridge) VerifyInstalledCapability(ctx context.Context, installed acquisition.InstalledCapability) (bool, error) {
	if b == nil || b.runtime == nil || b.runtime.container == nil {
		return false, fmt.Errorf("canonical skill installer unavailable")
	}
	extensionID := installed.Candidate.ExtensionID
	if extensionID == "" && len(installed.ExtensionIDs) > 0 {
		extensionID = installed.ExtensionIDs[0]
	}
	if extensionID == "" {
		return false, nil
	}
	record, err := b.runtime.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return false, nil
	}
	return record.InstallationState == domain.InstallationStateInstalled, nil
}

func (b *canonicalSkillInstallBridge) resolveSkillTarget(ctx context.Context, spaceID, skillID string) (string, string, string, error) {
	if b.runtime.container == nil || b.runtime.container.AgentSkillRuntime == nil {
		return "", "", "", fmt.Errorf("agent skill runtime unavailable")
	}
	definition, err := b.runtime.container.AgentSkillRuntime.GetInstalled(ctx, spaceID, skillID)
	if err != nil {
		return "", "", "", err
	}
	scopeType := string(definition.Scope)
	if scopeType == "" {
		scopeType = "global"
	}
	return definition.ExtensionID, scopeType, definition.ScopeID, nil
}

func readAgentSkillSource(ctx context.Context, sourceURI string) (map[string][]byte, string, error) {
	sourceURI = strings.TrimSpace(sourceURI)
	if sourceURI == "" {
		return nil, "", fmt.Errorf("agent skill source URI is empty")
	}
	if strings.HasPrefix(sourceURI, "http://") || strings.HasPrefix(sourceURI, "https://") {
		client := &http.Client{Timeout: 60 * time.Second}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURI, nil)
		if err != nil {
			return nil, "", err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, "", err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, "", fmt.Errorf("agent skill source returned status %d", response.StatusCode)
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, 55<<20))
		if err != nil {
			return nil, "", err
		}
		if len(raw) >= 4 && bytes.Equal(raw[:4], []byte{'P', 'K', 0x03, 0x04}) {
			return readAgentSkillZip(raw)
		}
		if strings.HasSuffix(strings.ToLower(strings.SplitN(sourceURI, "?", 2)[0]), "/skill.md") {
			parts := strings.Split(strings.TrimSuffix(strings.SplitN(sourceURI, "?", 2)[0], "/SKILL.md"), "/")
			rootName := ""
			if len(parts) > 0 {
				rootName = parts[len(parts)-1]
			}
			return map[string][]byte{"SKILL.md": raw}, rootName, nil
		}
		return nil, "", fmt.Errorf("agent skill URL must return a ZIP archive or SKILL.md")
	}
	sourceURI = strings.TrimPrefix(sourceURI, "file://")
	info, err := os.Stat(sourceURI)
	if err != nil {
		return nil, "", err
	}
	if !info.IsDir() {
		raw, err := os.ReadFile(sourceURI)
		if err != nil {
			return nil, "", err
		}
		if strings.EqualFold(filepath.Ext(sourceURI), ".zip") {
			return readAgentSkillZip(raw)
		}
		if filepath.Base(sourceURI) != "SKILL.md" {
			return nil, "", fmt.Errorf("agent skill source must be a directory, SKILL.md or ZIP")
		}
		files := map[string][]byte{"SKILL.md": raw}
		entries, _ := os.ReadDir(filepath.Dir(sourceURI))
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "SKILL.md" || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			content, err := os.ReadFile(filepath.Join(filepath.Dir(sourceURI), entry.Name()))
			if err == nil {
				files[entry.Name()] = content
			}
		}
		return files, filepath.Base(filepath.Dir(sourceURI)), nil
	}
	files := map[string][]byte{}
	var total int64
	err = filepath.WalkDir(sourceURI, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if info.Size() > 20<<20 || total > 50<<20 || len(files) >= 500 {
			return fmt.Errorf("agent skill source exceeds limits")
		}
		relative, err := filepath.Rel(sourceURI, filePath)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return files, filepath.Base(filepath.Clean(sourceURI)), nil
}

func readAgentSkillZip(raw []byte) (map[string][]byte, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, "", err
	}
	files := map[string][]byte{}
	var total int64
	for _, entry := range reader.File {
		clean := filepath.ToSlash(filepath.Clean(entry.Name))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
			return nil, "", fmt.Errorf("agent skill ZIP path is invalid")
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmt.Errorf("agent skill ZIP symlink is not allowed")
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.UncompressedSize64 > 20<<20 {
			return nil, "", fmt.Errorf("agent skill ZIP entry exceeds limit")
		}
		total += int64(entry.UncompressedSize64)
		if total > 50<<20 || len(files) >= 500 {
			return nil, "", fmt.Errorf("agent skill ZIP exceeds limit")
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, "", err
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, 20<<20))
		rc.Close()
		if readErr != nil {
			return nil, "", readErr
		}
		files[clean] = content
	}
	return files, "", nil
}

func (c *Container) runtimeIdentitySpaceID() string {
	if c == nil {
		return ""
	}
	return c.RuntimeIdentity.SpaceID.String()
}
