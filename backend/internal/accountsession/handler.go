package accountsession

import (
	"crypto/subtle"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type UserServiceProvider interface {
	FindByUsername(username string) (*AuthUserDTO, error)
	FindByID(id int) (*AuthUserDTO, error)
	UpdatePassword(id int, newHash string) error
	UpdateLoginTime(id int) error
	CreateUser(username, password, role string) (*AuthUserDTO, error)
	HasAdmin() (bool, error)
}

type Handler struct {
	svc          *AccountSessionService
	validator    *Validator
	audit        AuditLogger
	userProvider UserServiceProvider
}

func NewHandler(svc *AccountSessionService, validator *Validator, audit AuditLogger, userProvider UserServiceProvider) *Handler {
	return &Handler{svc: svc, validator: validator, audit: audit, userProvider: userProvider}
}

type loginRequestBody struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Setup(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "用户名和密码不能为空", nil)
		return
	}
	if len(req.Password) < 6 {
		util.ErrorResponse(c, response.InvalidParams, "密码至少 6 位", nil)
		return
	}

	hasAdmin, err := h.userProvider.HasAdmin()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	if hasAdmin {
		util.ErrorResponse(c, response.BusinessError, "管理员已存在，请直接登录", gin.H{"errorCode": "auth.admin_exists"})
		return
	}

	if err := authorizeInitialAdminSetup(c); err != nil {
		util.ErrorResponse(c, response.Forbidden, err.Error(), gin.H{"errorCode": "auth.setup_forbidden"})
		return
	}

	clientType := detectClientType(c)
	ip := c.ClientIP()
	ua := c.Request.UserAgent()

	userDTO, err := h.userProvider.CreateUser(req.Username, req.Password, "admin")
	if err != nil {
		util.ErrorResponse(c, response.BusinessError, "创建用户失败: "+err.Error(), nil)
		return
	}

	session, err := h.svc.CreateInitialSession(userDTO.ID, userDTO.Username, userDTO.Role, ip, ua)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	if err := h.userProvider.UpdateLoginTime(userDTO.ID); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	if err := h.audit.LogLoginSuccess(userDTO.ID, session.SessionPublicID, ip, ua, userDTO.Username); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	result := gin.H{
		"accessToken":           session.AccessToken,
		"accessTokenExpiresAt":  session.AccessExpiresAt.Format(time.RFC3339),
		"refreshToken":          session.RefreshToken,
		"refreshTokenExpiresAt": session.RefreshExpiresAt.Format(time.RFC3339),
		"token":                 session.AccessToken,
		"session": gin.H{
			"sessionId": session.SessionPublicID,
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		},
		"user": gin.H{
			"id":       session.UserID,
			"username": session.Username,
			"role":     session.Role,
		},
	}

	if clientType == "web" {
		c.SetCookie("AmitiaRefresh", session.RefreshToken, int(30*24*time.Hour.Seconds()), "/api", "", true, true)
		delete(result, "refreshToken")
		delete(result, "refreshTokenExpiresAt")
	}

	util.SuccessResponse(c, result)
}

// authorizeInitialAdminSetup keeps local desktop bootstrap frictionless while making
// network/cloud bootstrap fail closed. A Cloud Core operator must explicitly provide
// AMITIA_SETUP_TOKEN (minimum 32 characters) and send the same value in the
// X-Amitia-Setup-Token header. The token is never accepted from the URL/body so it is
// less likely to leak through access logs, analytics, or browser history.
func authorizeInitialAdminSetup(c *gin.Context) error {
	if config.AppCfg == nil || config.AppCfg.Security.Mode != "network" {
		return nil
	}

	expected := strings.TrimSpace(os.Getenv("AMITIA_SETUP_TOKEN"))
	if len(expected) < 32 {
		return fmt.Errorf("网络模式首次管理员初始化已禁用：请先设置至少 32 位的 AMITIA_SETUP_TOKEN")
	}

	provided := strings.TrimSpace(c.GetHeader("X-Amitia-Setup-Token"))
	if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return fmt.Errorf("首次管理员初始化凭据无效")
	}
	return nil
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "用户名和密码不能为空", nil)
		return
	}
	clientType := detectClientType(c)
	ip := c.ClientIP()
	ua := c.Request.UserAgent()

	resp, err := h.svc.Login(LoginRequestInternal{
		Username:   req.Username,
		Password:   req.Password,
		ClientType: clientType,
		IPAddress:  ip,
		UserAgent:  ua,
	}, h.userProvider)

	if err != nil {
		switch err {
		case ErrLoginRateLimited:
			util.ErrorResponse(c, 429, err.Error(), gin.H{"errorCode": "auth.login_rate_limited"})
		case ErrInvalidCredentials:
			util.ErrorResponse(c, response.Unauthorized, err.Error(), gin.H{"errorCode": "auth.invalid_credentials"})
		default:
			util.ErrorResponse(c, response.Unauthorized, err.Error(), gin.H{"errorCode": "auth.login_error"})
		}
		return
	}

	result := gin.H{
		"accessToken":           resp.AccessToken,
		"accessTokenExpiresAt":  resp.AccessExpiresAt.Format(time.RFC3339),
		"refreshToken":          resp.RefreshToken,
		"refreshTokenExpiresAt": resp.RefreshExpiresAt.Format(time.RFC3339),
		"token":                 resp.AccessToken,
		"session": gin.H{
			"sessionId": resp.SessionPublicID,
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		},
		"user": gin.H{
			"id":       resp.UserID,
			"username": resp.Username,
			"role":     resp.Role,
		},
	}

	if clientType == "web" {
		c.SetCookie("AmitiaRefresh", resp.RefreshToken, int(30*24*time.Hour.Seconds()), "/api", "", true, true)
		delete(result, "refreshToken")
		delete(result, "refreshTokenExpiresAt")
	}

	util.SuccessResponse(c, result)
}

type refreshRequestBody struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) Refresh(c *gin.Context) {
	var rawToken string
	clientType := detectClientType(c)

	if clientType == "web" {
		var cookieErr error
		rawToken, cookieErr = c.Cookie("AmitiaRefresh")
		if cookieErr != nil && rawToken == "" {
			util.ErrorResponse(c, response.Unauthorized, "缺少刷新令牌", gin.H{"errorCode": "auth.refresh_invalid"})
			return
		}
	} else {
		var req refreshRequestBody
		if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
			rawToken = req.RefreshToken
		}
		if rawToken == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "AmitiaRefresh ") {
				rawToken = strings.TrimPrefix(authHeader, "AmitiaRefresh ")
			}
		}
	}

	if rawToken == "" {
		util.ErrorResponse(c, response.Unauthorized, "缺少刷新令牌", gin.H{"errorCode": "auth.refresh_invalid"})
		return
	}

	result, err := h.svc.RefreshService().Rotate(rawToken)
	if err != nil {
		switch err {
		case ErrRefreshReused:
			h.audit.LogRefreshReuseDetected("", 0, c.ClientIP(), c.Request.UserAgent())
			h.clearRefreshCookie(c, clientType)
			util.ErrorResponse(c, response.Unauthorized, err.Error(), gin.H{"errorCode": "auth.refresh_reused"})
		case ErrRefreshExpired:
			h.clearRefreshCookie(c, clientType)
			util.ErrorResponse(c, response.Unauthorized, err.Error(), gin.H{"errorCode": "auth.refresh_expired"})
		case ErrRefreshRevoked:
			h.clearRefreshCookie(c, clientType)
			util.ErrorResponse(c, response.Unauthorized, err.Error(), gin.H{"errorCode": "auth.refresh_revoked"})
		default:
			util.ErrorResponse(c, response.Unauthorized, err.Error(), gin.H{"errorCode": "auth.refresh_invalid"})
		}
		return
	}

	userID := int64(0)
	if result.UserID != "" {
		fmt.Sscanf(result.UserID, "%d", &userID)
	}
	tokenSvc := NewTokenService()
	accessToken, accessExpiresAt, err := tokenSvc.SignAccessToken(int(userID), result.Username, result.Role, result.SessionID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	if err := h.audit.LogRefreshSuccess(result.SessionID, int(userID), c.ClientIP(), c.Request.UserAgent()); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	resp := gin.H{
		"accessToken":           accessToken,
		"accessTokenExpiresAt":  accessExpiresAt.Format(time.RFC3339),
		"refreshToken":          result.RefreshToken,
		"refreshTokenExpiresAt": result.RefreshExpiresAt.Format(time.RFC3339),
	}

	if clientType == "web" {
		c.SetCookie("AmitiaRefresh", result.RefreshToken, int(30*24*time.Hour.Seconds()), "/api", "", true, true)
		delete(resp, "refreshToken")
		delete(resp, "refreshTokenExpiresAt")
	}

	util.SuccessResponse(c, resp)
}

func (h *Handler) Logout(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	clientType := detectClientType(c)
	sessionID := actor.SessionID
	if sessionID != "" && actor.UserID != "" {
		var userID int64
		fmt.Sscanf(string(actor.UserID), "%d", &userID)
		if userID > 0 {
			if err := h.svc.RevokeCurrentSession(sessionID, userID); err != nil {
				util.ErrorResponse(c, response.InternalError, err.Error(), nil)
				return
			}
		}
	}

	h.clearRefreshCookie(c, clientType)
	util.SuccessMsgResponse(c, "已登出", nil)
}

func (h *Handler) RevokeRefresh(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	clientType := detectClientType(c)
	if clientType == "web" {
		rawToken, err := c.Cookie("AmitiaRefresh")
		if err == nil && rawToken != "" {
			if revokeErr := h.svc.RefreshService().RevokeByToken(rawToken); revokeErr != nil {
				util.ErrorResponse(c, response.InternalError, revokeErr.Error(), nil)
				return
			}
		}
		h.clearRefreshCookie(c, "web")
	} else {
		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		rawToken := ""
		if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
			rawToken = req.RefreshToken
		}
		if rawToken == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "AmitiaRefresh ") {
				rawToken = strings.TrimPrefix(authHeader, "AmitiaRefresh ")
			}
		}
		if rawToken != "" {
			if revokeErr := h.svc.RefreshService().RevokeByToken(rawToken); revokeErr != nil {
				util.ErrorResponse(c, response.InternalError, revokeErr.Error(), nil)
				return
			}
		}
	}

	util.SuccessMsgResponse(c, "刷新令牌已撤销", nil)
}

func (h *Handler) ListSessions(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	var userID int64
	fmt.Sscanf(string(actor.UserID), "%d", &userID)
	currentSessionID := actor.SessionID

	sessions, err := h.svc.ListActiveSessions(userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	result := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		item := gin.H{
			"sessionId":  s.PublicID,
			"current":    s.PublicID == currentSessionID,
			"status":     s.Status,
			"deviceName": s.DeviceName,
			"ipAddress":  s.IPAddress,
			"userAgent":  s.UserAgent,
			"createdAt":  s.CreatedAt.Format(time.RFC3339),
		}
		if s.LastActiveAt != nil {
			item["lastActiveAt"] = s.LastActiveAt.Format(time.RFC3339)
		}
		if s.LastRefreshedAt != nil {
			item["lastRefreshedAt"] = s.LastRefreshedAt.Format(time.RFC3339)
		}
		if s.ExpiresAt != nil {
			item["expiresAt"] = s.ExpiresAt.Format(time.RFC3339)
		}
		result = append(result, item)
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) RevokeSession(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	sessionID := c.Param("sessionId")
	var userID int64
	fmt.Sscanf(string(actor.UserID), "%d", &userID)

	if err := h.svc.RevokeOwnedSession(userID, sessionID); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, nil)
}

func (h *Handler) RevokeOtherSessions(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	var userID int64
	fmt.Sscanf(string(actor.UserID), "%d", &userID)
	count, err := h.svc.RevokeOtherSessions(actor.SessionID, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"revokedCount": count})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请输入新旧密码", nil)
		return
	}
	if len(req.NewPassword) < 6 {
		util.ErrorResponse(c, response.InvalidParams, "新密码至少 6 位", nil)
		return
	}

	var userID int64
	fmt.Sscanf(string(actor.UserID), "%d", &userID)
	if userID <= 0 {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	newSession, _, err := h.svc.ChangePasswordAndRotate(userID, req.OldPassword, req.NewPassword, h.userProvider)
	if err != nil {
		switch err {
		case ErrInvalidCredentials:
			util.ErrorResponse(c, response.Unauthorized, "旧密码不正确", gin.H{"errorCode": "auth.invalid_old_password"})
		default:
			util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		}
		return
	}

	result := gin.H{
		"accessToken":           newSession.AccessToken,
		"accessTokenExpiresAt":  newSession.AccessExpiresAt.Format(time.RFC3339),
		"refreshToken":          newSession.RefreshToken,
		"refreshTokenExpiresAt": newSession.RefreshExpiresAt.Format(time.RFC3339),
		"token":                 newSession.AccessToken,
		"session": gin.H{
			"sessionId": newSession.SessionPublicID,
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		},
		"user": gin.H{
			"id":       newSession.UserID,
			"username": newSession.Username,
			"role":     newSession.Role,
		},
	}

	clientType := detectClientType(c)
	if clientType == "web" {
		c.SetCookie("AmitiaRefresh", newSession.RefreshToken, int(30*24*time.Hour.Seconds()), "/api", "", true, true)
		delete(result, "refreshToken")
		delete(result, "refreshTokenExpiresAt")
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) LogoutAll(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	var userID int64
	fmt.Sscanf(string(actor.UserID), "%d", &userID)
	if err := h.svc.RevokeAllSessions(userID); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	h.clearRefreshCookie(c, detectClientType(c))
	util.SuccessMsgResponse(c, "已退出全部设备", nil)
}

func (h *Handler) clearRefreshCookie(c *gin.Context, clientType string) {
	if clientType == "web" {
		c.SetCookie("AmitiaRefresh", "", -1, "/api", "", true, true)
	}
}

func detectClientType(c *gin.Context) string {
	ct := strings.TrimSpace(c.GetHeader("X-Amitia-Client-Type"))
	switch ct {
	case "web", "desktop", "mobile":
		return ct
	}
	if ua := c.Request.UserAgent(); strings.Contains(ua, "Mozilla") || strings.Contains(ua, "Chrome") || strings.Contains(ua, "Safari") {
		return "web"
	}
	return "web"
}

func getActor(c *gin.Context) *auth.ActorContext {
	if v, exists := c.Get("actorContext"); exists {
		if actor, ok := v.(*auth.ActorContext); ok {
			return actor
		}
	}
	return nil
}

var _ = runtimeidentity.UserID("")
