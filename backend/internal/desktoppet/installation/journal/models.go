// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package journal

import (
	"errors"
)

var (
	ErrJournalStateConflict = errors.New("journal: state conflict")
	ErrJournalNotFound      = errors.New("journal: not found")
	ErrInvalidJournalStage  = errors.New("journal: invalid stage transition")
)

const (
	JournalStageOperationCreated    = "operation_created"
	JournalStageReleaseVerified    = "release_verified"
	JournalStageStagingPrepared   = "staging_prepared"
	JournalStageStagingVerified   = "staging_verified"
	JournalStageOldInstallParked  = "old_install_parked"
	JournalStageFilesPublished    = "files_published"
	JournalStageDatabaseCommitted  = "database_committed"
	JournalStageRuntimeDesiredUpdated = "runtime_desired_updated"
	JournalStageRuntimeApplied    = "runtime_applied"
	JournalStageCleanupCompleted  = "cleanup_completed"
	JournalStageCompleted         = "completed"
	JournalStageFailedRetryable = "failed_retryable"
	JournalStageFailedTerminal  = "failed_terminal"
)

type InstallationCommitJournal struct {
	ID string

	OperationID string

	UserID    string
	DeviceID  string
	RuntimeID string

	InstallationID    string
	PetID             string
	SourceReleaseID   string
	TargetReleaseID   string

	Stage   string
	Status  string

	ExecutionID      string
	ExpectedOldStatus string

	StagingPathKey  string
	RollbackPathKey string
	PublishedPathKey  string
	TrashPathKey     string

	ErrorCode    string
	ErrorMessage string

	CreatedAt string
	UpdatedAt string
}

func (InstallationCommitJournal) TableName() string {
	return "desktop_pet_installation_commit_journals"
}

func (j InstallationCommitJournal) IsTerminal() bool {
	return j.Stage == JournalStageCompleted ||
		j.Stage == JournalStageFailedRetryable ||
		j.Stage == JournalStageFailedTerminal
}

func (j InstallationCommitJournal) CanTransitionTo(newStage string) bool {
	if j.IsTerminal() {
		return false
	}
	valid, ok := commitJournalTransitions[j.Stage]
	return ok && valid[newStage]
}

var commitJournalTransitions = map[string]map[string]bool{
	JournalStageOperationCreated: {
		JournalStageReleaseVerified: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageReleaseVerified: {
		JournalStageStagingPrepared: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageStagingPrepared: {
		JournalStageStagingVerified: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageStagingVerified: {
		JournalStageOldInstallParked: true,
		JournalStageDatabaseCommitted: true,  // repair skip files publish
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageOldInstallParked: {
		JournalStageFilesPublished: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageFilesPublished: {
		JournalStageDatabaseCommitted: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageDatabaseCommitted: {
		JournalStageRuntimeDesiredUpdated: true, JournalStageCompleted: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageRuntimeDesiredUpdated: {
		JournalStageRuntimeApplied: true, JournalStageCompleted: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageRuntimeApplied: {
		JournalStageCleanupCompleted: true, JournalStageCompleted: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
	JournalStageCleanupCompleted: {
		JournalStageCompleted: true,
		JournalStageFailedRetryable: true, JournalStageFailedTerminal: true,
	},
}

const (
	SwitchJournalCreated          = "created"
	SwitchJournalBindingCommitted  = "binding_committed"
	SwitchJournalDesiredCommitted  = "desired_committed"
	SwitchJournalRuntimeApplied    = "runtime_applied"
	SwitchJournalCompleted         = "completed"
	SwitchJournalFailedRetryable = "failed_retryable"
	SwitchJournalFailedTerminal   = "failed_terminal"
)

type InstallationSwitchJournal struct {
	ID string

	OperationID string

	UserID    string
	DeviceID  string
	RuntimeID string

	OldInstallationID  string
	NewInstallationID  string

	OldBindingRevision int64
	NewBindingRevision int64

	OldDesiredRevision int64
	NewDesiredRevision int64

	OldReleaseID string
	NewReleaseID string

	Stage  string
	Status string

	ExecutionID      string
	ExpectedOldStage string

	ErrorCode    string
	ErrorMessage string

	CreatedAt string
	UpdatedAt string
}

func (InstallationSwitchJournal) TableName() string {
	return "desktop_pet_installation_switch_journals"
}

func (j InstallationSwitchJournal) IsTerminal() bool {
	return j.Stage == SwitchJournalCompleted ||
		j.Stage == SwitchJournalFailedRetryable ||
		j.Stage == SwitchJournalFailedTerminal
}

func (j InstallationSwitchJournal) CanTransitionTo(newStage string) bool {
	if j.IsTerminal() {
		return false
	}
	valid, ok := switchJournalTransitions[j.Stage]
	return ok && valid[newStage]
}

var switchJournalTransitions = map[string]map[string]bool{
	SwitchJournalCreated: {
		SwitchJournalBindingCommitted: true,
		SwitchJournalFailedRetryable: true, SwitchJournalFailedTerminal: true,
	},
	SwitchJournalBindingCommitted: {
		SwitchJournalDesiredCommitted: true,
		SwitchJournalFailedRetryable: true, SwitchJournalFailedTerminal: true,
	},
	SwitchJournalDesiredCommitted: {
		SwitchJournalRuntimeApplied: true, SwitchJournalCompleted: true,
		SwitchJournalFailedRetryable: true, SwitchJournalFailedTerminal: true,
	},
	SwitchJournalRuntimeApplied: {
		SwitchJournalCompleted: true,
		SwitchJournalFailedRetryable: true, SwitchJournalFailedTerminal: true,
	},
}
