package chat_ui_extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ChatSlotID string

const (
	SlotChatHeaderAction        ChatSlotID = "chat.header.action"
	SlotChatSidebarPanel        ChatSlotID = "chat.sidebar.panel"
	SlotChatMessageAction       ChatSlotID = "chat.message.action"
	SlotChatMessageBadge        ChatSlotID = "chat.message.badge"
	SlotChatMessageAttachment   ChatSlotID = "chat.message.attachment_renderer"
	SlotChatMessageCustom       ChatSlotID = "chat.message.custom_renderer"
	SlotChatComposerAction      ChatSlotID = "chat.composer.action"
	SlotChatComposerAttachment  ChatSlotID = "chat.composer.attachment"
	SlotChatComposerHint        ChatSlotID = "chat.composer.hint"
	SlotChatEmptyStateCard      ChatSlotID = "chat.empty_state.card"
	SlotChatStatusItem          ChatSlotID = "chat.status.item"
)

type ChatUIContext struct {
	CharacterID      string   `json:"characterId"`
	ConversationID   string   `json:"conversationId"`
	Channel          string   `json:"channel"`
	Platform         string   `json:"platform"`
	ConversationState string  `json:"conversationState"`
	Capabilities     []string `json:"capabilities"`
}

type MessageDirection string

const (
	DirectionIncoming MessageDirection = "incoming"
	DirectionOutgoing MessageDirection = "outgoing"
	DirectionSystem   MessageDirection = "system"
)

type SenderType string

const (
	SenderUser      SenderType = "user"
	SenderCharacter SenderType = "character"
	SenderSystem    SenderType = "system"
	SenderExtension SenderType = "extension"
)

type MessageSummary struct {
	MessageID       string           `json:"messageId"`
	Type            string           `json:"type"`
	Direction       MessageDirection `json:"direction"`
	SenderType      SenderType       `json:"senderType"`
	CreatedAt       time.Time        `json:"createdAt"`
	Status          string           `json:"status"`
	HasText         bool             `json:"hasText"`
	AttachmentTypes []string         `json:"attachmentTypes,omitempty"`
	ExtensionType   string           `json:"extensionType,omitempty"`
}

type MessageContent struct {
	MessageID string `json:"messageId"`
	Text      string `json:"text,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
}

type MessageAttachment struct {
	AttachmentID string `json:"attachmentId"`
	MIME         string `json:"mime"`
	Size         int64  `json:"size"`
	Name         string `json:"name"`
	ResourceHandle string `json:"resourceHandle,omitempty"`
}

const (
	PermissionMessageContentRead = "chat.message.content.read"
	PermissionComposerDraftRead  = "chat.composer.draft.read"
	PermissionComposerDraftWrite = "chat.composer.draft.write"
	PermissionMessageSend        = "chat.message.send"
	PermissionAttachmentRead     = "chat.attachment.read"
)

type ChatMessageActionSpec struct {
	ActionID        string   `json:"actionId"`
	ExtensionID     string   `json:"extensionId,omitempty"`
	SupportedTypes  []string `json:"supportedTypes,omitempty"`
	Position        string   `json:"position,omitempty"`
	RequiresContent bool     `json:"requiresContent"`
	Confirmation    string   `json:"confirmation,omitempty"`
	Target          string   `json:"target"`
	RiskLevel       string   `json:"riskLevel,omitempty"`
}

type CustomMessageType struct {
	TypeName       string          `json:"typeName"`
	OwnerExtension string          `json:"ownerExtension"`
	PayloadSchema  json.RawMessage `json:"payloadSchema"`
	Version        int             `json:"version"`
	TextFallback   string          `json:"textFallback"`
	ExportFallback string          `json:"exportFallback"`
	MaxBytes       int64           `json:"maxBytes"`
}

type ChatExtensionRegistry struct {
	mu                 sync.RWMutex
	messageActions     map[string]*ChatMessageActionSpec
	customMessageTypes map[string]*CustomMessageType
	attachmentRenderers map[string]*AttachmentRenderer
}

type AttachmentRenderer struct {
	RendererID    string
	OwnerExtension string
	MIMEPattern   string
	Priority      int
	Kind          string
}

func NewChatExtensionRegistry() *ChatExtensionRegistry {
	return &ChatExtensionRegistry{
		messageActions:     make(map[string]*ChatMessageActionSpec),
		customMessageTypes: make(map[string]*CustomMessageType),
		attachmentRenderers: make(map[string]*AttachmentRenderer),
	}
}

func (r *ChatExtensionRegistry) RegisterMessageAction(spec *ChatMessageActionSpec) error {
	if spec == nil || spec.ActionID == "" {
		return ErrInvalidActionSpec
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.messageActions[spec.ActionID]; exists {
		return fmt.Errorf("%w: %s", ErrActionExists, spec.ActionID)
	}
	r.messageActions[spec.ActionID] = spec
	return nil
}

func (r *ChatExtensionRegistry) RegisterCustomMessageType(mt *CustomMessageType) error {
	if mt == nil || mt.TypeName == "" || mt.OwnerExtension == "" {
		return ErrInvalidCustomType
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := buildCustomTypeKey(mt.OwnerExtension, mt.TypeName)
	if _, exists := r.customMessageTypes[key]; exists {
		return fmt.Errorf("%w: %s", ErrCustomTypeExists, key)
	}
	if mt.MaxBytes <= 0 {
		mt.MaxBytes = 64 * 1024
	}
	if mt.TextFallback == "" {
		mt.TextFallback = "[扩展消息]"
	}
	r.customMessageTypes[key] = mt
	return nil
}

func (r *ChatExtensionRegistry) RegisterAttachmentRenderer(renderer *AttachmentRenderer) error {
	if renderer == nil || renderer.RendererID == "" || renderer.MIMEPattern == "" {
		return ErrInvalidRenderer
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attachmentRenderers[renderer.RendererID] = renderer
	return nil
}

func (r *ChatExtensionRegistry) GetCustomMessageType(ownerExtension, typeName string) (*CustomMessageType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := buildCustomTypeKey(ownerExtension, typeName)
	mt, exists := r.customMessageTypes[key]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrCustomTypeNotFound, key)
	}
	return mt, nil
}

func (r *ChatExtensionRegistry) FindAttachmentRenderer(mime string) *AttachmentRenderer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var best *AttachmentRenderer
	for _, r := range r.attachmentRenderers {
		if matchMIME(r.MIMEPattern, mime) {
			if best == nil || r.Priority > best.Priority {
				best = r
			}
		}
	}
	return best
}

func (r *ChatExtensionRegistry) UnregisterByExtension(extensionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, a := range r.messageActions {
		if a.ExtensionID == extensionID {
			delete(r.messageActions, id)
		}
	}
	for key, mt := range r.customMessageTypes {
		if mt.OwnerExtension == extensionID {
			delete(r.customMessageTypes, key)
		}
	}
	for id, renderer := range r.attachmentRenderers {
		if renderer.OwnerExtension == extensionID {
			delete(r.attachmentRenderers, id)
		}
	}
}

type MessageDataRequest struct {
	MessageID string `json:"messageId"`
	Fields    []string `json:"fields,omitempty"`
}

type MessageQueryService interface {
	QuerySummary(ctx context.Context, messageID string) (*MessageSummary, error)
	QueryContent(ctx context.Context, messageID string, fields []string) (*MessageContent, error)
}

type ComposerDraftCommand struct {
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

const (
	DraftCommandInsertText         = "insert_text"
	DraftCommandReplaceSelection   = "replace_selection"
	DraftCommandAttachResource     = "attach_resource"
	DraftCommandSetExtMetadata    = "set_extension_metadata"
	DraftCommandOpenDialog         = "open_dialog"
)

func ValidateDraftCommand(cmd *ComposerDraftCommand) error {
	if cmd == nil {
		return ErrInvalidDraftCommand
	}
	switch cmd.Type {
	case DraftCommandInsertText, DraftCommandReplaceSelection, DraftCommandAttachResource,
		DraftCommandSetExtMetadata, DraftCommandOpenDialog:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnknownDraftCommand, cmd.Type)
	}
}

type MessageSendRequest struct {
	CharacterID   string            `json:"characterId"`
	ConversationID string           `json:"conversationId"`
	Text          string            `json:"text"`
	Attachments   []MessageAttachment `json:"attachments,omitempty"`
	ExtensionMetadata map[string]json.RawMessage `json:"extensionMetadata,omitempty"`
	ChannelRules  map[string]string `json:"channelRules,omitempty"`
}

func ValidateSendRequest(req *MessageSendRequest, capabilities []string) error {
	if req == nil {
		return ErrInvalidSendRequest
	}
	if req.CharacterID == "" || req.ConversationID == "" {
		return ErrInvalidSendRequest
	}
	if req.Text == "" && len(req.Attachments) == 0 {
		return ErrEmptyMessage
	}
	if len(req.Text) > 64*1024 {
		return ErrMessageTooLarge
	}
	if !hasCapability(capabilities, "text") && req.Text != "" {
		return ErrCapabilityMissing
	}
	if !hasCapability(capabilities, "image") && hasAttachmentType(req.Attachments, "image/") {
		return ErrCapabilityMissing
	}
	if !hasCapability(capabilities, "file") && hasAttachmentType(req.Attachments, "application/") {
		return ErrCapabilityMissing
	}
	return nil
}

type MessageAnnotation struct {
	AnnotationID string          `json:"annotationId"`
	MessageID    string          `json:"messageId"`
	OwnerExtension string        `json:"ownerExtension"`
	Key          string          `json:"key"`
	Value        json.RawMessage `json:"value"`
	CreatedAt    time.Time       `json:"createdAt"`
}

func (a *MessageAnnotation) Validate() error {
	if a.AnnotationID == "" || a.MessageID == "" || a.OwnerExtension == "" || a.Key == "" {
		return ErrInvalidAnnotation
	}
	if strings.HasPrefix(a.Key, "_") {
		return ErrReservedAnnotationKey
	}
	if len(a.Value) > 4*1024 {
		return ErrAnnotationTooLarge
	}
	return nil
}

func buildCustomTypeKey(ownerExtension, typeName string) string {
	return ownerExtension + "/" + typeName
}

func matchMIME(pattern, mime string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(mime, prefix+"/")
	}
	return pattern == mime
}

func hasCapability(caps []string, target string) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}

func hasAttachmentType(attachments []MessageAttachment, prefix string) bool {
	for _, a := range attachments {
		if strings.HasPrefix(a.MIME, prefix) {
			return true
		}
	}
	return false
}

var (
	ErrInvalidActionSpec       = errors.New("chat_ui_extension: invalid action spec")
	ErrActionExists            = errors.New("chat_ui_extension: action exists")
	ErrInvalidCustomType       = errors.New("chat_ui_extension: invalid custom type")
	ErrCustomTypeExists        = errors.New("chat_ui_extension: custom type exists")
	ErrCustomTypeNotFound      = errors.New("chat_ui_extension: custom type not found")
	ErrInvalidRenderer         = errors.New("chat_ui_extension: invalid renderer")
	ErrInvalidDraftCommand     = errors.New("chat_ui_extension: invalid draft command")
	ErrUnknownDraftCommand     = errors.New("chat_ui_extension: unknown draft command")
	ErrInvalidSendRequest      = errors.New("chat_ui_extension: invalid send request")
	ErrEmptyMessage            = errors.New("chat_ui_extension: empty message")
	ErrMessageTooLarge         = errors.New("chat_ui_extension: message too large")
	ErrCapabilityMissing       = errors.New("chat_ui_extension: capability missing")
	ErrInvalidAnnotation       = errors.New("chat_ui_extension: invalid annotation")
	ErrReservedAnnotationKey   = errors.New("chat_ui_extension: reserved annotation key")
	ErrAnnotationTooLarge      = errors.New("chat_ui_extension: annotation too large")
)
