package artifact

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidMediaTicket = errors.New("artifact: invalid media ticket")

type mediaTicketPayload struct {
	ArtifactID   ID     `json:"artifactId"`
	OwnerSpaceID string `json:"ownerSpaceId"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type mediaTicketSigner struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

func newMediaTicketSigner() *mediaTicketSigner {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return &mediaTicketSigner{
		key: key,
		ttl: time.Hour,
		now: time.Now,
	}
}

func (s *mediaTicketSigner) Issue(id ID, ownerSpaceID string) (string, time.Time, error) {
	if s == nil || id == "" || strings.TrimSpace(ownerSpaceID) == "" {
		return "", time.Time{}, ErrInvalidMediaTicket
	}
	expiresAt := s.now().UTC().Add(s.ttl)
	payload, err := json.Marshal(mediaTicketPayload{
		ArtifactID:   id,
		OwnerSpaceID: ownerSpaceID,
		ExpiresAt:    expiresAt.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := s.sign(encoded)
	return encoded + "." + signature, expiresAt, nil
}

func (s *mediaTicketSigner) Validate(ticket string, id ID) (string, error) {
	if s == nil || id == "" {
		return "", ErrInvalidMediaTicket
	}
	parts := strings.Split(strings.TrimSpace(ticket), ".")
	if len(parts) != 2 {
		return "", ErrInvalidMediaTicket
	}
	expected := s.sign(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return "", ErrInvalidMediaTicket
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", ErrInvalidMediaTicket
	}
	var parsed mediaTicketPayload
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", ErrInvalidMediaTicket
	}
	if parsed.ArtifactID != id || strings.TrimSpace(parsed.OwnerSpaceID) == "" || parsed.ExpiresAt <= s.now().UTC().Unix() {
		return "", ErrInvalidMediaTicket
	}
	return parsed.OwnerSpaceID, nil
}

func (s *mediaTicketSigner) sign(value string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
