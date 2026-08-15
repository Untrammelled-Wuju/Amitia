package acquisition

import (
	"github.com/u-ai/backend/internal/execution"
)

type ExecutionPort interface {
	CreateChildExecution(parent execution.ExecutionContext, source string) execution.ExecutionContext
	CreateResume(execCtx execution.ExecutionContext, resumeType execution.ResumeType, capabilityID string) (*execution.ResumeContext, error)
	CompleteExecution(execCtx execution.ExecutionContext, summary string)
}

func NewExecutionPort(execSvc interface {
	CreateChildExecution(parent execution.ExecutionContext, source string) execution.ExecutionContext
	CreateResume(execCtx execution.ExecutionContext, resumeType execution.ResumeType, capabilityID string) (*execution.ResumeContext, error)
	CompleteExecution(execCtx execution.ExecutionContext, summary string)
}) ExecutionPort {
	return &executionPortAdapter{execSvc: execSvc}
}

type executionPortAdapter struct {
	execSvc interface {
		CreateChildExecution(parent execution.ExecutionContext, source string) execution.ExecutionContext
		CreateResume(execCtx execution.ExecutionContext, resumeType execution.ResumeType, capabilityID string) (*execution.ResumeContext, error)
		CompleteExecution(execCtx execution.ExecutionContext, summary string)
	}
}

func (a *executionPortAdapter) CreateChildExecution(parent execution.ExecutionContext, source string) execution.ExecutionContext {
	return a.execSvc.CreateChildExecution(parent, source)
}

func (a *executionPortAdapter) CreateResume(execCtx execution.ExecutionContext, resumeType execution.ResumeType, capabilityID string) (*execution.ResumeContext, error) {
	return a.execSvc.CreateResume(execCtx, resumeType, capabilityID)
}

func (a *executionPortAdapter) CompleteExecution(execCtx execution.ExecutionContext, summary string) {
	a.execSvc.CompleteExecution(execCtx, summary)
}
