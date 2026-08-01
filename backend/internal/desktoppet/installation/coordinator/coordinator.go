package coordinator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

type ReleaseValidator interface {
	ValidateRelease(ctx context.Context, releaseID string) (*ReleaseValidationResult, error)
}

type ReleaseStager interface {
	PrepareStagingCopy(ctx context.Context, releaseID, installationID string) (stagingPathKey string, err error)
	VerifyStagingCopy(ctx context.Context, releaseID, installationID, stagingPathKey string) error
}

type RuntimeDesiredStatePublisher interface {
	PublishDesiredState(ctx context.Context, deviceCtx DeviceContext, snapshot *DesiredStateSnapshot) error
}

type DesiredStateSnapshot struct {
	DesiredRevision int64
	DesiredHash    string
	InstallationID string
	PetID          string
	ReleaseID      string
	UserID         string
	DeviceID       string
	RuntimeID      string
	EnsureAbsent   bool
}

type DeviceContext struct {
	UserID    string
	DeviceID  string
	RuntimeID string
}

func (d DeviceContext) IsValid() bool {
	return d.UserID != "" && d.DeviceID != ""
}

type Repository interface {
	CreateOperation(ctx context.Context, op *operation.InstallationOperation) error
}

type ProjectionService interface {
	UpdateProjection(ctx context.Context, userID, deviceID string, updateFn func(*Projection) error) error
	HandleRuntimeHeartbeat(ctx context.Context, userID, deviceID, runtimeID string, heartbeat *RuntimeHeartbeat) error
	HandleCommandResult(ctx context.Context, userID, deviceID string, result *CommandResult) error
}

type RuntimeHeartbeat struct {
	AppliedDesiredRevision  int64
	AppliedSettingsRevision int64
	ActualReleaseID        string
	ActualVisible          int
	ActualActionKey        string
	ActualHealth           string
	Timestamp              string
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
	ActualReleaseID       string
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
) *Coordinator {
	return &Coordinator{
		repo:             repo,
		releaseValidator: validator,
		releaseStager:    stager,
		runtimePublisher: publisher,
		projectionSvc:    projection,
		executionID:      generateExecutionID(),
		defaultLeaseTTL:   5 * time.Minute,
		maxRetryAttempts:  3,
	}
}

func (c *Coordinator) Install(ctx context.Context, req InstallRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED", ErrorMessage: err.Error()}, err
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = c.buildIdempotencyKey(operation.TypeInstall, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.TargetReleaseID)
	}
	return c.executeInstall(ctx, req, idempotencyKey)
}

func (c *Coordinator) Enable(ctx context.Context, req EnableDisableRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := c.buildIdempotencyKey(operation.TypeEnable, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID)
	return c.executeEnableDisable(ctx, req, idempotencyKey, true)
}

func (c *Coordinator) Disable(ctx context.Context, req EnableDisableRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := c.buildIdempotencyKey(operation.TypeDisable, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID)
	return c.executeEnableDisable(ctx, req, idempotencyKey, false)
}

func (c *Coordinator) Switch(ctx context.Context, req SwitchRequest) (*SwitchResult, error) {
	if err := req.Validate(); err != nil {
		return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := c.buildIdempotencyKey(operation.TypeSwitch, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.SourceInstallationID, req.TargetReleaseID)
	return c.executeSwitch(ctx, req, idempotencyKey)
}

func (c *Coordinator) Upgrade(ctx context.Context, req UpgradeRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := c.buildIdempotencyKey(operation.TypeUpgrade, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.TargetReleaseID)
	return c.executeInstall(ctx, InstallRequest{
		DeviceCtx:       req.DeviceCtx,
		TargetReleaseID: req.TargetReleaseID,
		IdempotencyKey:  idempotencyKey,
	}, idempotencyKey)
}

func (c *Coordinator) Downgrade(ctx context.Context, req DowngradeRequest) (*InstallResult, error) {
	if !req.SafetyConfirm {
		err := fmt.Errorf("%w: downgrade requires explicit safety confirmation", ErrInvalidDowngradeRequest)
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "SAFETY_CONFIRMATION_REQUIRED"}, err
	}
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := c.buildIdempotencyKey(operation.TypeDowngrade, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.TargetReleaseID)
	return c.executeInstall(ctx, InstallRequest{
		DeviceCtx:       req.DeviceCtx,
		TargetReleaseID: req.TargetReleaseID,
		SourceReleaseID: req.InstallationID,
		IdempotencyKey:  idempotencyKey,
	}, idempotencyKey)
}

func (c *Coordinator) Rollback(ctx context.Context, req UpgradeRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	idempotencyKey := c.buildIdempotencyKey(operation.TypeRepair, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.TargetReleaseID)
	return c.executeInstall(ctx, InstallRequest{
		DeviceCtx:       req.DeviceCtx,
		TargetReleaseID: req.TargetReleaseID,
		SourceReleaseID: req.InstallationID,
		IdempotencyKey:  idempotencyKey,
	}, idempotencyKey)
}

func (c *Coordinator) Repair(ctx context.Context, req RepairRequest) (*InstallResult, error) {
	if err := req.Validate(); err != nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeRepair,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		InstallationID:   req.InstallationID,
		IdempotencyKey:   c.buildIdempotencyKey(operation.TypeRepair, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID),
		Status:           operation.OpStatusRunning,
		Stage:            operation.OpStageRequestValidated,
		CreatedAt:        time.Now().Format(operationTimeFormat),
		UpdatedAt:        time.Now().Format(operationTimeFormat),
	}
	_ = op
	return &InstallResult{OperationID: op.ID, Status: operation.OpStatusRunning, Stage: operation.OpStageRequestValidated}, nil
}

func (c *Coordinator) Uninstall(ctx context.Context, req UninstallRequest) (*UninstallResult, error) {
	if err := req.Validate(); err != nil {
		return &UninstallResult{Status: operation.OpStatusFailedTerminal, ErrorMessage: err.Error()}, err
	}
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeUninstall,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		InstallationID:   req.InstallationID,
		IdempotencyKey:   c.buildIdempotencyKey(operation.TypeUninstall, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID),
		Status:           operation.OpStatusRunning,
		Stage:            operation.OpStageRequestValidated,
		CreatedAt:        time.Now().Format(operationTimeFormat),
		UpdatedAt:        time.Now().Format(operationTimeFormat),
	}
	_ = op
	return &UninstallResult{OperationID: op.ID, Status: operation.OpStatusRunning}, nil
}

func (c *Coordinator) UpdateSettings(ctx context.Context, req SettingsRequest) (*SettingsResult, error) {
	if err := req.Validate(); err != nil {
		return &SettingsResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeSettings,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		InstallationID:   req.InstallationID,
		IdempotencyKey:   c.buildIdempotencyKey(operation.TypeSettings, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID),
		Status:           operation.OpStatusCompleted,
		Stage:            operation.OpStageCompleted,
		CreatedAt:        time.Now().Format(operationTimeFormat),
		UpdatedAt:        time.Now().Format(operationTimeFormat),
		CompletedAt:      time.Now().Format(operationTimeFormat),
	}
	_ = op
	return &SettingsResult{
		OperationID:      op.ID,
		SettingsRevision: req.ExpectedRevision + 1,
		Status:           operation.OpStatusCompleted,
		DesiredRevision:  0,
	}, nil
}

func (c *Coordinator) ChangeDefaultAction(ctx context.Context, req DefaultActionRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeDefaultAction,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		InstallationID:   req.InstallationID,
		IdempotencyKey:   c.buildIdempotencyKey(operation.TypeDefaultAction, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID, req.DesiredActionKey),
		Status:           operation.OpStatusCompleted,
		Stage:            operation.OpStageCompleted,
		CreatedAt:        time.Now().Format(operationTimeFormat),
		UpdatedAt:        time.Now().Format(operationTimeFormat),
		CompletedAt:      time.Now().Format(operationTimeFormat),
	}
	_ = op
	return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusCompleted}, nil
}

func (c *Coordinator) Recenter(ctx context.Context, req RecenterRequest) (*EnableDisableResult, error) {
	if err := req.Validate(); err != nil {
		return &EnableDisableResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "VALIDATION_FAILED"}, err
	}
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeRecenter,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		InstallationID:   req.InstallationID,
		IdempotencyKey:   c.buildIdempotencyKey(operation.TypeRecenter, req.DeviceCtx.UserID, req.DeviceCtx.DeviceID, req.InstallationID),
		Status:           operation.OpStatusCompleted,
		Stage:            operation.OpStageCompleted,
		CreatedAt:        time.Now().Format(operationTimeFormat),
		UpdatedAt:        time.Now().Format(operationTimeFormat),
		CompletedAt:      time.Now().Format(operationTimeFormat),
	}
	_ = op
	return &EnableDisableResult{OperationID: op.ID, Status: operation.OpStatusCompleted}, nil
}

func (c *Coordinator) GetOperationStatus(ctx context.Context, userID, deviceID, operationID string) (*operation.InstallationOperation, error) {
	if operationID == "" {
		return nil, errors.New("coordinator: operationID required")
	}
	return nil, errors.New("coordinator: not found")
}

func (c *Coordinator) CancelOperation(ctx context.Context, userID, deviceID, operationID string) error {
	if operationID == "" {
		return errors.New("coordinator: operationID required")
	}
	return errors.New("coordinator: cancel not supported")
}

func (c *Coordinator) buildIdempotencyKey(opType string, parts ...string) string {
	allParts := append([]string{opType}, parts...)
	return operation.ComputeIdempotencyKey(allParts...)
}

func (c *Coordinator) executeInstall(ctx context.Context, req InstallRequest, idempotencyKey string) (*InstallResult, error) {
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   operation.TypeInstall,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		PetID:            req.PetID,
		TargetReleaseID:  req.TargetReleaseID,
		SourceReleaseID:  req.SourceReleaseID,
		IdempotencyKey:   idempotencyKey,
		Status:           operation.OpStatusCreated,
		Stage:            operation.OpStageRequestValidated,
		CreatedAt:        time.Now().Format(operationTimeFormat),
		UpdatedAt:        time.Now().Format(operationTimeFormat),
	}
	if c.releaseValidator == nil {
		return &InstallResult{OperationID: op.ID, Status: operation.OpStatusFailedTerminal, ErrorCode: "NOT_CONFIGURED", ErrorMessage: "releaseValidator not configured"}, errors.New("releaseValidator not configured")
	}
	validationResult, err := c.releaseValidator.ValidateRelease(ctx, req.TargetReleaseID)
	if err != nil {
		return c.failInstall(op, operation.OpStageReleaseVerified, err)
	}
	if !validationResult.IsInstallable {
		err := fmt.Errorf("%w: release %s is not installable", ErrReleaseNotInstallable, req.TargetReleaseID)
		return c.failInstall(op, operation.OpStageReleaseVerified, err)
	}
	op.Stage = operation.OpStageReleaseVerified
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	return &InstallResult{
		OperationID: op.ID,
		Status:      operation.OpStatusRunning,
		Stage:       operation.OpStageReleaseVerified,
	}, nil
}

func (c *Coordinator) executeEnableDisable(ctx context.Context, req EnableDisableRequest, idempotencyKey string, enabled bool) (*EnableDisableResult, error) {
	opType := operation.TypeEnable
	if !enabled {
		opType = operation.TypeDisable
	}
	op := &operation.InstallationOperation{
		ID:              uuidPrefix("opin_"),
		OperationType:   opType,
		UserID:           req.DeviceCtx.UserID,
		DeviceID:         req.DeviceCtx.DeviceID,
		RuntimeID:        req.DeviceCtx.RuntimeID,
		InstallationID:   req.InstallationID,
		IdempotencyKey:   idempotencyKey,
		Status:           operation.OpStatusCompleted,
		Stage:            operation.OpStageCompleted,
		CreatedAt:        time.Now().Format(operationTimeFormat),
		UpdatedAt:        time.Now().Format(operationTimeFormat),
		CompletedAt:      time.Now().Format(operationTimeFormat),
	}
	_ = op
	return &EnableDisableResult{Status: operation.OpStatusCompleted}, nil
}

func (c *Coordinator) executeSwitch(ctx context.Context, req SwitchRequest, idempotencyKey string) (*SwitchResult, error) {
	return &SwitchResult{Status: operation.OpStatusFailedTerminal, ErrorCode: "NOT_IMPLEMENTED"}, nil
}

func (c *Coordinator) failInstall(op *operation.InstallationOperation, stage string, cause error) (*InstallResult, error) {
	if op == nil {
		return &InstallResult{Status: operation.OpStatusFailedTerminal, ErrorMessage: cause.Error()}, cause
	}
	op.ErrorMessage = cause.Error()
	op.ErrorCode = "INSTALL_FAILED"
	op.Status = operation.OpStatusFailedTerminal
	op.Stage = stage
	op.UpdatedAt = time.Now().Format(operationTimeFormat)
	return &InstallResult{
		OperationID:   op.ID,
		Status:        operation.OpStatusFailedTerminal,
		Stage:         stage,
		ErrorCode:     op.ErrorCode,
		ErrorMessage:  cause.Error(),
	}, cause
}

func generateExecutionID() string {
	return "exec-" + uuid.New().String()[:8]
}

func uuidPrefix(prefix string) string {
	return prefix + uuid.New().String()
}

const operationTimeFormat = "2006-01-02 15:04:05"
