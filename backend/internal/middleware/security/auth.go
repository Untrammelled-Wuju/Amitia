// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type AuthConfig struct {
	Mode                     string
	LocalCredentials         *LocalCredentialStore
	SpaceID                  string
	ListenAddress            string
	AllowedOrigins           []string
	SessionService           *DesktopSessionService
	DesktopInstanceValidator func(string) bool
	DeviceCredentials        *credential.Service
	DeviceRegistry           *host_registry.Registry
}

const (
	AuthMethodDeviceCredential = "device_credential"
	AuthMethodLocalToken       = "local_token"
	AuthMethodDesktopSession   = "desktop_session"
	AuthMethodLocalAdminToken  = "local_admin_token"
)

var maintenanceAllowedPaths = map[string]bool{
	"/api/health": true, "/api/doctor": true, "/api/migration": true,
	"/api/backup": true, "/api/export": true, "/api/maintenance": true,
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
func validateOrigin(c *gin.Context, allowed []string) error {
	origin := c.GetHeader("Origin")
	if origin == "" {
		return nil
	}
	for _, a := range allowed {
		if a == origin {
			return nil
		}
	}
	return errors.New("origin not allowed")
}
func validateLocalOrigin(c *gin.Context, allowed []string) error {
	if validateOrigin(c, allowed) == nil || isTrustedDesktopOrigin(c.GetHeader("Origin")) {
		return nil
	}
	return errors.New("origin not allowed")
}
func isTrustedDesktopOrigin(origin string) bool {
	return strings.EqualFold(strings.TrimSpace(origin), "app://amitia") || isDesktopDevelopmentOrigin(origin)
}
func isDesktopDevelopmentOrigin(origin string) bool {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || parsed.Scheme != "http" {
		return false
	}
	switch parsed.Port() {
	case "5178", "15177", "15178":
	default:
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
		switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
		case "network", "cloud_core":
			handleNetworkAuth(c, cfg)
		case "local_single_user", "local":
			handleLocalAuth(c, cfg)
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
		if cfg.Mode != "local_single_user" && cfg.Mode != "local" {
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
		actor := buildLocalActor(cfg, AuthMethodLocalAdminToken)
		actor.IsLocalTrusted = true
		applyActorToContext(c, actor)
	}
}

func handleNetworkAuth(c *gin.Context, cfg AuthConfig) {
	if cfg.DeviceCredentials == nil {
		util.ErrorResponse(c, response.InternalError, "设备凭证服务未配置", nil)
		c.Abort()
		return
	}
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(header, "AmitiaDevice ") {
		util.ErrorResponse(c, response.Unauthorized, "需要已配对设备凭证", nil)
		c.Abort()
		return
	}
	raw := strings.TrimSpace(strings.TrimPrefix(header, "AmitiaDevice "))
	cred, err := cfg.DeviceCredentials.Validate(c.Request.Context(), raw)
	if err != nil {
		util.ErrorResponse(c, response.InvalidToken, "设备凭证无效", gin.H{"reason": err.Error()})
		c.Abort()
		return
	}
	if cfg.SpaceID != "" && cred.SpaceID.String() != cfg.SpaceID {
		util.ErrorResponse(c, response.Unauthorized, "设备不属于当前 Space", nil)
		c.Abort()
		return
	}
	if cfg.DeviceRegistry == nil {
		util.ErrorResponse(c, response.InternalError, "设备信任注册表未配置", nil)
		c.Abort()
		return
	}
	if err := cfg.DeviceRegistry.RequireTrustedDevice(c.Request.Context(), cred.SpaceID, cred.DeviceID); err != nil {
		util.ErrorResponse(c, response.Unauthorized, "设备未处于可信状态", gin.H{"reason": err.Error()})
		c.Abort()
		return
	}
	if h := strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID")); h != "" && h != cred.DeviceID.String() {
		util.ErrorResponse(c, response.Unauthorized, "设备标识不匹配", nil)
		c.Abort()
		return
	}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: cred.SpaceID, DeviceID: cred.DeviceID, RuntimeID: cred.RuntimeID, Capabilities: []string{"*"}, Permissions: auth.OwnerDevicePermissions(), AuthMethod: AuthMethodDeviceCredential, RequestID: generateRequestID(), CorrelationID: sanitizeCorrelationID(c.GetHeader("X-Request-ID"))}
	applyActorToContext(c, actor)
}

var localPublicPathPrefixes = []string{"/api/health", "/api/health/circuit-breakers", "/api/onboarding/status", "/api/tts/voices", "/api/public", "/api/core/info"}

func isLocalPublicPath(path string) bool {
	for _, p := range localPublicPathPrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}
func isWebUIResourcePath(path string) bool {
	return strings.HasPrefix(path, "/api/extension/webui/resource/")
}

func handleLocalAuth(c *gin.Context, cfg AuthConfig) {
	if isLocalPublicPath(c.Request.URL.Path) {
		c.Next()
		return
	}
	if isWebUIResourcePath(c.Request.URL.Path) {
		if c.Request.Method != http.MethodGet || !isLoopback(c.Request.RemoteAddr) || !isLoopback(cfg.ListenAddress) {
			util.ErrorResponse(c, response.Unauthorized, "本地资源仅允许回环读取", nil)
			c.Abort()
			return
		}
		c.Next()
		return
	}
	if !isLoopback(c.Request.RemoteAddr) || !isLoopback(cfg.ListenAddress) {
		util.ErrorResponse(c, response.Unauthorized, "本地模式仅允许回环访问", nil)
		c.Abort()
		return
	}
	if err := validateLocalOrigin(c, cfg.AllowedOrigins); err != nil {
		util.ErrorResponse(c, response.Unauthorized, "来源不允许", nil)
		c.Abort()
		return
	}
	sessionToken := strings.TrimSpace(c.GetHeader("X-Amitia-Desktop-Session"))
	if sessionToken != "" && cfg.SessionService != nil {
		instanceID := strings.TrimSpace(c.GetHeader("X-Amitia-Desktop-Instance"))
		if session, err := cfg.SessionService.ValidateSessionWithInstance(c.Request.Context(), sessionToken, instanceID); err == nil {
			_ = cfg.SessionService.TouchSessionWithContext(c.Request.Context(), session)
			_ = cfg.SessionService.RenewSessionWithContext(c.Request.Context(), session)
			actor := buildDesktopSessionActor(session, cfg)
			actor.CorrelationID = sanitizeCorrelationID(c.GetHeader("X-Request-ID"))
			applyActorToContext(c, actor)
			return
		}
	}
	token := strings.TrimSpace(c.GetHeader("X-Amitia-Local-Token"))
	if token != "" && cfg.LocalCredentials != nil && cfg.LocalCredentials.Validate(token) {
		actor := buildLocalActor(cfg, AuthMethodLocalToken)
		actor.CorrelationID = sanitizeCorrelationID(c.GetHeader("X-Request-ID"))
		applyActorToContext(c, actor)
		return
	}
	util.ErrorResponse(c, response.Unauthorized, "本地凭据无效", nil)
	c.Abort()
}

func handleMaintenanceAuth(c *gin.Context, cfg AuthConfig) {
	if !isMaintenanceAllowedPath(c.Request.URL.Path) || !isLoopback(c.Request.RemoteAddr) {
		util.ErrorResponse(c, response.Unauthorized, "维护模式仅允许本机管理接口", nil)
		c.Abort()
		return
	}
	token := strings.TrimSpace(c.GetHeader("X-Amitia-Local-Token"))
	if token != "" && cfg.LocalCredentials != nil && cfg.LocalCredentials.Validate(token) {
		applyActorToContext(c, buildLocalActor(cfg, AuthMethodLocalAdminToken))
		return
	}
	util.ErrorResponse(c, response.Unauthorized, "需要有效本地凭据", nil)
	c.Abort()
}

func buildLocalActor(cfg AuthConfig, method string) *auth.ActorContext {
	spaceID := strings.TrimSpace(cfg.SpaceID)
	if spaceID == "" {
		spaceID = "space_local"
	}
	return &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID(spaceID), Capabilities: []string{"*"}, Permissions: auth.OwnerDevicePermissions(), AuthMethod: method, RequestID: generateRequestID(), IsLocalTrusted: true}
}
func buildDesktopSessionActor(session *DesktopSession, cfg AuthConfig) *auth.ActorContext {
	spaceID := session.SpaceID
	if spaceID == "" {
		spaceID = cfg.SpaceID
	}
	return &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID(spaceID), Capabilities: []string{"*"}, Permissions: auth.OwnerDevicePermissions(), AuthMethod: AuthMethodDesktopSession, SessionID: session.ID, RequestID: generateRequestID(), IsLocalTrusted: true}
}
func applyActorToContext(c *gin.Context, actor *auth.ActorContext) {
	c.Set("actorContext", actor)
	c.Set("spaceId", actor.SpaceID)
	c.Set("principalType", actor.PrincipalType)
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
	if v, ok := c.Get("actorContext"); ok {
		if a, ok := v.(*auth.ActorContext); ok {
			return a
		}
	}
	return nil
}
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		a := GetActor(c)
		if a == nil || !a.HasPermission(perm) {
			util.ErrorResponse(c, response.Forbidden, "权限不足", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
func RequireAuthMethod(allowed ...string) gin.HandlerFunc {
	set := map[string]struct{}{}
	for _, m := range allowed {
		set[m] = struct{}{}
	}
	return func(c *gin.Context) {
		a := GetActor(c)
		if a == nil {
			util.ErrorResponse(c, response.Unauthorized, "认证失败", nil)
			c.Abort()
			return
		}
		if _, ok := set[a.AuthMethod]; !ok {
			util.ErrorResponse(c, response.Forbidden, "认证方式不允许", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
