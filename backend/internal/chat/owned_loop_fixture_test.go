package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	coreexec "github.com/u-ai/backend/internal/execution"
	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

type ownedLoopTestRuntime struct {
	runtime     ownedFixtureToolRuntime
	definitions []tool.Tool
}

type ownedFixtureToolRuntime interface {
	ExecuteModelTool(context.Context,string,json.RawMessage,SkillScope,string)(ToolResult,bool)
}

func (a *ownedCitationAudit) captureToolResultFixture(_ context.Context,_ string,name,result,_ string,_ string,_ int64)error{a.captureCitationIDs(name,result);return nil}

func (r *ownedLoopTestRuntime) Tools(context.Context, business.Inference) ([]tool.Tool, error) {
	return r.definitions, nil
}
func (r *ownedLoopTestRuntime) Execute(ctx context.Context, inference business.Inference, id, name string, input json.RawMessage) (ToolResult, error) {
	if r.runtime == nil {
		return ToolResult{}, fmt.Errorf("unexpected tool call")
	}
	scope := SkillScope{SpaceID: inference.Scope.SpaceID, CharacterID: inference.Scope.RoleID, ConversationID: inference.ConversationID, RequestID: inference.Scope.RequestID, PermissionMode: inference.PermissionMode}
	result, found := r.runtime.ExecuteModelTool(ctx, name, input, scope, inference.ConversationID+":"+inference.Scope.RequestID+":"+id)
	if !found {
		return ToolResult{Status: "FAILED", Error: &ToolError{Code: "TOOL_NOT_FOUND"}}, nil
	}
	return result, nil
}

func (s *service) invokeOwnedLoopFixture(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, _ applog.TraceFields, _ *promptir.PromptTrace, _ string, convID, charID, _ string, requestID, spaceID, _ string, permissionMode string, exec *coreexec.ExecutionContext, definitions []tool.Tool, _ map[string]bool, _ context.Context, recovered *conversationstream.ActiveTurnState) (string, string, bool, int, int64, error) {
	runtime, ok := s.ownedToolRuntime.(*ownedLoopTestRuntime)
	if !ok {
		runtime = &ownedLoopTestRuntime{}
		s.ownedToolRuntime = runtime
	}
	runtime.definitions = definitions
	inference := business.Inference{ConversationID: convID, PermissionMode: permissionMode, Scope: coordination.ExecutionScope{SpaceID: spaceID, RoleID: charID, RequestID: requestID}}
	if exec != nil && exec.WorkspaceID != "" {
		inference.Workspaces = []business.ProjectWorkspace{{ID: exec.WorkspaceID}}
	}
	inference.RecoveredRuntime=recovered

	result, err := s.generateOwnedWithTools(ctx, inference, cfg, messages, &ownedReplySink{inference: inference})
	return result.Text, result.Reasoning, result.ForceVoice, result.Tokens, result.ReasoningDurationMS, err
}
