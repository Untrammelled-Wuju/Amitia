// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"golang.org/x/crypto/argon2"
)

const (
	WebAccessCookieName = "amitia_web_access"
	webAccessStateFile  = "web-access.json"
	webSessionTTL       = 30 * 24 * time.Hour
)

var (
	ErrWebAccessNotConfigured = errors.New("web access password is not configured")
	ErrWebAccessInvalid       = errors.New("invalid web access password")
	ErrWebAccessRateLimited   = errors.New("too many web access login attempts")
)

type webAccessState struct {
	PasswordHash   string    `json:"passwordHash"`
	SessionSecret  string    `json:"sessionSecret"`
	SessionVersion int64     `json:"sessionVersion"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type webSessionPayload struct {
	Version   int64 `json:"v"`
	IssuedAt  int64 `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

type webLoginAttempt struct {
	Failures    int
	WindowStart time.Time
	LockedUntil time.Time
}

// WebAccessService is a deliberately small, account-less access gate for the
// Cloud Core browser UI. It does not replace DeviceCredential. Password state
// is stored in the runtime data directory and browser sessions are stateless,
// HMAC-signed cookies.
type WebAccessService struct {
	mu       sync.RWMutex
	path     string
	state    webAccessState
	attempts map[string]webLoginAttempt
}

func NewWebAccessService(dataDir string) (*WebAccessService, error) {
	securityDir := filepath.Join(strings.TrimSpace(dataDir), "security")
	if err := os.MkdirAll(securityDir, 0o700); err != nil {
		return nil, fmt.Errorf("web access: create security directory: %w", err)
	}
	s := &WebAccessService{
		path:     filepath.Join(securityDir, webAccessStateFile),
		attempts: make(map[string]webLoginAttempt),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *WebAccessService) load() error {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("web access: read state: %w", err)
	}
	var state webAccessState
	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("web access: decode state: %w", err)
	}
	if strings.TrimSpace(state.PasswordHash) == "" || strings.TrimSpace(state.SessionSecret) == "" || state.SessionVersion < 1 {
		return errors.New("web access: persisted state is incomplete")
	}
	s.state = state
	return nil
}

func (s *WebAccessService) Configured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.PasswordHash != "" && s.state.SessionSecret != "" && s.state.SessionVersion > 0
}

func (s *WebAccessService) Setup(password string) (string, error) {
	if err := validateWebAccessPassword(password); err != nil {
		return "", err
	}
	hash, err := hashWebAccessPassword(password)
	if err != nil {
		return "", err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("web access: generate session secret: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.PasswordHash != "" {
		return "", errors.New("web access password is already configured")
	}
	now := time.Now().UTC()
	s.state = webAccessState{
		PasswordHash:   hash,
		SessionSecret:  base64.RawURLEncoding.EncodeToString(secret),
		SessionVersion: 1,
		UpdatedAt:      now,
	}
	if err := s.persistLocked(); err != nil {
		s.state = webAccessState{}
		return "", err
	}
	return s.issueSessionLocked(now)
}

func (s *WebAccessService) Login(password, remoteAddr string) (string, error) {
	now := time.Now().UTC()
	key := remoteAddressKey(remoteAddr)
	if retryAfter := s.retryAfter(key, now); retryAfter > 0 {
		return "", fmt.Errorf("%w: retry after %s", ErrWebAccessRateLimited, retryAfter.Round(time.Second))
	}

	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	if state.PasswordHash == "" {
		return "", ErrWebAccessNotConfigured
	}
	ok, err := verifyWebAccessPassword(password, state.PasswordHash)
	if err != nil {
		return "", err
	}
	if !ok {
		s.recordFailure(key, now)
		return "", ErrWebAccessInvalid
	}
	s.clearFailures(key)

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.issueSessionLocked(now)
}

func (s *WebAccessService) ValidateSession(token string) bool {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	if state.SessionSecret == "" || state.SessionVersion < 1 {
		return false
	}
	secret, err := base64.RawURLEncoding.DecodeString(state.SessionSecret)
	if err != nil || len(secret) < 32 {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return false
	}

	var payload webSessionPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return false
	}
	now := time.Now().UTC().Unix()
	return payload.Version == state.SessionVersion && payload.IssuedAt > 0 && payload.ExpiresAt > now && payload.IssuedAt <= now+60
}

func (s *WebAccessService) issueSessionLocked(now time.Time) (string, error) {
	secret, err := base64.RawURLEncoding.DecodeString(s.state.SessionSecret)
	if err != nil || len(secret) < 32 {
		return "", errors.New("web access: invalid session secret")
	}
	payload := webSessionPayload{
		Version:   s.state.SessionVersion,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(webSessionTTL).Unix(),
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("web access: encode session: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(rawPayload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *WebAccessService) persistLocked() error {
	raw, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("web access: encode state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("web access: write state: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("web access: chmod state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("web access: commit state: %w", err)
	}
	return nil
}

func validateWebAccessPassword(password string) error {
	length := len([]rune(password))
	if length < 8 {
		return errors.New("web access password must contain at least 8 characters")
	}
	if length > 128 {
		return errors.New("web access password is too long")
	}
	return nil
}

func hashWebAccessPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("web access: generate password salt: %w", err)
	}
	const memory = 32 * 1024
	const iterations = 2
	const parallelism = 1
	const keyLength = 32
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyWebAccessPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("web access: invalid password hash")
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, errors.New("web access: invalid password parameters")
	}
	if memory < 8*1024 || memory > 128*1024 || iterations < 1 || iterations > 10 || parallelism < 1 || parallelism > 8 {
		return false, errors.New("web access: unsafe password parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 {
		return false, errors.New("web access: invalid password salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false, errors.New("web access: invalid password digest")
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func (s *WebAccessService) retryAfter(key string, now time.Time) time.Duration {
	if key == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt := s.attempts[key]
	if attempt.LockedUntil.After(now) {
		return attempt.LockedUntil.Sub(now)
	}
	if !attempt.WindowStart.IsZero() && now.Sub(attempt.WindowStart) > 5*time.Minute {
		delete(s.attempts, key)
	}
	return 0
}

func (s *WebAccessService) recordFailure(key string, now time.Time) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt := s.attempts[key]
	if attempt.WindowStart.IsZero() || now.Sub(attempt.WindowStart) > 5*time.Minute {
		attempt = webLoginAttempt{WindowStart: now}
	}
	attempt.Failures++
	if attempt.Failures >= 5 {
		attempt.LockedUntil = now.Add(30 * time.Second)
		attempt.Failures = 0
		attempt.WindowStart = now
	}
	s.attempts[key] = attempt
}

func (s *WebAccessService) clearFailures(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	delete(s.attempts, key)
	s.mu.Unlock()
}

func remoteAddressKey(raw string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err == nil {
		return host
	}
	return strings.TrimSpace(raw)
}

func webAccessCookie(c *gin.Context) string {
	value, err := c.Cookie(WebAccessCookieName)
	if err != nil {
		return ""
	}
	return value
}

func setWebAccessCookie(c *gin.Context, token string) {
	secure := c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
	sameSite := http.SameSiteLaxMode
	if secure && crossOriginRequest(c) {
		sameSite = http.SameSiteNoneMode
	}
	c.SetSameSite(sameSite)
	c.SetCookie(WebAccessCookieName, token, int(webSessionTTL.Seconds()), "/", "", secure, true)
}

func clearWebAccessCookie(c *gin.Context) {
	secure := c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(WebAccessCookieName, "", -1, "/", "", secure, true)
}

func crossOriginRequest(c *gin.Context) bool {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	requestHost := strings.TrimSpace(c.Request.Host)
	return requestHost != "" && !strings.EqualFold(parsed.Host, requestHost)
}

func bindWebAccessPassword(c *gin.Context) (string, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var body struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return "", err
	}
	return body.Password, nil
}

func RegisterWebAccessPublicRoutes(group *gin.RouterGroup, svc *WebAccessService) {
	if group == nil || svc == nil {
		return
	}
	group.GET("/web-access/status", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		configured := svc.Configured()
		authenticated := configured && svc.ValidateSession(webAccessCookie(c))
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{
			"configured": configured, "authenticated": authenticated,
		}})
	})
	group.POST("/web-access/login", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		password, bindErr := bindWebAccessPassword(c)
		if bindErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请输入访问密码"})
			return
		}
		token, err := svc.Login(password, c.Request.RemoteAddr)
		if err != nil {
			switch {
			case errors.Is(err, ErrWebAccessNotConfigured):
				c.JSON(http.StatusPreconditionRequired, gin.H{"code": 428, "msg": "Web 访问密码尚未设置"})
			case errors.Is(err, ErrWebAccessRateLimited):
				c.Header("Retry-After", "30")
				c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "msg": "尝试次数过多，请稍后再试"})
			default:
				c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "访问密码错误"})
			}
			return
		}
		setWebAccessCookie(c, token)
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"authenticated": true}})
	})
	group.POST("/web-access/logout", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		clearWebAccessCookie(c)
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"authenticated": false}})
	})
}

// RegisterWebAccessSetupRoutes is mounted behind DeviceCredential auth. Only a
// trusted web device may initialize the password, preventing a fresh public
// Cloud Core from being claimed by the first anonymous browser that reaches it.
func RegisterWebAccessSetupRoutes(router gin.IRouter, svc *WebAccessService, devices *host_registry.Registry) {
	if router == nil || svc == nil || devices == nil {
		return
	}
	router.POST("/web-access/setup", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		actor := GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "需要已配对的 Web 设备"})
			return
		}
		device, err := devices.GetDevice(c.Request.Context(), actor.DeviceID)
		if err != nil || device == nil || device.SpaceID != actor.SpaceID || device.TrustState != host_registry.DeviceTrustTrusted || device.Platform != runtimeidentity.PlatformWeb {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "只有已配对的 Web 设备可以设置 Web 访问密码"})
			return
		}
		password, bindErr := bindWebAccessPassword(c)
		if bindErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请输入访问密码"})
			return
		}
		token, err := svc.Setup(password)
		if err != nil {
			status := http.StatusBadRequest
			if svc.Configured() {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"code": status, "msg": err.Error()})
			return
		}
		setWebAccessCookie(c, token)
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"configured": true, "authenticated": true}})
	})
}

// RequireWebAccessForWebDevice is mounted after DeviceCredential auth. Device
// platform comes from the trusted device registry rather than a caller-supplied
// header, so desktop/mobile/device-agent traffic never gains this requirement
// and a web device cannot bypass it by spoofing X-Amitia-Client-Type.
func RequireWebAccessForWebDevice(svc *WebAccessService, devices *host_registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil || devices == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Web 访问认证服务不可用"})
			return
		}
		actor := GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "设备认证失败"})
			return
		}
		device, err := devices.GetDevice(c.Request.Context(), actor.DeviceID)
		if err != nil || device == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "设备身份不可用"})
			return
		}
		if device.Platform != runtimeidentity.PlatformWeb {
			c.Next()
			return
		}
		if !svc.Configured() {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{"code": 428, "msg": "请先设置 Web 访问密码"})
			return
		}
		if !svc.ValidateSession(webAccessCookie(c)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Web 登录已失效，请重新输入访问密码"})
			return
		}
		c.Next()
	}
}

// RequireWebAccessForDeclaredBrowser protects the unauthenticated Device Mesh
// pairing/bootstrap endpoints used by the official browser UI once an access
// password exists. General health/capability discovery remains public so the
// browser can boot far enough to present the password screen. Private business
// APIs are enforced authoritatively by RequireWebAccessForWebDevice.
func RequireWebAccessForDeclaredBrowser(svc *WebAccessService) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if svc == nil || !svc.Configured() || !strings.HasPrefix(path, "/api/public/device-mesh/v1/") || !strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Amitia-Client-Type")), "web") {
			c.Next()
			return
		}
		if !svc.ValidateSession(webAccessCookie(c)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Web 登录已失效，请重新输入访问密码"})
			return
		}
		c.Next()
	}
}

// RequireWebAccessForDeviceRuntimePrincipal mirrors the browser gate for the
// small set of Cloud Core routes that authenticate with Device Mesh's direct
// credential middleware instead of the normal /api AuthenticationMiddleware.
func RequireWebAccessForDeviceRuntimePrincipal(svc *WebAccessService, devices *host_registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil || devices == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Web 访问认证服务不可用"})
			return
		}
		principal, ok := credential.GinPrincipal(c)
		if !ok || principal.DeviceID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "设备认证失败"})
			return
		}
		device, err := devices.GetDevice(c.Request.Context(), principal.DeviceID)
		if err != nil || device == nil || device.SpaceID != principal.SpaceID || device.TrustState != host_registry.DeviceTrustTrusted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "设备身份不可用"})
			return
		}
		if device.Platform != runtimeidentity.PlatformWeb {
			c.Next()
			return
		}
		if !svc.Configured() {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{"code": 428, "msg": "请先设置 Web 访问密码"})
			return
		}
		if !svc.ValidateSession(webAccessCookie(c)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Web 登录已失效，请重新输入访问密码"})
			return
		}
		c.Next()
	}
}
