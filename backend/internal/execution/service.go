package execution

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type ResumeHandler interface {
	CanHandle(resumeType ResumeType) bool
	ResumeExecution(ctx context.Context, resume ResumeContext, execCtx ExecutionContext) (*ExecutionContext, error)
}

type ExecutionService struct {
	journal         *InMemoryJournal
	resumeHandlers  []ResumeHandler
	mu              sync.RWMutex
	activeContexts  map[string]ExecutionContext
	resumeContexts  map[string]ResumeContext
}

func NewExecutionService() *ExecutionService {
	return &ExecutionService{
		journal:        NewInMemoryJournal(),
		activeContexts: make(map[string]ExecutionContext),
		resumeContexts: make(map[string]ResumeContext),
	}
}

func (s *ExecutionService) RegisterResumeHandler(handler ResumeHandler) {
	s.resumeHandlers = append(s.resumeHandlers, handler)
}

func (s *ExecutionService) StartExecution(ctx context.Context, rootID, userID string) ExecutionContext {
	execCtx := NewExecutionContext(rootID, userID)
	s.mu.Lock()
	s.activeContexts[execCtx.ExecutionID] = execCtx
	s.mu.Unlock()
	s.journal.Record(JournalEntry{
		RootExecutionID: rootID,
		ExecutionID:     execCtx.ExecutionID,
		Kind:            JournalEntryExecutionStarted,
		TraceID:         execCtx.TraceID,
	})
	return execCtx
}

func (s *ExecutionService) CreateChildExecution(parent ExecutionContext, source string) ExecutionContext {
	child := NewChildExecution(parent, source)
	s.mu.Lock()
	s.activeContexts[child.ExecutionID] = child
	s.mu.Unlock()
	s.journal.Record(JournalEntry{
		RootExecutionID: child.RootExecutionID,
		ExecutionID:     child.ExecutionID,
		Kind:            JournalEntryExecutionStarted,
		TraceID:         child.TraceID,
		Source:          source,
	})
	return child
}

func (s *ExecutionService) CreateResume(execCtx ExecutionContext, resumeType ResumeType, capabilityID string) (*ResumeContext, error) {
	if !execCtx.Budget.CanAcquireCapability() && resumeType == ResumeTypeCapabilityAcquisition {
		return nil, ErrBudgetExhausted
	}

	resume := NewResumeContext(execCtx, resumeType)
	if capabilityID != "" {
		resume.RequiredCapabilityID = fakeCapabilityID(capabilityID)
	}
	resume.RootExecutionID = execCtx.RootExecutionID
	resume.ParentExecutionID = execCtx.ExecutionID

	s.mu.Lock()
	s.resumeContexts[resume.ResumeID] = *resume
	s.mu.Unlock()

	s.journal.Record(JournalEntry{
		RootExecutionID: execCtx.RootExecutionID,
		ExecutionID:     execCtx.ExecutionID,
		Kind:            JournalEntryResumeCreated,
		Summary:         string(resumeType),
	})

	if resumeType == ResumeTypeCapabilityAcquisition {
		execCtx.Budget.IncrementAcquisitions()
		s.mu.Lock()
		s.activeContexts[execCtx.ExecutionID] = execCtx
		s.mu.Unlock()
	}

	return resume, nil
}

func (s *ExecutionService) ResumeExecution(ctx context.Context, resumeID string) (*ExecutionContext, error) {
	s.mu.Lock()
	resume, ok := s.resumeContexts[resumeID]
	s.mu.Unlock()
	if !ok {
		return nil, ErrResumeNotFound
	}

	resume.MarkInProgress()
	s.mu.Lock()
	s.resumeContexts[resumeID] = resume
	s.mu.Unlock()

	var handled bool
	for _, handler := range s.resumeHandlers {
		if handler.CanHandle(resume.Type) {
			handled = true
			result, err := handler.ResumeExecution(ctx, resume, s.reconstructExecutionContext(resume))
			if err != nil {
				resume.MarkFailed(err.Error())
				s.mu.Lock()
				s.resumeContexts[resumeID] = resume
				s.mu.Unlock()
				return nil, err
			}
			resume.MarkCompleted()
			s.mu.Lock()
			s.resumeContexts[resumeID] = resume
			s.activeContexts[result.ExecutionID] = *result
			s.mu.Unlock()
			s.journal.Record(JournalEntry{
				RootExecutionID: resume.RootExecutionID,
				ExecutionID:     result.ExecutionID,
				Kind:            JournalEntryResumeCompleted,
			})
			return result, nil
		}
	}

	if !handled {
		resume.MarkFailed("no handler for resume type")
		s.mu.Lock()
		s.resumeContexts[resumeID] = resume
		s.mu.Unlock()
		return nil, fmt.Errorf("no resume handler for type: %s", resume.Type)
	}

	return nil, nil
}

func (s *ExecutionService) reconstructExecutionContext(resume ResumeContext) ExecutionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if execCtx, ok := s.activeContexts[resume.ParentExecutionID]; ok {
		return execCtx
	}
	return ExecutionContext{
		ExecutionID:    resume.ParentExecutionID,
		RootExecutionID: resume.RootExecutionID,
	}
}

func (s *ExecutionService) CompleteExecution(execCtx ExecutionContext, summary string) {
	s.mu.Lock()
	execCtx.Metadata = map[string]any{"completed": true, "summary": summary}
	s.activeContexts[execCtx.ExecutionID] = execCtx
	s.mu.Unlock()
	s.journal.Record(JournalEntry{
		RootExecutionID: execCtx.RootExecutionID,
		ExecutionID:     execCtx.ExecutionID,
		Kind:            JournalEntryExecutionCompleted,
		TraceID:         execCtx.TraceID,
		Summary:         summary,
	})
}

func (s *ExecutionService) FailExecution(execCtx ExecutionContext, err error) {
	s.mu.Lock()
	s.activeContexts[execCtx.ExecutionID] = execCtx
	s.mu.Unlock()
	s.journal.Record(JournalEntry{
		RootExecutionID: execCtx.RootExecutionID,
		ExecutionID:     execCtx.ExecutionID,
		Kind:            JournalEntryExecutionFailed,
		TraceID:         execCtx.TraceID,
		Summary:         err.Error(),
	})
}

func (s *ExecutionService) GetJournal() *InMemoryJournal {
	return s.journal
}

func fakeCapabilityID(s string) FakeCapabilityID {
	return FakeCapabilityID(s)
}

type FakeCapabilityID = fakeCapabilityIDType
type fakeCapabilityIDType string

var (
	ErrBudgetExhausted = errors.New("execution service: budget exhausted")
	ErrResumeNotFound  = errors.New("execution service: resume not found")
)
