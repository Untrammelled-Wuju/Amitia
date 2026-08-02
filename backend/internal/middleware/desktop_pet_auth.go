// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"strings"

	"github.com/gin-gonic/gin"

	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func handleAuthFailure(c *gin.Context, authErr error) {
	errMsg := authErr.Error()
	if strings.Contains(errMsg, "expired") {
		util.ErrorResponse(c, response.TokenExpired, "令牌已过期，请重新登录", gin.H{"errorCode": "TOKEN_EXPIRED"})
	} else if strings.Contains(errMsg, "bearer token required") || strings.Contains(errMsg, "local token required") {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", gin.H{"errorCode": "TOKEN_REQUIRED"})
	} else if strings.Contains(errMsg, "invalid token") {
		util.ErrorResponse(c, response.InvalidToken, "令牌无效", gin.H{"errorCode": "TOKEN_INVALID"})
	} else if strings.Contains(errMsg, "misconfigured") {
		util.ErrorResponse(c, response.InternalError, "服务配置错误", gin.H{"errorCode": "SERVER_MISCONFIGURED"})
	} else {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_FAILED"})
	}
	c.Abort()
}

func extractBearerToken(auth string) string {
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return auth
}

func getRequestID(c *gin.Context) string {
	if v, ok := c.Get("requestID"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return generateRequestID()
}

func isLoopbackRequest(c *gin.Context) bool {
	host, _, found := strings.Cut(c.Request.RemoteAddr, ":")
	if !found {
		host = c.Request.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func isPublicPetPath(path string) bool {
	publicPaths := []string{
		"/api/health",
		"/api/desktop-pets/status",
	}
	for _, pp := range publicPaths {
		if path == pp || strings.HasPrefix(path, pp+"/") {
			return true
		}
	}
	return false
}

func GetActorFromContext(c *gin.Context) (*desktoppetAuth.ActorContext, error) {
	v, ok := c.Get("actorContext")
	if !ok || v == nil {
		return nil, errors.New("no actor context")
	}
	actor, ok := v.(*desktoppetAuth.ActorContext)
	if !ok {
		return nil, errors.New("invalid actor context type")
	}
	return actor, nil
}

func ResolveActorID(c *gin.Context) (string, error) {
	actor, err := GetActorFromContext(c)
	if err != nil {
		return "", err
	}
	return actor.UserID, nil
}

func isValidCorrelationID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func EnsureCorrelationID(c *gin.Context) string {
	corrID := c.GetHeader("X-Correlation-ID")
	if !isValidCorrelationID(corrID) {
		corrID = generateRequestID()
	}
	c.Header("X-Correlation-ID", corrID)
	return corrID
}
