package event_bus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type HookPoint string

type HookPhase string

const (
	HookPhaseBefore   HookPhase = "before"
	HookPhaseFilter   HookPhase = "filter"
	HookPhaseTransform HookPhase = "transform"
	HookPhaseAfter    HookPhase = "after"
	HookPhaseObserve  HookPhase = "observe"
)

type FailurePolicy string

const (
	FailureAbort      FailurePolicy = "abort"
	FailureContinue   FailurePolicy = "continue"
	FailureSkip       FailurePolicy = "skip"
	FailureQuarantine FailurePolicy = "quarantine"
)

type HookContext struct {
	Operation     string
	OperationID   string
	Subject       string
	Actor         string
	Phase         HookPhase
	Input         map[string]any
	Output        map[string]any
	TraceID       string
	ScopeID       string
	Deadline      time.Time
	Metadata      map[string]any
	abort         bool
	abortReason   string
	transformed   bool
}

func (c *HookContext) Abort(reason string) {
	c.abort = true
	c.abortReason = reason
}

func (c *HookContext) IsAborted() bool {
	return c.abort
}

func (c *HookContext) AbortReason() string {
	return c.abortReason
}

func (c *HookContext) MarkTransformed() {
	c.transformed = true
}

func (c *HookContext) IsTransformed() bool {
	return c.transformed
}

type Hook struct {
	HookID        string
	Point         HookPoint
	Phase         HookPhase
	Owner         string
	OwnerExtension string
	Priority      int
	Handler       HookHandler
	FailurePolicy FailurePolicy
	Timeout       time.Duration
	MaxDepth      int
	Required      bool
	Active        bool
	CreatedAt     time.Time
}

type HookHandler func(ctx context.Context, hookCtx *HookContext) error

type HookRegistration struct {
	HookID        string
	Point         HookPoint
	Phase         HookPhase
	Owner         string
	OwnerExtension string
	Priority      int
	Handler       HookHandler
	FailurePolicy FailurePolicy
	Timeout       time.Duration
	Required      bool
	MaxDepth      int
}

type PipelineResult struct {
	OperationID    string
	Point          HookPoint
	Aborted        bool
	AbortReason    string
	Transformed    bool
	ExecutedHooks  []HookExecution
	TotalDuration  time.Duration
}

type HookExecution struct {
	HookID      string
	Phase       HookPhase
	Status      string
	Error       string
	Duration    time.Duration
	StartedAt   time.Time
}

type Pipeline interface {
	Register(ctx context.Context, registration HookRegistration) error
	Unregister(ctx context.Context, hookID string) error
	Execute(ctx context.Context, point HookPoint, hookCtx *HookContext) PipelineResult
	List(ctx context.Context, point HookPoint) []Hook
}

var (
	ErrHookExists        = errors.New("hook: already registered")
	ErrHookNotFound      = errors.New("hook: not found")
	ErrHookAbort         = errors.New("hook: abort")
	ErrHookTimeout       = errors.New("hook: timeout")
	ErrHookRequiredFail  = errors.New("hook: required hook failed")
)

const (
	HookStatusSuccess = "success"
	HookStatusFailed  = "failed"
	HookStatusSkipped = "skipped"
	HookStatusAbort   = "abort"
	HookStatusTimeout = "timeout"
)

type DefaultPipeline struct {
	mu     sync.RWMutex
	hooks  map[string]*Hook
	byPoint map[HookPoint][]string
}

func NewDefaultPipeline() *DefaultPipeline {
	return &DefaultPipeline{
		hooks:   make(map[string]*Hook),
		byPoint: make(map[HookPoint][]string),
	}
}

func (p *DefaultPipeline) Register(_ context.Context, reg HookRegistration) error {
	if reg.Point == "" {
		return fmt.Errorf("hook: point required")
	}
	if reg.Handler == nil {
		return fmt.Errorf("hook: handler required")
	}
	if reg.HookID == "" {
		reg.HookID = fmt.Sprintf("hook-%s", uuid.NewString())
	}
	if reg.FailurePolicy == "" {
		reg.FailurePolicy = FailureAbort
	}
	if reg.Timeout == 0 {
		reg.Timeout = 5 * time.Second
	}
	hook := &Hook{
		HookID:         reg.HookID,
		Point:          reg.Point,
		Phase:          reg.Phase,
		Owner:          reg.Owner,
		OwnerExtension: reg.OwnerExtension,
		Priority:       reg.Priority,
		Handler:        reg.Handler,
		FailurePolicy:  reg.FailurePolicy,
		Timeout:        reg.Timeout,
		MaxDepth:       reg.MaxDepth,
		Required:       reg.Required,
		Active:         true,
		CreatedAt:      time.Now().UTC(),
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.hooks[reg.HookID]; exists {
		return ErrHookExists
	}
	p.hooks[reg.HookID] = hook
	p.byPoint[reg.Point] = append(p.byPoint[reg.Point], reg.HookID)
	return nil
}

func (p *DefaultPipeline) Unregister(_ context.Context, hookID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	hook, ok := p.hooks[hookID]
	if !ok {
		return ErrHookNotFound
	}
	delete(p.hooks, hookID)
	ids := p.byPoint[hook.Point]
	for i, id := range ids {
		if id == hookID {
			p.byPoint[hook.Point] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	return nil
}

func (p *DefaultPipeline) Execute(ctx context.Context, point HookPoint, hookCtx *HookContext) PipelineResult {
	start := time.Now()
	result := PipelineResult{
		OperationID: hookCtx.OperationID,
		Point:       point,
	}
	if hookCtx.Phase == "" {
		hookCtx.Phase = HookPhaseBefore
	}
	p.mu.RLock()
	hooks := p.collectHooksLocked(point)
	p.mu.RUnlock()
	phaseOrder := []HookPhase{HookPhaseBefore, HookPhaseFilter, HookPhaseTransform, HookPhaseAfter, HookPhaseObserve}
	for _, phase := range phaseOrder {
		if hookCtx.IsAborted() {
			break
		}
		phaseHooks := filterByPhase(hooks, phase)
		for _, hook := range phaseHooks {
			if !hook.Active {
				continue
			}
			exec := p.executeHook(ctx, hook, hookCtx)
			result.ExecutedHooks = append(result.ExecutedHooks, exec)
			if exec.Status == HookStatusAbort {
				result.Aborted = true
				result.AbortReason = hookCtx.AbortReason()
				break
			}
			if exec.Status == HookStatusFailed {
				switch hook.FailurePolicy {
				case FailureAbort:
					result.Aborted = true
					result.AbortReason = exec.Error
					if hook.Required {
						result.AbortReason = fmt.Sprintf("required hook %s failed: %s", hook.HookID, exec.Error)
					}
				case FailureContinue:
				case FailureSkip:
				case FailureQuarantine:
					hook.Active = false
				}
				if hook.Required {
					result.Aborted = true
					break
				}
			}
			if hookCtx.IsAborted() {
				result.Aborted = true
				result.AbortReason = hookCtx.AbortReason()
				break
			}
		}
		if result.Aborted {
			break
		}
	}
	result.Transformed = hookCtx.IsTransformed()
	result.TotalDuration = time.Since(start)
	return result
}

func (p *DefaultPipeline) List(_ context.Context, point HookPoint) []Hook {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := p.byPoint[point]
	var out []Hook
	for _, id := range ids {
		if h, ok := p.hooks[id]; ok {
			out = append(out, *h)
		}
	}
	return out
}

func (p *DefaultPipeline) collectHooksLocked(point HookPoint) []*Hook {
	ids := p.byPoint[point]
	var hooks []*Hook
	for _, id := range ids {
		if h, ok := p.hooks[id]; ok && h.Active {
			hooks = append(hooks, h)
		}
	}
	sort.SliceStable(hooks, func(i, j int) bool {
		if hooks[i].Priority != hooks[j].Priority {
			return hooks[i].Priority > hooks[j].Priority
		}
		return hooks[i].HookID < hooks[j].HookID
	})
	return hooks
}

func (p *DefaultPipeline) executeHook(ctx context.Context, hook *Hook, hookCtx *HookContext) HookExecution {
	exec := HookExecution{
		HookID:    hook.HookID,
		Phase:     hook.Phase,
		StartedAt: time.Now().UTC(),
	}
	hookCtx.Phase = hook.Phase
	callCtx, cancel := context.WithTimeout(ctx, hook.Timeout)
	defer cancel()
	err := hook.Handler(callCtx, hookCtx)
	exec.Duration = time.Since(exec.StartedAt)
	if err != nil {
		if errors.Is(err, ErrHookAbort) {
			exec.Status = HookStatusAbort
		} else if errors.Is(err, context.DeadlineExceeded) {
			exec.Status = HookStatusTimeout
		} else {
			exec.Status = HookStatusFailed
		}
		exec.Error = err.Error()
		return exec
	}
	if hookCtx.IsAborted() {
		exec.Status = HookStatusAbort
		return exec
	}
	exec.Status = HookStatusSuccess
	return exec
}

func filterByPhase(hooks []*Hook, phase HookPhase) []*Hook {
	var out []*Hook
	for _, h := range hooks {
		if h.Phase == phase {
			out = append(out, h)
		}
	}
	return out
}

var _ Pipeline = (*DefaultPipeline)(nil)
