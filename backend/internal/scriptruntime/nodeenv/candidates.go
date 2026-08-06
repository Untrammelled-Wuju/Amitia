// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import (
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/pkg/platform"
)

type candidatePath struct {
	source Source
	path   string
}

type candidateURIPath struct {
	source Source
	uri    string
}

func nodeFileNameForGuest(guest platform.GuestPlatform) string {
	switch guest {
	case platform.GuestPlatformWindows:
		return "node.exe"
	default:
		return "node"
	}
}

func runtimePackageNodeCandidates(guest platform.GuestPlatform) []candidateURIPath {
	switch guest {
	case platform.GuestPlatformWindows:
		return []candidateURIPath{
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/node.exe"},
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/bin/node.exe"},
		}
	case platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		return []candidateURIPath{
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/bin/node"},
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/node"},
		}
	default:
		return nil
	}
}

func runtimePackageNPMCandidates(guest platform.GuestPlatform) []candidateURIPath {
	switch guest {
	case platform.GuestPlatformWindows:
		return []candidateURIPath{
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/node_modules/npm/bin/npm-cli.js"},
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/lib/node_modules/npm/bin/npm-cli.js"},
		}
	case platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		return []candidateURIPath{
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/lib/node_modules/npm/bin/npm-cli.js"},
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/node_modules/npm/bin/npm-cli.js"},
		}
	default:
		return nil
	}
}

func runtimePackageNPXCandidates(guest platform.GuestPlatform) []candidateURIPath {
	switch guest {
	case platform.GuestPlatformWindows:
		return []candidateURIPath{
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/node_modules/npm/bin/npx-cli.js"},
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/lib/node_modules/npm/bin/npx-cli.js"},
		}
	case platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		return []candidateURIPath{
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/lib/node_modules/npm/bin/npx-cli.js"},
			{source: SourceRuntimePackage, uri: "amitia://runtime/node/node_modules/npm/bin/npx-cli.js"},
		}
	default:
		return nil
	}
}

func legacyNodeCandidates(guest platform.GuestPlatform, runtimeRoot, workspaceDir string) []candidatePath {
	switch guest {
	case platform.GuestPlatformWindows:
		return buildLegacyCandidatesWindows(guest, runtimeRoot, workspaceDir)
	case platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		return buildLegacyCandidatesUnix(guest, runtimeRoot, workspaceDir)
	default:
		return nil
	}
}

func buildLegacyCandidatesWindows(guest platform.GuestPlatform, runtimeRoot, workspaceDir string) []candidatePath {
	var candidates []candidatePath
	fileName := nodeFileNameForGuest(guest)
	if runtimeRoot != "" {
		candidates = append(candidates,
			candidatePath{source: SourceLegacyBundled, path: filepath.Join(runtimeRoot, "node", fileName)},
			candidatePath{source: SourceLegacyBundled, path: filepath.Join(runtimeRoot, "backend", "node", fileName)},
		)
	}
	if workspaceDir != "" {
		candidates = append(candidates,
			candidatePath{source: SourceLegacyBundled, path: filepath.Join(workspaceDir, "backend", "node", fileName)},
		)
	}
	return dedupeCandidates(candidates)
}

func buildLegacyCandidatesUnix(guest platform.GuestPlatform, runtimeRoot, workspaceDir string) []candidatePath {
	var candidates []candidatePath
	fileName := nodeFileNameForGuest(guest)
	if runtimeRoot != "" {
		candidates = append(candidates,
			candidatePath{source: SourceLegacyBundled, path: filepath.Join(runtimeRoot, "node", "bin", fileName)},
			candidatePath{source: SourceLegacyBundled, path: filepath.Join(runtimeRoot, "node", fileName)},
			candidatePath{source: SourceLegacyBundled, path: filepath.Join(runtimeRoot, "backend", "node", fileName)},
		)
	}
	if workspaceDir != "" {
		candidates = append(candidates,
			candidatePath{source: SourceLegacyBundled, path: filepath.Join(workspaceDir, "backend", "node", fileName)},
		)
	}
	return dedupeCandidates(candidates)
}

func legacyNpmCandidates(distributionRoot string, guest platform.GuestPlatform) []candidatePath {
	if distributionRoot == "" {
		return nil
	}
	switch guest {
	case platform.GuestPlatformWindows:
		return []candidatePath{
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "node_modules", "npm", "bin", "npm-cli.js")},
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "lib", "node_modules", "npm", "bin", "npm-cli.js")},
		}
	default:
		return []candidatePath{
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "lib", "node_modules", "npm", "bin", "npm-cli.js")},
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "node_modules", "npm", "bin", "npm-cli.js")},
		}
	}
}

func legacyNpxCandidates(distributionRoot string, guest platform.GuestPlatform) []candidatePath {
	if distributionRoot == "" {
		return nil
	}
	switch guest {
	case platform.GuestPlatformWindows:
		return []candidatePath{
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "node_modules", "npm", "bin", "npx-cli.js")},
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "lib", "node_modules", "npm", "bin", "npx-cli.js")},
		}
	default:
		return []candidatePath{
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "lib", "node_modules", "npm", "bin", "npx-cli.js")},
			{source: SourceLegacyBundled, path: filepath.Join(distributionRoot, "node_modules", "npm", "bin", "npx-cli.js")},
		}
	}
}

func deriveDistributionRoot(nodeBinary string, guest platform.GuestPlatform) string {
	if nodeBinary == "" {
		return ""
	}
	dir := filepath.Dir(nodeBinary)
	switch guest {
	case platform.GuestPlatformWindows:
		base := filepath.Base(dir)
		if base == "bin" {
			return filepath.Dir(dir)
		}
		return dir
	case platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		base := filepath.Base(dir)
		if base == "bin" {
			return filepath.Dir(dir)
		}
		return dir
	default:
		return dir
	}
}

func dedupeCandidates(candidates []candidatePath) []candidatePath {
	if len(candidates) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(candidates))
	result := make([]candidatePath, 0, len(candidates))
	for _, c := range candidates {
		if _, ok := seen[c.path]; ok {
			continue
		}
		seen[c.path] = struct{}{}
		result = append(result, c)
	}
	return result
}

func isShellWrapperExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".cmd", ".bat", ".ps1", ".sh":
		return true
	default:
		return false
	}
}

func isPackageManagerExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".mjs", ".cjs":
		return true
	default:
		return false
	}
}
