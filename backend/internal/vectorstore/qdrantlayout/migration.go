// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Migrator interface {
	Migrate(context.Context) error
}

type MigratorOptions struct {
	Copier DirectoryCopier
	Clock  Clock
}

type migrator struct {
	layout Layout
	opts   MigratorOptions
}

func NewMigrator(layout Layout, opts MigratorOptions) Migrator {
	if opts.Copier == nil {
		opts.Copier = NewDirectoryCopier()
	}
	if opts.Clock == nil {
		opts.Clock = NewClock()
	}
	return &migrator{layout: layout, opts: opts}
}

func (m *migrator) Migrate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	journal, journalPath, err := m.readJournal()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read journal: %w", err)
	}

	if journal != nil && journal.StorageState == MigrationStateCompleted && journal.SnapshotsState == MigrationStateCompleted {
		return nil
	}

	if journal == nil {
		journal = &MigrationJournal{
			SchemaVersion:  1,
			StorageState:   MigrationStateNotRequired,
			SnapshotsState: MigrationStateNotRequired,
		}
	}

	if err := m.migrateStorage(ctx, journal, journalPath); err != nil {
		return err
	}

	if err := m.migrateSnapshots(ctx, journal, journalPath); err != nil {
		return err
	}

	if journal.StorageState == MigrationStateCompleted && journal.SnapshotsState == MigrationStateCompleted {
		journal.CompletedAtEpochMillis = m.opts.Clock.Now().UnixMilli()
	}

	return m.writeJournal(journal, journalPath)
}

func (m *migrator) migrateStorage(ctx context.Context, journal *MigrationJournal, journalPath string) error {
	journal.StorageState = MigrationStatePending

	legacyCandidates := []string{
		filepath.Join(m.layout.DistributionRoot, "data"),
		filepath.Join(m.layout.DistributionRoot, "storage"),
	}

	var legacyStorage string
	var legacyFound []string
	for _, cand := range legacyCandidates {
		exists, err := dirExistsNotEmpty(cand)
		if err != nil {
			journal.StorageState = MigrationStateFailed
			return fmt.Errorf("check legacy storage %s: %w", cand, err)
		}
		if exists {
			legacyFound = append(legacyFound, cand)
			if legacyStorage == "" {
				legacyStorage = cand
			}
		}
	}

	if len(legacyFound) > 1 {
		journal.StorageState = MigrationStateFailed
		return newLegacyDataConflict(legacyFound[0], legacyFound[1])
	}

	if legacyStorage == "" {
		journal.StorageState = MigrationStateNotRequired
		return nil
	}

	storage := m.layout.StorageDir
	newExists, err := dirExists(storage)
	if err != nil {
		journal.StorageState = MigrationStateFailed
		return fmt.Errorf("check storage: %w", err)
	}
	if newExists {
		empty, err := isEmptyDir(storage)
		if err != nil {
			journal.StorageState = MigrationStateFailed
			return fmt.Errorf("check storage empty: %w", err)
		}
		if !empty {
			journal.StorageState = MigrationStateNotRequired
			return nil
		}
		_ = os.Remove(storage)
	}

	journal.StorageSource = legacyStorage
	journal.StorageTarget = storage
	_ = m.writeJournal(journal, journalPath)

	stagingDir := filepath.Join(m.layout.DataRoot, ".storage-migrating")
	if _, err := os.Stat(stagingDir); err == nil {
		journal.StorageState = MigrationStateFailed
		return newMigrationFailed("storage-staging", fmt.Errorf("staging dir already exists"))
	}

	journal.StorageState = MigrationStateCopying
	_ = m.writeJournal(journal, journalPath)

	if err := m.opts.Copier.CopyAndVerify(ctx, legacyStorage, stagingDir); err != nil {
		journal.StorageState = MigrationStateFailed
		_ = m.writeJournal(journal, journalPath)
		return newMigrationFailed("storage-copy", err)
	}

	journal.StorageState = MigrationStateVerified
	_ = m.writeJournal(journal, journalPath)

	if err := os.Rename(stagingDir, storage); err != nil {
		journal.StorageState = MigrationStateFailed
		_ = m.writeJournal(journal, journalPath)
		return newMigrationFailed("storage-rename", err)
	}

	journal.StorageState = MigrationStateCompleted
	return nil
}

func (m *migrator) migrateSnapshots(ctx context.Context, journal *MigrationJournal, journalPath string) error {
	journal.SnapshotsState = MigrationStatePending

	legacy := filepath.Join(m.layout.DistributionRoot, "snapshots")
	legacyExists, err := dirExistsNotEmpty(legacy)
	if err != nil {
		journal.SnapshotsState = MigrationStateFailed
		return fmt.Errorf("check legacy snapshots: %w", err)
	}

	if !legacyExists {
		journal.SnapshotsState = MigrationStateNotRequired
		return nil
	}

	snapshots := m.layout.SnapshotsDir
	newExists, err := dirExists(snapshots)
	if err != nil {
		journal.SnapshotsState = MigrationStateFailed
		return fmt.Errorf("check snapshots: %w", err)
	}
	if newExists {
		empty, err := isEmptyDir(snapshots)
		if err != nil {
			journal.SnapshotsState = MigrationStateFailed
			return fmt.Errorf("check snapshots empty: %w", err)
		}
		if !empty {
			journal.SnapshotsState = MigrationStateNotRequired
			return nil
		}
		_ = os.Remove(snapshots)
	}

	journal.SnapshotsSource = legacy
	journal.SnapshotsTarget = snapshots
	_ = m.writeJournal(journal, journalPath)

	stagingDir := filepath.Join(m.layout.DataRoot, ".snapshots-migrating")
	if _, err := os.Stat(stagingDir); err == nil {
		journal.SnapshotsState = MigrationStateFailed
		return newMigrationFailed("snapshots-staging", fmt.Errorf("staging dir already exists"))
	}

	journal.SnapshotsState = MigrationStateCopying
	_ = m.writeJournal(journal, journalPath)

	if err := m.opts.Copier.CopyAndVerify(ctx, legacy, stagingDir); err != nil {
		journal.SnapshotsState = MigrationStateFailed
		_ = m.writeJournal(journal, journalPath)
		return newMigrationFailed("snapshots-copy", err)
	}

	journal.SnapshotsState = MigrationStateVerified
	_ = m.writeJournal(journal, journalPath)

	if err := os.Rename(stagingDir, snapshots); err != nil {
		journal.SnapshotsState = MigrationStateFailed
		_ = m.writeJournal(journal, journalPath)
		return newMigrationFailed("snapshots-rename", err)
	}

	journal.SnapshotsState = MigrationStateCompleted
	return nil
}

func (m *migrator) readJournal() (*MigrationJournal, string, error) {
	journalPath := filepath.Join(m.layout.MigrationDir, "layout-v1.json")
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return nil, journalPath, err
	}
	var journal MigrationJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return nil, journalPath, fmt.Errorf("unmarshal journal: %w", err)
	}
	return &journal, journalPath, nil
}

func (m *migrator) writeJournal(journal *MigrationJournal, journalPath string) error {
	if journalPath == "" {
		journalPath = filepath.Join(m.layout.MigrationDir, "layout-v1.json")
	}
	if err := os.MkdirAll(m.layout.MigrationDir, 0750); err != nil {
		return fmt.Errorf("create migration dir: %w", err)
	}
	data, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}
	tmpPath := journalPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write journal tmp: %w", err)
	}
	if err := os.Rename(tmpPath, journalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename journal: %w", err)
	}
	return nil
}

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func dirExistsNotEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	if isEmptyDirLocal(path) {
		return false, nil
	}
	return true, nil
}

func isEmptyDirLocal(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true
	}
	return err != nil
}

type Clock interface {
	Now() time.Time
}

type clockImpl struct{}

func NewClock() Clock {
	return &clockImpl{}
}

func (c *clockImpl) Now() time.Time {
	return time.Now().UTC()
}
