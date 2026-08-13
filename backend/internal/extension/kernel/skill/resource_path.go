// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	disallowedChars = "\x00"
)

func ValidateResourceRelativePath(relPath string) error {
	if relPath == "" {
		return ErrResourcePathInvalid
	}
	if !utf8.ValidString(relPath) {
		return ErrResourcePathInvalid
	}
	if strings.ContainsAny(relPath, disallowedChars) {
		return ErrResourcePathInvalid
	}
	if filepath.IsAbs(relPath) {
		return ErrResourcePathTraversal
	}
	if strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, "\\") {
		return ErrResourcePathTraversal
	}
	if strings.Contains(relPath, "\\") {
		return ErrResourcePathInvalid
	}
	if strings.Contains(relPath, "..") {
		return ErrResourcePathTraversal
	}
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if strings.HasPrefix(clean, "..") {
		return ErrResourcePathTraversal
	}
	if clean == ".." || strings.Contains(clean, "/../") {
		return ErrResourcePathTraversal
	}
	segments := strings.Split(clean, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return ErrResourcePathInvalid
		}
	}
	return nil
}

func NormalizeResourcePath(relPath string) string {
	relPath = strings.TrimSpace(relPath)
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	relPath = strings.TrimLeft(relPath, "./")
	return relPath
}

func ClassifyResourceKind(relPath string) string {
	clean := NormalizeResourcePath(relPath)
	if strings.HasPrefix(clean, "references/") {
		return ResourceKindReference
	}
	if strings.HasPrefix(clean, "assets/") {
		return ResourceKindAsset
	}
	if strings.HasPrefix(clean, "scripts/") {
		return ResourceKindScript
	}
	return ResourceKindOther
}

func FileExtension(relPath string) string {
	ext := filepath.Ext(relPath)
	ext = strings.ToLower(ext)
	return ext
}

func HasAllowedTextExtension(relPath string) bool {
	ext := FileExtension(relPath)
	switch ext {
	case ".md", ".txt", ".markdown", ".csv", ".html", ".htm",
		".json", ".yaml", ".yml", ".xml", ".toml", ".ndjson",
		".log", ".ini", ".cfg", ".conf", ".env":
		return true
	}
	return false
}

func RootSegment(relPath string) string {
	clean := NormalizeResourcePath(relPath)
	idx := strings.Index(clean, "/")
	if idx <= 0 {
		return clean
	}
	return clean[:idx]
}
