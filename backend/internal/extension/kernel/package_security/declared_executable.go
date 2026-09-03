package package_security

import (
	"archive/zip"
	"encoding/json"
	"io"
	"path"
	"strings"
)

type executableManifestView struct {
	Modules []struct {
		ID      string `json:"id"`
		Runtime *struct {
			Type       string `json:"type"`
			EntryPoint string `json:"entryPoint"`
		} `json:"runtime"`
	} `json:"modules"`
}

// discoverDeclaredServiceExecutables returns only exact package paths that a
// manifest declares as process service entry points. It deliberately does not
// make arbitrary executable content legal.
func discoverDeclaredServiceExecutables(reader *zip.Reader, policy ArchivePolicy) map[string]struct{} {
	allowed := make(map[string]struct{})
	if reader == nil || !policy.AllowDeclaredExecutable {
		return allowed
	}

	var manifestFile *zip.File
	for _, item := range reader.File {
		if item == nil || item.FileInfo().IsDir() {
			continue
		}
		if strings.ReplaceAll(item.Name, "\\", "/") == "manifest.json" {
			manifestFile = item
			break
		}
	}
	if manifestFile == nil || int64(manifestFile.UncompressedSize64) > policy.MaxSingleEntryBytes {
		return allowed
	}

	rc, err := manifestFile.Open()
	if err != nil {
		return allowed
	}
	data, err := io.ReadAll(limitReader(rc, policy.MaxSingleEntryBytes))
	rc.Close()
	if err != nil || int64(len(data)) > policy.MaxSingleEntryBytes {
		return allowed
	}

	var manifest executableManifestView
	if err := json.Unmarshal(data, &manifest); err != nil {
		return allowed
	}
	for _, module := range manifest.Modules {
		if module.Runtime == nil || strings.TrimSpace(module.Runtime.Type) != "service" {
			continue
		}
		moduleID := strings.TrimSpace(module.ID)
		entryPoint := strings.TrimSpace(module.Runtime.EntryPoint)
		if !safeModuleIDForExecutablePath(moduleID) || !safeRelativeEntrypoint(entryPoint) {
			continue
		}
		packagePath := path.Join("modules", moduleID, entryPoint)
		allowed[strings.ToLower(packagePath)] = struct{}{}
	}
	return allowed
}

func safeModuleIDForExecutablePath(moduleID string) bool {
	if moduleID == "" || moduleID == "." || moduleID == ".." {
		return false
	}
	if strings.ContainsAny(moduleID, `/\:`) || strings.ContainsRune(moduleID, '\x00') {
		return false
	}
	return path.Clean(moduleID) == moduleID
}

func safeRelativeEntrypoint(entryPoint string) bool {
	if entryPoint == "" || entryPoint == "." || entryPoint != strings.TrimSpace(entryPoint) {
		return false
	}
	if strings.Contains(entryPoint, "\\") || strings.ContainsRune(entryPoint, '\x00') || path.IsAbs(entryPoint) {
		return false
	}
	cleaned := path.Clean(entryPoint)
	if cleaned != entryPoint || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return false
	}
	// Drive-qualified and URI-like values must never become package paths.
	if len(entryPoint) >= 2 && entryPoint[1] == ':' {
		return false
	}
	return true
}
