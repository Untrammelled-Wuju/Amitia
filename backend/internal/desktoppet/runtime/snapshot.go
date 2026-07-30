// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/installation"
)

type SnapshotBuilder struct {
	repo    installation.Repository
	dataDir string
}

func NewSnapshotBuilder(repo installation.Repository, dataDir string) *SnapshotBuilder {
	return &SnapshotBuilder{
		repo:    repo,
		dataDir: dataDir,
	}
}

func (b *SnapshotBuilder) BuildForRuntime(ctx context.Context, runtimeID, userID string) (contracts.DesiredRuntimeSnapshot, error) {
	inst, err := b.repo.GetActiveInstallation(userID)
	if err != nil {
		if errors.Is(err, installation.ErrInstallationNotFound) {
			return contracts.DesiredRuntimeSnapshot{
				DesiredRevision: b.generateRevision(),
				EnsureAbsent:    true,
				DesiredPet:      nil,
				GeneratedAt:     time.Now(),
			}, nil
		}
		return contracts.DesiredRuntimeSnapshot{}, err
	}

	if inst.Status != installation.StatusEnabled {
		return contracts.DesiredRuntimeSnapshot{}, fmt.Errorf("active installation %s is not enabled (status=%s)", inst.ID, inst.Status)
	}

	installRootAbs := filepath.Join(b.dataDir, filepath.FromSlash(inst.InstallPath))
	if _, err := os.Stat(installRootAbs); err != nil {
		if os.IsNotExist(err) {
			return contracts.DesiredRuntimeSnapshot{}, fmt.Errorf("install root not found: %s", installRootAbs)
		}
		return contracts.DesiredRuntimeSnapshot{}, fmt.Errorf("failed to stat install root: %w", err)
	}

	manifestAbs := filepath.Join(b.dataDir, filepath.FromSlash(inst.ManifestPath))
	if _, err := os.Stat(manifestAbs); err != nil {
		if os.IsNotExist(err) {
			return contracts.DesiredRuntimeSnapshot{}, fmt.Errorf("manifest not found: %s", manifestAbs)
		}
		return contracts.DesiredRuntimeSnapshot{}, fmt.Errorf("failed to stat manifest: %w", err)
	}

	settings, err := b.repo.GetRuntimeSettings(inst.ID)
	if err != nil {
		return contracts.DesiredRuntimeSnapshot{}, fmt.Errorf("failed to get runtime settings: %w", err)
	}

	desiredRevision := b.generateRevision()

	instSnapshot, err := b.buildInstallationSnapshot(inst)
	if err != nil {
		return contracts.DesiredRuntimeSnapshot{}, err
	}

	settingsSnapshot := b.buildSettingsSnapshot(settings, desiredRevision)

	spawn := contracts.SpawnPayload{
		DesiredRevision: desiredRevision,
		Installation:    instSnapshot,
		Settings:        settingsSnapshot,
	}

	return contracts.DesiredRuntimeSnapshot{
		DesiredRevision: desiredRevision,
		EnsureAbsent:    false,
		DesiredPet:      &spawn,
		GeneratedAt:     time.Now(),
	}, nil
}

func (b *SnapshotBuilder) validateInstallPath(relPath string) (string, error) {
	cleanDataDir := filepath.Clean(b.dataDir)
	absPath := filepath.Join(cleanDataDir, filepath.FromSlash(relPath))
	cleaned := filepath.Clean(absPath)

	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate symlinks for path %s: %w", cleaned, err)
	}

	rel, err := filepath.Rel(cleanDataDir, resolved)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path from data dir: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes data directory: %s", resolved)
	}

	return resolved, nil
}

func (b *SnapshotBuilder) buildInstallationSnapshot(inst *installation.Installation) (contracts.InstallationSnapshot, error) {
	installRoot, err := b.validateInstallPath(inst.InstallPath)
	if err != nil {
		return contracts.InstallationSnapshot{}, fmt.Errorf("invalid install path: %w", err)
	}

	manifestPath, err := b.validateInstallPath(inst.ManifestPath)
	if err != nil {
		return contracts.InstallationSnapshot{}, fmt.Errorf("invalid manifest path: %w", err)
	}

	return contracts.InstallationSnapshot{
		InstallationID:   inst.ID,
		CharacterID:      inst.CharacterID,
		PackageID:        inst.PackageID,
		PackageVersion:   inst.PackageVersion,
		InstallRoot:      installRoot,
		ManifestPath:     manifestPath,
		PackageHash:      inst.PackageHash,
		DefaultActionKey: inst.DefaultActionKey,
		CanvasWidth:      inst.CanvasWidth,
		CanvasHeight:     inst.CanvasHeight,
	}, nil
}

func (b *SnapshotBuilder) buildSettingsSnapshot(settings *installation.RuntimeSettings, revision int64) contracts.SettingsSnapshot {
	return contracts.SettingsSnapshot{
		Revision:         revision,
		AlwaysOnTop:      settings.AlwaysOnTop != 0,
		Scale:            settings.Scale,
		PositionX:        settings.PositionX,
		PositionY:        settings.PositionY,
		ScreenID:         settings.ScreenID,
		ClickThroughMode: settings.ClickThroughMode,
		SoundEnabled:     settings.SoundEnabled != 0,
	}
}

func (b *SnapshotBuilder) generateRevision() int64 {
	return time.Now().UnixNano()
}
