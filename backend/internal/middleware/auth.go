// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/u-ai/backend/config"
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

		auth := c.GetHeader("Authorization")
		if len(auth) <= 7 || auth[:7] != "Bearer " {
			util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
			c.Abort()
			return
		}

		tokenStr := auth[7:]
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

		c.Set("userId", claims.UserId)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func GetUserID(c *gin.Context) int {
	if v, exists := c.Get("userId"); exists {
		if id, ok := v.(int); ok {
			return id
		}
	}
	return 0
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
