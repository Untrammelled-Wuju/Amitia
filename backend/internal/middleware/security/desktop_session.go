// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"gorm.io/gorm"
)

const (
	DesktopSessionStatusActive  = "active"
	DesktopSessionStatusRevoked = "revoked"
	DesktopSessionStatusExpired = "expired"

	DesktopSessionDuration      = 15 * time.Minute
	DesktopSessionRenewalWindow = 2 * time.Minute
)

const (
	RotationStagePrepared           = "prepared"
	RotationStageCredentialSwitched = "credential_switched"
	RotationStageSessionsRevoked    = "sessions_revoked"
	RotationStageCompleted          = "completed"
)

type RotationJournal struct {
	ID          string     `gorm:"primaryKey;column:id"`
	OldVersion  string     `gorm:"column:old_version;not null"`
	NewVersion  string     `gorm:"column:new_version;not null"`
	Stage       string     `gorm:"column:stage;not null"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
}

func (RotationJournal) TableName() string {
	return "desktop_pet_token_rotation_journal"
}

type DesktopSession struct {
	ID                string     `gorm:"primaryKey;column:id"`
	UserID            string     `gorm:"column:user_id;not null"`
	DesktopInstanceID string     `gorm:"column:desktop_instance_id;not null"`
	TokenHash         string     `gorm:"column:token_hash;not null"`
	Status            string     `gorm:"column:status;not null;default:active"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;not null"`
	LastUsedAt        time.Time  `gorm:"column:last_used_at"`
	RevokedAt         *time.Time `gorm:"column:revoked_at"`
}

func (DesktopSession) TableName() string {
	return "desktop_pet_local_sessions"
}

type DesktopSessionService struct {
	mu          sync.RWMutex
	db          *gorm.DB
	dataDir     string
	credentials *LocalCredentialStore
}

func NewDesktopSessionService(db *gorm.DB, dataDir string, credentials *LocalCredentialStore) (*DesktopSessionService, error) {
	if db == nil {
		return nil, errors.New("db is required")
	}
	if dataDir == "" {
		return nil, errors.New("dataDir is required")
	}
	svc := &DesktopSessionService{db: db, dataDir: dataDir, credentials: credentials}
	if err := svc.ensureTable(); err != nil {
		return nil, fmt.Errorf("ensure sessions table: %w", err)
	}
	return svc, nil
}

func (s *DesktopSessionService) ensureTable() error {
	return nil
}

func (s *DesktopSessionService) ReadinessCheck(ctx context.Context) error {
	if s.db == nil {
		return errors.New("database is nil")
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

func (s *DesktopSessionService) resolveLocalToken(userID string) (string, error) {
	tokenFile := filepath.Join(s.dataDir, "security", "local-token")
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("read local token: %w", err)
	}
	token := string(data)
	if len(token) < 32 {
		return "", errors.New("local token too short")
	}
	return token, nil
}

func (s *DesktopSessionService) validateLocalToken(token string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expected, err := s.resolveLocalToken("")
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(token))
	expectedHash := sha256.Sum256([]byte(expected))
	if subtle.ConstantTimeCompare(hash[:], expectedHash[:]) != 1 {
		return "", errors.New("invalid local token")
	}
	return expected, nil
}

type createSessionRequest struct {
	DesktopInstanceID string `json:"desktopInstanceId" binding:"required"`
}

type createSessionResponse struct {
	SessionToken string    `json:"sessionToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func (s *DesktopSessionService) CreateSession(c *gin.Context) {
	actor := GetActor(c)
	if actor == nil || !actor.IsLocalTrusted {
		c.JSON(403, gin.H{"code": 403, "msg": "forbidden"})
		return
	}

	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "invalid request"})
		return
	}

	expected, err := s.resolveLocalToken("")
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "local token not configured"})
		return
	}
	_ = expected

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "failed to generate session"})
		return
	}
	sessionToken := base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(sessionToken))
	tokenHashStr := base64.RawURLEncoding.EncodeToString(tokenHash[:])

	now := time.Now()
	session := &DesktopSession{
		ID:                generateSessionID(),
		UserID:            actor.UserID,
		DesktopInstanceID: req.DesktopInstanceID,
		TokenHash:         tokenHashStr,
		Status:            DesktopSessionStatusActive,
		CreatedAt:         now,
		ExpiresAt:         now.Add(DesktopSessionDuration),
		LastUsedAt:        now,
	}

	if err := s.db.Create(session).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "failed to store session"})
		return
	}

	c.JSON(200, createSessionResponse{
		SessionToken: sessionToken,
		ExpiresAt:    session.ExpiresAt,
	})
}

func (s *DesktopSessionService) ValidateSessionWithContext(ctx context.Context, sessionToken string) (*DesktopSession, error) {
	if sessionToken == "" {
		return nil, errors.New("empty session token")
	}

	hash := sha256.Sum256([]byte(sessionToken))
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:])

	var session DesktopSession
	if err := s.db.WithContext(ctx).Where("token_hash = ? AND status = ?", hashStr, DesktopSessionStatusActive).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		_ = s.revokeSessionWithContext(ctx, &session)
		return nil, errors.New("session expired")
	}

	return &session, nil
}

func (s *DesktopSessionService) TouchSessionWithContext(ctx context.Context, session *DesktopSession) error {
	now := time.Now()
	result := s.db.WithContext(ctx).Model(session).Update("last_used_at", now)
	return result.Error
}

func (s *DesktopSessionService) RenewSessionWithContext(ctx context.Context, session *DesktopSession) error {
	if time.Until(session.ExpiresAt) > DesktopSessionRenewalWindow {
		return nil
	}
	newExpiry := time.Now().Add(DesktopSessionDuration)
	result := s.db.WithContext(ctx).Model(session).Update("expires_at", newExpiry)
	return result.Error
}

func (s *DesktopSessionService) revokeSessionWithContext(ctx context.Context, session *DesktopSession) error {
	now := time.Now()
	result := s.db.WithContext(ctx).Model(session).Updates(map[string]interface{}{
		"status":     DesktopSessionStatusRevoked,
		"revoked_at": now,
	})
	return result.Error
}

func (s *DesktopSessionService) ValidateSessionWithInstance(ctx context.Context, sessionToken, instanceID string) (*DesktopSession, error) {
	session, err := s.ValidateSessionWithContext(ctx, sessionToken)
	if err != nil {
		return nil, err
	}
	if instanceID == "" {
		return nil, errors.New("instance header required")
	}
	if session.DesktopInstanceID != instanceID {
		return nil, errors.New("instance mismatch")
	}
	return session, nil
}

func (s *DesktopSessionService) TouchSession(session *DesktopSession) error {
	now := time.Now()
	result := s.db.Model(session).Update("last_used_at", now)
	return result.Error
}

func (s *DesktopSessionService) RenewSession(session *DesktopSession) error {
	if time.Until(session.ExpiresAt) > DesktopSessionRenewalWindow {
		return nil
	}
	newExpiry := time.Now().Add(DesktopSessionDuration)
	result := s.db.Model(session).Update("expires_at", newExpiry)
	return result.Error
}

func (s *DesktopSessionService) revokeSession(session *DesktopSession) error {
	now := time.Now()
	result := s.db.Model(session).Updates(map[string]interface{}{
		"status":     DesktopSessionStatusRevoked,
		"revoked_at": now,
	})
	return result.Error
}

func (s *DesktopSessionService) RevokeSession(sessionToken string) error {
	if sessionToken == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(sessionToken))
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:])

	var session DesktopSession
	if err := s.db.Where("token_hash = ? AND status = ?", hashStr, DesktopSessionStatusActive).First(&session).Error; err != nil {
		return err
	}
	return s.revokeSession(&session)
}

func (s *DesktopSessionService) RotateToken(c *gin.Context) {
	actor := GetActor(c)
	if actor == nil || !actor.IsLocalTrusted || actor.AuthMethod != AuthMethodLocalToken {
		c.JSON(403, gin.H{"code": 403, "msg": "forbidden"})
		return
	}

	if s.credentials == nil {
		c.JSON(500, gin.H{"code": 500, "msg": "credential store not initialized"})
		return
	}

	oldVersion := s.credentials.Version()

	newToken, err := generateSecureToken()
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "failed to generate token"})
		return
	}

	newVersion := credentialVersion(newToken)

	journal := &RotationJournal{
		ID:         generateJournalID(),
		OldVersion: oldVersion,
		NewVersion: newVersion,
		Stage:      RotationStagePrepared,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.db.Create(journal).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "failed to create rotation journal"})
		return
	}

	rotatedOldVersion, rotatedNewVersion, err := s.credentials.Rotate(newToken)
	if err != nil {
		_ = s.db.Delete(journal).Error
		c.JSON(500, gin.H{"code": 500, "msg": "failed to rotate credential"})
		return
	}

	if rotatedOldVersion != oldVersion || rotatedNewVersion != newVersion {
		_ = s.db.Delete(journal).Error
		c.JSON(500, gin.H{"code": 500, "msg": "credential rotation version mismatch"})
		return
	}

	journal.Stage = RotationStageCredentialSwitched
	journal.UpdatedAt = time.Now().UTC()
	if err := s.db.Model(journal).Updates(map[string]interface{}{
		"stage":      RotationStageCredentialSwitched,
		"updated_at": journal.UpdatedAt,
	}).Error; err != nil {
		_ = s.db.Delete(journal).Error
		c.JSON(500, gin.H{"code": 500, "msg": "failed to update journal"})
		return
	}

	if err := s.db.Model(&DesktopSession{}).Where("status = ?", DesktopSessionStatusActive).Updates(map[string]interface{}{
		"status":     DesktopSessionStatusRevoked,
		"revoked_at": time.Now().UTC(),
	}).Error; err != nil {
		_ = s.db.Delete(journal).Error
		c.JSON(500, gin.H{"code": 500, "msg": "failed to revoke old sessions"})
		return
	}

	journal.Stage = RotationStageSessionsRevoked
	journal.UpdatedAt = time.Now().UTC()
	_ = s.db.Model(journal).Updates(map[string]interface{}{
		"stage":      RotationStageSessionsRevoked,
		"updated_at": journal.UpdatedAt,
	}).Error

	now := time.Now().UTC()
	journal.Stage = RotationStageCompleted
	journal.UpdatedAt = now
	journal.CompletedAt = &now
	_ = s.db.Model(journal).Updates(map[string]interface{}{
		"stage":        RotationStageCompleted,
		"updated_at":   now,
		"completed_at": now,
	}).Error

	c.JSON(200, gin.H{"code": 200, "msg": "token rotated"})
}

func (s *DesktopSessionService) RecoverRotationJournals(ctx context.Context) error {
	if s.credentials == nil {
		return errors.New("credential store not initialized")
	}
	currentVersion := s.credentials.Version()
	var journals []RotationJournal
	if err := s.db.WithContext(ctx).Where("stage != ?", RotationStageCompleted).Find(&journals).Error; err != nil {
		return err
	}
	for _, journal := range journals {
		switch journal.Stage {
		case RotationStagePrepared:
			if currentVersion == journal.OldVersion {
				_ = s.db.WithContext(ctx).Delete(&journal).Error
			} else {
				return fmt.Errorf("recovery: prepared journal with mismatched old version")
			}
		case RotationStageCredentialSwitched:
			if currentVersion == journal.NewVersion {
				if err := s.db.WithContext(ctx).Model(&DesktopSession{}).Where("status = ?", DesktopSessionStatusActive).Updates(map[string]interface{}{
					"status":     DesktopSessionStatusRevoked,
					"revoked_at": time.Now().UTC(),
				}).Error; err != nil {
					return fmt.Errorf("recovery: failed to revoke sessions: %w", err)
				}
				now := time.Now().UTC()
				_ = s.db.WithContext(ctx).Model(&journal).Updates(map[string]interface{}{
					"stage":        RotationStageCompleted,
					"updated_at":   now,
					"completed_at": now,
				}).Error
			} else {
				return fmt.Errorf("recovery: credential_switched journal with mismatched new version")
			}
		case RotationStageSessionsRevoked:
			if currentVersion == journal.NewVersion {
				now := time.Now().UTC()
				_ = s.db.WithContext(ctx).Model(&journal).Updates(map[string]interface{}{
					"stage":        RotationStageCompleted,
					"updated_at":   now,
					"completed_at": now,
				}).Error
			} else {
				return fmt.Errorf("recovery: sessions_revoked journal with mismatched new version")
			}
		default:
			return fmt.Errorf("recovery: journal version mismatch, current=%s old=%s new=%s", currentVersion, journal.OldVersion, journal.NewVersion)
		}
	}
	return nil
}

func generateJournalID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("journal ID generation failed: %v", err))
	}
	return "rj_" + base64.RawURLEncoding.EncodeToString(b)
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "v1." + base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *DesktopSessionService) atomicWriteToken(token string) error {
	tokenFile := filepath.Join(s.dataDir, "security", "local-token")

	dir := filepath.Dir(tokenFile)
	tmpFile, err := os.CreateTemp(dir, "token-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(0600); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmpFile.WriteString(token); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, tokenFile); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

var sessionIDRand = func() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sess_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func generateSessionID() string {
	id, err := sessionIDRand()
	if err != nil {
		panic(fmt.Sprintf("session ID generation failed: %v", err))
	}
	return id
}

var requestIDRand = func() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "req_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func generateActorRequestID() string {
	id, err := requestIDRand()
	if err != nil {
		panic(fmt.Sprintf("request ID generation failed: %v", err))
	}
	return id
}

func DesktopSessionAuthMiddleware(sessionSvc *DesktopSessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sessionSvc == nil {
			c.JSON(500, gin.H{"code": 500, "msg": "session service not initialized"})
			c.Abort()
			return
		}

		sessionToken := c.GetHeader("X-Amitia-Desktop-Session")
		if sessionToken == "" {
			c.JSON(401, gin.H{"code": 401, "msg": "missing session"})
			c.Abort()
			return
		}

		session, err := sessionSvc.ValidateSessionWithContext(c.Request.Context(), sessionToken)
		if err != nil {
			c.JSON(401, gin.H{"code": 401, "msg": "invalid session"})
			c.Abort()
			return
		}

		_ = sessionSvc.TouchSessionWithContext(c.Request.Context(), session)

		actor := buildSessionActor(session)
		applySessionActorToContext(c, actor)
	}
}

func buildSessionActor(session *DesktopSession) *auth.ActorContext {
	perms := auth.DefaultUserPermissions()
	perms = append(perms, auth.PermDesktopPetRepair)

	return &auth.ActorContext{
		ActorType:      auth.ActorTypeLocalUser,
		UserID:         session.UserID,
		Roles:          []string{"local_user", "user"},
		Permissions:    perms,
		AuthMethod:     AuthMethodDesktopSession,
		SessionID:      session.ID,
		RequestID:      generateActorRequestID(),
		CorrelationID:  "",
		IsLocalTrusted: true,
	}
}

func applySessionActorToContext(c *gin.Context, actor *auth.ActorContext) {
	c.Set("actorContext", actor)
	c.Set("userId", actor.UserID)
	c.Set("username", actor.UserID)
	c.Set("role", actor.Roles[0])
	c.Set("desktopSession", actor.SessionID)
	c.Set("actorUserID", actor.UserID)
	ctx := auth.WithActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}
