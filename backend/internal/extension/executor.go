// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
	applog "github.com/u-ai/backend/log"
)

type SkillExecutor interface {
	Execute(context.Context, ExecuteSkillRequest) (SkillResult, error)
}

type Executor struct {
	registry      SkillRegistry
	validator     *SchemaValidator
	permissions   RuntimePermissionEvaluator
	repository    *Repository
	idempotencyMu sync.Mutex
	idempotency   map[string]SkillResult
	inFlight      map[string]bool
	handlerSlots  chan struct{}
}

func NewExecutor(registry SkillRegistry, validator *SchemaValidator, permissions RuntimePermissionEvaluator, repository *Repository) *Executor {
	return &Executor{registry: registry, validator: validator, permissions: permissions, repository: repository, idempotency: map[string]SkillResult{}, inFlight: map[string]bool{}, handlerSlots: make(chan struct{}, 64)}
}

func (e *Executor) Execute(ctx context.Context, request ExecuteSkillRequest) (result SkillResult, returnedErr error) {
	started := time.Now()
	registered, err := e.registry.GetScoped(ctx, request.SkillID, request.Scope)
	if err != nil {
		return SkillResult{}, err
	}
	definition := registered.Definition
	if definition.Entry.Kind == "instructions" {
		return SkillResult{}, NewExtensionError(ErrSkillNotExecutable, "Skill is not executable", definition.ID, false, nil)
	}
	if ctx.Err() != nil {
		result := SkillResult{RunID: uuid.New().String(), Status: RunCancelled, Error: NewExtensionError(ErrSkillCancelled, "Skill was cancelled", "", false, ctx.Err())}
		return result, result.Error
	}
	if !definition.Enabled {
		return SkillResult{}, NewExtensionError(ErrSkillDisabled, "Skill is disabled", definition.ID, false, nil)
	}
	if !definition.Compatible {
		return SkillResult{}, NewExtensionError(ErrSkillIncompatible, "Skill is incompatible", definition.CompatibilityReason, false, nil)
	}
	if !hasTrigger(definition.Triggers, request.Scope.Trigger) {
		return SkillResult{}, NewExtensionError(ErrSkillTriggerNotAllowed, "Skill trigger is not allowed", string(request.Scope.Trigger), false, nil)
	}
	request.Config = normalizeJSON(definition.DefaultConfig)
	if e.repository != nil {
		request.Config, err = e.repository.GetEffectiveConfig(ctx, definition.ID, request.Scope, definition.DefaultConfig)
		if err != nil {
			return SkillResult{}, fmt.Errorf("load skill config: %w", err)
		}
	}
	request.Input = normalizeJSON(request.Input)
	if err := e.validator.Validate(definition.ID+"-input", definition.InputSchema, request.Input); err != nil {
		return SkillResult{}, NewExtensionError(ErrSkillInputInvalid, "Skill input is invalid", err.Error(), false, err)
	}
	identity := ExtensionIdentity{ExtensionID: definition.ID, SkillID: definition.ID, Version: definition.Version}
	for _, capability := range definition.Capabilities {
		decision := e.permissions.EvaluateExecution(ctx, identity, capability, request.Scope)
		if decision == DecisionDeny {
			run := e.deniedResult(ctx, definition, request, capability, started)
			return run, run.Error
		}
	}
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = e.defaultIdempotencyKey(definition, request)
	}
	cacheKey := scopedIdempotencyKey(definition.ID, request.Scope, request.IdempotencyKey)
	if definition.Idempotent {
		e.idempotencyMu.Lock()
		if cached, ok := e.idempotency[cacheKey]; ok {
			e.idempotencyMu.Unlock()
			if cached.Error != nil {
				return cloneSkillResult(cached), cached.Error
			}
			return cloneSkillResult(cached), nil
		}
		if e.inFlight[cacheKey] {
			e.idempotencyMu.Unlock()
			return SkillResult{}, NewExtensionError(ErrSkillIdempotencyConflict, "Skill request is already running", request.IdempotencyKey, true, nil)
		}
		if e.repository != nil {
			existing, findErr := e.repository.FindIdempotentRun(ctx, definition.ID, request.Scope.CharacterID, request.Scope.ConversationID, request.IdempotencyKey)
			if findErr != nil {
				e.idempotencyMu.Unlock()
				return SkillResult{}, findErr
			}
			if existing != nil && existing.Status != RunPending && existing.Status != RunRunning {
				stored := runResultFromView(*existing)
				e.idempotency[cacheKey] = stored
				if stored.Error != nil {
					e.idempotencyMu.Unlock()
					return cloneSkillResult(stored), stored.Error
				}
				e.idempotencyMu.Unlock()
				return cloneSkillResult(stored), nil
			}
			if existing != nil {
				e.idempotencyMu.Unlock()
				return SkillResult{}, NewExtensionError(ErrSkillIdempotencyConflict, "Skill request is already running", request.IdempotencyKey, true, nil)
			}
		}
		e.inFlight[cacheKey] = true
		e.idempotencyMu.Unlock()
		defer func() {
			e.idempotencyMu.Lock()
			delete(e.inFlight, cacheKey)
			if result.Status != RunRunning && result.Status != RunPending {
				e.idempotency[cacheKey] = cloneSkillResult(result)
			}
			e.idempotencyMu.Unlock()
		}()
	}
	runID := uuid.New().String()
	request.Scope.ExtensionID = definition.ID
	request.Scope.ExtensionVersion = definition.Version
	request.Scope.RunID = runID
	result = SkillResult{RunID: runID, Status: RunPending}
	run := RunView{
		RunID: runID, ExtensionID: definition.ID, ExtensionVersion: definition.Version, SkillID: definition.ID,
		UserID: request.Scope.UserID, CharacterID: request.Scope.CharacterID, ConversationID: request.Scope.ConversationID,
		Channel: request.Scope.Channel, Trigger: request.Scope.Trigger, Status: RunPending, InputSummary: compactSensitiveJSON(request.Input),
		OutputSummary: "{}", IdempotencyKey: request.IdempotencyKey, StartedAt: started.UTC().Format(time.RFC3339Nano), TraceID: request.Scope.TraceID,
	}
	if e.repository != nil {
		if err := e.repository.CreateRun(ctx, run); err != nil {
			return SkillResult{}, fmt.Errorf("create skill run: %w", err)
		}
		if err := e.repository.SetRunStatus(ctx, runID, RunRunning); err != nil {
			return SkillResult{}, fmt.Errorf("mark skill run running: %w", err)
		}
	}
	result.Status = RunRunning
	timeout := definition.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			applog.Error("skill panic recovered skill=", definition.ID, " run=", runID, " stack=", string(debug.Stack()))
			result = SkillResult{RunID: runID, Status: RunFailed, Error: NewExtensionError(ErrSkillExecutionFailed, "Skill execution failed", "skill handler panicked", false, nil)}
			returnedErr = result.Error
		}
		result.Duration = time.Since(started)
		result.DurationMS = result.Duration.Milliseconds()
		if result.RunID == "" {
			result.RunID = runID
		}
		if e.repository != nil {
			persistCtx, persistCancel := context.WithTimeout(context.Background(), 3*time.Second)
			resourceErr := e.repository.RegisterOwnedSideEffects(persistCtx, request.Scope, result.SideEffects)
			if resourceErr != nil {
				e.repository.CompensateUnownedSideEffects(persistCtx, request.Scope, result.SideEffects)
				applog.Error("skill owned resource persist failed skill=", definition.ID, " run=", runID, " error=", resourceErr)
				if returnedErr == nil {
					result.Status = RunPartiallySucceeded
					result.Error = NewExtensionError(ErrSkillExecutionFailed, "Skill owned resource persistence failed", resourceErr.Error(), true, resourceErr)
					returnedErr = result.Error
				}
			}
			persistErr := e.repository.UpdateRun(persistCtx, result, compactSensitiveJSON(result.Output))
			persistCancel()
			if persistErr != nil {
				applog.Error("skill run persist failed skill=", definition.ID, " run=", runID, " error=", persistErr)
				if definition.HasSideEffects && returnedErr == nil {
					result.Status = RunPartiallySucceeded
					result.Error = NewExtensionError(ErrSkillExecutionFailed, "Skill audit persistence failed", persistErr.Error(), true, persistErr)
					returnedErr = result.Error
				}
			}
		}
	}()
	result, err = e.executeHandler(execCtx, registered.Handler, request, definition)
	result.RunID = runID
	if err != nil {
		result.Error = asExtensionError(err)
		result.Status = RunFailed
		if execCtx.Err() == context.DeadlineExceeded {
			result.Status = RunTimedOut
			result.Error = NewExtensionError(ErrSkillTimeout, "Skill timed out", timeout.String(), true, execCtx.Err())
		} else if execCtx.Err() == context.Canceled || ctx.Err() == context.Canceled {
			result.Status = RunCancelled
			result.Error = NewExtensionError(ErrSkillCancelled, "Skill was cancelled", "", false, execCtx.Err())
		}
		return result, result.Error
	}
	if execCtx.Err() == context.DeadlineExceeded {
		result.Status = RunTimedOut
		result.Error = NewExtensionError(ErrSkillTimeout, "Skill timed out", timeout.String(), true, execCtx.Err())
		return result, result.Error
	}
	if execCtx.Err() == context.Canceled || ctx.Err() == context.Canceled {
		result.Status = RunCancelled
		result.Error = NewExtensionError(ErrSkillCancelled, "Skill was cancelled", "", false, execCtx.Err())
		return result, result.Error
	}
	if result.Status == "" || result.Status == RunRunning || result.Status == RunPending {
		result.Status = RunSucceeded
	}
	if result.Error == nil && result.Status == RunSucceeded {
		if err := e.validator.Validate(definition.ID+"-output", definition.OutputSchema, result.Output); err != nil {
			result.Status = RunFailed
			result.Output = nil
			result.VisibleText = ""
			result.Error = NewExtensionError(ErrSkillOutputInvalid, "Skill output is invalid", err.Error(), false, err)
			return result, result.Error
		}
	}
	if result.Error != nil {
		return result, result.Error
	}
	return result, nil
}

func (e *Executor) executeHandler(ctx context.Context, handler SkillHandler, request ExecuteSkillRequest, definition SkillDefinition) (SkillResult, error) {
	result, err := e.callHandler(ctx, handler, request, definition.ID)
	if err == nil || !definition.Retryable || definition.HasSideEffects {
		return result, err
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return e.callHandler(ctx, handler, request, definition.ID)
}

type handlerResponse struct {
	result SkillResult
	err    error
}

func (e *Executor) callHandler(ctx context.Context, handler SkillHandler, request ExecuteSkillRequest, skillID string) (SkillResult, error) {
	select {
	case e.handlerSlots <- struct{}{}:
	case <-ctx.Done():
		return SkillResult{}, ctx.Err()
	}
	response := make(chan handlerResponse, 1)
	go func() {
		defer func() { <-e.handlerSlots }()
		defer func() {
			if recovered := recover(); recovered != nil {
				applog.Error("skill panic recovered skill=", skillID, " stack=", string(debug.Stack()))
				response <- handlerResponse{err: NewExtensionError(ErrSkillExecutionFailed, "Skill execution failed", "skill handler panicked", false, nil)}
			}
		}()
		result, err := handler(ctx, request)
		response <- handlerResponse{result: result, err: err}
	}()
	select {
	case item := <-response:
		return item.result, item.err
	case <-ctx.Done():
		return SkillResult{}, ctx.Err()
	}
}

func (e *Executor) deniedResult(ctx context.Context, definition SkillDefinition, request ExecuteSkillRequest, capability string, started time.Time) SkillResult {
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = uuid.New().String()
	}
	result := SkillResult{RunID: uuid.New().String(), Status: RunFailed, Error: NewExtensionError(ErrSkillPermissionDenied, "Skill permission denied", "The skill has not been granted "+capability, false, nil)}
	result.Duration = time.Since(started)
	result.DurationMS = result.Duration.Milliseconds()
	if e.repository != nil {
		run := RunView{RunID: result.RunID, ExtensionID: definition.ID, ExtensionVersion: definition.Version, SkillID: definition.ID, UserID: request.Scope.UserID, CharacterID: request.Scope.CharacterID, ConversationID: request.Scope.ConversationID, Channel: request.Scope.Channel, Trigger: request.Scope.Trigger, Status: result.Status, InputSummary: compactSensitiveJSON(request.Input), OutputSummary: "{}", StartedAt: started.UTC().Format(time.RFC3339Nano), FinishedAt: time.Now().UTC().Format(time.RFC3339Nano), DurationMS: result.DurationMS, ErrorCode: result.Error.Code, ErrorDetail: result.Error.Detail, TraceID: request.Scope.TraceID, IdempotencyKey: request.IdempotencyKey}
		if err := e.repository.CreateRun(ctx, run); err != nil {
			applog.Error("skill permission audit persist failed skill=", definition.ID, " capability=", capability, " error=", err)
		}
	}
	return result
}

func (e *Executor) defaultIdempotencyKey(definition SkillDefinition, request ExecuteSkillRequest) string {
	if !definition.Idempotent {
		return uuid.New().String()
	}
	sum := sha256.Sum256([]byte(definition.ID + "|" + request.Scope.UserID + "|" + request.Scope.CharacterID + "|" + request.Scope.ConversationID + "|" + request.Scope.RequestID + "|" + request.Scope.ToolCallID + "|" + string(request.Input)))
	return hex.EncodeToString(sum[:])
}

func scopedIdempotencyKey(skillID string, scope ExecutionScope, key string) string {
	return skillID + "|" + scope.UserID + "|" + scope.CharacterID + "|" + scope.ConversationID + "|" + key
}

func cloneSkillResult(result SkillResult) SkillResult {
	copyResult := result
	copyResult.Output = append(json.RawMessage(nil), result.Output...)
	copyResult.SideEffects = append([]SideEffectRecord(nil), result.SideEffects...)
	if result.Error != nil {
		copyError := *result.Error
		copyResult.Error = &copyError
	}
	return copyResult
}
