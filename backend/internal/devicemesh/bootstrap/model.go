package bootstrap

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TicketStatus string

const (
	TicketActive   TicketStatus = "active"
	TicketConsumed TicketStatus = "consumed"
	TicketExpired  TicketStatus = "expired"
	TicketRevoked  TicketStatus = "revoked"
)

type BootstrapTicket struct {
	TicketID   string
	TicketHash string
	UserID     runtimeidentity.UserID
	DeviceID   runtimeidentity.DeviceID
	RuntimeID  runtimeidentity.RuntimeID
	Platform   runtimeidentity.Platform
	Status     TicketStatus
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func GenerateRawTicket() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("bootstrap: generate random: %w", err)
	}
	return "amt_mesh_bt_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashRawTicket(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func ParsePlatform(raw string) (runtimeidentity.Platform, error) {
	return runtimeidentity.ParsePlatform(raw)
}
