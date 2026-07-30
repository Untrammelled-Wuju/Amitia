package packageformat

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var windowsReservedNames = map[string]bool{
	"con":  true,
	"prn":  true,
	"aux":  true,
	"nul":  true,
	"com1": true,
	"com2": true,
	"com3": true,
	"com4": true,
	"com5": true,
	"com6": true,
	"com7": true,
	"com8": true,
	"com9": true,
	"lpt1": true,
	"lpt2": true,
	"lpt3": true,
	"lpt4": true,
	"lpt5": true,
	"lpt6": true,
	"lpt7": true,
	"lpt8": true,
	"lpt9": true,
}

func NormalizePackagePath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("path is empty")
	}

	nfc := norm.NFC.String(raw)
	if nfc != raw {
		raw = nfc
	}

	if strings.ContainsAny(raw, "\\\r\n\t\x00") {
		return "", fmt.Errorf("path contains backslash or control character: %q", raw)
	}

	if strings.Contains(raw, "\x00") {
		return "", fmt.Errorf("path contains NUL byte")
	}

	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("path contains control character: U+%04X", r)
		}
	}

	if len(raw) >= 2 && raw[1] == ':' {
		drive := unicode.ToLower(rune(raw[0]))
		if drive >= 'a' && drive <= 'z' {
			return "", fmt.Errorf("path contains drive letter: %q", raw)
		}
	}

	if strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("path is absolute: %q", raw)
	}

	cleaned := filepath.Clean(raw)
	cleaned = filepath.ToSlash(cleaned)

	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("path resolves to empty or current directory: %q", raw)
	}

	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("path escapes root via parent reference: %q", raw)
	}

	segments := strings.Split(cleaned, "/")
	for _, seg := range segments {
		if seg == "" {
			return "", fmt.Errorf("path contains empty segment: %q", raw)
		}
		if seg == "." {
			return "", fmt.Errorf("path contains dot segment after clean: %q", raw)
		}
		if seg == ".." {
			return "", fmt.Errorf("path contains parent segment after clean: %q", raw)
		}

		lower := strings.ToLower(seg)
		if windowsReservedNames[lower] {
			return "", fmt.Errorf("path contains Windows reserved name: %q", seg)
		}

		if strings.Contains(seg, ":") {
			return "", fmt.Errorf("path contains ADS colon in segment: %q", seg)
		}

		trimmed := strings.TrimRight(seg, ". ")
		if trimmed != seg {
			return "", fmt.Errorf("path segment has trailing dot or space: %q", seg)
		}

		if !utf8.ValidString(seg) {
			return "", fmt.Errorf("path segment is not valid UTF-8: %q", seg)
		}
	}

	return cleaned, nil
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

	rootSlash := filepath.ToSlash(cleanRoot) + "/"
	joinedSlash := filepath.ToSlash(absJoined)
	if absJoined != cleanRoot && !strings.HasPrefix(joinedSlash, rootSlash) {
		return "", fmt.Errorf("path escapes root: %q", relPath)
	}

	return absJoined, nil
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
