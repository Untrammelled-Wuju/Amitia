package host_api

import (
	"strings"

	"github.com/u-ai/backend/internal/execution"
)

func (l SideEffectLevel) Class() execution.SideEffectClass {
	switch l {
	case SideEffectNone:
		return execution.SideEffectClassNone
	case SideEffectReadOnly:
		return execution.SideEffectClassRead
	case SideEffectWrite:
		return execution.SideEffectClassWrite
	case SideEffectExternal:
		return execution.SideEffectClassExternal
	}
	return ""
}

func (r CallRequest) InvocationTrace() execution.InvocationTrace {
	return execution.InvocationTrace{
		RootExecutionID:    strings.TrimSpace(r.RootExecutionID),
		TraceID:            strings.TrimSpace(r.TraceID),
		InvocationID:       strings.TrimSpace(r.InvocationID),
		ParentInvocationID: strings.TrimSpace(r.ParentID),
	}
}

func (r CallRequest) InvocationContract(sideEffect SideEffectLevel) (execution.InvocationContract, error) {
	return execution.NewInvocationContract(r.InvocationTrace(), string(r.Method), sideEffect.Class(), r.IdempotencyKey)
}

func (r Route) InvocationContract(request CallRequest) (execution.InvocationContract, error) {
	return request.InvocationContract(r.SideEffectLevel)
}
