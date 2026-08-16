package accountsession

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"gorm.io/gorm"
)

type LoginGuardService struct {
	repo           GuardRepository
	ipMaxFailures  int
	userMaxFailures int
	windowDuration  time.Duration
	blockDuration   time.Duration
}

func NewLoginGuardService(repo GuardRepository) *LoginGuardService {
	return &LoginGuardService{
		repo:            repo,
		ipMaxFailures:   10,
		userMaxFailures: 5,
		windowDuration:  10 * time.Minute,
		blockDuration:   15 * time.Minute,
	}
}

func (g *LoginGuardService) CheckBlocked(dimension, key string) bool {
	guard, err := g.repo.GetGuard(dimension, key)
	if err != nil || guard == nil {
		return false
	}
	if guard.BlockedUntil != nil && guard.BlockedUntil.After(time.Now().UTC()) {
		return true
	}
	return false
}

func (g *LoginGuardService) RecordFailure(dimension, key string) error {
	guard, err := g.repo.RecordFailure(dimension, key, g.windowDuration)
	if err != nil || guard == nil {
		return err
	}
	maxFailures := g.ipMaxFailures
	if dimension == "username" {
		maxFailures = g.userMaxFailures
	}
	if guard.FailureCount >= int64(maxFailures) {
		until := time.Now().UTC().Add(g.blockDuration)
		if err := g.repo.BlockGuard(dimension, key, until); err != nil {
			return err
		}
	}
	return nil
}

func (g *LoginGuardService) ClearFailures(dimension, key string) error {
	return g.repo.ClearGuard(dimension, key)
}

func GuardKey(dimension, value string) string {
	pepper := config.AppCfg.Security.AuditHmacSecret
	if pepper == "" {
		pepper = config.AppCfg.JWT.Secret
	}
	h := hmac.New(sha256.New, []byte(pepper))
	h.Write([]byte(dimension + ":" + strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(h.Sum(nil))
}

var _ = gorm.ErrRecordNotFound
