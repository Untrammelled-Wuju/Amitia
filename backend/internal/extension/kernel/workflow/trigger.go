package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	Config     json.RawMessage `json:"config,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Generation int64           `json:"generation"`
	Enabled    bool            `json:"enabled"`
}

type TriggerStore interface {
	SaveTrigger(ctx context.Context, binding TriggerBinding) error
	ListTriggers(ctx context.Context, triggerType TriggerType, eventType, scheduleID string) ([]TriggerBinding, error)
	DeleteTrigger(ctx context.Context, bindingID string) error
}

type TriggerReceiptStore interface {
	ClaimTriggerReceipt(ctx context.Context, eventID, bindingID, invocationID string, occurredAt time.Time) (bool, error)
	CompleteTriggerReceipt(ctx context.Context, eventID, bindingID string) error
	ReleaseTriggerReceipt(ctx context.Context, eventID, bindingID string) error
}

type TriggerManager struct {
	mu              sync.RWMutex
	bindings        []TriggerBinding
	executor        *WorkflowExecutor
	store           TriggerStore
	matcherRegistry *WorkflowEventMatcherRegistry
	secretResolver  WorkflowTriggerSecretResolver
}

func NewTriggerManager(executor *WorkflowExecutor) *TriggerManager {
	return &TriggerManager{executor: executor, matcherRegistry: NewWorkflowEventMatcherRegistry()}
}

func (tm *TriggerManager) SetStore(store TriggerStore) {
	tm.mu.Lock()
	tm.store = store
	tm.mu.Unlock()
}

func (tm *TriggerManager) SetEventMatcherRegistry(registry *WorkflowEventMatcherRegistry) {
	if registry == nil {
		registry = NewWorkflowEventMatcherRegistry()
	}
	tm.mu.Lock()
	tm.matcherRegistry = registry
	tm.mu.Unlock()
}

func (tm *TriggerManager) SetSecretResolver(resolver WorkflowTriggerSecretResolver) {
	tm.mu.Lock()
	tm.secretResolver = resolver
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

func (tm *TriggerManager) HandleStructuredEvent(ctx context.Context, event WorkflowTriggerEvent, execution ExecutionContext) error {
	event.EventID = strings.TrimSpace(event.EventID)
	event.EventType = strings.TrimSpace(event.EventType)
	if event.EventID == "" || event.EventType == "" {
		return fmt.Errorf("structured workflow event requires eventId and eventType")
	}
	if !validStructuredWorkflowEventID(event.EventID) {
		return fmt.Errorf("structured workflow eventId contains unsupported characters")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	bindings, err := tm.structuredEventBindings(ctx, event)
	if err != nil {
		return err
	}
	tm.mu.RLock()
	registry := tm.matcherRegistry
	resolver := tm.secretResolver
	store := tm.store
	tm.mu.RUnlock()
	var bindingErrors []error
	for _, binding := range bindings {
		bindingEvent := event
		if strings.TrimSpace(bindingEvent.OwnerUserID) == "" {
			bindingEvent.OwnerUserID = ownerFromQualifiedWorkflowEventType(binding.EventType, event.EventType)
		}
		if strings.TrimSpace(bindingEvent.EventType) == "" || !strings.HasPrefix(strings.TrimSpace(bindingEvent.EventType), "user:") {
			bindingEvent.EventType = binding.EventType
		}
		match, err := registry.Match(ctx, bindingEvent, binding, resolver)
		if err != nil {
			bindingErrors = append(bindingErrors, fmt.Errorf("workflow trigger binding %s match: %w", binding.BindingID, err))
			continue
		}
		if !match.Matched {
			continue
		}
		effectiveEventID := event.EventID
		if strings.TrimSpace(match.DedupEventID) != "" {
			effectiveEventID = strings.TrimSpace(match.DedupEventID)
		}
		invocationID := fmt.Sprintf("device-event/%s/%s", effectiveEventID, binding.BindingID)
		receiptStore, hasReceiptStore := store.(TriggerReceiptStore)
		if hasReceiptStore {
			claimed, err := receiptStore.ClaimTriggerReceipt(ctx, effectiveEventID, binding.BindingID, invocationID, bindingEvent.OccurredAt)
			if err != nil {
				registry.Rollback(bindingEvent, binding)
				bindingErrors = append(bindingErrors, fmt.Errorf("workflow trigger binding %s claim receipt: %w", binding.BindingID, err))
				continue
			}
			if !claimed {
				registry.Rollback(bindingEvent, binding)
				continue
			}
		}
		bindingExecution := execution
		if bindingExecution.UserID == "" {
			bindingExecution.UserID = bindingEvent.OwnerUserID
		}
		if bindingExecution.DeviceID == "" {
			bindingExecution.DeviceID = event.DeviceID
		}
		bindingExecution.Generation = binding.Generation
		bindingExecution.InvocationID = invocationID
		bindingExecution.TriggerID = binding.BindingID
		if bindingExecution.RootID == "" {
			bindingExecution.RootID = "device-event/" + effectiveEventID
		}
		if bindingExecution.IdempotencyKey == "" {
			bindingExecution.IdempotencyKey = invocationID
		}
		input := binding.Input
		if len(match.Payload) > 0 {
			input = match.Payload
		}
		if _, err := tm.executor.Execute(ctx, ExecuteRequest{WorkflowID: binding.WorkflowID, Input: input, Context: bindingExecution}); err != nil {
			registry.Rollback(bindingEvent, binding)
			if hasReceiptStore {
				_ = receiptStore.ReleaseTriggerReceipt(ctx, effectiveEventID, binding.BindingID)
			}
			bindingErrors = append(bindingErrors, fmt.Errorf("workflow trigger binding %s execute: %w", binding.BindingID, err))
			continue
		}
		if hasReceiptStore {
			if err := receiptStore.CompleteTriggerReceipt(ctx, effectiveEventID, binding.BindingID); err != nil {
				bindingErrors = append(bindingErrors, fmt.Errorf("workflow trigger binding %s complete receipt: %w", binding.BindingID, err))
			}
		}
	}
	return errors.Join(bindingErrors...)
}

func validStructuredWorkflowEventID(value string) bool {
	if value == "" || len(value) > 200 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == ':' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func (tm *TriggerManager) structuredEventBindings(ctx context.Context, event WorkflowTriggerEvent) ([]TriggerBinding, error) {
	if strings.TrimSpace(event.OwnerUserID) != "" || strings.HasPrefix(strings.TrimSpace(event.EventType), "user:") {
		return tm.list(ctx, TriggerTypeEvent, event.EventType, "")
	}
	all, err := tm.list(ctx, TriggerTypeEvent, "", "")
	if err != nil {
		return nil, err
	}
	eventType := strings.TrimSpace(event.EventType)
	result := make([]TriggerBinding, 0)
	for _, binding := range all {
		qualified := strings.TrimSpace(binding.EventType)
		if qualified == eventType || ownerFromQualifiedWorkflowEventType(qualified, eventType) != "" {
			result = append(result, binding)
		}
	}
	return result, nil
}

func ownerFromQualifiedWorkflowEventType(qualified, eventType string) string {
	qualified = strings.TrimSpace(qualified)
	eventType = strings.TrimSpace(eventType)
	if eventType == "" || !strings.HasPrefix(qualified, "user:") {
		return ""
	}
	suffix := ":" + eventType
	if !strings.HasSuffix(qualified, suffix) {
		return ""
	}
	owner := strings.TrimSuffix(strings.TrimPrefix(qualified, "user:"), suffix)
	return strings.TrimSpace(owner)
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
