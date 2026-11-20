package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

var errInvalidChatAttachmentTicket = errors.New("invalid chat attachment media ticket")

type chatAttachmentMediaTicketPayload struct {
	ArtifactID          string `json:"artifactId"`
	SpaceID             string `json:"spaceId"`
	DeviceID            string `json:"deviceId"`
	CoreID              string `json:"coreId"`
	ConversationID      string `json:"conversationId"`
	MessageID           string `json:"messageId"`
	CharacterID         string `json:"characterId"`
	ConversationOwnerID string `json:"conversationOwnerId,omitempty"`
	HistoricalRoleID    string `json:"historicalRoleId,omitempty"`
	ExpiresAt           int64  `json:"expiresAt"`
}

type chatAttachmentMediaTicketSigner struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

func newChatAttachmentMediaTicketSigner() (*chatAttachmentMediaTicketSigner, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate chat attachment media ticket key: %w", err)
	}
	return &chatAttachmentMediaTicketSigner{key: key, ttl: 15 * time.Minute, now: time.Now}, nil
}

func (s *chatAttachmentMediaTicketSigner) Issue(payload chatAttachmentMediaTicketPayload) (string, time.Time, error) {
	if s == nil || payload.ArtifactID == "" || payload.SpaceID == "" || payload.DeviceID == "" || payload.CoreID == "" || payload.ConversationID == "" || payload.MessageID == "" || payload.CharacterID == "" {
		return "", time.Time{}, errInvalidChatAttachmentTicket
	}
	expiresAt := s.now().UTC().Add(s.ttl)
	payload.ExpiresAt = expiresAt.Unix()
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("encode chat attachment media ticket: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(encodedPayload)
	return encoded + "." + s.sign(encoded), expiresAt, nil
}

func (s *chatAttachmentMediaTicketSigner) Validate(ticket, artifactID string) (chatAttachmentMediaTicketPayload, error) {
	var payload chatAttachmentMediaTicketPayload
	parts := strings.Split(strings.TrimSpace(ticket), ".")
	if s == nil || artifactID == "" || len(parts) != 2 || !hmac.Equal([]byte(s.sign(parts[0])), []byte(parts[1])) {
		return payload, errInvalidChatAttachmentTicket
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return payload, fmt.Errorf("decode chat attachment media ticket: %w", err)
	}
	if len(decoded) > 8192 {
		return payload, errInvalidChatAttachmentTicket
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return payload, fmt.Errorf("parse chat attachment media ticket: %w", err)
	}
	if payload.ArtifactID != artifactID || payload.ExpiresAt <= s.now().UTC().Unix() {
		return payload, errInvalidChatAttachmentTicket
	}
	return payload, nil
}

func (s *chatAttachmentMediaTicketSigner) sign(value string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func registerChatAttachmentPublicMedia(engine *gin.Engine, services *AppServices, coreID string) {
	if engine == nil || services == nil || services.ChatAttachmentTickets == nil || services.OwnedBusiness == nil {
		return
	}
	engine.GET("/media/chat-attachments/:artifactId/:ticket", func(c *gin.Context) {
		payload, err := services.ChatAttachmentTickets.Validate(c.Param("ticket"), strings.TrimSpace(c.Param("artifactId")))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "mesh.invalid_attachment_ticket", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
			return
		}
		serveSavedChatAttachment(c, services, coreID, chatAttachmentReadTarget{
			SpaceID:             payload.SpaceID,
			DeviceID:            payload.DeviceID,
			ConversationID:      payload.ConversationID,
			MessageID:           payload.MessageID,
			ArtifactID:          payload.ArtifactID,
			CharacterID:         payload.CharacterID,
			ConversationOwnerID: payload.ConversationOwnerID,
			HistoricalRoleID:    payload.HistoricalRoleID,
		})
	})
}

type chatAttachmentReadTarget struct {
	SpaceID             string
	DeviceID            string
	ConversationID      string
	MessageID           string
	ArtifactID          string
	CharacterID         string
	ConversationOwnerID string
	HistoricalRoleID    string
}

func resolveSavedChatAttachment(ctx context.Context, services *AppServices, coreID string, target chatAttachmentReadTarget) (business.QueryResult, coordination.ChatAttachmentReference, error) {
	if services == nil || services.OwnedBusiness == nil {
		return business.QueryResult{}, coordination.ChatAttachmentReference{}, errors.New("owned business service unavailable")
	}
	request := business.Request{
		SpaceID:          target.SpaceID,
		DeviceID:         target.DeviceID,
		CoreID:           coreID,
		TargetDeviceID:   target.DeviceID,
		ResourceOwnerID:  target.DeviceID,
		RoleOwnerID:      coreID,
		RoleID:           target.CharacterID,
		RequestID:        uuid.NewString(),
		HistoricalRoleID: target.HistoricalRoleID,
	}
	if target.ConversationOwnerID != "" {
		request.ConversationOrigin = &business.ConversationOrigin{OwnerID: target.ConversationOwnerID, ID: target.ConversationID}
	}
	view, err := services.OwnedBusiness.Query(ctx, request, coordination.DataQuery{
		ConversationID: target.ConversationID,
		ResourceKind:   "message",
		ResourceID:     target.MessageID,
		Limit:          1,
	})
	if err != nil {
		return business.QueryResult{}, coordination.ChatAttachmentReference{}, fmt.Errorf("query saved attachment message: %w", err)
	}
	reference, err := savedChatAttachmentReference(view, target.ConversationID, target.MessageID, target.ArtifactID)
	if err != nil {
		return business.QueryResult{}, coordination.ChatAttachmentReference{}, err
	}
	return view, reference, nil
}

func chatAttachmentTargetFromActor(c *gin.Context, coreID string) (chatAttachmentReadTarget, error) {
	actor := security.GetActor(c)
	if actor == nil || actor.DeviceID == "" || actor.SpaceID == "" {
		return chatAttachmentReadTarget{}, fmt.Errorf("authenticated device is missing: %w", coordination.ErrWrongOwner)
	}
	target := chatAttachmentReadTarget{
		SpaceID:             actor.SpaceID.String(),
		DeviceID:            actor.DeviceID.String(),
		ConversationID:      strings.TrimSpace(c.Query("conversationId")),
		MessageID:           strings.TrimSpace(c.Query("messageId")),
		ArtifactID:          strings.TrimSpace(c.Param("artifactId")),
		CharacterID:         strings.TrimSpace(c.Query("characterId")),
		ConversationOwnerID: strings.TrimSpace(c.Query("conversationOwnerId")),
		HistoricalRoleID:    strings.TrimSpace(c.Query("historicalRoleId")),
	}
	if target.ConversationID == "" || target.MessageID == "" || target.ArtifactID == "" || target.CharacterID == "" {
		return chatAttachmentReadTarget{}, fmt.Errorf("附件读取缺少会话、消息、角色或制品编号: %w", coordination.ErrWrongOwner)
	}
	return target, nil
}
