// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type InstallRequest struct {
	Target    Environment
	Inspector FileInspector
	Unzip     func(archivePath, destination string) error
}

type ensureInstaller struct {
	inspector FileInspector
	unzip     func(archivePath, destination string) error
}

func NewInstaller(inspector FileInspector, unzip func(archivePath, destination string) error) *ensureInstaller {
	if inspector == nil {
		inspector = newDefaultFileInspector()
	}
	if unzip == nil {
		unzip = util.UnzipFile
	}
	return &ensureInstaller{
		inspector: inspector,
		unzip:     unzip,
	}
}

func (i *ensureInstaller) EnsureInstalled(req InstallRequest) error {
	if req.Target.Explicit {
		return NewInvalidInstallTargetError("explicit binary cannot be auto-installed")
	}
	if req.Target.Source != SourceRuntimePackage {
		return NewInvalidInstallTargetError("only runtime-package source supports auto-install")
	}
	if req.Target.Installed {
		return nil
	}
	if req.Target.DistributionRoot == "" {
		return NewInvalidInstallTargetError("distribution root is empty")
	}

	guest := req.Target.Guest
	candidates := archiveCandidates(guest, req.Target.Architecture)

	searchDirs := []string{
		req.Target.DistributionRoot,
	}
	if req.Target.DistributionRoot != "" {
		parent := filepath.Dir(req.Target.DistributionRoot)
		if parent != "" && parent != req.Target.DistributionRoot {
			searchDirs = append(searchDirs, filepath.Join(parent, "qdrant"))
		}
	}

	for _, dir := range searchDirs {
		for _, name := range candidates {
			archivePath := filepath.Join(dir, name)
			if _, err := i.inspector.Stat(archivePath); err != nil {
				continue
			}
			dest := req.Target.DistributionRoot
			if err := i.unzip(archivePath, dest); err != nil {
				return NewArchiveExtractionError(archivePath, err)
			}
			if i.hasExecError(req.Target.BinaryPath) {
				_ = i.ensureExecutable(req.Target.BinaryPath, guest)
			}
			return nil
		}
	}

	return NewInstallArchiveNotFoundError(req.Target.DistributionRoot)
}

func (i *ensureInstaller) hasExecError(binaryPath string) bool {
	info, err := i.inspector.Stat(binaryPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0111 == 0
}

func (i *ensureInstaller) ensureExecutable(binaryPath string, guest platform.GuestPlatform) error {
	if guest == platform.GuestPlatformWindows {
		return nil
	}
	info, err := i.inspector.Stat(binaryPath)
	if err != nil {
		return err
	}
	mode := info.Mode()
	if mode.Perm()&0111 != 0 {
		return nil
	}
	return os.Chmod(binaryPath, 0755)
}

func archiveCandidates(guest platform.GuestPlatform, architecture string) []string {
	switch guest {
	case platform.GuestPlatformWindows:
		return []string{
			"qdrant.exe.zip",
			"qdrant-windows-amd64.zip",
			"qdrant.zip",
		}
	case platform.GuestPlatformLinux:
		switch architecture {
		case "arm64":
			return []string{
				"qdrant_linux_aarch64.zip",
				"qdrant-aarch64-unknown-linux-gnu.zip",
				"qdrant-arm64.zip",
				"qdrant.zip",
			}
		case "amd64":
			return []string{
				"qdrant_linux_x86.zip",
				"qdrant-x86_64-unknown-linux-gnu.zip",
				"qdrant-amd64.zip",
				"qdrant.zip",
			}
		default:
			return []string{"qdrant.zip"}
		}
	case platform.GuestPlatformMacOS:
		switch architecture {
		case "arm64":
			return []string{
				"qdrant-darwin-arm64.zip",
				"qdrant-macos-arm64.zip",
				"qdrant.zip",
			}
		case "amd64":
			return []string{
				"qdrant-darwin-amd64.zip",
				"qdrant-macos-amd64.zip",
				"qdrant.zip",
			}
		default:
			return []string{"qdrant.zip"}
		}
	default:
		return []string{"qdrant.zip"}
	}
}

var (
	ErrInvalidInstallTarget   = errors.New("qdrantenv: invalid install target")
	ErrArchiveExtraction      = errors.New("qdrantenv: archive extraction failed")
	ErrInstallArchiveNotFound = errors.New("qdrantenv: no archive found for installation")
)

type invalidInstallTargetError struct {
	reason string
}

func (e *invalidInstallTargetError) Error() string {
	return fmt.Sprintf("%s: %s", ErrInvalidInstallTarget.Error(), e.reason)
}

func (e *invalidInstallTargetError) Is(target error) bool {
	return target == ErrInvalidInstallTarget
}

type archiveExtractionError struct {
	archive string
	err     error
}

func (e *archiveExtractionError) Error() string {
	return fmt.Sprintf("%s: archive=%s err=%v", ErrArchiveExtraction.Error(), e.archive, e.err)
}

func (e *archiveExtractionError) Is(target error) bool {
	return target == ErrArchiveExtraction
}

func (e *archiveExtractionError) Unwrap() error {
	return e.err
}

type installArchiveNotFoundError struct {
	distributionRoot string
}

func (e *installArchiveNotFoundError) Error() string {
	return fmt.Sprintf("%s: root=%s", ErrInstallArchiveNotFound.Error(), e.distributionRoot)
}

func (e *installArchiveNotFoundError) Is(target error) bool {
	return target == ErrInstallArchiveNotFound
}

func NewInvalidInstallTargetError(reason string) error {
	return &invalidInstallTargetError{reason: reason}
}

func NewArchiveExtractionError(archive string, err error) error {
	return &archiveExtractionError{archive: archive, err: err}
}

func NewInstallArchiveNotFoundError(distributionRoot string) error {
	return &installArchiveNotFoundError{distributionRoot: distributionRoot}
}
