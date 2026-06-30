// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/app"
	"golang.org/x/crypto/scrypt"
	"gorm.io/gorm"
)

const (
	scryptN = 16384
	scryptR = 8
	scryptP = 1
	saltLen = 32
	keyLen  = 64
)

type Service interface {
	Login(username, password, ipAddr, userAgent string) (*LoginResponse, string, error)

	Setup(username, password, ipAddr, userAgent string) (*LoginResponse, error)

	GetMe(token string) (*UserInfoResponse, error)

	Logout(token string) error

	HasAdmin() (bool, error)

	ListSessions(token string) ([]SessionResponse, error)

	RevokeSession(token string, sessionID int) error

	RevokeAllSessions(token string) error

	ChangePassword(token, oldPassword, newPassword string) error
}

type service struct {
	repo      Repository
	db        *gorm.DB
	jwtSecret string
}

func NewService(repo Repository, ctx *app.AppContext) Service {
	return &service{
		repo:      repo,
		db:        ctx.DB,
		jwtSecret: config.AppCfg.JWT.Secret,
	}
}

func hashPassword(password string) string {
	rawSalt := make([]byte, saltLen)
	rand.Read(rawSalt)
	saltHex := hex.EncodeToString(rawSalt)
	dk, _ := scrypt.Key([]byte(password), []byte(saltHex), scryptN, scryptR, scryptP, keyLen)
	return saltHex + ":" + hex.EncodeToString(dk)
}

func verifyPassword(password, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt := []byte(parts[0])
	key, _ := hex.DecodeString(parts[1])
	if key == nil {
		return false
	}
	dk, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return false
	}
	return hmacEqual(dk, key)
}

func hmacEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type JWTClaims struct {
	UserId   int    `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *service) signJWT(userID int, username, role string) (string, error) {
	claims := jwt.MapClaims{
		"userId":   userID,
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(time.Duration(config.AppCfg.JWT.ExpireDays) * 24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(s.jwtSecret))
}

func (s *service) parseJWT(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func (s *service) Login(username, password, ipAddr, userAgent string) (*LoginResponse, string, error) {
	user, err := s.repo.FindByUsername(username)
	if err != nil {
		return nil, "", fmt.Errorf("用户名或密码错误")
	}
	if !verifyPassword(password, user.PasswordHash) {
		return nil, "", fmt.Errorf("用户名或密码错误")
	}

	token, err := s.signJWT(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, "", fmt.Errorf("签发令牌失败")
	}

	deviceName := parseDevice(userAgent)
	expiresAtStr := time.Now().Add(time.Duration(config.AppCfg.JWT.ExpireDays) * 24 * time.Hour).Format("2006-01-02 15:04:05")
	session := &AuthSession{
		UserID:     user.ID,
		TokenHash:  hashToken(token),
		DeviceName: deviceName,
		IPAddress:  ipAddr,
		UserAgent:  userAgent,
		ExpiresAt:  &expiresAtStr,
	}
	if err := s.repo.CreateSession(session); err != nil {
		return nil, "", fmt.Errorf("创建会话失败")
	}

	s.repo.UpdateLoginTime(user.ID)

	return &LoginResponse{
		Token:    token,
		Username: user.Username,
		Role:     user.Role,
	}, token, nil
}

func (s *service) Setup(username, password, ipAddr, userAgent string) (*LoginResponse, error) {
	hasAdmin, err := s.repo.HasAdmin()
	if err != nil {
		return nil, fmt.Errorf("检查管理员失败")
	}
	if hasAdmin {
		return nil, fmt.Errorf("管理员已存在，请直接登录")
	}

	user := &AuthUser{
		Username:     username,
		PasswordHash: hashPassword(password),
		Role:         "admin",
		IsActive:     1,
	}
	if err := s.repo.Create(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	token, err := s.signJWT(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("签发令牌失败")
	}

	deviceName := parseDevice(userAgent)
	expiresAtStr := time.Now().Add(time.Duration(config.AppCfg.JWT.ExpireDays) * 24 * time.Hour).Format("2006-01-02 15:04:05")
	session := &AuthSession{
		UserID:     user.ID,
		TokenHash:  hashToken(token),
		DeviceName: deviceName,
		IPAddress:  ipAddr,
		UserAgent:  userAgent,
		ExpiresAt:  &expiresAtStr,
	}
	s.repo.CreateSession(session)

	return &LoginResponse{
		Token:    token,
		Username: user.Username,
		Role:     user.Role,
	}, nil
}

func (s *service) GetMe(token string) (*UserInfoResponse, error) {
	claims, err := s.parseJWT(token)
	if err != nil {
		return nil, fmt.Errorf("令牌无效")
	}

	user, err := s.repo.FindByID(claims.UserId)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	resp := &UserInfoResponse{
		ID:             user.ID,
		Username:       user.Username,
		Role:           user.Role,
		CreatedAt:      user.CreatedAt,
		JwtExpiryDays:  config.AppCfg.JWT.ExpireDays,
		SessionTimeout: 1440,
		RequireAuth:    true,
	}
	if user.LastLoginAt != nil {
		resp.LastLoginAt = *user.LastLoginAt
	}
	return resp, nil
}

func (s *service) Logout(token string) error {
	return s.repo.DeleteSessionByHash(hashToken(token))
}

func (s *service) HasAdmin() (bool, error) {
	return s.repo.HasAdmin()
}

func (s *service) ListSessions(token string) ([]SessionResponse, error) {
	claims, err := s.parseJWT(token)
	if err != nil {
		return nil, fmt.Errorf("令牌无效")
	}
	return s.repo.ListSessions(claims.UserId)
}

func (s *service) RevokeSession(token string, sessionID int) error {

	currentHash := hashToken(token)
	var tokHash string
	s.db.Table("auth_sessions").Select("token_hash").Where("id = ?", sessionID).Scan(&tokHash)
	if tokHash == currentHash {
		return fmt.Errorf("不能撤销当前会话")
	}
	return s.repo.DeleteSession(sessionID)
}

func (s *service) RevokeAllSessions(token string) error {
	claims, err := s.parseJWT(token)
	if err != nil {
		return fmt.Errorf("令牌无效")
	}
	currentHash := hashToken(token)
	return s.repo.DeleteOtherSessions(claims.UserId, currentHash)
}

func (s *service) ChangePassword(token, oldPassword, newPassword string) error {
	claims, err := s.parseJWT(token)
	if err != nil {
		return fmt.Errorf("令牌无效")
	}

	user, err := s.repo.FindByID(claims.UserId)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if !verifyPassword(oldPassword, user.PasswordHash) {
		return fmt.Errorf("旧密码不正确")
	}

	return s.repo.UpdatePassword(claims.UserId, hashPassword(newPassword))
}

func parseDevice(ua string) string {
	if ua == "" {
		return "Unknown"
	}
	if strings.Contains(ua, "Android") {
		return "Android"
	}
	if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") {
		return "iOS"
	}
	if strings.Contains(ua, "Mobile") {
		return "Mobile"
	}
	return "Desktop"
}
