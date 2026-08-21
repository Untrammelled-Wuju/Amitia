package accountsession

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

type SessionRepository interface {
	Create(session *Session) error
	GetByPublicID(publicID string) (*Session, error)
	GetByID(id int64) (*Session, error)
	ListUserSessions(userID int64) ([]Session, error)
	UpdateRevision(publicID string, revision int64) error
	Revoke(publicID, reason string) error
	RevokeOwned(userID int64, publicID, reason string) error
	RevokeAllUser(userID int64, reason string) error
	RevokeAllUserExcept(userID int64, exceptPublicID, reason string) error
	TouchLastActive(publicID string) error
	UpdateLastRefreshed(publicID string) error
	CountActiveSessions(userID int64) (int64, error)
	GetOldestActiveSession(userID int64) (*Session, error)
	ExpireSessions(before time.Time) (int64, error)
	MarkLegacySessionsRevoked(userID int64) error
}

type RefreshRepository interface {
	Create(token *RefreshToken) error
	GetByHash(hash string) (*RefreshToken, error)
	GetByID(tokenID string) (*RefreshToken, error)
	MarkUsed(tokenID string, replacedBy string) error
	RevokeBySession(sessionID string) error
	RevokeByID(tokenID string) error
	RevokeByTokenID(tokenID string) error
	ExpireTokens(before time.Time) (int64, error)
}

type GuardRepository interface {
	GetGuard(dimension, key string) (*LoginGuard, error)
	RecordFailure(dimension, key string, window time.Duration) (*LoginGuard, error)
	ClearGuard(dimension, key string) error
	BlockGuard(dimension, key string, until time.Time) error
	CleanExpired(before time.Time) error
}

type RecoveryRepository interface {
	CreateBatch(userID int64, hashes []string, generation int64, expiresAt *time.Time) error
	ConsumeCode(userID int64, hash string) (bool, *RecoveryCode, error)
	RevokeUnused(userID int64) error
	ListUserCodes(userID int64) ([]RecoveryCode, error)
}

type GrantRepository interface {
	CreateGrant(grant *RecoveryGrant) error
	GetByHash(hash string) (*RecoveryGrant, error)
	Consume(grantID string) error
	ExpireGrants(before time.Time) (int64, error)
}

type sessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(session *Session) error {
	return r.db.Create(session).Error
}

func (r *sessionRepository) GetByPublicID(publicID string) (*Session, error) {
	var s Session
	err := r.db.Where("public_id = ?", publicID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

func (r *sessionRepository) GetByID(id int64) (*Session, error) {
	var s Session
	err := r.db.First(&s, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

func (r *sessionRepository) ListUserSessions(userID int64) ([]Session, error) {
	var sessions []Session
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

func (r *sessionRepository) UpdateRevision(publicID string, revision int64) error {
	return r.db.Model(&Session{}).Where("public_id = ?", publicID).Update("revision", revision).Error
}

func (r *sessionRepository) Revoke(publicID, reason string) error {
	now := time.Now().UTC()
	return r.db.Model(&Session{}).Where("public_id = ? AND status = ?", publicID, SessionStatusActive).
		Updates(map[string]interface{}{
			"status":        SessionStatusRevoked,
			"revoked_at":    now,
			"revoke_reason": reason,
		}).Error
}

func (r *sessionRepository) RevokeOwned(userID int64, publicID, reason string) error {
	now := time.Now().UTC()
	return r.db.Model(&Session{}).Where("public_id = ? AND user_id = ? AND status = ?", publicID, userID, SessionStatusActive).
		Updates(map[string]interface{}{
			"status":        SessionStatusRevoked,
			"revoked_at":    now,
			"revoke_reason": reason,
		}).Error
}

func (r *sessionRepository) RevokeAllUser(userID int64, reason string) error {
	now := time.Now().UTC()
	return r.db.Model(&Session{}).Where("user_id = ? AND status = ?", userID, SessionStatusActive).
		Updates(map[string]interface{}{
			"status":        SessionStatusRevoked,
			"revoked_at":    now,
			"revoke_reason": reason,
		}).Error
}

func (r *sessionRepository) RevokeAllUserExcept(userID int64, exceptPublicID, reason string) error {
	now := time.Now().UTC()
	return r.db.Model(&Session{}).Where("user_id = ? AND status = ? AND public_id != ?", userID, SessionStatusActive, exceptPublicID).
		Updates(map[string]interface{}{
			"status":        SessionStatusRevoked,
			"revoked_at":    now,
			"revoke_reason": reason,
		}).Error
}

func (r *sessionRepository) TouchLastActive(publicID string) error {
	now := time.Now().UTC()
	return r.db.Model(&Session{}).Where("public_id = ? AND status = ?", publicID, SessionStatusActive).
		Update("last_active_at", now).Error
}

func (r *sessionRepository) UpdateLastRefreshed(publicID string) error {
	now := time.Now().UTC()
	return r.db.Model(&Session{}).Where("public_id = ?", publicID).
		Updates(map[string]interface{}{
			"last_refreshed_at": now,
			"status":            SessionStatusActive,
		}).Error
}

func (r *sessionRepository) CountActiveSessions(userID int64) (int64, error) {
	var count int64
	err := r.db.Model(&Session{}).Where("user_id = ? AND status = ?", userID, SessionStatusActive).Count(&count).Error
	return count, err
}

func (r *sessionRepository) GetOldestActiveSession(userID int64) (*Session, error) {
	var s Session
	err := r.db.Where("user_id = ? AND status = ?", userID, SessionStatusActive).Order("created_at ASC").First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

func (r *sessionRepository) ExpireSessions(before time.Time) (int64, error) {
	result := r.db.Model(&Session{}).Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", SessionStatusActive, before).
		Update("status", SessionStatusExpired)
	return result.RowsAffected, result.Error
}

func (r *sessionRepository) MarkLegacySessionsRevoked(userID int64) error {
	now := time.Now().UTC()
	return r.db.Model(&Session{}).Where("user_id = ? AND status = ? AND public_id = ''", userID, SessionStatusActive).
		Updates(map[string]interface{}{
			"status":        SessionStatusRevoked,
			"revoked_at":    now,
			"revoke_reason": "legacy_session_migrated",
		}).Error
}

type refreshRepository struct {
	db *gorm.DB
}

func NewRefreshRepository(db *gorm.DB) RefreshRepository {
	return &refreshRepository{db: db}
}

func (r *refreshRepository) Create(token *RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *refreshRepository) GetByHash(hash string) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.Where("token_hash = ?", hash).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &t, err
}

func (r *refreshRepository) GetByID(tokenID string) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.Where("token_id = ?", tokenID).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &t, err
}

func (r *refreshRepository) MarkUsed(tokenID string, replacedBy string) error {
	now := time.Now().UTC()
	return r.db.Model(&RefreshToken{}).Where("token_id = ? AND status = ?", tokenID, RefreshStatusActive).
		Updates(map[string]interface{}{
			"status":               RefreshStatusUsed,
			"used_at":              now,
			"replaced_by_token_id": replacedBy,
		}).Error
}

func (r *refreshRepository) RevokeBySession(sessionID string) error {
	now := time.Now().UTC()
	return r.db.Model(&RefreshToken{}).Where("session_id = ? AND (status = ? OR status = ?)", sessionID, RefreshStatusActive, RefreshStatusUsed).
		Updates(map[string]interface{}{
			"status":     RefreshStatusRevoked,
			"revoked_at": now,
		}).Error
}

func (r *refreshRepository) RevokeByID(tokenID string) error {
	now := time.Now().UTC()
	return r.db.Model(&RefreshToken{}).Where("token_id = ? AND (status = ? OR status = ?)", tokenID, RefreshStatusActive, RefreshStatusUsed).
		Updates(map[string]interface{}{
			"status":     RefreshStatusRevoked,
			"revoked_at": now,
		}).Error
}

func (r *refreshRepository) RevokeByTokenID(tokenID string) error {
	now := time.Now().UTC()
	return r.db.Model(&RefreshToken{}).Where("token_id = ? AND status = ?", tokenID, RefreshStatusActive).
		Updates(map[string]interface{}{
			"status":     RefreshStatusRevoked,
			"revoked_at": now,
		}).Error
}

func (r *refreshRepository) ExpireTokens(before time.Time) (int64, error) {
	result := r.db.Model(&RefreshToken{}).Where("status = ? AND expires_at < ?", RefreshStatusActive, before).
		Update("status", RefreshStatusExpired)
	return result.RowsAffected, result.Error
}

type guardRepository struct {
	db *gorm.DB
}

func NewGuardRepository(db *gorm.DB) GuardRepository {
	return &guardRepository{db: db}
}

func (r *guardRepository) GetGuard(dimension, key string) (*LoginGuard, error) {
	var g LoginGuard
	err := r.db.Where("guard_key = ? AND dimension = ?", key, dimension).First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &g, err
}

func (r *guardRepository) RecordFailure(dimension, key string, window time.Duration) (*LoginGuard, error) {
	now := time.Now().UTC()
	windowStart := now.Add(-window)
	var g LoginGuard
	err := r.db.Where("guard_key = ? AND dimension = ?", key, dimension).First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		g = LoginGuard{
			GuardKey:        key,
			Dimension:       dimension,
			FailureCount:    1,
			WindowStartedAt: now,
		}
		if createErr := r.db.Create(&g).Error; createErr != nil {
			return nil, createErr
		}
		return &g, nil
	}
	if err != nil {
		return nil, err
	}
	if g.WindowStartedAt.Before(windowStart) {
		g.FailureCount = 1
		g.WindowStartedAt = now
	} else {
		g.FailureCount++
	}
	g.UpdatedAt = now
	if err := r.db.Save(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *guardRepository) ClearGuard(dimension, key string) error {
	return r.db.Where("guard_key = ? AND dimension = ?", key, dimension).Delete(&LoginGuard{}).Error
}

func (r *guardRepository) BlockGuard(dimension, key string, until time.Time) error {
	return r.db.Model(&LoginGuard{}).Where("guard_key = ? AND dimension = ?", key, dimension).
		Updates(map[string]interface{}{
			"blocked_until": until,
			"updated_at":    time.Now().UTC(),
		}).Error
}

func (r *guardRepository) CleanExpired(before time.Time) error {
	return r.db.Where("blocked_until IS NOT NULL AND blocked_until < ?", before).Delete(&LoginGuard{}).Error
}

type recoveryRepository struct {
	db *gorm.DB
}

func NewRecoveryRepository(db *gorm.DB) RecoveryRepository {
	return &recoveryRepository{db: db}
}

func (r *recoveryRepository) CreateBatch(userID int64, hashes []string, generation int64, expiresAt *time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RecoveryCode{}).Where("user_id = ? AND status = ?", userID, RecoveryStatusActive).
			Update("status", RecoveryStatusRevoked).Error; err != nil {
			return err
		}
		for _, h := range hashes {
			code := RecoveryCode{
				CodeID:     generateCodeID(),
				UserID:     userID,
				CodeHash:   h,
				Status:     RecoveryStatusActive,
				ExpiresAt:  expiresAt,
				Generation: generation,
			}
			if err := tx.Create(&code).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *recoveryRepository) ConsumeCode(userID int64, hash string) (bool, *RecoveryCode, error) {
	var code RecoveryCode
	query := r.db.Where("code_hash = ? AND status = ?", hash, RecoveryStatusActive)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.First(&code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	if code.ExpiresAt != nil && code.ExpiresAt.Before(time.Now().UTC()) {
		return false, &code, nil
	}
	now := time.Now().UTC()
	result := r.db.Model(&RecoveryCode{}).Where("code_id = ? AND status = ?", code.CodeID, RecoveryStatusActive).
		Updates(map[string]interface{}{
			"status":  RecoveryStatusUsed,
			"used_at": now,
		})
	if result.RowsAffected != 1 {
		return false, nil, nil
	}
	code.Status = RecoveryStatusUsed
	code.UsedAt = &now
	return true, &code, nil
}

func (r *recoveryRepository) RevokeUnused(userID int64) error {
	return r.db.Model(&RecoveryCode{}).Where("user_id = ? AND status = ?", userID, RecoveryStatusActive).
		Update("status", RecoveryStatusRevoked).Error
}

func (r *recoveryRepository) ListUserCodes(userID int64) ([]RecoveryCode, error) {
	var codes []RecoveryCode
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&codes).Error
	return codes, err
}

type grantRepository struct {
	db *gorm.DB
}

func NewGrantRepository(db *gorm.DB) GrantRepository {
	return &grantRepository{db: db}
}

func (r *grantRepository) CreateGrant(grant *RecoveryGrant) error {
	return r.db.Create(grant).Error
}

func (r *grantRepository) GetByHash(hash string) (*RecoveryGrant, error) {
	var g RecoveryGrant
	err := r.db.Where("grant_hash = ?", hash).First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &g, err
}

func (r *grantRepository) Consume(grantID string) error {
	now := time.Now().UTC()
	return r.db.Model(&RecoveryGrant{}).Where("grant_id = ? AND status = ?", grantID, GrantStatusActive).
		Updates(map[string]interface{}{
			"status":      GrantStatusConsumed,
			"consumed_at": now,
		}).Error
}

func (r *grantRepository) ExpireGrants(before time.Time) (int64, error) {
	result := r.db.Model(&RecoveryGrant{}).Where("status = ? AND expires_at < ?", GrantStatusActive, before).
		Update("status", GrantStatusExpired)
	return result.RowsAffected, result.Error
}

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func generateCodeID() string {
	return GeneratePublicID("rc_")
}
