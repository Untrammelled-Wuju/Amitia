// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import (
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/pkg/platform"
)

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

func bundledNodeArchiveNames(guest platform.GuestPlatform) []string {
	switch guest {
	case platform.GuestPlatformWindows:
		return []string{"node.exe.zip", "node.zip"}
	case platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		return []string{"node.zip"}
	default:
		return nil
	}
}

func bundledNodePaths(guest platform.GuestPlatform, nodeDir string) []string {
	fileName := nodeFileNameForGuest(guest)
	switch guest {
	case platform.GuestPlatformWindows:
		return []string{
			filepath.Join(nodeDir, fileName),
			filepath.Join(nodeDir, "bin", fileName),
		}
	case platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		return []string{
			filepath.Join(nodeDir, "bin", fileName),
			filepath.Join(nodeDir, fileName),
		}
	default:
		return nil
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
