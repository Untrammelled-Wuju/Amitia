// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import "fmt"

const ownershipSchemaVersion = 1

type OwnershipState string

const (
	StateAcquiring OwnershipState = "acquiring"
	StateRunning   OwnershipState = "running"
	StateStopping  OwnershipState = "stopping"
	StateExited    OwnershipState = "exited"
	StateFailed    OwnershipState = "failed"
)

type OwnershipRecord struct {
	SchemaVersion int            `json:"schemaVersion"`
	ComponentID   string         `json:"componentId"`
	LaunchID      string         `json:"launchId"`
	State         OwnershipState `json:"state"`

	Owner ProcessIdentity  `json:"owner"`
	Child *ProcessIdentity `json:"child,omitempty"`

	ExecutablePath string `json:"executablePath"`
	ConfigPath     string `json:"configPath"`

	CreatedAtEpochMillis int64 `json:"createdAtEpochMillis"`
	UpdatedAtEpochMillis int64 `json:"updatedAtEpochMillis"`
}

func (r OwnershipRecord) Validate() error {
	if r.SchemaVersion != ownershipSchemaVersion {
		return fmt.Errorf("%w: schema version %d", ErrOwnershipRecordCorrupted, r.SchemaVersion)
	}
	if r.ComponentID == "" {
		return fmt.Errorf("%w: empty component ID", ErrOwnershipRecordInvalid)
	}
	if r.LaunchID == "" {
		return fmt.Errorf("%w: empty launch ID", ErrOwnershipRecordInvalid)
	}
	if err := r.Owner.Validate(); err != nil {
		return fmt.Errorf("%w: owner: %v", ErrOwnershipRecordInvalid, err)
	}
	return nil
}
