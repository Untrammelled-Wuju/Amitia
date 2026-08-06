// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import (
	"path/filepath"

	"github.com/u-ai/backend/pkg/platform"
)

type Environment struct {
	NodeBinary                 string
	NPMCLI                     string
	NPXCLI                     string
	WorkDir                    string
	DistributionRoot           string
	Source                     Source
	Guest                      platform.GuestPlatform
	Architecture               string
	PackageManagementAvailable bool
}

func (e Environment) Clone() Environment {
	return e
}

func (e Environment) Validate() error {
	if e.NodeBinary == "" {
		return newInvalidNodeBinary("", "node binary path is empty")
	}
	if !filepath.IsAbs(e.NodeBinary) {
		return newInvalidNodeBinary(e.NodeBinary, "node binary path is not absolute")
	}
	if e.WorkDir == "" {
		return &invalidWorkDirError{reason: "work directory is empty"}
	}
	if !filepath.IsAbs(e.WorkDir) {
		return &invalidWorkDirError{reason: "work directory is not absolute"}
	}
	if e.DistributionRoot == "" {
		return newInvalidNodeBinary("", "distribution root is empty")
	}
	if !filepath.IsAbs(e.DistributionRoot) {
		return newInvalidNodeBinary(e.DistributionRoot, "distribution root is not absolute")
	}
	if e.NPMCLI != "" && !filepath.IsAbs(e.NPMCLI) {
		return &invalidPackageManagerCLIError{path: e.NPMCLI, reason: "npm CLI path is not absolute"}
	}
	if e.NPXCLI != "" && !filepath.IsAbs(e.NPXCLI) {
		return &invalidPackageManagerCLIError{path: e.NPXCLI, reason: "npx CLI path is not absolute"}
	}
	if e.PackageManagementAvailable {
		if e.NPMCLI == "" || e.NPXCLI == "" {
			return &invalidPackageManagerCLIError{reason: "package management marked available but CLI paths are incomplete"}
		}
	}
	if e.Guest == platform.GuestPlatformUnknown {
		return newUnsupportedGuest(e.Guest)
	}
	if e.Architecture == "" {
		return newInvalidNodeBinary(e.NodeBinary, "architecture is empty")
	}
	return nil
}

type invalidWorkDirError struct {
	reason string
}

func (e *invalidWorkDirError) Error() string {
	return ErrInvalidWorkDir.Error() + ": " + e.reason
}

func (e *invalidWorkDirError) Is(target error) bool {
	return target == ErrInvalidWorkDir
}

type invalidPackageManagerCLIError struct {
	path   string
	reason string
}

func (e *invalidPackageManagerCLIError) Error() string {
	if e.path != "" {
		return ErrInvalidPackageManagerCLI.Error() + ": path=" + e.path + " " + e.reason
	}
	return ErrInvalidPackageManagerCLI.Error() + ": " + e.reason
}

func (e *invalidPackageManagerCLIError) Is(target error) bool {
	return target == ErrInvalidPackageManagerCLI
}
