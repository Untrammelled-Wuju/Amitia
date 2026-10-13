package tool

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/business"
)

type OwnedMemoryRuntime interface {
	ExecuteMemory(context.Context, ToolExecutionContext, string, map[string]interface{}) (business.MemoryToolResponse, error)
}

var ownedMemoryRuntime OwnedMemoryRuntime

func SetOwnedMemoryRuntime(runtime OwnedMemoryRuntime) { ownedMemoryRuntime = runtime }

func executeOwnedMemoryTool(ctx context.Context, execution ToolExecutionContext, name string, args map[string]interface{}) ToolCallResult {
	if err := ctx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	var scopeError *ToolCallResult
	if name == "query_memory" || name == "get_memory_by_title" || name == "summarize_memories" {
		execution, scopeError = requireCharacterReadScope(execution)
	} else {
		execution, scopeError = requireScopedWrite(execution)
	}
	if scopeError != nil {
		return *scopeError
	}
	if ownedMemoryRuntime == nil {
		return ErrorResult("memory_service_not_initialized", "统一记忆服务尚未就绪")
	}
	response, err := ownedMemoryRuntime.ExecuteMemory(ctx, execution, name, args)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return CancelledResult(err.Error())
		}
		return ErrorResult("memory_operation_failed", err.Error())
	}
	result := TextResult(response.Text)
	result.Audit = response.Metadata
	if name != "query_memory" && name != "get_memory_by_title" && name != "summarize_memories" && !response.Saved {
		return ErrorResult("memory_save_unconfirmed", "记忆操作尚未获得所有者保存确认")
	}
	if response.Saved {
		if response.Acknowledgement.OwnerID == "" || response.Acknowledgement.RequestID == "" {
			return ErrorResult("memory_save_unconfirmed", "记忆保存尚未获得所有者确认")
		}
		for _, resource := range response.Resources {
			if response.Acknowledgement.Versions[resource.Kind+"/"+resource.ID] != resource.Revision {
				return ErrorResult("memory_save_unconfirmed", "记忆保存版本尚未确认")
			}
			result.SideEffects = append(result.SideEffects, ToolSideEffect{Type: resource.Kind + "_saved", TargetID: resource.ID, Confirmed: true})
			if resource.Kind == "memory" || resource.Kind == "checkpoint" {
				result.ExternalOperationID = resource.ID
			}
		}
	}
	return result
}
