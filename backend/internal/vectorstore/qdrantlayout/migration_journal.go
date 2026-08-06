// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

type MigrationState string

const (
	MigrationStateNotRequired MigrationState = "not-required"
	MigrationStatePending     MigrationState = "pending"
	MigrationStateCopying     MigrationState = "copying"
	MigrationStateVerified    MigrationState = "verified"
	MigrationStateCompleted   MigrationState = "completed"
	MigrationStateFailed      MigrationState = "failed"
)

type MigrationJournal struct {
	SchemaVersion int `json:"schemaVersion"`

	StorageSource string `json:"storageSource"`
	StorageTarget string `json:"storageTarget"`

	SnapshotsSource string `json:"snapshotsSource"`
	SnapshotsTarget string `json:"snapshotsTarget"`

	StorageState   MigrationState `json:"storageState"`
	SnapshotsState MigrationState `json:"snapshotsState"`

	CompletedAtEpochMillis int64 `json:"completedAtEpochMillis"`
}
