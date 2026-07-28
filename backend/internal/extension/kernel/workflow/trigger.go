package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type TriggerType string

const (
	TriggerTypeManual   TriggerType = "manual"
	TriggerTypeEvent    TriggerType = "event"
	TriggerTypeSchedule TriggerType = "schedule"
)

type TriggerBinding struct {
	BindingID  string          `json:"bindingId"`
	Type       TriggerType     `json:"type"`
	EventType  string          `json:"eventType,omitempty"`
	ScheduleID string          `json:"scheduleId,omitempty"`
	WorkflowID string          `json:"workflowId"`
	Input      json.RawMessage `json:"input,omitempty"`
	Generation int64           `json:"generation"`
	Enabled    bool            `json:"enabled"`
}

type TriggerStore interface {
	SaveTrigger(ctx context.Context, binding TriggerBinding) error
	ListTriggers(ctx context.Context, triggerType TriggerType, eventType, scheduleID string) ([]TriggerBinding, error)
	DeleteTrigger(ctx context.Context, bindingID string) error
}

type TriggerManager struct {
	mu       sync.RWMutex
	bindings []TriggerBinding
	executor *WorkflowExecutor
	store    TriggerStore
}

func NewTriggerManager(executor *WorkflowExecutor) *TriggerManager {
	return &TriggerManager{executor: executor}
}

func (tm *TriggerManager) SetStore(store TriggerStore) {
	tm.mu.Lock()
	tm.store = store
	tm.mu.Unlock()
}

func (tm *TriggerManager) Register(binding TriggerBinding) {
	if binding.BindingID == "" {
		binding.BindingID = fmt.Sprintf("%s:%s:%s", binding.Type, binding.WorkflowID, firstNonEmpty(binding.EventType, binding.ScheduleID))
	}
	if !binding.Enabled {
		binding.Enabled = true
	}
	tm.mu.Lock()
	tm.bindings = append(tm.bindings, binding)
	store := tm.store
	tm.mu.Unlock()
	if store != nil {
		_ = store.SaveTrigger(context.Background(), binding)
	}
}

func (tm *TriggerManager) ExecuteManual(ctx context.Context, workflowID string, input json.RawMessage, execution ExecutionContext) (*ExecuteResult, error) {
	if execution.InvocationID == "" {
		execution.InvocationID = fmt.Sprintf("manual-wf-%s-%d", workflowID, time.Now().UnixNano())
	}
	return tm.executor.Execute(ctx, ExecuteRequest{WorkflowID: workflowID, Input: input, Context: execution})
}

func (tm *TriggerManager) HandleEvent(ctx context.Context, eventType string, payload json.RawMessage) error {
	return tm.HandleEventWithContext(ctx, eventType, payload, ExecutionContext{})
}

func (tm *TriggerManager) HandleEventWithContext(ctx context.Context, eventType string, payload json.RawMessage, execution ExecutionContext) error {
	bindings, err := tm.list(ctx, TriggerTypeEvent, eventType, "")
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		bindingExecution := execution
		input := binding.Input
		if len(payload) > 0 {
			input = payload
		}
		bindingExecution.Generation = binding.Generation
		if bindingExecution.InvocationID == "" {
			bindingExecution.InvocationID = fmt.Sprintf("event-wf-%s-%d", binding.BindingID, time.Now().UnixNano())
		} else {
			bindingExecution.InvocationID = fmt.Sprintf("%s/%s", bindingExecution.InvocationID, binding.BindingID)
		}
		if _, err := tm.executor.Execute(ctx, ExecuteRequest{WorkflowID: binding.WorkflowID, Input: input, Context: bindingExecution}); err != nil {
			return err
		}
	}
	return nil
}

func (tm *TriggerManager) HandleSchedule(ctx context.Context, scheduleID string, payload json.RawMessage) error {
	return tm.HandleScheduleWithContext(ctx, scheduleID, payload, ExecutionContext{ScheduleID: scheduleID})
}

func (tm *TriggerManager) HandleScheduleWithContext(ctx context.Context, scheduleID string, payload json.RawMessage, execution ExecutionContext) error {
	bindings, err := tm.list(ctx, TriggerTypeSchedule, "", scheduleID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		bindingExecution := execution
		input := binding.Input
		if len(payload) > 0 {
			input = payload
		}
		bindingExecution.Generation = binding.Generation
		if bindingExecution.InvocationID != "" {
			bindingExecution.InvocationID = fmt.Sprintf("%s/%s", bindingExecution.InvocationID, binding.BindingID)
		}
		if _, err := tm.executor.Execute(ctx, ExecuteRequest{WorkflowID: binding.WorkflowID, Input: input, Context: bindingExecution}); err != nil {
			return err
		}
	}
	return nil
}

func (tm *TriggerManager) list(ctx context.Context, triggerType TriggerType, eventType, scheduleID string) ([]TriggerBinding, error) {
	tm.mu.RLock()
	store := tm.store
	local := append([]TriggerBinding(nil), tm.bindings...)
	tm.mu.RUnlock()
	if store != nil {
		return store.ListTriggers(ctx, triggerType, eventType, scheduleID)
	}
	result := make([]TriggerBinding, 0)
	for _, binding := range local {
		if !binding.Enabled || binding.Type != triggerType {
			continue
		}
		if eventType != "" && binding.EventType != eventType {
			continue
		}
		if scheduleID != "" && binding.ScheduleID != scheduleID {
			continue
		}
		result = append(result, binding)
	}
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "manual"
}
