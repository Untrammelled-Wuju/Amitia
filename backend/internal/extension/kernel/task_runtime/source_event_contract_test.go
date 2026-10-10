package task_runtime

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

func TestSourceEventTaskPreflightRejectsMissingCurrentContract(t *testing.T) {
	_, _, definition, _ := sourcePermissionFixture(t)
	definition.PermissionRequirementStrings = []string{"event.emit"}
	config := DefaultTaskRuntimeConfig()
	config.SourceHostPermissionGuard = NewSourceTaskHostPermissionGuard(taskPermissionEvaluatorFunc(nil), nil)
	config.SourceHostCapabilities = SourceTaskCapabilities{EmitEvent: true}
	var contracts []event.EventTypeDefinition
	config.SourceEventContracts = func(context.Context, *TaskDefinition) ([]event.EventTypeDefinition, error) { return contracts, nil }
	service := NewTaskRuntimeService(nil, config)
	if err := service.validateSourceHostCapabilities(t.Context(), definition); err == nil {
		t.Fatal("task accepted before Source contract existed")
	}
	contracts = []event.EventTypeDefinition{{EventTypeID: "extension.extension.updated", Version: 1}}
	if err := service.validateSourceHostCapabilities(t.Context(), definition); err == nil {
		t.Fatal("unverified Source contract accepted")
	}
	contracts[0].DefinitionHash = contracts[0].Hash()
	if err := service.validateSourceHostCapabilities(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	contracts[0].EventTypeID = "extension.foreign.updated"
	contracts[0].DefinitionHash = contracts[0].Hash()
	if err := service.validateSourceHostCapabilities(t.Context(), definition); err == nil {
		t.Fatal("foreign namespace accepted")
	}
}
