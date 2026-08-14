// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/u-ai/backend/config"
	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type JWTClaims struct {
	UserId   int    `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

var publicPathPrefixes = []string{
	"/api/auth/login",
	"/api/auth/setup",
	"/api/auth/status",
	"/api/health",
	"/api/onboarding/status",
	"/api/onboarding/complete",
	"/api/tts/voices",
}

func isPublicPath(path string) bool {
	for _, prefix := range publicPathPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func AuthMiddleware() gin.HandlerFunc {
	secret := config.AppCfg.JWT.Secret
	return func(c *gin.Context) {
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		tokenStr := ""
		auth := c.GetHeader("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			tokenStr = auth[7:]
		}

		if tokenStr == "" {
			util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
			c.Abort()
			return
		}
		token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})
		if err != nil {
			if strings.Contains(err.Error(), "token is expired") {
				util.ErrorResponse(c, response.TokenExpired, "令牌已过期，请重新登录", nil)
			} else {
				util.ErrorResponse(c, response.InvalidToken, "令牌无效", nil)
			}
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*JWTClaims)
		if !ok || !token.Valid {
			util.ErrorResponse(c, response.InvalidToken, "令牌无效", nil)
			c.Abort()
			return
		}

		actorType := desktoppetAuth.ActorTypeUser
		roles := []string{"user"}
		permissions := desktoppetAuth.DefaultUserPermissions()
		if claims.Role == "admin" {
			actorType = desktoppetAuth.ActorTypeAdmin
			roles = []string{"admin"}
			permissions = desktoppetAuth.AdminPermissions()
		}

		actor := &desktoppetAuth.ActorContext{
			ActorType:   actorType,
			UserID:      runtimeidentity.UserID(fmt.Sprintf("%d", claims.UserId)),
			Roles:       roles,
			Permissions: permissions,
			AuthMethod:  "jwt",
		}

		ctx := desktoppetAuth.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		c.Set("actorContext", actor)
		c.Set("userId", fmt.Sprintf("%d", claims.UserId))
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	if v, exists := c.Get("userId"); exists {
		if s, ok := v.(string); ok {
			return s
		}
		if i, ok := v.(int); ok {
			return fmt.Sprintf("%d", i)
		}
	}
	return ""
}

func GetUsername(c *gin.Context) string {
	if v, exists := c.Get("username"); exists {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func GetRole(c *gin.Context) string {
	if v, exists := c.Get("role"); exists {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
