package coordinator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/log"
)

type ReleaseValidator interface {
	ValidateRelease(ctx context.Context, userID, releaseID string) (*ReleaseValidationResult, error)
}

type ReleaseStager interface {
	PrepareStagingCopy(ctx context.Context, releaseID, installationID string) (stagingPathKey string, err error)
	VerifyStagingCopy(ctx context.Context, releaseID, installationID, stagingPathKey string) error
}

type RuntimeDesiredStatePublisher interface {
	PublishDesiredState(ctx context.Context, deviceCtx device.DeviceContext, snapshot *DesiredStateSnapshot) error
	PublishRecenter(ctx context.Context, deviceCtx device.DeviceContext, installationID string) error
	PublishPlayAction(ctx context.Context, deviceCtx device.DeviceContext, installationID, actionKey string) error
}

type DesiredStateSnapshot struct {
	DesiredRevision      int64  `json:"desiredRevision"`
	DesiredHash          string `json:"desiredHash"`
	InstallationID       string `json:"installationId"`
	PetID                string `json:"petId"`
	ReleaseID            string `json:"releaseId"`
	UserID               string `json:"userId"`
	DeviceID             string `json:"deviceId"`
	RuntimeID            string `json:"runtimeId"`
	EnsureAbsent         bool   `json:"ensureAbsent"`
	DefaultActionKey     string `json:"defaultActionKey"`
	SettingsRevision     int64  `json:"settingsRevision"`
	SettingsSnapshotJSON string `json:"settingsSnapshotJSON"`
}

type InstallationRecord struct {
	ID                string
	UserID            string
	DeviceID          string
	PetID             string
	ReleaseID         string
	Status            string
	Enabled           bool
	InstallStorageKey string
	DefaultActionKey  string
}

type Repository interface {
	CreateOperation(ctx context.Context, op *operation.InstallationOperation) error
	GetOperation(ctx context.Context, userID, deviceID, operationID string) (*operation.InstallationOperation, error)
	UpdateOperation(ctx context.Context, op *operation.InstallationOperation) error
	FindOperationByIdempotencyKey(ctx context.Context, userID, deviceID, key, operationType string) (*operation.InstallationOperation, error)

	GetInstallation(ctx context.Context, userID, deviceID, installationID string) (*InstallationRecord, error)
	GetDesiredStateSnapshot(ctx context.Context, userID, deviceID string) (*DesiredStateSnapshot, error)
	CreateInstallationAndDesiredState(ctx context.Context, op *operation.InstallationOperation, install *InstallationRecord, desired *DesiredStateSnapshot, stagingPathKey string) (desiredRevision int64, err error)
	UpdateDesiredEnabled(ctx context.Context, op *operation.InstallationOperation, installationID string, enabled bool) (desiredRevision int64, err error)
	SwitchRelease(ctx context.Context, op *operation.InstallationOperation, installationID, targetReleaseID, stagingPathKey, defaultActionKey string) (desiredRevision int64, err error)
	UpdateSettings(ctx context.Context, op *operation.InstallationOperation, installationID string, expectedRevision int, updates map[string]interface{}) (settingsRevision int, desiredRevision int64, err error)
	ChangeDefaultAction(ctx context.Context, op *operation.InstallationOperation, installationID, actionKey string) (desiredRevision int64, err error)
	MarkUninstallDesired(ctx context.Context, op *operation.InstallationOperation, installationID string) (desiredRevision int64, err error)
	MarkOperationCancelRequested(ctx context.Context, userID, deviceID, operationID string) error
}

type ProjectionService interface {
	UpdateProjection(ctx context.Context, userID, deviceID string, updateFn func(*Projection) error) error
	HandleRuntimeHeartbeat(ctx context.Context, userID, deviceID, runtimeID string, heartbeat *RuntimeHeartbeat) error
	HandleCommandResult(ctx context.Context, userID, deviceID string, result *CommandResult) error
}

type RuntimeHeartbeat struct {
	InstallationID          string
	PetID                   string
	AppliedDesiredRevision  int64
	AppliedSettingsRevision int64
	ActualReleaseID         string
	ActualVisible           int
	ActualActionKey         string
	ActualHealth            string
	Timestamp               string
}

type CommandResult struct {
	OperationID     string
	Success         bool
	AppliedStage    string
	ErrorCode       string
	ErrorMessage    string
	AppliedRevision int64
	Timestamp       string
}

type Projection struct {
	InstallationID         string
	PetID                  string
	AppliedDesiredRevision int64
	ActualReleaseID        string
	RuntimeSyncState       string
}

type InstallationCoordinator interface {
	Install(ctx context.Context, req InstallRequest) (*InstallResult, error)
	Enable(ctx context.Context, req EnableDisableRequest) (*EnableDisableResult, error)
	Disable(ctx context.Context, req EnableDisableRequest) (*EnableDisableResult, error)
	Switch(ctx context.Context, req SwitchRequest) (*SwitchResult, error)
	Upgrade(ctx context.Context, req UpgradeRequest) (*InstallResult, error)
	Downgrade(ctx context.Context, req DowngradeRequest) (*InstallResult, error)
	Rollback(ctx context.Context, req UpgradeRequest) (*InstallResult, error)
	Repair(ctx context.Context, req RepairRequest) (*InstallResult, error)
	Uninstall(ctx context.Context, req UninstallRequest) (*UninstallResult, error)
	UpdateSettings(ctx context.Context, req SettingsRequest) (*SettingsResult, error)
	ChangeDefaultAction(ctx context.Context, req DefaultActionRequest) (*EnableDisableResult, error)
	Recenter(ctx context.Context, req RecenterRequest) (*EnableDisableResult, error)
	PlayAction(ctx context.Context, deviceCtx device.DeviceContext, installationID, actionKey string) error
	GetOperationStatus(ctx context.Context, userID, deviceID, operationID string) (*operation.InstallationOperation, error)
	CancelOperation(ctx context.Context, userID, deviceID, operationID string) error
}

type Coordinator struct {
	repo             Repository
	releaseValidator ReleaseValidator
	releaseStager    ReleaseStager
	runtimePublisher RuntimeDesiredStatePublisher
	projectionSvc    ProjectionService
	executionID      string
	defaultLeaseTTL  time.Duration
	maxRetryAttempts int
}

func NewCoordinator(
	repo Repository,
	validator ReleaseValidator,
	stager ReleaseStager,
	publisher RuntimeDesiredStatePublisher,
	projection ProjectionService,
) (*Coordinator, error) {
	if repo == nil {
		return nil, errors.New("installation coordinator: repository is required")
	}
	if validator == nil {
		return nil, errors.New("installation coordinator: release validator is required")
	}
	if stager == nil {
		return nil, errors.New("installation coordinator: release stager is required")
	}
	if publisher == nil {
		return nil, errors.New("installation coordinator: runtime publisher is required")
	}
	if projection == nil {
		return nil, errors.New("installation coordinator: projection service is required")
	}
	return &Coordinator{
		repo:             repo,
		releaseValidator: validator,
		releaseStager:    stager,
		runtimePublisher: publisher,
		projectionSvc:    projection,
		executionID:      generateExecutionID(),
		defaultLeaseTTL:  5 * time.Minute,
		maxRetryAttempts: 3,
	}, nil
}

func (c *Coordinator) Install(ctx context.Context, req InstallRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED", ErrorMessage: err.Error()}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeInstall, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.TargetReleaseID, req.PetID, req.CharacterID)
	}
	return c.executeInstall(ctx, req, idempotencyKey)
}

func (c *Coordinator) Enable(ctx context.Context, req EnableDisableRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeEnable, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID)
	}
	return c.executeEnableDisable(ctx, req, idempotencyKey, true)
}

func (c *Coordinator) Disable(ctx context.Context, req EnableDisableRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeDisable, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID)
	}
	return c.executeEnableDisable(ctx, req, idempotencyKey, false)
}

func (c *Coordinator) Switch(ctx context.Context, req SwitchRequest) (*SwitchResult, error) {
	if err := req.Validate(); err != nil {
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeSwitch, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.SourceInstallationID, req.TargetReleaseID)
	}
	return c.executeSwitch(ctx, req, idempotencyKey, operation.TypeSwitch)
}

func (c *Coordinator) Upgrade(ctx context.Context, req UpgradeRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeUpgrade, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.TargetReleaseID)
	}
	result, err := c.executeSwitch(ctx, SwitchRequest{
		DeviceCtx:            req.DeviceCtx,
		SourceInstallationID: req.InstallationID,
		TargetReleaseID:      req.TargetReleaseID,
		IdempotencyKey:       idempotencyKey,
	}, idempotencyKey, operation.TypeUpgrade)
	if result == nil {
		return nil, err
	}
	return &InstallResult{
		OperationID:    result.OperationID,
		InstallationID: result.InstallationID,
		Status:         result.Status,
		Stage:          operation.OpStageWaitingRuntimeACK,
		ErrorCode:      result.ErrorCode,
	}, err
}

func (c *Coordinator) Downgrade(ctx context.Context, req DowngradeRequest) (*InstallResult, error) {
	if !req.SafetyConfirm {
		err := fmt.Errorf("%w: downgrade requires explicit safety confirmation", ErrInvalidDowngradeRequest)
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "SAFETY_CONFIRMATION_REQUIRED"}, err
	}
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeDowngrade, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.TargetReleaseID)
	}
	result, err := c.executeSwitch(ctx, SwitchRequest{
		DeviceCtx:            req.DeviceCtx,
		SourceInstallationID: req.InstallationID,
		TargetReleaseID:      req.TargetReleaseID,
		IdempotencyKey:       idempotencyKey,
	}, idempotencyKey, operation.TypeDowngrade)
	if result == nil {
		return nil, err
	}
	return &InstallResult{
		OperationID:    result.OperationID,
		InstallationID: result.InstallationID,
		Status:         result.Status,
		Stage:          operation.OpStageWaitingRuntimeACK,
		ErrorCode:      result.ErrorCode,
	}, err
}

func (c *Coordinator) Rollback(ctx context.Context, req UpgradeRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeRepair, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.TargetReleaseID)
	}
	result, err := c.executeSwitch(ctx, SwitchRequest{
		DeviceCtx:            req.DeviceCtx,
		SourceInstallationID: req.InstallationID,
		TargetReleaseID:      req.TargetReleaseID,
		IdempotencyKey:       idempotencyKey,
	}, idempotencyKey, operation.TypeRepair)
	if result == nil {
		return nil, err
	}
	return &InstallResult{
		OperationID:    result.OperationID,
		InstallationID: result.InstallationID,
		Status:         result.Status,
		Stage:          operation.OpStageWaitingRuntimeACK,
		ErrorCode:      result.ErrorCode,
	}, err
}

func (c *Coordinator) Repair(ctx context.Context, req RepairRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeRepair, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID)
	}
	return c.executeRepair(ctx, req, idempotencyKey)
}

func (c *Coordinator) Uninstall(ctx context.Context, req UninstallRequest) (*UninstallResult, error) {
	if err := req.Validate(); err != nil {
		return &UninstallResult{Status: operation.OpStatusFailedTerminal, ErrorMessage: err.Error()}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeUninstall, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID)
	}
	return c.executeUninstall(ctx, req, idempotencyKey)
}

func (c *Coordinator) UpdateSettings(ctx context.Context, req SettingsRequest) (*SettingsResult, error) {
	if err := req.Validate(); err != nil {
		return &SettingsResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeSettings, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, fmt.Sprintf("%d", req.ExpectedRevision), stableJSON(req.Updates))
	}
	return c.executeSettings(ctx, req, idempotencyKey)
}

func (c *Coordinator) ChangeDefaultAction(ctx context.Context, req DefaultActionRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeDefaultAction, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.DesiredActionKey)
	}
	return c.executeDefaultAction(ctx, req, idempotencyKey)
}

func (c *Coordinator) Recenter(ctx context.Context, req RecenterRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeRecenter, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, uuid.NewString())
	}
	return c.executeRecenter(ctx, req, idempotencyKey)
}

func (c *Coordinator) PlayAction(ctx context.Context, deviceCtx device.DeviceContext, installationID, actionKey string) error {
	if !deviceCtx.IsValid() || installationID == "" || actionKey == "" {
		return errors.New("coordinator: valid device context, installationID and actionKey are required")
	}
	inst, err := c.repo.GetInstallation(ctx, deviceCtx.UserID, deviceCtx.DeviceID, installationID)
	if err != nil {
		return err
	}
	if inst == nil || inst.UserID != deviceCtx.UserID || inst.DeviceID != deviceCtx.DeviceID {
		return ErrOwnershipMismatch
	}
	return c.runtimePublisher.PublishPlayAction(ctx, deviceCtx, installationID, actionKey)
}

func (c *Coordinator) GetOperationStatus(ctx context.Context, userID, deviceID, operationID string) (*operation.InstallationOperation, error) {
	if operationID == "" {
		return nil, errors.New("coordinator: operationID required")
	}
	return c.repo.GetOperation(ctx, userID, deviceID, operationID)
}

func (c *Coordinator) CancelOperation(ctx context.Context, userID, deviceID, operationID string) error {
	if operationID == "" {
		return errors.New("coordinator: operationID required")
	}
	return c.repo.MarkOperationCancelRequested(ctx, userID, deviceID, operationID)
}

func stableJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func operationRequestHash(op *operation.InstallationOperation, extra map[string]string) string {
	fields := map[string]string{
		"operationType":   op.OperationType,
		"userID":          op.UserID,
		"deviceID":        op.DeviceID,
		"installationID":  op.InstallationID,
		"petID":           op.PetID,
		"sourceReleaseID": op.SourceReleaseID,
		"targetReleaseID": op.TargetReleaseID,
	}
	for key, value := range extra {
		fields[key] = value
	}
	return operation.ComputeRequestHash(fields)
}

func (c *Coordinator) resolveIdempotentOperation(ctx context.Context, op *operation.InstallationOperation) (*operation.InstallationOperation, error) {
	if op == nil || op.IdempotencyKey == "" {
		return nil, nil
	}
	existing, err := c.repo.FindOperationByIdempotencyKey(ctx, op.UserID, op.DeviceID, op.IdempotencyKey, op.OperationType)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	if existing.RequestHash != "" && op.RequestHash != "" && existing.RequestHash != op.RequestHash {
		return nil, operation.ErrIdempotencyConflict
	}
	return existing, nil
}

func (c *Coordinator) buildIdempotencyKey(opType string, parts ...string) string {
	allParts := append([]string{opType}, parts...)
	return operation.ComputeIdempotencyKey(allParts...)
}

func (c *Coordinator) executeInstall(ctx context.Context, req InstallRequest, idempotencyKey string) (*InstallResult, error) {
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeInstall,
		InstallationID:  uuidPrefix("ins_"),
		UserID:          req.DeviceCtx.UserID,
		DeviceID:        req.DeviceCtx.DeviceID,
		RuntimeID:       req.DeviceCtx.RuntimeID,
		PetID:           req.PetID,
		TargetReleaseID: req.TargetReleaseID,
		SourceReleaseID: req.SourceReleaseID,
		IdempotencyKey:  idempotencyKey,
		Status:          operation.OpStatusCreated,
		Stage:           operation.OpStageRequestValidated,
		CreatedAt:       time.Now().Format(operationTimeFormat),
		UpdatedAt:       time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, map[string]string{"characterID": req.CharacterID})
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &InstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: operation.ErrCodeIDEMPOTENCYConflict, ErrorMessage: err.Error()}, err
	} else if existing != nil {
		return &InstallResult{OperationID: existing.ID, InstallationID: existing.InstallationID, Status: existing.Status, Stage: existing.Stage, ErrorCode: existing.ErrorCode, ErrorMessage: existing.ErrorMessage}, nil
	}
	validationResult, err := c.releaseValidator.ValidateRelease(ctx, req.DeviceCtx.UserID, req.TargetReleaseID)
	if err != nil {
		return c.failInstall(ctx, op, operation.OpStageReleaseVerified, err)
	}
	if !validationResult.IsInstallable {
		err := fmt.Errorf("%w: release %s is not installable", ErrReleaseNotInstallable, req.TargetReleaseID)
		return c.failInstall(ctx, op, operation.OpStageReleaseVerified, err)
	}
	if op.PetID == "" {
		op.PetID = validationResult.PetID
	}
	op.Stage = operation.OpStageReleaseVerified
	op.UpdatedAt = time.Now().Format(operationTimeFormat)

	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return c.failInstall(ctx, op, operation.OpStageReleaseVerified, err)
	}

	stagingKey, err := c.releaseStager.PrepareStagingCopy(ctx, req.TargetReleaseID, op.InstallationID)
	if err != nil {
		return c.failInstall(ctx, op, operation.OpStageStagingPrepared, err)
	}

	op.Stage = operation.OpStageStagingPrepared
	op.UpdatedAt = time.Now().Format(operationTimeFormat)

	if err := c.releaseStager.VerifyStagingCopy(ctx, req.TargetReleaseID, op.InstallationID, stagingKey); err != nil {
		return c.failInstall(ctx, op, operation.OpStageStagingVerified, err)
	}

	install := &InstallationRecord{
		ID:                op.InstallationID,
		UserID:            req.DeviceCtx.UserID,
		DeviceID:          req.DeviceCtx.DeviceID,
		PetID:             op.PetID,
		ReleaseID:         req.TargetReleaseID,
		Status:            "installed",
		Enabled:           true,
		InstallStorageKey: stagingKey,
		DefaultActionKey:  validationResult.DefaultActionKey,
	}

	desiredRev, err := c.repo.CreateInstallationAndDesiredState(ctx, op, install, &DesiredStateSnapshot{
		InstallationID:   install.ID,
		PetID:            op.PetID,
		ReleaseID:        req.TargetReleaseID,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		EnsureAbsent:     false,
		DefaultActionKey: validationResult.DefaultActionKey,
	}, stagingKey)
	if err != nil {
		return c.failInstall(ctx, op, operation.OpStageDatabaseCommitted, err)
	}

	op.DesiredRevision = desiredRev
	op.Stage = operation.OpStageDatabaseCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.repo.UpdateOperation(ctx, op); err != nil {
		return c.failInstall(ctx, op, operation.OpStageDatabaseCommitted, err)
	}

	op.Stage = operation.OpStageDesiredStateCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.repo.UpdateOperation(ctx, op); err != nil {
		return c.failInstall(ctx, op, operation.OpStageDesiredStateCommitted, err)
	}

	if err := c.publishPersistedDesiredState(ctx, req.DeviceCtx, false); err != nil {
		return c.failInstall(ctx, op, operation.OpStageRuntimeCommandEnqueued, err)
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.repo.UpdateOperation(ctx, op); err != nil {
		return c.failInstall(ctx, op, operation.OpStageWaitingRuntimeACK, err)
	}

	return &InstallResult{
		OperationID:    op.ID,
		InstallationID: install.ID,
		Status:         operation.OpStatusWaitingRuntimeACK,
		Stage:          operation.OpStageWaitingRuntimeACK,
	}, nil
}

func (c *Coordinator) executeEnableDisable(ctx context.Context, req EnableDisableRequest, idempotencyKey string, enabled bool) (*EnableDisableResult, error) {
	opType := operation.TypeEnable
	if !enabled {
		opType = operation.TypeDisable
	}
	op := &operation.InstallationOperation{
		ID:             uuidPrefix("opin_"),
		OperationType:  opType,
		UserID:         req.DeviceCtx.UserID,
		DeviceID:       req.DeviceCtx.DeviceID,
		RuntimeID:      req.DeviceCtx.RuntimeID,
		InstallationID: req.InstallationID,
		IdempotencyKey: idempotencyKey,
		Status:         operation.OpStatusCreated,
		Stage:          operation.OpStageRequestValidated,
		CreatedAt:      time.Now().Format(operationTimeFormat),
		UpdatedAt:      time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, map[string]string{"enabled": fmt.Sprintf("%t", enabled)})
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: operation.ErrCodeIDEMPOTENCYConflict}, err
	} else if existing != nil {
		return &EnableDisableResult{OperationID: existing.ID, DesiredRevision: existing.DesiredRevision, Status: existing.Status, ErrorCode: existing.ErrorCode}, nil
	}
	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_CREATE_FAILED"}, err
	}

	desiredRev, err := c.repo.UpdateDesiredEnabled(ctx, op, req.InstallationID, enabled)
	if err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "DESIRED_UPDATE_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "DESIRED_UPDATE_FAILED"}, persistErr
	}
	op.DesiredRevision = desiredRev
	op.Stage = operation.OpStageDesiredStateCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.repo.UpdateOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	if err := c.publishPersistedDesiredState(ctx, req.DeviceCtx, false); err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "PUBLISH_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "PUBLISH_FAILED"}, persistErr
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.repo.UpdateOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	return &EnableDisableResult{OperationID: op.ID, DesiredRevision: desiredRev, Status: operation.OpStatusWaitingRuntimeACK}, nil
}

func (c *Coordinator) executeSwitch(ctx context.Context, req SwitchRequest, idempotencyKey, operationType string) (*SwitchResult, error) {
	currentInst, err := c.repo.GetInstallation(ctx, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.SourceInstallationID)
	if err != nil {
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "INSTALLATION_NOT_FOUND"}, err
	}
	if currentInst == nil {
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "INSTALLATION_NOT_FOUND"}, fmt.Errorf("installation not found")
	}
	if currentInst.UserID != req.DeviceCtx.UserID {
		err := fmt.Errorf("%w: installation does not belong to user", ErrOwnershipMismatch)
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "OWNERSHIP_MISMATCH"}, err
	}

	validationResult, err := c.releaseValidator.ValidateRelease(ctx, req.DeviceCtx.UserID, req.TargetReleaseID)
	if err != nil {
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "RELEASE_VALIDATION_FAILED"}, err
	}
	if !validationResult.IsInstallable {
		err := fmt.Errorf("%w: release %s is not installable", ErrReleaseNotInstallable, req.TargetReleaseID)
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "RELEASE_NOT_INSTALLABLE"}, err
	}
	if validationResult.PetID != "" && currentInst.PetID != "" && validationResult.PetID != currentInst.PetID {
		err := fmt.Errorf("%w: target release pet %s does not match installation pet %s", ErrReleaseNotInstallable, validationResult.PetID, currentInst.PetID)
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "RELEASE_PET_MISMATCH"}, err
	}

	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operationType,
		UserID:          req.DeviceCtx.UserID,
		DeviceID:        req.DeviceCtx.DeviceID,
		RuntimeID:       req.DeviceCtx.RuntimeID,
		InstallationID:  req.SourceInstallationID,
		TargetReleaseID: req.TargetReleaseID,
		SourceReleaseID: currentInst.ReleaseID,
		IdempotencyKey:  idempotencyKey,
		Status:          operation.OpStatusCreated,
		Stage:           operation.OpStageRequestValidated,
		CreatedAt:       time.Now().Format(operationTimeFormat),
		UpdatedAt:       time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, nil)
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &SwitchResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: operation.ErrCodeIDEMPOTENCYConflict}, err
	} else if existing != nil {
		return &SwitchResult{OperationID: existing.ID, InstallationID: existing.InstallationID, DesiredRevision: existing.DesiredRevision, Status: existing.Status, ErrorCode: existing.ErrorCode}, nil
	}
	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return &SwitchResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_CREATE_FAILED"}, err
	}

	stagingKey, err := c.releaseStager.PrepareStagingCopy(ctx, req.TargetReleaseID, req.SourceInstallationID)
	if err != nil {
		return &SwitchResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "STAGING_FAILED"}, err
	}

	if err := c.releaseStager.VerifyStagingCopy(ctx, req.TargetReleaseID, req.SourceInstallationID, stagingKey); err != nil {
		return &SwitchResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "STAGING_VERIFY_FAILED"}, err
	}

	desiredRev, err := c.repo.SwitchRelease(ctx, op, req.SourceInstallationID, req.TargetReleaseID, stagingKey, validationResult.DefaultActionKey)
	if err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "SWITCH_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &SwitchResult{OperationID: op.ID, InstallationID: req.SourceInstallationID, Status: operation.OpStatusFailedTerminal, ErrorCode: "SWITCH_FAILED"}, persistErr
	}
	op.DesiredRevision = desiredRev
	op.Stage = operation.OpStageDatabaseCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.repo.UpdateOperation(ctx, op); err != nil {
		return &SwitchResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	if err := c.publishPersistedDesiredState(ctx, req.DeviceCtx, false); err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "PUBLISH_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &SwitchResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "PUBLISH_FAILED"}, persistErr
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &SwitchResult{OperationID: op.ID, InstallationID: req.SourceInstallationID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	return &SwitchResult{OperationID: op.ID, InstallationID: req.SourceInstallationID, DesiredRevision: desiredRev, Status: operation.OpStatusWaitingRuntimeACK}, nil
}

func (c *Coordinator) executeRepair(ctx context.Context, req RepairRequest, idempotencyKey string) (*InstallResult, error) {
	currentInst, err := c.repo.GetInstallation(ctx, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID)
	if err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "INSTALLATION_NOT_FOUND"}, err
	}
	if currentInst == nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "INSTALLATION_NOT_FOUND"}, fmt.Errorf("installation not found")
	}

	validationResult, err := c.releaseValidator.ValidateRelease(ctx, req.DeviceCtx.UserID, currentInst.ReleaseID)
	if err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "RELEASE_VALIDATION_FAILED"}, err
	}
	if !validationResult.IsInstallable {
		err := fmt.Errorf("%w: release %s is not installable", ErrReleaseNotInstallable, currentInst.ReleaseID)
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "RELEASE_NOT_INSTALLABLE"}, err
	}

	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeRepair,
		UserID:          req.DeviceCtx.UserID,
		DeviceID:        req.DeviceCtx.DeviceID,
		RuntimeID:       req.DeviceCtx.RuntimeID,
		InstallationID:  req.InstallationID,
		TargetReleaseID: currentInst.ReleaseID,
		IdempotencyKey:  idempotencyKey,
		Status:          operation.OpStatusCreated,
		Stage:           operation.OpStageRequestValidated,
		CreatedAt:       time.Now().Format(operationTimeFormat),
		UpdatedAt:       time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, nil)
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &InstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: operation.ErrCodeIDEMPOTENCYConflict, ErrorMessage: err.Error()}, err
	} else if existing != nil {
		return &InstallResult{OperationID: existing.ID, InstallationID: existing.InstallationID, Status: existing.Status, Stage: existing.Stage, ErrorCode: existing.ErrorCode, ErrorMessage: existing.ErrorMessage}, nil
	}
	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return &InstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_CREATE_FAILED"}, err
	}

	stagingKey, err := c.releaseStager.PrepareStagingCopy(ctx, currentInst.ReleaseID, req.InstallationID)
	if err != nil {
		return &InstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "STAGING_FAILED"}, err
	}
	if err := c.releaseStager.VerifyStagingCopy(ctx, currentInst.ReleaseID, req.InstallationID, stagingKey); err != nil {
		return c.failInstall(ctx, op, operation.OpStageStagingVerified, err)
	}

	desiredRev, err := c.repo.CreateInstallationAndDesiredState(ctx, op, currentInst, &DesiredStateSnapshot{
		InstallationID:   req.InstallationID,
		PetID:            currentInst.PetID,
		ReleaseID:        currentInst.ReleaseID,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		EnsureAbsent:     false,
		DefaultActionKey: currentInst.DefaultActionKey,
	}, stagingKey)
	if err != nil {
		return c.failInstall(ctx, op, operation.OpStageDatabaseCommitted, err)
	}
	op.DesiredRevision = desiredRev
	op.Stage = operation.OpStageDatabaseCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &InstallResult{OperationID: op.ID, InstallationID: req.InstallationID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	if err := c.publishPersistedDesiredState(ctx, req.DeviceCtx, false); err != nil {
		return c.failInstall(ctx, op, operation.OpStageRuntimeCommandEnqueued, err)
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &InstallResult{OperationID: op.ID, InstallationID: req.InstallationID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	return &InstallResult{OperationID: op.ID, InstallationID: req.InstallationID, Status: operation.OpStatusWaitingRuntimeACK, Stage: operation.OpStageWaitingRuntimeACK}, nil
}

func (c *Coordinator) executeUninstall(ctx context.Context, req UninstallRequest, idempotencyKey string) (*UninstallResult, error) {
	op := &operation.InstallationOperation{
		ID:             uuidPrefix("opin_"),
		OperationType:  operation.TypeUninstall,
		UserID:         req.DeviceCtx.UserID,
		DeviceID:       req.DeviceCtx.DeviceID,
		RuntimeID:      req.DeviceCtx.RuntimeID,
		InstallationID: req.InstallationID,
		IdempotencyKey: idempotencyKey,
		Status:         operation.OpStatusCreated,
		Stage:          operation.OpStageRequestValidated,
		CreatedAt:      time.Now().Format(operationTimeFormat),
		UpdatedAt:      time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, nil)
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorMessage: err.Error()}, err
	} else if existing != nil {
		return &UninstallResult{OperationID: existing.ID, Status: existing.Status, ErrorMessage: existing.ErrorMessage}, nil
	}
	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorMessage: err.Error()}, err
	}

	desiredRev, err := c.repo.MarkUninstallDesired(ctx, op, req.InstallationID)
	if err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "UNINSTALL_DESIRED_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorMessage: persistErr.Error()}, persistErr
	}
	op.DesiredRevision = desiredRev
	op.Stage = operation.OpStageDesiredStateCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorMessage: err.Error()}, err
	}

	if err := c.publishPersistedDesiredState(ctx, req.DeviceCtx, true); err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "PUBLISH_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorMessage: persistErr.Error()}, persistErr
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorMessage: err.Error()}, err
	}

	return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusWaitingRuntimeACK}, nil
}

func (c *Coordinator) executeSettings(ctx context.Context, req SettingsRequest, idempotencyKey string) (*SettingsResult, error) {
	op := &operation.InstallationOperation{
		ID:             uuidPrefix("opin_"),
		OperationType:  operation.TypeSettings,
		UserID:         req.DeviceCtx.UserID,
		DeviceID:       req.DeviceCtx.DeviceID,
		RuntimeID:      req.DeviceCtx.RuntimeID,
		InstallationID: req.InstallationID,
		IdempotencyKey: idempotencyKey,
		Status:         operation.OpStatusCreated,
		Stage:          operation.OpStageRequestValidated,
		CreatedAt:      time.Now().Format(operationTimeFormat),
		UpdatedAt:      time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, map[string]string{"expectedRevision": fmt.Sprintf("%d", req.ExpectedRevision), "updates": stableJSON(req.Updates)})
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &SettingsResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: operation.ErrCodeIDEMPOTENCYConflict, ErrorMessage: err.Error()}, err
	} else if existing != nil {
		return &SettingsResult{OperationID: existing.ID, DesiredRevision: existing.DesiredRevision, Status: existing.Status, ErrorCode: existing.ErrorCode, ErrorMessage: existing.ErrorMessage}, nil
	}
	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return &SettingsResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_CREATE_FAILED"}, err
	}

	settingsRev, desiredRev, err := c.repo.UpdateSettings(ctx, op, req.InstallationID, req.ExpectedRevision, req.Updates)
	if err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "SETTINGS_UPDATE_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &SettingsResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "SETTINGS_UPDATE_FAILED"}, persistErr
	}
	op.DesiredRevision = desiredRev
	op.Stage = operation.OpStageDatabaseCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &SettingsResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED", ErrorMessage: err.Error()}, err
	}

	if err := c.publishPersistedDesiredState(ctx, req.DeviceCtx, false); err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "PUBLISH_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &SettingsResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "PUBLISH_FAILED"}, persistErr
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &SettingsResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	return &SettingsResult{OperationID: op.ID, SettingsRevision: settingsRev, DesiredRevision: desiredRev, Status: operation.OpStatusWaitingRuntimeACK}, nil
}

func (c *Coordinator) executeDefaultAction(ctx context.Context, req DefaultActionRequest, idempotencyKey string) (*EnableDisableResult, error) {
	op := &operation.InstallationOperation{
		ID:             uuidPrefix("opin_"),
		OperationType:  operation.TypeDefaultAction,
		UserID:         req.DeviceCtx.UserID,
		DeviceID:       req.DeviceCtx.DeviceID,
		RuntimeID:      req.DeviceCtx.RuntimeID,
		InstallationID: req.InstallationID,
		IdempotencyKey: idempotencyKey,
		Status:         operation.OpStatusCreated,
		Stage:          operation.OpStageRequestValidated,
		CreatedAt:      time.Now().Format(operationTimeFormat),
		UpdatedAt:      time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, map[string]string{"actionKey": req.DesiredActionKey})
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: operation.ErrCodeIDEMPOTENCYConflict}, err
	} else if existing != nil {
		return &EnableDisableResult{OperationID: existing.ID, DesiredRevision: existing.DesiredRevision, Status: existing.Status, ErrorCode: existing.ErrorCode}, nil
	}
	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_CREATE_FAILED"}, err
	}

	desiredRev, err := c.repo.ChangeDefaultAction(ctx, op, req.InstallationID, req.DesiredActionKey)
	if err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "DEFAULT_ACTION_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "DEFAULT_ACTION_FAILED"}, persistErr
	}
	op.DesiredRevision = desiredRev
	op.Stage = operation.OpStageDesiredStateCommitted
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	if err := c.publishPersistedDesiredState(ctx, req.DeviceCtx, false); err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "PUBLISH_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "PUBLISH_FAILED"}, persistErr
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	return &EnableDisableResult{OperationID: op.ID, DesiredRevision: desiredRev, Status: operation.OpStatusWaitingRuntimeACK}, nil
}

func (c *Coordinator) executeRecenter(ctx context.Context, req RecenterRequest, idempotencyKey string) (*EnableDisableResult, error) {
	op := &operation.InstallationOperation{
		ID:             uuidPrefix("opin_"),
		OperationType:  operation.TypeRecenter,
		UserID:         req.DeviceCtx.UserID,
		DeviceID:       req.DeviceCtx.DeviceID,
		RuntimeID:      req.DeviceCtx.RuntimeID,
		InstallationID: req.InstallationID,
		IdempotencyKey: idempotencyKey,
		Status:         operation.OpStatusCreated,
		Stage:          operation.OpStageRequestValidated,
		CreatedAt:      time.Now().Format(operationTimeFormat),
		UpdatedAt:      time.Now().Format(operationTimeFormat),
	}
	op.RequestHash = operationRequestHash(op, nil)
	if existing, err := c.resolveIdempotentOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: operation.ErrCodeIDEMPOTENCYConflict}, err
	} else if existing != nil {
		return &EnableDisableResult{OperationID: existing.ID, DesiredRevision: existing.DesiredRevision, Status: existing.Status, ErrorCode: existing.ErrorCode}, nil
	}
	if err := c.repo.CreateOperation(ctx, op); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_CREATE_FAILED"}, err
	}

	if err := c.runtimePublisher.PublishRecenter(ctx, req.DeviceCtx, req.InstallationID); err != nil {
		op.Status = operation.OpStatusFailedTerminal
		op.ErrorCode = "PUBLISH_FAILED"
		op.ErrorMessage = err.Error()
		persistErr := c.persistOperation(ctx, op, err)
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "PUBLISH_FAILED"}, persistErr
	}

	op.Status = operation.OpStatusWaitingRuntimeACK
	op.Stage = operation.OpStageWaitingRuntimeACK
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if err := c.persistOperation(ctx, op, nil); err != nil {
		return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "OPERATION_UPDATE_FAILED"}, err
	}

	return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusWaitingRuntimeACK}, nil
}

func (c *Coordinator) failInstall(ctx context.Context, op *operation.InstallationOperation, stage string, cause error) (*InstallResult, error) {
	if op == nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorMessage: cause.Error()}, cause
	}
	op.ErrorMessage = cause.Error()
	op.ErrorCode = "INSTALL_FAILED"
	op.Status = operation.OpStatusFailedTerminal
	op.Stage = stage
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	if existing, err := c.repo.GetOperation(ctx, op.UserID, op.DeviceID, op.ID); err == nil && existing != nil {
		if updateErr := c.repo.UpdateOperation(ctx, op); updateErr != nil {
			cause = errors.Join(cause, updateErr)
		}
	}
	return &InstallResult{
		OperationID:  op.ID,
		Status:       operation.OpStatusFailedTerminal,
		Stage:        stage,
		ErrorCode:    op.ErrorCode,
		ErrorMessage: cause.Error(),
	}, cause
}

func (c *Coordinator) publishPersistedDesiredState(ctx context.Context, deviceCtx device.DeviceContext, ensureAbsent bool) error {
	snapshot, err := c.repo.GetDesiredStateSnapshot(ctx, deviceCtx.UserID, deviceCtx.DeviceID)
	if err != nil {
		return fmt.Errorf("load persisted desired state: %w", err)
	}
	snapshot.EnsureAbsent = ensureAbsent
	if snapshot.RuntimeID == "" {
		snapshot.RuntimeID = deviceCtx.RuntimeID
	}
	// The desired-state row and its outbox event are committed in the same
	// transaction. Immediate dispatch is only a latency optimization: if it
	// fails, the durable outbox worker will retry without turning a committed
	// desired state into a terminal operation failure.
	if err := c.runtimePublisher.PublishDesiredState(ctx, deviceCtx, snapshot); err != nil {
		// The same desired snapshot is committed to the durable outbox in the
		// repository transaction. Immediate dispatch is a latency optimization,
		// so failure is observable but does not invalidate the committed intent.
		log.Warn("installation coordinator: immediate desired-state publish failed; durable outbox will retry: ", err)
	}
	return nil
}

func (c *Coordinator) persistOperation(ctx context.Context, op *operation.InstallationOperation, cause error) error {
	updateErr := c.repo.UpdateOperation(ctx, op)
	if updateErr == nil {
		return cause
	}
	if cause != nil {
		return errors.Join(cause, updateErr)
	}
	return updateErr
}

func generateExecutionID() string {
	return "exec-" + uuid.New().String()[:8]
}

func uuidPrefix(prefix string) string {
	return prefix + uuid.New().String()
}

const operationTimeFormat = "2006-01-02 15:04:05"
