package workflow

import (
	"context"
	"encoding/json"
)

type TriggerType string

const (
	TriggerTypeEvent    TriggerType = "event"
	TriggerTypeSchedule TriggerType = "schedule"
)

type TriggerBinding struct {
	Type       TriggerType
	EventType  string
	ScheduleID string
	WorkflowID string
	Input      json.RawMessage
}

type TriggerManager struct {
	bindings []TriggerBinding
	executor *WorkflowExecutor
}

func NewTriggerManager(executor *WorkflowExecutor) *TriggerManager {
	return &TriggerManager{
		executor: executor,
	}
}

func (tm *TriggerManager) Register(binding TriggerBinding) {
	tm.bindings = append(tm.bindings, binding)
}

func (tm *TriggerManager) HandleEvent(ctx context.Context, eventType string, payload json.RawMessage) error {
	for _, binding := range tm.bindings {
		if binding.Type != TriggerTypeEvent || binding.EventType != eventType {
			continue
		}
		input := binding.Input
		if len(payload) > 0 {
			input = payload
		}
		_, err := tm.executor.Execute(ctx, ExecuteRequest{
			WorkflowID: binding.WorkflowID,
			Input:      input,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (tm *TriggerManager) HandleSchedule(ctx context.Context, scheduleID string, payload json.RawMessage) error {
	for _, binding := range tm.bindings {
		if binding.Type != TriggerTypeSchedule || binding.ScheduleID != scheduleID {
			continue
		}
		input := binding.Input
		if len(payload) > 0 {
			input = payload
		}
		_, err := tm.executor.Execute(ctx, ExecuteRequest{
			WorkflowID: binding.WorkflowID,
			Input:      input,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
