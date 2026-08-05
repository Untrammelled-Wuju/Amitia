// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type AuthConfig struct {
	Mode                     string
	JWTSecret                string
	JWTIssuer                string
	JWTAudience              string
	LocalCredentials         *LocalCredentialStore
	LocalUserID              string
	ListenAddress            string
	AllowedOrigins           []string
	SessionService           *DesktopSessionService
	DesktopInstanceValidator func(string) bool
}

type JWTClaims struct {
	UserId   int    `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

const (
	AuthMethodJWT             = "jwt"
	AuthMethodLocalToken      = "local_token"
	AuthMethodDesktopSession  = "desktop_session"
	AuthMethodLocalAdminToken = "local_admin_token"
)

var maintenanceAllowedPaths = map[string]bool{
	"/api/health":    true,
	"/api/doctor":    true,
	"/api/migration": true,
	"/api/backup":    true,
	"/api/export":    true,
}

var ErrNotFound = errors.New("not found")

func isMaintenanceAllowedPath(path string) bool {
	for prefix := range maintenanceAllowedPaths {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func isLoopback(addr string) bool {
	if strings.TrimSpace(addr) == "" {
		return false
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	} else if port == "" {
		return false
	}

	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateOrigin(c *gin.Context, allowedOrigins []string) error {
	origin := c.GetHeader("Origin")
	if origin == "" {
		return nil
	}

	for _, allowed := range allowedOrigins {
		if allowed == origin {
			return nil
		}
	}
	return errors.New("origin not allowed")
}

func AuthenticationMiddleware(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch cfg.Mode {
		case "network":
			handleNetworkAuth(c, cfg)
		case "local_single_user":
			handleLocalSingleUserAuth(c, cfg)
		case "maintenance":
			handleMaintenanceAuth(c, cfg)
		default:
			util.ErrorResponse(c, response.Unauthorized, "未知的认证模式", nil)
			c.Abort()
		}
	}
}

func LocalAdminAuthenticationMiddleware(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Mode != "local_single_user" {
			util.ErrorResponse(c, response.Unauthorized, "本地管理接口仅允许本地模式", nil)
			c.Abort()
			return
		}

		if !isLoopback(c.Request.RemoteAddr) || !isLoopback(cfg.ListenAddress) {
			util.ErrorResponse(c, response.Unauthorized, "本地管理接口仅允许回环访问", nil)
			c.Abort()
			return
		}

		if err := validateOrigin(c, cfg.AllowedOrigins); err != nil {
			util.ErrorResponse(c, response.Unauthorized, "来源不允许", nil)
			c.Abort()
			return
		}

		instanceID := strings.TrimSpace(c.GetHeader("X-Amitia-Desktop-Instance"))
		if instanceID == "" || cfg.DesktopInstanceValidator == nil || !cfg.DesktopInstanceValidator(instanceID) {
			util.ErrorResponse(c, response.Unauthorized, "桌面实例无效", nil)
			c.Abort()
			return
		}

		token := strings.TrimSpace(c.GetHeader("X-Amitia-Local-Token"))
		if token == "" || cfg.LocalCredentials == nil || !cfg.LocalCredentials.Validate(token) {
			util.ErrorResponse(c, response.Unauthorized, "本地管理凭据无效", nil)
			c.Abort()
			return
		}

		actor := buildAdminActor(c, AuthMethodLocalAdminToken)
		actor.IsLocalTrusted = true
		applyActorToContext(c, actor)
	}
}

func handleNetworkAuth(c *gin.Context, cfg AuthConfig) {
	tokenStr := extractBearerToken(c)
	if tokenStr == "" {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
		c.Abort()
		return
	}

	actor, err := parseAndValidateJWT(tokenStr, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
	if err != nil {
		util.ErrorResponse(c, response.InvalidToken, "令牌无效", nil)
		c.Abort()
		return
	}
	actor.CorrelationID = sanitizeCorrelationID(c.GetHeader("X-Request-ID"))

	applyActorToContext(c, actor)
}

func handleLocalSingleUserAuth(c *gin.Context, cfg AuthConfig) {
	if !isLoopback(c.Request.RemoteAddr) || !isLoopback(cfg.ListenAddress) {
		util.ErrorResponse(c, response.Unauthorized, "本地单用户模式仅允许回环访问", nil)
		c.Abort()
		return
	}

	if err := validateOrigin(c, cfg.AllowedOrigins); err != nil {
		util.ErrorResponse(c, response.Unauthorized, "来源不允许", nil)
		c.Abort()
		return
	}

	sessionToken := c.GetHeader("X-Amitia-Desktop-Session")
	if sessionToken != "" && cfg.SessionService != nil {
		session, err := cfg.SessionService.ValidateSessionWithContext(c.Request.Context(), sessionToken)
		if err == nil {
			_ = cfg.SessionService.TouchSessionWithContext(c.Request.Context(), session)
			_ = cfg.SessionService.RenewSessionWithContext(c.Request.Context(), session)
			actor := buildDesktopSessionActor(session, cfg)
			actor.CorrelationID = sanitizeCorrelationID(c.GetHeader("X-Request-ID"))
			applyActorToContext(c, actor)
			return
		}
	}

	token := c.GetHeader("X-Amitia-Local-Token")
	if token == "" {
		token = c.GetHeader("Authorization")
		if strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}
	}

	if token != "" && cfg.LocalCredentials != nil && cfg.LocalCredentials.Validate(token) {
		actor := buildLocalUserActor(cfg)
		actor.CorrelationID = sanitizeCorrelationID(c.GetHeader("X-Request-ID"))
		applyActorToContext(c, actor)
		return
	}

	jwtStr := extractBearerToken(c)
	if jwtStr != "" {
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(jwtStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(cfg.JWTSecret), nil
		}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
		if err == nil && token.Valid && claims.UserId > 0 {
			actor := &auth.ActorContext{
				ActorType:      auth.ActorTypeUser,
				UserID:         fmt.Sprintf("%d", claims.UserId),
				Roles:          []string{claims.Role},
				AuthMethod:     "jwt",
				IsLocalTrusted: true,
			}
			applyActorToContext(c, actor)
			return
		}
	}

	util.ErrorResponse(c, response.Unauthorized, "本地令牌无效", nil)
	c.Abort()
}

func handleMaintenanceAuth(c *gin.Context, cfg AuthConfig) {
	if !isMaintenanceAllowedPath(c.Request.URL.Path) {
		util.ErrorResponse(c, response.Unauthorized, "维护模式仅允许访问管理接口", nil)
		c.Abort()
		return
	}

	if !isLoopback(c.Request.RemoteAddr) {
		util.ErrorResponse(c, response.Unauthorized, "维护模式仅允许回环访问", nil)
		c.Abort()
		return
	}

	token := c.GetHeader("X-Amitia-Local-Token")
	if token != "" && cfg.LocalCredentials != nil && cfg.LocalCredentials.Validate(token) {
		actor := buildAdminActor(c, AuthMethodLocalToken)
		applyActorToContext(c, actor)
		return
	}

	tokenStr := extractBearerToken(c)
	if tokenStr != "" {
		actor, err := parseAndValidateJWT(tokenStr, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
		if err == nil {
			applyActorToContext(c, actor)
			return
		}
	}

	util.ErrorResponse(c, response.Unauthorized, "需要有效凭据", nil)
	c.Abort()
}

func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && strings.HasPrefix(authHeader, "Bearer ") {
		return authHeader[7:]
	}
	return ""
}

func parseAndValidateJWT(tokenStr, secret, issuer, audience string) (*auth.ActorContext, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	},
		jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
	)

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	if claims.UserId <= 0 {
		return nil, errors.New("invalid user id")
	}

	actorType := auth.ActorTypeUser
	roles := []string{"user"}
	permissions := auth.DefaultUserPermissions()
	if claims.Role == "admin" {
		actorType = auth.ActorTypeAdmin
		roles = []string{"admin"}
		permissions = auth.AdminPermissions()
	}

	return &auth.ActorContext{
		ActorType:     actorType,
		UserID:        fmt.Sprintf("%d", claims.UserId),
		Roles:         roles,
		Permissions:   permissions,
		AuthMethod:    AuthMethodJWT,
		RequestID:     generateRequestID(),
		CorrelationID: "",
	}, nil
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func buildLocalUserActor(cfg AuthConfig) *auth.ActorContext {
	userID := strings.TrimSpace(cfg.LocalUserID)
	if userID == "" {
		userID = "local_user"
	}

	return &auth.ActorContext{
		ActorType:      auth.ActorTypeLocalUser,
		UserID:         userID,
		Roles:          []string{"local_user", "user"},
		Permissions:    append(auth.DefaultUserPermissions(), auth.PermDesktopPetRepair),
		AuthMethod:     AuthMethodLocalToken,
		RequestID:      generateRequestID(),
		CorrelationID:  "",
		IsLocalTrusted: true,
	}
}

func buildDesktopSessionActor(session *DesktopSession, cfg AuthConfig) *auth.ActorContext {
	perms := auth.DefaultUserPermissions()
	perms = append(perms, auth.PermDesktopPetRepair)

	return &auth.ActorContext{
		ActorType:      auth.ActorTypeLocalUser,
		UserID:         session.UserID,
		Roles:          []string{"local_user", "user"},
		Permissions:    perms,
		AuthMethod:     AuthMethodDesktopSession,
		SessionID:      session.ID,
		RequestID:      generateRequestID(),
		CorrelationID:  "",
		IsLocalTrusted: true,
	}
}

func buildAdminActor(c *gin.Context, authMethod string) *auth.ActorContext {
	return &auth.ActorContext{
		ActorType:      auth.ActorTypeAdmin,
		UserID:         "local_admin",
		Roles:          []string{"admin"},
		Permissions:    auth.AdminPermissions(),
		AuthMethod:     authMethod,
		RequestID:      generateRequestID(),
		CorrelationID:  sanitizeCorrelationID(c.GetHeader("X-Request-ID")),
		IsLocalTrusted: true,
	}
}

func applyActorToContext(c *gin.Context, actor *auth.ActorContext) {
	c.Set("actorContext", actor)
	c.Set("userId", actor.UserID)
	c.Set("username", actor.UserID)
	c.Set("role", actor.Roles[0])
	ctx := auth.WithActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func generateRequestID() string {
	id, err := NewRequestID()
	if err != nil {
		panic(fmt.Sprintf("request ID generation failed: %v", err))
	}
	return id
}

func NewRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "req_" + base64.RawURLEncoding.EncodeToString(b), nil
}

var correlationIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func sanitizeCorrelationID(raw string) string {
	if correlationIDPattern.MatchString(raw) {
		return raw
	}
	id, err := NewRequestID()
	if err != nil {
		return ""
	}
	return id
}

func GetActor(c *gin.Context) *auth.ActorContext {
	if v, exists := c.Get("actorContext"); exists {
		if actor, ok := v.(*auth.ActorContext); ok {
			return actor
		}
	}
	return nil
}

func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil || !actor.HasPermission(perm) {
			util.ErrorResponse(c, response.Forbidden, "权限不足", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
