// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	applog "github.com/u-ai/backend/log"
)

const (
	HeaderRequestID   = "X-Request-Id"
	HeaderCorrelation = "X-Correlation-Id"
	HeaderCausation   = "X-Causation-Id"
)

const (
	CtxKeyRequestID     = "trace_request_id"
	CtxKeyCorrelationID = "trace_correlation_id"
	CtxKeyCausationID   = "trace_causation_id"
	CtxKeyTracePath     = "trace_path"
)

func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
			c.Request.Header.Set(HeaderRequestID, requestID)
		}

		correlationID := c.GetHeader(HeaderCorrelation)
		if correlationID == "" {
			correlationID = requestID
			c.Request.Header.Set(HeaderCorrelation, correlationID)
		}

		causationID := c.GetHeader(HeaderCausation)
		if causationID == "" {
			causationID = uuid.New().String()
			c.Request.Header.Set(HeaderCausation, causationID)
		}

		path := c.Request.Method + " " + c.Request.URL.Path

		c.Set(CtxKeyRequestID, requestID)
		c.Set(CtxKeyCorrelationID, correlationID)
		c.Set(CtxKeyCausationID, causationID)
		c.Set(CtxKeyTracePath, path)

		c.Header(HeaderRequestID, requestID)

		ctx := c.Request.Context()
		ctx = applog.CtxWithTrace(ctx, applog.TraceFields{
			RequestID:     requestID,
			CorrelationID: correlationID,
			CausationID:   causationID,
			Path:          path,
			Stage:         "request_begin",
		})
		c.Request = c.Request.WithContext(ctx)

		start := time.Now()

		applog.TraceInfo(applog.TraceFields{
			RequestID:     requestID,
			CorrelationID: correlationID,
			CausationID:   causationID,
			Path:          path,
			Stage:         "request_begin",
		}, nil, "请求开始")

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		applog.TraceInfo(applog.TraceFields{
			RequestID:     requestID,
			CorrelationID: correlationID,
			CausationID:   causationID,
			Path:          path,
			Stage:         "request_end",
		}, applog.Fields{
			"status":  statusCode,
			"latency": latency.String(),
		}, "请求结束")
	}
}
