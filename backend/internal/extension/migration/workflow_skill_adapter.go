package migration

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/extension"
	kwf "github.com/u-ai/backend/internal/extension/kernel/workflow"
)

func WorkflowToDefinition(wf extension.WorkflowDefinition, id, name, description, extensionID, moduleID string) kwf.WorkflowDefinition {
	nodes := make([]kwf.WorkflowNode, 0, len(wf.Steps))
	for _, step := range wf.Steps {
		onError := kwf.WorkflowOnError{Mode: "fail"}
		if step.OnError.Mode != "" {
			onError.Mode = step.OnError.Mode
			if len(step.OnError.Default) > 0 {
				onError.Default = json.RawMessage(append([]byte(nil), step.OnError.Default...))
			}
		}
		node := kwf.WorkflowNode{
			ID:   step.ID,
			Type: step.Type,
			Step: kwf.WorkflowStepInput{
				Input:   json.RawMessage(append([]byte(nil), step.Input...)),
				OnError: onError,
			},
		}
		if step.When != nil {
			if whenCopy, err := json.Marshal(step.When); err == nil {
				node.Step.When = (*json.RawMessage)(&whenCopy)
			}
		}
		nodes = append(nodes, node)
	}

	limits := kwf.WorkflowLimits{
		MaxSteps:               wf.Limits.MaxSteps,
		MaxExecutionDurationMS: wf.Limits.MaxExecutionDurationMS,
		MaxStepDurationMS:      wf.Limits.MaxStepDurationMS,
		MaxInputBytes:          wf.Limits.MaxInputBytes,
		MaxOutputBytes:         wf.Limits.MaxOutputBytes,
		MaxIntermediateBytes:   wf.Limits.MaxIntermediateBytes,
		MaxHTTPResponseBytes:   wf.Limits.MaxHTTPResponseBytes,
		MaxHTTPRedirects:       wf.Limits.MaxHTTPRedirects,
		MaxSkillCallDepth:      wf.Limits.MaxSkillCallDepth,
		MaxSkillCalls:          wf.Limits.MaxSkillCalls,
		MaxArrayItems:          wf.Limits.MaxArrayItems,
		MaxExpressionDepth:     wf.Limits.MaxExpressionDepth,
		MaxTemplateLength:      wf.Limits.MaxTemplateLength,
		MaxEventsEmitted:       wf.Limits.MaxEventsEmitted,
		MaxSchedulesCreated:    wf.Limits.MaxSchedulesCreated,
		MaxSideEffects:         wf.Limits.MaxSideEffects,
	}

	inputSchema := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	outputSchema := wf.Output
	if len(outputSchema) == 0 {
		outputSchema = json.RawMessage(`{}`)
	}

	return kwf.WorkflowDefinition{
		SchemaVersion:   wf.SchemaVersion,
		ID:              id,
		ExtensionID:     extensionID,
		ModuleID:        moduleID,
		Name:            name,
		Description:     description,
		InputSchema:     json.RawMessage(append([]byte(nil), inputSchema...)),
		OutputSchema:    json.RawMessage(append([]byte(nil), outputSchema...)),
		Nodes:           nodes,
		CallableByAgent: false,
		Enabled:         true,
		Limits:          limits,
		Version:         "1.0.0",
		Source:          "workshop",
	}
}

func WorkflowToCallable(wf extension.WorkflowDefinition, id, name, description, extensionID, moduleID string) kwf.WorkflowDefinition {
	def := WorkflowToDefinition(wf, id, name, description, extensionID, moduleID)
	def.CallableByAgent = true
	return def
}
