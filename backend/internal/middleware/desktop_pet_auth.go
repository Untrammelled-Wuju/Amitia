// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/u-ai/backend/config"
	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/security"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type DesktopPetJWTClaims struct {
	UserId     int    `json:"userId"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	AuthMethod string `json:"authMethod"`
	jwt.RegisteredClaims
}

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func DesktopPetAuthMiddleware(securityCfg *security.SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicPetPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		requestID := generateRequestID()
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)

		actor, err := resolvePetActor(c, securityCfg)
		if err != nil {
			handleAuthFailure(c, err)
			return
		}

		ctx := desktoppetAuth.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		c.Set("actorContext", actor)
		c.Set("userId", actor.UserID)
		c.Set("role", actor.Roles)
		c.Next()
	}
}

func resolvePetActor(c *gin.Context, securityCfg *security.SecurityConfig) (*desktoppetAuth.ActorContext, error) {
	localToken := securityCfg.LocalToken
	headerToken := extractBearerToken(c.Query("token"))
	if headerToken == "" {
		headerToken = extractBearerToken(c.GetHeader("Authorization"))
	}

	// Local single user mode: token-based
	if securityCfg.Mode == security.SecurityModeLocalSingle || securityCfg.Mode == security.SecurityModeMaintenance {
		if headerToken != "" && headerToken == localToken {
			return &desktoppetAuth.ActorContext{
				ActorType:      desktoppetAuth.ActorTypeLocalUser,
				UserID:         "local",
				Roles:          []string{"local_user"},
				Permissions:    desktoppetAuth.DefaultUserPermissions(),
				AuthMethod:     "local_token",
				IsLocalTrusted: true,
				RequestID:      getRequestID(c),
			}, nil
		}
		if securityCfg.FailOpen {
			return &desktoppetAuth.ActorContext{
				ActorType:      desktoppetAuth.ActorTypeLocalUser,
				UserID:         "local",
				Roles:          []string{"local_user"},
				Permissions:    desktoppetAuth.DefaultUserPermissions(),
				AuthMethod:     "fail_open_local",
				IsLocalTrusted: true,
				RequestID:      getRequestID(c),
			}, nil
		}
		return nil, errors.New("unauthorized: local token required")
	}

	// Network mode: JWT required
	if headerToken == "" {
		return nil, errors.New("unauthorized: bearer token required")
	}

	secret := securityCfg.JWTSecret
	if secret == "" {
		if securityCfg.FailOpen {
			return &desktoppetAuth.ActorContext{
				ActorType:   desktoppetAuth.ActorTypeUser,
				UserID:      "default",
				Roles:       []string{"user"},
				Permissions: desktoppetAuth.DefaultUserPermissions(),
				AuthMethod:  "fail_open_no_secret",
				RequestID:   getRequestID(c),
			}, nil
		}
		return nil, errors.New("unauthorized: server misconfigured")
	}

	claims := DesktopPetJWTClaims{}
	token, err := jwt.ParseWithClaims(headerToken, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil || token == nil || !token.Valid {
		return nil, fmt.Errorf("unauthorized: invalid token: %w", err)
	}

	actorType := desktoppetAuth.ActorTypeUser
	permissions := desktoppetAuth.DefaultUserPermissions()
	roles := []string{"user"}
	if claims.Role == "admin" {
		actorType = desktoppetAuth.ActorTypeAdmin
		roles = []string{"admin"}
		permissions = desktoppetAuth.AdminPermissions()
	}

	return &desktoppetAuth.ActorContext{
		ActorType:      actorType,
		UserID:         fmt.Sprintf("%v", claims.UserId),
		DeviceID:       c.GetHeader("X-Device-ID"),
		Roles:          roles,
		Permissions:    permissions,
		AuthMethod:     "jwt",
		SessionID:      c.GetHeader("X-Session-ID"),
		CorrelationID:  c.GetHeader("X-Correlation-ID"),
		RequestID:      getRequestID(c),
		IsLocalTrusted: isLoopbackRequest(c),
	}, nil
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
	return host == "127.0.0.1" || host == "::1" || host == "localhost" || host == ""
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

func EnsureCorrelationID(c *gin.Context) string {
	corrID := c.GetHeader("X-Correlation-ID")
	if corrID == "" {
		corrID = generateRequestID()
	}
	correlationDeadline := time.Now().Format(time.RFC3339)
	c.Header("X-Correlation-ID", corrID)
	_ = correlationDeadline
	return corrID
}

func init() {
	_ = config.AppCfg
}
