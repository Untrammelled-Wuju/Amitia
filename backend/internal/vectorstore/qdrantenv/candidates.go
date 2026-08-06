// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"os"
	"path/filepath"

	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/resourceuri"
)

func resolveGuestFilename(guest platform.GuestPlatform) string {
	switch guest {
	case platform.GuestPlatformWindows:
		return "qdrant.exe"
	default:
		return "qdrant"
	}
}

func runtimePackageCandidates(guest platform.GuestPlatform) []string {
	base := resolveGuestFilename(guest)
	return []string{
		"amitia://runtime/qdrant/bin/" + base,
		"amitia://runtime/qdrant/" + base,
	}
}

func standardInstallTarget(guest platform.GuestPlatform, resourceRoot string) string {
	base := resolveGuestFilename(guest)
	return filepath.Join(resourceRoot, "bin", base)
}

func legacyCandidates(guest platform.GuestPlatform, architecture string, runtimeRoot, workspaceDir string) []string {
	var candidates []string

	switch guest {
	case platform.GuestPlatformWindows:
		if runtimeRoot != "" {
			candidates = append(candidates,
				filepath.Join(runtimeRoot, "qdrant", "qdrant.exe"),
				filepath.Join(runtimeRoot, "backend", "qdrant", "qdrant.exe"),
			)
		}
		if workspaceDir != "" {
			candidates = append(candidates,
				filepath.Join(workspaceDir, "backend", "qdrant", "qdrant.exe"),
			)
		}
	case platform.GuestPlatformLinux:
		switch architecture {
		case "arm64":
			if runtimeRoot != "" {
				candidates = append(candidates,
					filepath.Join(runtimeRoot, "qdrant", "qdrant"),
					filepath.Join(runtimeRoot, "qdrant", "qdrant_linux_aarch64"),
					filepath.Join(runtimeRoot, "qdrant", "qdrant-aarch64-unknown-linux-gnu"),
					filepath.Join(runtimeRoot, "backend", "qdrant", "qdrant"),
					filepath.Join(runtimeRoot, "backend", "qdrant", "qdrant_linux_aarch64"),
				)
			}
			if workspaceDir != "" {
				candidates = append(candidates,
					filepath.Join(workspaceDir, "backend", "qdrant", "qdrant"),
					filepath.Join(workspaceDir, "backend", "qdrant", "qdrant_linux_aarch64"),
				)
			}
		case "amd64":
			if runtimeRoot != "" {
				candidates = append(candidates,
					filepath.Join(runtimeRoot, "qdrant", "qdrant"),
					filepath.Join(runtimeRoot, "qdrant", "qdrant_linux_x86"),
					filepath.Join(runtimeRoot, "qdrant", "qdrant-x86_64-unknown-linux-gnu"),
					filepath.Join(runtimeRoot, "backend", "qdrant", "qdrant"),
					filepath.Join(runtimeRoot, "backend", "qdrant", "qdrant_linux_x86"),
				)
			}
			if workspaceDir != "" {
				candidates = append(candidates,
					filepath.Join(workspaceDir, "backend", "qdrant", "qdrant"),
					filepath.Join(workspaceDir, "backend", "qdrant", "qdrant_linux_x86"),
				)
			}
		default:
			if runtimeRoot != "" {
				candidates = append(candidates,
					filepath.Join(runtimeRoot, "qdrant", "qdrant"),
					filepath.Join(runtimeRoot, "backend", "qdrant", "qdrant"),
				)
			}
			if workspaceDir != "" {
				candidates = append(candidates,
					filepath.Join(workspaceDir, "backend", "qdrant", "qdrant"),
				)
			}
		}
	case platform.GuestPlatformMacOS:
		if runtimeRoot != "" {
			candidates = append(candidates,
				filepath.Join(runtimeRoot, "qdrant", "qdrant"),
				filepath.Join(runtimeRoot, "backend", "qdrant", "qdrant"),
			)
		}
		if workspaceDir != "" {
			candidates = append(candidates,
				filepath.Join(workspaceDir, "backend", "qdrant", "qdrant"),
			)
		}
	}

	return deduplicateCandidates(candidates)
}

func deduplicateCandidates(candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, c := range candidates {
		cleaned := filepath.Clean(c)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func parseResourceURI(raw string) (resourceuri.ResourceURI, error) {
	return resourceuri.Parse(raw)
}

func isValidBinaryPath(info os.FileInfo, guest platform.GuestPlatform) (bool, string) {
	if info == nil {
		return false, "not found"
	}
	if info.IsDir() {
		return false, "is directory"
	}
	if guest != platform.GuestPlatformWindows {
		if !hasExecutableBit(info) {
			return false, "not executable"
		}
	}
	return true, ""
}

func hasExecutableBit(info os.FileInfo) bool {
	return info.Mode().Perm()&0111 != 0
}
