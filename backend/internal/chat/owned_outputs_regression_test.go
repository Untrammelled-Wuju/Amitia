package chat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedOutputFixture struct{ outputs []MessageOutput }

func (*ownedOutputFixture) Tools(context.Context, business.Inference) ([]tool.Tool, error) {
	return nil, nil
}
func (*ownedOutputFixture) Execute(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error) {
	panic("unexpected tool call")
}
func (*ownedOutputFixture) PrepareOwnedPrompt(context.Context, business.Inference) (string, func(), error) {
	return "", func() {}, nil
}
func (p *ownedOutputFixture) PlanOwnedOutputs(context.Context, business.Inference, business.Generation) ([]MessageOutput, error) {
	return p.outputs, nil
}
func (*ownedOutputFixture) OwnedReplyCommitted(context.Context, business.Inference, business.Generation, string) error {
	return nil
}

func TestOwnedCommitPreservesGenericAndToolMediaOutputs(t *testing.T) {
	for _, kind := range []string{"image", "audio", "video", "file"} {
		t.Run(kind, func(t *testing.T) {
			f := newOwnedExecutionFixture(t)
			f.model.llmWithTools = ownedFixedReply("正文")
			output := MessageOutput{OutputID: "output", ExtensionID: "example", Part: MessagePart{Type: kind, URL: "/api/extension/resources/example", AltText: "媒体", MIMEType: "application/octet-stream"}}
			f.model.ownedToolRuntime = &ownedOutputFixture{outputs: []MessageOutput{output}}
			response, err := f.engine.Run(t.Context(), f.request(kind))
			if err != nil || !response.Saved {
				t.Fatalf("media save failed: %+v %v", response, err)
			}
			var decoded []MessageOutput
			if err = json.Unmarshal(response.Outputs, &decoded); err != nil || len(decoded) != 1 || decoded[0].Part.Type != kind {
				t.Fatalf("media lost: %s %v", response.Outputs, err)
			}
			rows, err := f.port.Snapshot(t.Context(), response.Scope, coordination.DataQuery{})
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows.Resources {
				if row.Kind == "message" && row.ID != response.RequestID+"/user" {
					var item struct {
						Outputs []MessageOutput `json:"outputs"`
					}
					if err = json.Unmarshal(row.Body, &item); err != nil || len(item.Outputs) != 1 {
						t.Fatalf("owner media missing: %s %v", row.Body, err)
					}
				}
			}
		})
	}
}
