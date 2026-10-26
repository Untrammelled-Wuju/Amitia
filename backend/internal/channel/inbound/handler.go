package inbound

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/serviceauth"
	"github.com/u-ai/backend/internal/interaction"
)

const inboundProviderCapability capability.CapabilityID = "channel.provider"

type ProviderDefinitionSource interface {
	ListByCapability(capabilityID capability.CapabilityID) []*capability.CapabilityProviderDefinition
}

type UnifiedEntryHandler interface {
	Handle(ctx context.Context, req *interaction.UnifiedEntryRequest) (*interaction.OrchestrationResult, error)
}

type UnifiedEntryScopeResolver interface {
	ResolveScope(ctx context.Context, req *interaction.UnifiedEntryRequest) (interaction.ScopeResolution, error)
}

type InboundRequest struct {
	ChannelID           string   `json:"channelId"`
	AccountID           string   `json:"accountId,omitempty"`
	ConversationID      string   `json:"conversationId,omitempty"`
	PeerID              string   `json:"peerId"`
	MessageID           string   `json:"messageId,omitempty"`
	Text                string   `json:"text,omitempty"`
	ContentType         string   `json:"contentType,omitempty"`
	ImageURL            string   `json:"imageUrl,omitempty"`
	VideoURL            string   `json:"videoUrl,omitempty"`
	AudioURL            string   `json:"audioUrl,omitempty"`
	AudioDuration       float64  `json:"audioDuration,omitempty"`
	ReplyToMessageID    string   `json:"replyToMessageId,omitempty"`
	ReplyExcerpt        string   `json:"replyExcerpt,omitempty"`
	SenderName          string   `json:"senderName,omitempty"`
	SenderID            string   `json:"senderId,omitempty"`
	Mentions            []string `json:"mentions,omitempty"`
	SandboxWorkspaceID  string   `json:"sandboxWorkspaceId,omitempty"`
	SandboxRoot         string   `json:"sandboxRoot,omitempty"`
	SandboxAllowNetwork bool     `json:"sandboxAllowNetwork,omitempty"`
	FileURL             string   `json:"fileUrl,omitempty"`
	FileName            string   `json:"fileName,omitempty"`
	ConversationTitle   string   `json:"conversationTitle,omitempty"`
	PeerKind            string   `json:"peerKind,omitempty"`
	SpaceID             string   `json:"spaceId,omitempty"`
	CharacterID         string   `json:"characterId,omitempty"`
	Source              string   `json:"source,omitempty"`
}

type SandboxResolver func(spaceID, channelID string) (workspaceID string, root string, allowNetwork bool, enabled bool)

type InboundHandler struct {
	providers ProviderDefinitionSource
	entry     UnifiedEntryHandler
	owner     OwnerResolver
	sandbox   SandboxResolver
}

func (h *InboundHandler) SetSandboxResolver(resolver SandboxResolver) {
	if h == nil {
		return
	}
	h.sandbox = resolver
}

type OwnerResolver func(context.Context, string, string, string) (context.Context, error)

func RegisterInboundRouter(router gin.IRouter, providers ProviderDefinitionSource, entry UnifiedEntryHandler, owners ...OwnerResolver) {
	if router == nil {
		return
	}
	handler := &InboundHandler{providers: providers, entry: entry}
	if len(owners) > 0 {
		handler.owner = owners[0]
	}
	router.POST("/api/channels/inbound", handler.Handle)
}

func RegisterInboundRouterWithSandbox(router gin.IRouter, providers ProviderDefinitionSource, entry UnifiedEntryHandler, resolver SandboxResolver, owners ...OwnerResolver) {
	if router == nil {
		return
	}
	handler := &InboundHandler{providers: providers, entry: entry}
	if len(owners) > 0 {
		handler.owner = owners[0]
	}
	handler.SetSandboxResolver(resolver)
	router.POST("/api/channels/inbound", handler.Handle)
}

func (h *InboundHandler) Handle(c *gin.Context) {
	if h == nil || h.entry == nil || h.providers == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": http.StatusServiceUnavailable, "msg": "channel inbound service unavailable"})
		return
	}
	if !isLoopbackRequest(c.Request.RemoteAddr) {
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "channel inbound requires loopback access"})
		return
	}

	extensionID := strings.TrimSpace(c.GetHeader("X-Amitia-Extension-ID"))
	moduleID := strings.TrimSpace(c.GetHeader("X-Amitia-Module-ID"))
	if extensionID == "" || moduleID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "channel provider identity is required"})
		return
	}
	if !validInboundToken(c.GetHeader("Authorization"), extensionID, moduleID) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "invalid channel provider credential"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var request InboundRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "invalid channel inbound payload"})
		return
	}
	normalized, err := normalizeInboundRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}
	if !providerDefinitionMatches(h.providers.ListByCapability(inboundProviderCapability), extensionID, moduleID, normalized.ChannelID) {
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "channel provider identity does not match the declared provider"})
		return
	}

	entryRequest := buildUnifiedEntryRequest(extensionID, moduleID, normalized)
	ctx := c.Request.Context()
	if resolver, ok := h.entry.(UnifiedEntryScopeResolver); ok {
		resolution, resolveErr := resolver.ResolveScope(ctx, entryRequest)
		if resolveErr != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"code": http.StatusUnprocessableEntity, "msg": resolveErr.Error()})
			return
		}
		if resolution.Scope.SpaceID != "" {
			entryRequest.SpaceID = resolution.Scope.SpaceID
		}
		if resolution.Scope.CharacterID != "" {
			entryRequest.CharacterID = resolution.Scope.CharacterID
		}
		if resolution.Scope.ConversationID != "" {
			entryRequest.ConversationID = resolution.Scope.ConversationID
		}
	}
	if h.sandbox != nil {
		if workspaceID, root, allowNetwork, enabled := h.sandbox(entryRequest.SpaceID, normalized.ChannelID); enabled && workspaceID != "" && root != "" {
			entryRequest.SandboxWorkspaceID = workspaceID
			entryRequest.SandboxRoot = root
			entryRequest.SandboxAllowNetwork = allowNetwork
		}
	}
	if h.owner != nil {
		ctx, err = h.owner(ctx, extensionID, moduleID, normalized.ChannelID)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": err.Error()})
			return
		}
	}
	result, err := h.entry.Handle(ctx, entryRequest)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": http.StatusBadGateway, "msg": err.Error()})
		return
	}
	response := gin.H{
		"conversationId": entryRequest.ConversationID,
		"requestId":      entryRequest.RequestID,
	}
	if result != nil {
		response["interactionId"] = result.InteractionID
		response["outcome"] = result.Outcome
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": response})
}

func validInboundToken(authorization, extensionID, moduleID string) bool {
	parts := strings.Fields(strings.TrimSpace(authorization))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	expected, err := serviceauth.Token(extensionID, moduleID)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) == 1
}

func normalizeInboundRequest(request InboundRequest) (InboundRequest, error) {
	request.ChannelID = strings.TrimSpace(request.ChannelID)
	request.AccountID = strings.TrimSpace(request.AccountID)
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.PeerID = strings.TrimSpace(request.PeerID)
	request.MessageID = strings.TrimSpace(request.MessageID)
	request.Text = strings.TrimSpace(request.Text)
	request.ContentType = strings.ToLower(strings.TrimSpace(request.ContentType))
	request.ImageURL = strings.TrimSpace(request.ImageURL)
	request.VideoURL = strings.TrimSpace(request.VideoURL)
	request.AudioURL = strings.TrimSpace(request.AudioURL)
	request.ReplyToMessageID = strings.TrimSpace(request.ReplyToMessageID)
	request.SpaceID = strings.TrimSpace(request.SpaceID)
	request.CharacterID = strings.TrimSpace(request.CharacterID)
	request.Source = strings.ToLower(strings.TrimSpace(request.Source))
	if request.ChannelID == "" {
		return request, errors.New("channelId is required")
	}
	if request.PeerID == "" {
		return request, errors.New("peerId is required")
	}
	if request.ContentType == "" {
		request.ContentType = "text"
	}
	switch request.ContentType {
	case "text":
		if request.Text == "" {
			return request, errors.New("text is required")
		}
	case "image":
		if request.ImageURL == "" {
			return request, errors.New("imageUrl is required")
		}
	case "video":
		if request.VideoURL == "" {
			return request, errors.New("videoUrl is required")
		}
	case "audio", "voice":
		if request.AudioURL == "" {
			return request, errors.New("audioUrl is required")
		}
	default:
		return request, errors.New("unsupported contentType")
	}
	return request, nil
}

func buildUnifiedEntryRequest(extensionID, moduleID string, request InboundRequest) *interaction.UnifiedEntryRequest {
	requestID := strings.TrimSpace(request.MessageID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	requestID = strings.Join([]string{extensionID, moduleID, request.ChannelID, requestID}, ":")
	source := request.Source
	if source == "" {
		source = "channel"
	}
	conversationID := request.ConversationID
	if conversationID == "" {
		conversationID = derivedConversationID(request)
	}
	entryRequest := &interaction.UnifiedEntryRequest{
		Channel:             request.ChannelID,
		Message:             request.Text,
		PeerID:              request.PeerID,
		SpaceID:             request.SpaceID,
		Source:              source,
		CharacterID:         request.CharacterID,
		ConversationID:      conversationID,
		RequestID:           requestID,
		SessionID:           request.AccountID,
		ImageUrl:            request.ImageURL,
		VideoUrl:            request.VideoURL,
		AudioUrl:            request.AudioURL,
		AudioDuration:       request.AudioDuration,
		VoiceMessage:        request.ContentType == "audio" || request.ContentType == "voice",
		ReplyToMessageID:    optionalString(request.ReplyToMessageID),
		ReplyExcerpt:        strings.TrimSpace(request.ReplyExcerpt),
		SenderName:          strings.TrimSpace(request.SenderName),
		SenderID:            strings.TrimSpace(request.SenderID),
		MentionNames:        request.Mentions,
		PeerKind:            strings.TrimSpace(request.PeerKind),
		ConversationTitle:   strings.TrimSpace(request.ConversationTitle),
		SandboxWorkspaceID:  strings.TrimSpace(request.SandboxWorkspaceID),
		SandboxRoot:         strings.TrimSpace(request.SandboxRoot),
		SandboxAllowNetwork: request.SandboxAllowNetwork,
		FileUrl:             strings.TrimSpace(request.FileURL),
		FileName:            strings.TrimSpace(request.FileName),
	}
	return entryRequest
}

func derivedConversationID(request InboundRequest) string {
	seed := strings.Join([]string{request.ChannelID, request.AccountID, request.PeerID}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	return "channel-" + hex.EncodeToString(sum[:16])
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func providerDefinitionMatches(definitions []*capability.CapabilityProviderDefinition, extensionID, moduleID, channelID string) bool {
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		if strings.TrimSpace(definition.ExtensionID) != extensionID || strings.TrimSpace(definition.ModuleID) != moduleID {
			continue
		}
		if definition.CapabilityID != inboundProviderCapability {
			continue
		}
		if providerChannelID(definition) == channelID {
			return true
		}
	}
	return false
}

func providerChannelID(definition *capability.CapabilityProviderDefinition) string {
	if definition == nil {
		return ""
	}
	if value := metadataString(definition.Metadata, "channelId"); value != "" {
		return value
	}
	if value := metadataString(definition.Metadata, "id"); value != "" {
		return value
	}
	labels, _ := definition.Metadata["labels"].(map[string]any)
	return metadataString(labels, "channelId")
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func isLoopbackRequest(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
