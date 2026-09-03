// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const realtimeAccessTicketTTL = 60 * time.Second

type realtimeAccessTicketRecord struct {
	UserID         string
	ConversationID string
	DialogID       string
	VoiceType      string
	ResourceID     string
	ExpiresAt      time.Time
}

type realtimeAccessTicketStore struct {
	mu      sync.Mutex
	tickets map[[32]byte]realtimeAccessTicketRecord
}

var realtimeTickets = &realtimeAccessTicketStore{tickets: make(map[[32]byte]realtimeAccessTicketRecord)}

func IssueRealtimeAccessTicket(c *gin.Context) {
	var request struct {
		ConversationID string `json:"conversationId"`
		DialogID       string `json:"dialogId"`
		VoiceType      string `json:"voiceType"`
		ResourceID     string `json:"resourceId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid realtime ticket request"})
		return
	}

	userID := ""
	if value, exists := c.Get("userId"); exists && value != nil {
		userID = strings.TrimSpace(fmt.Sprint(value))
	}
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "authenticated user is required"})
		return
	}

	token, err := newSecureRealtimeToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "failed to create realtime ticket"})
		return
	}
	expiresAt := time.Now().UTC().Add(realtimeAccessTicketTTL)
	realtimeTickets.put(token, realtimeAccessTicketRecord{
		UserID:         userID,
		ConversationID: strings.TrimSpace(request.ConversationID),
		DialogID:       strings.TrimSpace(request.DialogID),
		VoiceType:      strings.TrimSpace(request.VoiceType),
		ResourceID:     strings.TrimSpace(request.ResourceID),
		ExpiresAt:      expiresAt,
	})

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": gin.H{
			"ticket":    token,
			"expiresAt": expiresAt,
			"wsPath":    "/api/realtime/v2/ws/session",
		},
		"message": "ok",
	})
}

func HandleTicketedSession(c *gin.Context) {
	token := strings.TrimSpace(c.Query("ticket"))
	record, ok := realtimeTickets.consume(token)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "invalid or expired realtime access ticket"})
		return
	}

	query := make(url.Values)
	if record.ConversationID != "" {
		query.Set("conversationId", record.ConversationID)
	}
	if record.DialogID != "" {
		query.Set("dialogId", record.DialogID)
	}
	if record.VoiceType != "" {
		query.Set("voiceType", record.VoiceType)
	}
	if record.ResourceID != "" {
		query.Set("resourceId", record.ResourceID)
	}
	c.Request.URL.RawQuery = query.Encode()
	c.Set("realtimeUserId", record.UserID)
	c.Set("realtimeVisualEndpoint", "/api/realtime/v2/ws/visual")
	HandleSession(c)
}

func HandleTicketedVisualSession(c *gin.Context) {
	HandleVisualSession(c)
}

func (s *realtimeAccessTicketStore) put(token string, record realtimeAccessTicketRecord) {
	now := time.Now().UTC()
	hash := sha256.Sum256([]byte(token))
	s.mu.Lock()
	for key, candidate := range s.tickets {
		if !candidate.ExpiresAt.After(now) {
			delete(s.tickets, key)
		}
	}
	s.tickets[hash] = record
	s.mu.Unlock()
}

func (s *realtimeAccessTicketStore) consume(token string) (realtimeAccessTicketRecord, bool) {
	if token == "" {
		return realtimeAccessTicketRecord{}, false
	}
	hash := sha256.Sum256([]byte(token))
	now := time.Now().UTC()
	s.mu.Lock()
	record, ok := s.tickets[hash]
	delete(s.tickets, hash)
	for key, candidate := range s.tickets {
		if !candidate.ExpiresAt.After(now) {
			delete(s.tickets, key)
		}
	}
	s.mu.Unlock()
	if !ok || !record.ExpiresAt.After(now) {
		return realtimeAccessTicketRecord{}, false
	}
	return record, true
}

func newSecureRealtimeToken(byteLength int) (string, error) {
	if byteLength < 16 {
		byteLength = 16
	}
	raw := make([]byte, byteLength)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
