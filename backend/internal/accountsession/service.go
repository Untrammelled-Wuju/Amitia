package accountsession

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"gorm.io/gorm"
	"golang.org/x/crypto/scrypt"
)

const (
	scryptN = 16384
	scryptR = 8
	scryptP = 1
	saltLen = 32
	keyLen  = 64
)

type PasswordVerifier interface {
	VerifyPassword(password, storedHash string) bool
	HashPassword(password string) string
}

type defaultPasswordVerifier struct{}

func (defaultPasswordVerifier) VerifyPassword(password, storedHash string) bool {
	parts := strings.SplitN(storedHash, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt := []byte(parts[0])
	key, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	dk, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return false
	}
	if len(dk) != len(key) {
		return false
	}
	var diff byte
	for i := 0; i < len(dk); i++ {
		diff |= dk[i] ^ key[i]
	}
	return diff == 0
}

func (defaultPasswordVerifier) HashPassword(password string) string {
	rawSalt := make([]byte, saltLen)
	if _, err := rand.Read(rawSalt); err != nil {
		return ""
	}
	saltHex := hex.EncodeToString(rawSalt)
	dk, _ := scrypt.Key([]byte(password), []byte(saltHex), scryptN, scryptR, scryptP, keyLen)
	return saltHex + ":" + hex.EncodeToString(dk)
}

func HashPassword(password string) string {
	v := defaultPasswordVerifier{}
	return v.HashPassword(password)
}

type AccountUserService interface {
	FindByUsername(username string) (*AuthUserDTO, error)
	FindByID(id int) (*AuthUserDTO, error)
	UpdatePassword(id int, newHash string) error
	UpdateLoginTime(id int) error
	CreateUser(username, password, role string) (*AuthUserDTO, error)
	HasAdmin() (bool, error)
}

type AuthUserDTO struct {
	ID           int
	Username     string
	PasswordHash string
	Role         string
	IsActive     int
}

type AccountSessionService struct {
	db          *gorm.DB
	sessions    SessionRepository
	refresh     RefreshRepository
	refreshSvc  *RefreshService
	audit       AuditLogger
	guard       *LoginGuardService
	recovery    RecoveryService
	maxSessions int
}

type AccountSessionServiceConfig struct {
	Sessions    SessionRepository
	Refresh     RefreshRepository
	Audit       AuditLogger
	Guard       *LoginGuardService
	Recovery    RecoveryService
	UserService AccountUserService
	MaxSessions int
}

func NewAccountSessionService(db *gorm.DB, cfg AccountSessionServiceConfig) *AccountSessionService {
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 10
	}
	svc := &AccountSessionService{
		db:          db,
		sessions:    cfg.Sessions,
		refresh:     cfg.Refresh,
		audit:       cfg.Audit,
		guard:       cfg.Guard,
		recovery:    cfg.Recovery,
		maxSessions: cfg.MaxSessions,
	}
	svc.refreshSvc = NewRefreshService(cfg.Refresh, cfg.Sessions, db)
	return svc
}

func (s *AccountSessionService) RefreshService() *RefreshService {
	return s.refreshSvc
}

func (s *AccountSessionService) SessionRepository() SessionRepository {
	return s.sessions
}

func (s *AccountSessionService) AuditLogger() AuditLogger {
	return s.audit
}

type LoginRequestInternal struct {
	Username  string
	Password  string
	ClientType string
	IPAddress string
	UserAgent string
}

type LoginResponseInternal struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	SessionPublicID  string
	UserID           int
	Username         string
	Role             string
}

func (s *AccountSessionService) Login(req LoginRequestInternal, svc AccountUserService) (*LoginResponseInternal, error) {
	if s.guard != nil {
		ipKey := guardIPKey(req.IPAddress)
		if blocked := s.guard.CheckBlocked("ip", ipKey); blocked {
			if auditErr := s.audit.LogLoginRateLimited(req.IPAddress, req.UserAgent, ""); auditErr != nil {
				return nil, fmt.Errorf("记录审计日志失败: %w", auditErr)
			}
			return nil, ErrLoginRateLimited
		}
		usernameKey := guardUsernameKey(req.Username)
		if blocked := s.guard.CheckBlocked("username", usernameKey); blocked {
			if auditErr := s.audit.LogLoginRateLimited(req.IPAddress, req.UserAgent, req.Username); auditErr != nil {
				return nil, fmt.Errorf("记录审计日志失败: %w", auditErr)
			}
			return nil, ErrLoginRateLimited
		}
	}

	user, err := svc.FindByUsername(req.Username)
	if err != nil {
		if s.guard != nil {
			if guardErr := s.guard.RecordFailure("ip", guardIPKey(req.IPAddress)); guardErr != nil {
				return nil, fmt.Errorf("记录登录失败失败: %w", guardErr)
			}
			if guardErr := s.guard.RecordFailure("username", guardUsernameKey(req.Username)); guardErr != nil {
				return nil, fmt.Errorf("记录登录失败失败: %w", guardErr)
			}
		}
		if auditErr := s.audit.LogLoginFailed(req.IPAddress, req.UserAgent, req.Username, "invalid_credentials"); auditErr != nil {
			return nil, fmt.Errorf("记录审计日志失败: %w", auditErr)
		}
		return nil, ErrInvalidCredentials
	}

	verifier := defaultPasswordVerifier{}
	if !verifier.VerifyPassword(req.Password, user.PasswordHash) {
		if s.guard != nil {
			if guardErr := s.guard.RecordFailure("ip", guardIPKey(req.IPAddress)); guardErr != nil {
				return nil, fmt.Errorf("记录登录失败失败: %w", guardErr)
			}
			if guardErr := s.guard.RecordFailure("username", guardUsernameKey(req.Username)); guardErr != nil {
				return nil, fmt.Errorf("记录登录失败失败: %w", guardErr)
			}
		}
		if auditErr := s.audit.LogLoginFailed(req.IPAddress, req.UserAgent, req.Username, "invalid_credentials"); auditErr != nil {
			return nil, fmt.Errorf("记录审计日志失败: %w", auditErr)
		}
		return nil, ErrInvalidCredentials
	}

	if s.guard != nil {
		if guardErr := s.guard.ClearFailures("ip", guardIPKey(req.IPAddress)); guardErr != nil {
			return nil, fmt.Errorf("清除登录失败记录失败: %w", guardErr)
		}
		if guardErr := s.guard.ClearFailures("username", guardUsernameKey(req.Username)); guardErr != nil {
			return nil, fmt.Errorf("清除登录失败记录失败: %w", guardErr)
		}
	}

	result, err := s.createSessionAndTokens(user.ID, user.Username, user.Role, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}

	if err := svc.UpdateLoginTime(user.ID); err != nil {
		return nil, fmt.Errorf("更新登录时间失败: %w", err)
	}

	if err := s.audit.LogLoginSuccess(user.ID, result.SessionPublicID, req.IPAddress, req.UserAgent, req.Username); err != nil {
		return nil, fmt.Errorf("记录审计日志失败: %w", err)
	}
	return result, nil
}

func (s *AccountSessionService) createSessionAndTokens(userID int, username, role, ip, ua string) (*LoginResponseInternal, error) {
	tokenSvc := NewTokenService()
	sessionPublicID := GeneratePublicID("sess_")
	now := time.Now().UTC()
	expiresAt := now.Add(tokenSvc.AccessTTL())
	absoluteExpiresAt := now.Add(tokenSvc.AbsoluteTTL())
	refreshExpiresAt := now.Add(tokenSvc.RefreshTTL())

	rawRefresh, refreshHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		activeCount, err := s.sessions.CountActiveSessions(int64(userID))
		if err != nil {
			return err
		}
		for activeCount >= int64(s.maxSessions) {
			oldest, err := s.sessions.GetOldestActiveSession(int64(userID))
			if err != nil {
				return err
			}
			if oldest == nil {
				break
			}
			if err := s.sessions.Revoke(oldest.PublicID, "max_sessions"); err != nil {
				return err
			}
			if err := s.refresh.RevokeBySession(oldest.PublicID); err != nil {
				return err
			}
			activeCount--
		}

		session := &Session{
			PublicID:          sessionPublicID,
			UserID:            int64(userID),
			Username:          username,
			Role:              role,
			Status:            SessionStatusActive,
			DeviceName:        parseDeviceName(ua),
			IPAddress:         ip,
			UserAgent:         ua,
			Revision:          1,
			CreatedAt:         now,
			ExpiresAt:         &expiresAt,
			AbsoluteExpiresAt: &absoluteExpiresAt,
		}
		if err := s.sessions.Create(session); err != nil {
			return err
		}

		refreshRecord := &RefreshToken{
			TokenID:   GeneratePublicID("rt_"),
			SessionID: sessionPublicID,
			TokenHash: refreshHash,
			Status:    RefreshStatusActive,
			IssuedAt:  now,
			ExpiresAt: refreshExpiresAt,
		}
		if err := s.refresh.Create(refreshRecord); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	accessToken, accessExpiresAt, err := tokenSvc.SignAccessToken(userID, username, role, sessionPublicID)
	if err != nil {
		if revokeErr := s.sessions.Revoke(sessionPublicID, "token_sign_failure"); revokeErr != nil {
			return nil, fmt.Errorf("令牌签名失败且撤销会话失败: %v: %w", revokeErr, err)
		}
		if revokeErr := s.refresh.RevokeBySession(sessionPublicID); revokeErr != nil {
			return nil, fmt.Errorf("令牌签名失败且撤销刷新令牌失败: %v: %w", revokeErr, err)
		}
		return nil, err
	}

	return &LoginResponseInternal{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     rawRefresh,
		RefreshExpiresAt: refreshExpiresAt,
		SessionPublicID:  sessionPublicID,
		UserID:           userID,
		Username:         username,
		Role:             role,
	}, nil
}

func (s *AccountSessionService) RevokeCurrentSession(sessionPublicID string, userID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.sessions.Revoke(sessionPublicID, "user_logout"); err != nil {
			return err
		}
		if err := s.refresh.RevokeBySession(sessionPublicID); err != nil {
			return err
		}
		if auditErr := s.audit.LogSessionRevoked(sessionPublicID, userID, "logout"); auditErr != nil {
			return auditErr
		}
		return nil
	})
}

func (s *AccountSessionService) RevokeOwnedSession(userID int64, sessionPublicID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.sessions.RevokeOwned(userID, sessionPublicID, "user_revoke"); err != nil {
			return err
		}
		if err := s.refresh.RevokeBySession(sessionPublicID); err != nil {
			return err
		}
		if auditErr := s.audit.LogSessionRevoked(sessionPublicID, userID, "revoke_owned"); auditErr != nil {
			return auditErr
		}
		return nil
	})
}

func (s *AccountSessionService) RevokeOtherSessions(currentSessionID string, userID int64) (int, error) {
	var count int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		sessions, err := s.sessions.ListUserSessions(userID)
		if err != nil {
			return err
		}
		if err := s.sessions.RevokeAllUserExcept(userID, currentSessionID, "logout_other"); err != nil {
			return err
		}
		for _, sess := range sessions {
			if sess.PublicID != currentSessionID && sess.Status == RefreshStatusActive {
				count++
				if err := s.refresh.RevokeBySession(sess.PublicID); err != nil {
					return err
				}
			}
		}
		if auditErr := s.audit.LogSessionsRevoked(currentSessionID, userID, count); auditErr != nil {
			return auditErr
		}
		return nil
	})
	return count, err
}

func (s *AccountSessionService) RevokeAllSessions(userID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.sessions.RevokeAllUser(userID, "logout_all"); err != nil {
			return err
		}
		sessions, err := s.sessions.ListUserSessions(userID)
		if err != nil {
			return err
		}
		for _, sess := range sessions {
			if err := s.refresh.RevokeBySession(sess.PublicID); err != nil {
				return err
			}
		}
		if auditErr := s.audit.LogLogoutAll(userID); auditErr != nil {
			return auditErr
		}
		return nil
	})
}

func (s *AccountSessionService) ListActiveSessions(userID int64) ([]Session, error) {
	return s.sessions.ListUserSessions(userID)
}

func (s *AccountSessionService) GetGuard() *LoginGuardService {
	return s.guard
}

func parseDeviceName(ua string) string {
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

func guardIPKey(ip string) string {
	pepper := config.AppCfg.Security.AuditHmacSecret
	if pepper == "" {
		pepper = config.AppCfg.JWT.Secret
	}
	h := hmac.New(sha256.New, []byte(pepper))
	h.Write([]byte("ip:" + ip))
	return hex.EncodeToString(h.Sum(nil))
}

func guardUsernameKey(username string) string {
	pepper := config.AppCfg.Security.AuditHmacSecret
	if pepper == "" {
		pepper = config.AppCfg.JWT.Secret
	}
	h := hmac.New(sha256.New, []byte(pepper))
	h.Write([]byte("user:" + strings.ToLower(strings.TrimSpace(username))))
	return hex.EncodeToString(h.Sum(nil))
}

var _ = fmt.Sprintf
