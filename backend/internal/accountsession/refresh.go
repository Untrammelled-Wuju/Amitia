package accountsession

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type RefreshService struct {
	refresh  RefreshRepository
	sessions SessionRepository
	db       *gorm.DB
}

func NewRefreshService(refresh RefreshRepository, sessions SessionRepository, db *gorm.DB) *RefreshService {
	return &RefreshService{
		refresh:  refresh,
		sessions: sessions,
		db:       db,
	}
}

type RefreshResult struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	SessionID        string
	UserID           string
	Username         string
	Role             string
}

func (s *RefreshService) Rotate(rawRefreshToken string) (*RefreshResult, error) {
	tokenHash := hashRefreshToken(rawRefreshToken)
	existing, err := s.refresh.GetByHash(tokenHash)
	if err != nil {
		return nil, ErrRefreshInvalid
	}
	if existing == nil {
		return nil, ErrRefreshInvalid
	}

	if existing.Status == RefreshStatusRevoked || existing.Status == RefreshStatusExpired {
		return nil, ErrRefreshRevoked
	}

	var result *RefreshResult

	err = s.db.Transaction(func(tx *gorm.DB) error {
		freshRefresh := NewRefreshRepository(tx)
		freshSessions := NewSessionRepository(tx)

		if existing.Status == RefreshStatusUsed {
			if err := freshSessions.Revoke(existing.SessionID, "refresh_token_reuse"); err != nil {
				return err
			}
			if err := freshRefresh.RevokeBySession(existing.SessionID); err != nil {
				return err
			}
			return ErrRefreshReused
		}

		if existing.ExpiresAt.Before(time.Now().UTC()) {
			if err := tx.Model(&RefreshToken{}).Where("token_id = ?", existing.TokenID).
				Update("status", RefreshStatusExpired).Error; err != nil {
				return err
			}
			return ErrRefreshExpired
		}

		session, err := freshSessions.GetByPublicID(existing.SessionID)
		if err != nil {
			return ErrSessionNotFound
		}
		if session == nil {
			return ErrSessionNotFound
		}
		if session.Status != SessionStatusActive {
			return ErrSessionRevoked
		}
		if session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now().UTC()) {
			return ErrSessionExpired
		}
		if session.AbsoluteExpiresAt != nil && session.AbsoluteExpiresAt.Before(time.Now().UTC()) {
			return ErrSessionExpired
		}

		newTokenID := GeneratePublicID("rt_")
		newRaw, err := GenerateOpaqueToken("amt_rt_")
		if err != nil {
			return err
		}
		newHash := hashRefreshToken(newRaw)
		now := time.Now().UTC()
		refreshTTL := 30 * 24 * time.Hour
		newExpiresAt := now.Add(refreshTTL)
		if session.AbsoluteExpiresAt != nil && session.AbsoluteExpiresAt.Before(newExpiresAt) {
			newExpiresAt = *session.AbsoluteExpiresAt
		}

		newRecord := &RefreshToken{
			TokenID:   newTokenID,
			SessionID: existing.SessionID,
			TokenHash: newHash,
			Status:    RefreshStatusActive,
			IssuedAt:  now,
			ExpiresAt: newExpiresAt,
		}
		if err := freshRefresh.Create(newRecord); err != nil {
			return err
		}

		if err := freshRefresh.MarkUsed(existing.TokenID, newTokenID); err != nil {
			return err
		}

		if err := freshSessions.UpdateLastRefreshed(existing.SessionID); err != nil {
			return err
		}

		result = &RefreshResult{
			AccessToken:      "",
			RefreshToken:     newRaw,
			RefreshExpiresAt: newExpiresAt,
			SessionID:        existing.SessionID,
			UserID:           fmt.Sprintf("%d", session.UserID),
			Username:         session.Username,
			Role:             session.Role,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *RefreshService) RevokeAllForSession(sessionID string) error {
	return s.refresh.RevokeBySession(sessionID)
}

func (s *RefreshService) RevokeByToken(rawToken string) error {
	tokenHash := hashRefreshToken(rawToken)
	record, err := s.refresh.GetByHash(tokenHash)
	if err != nil {
		return ErrRefreshInvalid
	}
	if record == nil {
		return nil
	}
	if record.Status != RefreshStatusActive {
		return nil
	}
	return s.refresh.RevokeByTokenID(record.TokenID)
}

func hashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func GenerateRefreshToken() (string, string, error) {
	raw, err := GenerateOpaqueToken("amt_rt_")
	if err != nil {
		return "", "", err
	}
	return raw, hashRefreshToken(raw), nil
}
