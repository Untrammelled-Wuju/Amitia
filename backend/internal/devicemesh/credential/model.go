package credential

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CredentialStatus string

const (
	CredentialActive  CredentialStatus = "active"
	CredentialRevoked CredentialStatus = "revoked"
	CredentialExpired CredentialStatus = "expired"
)

type DeviceRuntimeCredential struct {
	ID             string
	UserID         runtimeidentity.UserID
	DeviceID       runtimeidentity.DeviceID
	RuntimeID      runtimeidentity.RuntimeID
	CredentialHash string
	Status         CredentialStatus
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastUsedAt     time.Time
	RevokedAt      *time.Time
	Revision       int64
}

func GenerateRawCredential() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("credential: generate random: %w", err)
	}
	return "amt_mesh_dc_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashRawCredential(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
