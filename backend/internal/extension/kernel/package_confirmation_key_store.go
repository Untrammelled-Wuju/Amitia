package kernel

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type confirmationKeyEntry struct {
	KeyID     string
	Secret    []byte
	Algorithm string
	State     string
}

type PackageConfirmationKeyStore struct {
	db        *sql.DB
	mu        sync.RWMutex
	activeKey *confirmationKeyEntry
	allKeys   map[string]*confirmationKeyEntry
}

func NewPackageConfirmationKeyStore(db *sql.DB) *PackageConfirmationKeyStore {
	return &PackageConfirmationKeyStore{db: db, allKeys: make(map[string]*confirmationKeyEntry)}
}

func (s *PackageConfirmationKeyStore) Init(ctx context.Context) error {
	if s.db == nil {
		return errors.New("kernel: confirmation key store requires database")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key_id, secret_reference, algorithm, state FROM package_confirmation_keys WHERE state IN ('active','rotating') ORDER BY created_at DESC`)
	if err != nil {
		return fmt.Errorf("kernel: load confirmation keys: %w", err)
	}
	defer rows.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allKeys = make(map[string]*confirmationKeyEntry)
	for rows.Next() {
		var keyID, secretRef, algorithm, state string
		if err := rows.Scan(&keyID, &secretRef, &algorithm, &state); err != nil {
			return fmt.Errorf("kernel: scan confirmation key: %w", err)
		}
		secret, err := base64.RawURLEncoding.DecodeString(secretRef)
		if err != nil {
			continue
		}
		entry := &confirmationKeyEntry{KeyID: keyID, Secret: secret, Algorithm: algorithm, State: state}
		s.allKeys[keyID] = entry
		if state == "active" && s.activeKey == nil {
			s.activeKey = entry
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("kernel: iterate confirmation keys: %w", err)
	}
	if s.activeKey == nil {
		return s.createInitialKey(ctx)
	}
	return nil
}

func (s *PackageConfirmationKeyStore) createInitialKey(ctx context.Context) error {
	keyID := "pkg-confirm-" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("kernel: generate confirmation key: %w", err)
	}
	secretRef := base64.RawURLEncoding.EncodeToString(secret)
	now := time.Now().UTC().Unix()
	_, err := s.db.ExecContext(ctx, `INSERT INTO package_confirmation_keys (key_id, secret_reference, algorithm, state, active_from, created_at) VALUES (?, ?, 'hmac-sha256', 'active', ?, ?)`,
		keyID, secretRef, now, now)
	if err != nil {
		return fmt.Errorf("kernel: persist confirmation key: %w", err)
	}
	entry := &confirmationKeyEntry{KeyID: keyID, Secret: secret, Algorithm: "hmac-sha256", State: "active"}
	s.allKeys[keyID] = entry
	s.activeKey = entry
	return nil
}

func (s *PackageConfirmationKeyStore) Rotate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeKey == nil {
		return errors.New("kernel: no active confirmation key to rotate")
	}
	keyID := "pkg-confirm-" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("kernel: generate rotated confirmation key: %w", err)
	}
	secretRef := base64.RawURLEncoding.EncodeToString(secret)
	now := time.Now().UTC().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("kernel: begin confirmation key rotation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE package_confirmation_keys SET state='rotating', expires_at=? WHERE key_id=?`, now+86400, s.activeKey.KeyID); err != nil {
		return fmt.Errorf("kernel: mark old confirmation key rotating: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO package_confirmation_keys (key_id, secret_reference, algorithm, state, active_from, created_at) VALUES (?, ?, 'hmac-sha256', 'active', ?, ?)`,
		keyID, secretRef, now, now); err != nil {
		return fmt.Errorf("kernel: persist rotated confirmation key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("kernel: commit confirmation key rotation: %w", err)
	}
	oldKey := s.activeKey
	oldKey.State = "rotating"
	entry := &confirmationKeyEntry{KeyID: keyID, Secret: secret, Algorithm: "hmac-sha256", State: "active"}
	s.allKeys[keyID] = entry
	s.activeKey = entry
	return nil
}

func (s *PackageConfirmationKeyStore) signConfirmation(claims packageConfirmationClaims) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.activeKey == nil {
		return "", errors.New("kernel: no active confirmation key")
	}
	claims.KeyID = s.activeKey.KeyID
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.activeKey.Secret)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature + "." + s.activeKey.KeyID, nil
}

func (s *PackageConfirmationKeyStore) verifyConfirmation(token string) (packageConfirmationClaims, error) {
	var claims packageConfirmationClaims
	parts := strings.Split(token, ".")
	var encoded, signatureStr, keyID string
	if len(parts) == 3 {
		encoded = parts[0]
		signatureStr = parts[1]
		keyID = parts[2]
	} else if len(parts) == 2 {
		encoded = parts[0]
		signatureStr = parts[1]
	} else {
		return claims, fmt.Errorf("kernel: confirmation token invalid")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	provided, err := base64.RawURLEncoding.DecodeString(signatureStr)
	if err != nil {
		return claims, fmt.Errorf("kernel: confirmation token invalid")
	}
	if keyID != "" {
		entry, ok := s.allKeys[keyID]
		if !ok {
			return claims, fmt.Errorf("kernel: confirmation key not found")
		}
		return s.verifyWithKey(entry, encoded, provided)
	}
	for _, entry := range s.allKeys {
		if result, err := s.verifyWithKey(entry, encoded, provided); err == nil {
			return result, nil
		}
	}
	return claims, fmt.Errorf("kernel: confirmation token invalid")
}

func (s *PackageConfirmationKeyStore) verifyWithKey(entry *confirmationKeyEntry, encoded string, provided []byte) (packageConfirmationClaims, error) {
	var claims packageConfirmationClaims
	mac := hmac.New(sha256.New, entry.Secret)
	_, _ = mac.Write([]byte(encoded))
	if !hmac.Equal(mac.Sum(nil), provided) {
		return claims, fmt.Errorf("kernel: confirmation token invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return claims, fmt.Errorf("kernel: confirmation token invalid")
	}
	if claims.ExpiresAt <= time.Now().UTC().Unix() {
		return claims, fmt.Errorf("kernel: confirmation token expired")
	}
	return claims, nil
}

func (s *PackageConfirmationKeyStore) HasActiveKey() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeKey != nil
}
