package package_security

import (
	"path"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type NormalizedPath string

type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "macos"
	PlatformLinux   Platform = "linux"
)

type PathCollision struct {
	PathA    string
	PathB    string
	Reason   string
	Platform Platform
}

type SafePathResolver struct {
	maxPathLength     int
	maxDirectoryDepth int
}

func NewSafePathResolver(maxPathLength, maxDirectoryDepth int) *SafePathResolver {
	if maxPathLength <= 0 {
		maxPathLength = 512
	}
	if maxDirectoryDepth <= 0 {
		maxDirectoryDepth = 32
	}
	return &SafePathResolver{
		maxPathLength:     maxPathLength,
		maxDirectoryDepth: maxDirectoryDepth,
	}
}

var (
	windowsReserved = map[string]bool{
		"con": true, "prn": true, "aux": true, "nul": true,
		"com1": true, "com2": true, "com3": true, "com4": true,
		"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
		"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
		"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
	}

	drivePattern = ""
)

func (r *SafePathResolver) NormalizeArchivePath(input string) (NormalizedPath, error) {
	if !utf8.ValidString(input) || strings.ContainsRune(input, 0) {
		return "", ErrPathTraversal
	}

	if len(input) > r.maxPathLength {
		return "", ErrPathTooLong
	}

	if strings.HasPrefix(input, "/") || strings.HasPrefix(input, `\`) ||
		strings.HasPrefix(input, "//") || strings.HasPrefix(input, `\\`) {
		return "", ErrAbsolutePath
	}

	if len(input) >= 2 && input[1] == ':' {
		return "", ErrAbsolutePath
	}

	normalized := strings.ReplaceAll(input, `\`, "/")

	if strings.Contains(normalized, "../") || strings.Contains(normalized, "..\\") ||
		strings.HasSuffix(normalized, "..") || strings.HasPrefix(normalized, "../") {
		return "", ErrPathTraversal
	}

	if strings.HasPrefix(normalized, "./") {
		normalized = normalized[2:]
	}

	clean := path.Clean(normalized)
	if clean == "." || clean == ".." || clean != normalized {
		return "", ErrPathTraversal
	}

	parts := strings.Split(clean, "/")
	if len(parts) > r.maxDirectoryDepth {
		return "", ErrPathDepthExceeded
	}

	for _, part := range parts {
		if err := r.validatePathComponent(part); err != nil {
			return "", err
		}
	}

	return NormalizedPath(clean), nil
}

func (r *SafePathResolver) validatePathComponent(part string) error {
	if part == "" {
		return nil
	}

	trimmed := strings.TrimRight(part, " .")
	if trimmed != part {
		return ErrWindowsReservedName
	}

	base := strings.ToLower(trimmed)
	if dotIdx := strings.LastIndex(base, "."); dotIdx >= 0 {
		base = base[:dotIdx]
	}

	if windowsReserved[base] {
		return ErrWindowsReservedName
	}

	return nil
}

func (r *SafePathResolver) ResolveWithinRoot(root string, normalized NormalizedPath) (string, error) {
	cleanRoot := path.Clean(strings.ReplaceAll(root, `\`, "/"))
	resolved := path.Join(cleanRoot, string(normalized))

	if !strings.HasPrefix(path.Clean(resolved), cleanRoot+"/") && path.Clean(resolved) != cleanRoot {
		return "", ErrPathTraversal
	}

	return resolved, nil
}

func (r *SafePathResolver) DetectCollision(paths []NormalizedPath, platform Platform) []PathCollision {
	seen := make(map[string]string)
	var collisions []PathCollision

	for _, p := range paths {
		key := r.collisionKey(string(p), platform)
		if existing, ok := seen[key]; ok {
			collisions = append(collisions, PathCollision{
				PathA:    existing,
				PathB:    string(p),
				Reason:   "case_insensitive_collision",
				Platform: platform,
			})
		} else {
			seen[key] = string(p)
		}

		if platform == PlatformMacOS {
			nfcKey := norm.NFC.String(string(p))
			if nfcKey != string(p) {
				if existing, ok := seen[nfcKey]; ok {
					collisions = append(collisions, PathCollision{
						PathA:    existing,
						PathB:    string(p),
						Reason:   "unicode_normalization_collision",
						Platform: platform,
					})
				}
			}
		}
	}

	return collisions
}

func (r *SafePathResolver) collisionKey(p string, platform Platform) string {
	if platform == PlatformWindows || platform == PlatformMacOS {
		return strings.ToLower(p)
	}
	return p
}
