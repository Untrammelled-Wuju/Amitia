// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type Kind string

const (
	KindPluginHost Kind = "plugin-host"
	KindTaskHost   Kind = "task-host"
)

type Artifact struct {
	Kind             Kind
	EntryPath        string
	DistributionRoot string
	Source           Source
}

type NodeEnvironmentResolver interface {
	Resolve(context.Context) (nodeenv.Environment, error)
}

type ArtifactResolver interface {
	Resolve(context.Context, Kind) (Artifact, error)
}

func (a Artifact) Validate() error {
	if !knownKind(a.Kind) {
		return newInvalidHostArtifact(a.Kind, a.EntryPath, "unknown kind")
	}
	if a.EntryPath == "" {
		return newInvalidHostArtifact(a.Kind, a.EntryPath, "entry path is empty")
	}
	if !filepath.IsAbs(a.EntryPath) {
		return newInvalidHostArtifact(a.Kind, a.EntryPath, "entry path is not absolute")
	}
	if a.DistributionRoot == "" {
		return newInvalidHostArtifact(a.Kind, a.EntryPath, "distribution root is empty")
	}
	if !filepath.IsAbs(a.DistributionRoot) {
		return newInvalidHostArtifact(a.Kind, a.EntryPath, "distribution root is not absolute")
	}
	if !knownSource(a.Source) {
		return newInvalidHostArtifact(a.Kind, a.EntryPath, "unknown source: "+string(a.Source))
	}
	if !isHostEntryExtension(a.EntryPath) {
		return newUnsupportedHostEntry(a.Kind, a.EntryPath, filepath.Ext(a.EntryPath))
	}
	return nil
}

func knownKind(k Kind) bool {
	switch k {
	case KindPluginHost, KindTaskHost:
		return true
	}
	return false
}

func isHostEntryExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".mjs", ".cjs":
		return true
	}
	return false
}

func deriveDistributionRoot(entryPath string) string {
	base := filepath.Base(entryPath)
	if base == "index.js" || base == "index.mjs" || base == "index.cjs" {
		dir := filepath.Dir(entryPath)
		parent := filepath.Dir(dir)
		return filepath.Clean(parent)
	}
	return filepath.Clean(filepath.Dir(entryPath))
}
