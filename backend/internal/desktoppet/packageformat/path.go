package packageformat

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	maxPackagePathBytes    = 512
	maxPackageSegmentBytes = 255
)

var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// NormalizePackagePath validates a package-relative path against the frozen
// Package V2 portability contract. Paths use '/' on every host, must already
// be NFC and canonical, and must be materializable on Windows/macOS/Linux.
func NormalizePackagePath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("path is empty")
	}
	if !utf8.ValidString(raw) {
		return "", fmt.Errorf("path is not valid UTF-8")
	}
	if !norm.NFC.IsNormalString(raw) {
		return "", fmt.Errorf("path is not NFC-normalized: %q", raw)
	}
	if len(raw) > maxPackagePathBytes {
		return "", fmt.Errorf("path exceeds %d UTF-8 bytes", maxPackagePathBytes)
	}
	if strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("path is absolute: %q", raw)
	}
	if strings.Contains(raw, "\\") {
		return "", fmt.Errorf("path contains backslash: %q", raw)
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("path contains control character: U+%04X", r)
		}
	}

	cleaned := pathpkg.Clean(raw)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("path resolves to empty or current directory: %q", raw)
	}
	if cleaned != raw {
		return "", fmt.Errorf("path is not canonical: %q", raw)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path escapes root via parent reference: %q", raw)
	}

	for _, seg := range strings.Split(cleaned, "/") {
		if err := validatePackagePathSegment(seg); err != nil {
			return "", err
		}
	}
	return cleaned, nil
}

func validatePackagePathSegment(seg string) error {
	if seg == "" {
		return fmt.Errorf("path contains empty segment")
	}
	if seg == "." {
		return fmt.Errorf("path contains dot segment")
	}
	if seg == ".." {
		return fmt.Errorf("path contains parent segment")
	}
	if len(seg) > maxPackageSegmentBytes {
		return fmt.Errorf("path segment exceeds %d UTF-8 bytes: %q", maxPackageSegmentBytes, seg)
	}
	if strings.ContainsAny(seg, `<>:"|?*`) {
		return fmt.Errorf("path segment contains Windows-forbidden character: %q", seg)
	}
	if strings.TrimRight(seg, ". ") != seg {
		return fmt.Errorf("path segment has trailing dot or space: %q", seg)
	}
	base := seg
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	if windowsReservedNames[strings.ToLower(base)] {
		return fmt.Errorf("path contains Windows reserved name: %q", seg)
	}
	return nil
}

func SecureJoinUnderRoot(root, relPath string) (string, error) {
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("failed to resolve root: %w", err)
	}
	normalized, err := NormalizePackagePath(relPath)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(cleanRoot, filepath.FromSlash(normalized))
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("failed to resolve joined path: %w", err)
	}
	rel, err := filepath.Rel(cleanRoot, absJoined)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %q", relPath)
	}
	return absJoined, nil
}

// SecureResolveExistingUnderRoot resolves an existing package path without
// following symlinks in any package-owned path component.
func SecureResolveExistingUnderRoot(root, relPath string) (string, error) {
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("failed to resolve root: %w", err)
	}
	rootInfo, err := os.Lstat(cleanRoot)
	if err != nil {
		return "", fmt.Errorf("failed to inspect root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("package root must be a real directory")
	}
	normalized, err := NormalizePackagePath(relPath)
	if err != nil {
		return "", err
	}
	current := cleanRoot
	segments := strings.Split(normalized, "/")
	for index, segment := range segments {
		current = filepath.Join(current, filepath.FromSlash(segment))
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink is not allowed in package path: %q", normalized)
		}
		if index < len(segments)-1 && !info.IsDir() {
			return "", fmt.Errorf("non-directory package path component: %q", segment)
		}
	}
	return SecureJoinUnderRoot(cleanRoot, normalized)
}

func IsSafeRelativePath(p string) bool {
	_, err := NormalizePackagePath(p)
	return err == nil
}

func CaseFoldPath(p string) string {
	normalized, err := NormalizePackagePath(p)
	if err != nil {
		normalized = p
	}
	return strings.ToLower(normalized)
}
