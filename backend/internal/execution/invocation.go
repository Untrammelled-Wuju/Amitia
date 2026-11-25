package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvocationContractInvalid = errors.New("execution: invocation contract invalid")
	ErrInvocationOutcomeUnknown  = errors.New("execution: invocation outcome unknown")
	ErrInvocationTransportLost   = errors.New("execution: invocation transport lost")
)

type SideEffectClass string

const (
	SideEffectClassNone     SideEffectClass = "none"
	SideEffectClassRead     SideEffectClass = "read"
	SideEffectClassWrite    SideEffectClass = "write"
	SideEffectClassExternal SideEffectClass = "external"
)

func ParseSideEffectClass(value string) (SideEffectClass, error) {
	class := SideEffectClass(strings.TrimSpace(value))
	if !class.Valid() {
		return "", &InvocationContractInvalidError{Field: "sideEffect", Value: value, Reason: "want none, read, write or external"}
	}
	return class, nil
}

func (c SideEffectClass) Valid() bool {
	switch c {
	case SideEffectClassNone, SideEffectClassRead, SideEffectClassWrite, SideEffectClassExternal:
		return true
	}
	return false
}

func (c SideEffectClass) Mutating() bool {
	return c == SideEffectClassWrite || c == SideEffectClassExternal
}

type InvocationOutcome string

const (
	InvocationOutcomeSuccess InvocationOutcome = "success"
	InvocationOutcomeFailed  InvocationOutcome = "failed"
	InvocationOutcomeUnknown InvocationOutcome = "unknown"
)

type InvocationContractInvalidError struct {
	Field  string
	Value  string
	Reason string
}

func (e *InvocationContractInvalidError) Error() string {
	return fmt.Sprintf("execution: invocation contract field %q value %q: %s", e.Field, e.Value, e.Reason)
}

func (e *InvocationContractInvalidError) Unwrap() error { return ErrInvocationContractInvalid }

type InvocationTrace struct {
	RootExecutionID    string `json:"rootExecutionId"`
	TraceID            string `json:"traceId"`
	InvocationID       string `json:"invocationId"`
	ParentInvocationID string `json:"parentInvocationId,omitempty"`
}

func (t InvocationTrace) Validate() error {
	if strings.TrimSpace(t.RootExecutionID) == "" {
		return &InvocationContractInvalidError{Field: "trace.rootExecutionId", Value: t.RootExecutionID, Reason: "must not be empty"}
	}
	if strings.TrimSpace(t.TraceID) == "" {
		return &InvocationContractInvalidError{Field: "trace.traceId", Value: t.TraceID, Reason: "must not be empty"}
	}
	if strings.TrimSpace(t.InvocationID) == "" {
		return &InvocationContractInvalidError{Field: "trace.invocationId", Value: t.InvocationID, Reason: "must not be empty"}
	}
	return nil
}

func (t InvocationTrace) Child(invocationID string) (InvocationTrace, error) {
	if err := t.Validate(); err != nil {
		return InvocationTrace{}, err
	}
	if strings.TrimSpace(invocationID) == "" {
		return InvocationTrace{}, &InvocationContractInvalidError{Field: "trace.childInvocationId", Value: invocationID, Reason: "must not be empty"}
	}
	if invocationID == t.InvocationID {
		return InvocationTrace{}, &InvocationContractInvalidError{Field: "trace.childInvocationId", Value: invocationID, Reason: "must differ from the parent invocation"}
	}
	return InvocationTrace{
		RootExecutionID:    t.RootExecutionID,
		TraceID:            t.TraceID,
		InvocationID:       invocationID,
		ParentInvocationID: t.InvocationID,
	}, nil
}

type InvocationContract struct {
	Trace          InvocationTrace
	CapabilityID   string
	SideEffect     SideEffectClass
	IdempotencyKey string
}

func NewInvocationContract(trace InvocationTrace, capabilityID string, sideEffect SideEffectClass, idempotencyKey string) (InvocationContract, error) {
	contract := InvocationContract{
		Trace:          trace,
		CapabilityID:   strings.TrimSpace(capabilityID),
		SideEffect:     sideEffect,
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
	}
	if err := contract.Validate(); err != nil {
		return InvocationContract{}, err
	}
	return contract, nil
}

func (c InvocationContract) Validate() error {
	if err := c.Trace.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.CapabilityID) == "" {
		return &InvocationContractInvalidError{Field: "capabilityId", Value: c.CapabilityID, Reason: "must not be empty"}
	}
	if !c.SideEffect.Valid() {
		return &InvocationContractInvalidError{Field: "sideEffect", Value: string(c.SideEffect), Reason: "want none, read, write or external"}
	}
	if c.SideEffect.Mutating() && strings.TrimSpace(c.IdempotencyKey) == "" {
		return &InvocationContractInvalidError{Field: "idempotencyKey", Value: c.IdempotencyKey, Reason: "mutating invocations require an idempotency key"}
	}
	return nil
}

type UnknownInvocationOutcomeError struct {
	Trace        InvocationTrace
	CapabilityID string
	SideEffect   SideEffectClass
	Cause        error
}

func (e *UnknownInvocationOutcomeError) Error() string {
	return fmt.Sprintf(
		"execution: invocation %q of capability %q has an unknown outcome after dispatch (sideEffect=%s): %v",
		e.Trace.InvocationID, e.CapabilityID, e.SideEffect, e.Cause,
	)
}

func (e *UnknownInvocationOutcomeError) Unwrap() error { return e.Cause }

func (e *UnknownInvocationOutcomeError) Is(target error) bool {
	return target == ErrInvocationOutcomeUnknown
}

func (e *UnknownInvocationOutcomeError) RetrySafe() bool { return false }

func IsAmbiguousInvocationFailure(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrInvocationTransportLost) || errors.Is(err, context.Canceled)
}

func ClassifyInvocationOutcome(contract InvocationContract, dispatched bool, err error) (InvocationOutcome, error) {
	if err == nil {
		return InvocationOutcomeSuccess, nil
	}
	if !dispatched {
		return InvocationOutcomeFailed, err
	}
	if IsAmbiguousInvocationFailure(err) && contract.SideEffect.Mutating() {
		return InvocationOutcomeUnknown, &UnknownInvocationOutcomeError{
			Trace:        contract.Trace,
			CapabilityID: contract.CapabilityID,
			SideEffect:   contract.SideEffect,
			Cause:        err,
		}
	}
	return InvocationOutcomeFailed, err
}

func (c InvocationContract) JournalEntry(kind JournalEntryKind, summary string) JournalEntry {
	return JournalEntry{
		RootExecutionID: c.Trace.RootExecutionID,
		ExecutionID:     c.Trace.InvocationID,
		Kind:            kind,
		TraceID:         c.Trace.TraceID,
		InvocationID:    c.Trace.InvocationID,
		Summary:         summary,
	}
}
