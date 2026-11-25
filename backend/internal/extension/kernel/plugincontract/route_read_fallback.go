package plugincontract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/u-ai/backend/internal/execution"
)

var (
	ErrReadFallbackInvalid          = errors.New("plugincontract: read fallback input invalid")
	ErrReadFallbackMutating         = errors.New("plugincontract: read fallback rejects mutating invocations")
	ErrReadFallbackPermissionDenied = errors.New("plugincontract: read fallback permission denied")
	ErrReadFallbackExhausted        = errors.New("plugincontract: read fallback candidates exhausted")
)

type ReadFallbackInvalidError struct {
	Field  string
	Value  string
	Reason string
}

func (e *ReadFallbackInvalidError) Error() string {
	return fmt.Sprintf("plugincontract: read fallback field %q value %q: %s", e.Field, e.Value, e.Reason)
}

func (e *ReadFallbackInvalidError) Unwrap() error { return ErrReadFallbackInvalid }

type ReadFallbackRequest struct {
	Contract   execution.InvocationContract
	Permission string
	Scope      ScopeRequirement
}

type ReadFallbackCandidate struct {
	ProviderID         string
	Core               bool
	GrantedPermissions []string
	Claims             ScopeClaims
	Invoke             func(context.Context) (json.RawMessage, error)
}

type ReadFallbackOptions struct {
	PermissionDenied func(error) bool
	Available        func(providerID string) bool
}

type ReadFallbackAttempt struct {
	ProviderID string       `json:"providerId"`
	Core       bool         `json:"core"`
	Executed   bool         `json:"executed"`
	Outcome    RouteOutcome `json:"outcome"`
	Reason     string       `json:"reason"`
	Err        error        `json:"-"`
}

type ReadFallbackResult struct {
	Output     json.RawMessage       `json:"output,omitempty"`
	ProviderID string                `json:"providerId"`
	Core       bool                  `json:"core"`
	Fallback   bool                  `json:"fallback"`
	Attempts   []ReadFallbackAttempt `json:"attempts"`
}

func (r ReadFallbackResult) Failure() error {
	for _, attempt := range r.Attempts {
		if attempt.Err != nil {
			return attempt.Err
		}
	}
	return nil
}

type ReadFallbackPanicError struct {
	CapabilityID string
	ProviderID   string
	Value        string
	Stack        string
}

func (e *ReadFallbackPanicError) Error() string {
	return fmt.Sprintf(
		"plugincontract: read provider %q panicked while serving capability %q: %s\n%s",
		e.ProviderID, e.CapabilityID, e.Value, e.Stack,
	)
}

type ReadFallbackExhaustedError struct {
	CapabilityID string
	Attempts     []ReadFallbackAttempt
	causes       []error
}

func (e *ReadFallbackExhaustedError) Error() string {
	reasons := make([]string, 0, len(e.Attempts))
	for _, attempt := range e.Attempts {
		reasons = append(reasons, fmt.Sprintf("%s=%s", attempt.ProviderID, attempt.Reason))
	}
	causes := make([]string, 0, len(e.causes))
	for _, cause := range e.causes {
		causes = append(causes, cause.Error())
	}
	return fmt.Sprintf(
		"plugincontract: read fallback exhausted for capability %q after %d candidates (%s) causes=[%s]",
		e.CapabilityID, len(e.Attempts), strings.Join(reasons, "; "), strings.Join(causes, " | "),
	)
}

func (e *ReadFallbackExhaustedError) Unwrap() []error {
	causes := make([]error, 0, len(e.causes)+1)
	causes = append(causes, ErrReadFallbackExhausted)
	causes = append(causes, e.causes...)
	return causes
}

func InvokeReadWithFallback(
	ctx context.Context,
	request ReadFallbackRequest,
	candidates []ReadFallbackCandidate,
	options ReadFallbackOptions,
) (ReadFallbackResult, error) {
	if err := validateReadFallbackRequest(request); err != nil {
		return ReadFallbackResult{}, err
	}
	if len(candidates) == 0 {
		return ReadFallbackResult{}, &ReadFallbackInvalidError{Field: "candidates", Value: "0", Reason: "at least one candidate is required"}
	}
	for index, candidate := range candidates {
		if strings.TrimSpace(candidate.ProviderID) == "" {
			return ReadFallbackResult{}, &ReadFallbackInvalidError{Field: fmt.Sprintf("candidates[%d].providerId", index), Value: "", Reason: "must not be empty"}
		}
		if candidate.Invoke == nil {
			return ReadFallbackResult{}, &ReadFallbackInvalidError{Field: fmt.Sprintf("candidates[%d].invoke", index), Value: candidate.ProviderID, Reason: "must not be nil"}
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	denied := options.PermissionDenied
	if denied == nil {
		denied = func(err error) bool { return errors.Is(err, ErrReadFallbackPermissionDenied) }
	}
	result := ReadFallbackResult{Attempts: make([]ReadFallbackAttempt, 0, len(candidates))}
	executed := 0
	exclusions := make([]error, 0, len(candidates))
	for _, candidate := range candidates {
		attempt := ReadFallbackAttempt{ProviderID: candidate.ProviderID, Core: candidate.Core}
		if reason, excluded := readCandidateExclusion(request, candidate, options); excluded {
			attempt.Outcome = RouteOutcomeUnavailable
			attempt.Reason = reason
			result.Attempts = append(result.Attempts, attempt)
			exclusions = append(exclusions, readExclusionCause(reason))
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("plugincontract: read fallback aborted for capability %q before provider %q: %w", request.Contract.CapabilityID, candidate.ProviderID, err)
		}
		executed++
		attempt.Executed = true
		output, err := invokeReadCandidate(ctx, candidate, request.Contract.CapabilityID)
		if err == nil && len(output) == 0 {
			err = &ReadFallbackInvalidError{Field: "output", Value: "", Reason: "provider returned an empty result"}
		}
		if err == nil {
			attempt.Outcome = RouteOutcomeSuccess
			attempt.Reason = "provider returned a result"
			result.Attempts = append(result.Attempts, attempt)
			result.Output = append(json.RawMessage(nil), output...)
			result.ProviderID = candidate.ProviderID
			result.Core = candidate.Core
			result.Fallback = executed > 1
			return result, nil
		}
		attempt.Err = err
		if denied(err) {
			attempt.Outcome = RouteOutcomeFailed
			attempt.Reason = "permission denied must never fall back"
			result.Attempts = append(result.Attempts, attempt)
			return result, fmt.Errorf(
				"plugincontract: read provider %q denied capability %q: %w",
				candidate.ProviderID, request.Contract.CapabilityID, errors.Join(ErrReadFallbackPermissionDenied, err),
			)
		}
		if errors.Is(err, execution.ErrInvocationOutcomeUnknown) {
			attempt.Outcome = RouteOutcomeUnknown
			attempt.Reason = "an unknown outcome must never be replayed"
			result.Attempts = append(result.Attempts, attempt)
			return result, fmt.Errorf(
				"plugincontract: read provider %q reported an unknown outcome for capability %q: %w",
				candidate.ProviderID, request.Contract.CapabilityID, err,
			)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			attempt.Outcome = RouteOutcomeFailed
			attempt.Reason = "caller cancelled the invocation"
			result.Attempts = append(result.Attempts, attempt)
			return result, fmt.Errorf(
				"plugincontract: read fallback aborted for capability %q after provider %q: %w",
				request.Contract.CapabilityID, candidate.ProviderID, errors.Join(err, ctxErr),
			)
		}
		attempt.Outcome = RouteOutcomeFailed
		attempt.Reason = "read provider failure allows a lower candidate"
		result.Attempts = append(result.Attempts, attempt)
	}
	causes := make([]error, 0, len(exclusions)+len(result.Attempts))
	causes = append(causes, exclusions...)
	for _, attempt := range result.Attempts {
		if attempt.Err != nil {
			causes = append(causes, attempt.Err)
		}
	}
	return result, &ReadFallbackExhaustedError{
		CapabilityID: request.Contract.CapabilityID,
		Attempts:     result.Attempts,
		causes:       causes,
	}
}

func validateReadFallbackRequest(request ReadFallbackRequest) error {
	if err := request.Contract.Validate(); err != nil {
		return fmt.Errorf("plugincontract: read fallback contract: %w", err)
	}
	if request.Contract.SideEffect.Mutating() {
		return fmt.Errorf(
			"%w: capability %q sideEffect=%s",
			ErrReadFallbackMutating, request.Contract.CapabilityID, request.Contract.SideEffect,
		)
	}
	if permission := strings.TrimSpace(request.Permission); permission != request.Permission {
		return &ReadFallbackInvalidError{Field: "permission", Value: request.Permission, Reason: "must not carry surrounding whitespace"}
	}
	if err := request.Scope.validate(); err != nil {
		return fmt.Errorf("plugincontract: read fallback scope: %w", err)
	}
	return nil
}

func readCandidateExclusion(request ReadFallbackRequest, candidate ReadFallbackCandidate, options ReadFallbackOptions) (string, bool) {
	if permission := strings.TrimSpace(request.Permission); permission != "" {
		granted := false
		for _, item := range candidate.GrantedPermissions {
			if strings.TrimSpace(item) == permission {
				granted = true
				break
			}
		}
		if !granted {
			return "permission:required permission is not declared by the provider", true
		}
	}
	if len(request.Scope) > 0 || len(candidate.Claims) > 0 {
		contract := ProviderScopeContract{
			ProviderID: candidate.ProviderID,
			ServiceID:  request.Contract.CapabilityID,
			Claims:     candidate.Claims,
		}
		match, err := contract.Matches(request.Scope)
		if err != nil {
			return fmt.Sprintf("scope:contract invalid: %v", err), true
		}
		if !match.Matched {
			return fmt.Sprintf("scope:%s required=%q actual=%q", match.Reason, match.Required, match.Actual), true
		}
	}
	if options.Available != nil && !options.Available(candidate.ProviderID) {
		return "availability:provider health probe reported unavailable", true
	}
	return "", false
}

func readExclusionCause(reason string) error {
	switch {
	case strings.HasPrefix(reason, "permission:"):
		return ErrReadFallbackPermissionDenied
	case strings.HasPrefix(reason, "scope:"):
		return ErrScopeNoMatch
	case strings.HasPrefix(reason, "availability:"):
		return ErrRouteUnavailable
	default:
		return ErrReadFallbackInvalid
	}
}

func invokeReadCandidate(ctx context.Context, candidate ReadFallbackCandidate, capabilityID string) (output json.RawMessage, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			output = nil
			err = &ReadFallbackPanicError{
				CapabilityID: capabilityID,
				ProviderID:   candidate.ProviderID,
				Value:        fmt.Sprint(recovered),
				Stack:        string(debug.Stack()),
			}
		}
	}()
	return candidate.Invoke(ctx)
}
