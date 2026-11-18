package extension

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/system/dataportability"
)

const (
	extensionPluginDataComponentID = "extension.plugin_data.v1"
	extensionPluginDataLogicalName = "extension.plugin_data"

	maxPluginDataFileBytes  = int64(64) << 20
	maxPluginDataTotalBytes = int64(512) << 20
)

type extensionPluginDataRecord struct {
	ExtensionID   string `json:"extension_id"`
	Path          string `json:"path"`
	Mode          uint32 `json:"mode"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256"`
	ContentBase64 string `json:"content_base64"`
}

type pluginDataScopeEntry struct {
	ExtensionID string
	Root        string
	Includes    []string
}

type pluginDataFile struct {
	Path string
	Size int64
	Mode uint32
}

func pluginDataScopeRequested(req dataportability.BackupRequest) bool {
	switch req.EffectiveScope() {
	case dataportability.ScopeAll, dataportability.ScopeCharacter, dataportability.ScopeSettings:
		return true
	default:
		return false
	}
}

func (c *ExtensionBackupContributor) pluginDataScopeEntries(ctx context.Context, req dataportability.BackupRequest) ([]pluginDataScopeEntry, error) {
	if !pluginDataScopeRequested(req) {
		return nil, nil
	}
	if c.Runtime == nil || c.Runtime.Container() == nil {
		return nil, fmt.Errorf("extension kernel runtime unavailable")
	}
	container := c.Runtime.Container()
	if container.InstallationRepository == nil {
		return nil, fmt.Errorf("extension installation repository unavailable")
	}
	if container.DefinitionRepository == nil {
		return nil, fmt.Errorf("extension definition repository unavailable")
	}
	if strings.TrimSpace(container.ExtRoot) == "" {
		return nil, fmt.Errorf("extension root unavailable for plugin data backup")
	}
	installations, err := container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]pluginDataScopeEntry, 0, len(installations))
	for _, installation := range installations {
		if !extensionInScope(req, installation) {
			continue
		}
		definition, err := container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
		if err != nil {
			return nil, err
		}
		includes := kernel.ExtensionPluginDataBackupPaths(definition)
		if len(includes) == 0 {
			continue
		}
		entries = append(entries, pluginDataScopeEntry{
			ExtensionID: string(installation.ExtensionID),
			Root:        kernel.PluginDataRoot(container.ExtRoot, string(installation.ExtensionID)),
			Includes:    includes,
		})
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].ExtensionID < entries[right].ExtensionID })
	return entries, nil
}

func collectPluginDataFiles(entry pluginDataScopeEntry) ([]pluginDataFile, int64, error) {
	files := make([]pluginDataFile, 0, 8)
	var total int64
	seen := map[string]bool{}
	for _, include := range entry.Includes {
		absolute := filepath.Join(entry.Root, filepath.FromSlash(include))
		relative, err := filepath.Rel(entry.Root, absolute)
		if err != nil {
			return nil, 0, fmt.Errorf("extension plugin data include %q is outside the plugin data directory: %w", include, err)
		}
		relative = filepath.ToSlash(relative)
		if !pluginDataPathDeclared(relative, entry.Includes) {
			return nil, 0, fmt.Errorf("extension plugin data include %q is outside the plugin data directory", include)
		}
		info, err := os.Lstat(absolute)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, 0, err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil, 0, fmt.Errorf("extension plugin data path %q is a symbolic link, which cannot be archived", relative)
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return nil, 0, fmt.Errorf("extension plugin data path %q is not a regular file", relative)
			}
			if err := appendPluginDataFile(&files, seen, relative, info.Size(), uint32(info.Mode().Perm())); err != nil {
				return nil, 0, err
			}
			total += info.Size()
			if total > maxPluginDataTotalBytes {
				return nil, 0, fmt.Errorf("extension plugin data exceeds the %d byte archive limit at %d bytes", maxPluginDataTotalBytes, total)
			}
			continue
		}
		walkErr := filepath.WalkDir(absolute, func(path string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if dirEntry.IsDir() {
				return nil
			}
			relativePath, relErr := filepath.Rel(entry.Root, path)
			if relErr != nil {
				return relErr
			}
			slashPath := filepath.ToSlash(relativePath)
			if dirEntry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("extension plugin data path %q is a symbolic link, which cannot be archived", slashPath)
			}
			info, infoErr := dirEntry.Info()
			if infoErr != nil {
				return infoErr
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("extension plugin data path %q is not a regular file", slashPath)
			}
			if info.Size() > maxPluginDataFileBytes {
				return fmt.Errorf("extension plugin data file %q is %d bytes, which exceeds the %d byte limit", slashPath, info.Size(), maxPluginDataFileBytes)
			}
			if appendErr := appendPluginDataFile(&files, seen, slashPath, info.Size(), uint32(info.Mode().Perm())); appendErr != nil {
				return appendErr
			}
			total += info.Size()
			if total > maxPluginDataTotalBytes {
				return fmt.Errorf("extension plugin data exceeds the %d byte archive limit at %d bytes", maxPluginDataTotalBytes, total)
			}
			return nil
		})
		if walkErr != nil {
			return nil, 0, walkErr
		}
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	return files, total, nil
}

func appendPluginDataFile(files *[]pluginDataFile, seen map[string]bool, path string, size int64, mode uint32) error {
	if size > maxPluginDataFileBytes {
		return fmt.Errorf("extension plugin data file %q is %d bytes, which exceeds the %d byte limit", path, size, maxPluginDataFileBytes)
	}
	if seen[path] {
		return nil
	}
	seen[path] = true
	*files = append(*files, pluginDataFile{Path: path, Size: size, Mode: mode})
	return nil
}

func readPluginDataRecord(extensionID, root string, file pluginDataFile) (extensionPluginDataRecord, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
	if err != nil {
		return extensionPluginDataRecord{}, err
	}
	if int64(len(raw)) > maxPluginDataFileBytes {
		return extensionPluginDataRecord{}, fmt.Errorf("extension plugin data file %q is %d bytes, which exceeds the %d byte limit", file.Path, len(raw), maxPluginDataFileBytes)
	}
	sum := sha256.Sum256(raw)
	return extensionPluginDataRecord{
		ExtensionID: extensionID, Path: file.Path, Mode: file.Mode, Size: int64(len(raw)),
		SHA256: hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(raw),
	}, nil
}

func writePluginDataRecord(root string, includes []string, record extensionPluginDataRecord) error {
	if !pluginDataPathDeclared(record.Path, includes) {
		return fmt.Errorf("extension plugin data path %q is not declared by data.pluginData.include", record.Path)
	}
	raw, err := base64.StdEncoding.DecodeString(record.ContentBase64)
	if err != nil {
		return err
	}
	if int64(len(raw)) != record.Size {
		return fmt.Errorf("extension plugin data path %q declares %d bytes but carries %d bytes", record.Path, record.Size, len(raw))
	}
	sum := sha256.Sum256(raw)
	if digest := hex.EncodeToString(sum[:]); digest != record.SHA256 {
		return fmt.Errorf("extension plugin data path %q hash mismatch: archive=%s payload=%s", record.Path, record.SHA256, digest)
	}
	targetPath := filepath.Join(root, filepath.FromSlash(record.Path))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return fmt.Errorf("kernel: create extension plugin data directory: %w", err)
	}
	tempPath := targetPath + ".restore.tmp"
	if err := os.WriteFile(tempPath, raw, 0o600); err != nil {
		return fmt.Errorf("kernel: write extension plugin data file: %w", err)
	}
	mode := os.FileMode(record.Mode).Perm()
	if mode == 0 {
		mode = 0o600
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("kernel: set extension plugin data mode: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("kernel: commit extension plugin data file: %w", err)
	}
	return nil
}

func pluginDataPathDeclared(path string, includes []string) bool {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" || cleaned != path {
		return false
	}
	if strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, "\\") {
		return false
	}
	normalized := filepath.ToSlash(filepath.Clean(cleaned))
	if normalized != cleaned || normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return false
	}
	for _, include := range includes {
		if normalized == include || strings.HasPrefix(normalized, include+"/") {
			return true
		}
	}
	return false
}

func (c *ExtensionBackupContributor) pluginDataPlan(ctx context.Context, req dataportability.BackupRequest) (dataportability.BackupComponentPlan, error) {
	plan := dataportability.BackupComponentPlan{
		ID: extensionPluginDataComponentID, Kind: dataportability.KindNDJSON, LogicalName: extensionPluginDataLogicalName,
		Required: false, SourceOfTruth: true,
	}
	entries, err := c.pluginDataScopeEntries(ctx, req)
	if err != nil {
		return plan, err
	}
	for _, entry := range entries {
		files, total, err := collectPluginDataFiles(entry)
		if err != nil {
			return plan, err
		}
		plan.ItemCount += int64(len(files))
		plan.EstimatedSize += total
	}
	return plan, nil
}

func (c *ExtensionBackupContributor) pluginDataExport(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if !pluginDataScopeRequested(req) {
		return nil
	}
	entries, err := c.pluginDataScopeEntries(ctx, req)
	if err != nil {
		return err
	}
	component, err := out.CreateComponent(extensionPluginDataComponentID, extensionPluginDataLogicalName, dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer component.Close()
	for _, entry := range entries {
		files, _, err := collectPluginDataFiles(entry)
		if err != nil {
			return err
		}
		for _, file := range files {
			record, err := readPluginDataRecord(entry.ExtensionID, entry.Root, file)
			if err != nil {
				return err
			}
			payload, err := json.Marshal(record)
			if err != nil {
				return err
			}
			if _, err := component.Write(payload); err != nil {
				return err
			}
			if _, err := component.Write([]byte("\n")); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *ExtensionBackupContributor) pluginDataPreview(ctx context.Context, in dataportability.BackupReader) (dataportability.ImportComponentPreview, error) {
	preview := dataportability.ImportComponentPreview{ComponentID: extensionPluginDataComponentID, Kind: dataportability.KindNDJSON, LogicalName: extensionPluginDataLogicalName}
	reader, err := in.ReadComponent(extensionPluginDataComponentID)
	if err != nil {
		return preview, err
	}
	defer reader.Close()
	extRoot := ""
	if c.Runtime != nil && c.Runtime.Container() != nil {
		extRoot = c.Runtime.Container().ExtRoot
	}
	if strings.TrimSpace(extRoot) == "" {
		return preview, fmt.Errorf("extension root unavailable for plugin data import preview")
	}
	buffered := bufio.NewReader(reader)
	for {
		line, readErr := buffered.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var record extensionPluginDataRecord
			if err := json.Unmarshal(bytes.TrimSpace(line), &record); err != nil {
				return preview, err
			}
			preview.ItemCount++
			targetPath := filepath.Join(kernel.PluginDataRoot(extRoot, record.ExtensionID), filepath.FromSlash(record.Path))
			if _, statErr := os.Lstat(targetPath); statErr == nil {
				preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
					SourceID: record.ExtensionID + ":" + record.Path, TargetID: targetPath, EntityType: "plugin_data", Policy: dataportability.CollisionDuplicate,
				})
			} else if !os.IsNotExist(statErr) {
				return preview, statErr
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return preview, readErr
		}
	}
	return preview, nil
}

func (c *ExtensionBackupContributor) restorePluginData(ctx context.Context, in dataportability.BackupReader) error {
	if c.Runtime == nil || c.Runtime.Container() == nil {
		return fmt.Errorf("extension kernel runtime unavailable")
	}
	container := c.Runtime.Container()
	if strings.TrimSpace(container.ExtRoot) == "" {
		return fmt.Errorf("extension root unavailable for plugin data restore")
	}
	reader, err := in.ReadComponent(extensionPluginDataComponentID)
	if err != nil {
		return err
	}
	defer reader.Close()
	buffered := bufio.NewReader(reader)
	for {
		line, readErr := buffered.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var record extensionPluginDataRecord
			if err := json.Unmarshal(bytes.TrimSpace(line), &record); err != nil {
				return err
			}
			includes, err := c.pluginDataIncludesForRestore(ctx, record.ExtensionID)
			if err != nil {
				return err
			}
			if err := writePluginDataRecord(kernel.PluginDataRoot(container.ExtRoot, record.ExtensionID), includes, record); err != nil {
				return err
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

func (c *ExtensionBackupContributor) pluginDataIncludesForRestore(ctx context.Context, extensionID string) ([]string, error) {
	container := c.Runtime.Container()
	if container.InstallationRepository == nil || container.DefinitionRepository == nil {
		return nil, fmt.Errorf("extension repositories unavailable for plugin data restore")
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return nil, err
	}
	definition, err := container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
	if err != nil {
		return nil, err
	}
	includes := kernel.ExtensionPluginDataBackupPaths(definition)
	if len(includes) == 0 {
		return nil, fmt.Errorf("extension %s does not declare data.pluginData.include for the backup payload", extensionID)
	}
	return includes, nil
}
