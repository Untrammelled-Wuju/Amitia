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
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/accountsession"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
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
	AccountSessions          *accountsession.Validator
}

type AccessClaims = accountsession.AccessClaims

const (
	AuthMethodJWT             = "jwt"
	AuthMethodLocalToken      = "local_token"
	AuthMethodDesktopSession  = "desktop_session"
	AuthMethodLocalAdminToken = "local_admin_token"
)

var maintenanceAllowedPaths = map[string]bool{
	"/api/health":      true,
	"/api/doctor":      true,
	"/api/migration":   true,
	"/api/backup":      true,
	"/api/export":      true,
	"/api/maintenance": true,
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

func validateLocalOrigin(c *gin.Context, allowedOrigins []string) error {
	if err := validateOrigin(c, allowedOrigins); err == nil {
		return nil
	}
	if isDesktopDevelopmentOrigin(c.GetHeader("Origin")) {
		return nil
	}
	return errors.New("origin not allowed")
}

func isDesktopDevelopmentOrigin(origin string) bool {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || parsed.Scheme != "http" || parsed.Port() != "5178" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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

		if err := validateLocalOrigin(c, cfg.AllowedOrigins); err != nil {
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

	if err := validateJWTSecret(cfg.JWTSecret); err != nil {
		util.ErrorResponse(c, response.InternalError, "JWT 配置无效", nil)
		c.Abort()
		return
	}

	claims, err := parseAndValidateJWT(tokenStr, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
	if err != nil {
		util.ErrorResponse(c, response.InvalidToken, "令牌无效", nil)
		c.Abort()
		return
	}

	if cfg.AccountSessions == nil {
		util.ErrorResponse(c, response.InternalError, "会话验证服务未配置", nil)
		c.Abort()
		return
	}

	if claims.SessionID != "" {
		result := cfg.AccountSessions.ValidateAccessSession(claims.SessionID, claims.UserID)
		if !result.Valid {
			switch {
			case errors.Is(result.Reason, accountsession.ErrSessionRevoked):
				util.ErrorResponse(c, response.Unauthorized, "auth.session_revoked", nil)
			case errors.Is(result.Reason, accountsession.ErrSessionExpired):
				util.ErrorResponse(c, response.Unauthorized, "auth.session_expired", nil)
			default:
				util.ErrorResponse(c, response.Unauthorized, "auth.access_invalid", nil)
			}
			c.Abort()
			return
		}
		if result.Session != nil {
			if err := cfg.AccountSessions.TouchSession(result.Session.PublicID); err != nil {
				util.ErrorResponse(c, response.InternalError, "更新会话失败", nil)
				c.Abort()
				return
			}
		}
	}

	actorType := auth.ActorTypeUser
	roles := []string{"user"}
	permissions := auth.DefaultUserPermissions()
	if claims.Role == "admin" {
		actorType = auth.ActorTypeAdmin
		roles = []string{"admin"}
		permissions = auth.AdminPermissions()
	}

	actor := &auth.ActorContext{
		ActorType:     actorType,
		UserID:        runtimeidentity.UserID(fmt.Sprintf("%d", claims.UserID)),
		Roles:         roles,
		Permissions:   permissions,
		AuthMethod:    AuthMethodJWT,
		SessionID:     claims.SessionID,
		RequestID:     generateRequestID(),
		CorrelationID: sanitizeCorrelationID(c.GetHeader("X-Request-ID")),
	}
	applyActorToContext(c, actor)
}

var localPublicPathPrefixes = []string{
	"/api/auth/login",
	"/api/auth/setup",
	"/api/auth/status",
	"/api/health",
	"/api/health/circuit-breakers",
	"/api/onboarding/status",
	"/api/onboarding/complete",
	"/api/tts/voices",
	"/api/public",
}

func isLocalPublicPath(path string) bool {
	for _, prefix := range localPublicPathPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func handleLocalSingleUserAuth(c *gin.Context, cfg AuthConfig) {
	if isLocalPublicPath(c.Request.URL.Path) {
		c.Next()
		return
	}

	if !isLoopback(c.Request.RemoteAddr) || !isLoopback(cfg.ListenAddress) {
		util.ErrorResponse(c, response.Unauthorized, "本地单用户模式仅允许回环访问", nil)
		c.Abort()
		return
	}

	if err := validateLocalOrigin(c, cfg.AllowedOrigins); err != nil {
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
		claims, err := parseAndValidateJWT(jwtStr, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
		if err == nil && claims.UserID > 0 {
			actorType := auth.ActorTypeUser
			roles := []string{"user"}
			permissions := auth.DefaultUserPermissions()
			if claims.Role == "admin" {
				actorType = auth.ActorTypeAdmin
				roles = []string{"admin"}
				permissions = auth.AdminPermissions()
			}
			actor := &auth.ActorContext{
				ActorType:      actorType,
				UserID:         runtimeidentity.UserID(fmt.Sprintf("%d", claims.UserID)),
				Roles:          roles,
				Permissions:    permissions,
				AuthMethod:     AuthMethodJWT,
				SessionID:      claims.SessionID,
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
		claims, err := parseAndValidateJWT(tokenStr, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
		if err == nil {
			actor := buildAdminActor(c, AuthMethodLocalToken)
			actor.SessionID = claims.SessionID
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

func validateJWTSecret(secret string) error {
	if len(secret) == 0 {
		return errors.New("JWT secret 为空")
	}
	if len(secret) < 32 {
		return errors.New("JWT secret 长度不足 32 字节")
	}
	return nil
}

func parseAndValidateJWT(tokenStr, secret, issuer, audience string) (*AccessClaims, error) {
	if err := validateJWTSecret(secret); err != nil {
		return nil, err
	}
	tokenSvc := accountsession.NewTokenServiceFromParams(secret, issuer, audience)
	claims, err := tokenSvc.ParseAccessToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.UserID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return claims, nil
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
		UserID:         runtimeidentity.UserID(userID),
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
		UserID:         runtimeidentity.UserID(session.UserID),
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
		UserID:         runtimeidentity.UserID("local_admin"),
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

func RequireAuthMethod(
	allowed ...string,
) gin.HandlerFunc {
	allowedSet :=
		make(map[string]struct{})

	for _, method := range allowed {
		allowedSet[method] =
			struct{}{}
	}

	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil {
			util.ErrorResponse(
				c,
				response.Unauthorized,
				"认证失败",
				nil,
			)
			c.Abort()
			return
		}

		if _, ok :=
			allowedSet[actor.AuthMethod]; !ok {
			util.ErrorResponse(
				c,
				response.Forbidden,
				"认证方式不允许",
				nil,
			)
			c.Abort()
			return
		}

		c.Next()
	}
}
