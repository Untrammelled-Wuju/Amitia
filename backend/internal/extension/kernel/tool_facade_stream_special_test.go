package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

type noopToolStreamSink struct{}

func (noopToolStreamSink) Emit(context.Context, capability.ToolStreamEvent) error {
	return nil
}

func TestExecuteModelToolStreamDispatchesAcquisitionTools(t *testing.T) {
	facade := NewToolFacade(nil, nil)
	facade.SetAcquisitionBridge(&acquisition.AgentCapabilityBridge{})

	result, found, err := facade.ExecuteModelToolStream(
		context.Background(),
		acquisition.FindCapabilitiesToolID,
		json.RawMessage(`{"capabilityId":`),
		InvocationScope{},
		"",
		noopToolStreamSink{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("acquisition tool was not dispatched")
	}
	if result.Error == nil || result.Error.Code != "INVALID_INPUT" {
		t.Fatalf("error = %#v, want INVALID_INPUT", result.Error)
	}
}
