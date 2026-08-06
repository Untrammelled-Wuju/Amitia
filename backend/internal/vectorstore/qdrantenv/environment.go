// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"path/filepath"

	"github.com/u-ai/backend/pkg/platform"
)

type Environment struct {
	BinaryPath        string
	DistributionRoot  string
	Source            Source
	Guest             platform.GuestPlatform
	Architecture      string
	Installed         bool
	Explicit          bool
}

func (e Environment) Clone() Environment {
	return e
}

func (e Environment) Validate() error {
	if e.BinaryPath == "" {
		return newInvalidQdrantBinary("", "binary path is empty")
	}
	if !filepath.IsAbs(e.BinaryPath) {
		return newInvalidQdrantBinary(e.BinaryPath, "binary path is not absolute")
	}
	if e.DistributionRoot == "" {
		return newInvalidQdrantBinary("", "distribution root is empty")
	}
	if !filepath.IsAbs(e.DistributionRoot) {
		return newInvalidQdrantBinary(e.DistributionRoot, "distribution root is not absolute")
	}
	if e.Source == "" {
		return newInvalidQdrantBinary("", "source is empty")
	}
	if e.Source != SourceExplicit && e.Source != SourceRuntimePackage && e.Source != SourceLegacyBundled {
		return newInvalidQdrantBinary("", "unknown source: "+string(e.Source))
	}
	if e.Guest == platform.GuestPlatformUnknown {
		return newUnsupportedGuest(e.Guest)
	}
	if e.Architecture == "" {
		return newInvalidQdrantBinary("", "architecture is empty")
	}
	if e.Explicit && e.Source != SourceExplicit {
		return newInvalidQdrantBinary("", "explicit flag requires explicit source")
	}
	if !e.Installed && e.Explicit {
		return newInvalidQdrantBinary("", "explicit binary cannot be uninstalled")
	}
	return nil
}
