// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

func (s *Server) handleLogin(c *gin.Context) {
	clientKey := c.ClientIP()
	if !s.login.allow(clientKey) {
		util.ErrorResponse(c, 429, "登录尝试过于频繁，请稍后再试", nil)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, "登录参数无效", nil)
		return
	}
	var user AdminUser
	if err := s.db.Where("username = ? AND active = ?", strings.TrimSpace(body.Username), true).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.login.fail(clientKey)
			util.ErrorResponse(c, 401, "账号或密码错误", nil)
			return
		}
		util.ErrorResponse(c, 500, "读取管理员账号失败", nil)
		return
	}
	if !verifyPassword(body.Password, user.PasswordHash) {
		s.login.fail(clientKey)
		util.ErrorResponse(c, 401, "账号或密码错误", nil)
		return
	}
	sessionToken, csrfToken, expiresAt, err := s.createSession(user)
	if err != nil {
		util.ErrorResponse(c, 500, "创建登录会话失败", nil)
		return
	}
	s.login.reset(clientKey)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})
	util.SuccessMsgResponse(c, "登录成功", gin.H{
		"user":      gin.H{"id": user.ID, "username": user.Username, "role": user.Role},
		"csrfToken": csrfToken,
		"expiresAt": expiresAt,
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	token, err := c.Cookie(s.cfg.CookieName)
	if err == nil && token != "" {
		_ = s.db.Where("token_hash = ?", tokenHash(token)).Delete(&AdminSession{}).Error
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	util.SuccessMsgResponse(c, "已退出登录", nil)
}

func (s *Server) handleChangePassword(c *gin.Context) {
	actor := currentActor(c)
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, "密码参数无效", nil)
		return
	}
	if len(body.NewPassword) < 12 {
		util.ErrorResponse(c, 400, "新密码至少需要 12 个字符", nil)
		return
	}
	if body.NewPassword == body.CurrentPassword {
		util.ErrorResponse(c, 400, "新密码不能与当前密码相同", nil)
		return
	}
	var user AdminUser
	if err := s.db.First(&user, actor.UserID).Error; err != nil {
		util.ErrorResponse(c, 500, "读取管理员账号失败", nil)
		return
	}
	if !verifyPassword(body.CurrentPassword, user.PasswordHash) {
		util.ErrorResponse(c, 400, "当前密码不正确", nil)
		return
	}
	hash, err := hashPassword(body.NewPassword)
	if err != nil {
		util.ErrorResponse(c, 500, "生成密码摘要失败", nil)
		return
	}
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AdminUser{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
			"password_hash": hash,
			"updated_at":    now,
		}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", user.ID).Delete(&AdminSession{}).Error
	}); err != nil {
		util.ErrorResponse(c, 500, "更新密码失败", nil)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	util.SuccessMsgResponse(c, "密码已更新，请重新登录", nil)
}

func (s *Server) handleMe(c *gin.Context) {
	actor := currentActor(c)
	if actor == nil {
		util.ErrorResponse(c, 401, "登录状态已失效", nil)
		return
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		util.ErrorResponse(c, 500, "生成请求令牌失败", nil)
		return
	}
	if err := s.db.Model(&AdminSession{}).Where("token_hash = ?", tokenHash(mustCookie(c, s.cfg.CookieName))).Update("csrf_hash", tokenHash(csrfToken)).Error; err != nil {
		util.ErrorResponse(c, 500, "更新请求令牌失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{
		"user":      gin.H{"id": actor.UserID, "username": actor.Username, "role": actor.Role},
		"csrfToken": csrfToken,
	})
}

func mustCookie(c *gin.Context, name string) string {
	value, _ := c.Cookie(name)
	return value
}

func (s *Server) handleOverview(c *gin.Context) {
	result, err := s.overview()
	if err != nil {
		util.ErrorResponse(c, 500, "读取概览失败", nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (s *Server) handleListReleases(c *gin.Context) {
	releases, err := s.listReleases(c.Query("product"), c.Query("channel"), c.Query("status"))
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, releases)
}

func (s *Server) handleGetRelease(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	release, err := s.getRelease(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			util.ErrorResponse(c, 404, "发布记录不存在", nil)
			return
		}
		util.ErrorResponse(c, 500, "读取发布记录失败", nil)
		return
	}
	util.SuccessResponse(c, release)
}

func (s *Server) handleCreateRelease(c *gin.Context) {
	actor := currentActor(c)
	var input CreateReleaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.ErrorResponse(c, 400, "发布参数无效", nil)
		return
	}
	release, err := s.createRelease(*actor, input)
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, release)
}

func (s *Server) handleDeleteRelease(c *gin.Context) {
	actor := currentActor(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	if err := s.deleteDraft(*actor, uint(id)); err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "删除成功", nil)
}

func (s *Server) handleUploadArtifact(c *gin.Context) {
	actor := currentActor(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.cfg.MaxUploadBytes+1024*1024)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, 400, "读取上传文件失败", nil)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		util.ErrorResponse(c, 400, "打开上传文件失败", nil)
		return
	}
	defer file.Close()
	artifact, err := s.saveArtifact(*actor, uint(id), c.PostForm("kind"), fileHeader.Filename, file)
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, artifact)
}

func (s *Server) handleValidateRelease(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	result, err := s.validateRelease(uint(id))
	if err != nil {
		util.ErrorResponse(c, 500, "校验发布失败", nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (s *Server) handlePublishRelease(c *gin.Context) {
	actor := currentActor(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	release, err := s.publishRelease(*actor, uint(id))
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "发布成功", release)
}

func (s *Server) handlePauseRelease(c *gin.Context) {
	actor := currentActor(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	var body struct {
		RolloutPercentage int `json:"rolloutPercentage"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, "灰度参数无效", nil)
		return
	}
	release, err := s.pauseRelease(*actor, uint(id), body.RolloutPercentage)
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "灰度已更新", release)
}

func (s *Server) handleRollbackDesktop(c *gin.Context) {
	actor := currentActor(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	release, err := s.rollbackDesktop(*actor, uint(id))
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "最新指针已切换", release)
}

func (s *Server) handleAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	logs, err := s.listAuditLogs(limit)
	if err != nil {
		util.ErrorResponse(c, 500, "读取审计日志失败", nil)
		return
	}
	util.SuccessResponse(c, logs)
}

func (s *Server) handleSettings(c *gin.Context) {
	util.SuccessResponse(c, gin.H{
		"desktopPublishDir":    s.cfg.DesktopPublishDir,
		"androidPublishDir":    s.cfg.AndroidPublishDir,
		"publicBaseURL":        s.cfg.PublicBaseURL,
		"androidPackageName":   s.cfg.AndroidPackageName,
		"desktopClientURL":     s.cfg.PublicBaseURL + "/latest.yml",
		"androidClientBaseURL": s.cfg.PublicBaseURL + "/android",
		"signingKeyConfigured": s.cfg.AndroidManifestKeyPath != "",
		"cookieSecure":         s.cfg.CookieSecure,
	})
}
