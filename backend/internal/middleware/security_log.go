// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SanitizedLogger struct {
	base *zap.Logger
}

func NewSanitizedLogger(base *zap.Logger) *SanitizedLogger {
	return &SanitizedLogger{base: base}
}

func (l *SanitizedLogger) LogRequest(c *gin.Context, duration time.Duration, status int) {
	fields := []zap.Field{
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.Int("status", status),
		zap.Duration("duration", duration),
		zap.String("ip", c.ClientIP()),
		zap.String("user-agent", c.Request.UserAgent()),
	}
	if actor, err := GetActorFromContext(c); err == nil {
		fields = append(fields, zap.String("actor", string(actor.UserID)))
		fields = append(fields, zap.String("actorType", string(actor.ActorType)))
		fields = append(fields, zap.String("requestID", actor.RequestID))
	}
	l.base.Info("request", fields...)
}

func SanitizeLogValue(key string, value string) string {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") || strings.Contains(lower, "jwt") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") || strings.Contains(lower, "authorization") {
		if len(value) > 8 {
			return value[:4] + "****" + value[len(value)-4:]
		}
		return "****"
	}
	if strings.Contains(lower, "prompt") || strings.Contains(lower, "message") || strings.Contains(lower, "content") || strings.Contains(lower, "receipt") {
		if len(value) > 32 {
			return value[:16] + "..." + value[len(value)-8:]
		}
	}
	return value
}

func InitRequestLogger(logger *zap.Logger) gin.HandlerFunc {
	sanitized := NewSanitizedLogger(logger)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		sanitized.LogRequest(c, duration, c.Writer.Status())
	}
}
