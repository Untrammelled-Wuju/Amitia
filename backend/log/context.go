// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package log

import (
	"context"

	"github.com/gin-gonic/gin"
)

type contextKey string

const traceContextKey contextKey = "trace_fields"

func CtxWithTrace(ctx context.Context, trace TraceFields) context.Context {
	return context.WithValue(ctx, traceContextKey, trace)
}

func FromContext(ctx context.Context) TraceFields {
	if tf, ok := ctx.Value(traceContextKey).(TraceFields); ok {
		return tf
	}
	return TraceFields{}
}

func FromGin(c *gin.Context) TraceFields {
	requestID, _ := c.Get("trace_request_id")
	correlationID, _ := c.Get("trace_correlation_id")
	causationID, _ := c.Get("trace_causation_id")
	path, _ := c.Get("trace_path")

	tf := TraceFields{}

	if v, ok := requestID.(string); ok {
		tf.RequestID = v
	}
	if v, ok := correlationID.(string); ok {
		tf.CorrelationID = v
	}
	if v, ok := causationID.(string); ok {
		tf.CausationID = v
	}
	if v, ok := path.(string); ok {
		tf.Path = v
	}

	return tf
}

func MergeTrace(original TraceFields, updates TraceFields) TraceFields {
	result := original.Clone()
	if updates.RequestID != "" {
		result.RequestID = updates.RequestID
	}
	if updates.CorrelationID != "" {
		result.CorrelationID = updates.CorrelationID
	}
	if updates.CausationID != "" {
		result.CausationID = updates.CausationID
	}
	if updates.User != "" {
		result.User = updates.User
	}
	if updates.Character != "" {
		result.Character = updates.Character
	}
	if updates.Conversation != "" {
		result.Conversation = updates.Conversation
	}
	if updates.Channel != "" {
		result.Channel = updates.Channel
	}
	if updates.StateVersion != "" {
		result.StateVersion = updates.StateVersion
	}
	if updates.Path != "" {
		result.Path = updates.Path
	}
	if updates.Stage != "" {
		result.Stage = updates.Stage
	}
	return result
}
