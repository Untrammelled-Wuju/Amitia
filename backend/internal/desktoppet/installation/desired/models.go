// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desired

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type RuntimeDesiredState struct {
	ID string

	UserID    string
	DeviceID  string
	RuntimeID string

	InstallationID string
	PetID           string
	ReleaseID       string

	DesiredEnabled bool
	DesiredVisible bool

	DesiredActionKey string

	SettingsSnapshotJSON string
	SettingsRevision     int64

	DesiredRevision int64
	DesiredHash     string

	Reason      string
	OperationID string

	CreatedAt string
	UpdatedAt string
}

func (RuntimeDesiredState) TableName() string {
	return "desktop_pet_runtime_desired_states"
}

type DeviceDesiredRevisionCounter struct {
	UserID    string
	DeviceID  string
	CurrentRevision int64
	UpdatedAt      string
}

func (DeviceDesiredRevisionCounter) TableName() string {
	return "desktop_pet_device_desired_revision_counters"
}

type DesiredStateOutboxEvent struct {
	EventID string
	EventType string
	UserID    string
	DeviceID  string
	RuntimeID string
	InstallationID string
	DesiredRevision int64
	DesiredHash     string
	OperationID     string
	PayloadJSON     string
	Status          string
	AttemptCount    int
	AvailableAt     string
	LastError       string
	CreatedAt       string
	PublishedAt     string
}

func (DesiredStateOutboxEvent) TableName() string {
	return "desktop_pet_runtime_desired_state_outbox"
}

type DeviceDesiredSnapshot struct {
	DesiredRevision int64
	DesiredHash     string
	EnsureAbsent    bool
	InstallationID  string
	PetID           string
	ReleaseID       string
	UserID          string
	DeviceID        string
	RuntimeID       string
}

type DesiredHashFields struct {
	InstallationLabels map[string]string
	DesiredLabels      map[string]string
	SettingsLabels     map[string]string
}

func ComputeDesiredHash(fields DesiredHashFields) string {
	parts := make([]string, 0, 64)
	parts = append(parts, collectSortedPairs(fields.InstallationLabels)...)
	parts = append(parts, collectSortedPairs(fields.DesiredLabels)...)
	parts = append(parts, collectSortedPairs(fields.SettingsLabels)...)
	body := strings.Join(parts, "&")
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:16])
}

func collectSortedPairs(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+m[k])
	}
	return pairs
}
