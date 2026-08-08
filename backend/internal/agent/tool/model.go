// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`

	rawSchema json.RawMessage `json:"-"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolExecutionContext struct {
	Context        context.Context `json:"-"`
	ConversationID string
	CharacterID    string
	Channel        string
	RequestID      string
	CorrelationID  string
	CausationID    string
	User           string
	StateVersion   string
	Path           string
	ToolCallID     string
	IdempotencyKey string
}

type ToolStatus string

const (
	ToolStatusSuccess        ToolStatus = "SUCCESS"
	ToolStatusFailed         ToolStatus = "FAILED"
	ToolStatusCancelled      ToolStatus = "CANCELLED"
	ToolStatusUnknown        ToolStatus = "UNKNOWN"
	ToolStatusPartialSuccess ToolStatus = "PARTIAL_SUCCESS"
)

type ToolSideEffect struct {
	Type      string `json:"type"`
	TargetID  string `json:"target_id,omitempty"`
	Confirmed bool   `json:"confirmed"`
}

type ToolCallResult struct {
	Status              ToolStatus             `json:"status"`
	Content             string                 `json:"content"`
	ErrorCode           string                 `json:"error_code,omitempty"`
	VisibleText         string                 `json:"visible_text,omitempty"`
	SideEffects         []ToolSideEffect       `json:"side_effects,omitempty"`
	ExternalOperationID string                 `json:"external_operation_id,omitempty"`
	IdempotencyKey      string                 `json:"idempotency_key,omitempty"`
	Audit               map[string]interface{} `json:"audit,omitempty"`
	Confidence          float64                `json:"confidence"`
	ForceVoice          bool                   `json:"force_voice"`
}

func TextResult(content string) ToolCallResult {
	return ToolCallResult{Status: ToolStatusSuccess, Content: content, VisibleText: content, Confidence: 1}
}

func ErrorResult(code, content string) ToolCallResult {
	return ToolCallResult{Status: ToolStatusFailed, Content: content, ErrorCode: code, VisibleText: content, Confidence: 1}
}

func CancelledResult(content string) ToolCallResult {
	return ToolCallResult{Status: ToolStatusCancelled, Content: content, ErrorCode: "cancelled", VisibleText: content}
}

func UnknownResult(code, content string) ToolCallResult {
	return ToolCallResult{Status: ToolStatusUnknown, Content: content, ErrorCode: code, VisibleText: content}
}

type ToolCallFunc func(ctx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult

func requireScopedWrite(execCtx ToolExecutionContext) (ToolExecutionContext, *ToolCallResult) {
	execCtx.CharacterID = strings.TrimSpace(execCtx.CharacterID)
	execCtx.ConversationID = strings.TrimSpace(execCtx.ConversationID)
	execCtx.Channel = strings.TrimSpace(execCtx.Channel)
	if execCtx.CharacterID == "" {
		result := ErrorResult("missing_character_scope", "ERROR: character scope is required")
		result.Audit = map[string]interface{}{"conversation_id": execCtx.ConversationID, "channel": execCtx.Channel}
		return execCtx, &result
	}
	if execCtx.ConversationID == "" {
		result := ErrorResult("missing_conversation_scope", "ERROR: conversation scope is required")
		result.Audit = map[string]interface{}{"character_id": execCtx.CharacterID, "channel": execCtx.Channel}
		return execCtx, &result
	}
	return execCtx, nil
}

func ParseParametersSchema(raw json.RawMessage) (Parameters, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Parameters{}, fmt.Errorf("empty parameters schema")
	}

	type plain Parameters

	var decoded plain

	if err := json.Unmarshal(raw, &decoded); err != nil {
		return Parameters{}, err
	}

	result := Parameters(decoded)

	result.rawSchema = append(json.RawMessage(nil), raw...)

	return result, nil
}

func (p Parameters) MarshalJSON() ([]byte, error) {
	if len(p.rawSchema) > 0 {
		if !json.Valid(p.rawSchema) {
			return nil, fmt.Errorf("invalid raw parameter schema")
		}

		return append([]byte(nil), p.rawSchema...), nil
	}

	type plain Parameters

	return json.Marshal(plain(p))
}
