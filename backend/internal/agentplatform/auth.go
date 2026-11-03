package agentplatform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

const (
	platformPasswordMemory      = 64 * 1024
	platformPasswordIterations  = 3
	platformPasswordParallelism = 2
	platformPasswordKeyLength   = 32
)

type PlatformLoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type PlatformLoginResult struct {
	User       PlatformUser         `json:"user"`
	Membership []PlatformMembership `json:"memberships"`
	Token      string               `json:"token"`
	ExpiresAt  time.Time            `json:"expiresAt"`
}

func hashPlatformPassword(password string) (string, error) {
	if len(strings.TrimSpace(password)) < 8 {
		return "", errors.New("密码至少需要 8 个字符")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成密码盐: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, platformPasswordIterations, platformPasswordMemory, platformPasswordParallelism, platformPasswordKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", platformPasswordMemory, platformPasswordIterations, platformPasswordParallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPlatformPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
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
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

func (s *Service) LoginPlatformUser(ctx context.Context, spaceID string, input PlatformLoginInput) (PlatformLoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	var user PlatformUser
	if err := s.db.WithContext(ctx).Where("space_id = ? AND LOWER(email) = ? AND status = ?", spaceID, email, "active").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PlatformLoginResult{}, errors.New("账号或密码错误")
		}
		return PlatformLoginResult{}, fmt.Errorf("读取平台账号: %w", err)
	}
	if user.AuthHash == "" || !verifyPlatformPassword(user.AuthHash, input.Password) {
		return PlatformLoginResult{}, errors.New("账号或密码错误")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return PlatformLoginResult{}, fmt.Errorf("生成平台会话: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	now := time.Now().UTC()
	session := PlatformSession{
		ID: newID("platformsession"), SpaceID: spaceID, UserID: user.ID,
		TokenHash: hex.EncodeToString(sum[:]), Status: "active",
		ExpiresAt: now.Add(12 * time.Hour), CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return PlatformLoginResult{}, fmt.Errorf("保存平台会话: %w", err)
	}
	var memberships []PlatformMembership
	if err := s.db.WithContext(ctx).Where("space_id = ? AND user_id = ? AND status = ?", spaceID, user.ID, "active").Find(&memberships).Error; err != nil {
		return PlatformLoginResult{}, fmt.Errorf("读取平台权限: %w", err)
	}
	return PlatformLoginResult{User: user, Membership: memberships, Token: token, ExpiresAt: session.ExpiresAt}, nil
}

func (s *Service) ResolvePlatformSession(ctx context.Context, token string) (PlatformUser, []PlatformMembership, error) {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	var session PlatformSession
	if err := s.db.WithContext(ctx).Where("token_hash = ? AND status = ? AND expires_at > ?", hex.EncodeToString(sum[:]), "active", time.Now().UTC()).First(&session).Error; err != nil {
		return PlatformUser{}, nil, errors.New("平台会话无效或已经过期")
	}
	var user PlatformUser
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ? AND status = ?", session.UserID, session.SpaceID, "active").First(&user).Error; err != nil {
		return PlatformUser{}, nil, fmt.Errorf("读取平台会话用户: %w", err)
	}
	var memberships []PlatformMembership
	if err := s.db.WithContext(ctx).Where("space_id = ? AND user_id = ? AND status = ?", session.SpaceID, user.ID, "active").Find(&memberships).Error; err != nil {
		return PlatformUser{}, nil, fmt.Errorf("读取平台会话权限: %w", err)
	}
	return user, memberships, nil
}

func (s *Service) LogoutPlatformSession(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&PlatformSession{}).Where("token_hash = ? AND status = ?", hex.EncodeToString(sum[:]), "active").Updates(map[string]any{"status": "revoked", "revoked_at": now})
	if result.Error != nil {
		return fmt.Errorf("注销平台会话: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.New("平台会话不存在或已经注销")
	}
	return nil
}
