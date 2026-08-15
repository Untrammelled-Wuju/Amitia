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
)

const recoveryCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type RecoveryService struct {
	codes RecoveryRepository
	grants GrantRepository
	audit  AuditLogger
}

func NewRecoveryService(codes RecoveryRepository, grants GrantRepository, audit AuditLogger) RecoveryService {
	return RecoveryService{codes: codes, grants: grants, audit: audit}
}

type GeneratedCode struct {
	Raw        string
	Generation int64
}

func (s RecoveryService) GenerateCodes(userID int64) ([]GeneratedCode, int64, error) {
	codes := make([]GeneratedCode, 8)
	hashes := make([]string, 8)
	for i := 0; i < 8; i++ {
		raw := generateRecoveryCode()
		hashes[i] = hashRecoveryCode(raw)
		codes[i] = GeneratedCode{Raw: raw}
	}
	err := s.codes.CreateBatch(userID, hashes, time.Now().Unix(), nil)
	if err != nil {
		return nil, 0, err
	}
	s.audit.LogRecoveryCodesGenerated(userID, len(codes))
	return codes, 0, nil
}

type ConsumeResult struct {
	Success  bool
	CodeID   string
	UserID   int64
}

func (s RecoveryService) ConsumeCode(code string) ConsumeResult {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	hashValue := hashRecoveryCode(normalized)
	success, rec, _ := s.codes.ConsumeCode(0, hashValue)
	if !success || rec == nil {
		return ConsumeResult{Success: false}
	}
	return ConsumeResult{
		Success: true,
		CodeID:  rec.CodeID,
		UserID:  rec.UserID,
	}
}

func (s RecoveryService) CreateRecoveryGrant(userID int64) (string, time.Time, error) {
	raw, err := GenerateOpaqueToken("amt_rg_")
	if err != nil {
		return "", time.Time{}, err
	}
	hashValue := hashRecoveryCode(raw)
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	grant := &RecoveryGrant{
		GrantID:   GeneratePublicID("rg_"),
		UserID:    userID,
		GrantHash: hashValue,
		Status:    GrantStatusActive,
		ExpiresAt: expiresAt,
	}
	if err := s.grants.CreateGrant(grant); err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}

func (s RecoveryService) ConsumeGrant(rawToken string) (string, bool) {
	hashValue := hashRecoveryCode(rawToken)
	grant, err := s.grants.GetByHash(hashValue)
	if err != nil || grant == nil {
		return "", false
	}
	if grant.Status != GrantStatusActive {
		return "", false
	}
	if grant.ExpiresAt.Before(time.Now().UTC()) {
		return "", false
	}
	_ = s.grants.Consume(grant.GrantID)
	return fmt.Sprintf("%d", grant.UserID), true
}

func generateRecoveryCode() string {
	b := make([]byte, 20)
	rand.Read(b)
	parts := make([]string, 4)
	for i := 0; i < 4; i++ {
		var val uint64
		for j := 0; j < 4; j++ {
			val = val<<8 | uint64(b[i*4+j])
		}
		code := make([]byte, 4)
		for j := 0; j < 4; j++ {
			code[j] = recoveryCodeAlphabet[val%uint64(len(recoveryCodeAlphabet))]
			val /= uint64(len(recoveryCodeAlphabet))
		}
		parts[i] = string(code)
	}
	return strings.Join(parts, "-")
}

func hashRecoveryCode(raw string) string {
	pepper := config.AppCfg.Security.RecoveryPepper
	if pepper == "" {
		pepper = config.AppCfg.JWT.Secret
	}
	h := hmac.New(sha256.New, []byte(pepper))
	h.Write([]byte(strings.ToLower(strings.TrimSpace(raw))))
	return hex.EncodeToString(h.Sum(nil))
}
