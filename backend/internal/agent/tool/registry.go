// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	applog "github.com/u-ai/backend/log"
)

var (
	tools      []Tool
	funcMap    = make(map[string]ToolCallFunc)
	memTools   []Tool
	memFuncMap = make(map[string]ToolCallFunc)
)

func Register(t Tool, fn ToolCallFunc) {
	tools = append(tools, t)
	funcMap[t.Function.Name] = fn
}

func RegisterMemory(t Tool, fn ToolCallFunc) {
	memTools = append(memTools, t)
	memFuncMap[t.Function.Name] = fn
}

func GetAll() []Tool {
	return tools
}

func GetMemoryTools() []Tool {
	return memTools
}

func Execute(name, argsJSON string) (string, bool) {
	result, ok := ExecuteWithContext(ToolExecutionContext{}, name, argsJSON)
	return result.Content, ok
}

func ExecuteWithContext(ctx ToolExecutionContext, name, argsJSON string) (ToolCallResult, bool) {
	return ExecuteWithContextAndCancel(callContextFromExecutionContext(ctx), ctx, name, argsJSON)
}

func ExecuteWithContextAndCancel(callCtx context.Context, execCtx ToolExecutionContext, name, argsJSON string) (ToolCallResult, bool) {
	if callCtx == nil {
		callCtx = context.Background()
	}
	trace := traceFromExecutionContext(execCtx)
	applog.TraceInfo(trace.WithStage("tool_execute_started"), applog.Fields{
		"tool_name":    name,
		"tool_call_id": execCtx.ToolCallID,
		"args_size":    len(argsJSON),
	}, "tool execute started")
	if err := callCtx.Err(); err != nil {
		applog.TraceWarn(trace.WithStage("tool_execute_cancelled"), applog.Fields{
			"tool_name":    name,
			"tool_call_id": execCtx.ToolCallID,
		}, "tool execute cancelled")
		return CancelledResult(err.Error()), true
	}
	fn, ok := funcMap[name]
	if !ok {
		fn, ok = memFuncMap[name]
	}
	if !ok {
		applog.TraceWarn(trace.WithStage("tool_execute_missing"), applog.Fields{
			"tool_name":    name,
			"tool_call_id": execCtx.ToolCallID,
		}, "tool execute missing")
		return ErrorResult("tool_not_found", "tool not found: "+name), false
	}
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			applog.TraceError(trace.WithStage("tool_execute_args_invalid"), applog.Fields{
				"tool_name":    name,
				"tool_call_id": execCtx.ToolCallID,
			}, err, "tool execute args invalid")
			return ErrorResult("args_parse_error", "args parse error: "+err.Error()), false
		}
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	if execCtx.IdempotencyKey == "" {
		execCtx.IdempotencyKey = defaultIdempotencyKey(execCtx, name, argsJSON)
	}
	intentID := recordToolCallIntent(execCtx, name, argsJSON)
	result := fn(callCtx, execCtx, args)
	result = normalizeResult(result, execCtx)
	recordToolCallResult(intentID, execCtx, name, result)
	applog.TraceInfo(trace.WithStage("tool_execute_completed"), applog.Fields{
		"tool_name":    name,
		"tool_call_id": execCtx.ToolCallID,
		"status":       string(result.Status),
		"error_code":   result.ErrorCode,
		"result_size":  len(result.VisibleText),
	}, "tool execute completed")
	return result, true
}

func ExecuteMemory(name, argsJSON string) (string, bool) {
	result, ok := ExecuteMemoryWithContext(ToolExecutionContext{}, name, argsJSON)
	return result.Content, ok
}

func ExecuteMemoryWithContext(ctx ToolExecutionContext, name, argsJSON string) (ToolCallResult, bool) {
	return ExecuteMemoryWithContextAndCancel(callContextFromExecutionContext(ctx), ctx, name, argsJSON)
}

func ExecuteMemoryWithContextAndCancel(callCtx context.Context, execCtx ToolExecutionContext, name, argsJSON string) (ToolCallResult, bool) {
	if callCtx == nil {
		callCtx = context.Background()
	}
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error()), true
	}
	fn, ok := memFuncMap[name]
	if !ok {
		return ErrorResult("tool_not_found", "tool not found: "+name), false
	}
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrorResult("args_parse_error", "args parse error: "+err.Error()), false
		}
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	if execCtx.IdempotencyKey == "" {
		execCtx.IdempotencyKey = defaultIdempotencyKey(execCtx, name, argsJSON)
	}
	intentID := recordToolCallIntent(execCtx, name, argsJSON)
	result := fn(callCtx, execCtx, args)
	result = normalizeResult(result, execCtx)
	recordToolCallResult(intentID, execCtx, name, result)
	return result, true
}

func callContextFromExecutionContext(execCtx ToolExecutionContext) context.Context {
	if execCtx.Context != nil {
		return execCtx.Context
	}
	return context.Background()
}

func defaultIdempotencyKey(ctx ToolExecutionContext, name, argsJSON string) string {
	sum := sha256.Sum256([]byte(ctx.RequestID + "|" + ctx.ConversationID + "|" + ctx.CharacterID + "|" + ctx.ToolCallID + "|" + name + "|" + argsJSON))
	return hex.EncodeToString(sum[:])
}

func normalizeResult(result ToolCallResult, ctx ToolExecutionContext) ToolCallResult {
	if result.Status == "" {
		result.Status = ToolStatusSuccess
	}
	if result.VisibleText == "" {
		result.VisibleText = result.Content
	}
	if result.IdempotencyKey == "" {
		result.IdempotencyKey = ctx.IdempotencyKey
	}
	if result.Confidence == 0 && result.Status == ToolStatusSuccess {
		result.Confidence = 1
	}
	return result
}

func recordToolCallIntent(ctx ToolExecutionContext, name, argsJSON string) string {
	if toolDB == nil {
		return ""
	}
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := toolDB.Exec(
		"INSERT INTO tool_call_intents (id, request_id, conversation_id, character_id, channel, tool_call_id, tool_name, args_json, idempotency_key, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', ?, ?)",
		id, ctx.RequestID, ctx.ConversationID, ctx.CharacterID, ctx.Channel, ctx.ToolCallID, name, argsJSON, ctx.IdempotencyKey, now, now,
	)
	if err != nil {
		applog.TraceError(traceFromExecutionContext(ctx).WithStage("tool_intent_persist_failed"), applog.Fields{
			"tool_name":    name,
			"tool_call_id": ctx.ToolCallID,
			"args_size":    len(argsJSON),
		}, err, "tool intent persist failed")
		return ""
	}
	applog.TraceInfo(traceFromExecutionContext(ctx).WithStage("tool_intent_persisted"), applog.Fields{
		"tool_name":    name,
		"tool_call_id": ctx.ToolCallID,
		"intent_id":    id,
		"args_size":    len(argsJSON),
	}, "tool intent persisted")
	return id
}

func recordToolCallResult(intentID string, ctx ToolExecutionContext, name string, result ToolCallResult) {
	if toolDB == nil {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	sideEffects, _ := json.Marshal(result.SideEffects)
	audit, _ := json.Marshal(result.Audit)
	resultID := uuid.New().String()
	status := string(result.Status)
	_, _ = toolDB.Exec(
		"INSERT INTO tool_call_results (id, intent_id, request_id, conversation_id, character_id, channel, tool_call_id, tool_name, status, content, error_code, visible_text, side_effects_json, external_operation_id, idempotency_key, audit_json, confidence, force_voice, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		resultID, nullableString(intentID), ctx.RequestID, ctx.ConversationID, ctx.CharacterID, ctx.Channel, ctx.ToolCallID, name, status, result.Content, result.ErrorCode, result.VisibleText, string(sideEffects), result.ExternalOperationID, result.IdempotencyKey, string(audit), result.Confidence, boolInt(result.ForceVoice), now,
	)
	if intentID != "" {
		_, _ = toolDB.Exec("UPDATE tool_call_intents SET status = ?, updated_at = ? WHERE id = ?", status, now, intentID)
	}
	applog.TraceInfo(traceFromExecutionContext(ctx).WithStage("tool_result_persisted"), applog.Fields{
		"tool_name":    name,
		"tool_call_id": ctx.ToolCallID,
		"intent_id":    intentID,
		"result_id":    resultID,
		"status":       status,
		"error_code":   result.ErrorCode,
		"force_voice":  result.ForceVoice,
	}, "tool result persisted")
}

func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func traceFromExecutionContext(ctx ToolExecutionContext) applog.TraceFields {
	path := ctx.Path
	if path == "" {
		path = "agent.tool.execute"
	}
	stateVersion := ctx.StateVersion
	if stateVersion == "" {
		stateVersion = "chat-runtime-trace-v1"
	}
	return applog.TraceFields{
		RequestID:     ctx.RequestID,
		CorrelationID: ctx.CorrelationID,
		CausationID:   ctx.CausationID,
		User:          ctx.User,
		Character:     ctx.CharacterID,
		Conversation:  ctx.ConversationID,
		Channel:       ctx.Channel,
		StateVersion:  stateVersion,
		Path:          path,
	}
}
