// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

const actorContextKey = "admin_release_actor"

type adminActor struct {
	UserID   uint
	Username string
	Role     string
	CSRFHash string
}

type passwordParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultPasswordParams = passwordParams{
	memory:      64 * 1024,
	iterations:  3,
	parallelism: 2,
	saltLength:  16,
	keyLength:   32,
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, defaultPasswordParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		defaultPasswordParams.iterations,
		defaultPasswordParams.memory,
		defaultPasswordParams.parallelism,
		defaultPasswordParams.keyLength,
	)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		defaultPasswordParams.memory,
		defaultPasswordParams.iterations,
		defaultPasswordParams.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func randomToken(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Server) bootstrapAdmin() (string, error) {
	var count int64
	if err := s.db.Model(&AdminUser{}).Count(&count).Error; err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil
	}
	password := strings.TrimSpace(s.cfg.BootstrapPassword)
	generated := false
	if password == "" {
		var err error
		password, err = randomToken(18)
		if err != nil {
			return "", err
		}
		generated = true
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	user := AdminUser{
		Username:     strings.TrimSpace(s.cfg.BootstrapUsername),
		PasswordHash: hash,
		Role:         RoleAdmin,
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if user.Username == "" {
		user.Username = "admin"
	}
	if err := s.db.Create(&user).Error; err != nil {
		return "", err
	}
	if generated {
		return password, nil
	}
	return "", nil
}

func (s *Server) createSession(user AdminUser) (sessionToken, csrfToken string, expiresAt time.Time, err error) {
	sessionToken, err = randomToken(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	csrfToken, err = randomToken(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	expiresAt = time.Now().Add(s.cfg.SessionTTL)
	session := AdminSession{
		UserID:     user.ID,
		TokenHash:  tokenHash(sessionToken),
		CSRFHash:   tokenHash(csrfToken),
		ExpiresAt:  expiresAt,
		LastSeenAt: time.Now(),
		CreatedAt:  time.Now(),
	}
	if err := s.db.Create(&session).Error; err != nil {
		return "", "", time.Time{}, err
	}
	return sessionToken, csrfToken, expiresAt, nil
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(s.cfg.CookieName)
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "登录状态已失效", "data": nil})
			return
		}
		var session AdminSession
		if err := s.db.Where("token_hash = ? AND expires_at > ?", tokenHash(token), time.Now()).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "登录状态已失效", "data": nil})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取登录状态失败", "data": nil})
			return
		}
		var user AdminUser
		if err := s.db.First(&user, session.UserID).Error; err != nil || !user.Active {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "管理员账号不可用", "data": nil})
			return
		}
		now := time.Now()
		_ = s.db.Model(&AdminSession{}).Where("id = ?", session.ID).Updates(map[string]interface{}{
			"last_seen_at": now,
			"expires_at":   now.Add(s.cfg.SessionTTL),
		}).Error
		c.Set(actorContextKey, adminActor{
			UserID:   user.ID,
			Username: user.Username,
			Role:     user.Role,
			CSRFHash: session.CSRFHash,
		})
		c.Next()
	}
}

func requireMutationToken(c *gin.Context) {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
		return
	}
	actor := currentActor(c)
	provided := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
	if actor == nil || provided == "" || subtle.ConstantTimeCompare([]byte(actor.CSRFHash), []byte(tokenHash(provided))) != 1 {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": "请求校验失败", "data": nil})
	}
}

func requirePublisher(c *gin.Context) {
	actor := currentActor(c)
	if actor == nil || (actor.Role != RoleAdmin && actor.Role != RolePublisher) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": "没有发布权限", "data": nil})
		return
	}
}

func requireAdmin(c *gin.Context) {
	actor := currentActor(c)
	if actor == nil || actor.Role != RoleAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": "需要管理员权限", "data": nil})
		return
	}
}

func currentActor(c *gin.Context) *adminActor {
	value, exists := c.Get(actorContextKey)
	if !exists {
		return nil
	}
	actor, ok := value.(adminActor)
	if !ok {
		return nil
	}
	return &actor
}
