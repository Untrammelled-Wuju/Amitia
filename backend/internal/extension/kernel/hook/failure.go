package hook

import (
	"context"
	"fmt"
)

type FailureAction string

const (
	FailureActionAbort      FailureAction = "abort"
	FailureActionContinue   FailureAction = "continue"
	FailureActionSkip       FailureAction = "skip"
	FailureActionDisable    FailureAction = "disable_contribution"
	FailureActionOpenCircuit FailureAction = "open_circuit"
)

type FailureContext struct {
	Err          error
	ErrCode      HookErrorCode
	Policy       HookFailurePolicy
	Point        HookPointDefinition
	Contrib      HookContributionDefinition
	Phase        HookPhase
	IsFilter     bool
}

func ResolveFailureAction(fctx FailureContext) FailureAction {
	policy := fctx.Policy
	if fctx.Contrib.FailurePolicy != nil {
		policy = *fctx.Contrib.FailurePolicy
	}

	var rawPolicy string
	switch {
	case fctx.ErrCode == ErrCodeHookTimeout:
		rawPolicy = policy.OnTimeout
	case fctx.ErrCode == ErrCodeHookRuntimeError:
		rawPolicy = policy.OnRuntimeError
	case fctx.ErrCode == ErrCodeHookResultInvalid:
		rawPolicy = policy.OnInvalidResult
	case fctx.ErrCode == ErrCodeRuntimeNotReady || fctx.ErrCode == ErrCodeDependencyUnavailable:
		rawPolicy = policy.OnUnavailable
	case fctx.ErrCode == ErrCodePermissionDenied || fctx.ErrCode == ErrCodeScopeDenied:
		rawPolicy = policy.OnPermissionDenied
	case fctx.ErrCode == ErrCodeCircuitOpen:
		return FailureActionSkip
	default:
		rawPolicy = policy.OnRuntimeError
	}

	if fctx.IsFilter && rawPolicy == FailureFailOpen {
		return FailureActionContinue
	}

	switch rawPolicy {
	case FailureFailOpen:
		return FailureActionContinue
	case FailureFailClosed:
		return FailureActionAbort
	case FailureSkip:
		return FailureActionSkip
	case FailureDisableContribution:
		return FailureActionDisable
	case FailureOpenCircuit:
		return FailureActionOpenCircuit
	default:
		return FailureActionContinue
	}
}

type FailureOutcome struct {
	Action       FailureAction
	AbortPipeline bool
	DisableContrib bool
	RecordCircuit  bool
	SkipRemaining bool
	Message       string
}

func ProcessFailure(fctx FailureContext) FailureOutcome {
	action := ResolveFailureAction(fctx)
	outcome := FailureOutcome{
		Action: action,
		Message: fmt.Sprintf("hook %s failed: %s (code: %s)", fctx.Contrib.ContributionID, fctx.Err.Error(), fctx.ErrCode),
	}

	switch action {
	case FailureActionAbort:
		outcome.AbortPipeline = true
	case FailureActionContinue:
	case FailureActionSkip:
		outcome.SkipRemaining = true
	case FailureActionDisable:
		outcome.DisableContrib = true
		outcome.SkipRemaining = true
	case FailureActionOpenCircuit:
		outcome.RecordCircuit = true
		outcome.SkipRemaining = true
	}

	return outcome
}

func IsRecoverableError(code HookErrorCode) bool {
	switch code {
	case ErrCodeHookTimeout, ErrCodeHookRuntimeError, ErrCodeRuntimeNotReady:
		return true
	default:
		return false
	}
}

func ShouldCountCircuitFailure(code HookErrorCode) bool {
	switch code {
	case ErrCodeHookTimeout,
		ErrCodeHookRuntimeError,
		ErrCodeHookResultInvalid,
		ErrCodePermissionDenied,
		ErrCodeCircuitOpen:
		return true
	default:
		return false
	}
}

type CancelContext struct {
	Ctx        context.Context
	CancelFunc context.CancelFunc
}

func NewCancelContext(parent context.Context) *CancelContext {
	ctx, cancel := context.WithCancel(parent)
	return &CancelContext{Ctx: ctx, CancelFunc: cancel}
}

func (c *CancelContext) Cancel() {
	if c.CancelFunc != nil {
		c.CancelFunc()
	}
}
