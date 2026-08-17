// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"gorm.io/gorm"
)

type RepositoryV2 interface {
	DB() *gorm.DB

	GetInstallationForUserDevice(userID, deviceID, installationID string) (*Installation, error)
	ListInstallationsForUserDevice(userID, deviceID string) ([]*Installation, error)
	CreateInstallationTx(tx *gorm.DB, installation *Installation) error
	UpdateInstallationTx(tx *gorm.DB, installation *Installation) error
	GetInstallationByUserDevicePetTx(tx *gorm.DB, userID, deviceID, petID string) (*Installation, error)
	DeleteInstallationTx(tx *gorm.DB, id string) error

	GetRuntimeSettingsForUserDevice(userID, deviceID, installationID string) (*RuntimeSettings, error)
	CreateRuntimeSettingsTx(tx *gorm.DB, settings *RuntimeSettings) error
	UpdateRuntimeSettingsCAS(tx *gorm.DB, installationID, userID, deviceID string, expectedRevision int, updates map[string]interface{}) (*RuntimeSettings, error)

	GetActiveBindingForUserDeviceTx(tx *gorm.DB, userID, deviceID string) (*binding.DeviceActiveInstallationBinding, error)
	UpsertActiveBindingTx(tx *gorm.DB, binding *binding.DeviceActiveInstallationBinding) error
	DeleteActiveBindingTx(tx *gorm.DB, userID, deviceID string) error
	InsertBindingHistoryTx(tx *gorm.DB, entry *binding.BindingHistoryEntry) error

	GetRuntimeDesiredStateTx(tx *gorm.DB, userID, deviceID string) (*desired.RuntimeDesiredState, error)
	UpsertRuntimeDesiredStateCAS(tx *gorm.DB, userID, deviceID string, state *desired.RuntimeDesiredState, expectedRevision int64) (*desired.RuntimeDesiredState, error)
	AllocateDeviceDesiredRevisionCAS(tx *gorm.DB, userID, deviceID string) (int64, error)
	GetDeviceDesiredRevisionCounterTx(tx *gorm.DB, userID, deviceID string) (*desired.DeviceDesiredRevisionCounter, error)

	CreateOutboxEventTx(tx *gorm.DB, event *desired.DesiredStateOutboxEvent) error
	ListPendingOutboxEvents(limit int) ([]*desired.DesiredStateOutboxEvent, error)
	MarkOutboxEventPublished(tx *gorm.DB, eventID string) error
	MarkOutboxEventFailed(tx *gorm.DB, eventID, errorMsg string) error
	RequeueOutboxEventsBefore(tx *gorm.DB, availableBefore string) error

	CreateOperationTx(tx *gorm.DB, op *operation.InstallationOperation) error
	GetOperationTx(tx *gorm.DB, operationID string) (*operation.InstallationOperation, error)
	GetOperationByIdempotencyKeyTx(tx *gorm.DB, userID, deviceID, idempotencyKey, opType string) (*operation.InstallationOperation, error)
	UpdateOperationTx(tx *gorm.DB, op *operation.InstallationOperation) error
	UpdateOperationStatusCAS(tx *gorm.DB, operationID, expectedStatus, newStatus, executionID string) (*operation.InstallationOperation, error)
	ClaimOperationLeaseCAS(tx *gorm.DB, lease *operation.Lease, expectedStatuses []string) (*operation.InstallationOperation, error)
	RenewOperationLeaseTx(tx *gorm.DB, operationID, executionID string) error
	ListPendingOperations(limit int) ([]*operation.InstallationOperation, error)
	ListExpiredLeaseOperations(leaseTimeout string, limit int) ([]*operation.InstallationOperation, error)

	CreateCommitJournalTx(tx *gorm.DB, journal *journal.InstallationCommitJournal) error
	GetCommitJournalTx(tx *gorm.DB, operationID string) (*journal.InstallationCommitJournal, error)
	CASUpdateCommitJournalStageTx(tx *gorm.DB, operationID, expectedStage, newStage, executionID string) (*journal.InstallationCommitJournal, error)
	ListPendingCommitJournals(limit int) ([]*journal.InstallationCommitJournal, error)

	CreateSwitchJournalTx(tx *gorm.DB, journal *journal.InstallationSwitchJournal) error
	GetSwitchJournalTx(tx *gorm.DB, operationID string) (*journal.InstallationSwitchJournal, error)
	CASUpdateSwitchJournalStageTx(tx *gorm.DB, operationID, expectedStage, newStage, executionID string) (*journal.InstallationSwitchJournal, error)
	ListPendingSwitchJournals(limit int) ([]*journal.InstallationSwitchJournal, error)

	CreateTrashEntryTx(tx *gorm.DB, entry *TrashEntry) error
	ListExpiredTrashEntries(retainBefore string, limit int) ([]*TrashEntry, error)
	MarkTrashEntryPurged(tx *gorm.DB, id string) error

	GetRuntimeProjectionTx(tx *gorm.DB, userID, deviceID string) (*projection.InstallationRuntimeProjection, error)
	UpsertRuntimeProjectionTx(tx *gorm.DB, projection *projection.InstallationRuntimeProjection) error

	GetOrCreateDeviceContext(ctx context.Context, userID string, reqCtx device.RequestContext) (*device.DeviceContext, error)

	Transaction(ctx context.Context, fn func(repo RepositoryV2) error) error
}

type TrashEntry struct {
	ID             string
	OperationID    string
	InstallationID string
	StorageKey     string
	Reason         string
	ContentHash    string
	RetainUntil    string
	Status         string
	CreatedAt      string
	PurgedAt       string
}

func (TrashEntry) TableName() string {
	return "desktop_pet_installation_trash_entries"
}
