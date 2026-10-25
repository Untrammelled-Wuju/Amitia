package ui_handler

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (h *HTTPHandler) resolveExtensionBasePath(extensionID string) string {
	if h.extRoot == "" || extensionID == "" {
		return ""
	}
	safeID := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(extensionID)
	installationsRoot := filepath.Join(h.extRoot, "installations", safeID)
	if currentData, err := os.ReadFile(filepath.Join(installationsRoot, "current.json")); err == nil {
		var current struct {
			GenerationID string `json:"generationID"`
		}
		if json.Unmarshal(currentData, &current) == nil && current.GenerationID != "" {
			candidate := filepath.Join(installationsRoot, "generations", current.GenerationID)
			if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
				return candidate
			}
		}
	}
	installedRoot := filepath.Join(h.extRoot, "installed", safeID)
	entries, err := os.ReadDir(installedRoot)
	if err != nil {
		return ""
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}
		versionDir := filepath.Join(installedRoot, entries[i].Name())
		subEntries, err := os.ReadDir(versionDir)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			candidate := filepath.Join(versionDir, sub.Name())
			if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
				return candidate
			}
		}
	}
	return ""
}

func sanitizeUIModulePath(root, resourcePath string) (string, error) {
	decoded, err := url.PathUnescape(strings.TrimSpace(resourcePath))
	if err != nil || decoded == "" || strings.ContainsRune(decoded, '\x00') {
		return "", errors.New("ui module resource path is invalid")
	}
	clean := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(decoded, "/")))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", errors.New("ui module resource path escapes the extension")
	}
	fullPath := filepath.Join(root, clean)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			resolvedParent, parentErr := filepath.EvalSymlinks(filepath.Dir(fullPath))
			if parentErr != nil {
				return "", parentErr
			}
			parentRelative, parentRelErr := filepath.Rel(resolvedRoot, resolvedParent)
			if parentRelErr != nil || parentRelative == ".." || strings.HasPrefix(parentRelative, ".."+string(filepath.Separator)) || filepath.IsAbs(parentRelative) {
				return "", errors.New("ui module resource path escapes the extension")
			}
			return filepath.ToSlash(clean), nil
		}
		return "", err
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("ui module resource path escapes the extension")
	}
	return filepath.ToSlash(relative), nil
}

func uiModuleMIME(resourcePath string) string {
	switch strings.ToLower(filepath.Ext(resourcePath)) {
	case ".mjs", ".js":
		return "text/javascript"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".woff2":
		return "font/woff2"
	case ".wasm":
		return "application/wasm"
	default:
		return ""
	}
}
