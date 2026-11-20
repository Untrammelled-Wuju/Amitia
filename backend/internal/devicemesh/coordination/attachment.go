package coordination

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const ChatAttachmentChunkMaxBytes int64 = 512 << 10

type ChatAttachmentReference struct {
	ArtifactID      string `json:"artifactId"`
	OwnerDeviceID   string `json:"ownerDeviceId"`
	StorageDeviceID string `json:"storageDeviceId"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	MIMEType        string `json:"mimeType"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"sizeBytes"`
}

type ChatAttachmentReferenceAction string

const (
	ChatAttachmentReferencePrepare ChatAttachmentReferenceAction = "prepare"
	ChatAttachmentReferenceCommit  ChatAttachmentReferenceAction = "commit"
	ChatAttachmentReferenceAbort   ChatAttachmentReferenceAction = "abort"
	ChatAttachmentReferenceRemove  ChatAttachmentReferenceAction = "remove"
)

type ChatAttachmentReferenceBinding struct {
	Scope     ExecutionScope          `json:"scope"`
	Reference ChatAttachmentReference `json:"reference"`
	MessageID string                  `json:"messageId"`
}

func (b ChatAttachmentReferenceBinding) Validate() error {
	if b.Scope.ResourceOwnerID == "" || b.Scope.RoleID == "" || b.Scope.RequestID == "" || b.MessageID == "" || len(b.MessageID) > 512 || strings.ContainsAny(b.MessageID, "\x00\r\n") {
		return fmt.Errorf("invalid chat attachment binding: %w", ErrWrongOwner)
	}
	return b.Reference.Validate()
}

func (b ChatAttachmentReferenceBinding) Token() (string, error) {
	if err := b.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(b)
	if err != nil {
		return "", fmt.Errorf("encode chat attachment binding: %w", err)
	}
	if len(payload) > 16<<10 {
		return "", fmt.Errorf("chat attachment binding exceeds limit: %w", ErrPendingLimit)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func ParseChatAttachmentReferenceBinding(token string) (ChatAttachmentReferenceBinding, error) {
	var binding ChatAttachmentReferenceBinding
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return binding, fmt.Errorf("decode chat attachment binding: %w", err)
	}
	if len(payload) > 16<<10 {
		return binding, fmt.Errorf("chat attachment binding exceeds limit: %w", ErrPendingLimit)
	}
	if err := json.Unmarshal(payload, &binding); err != nil {
		return binding, fmt.Errorf("parse chat attachment binding: %w", err)
	}
	return binding, binding.Validate()
}

func (r ChatAttachmentReference) Validate() error {
	if r.ArtifactID == "" || len(r.ArtifactID) > 256 || strings.ContainsAny(r.ArtifactID, "\x00\r\n/\\") ||
		r.OwnerDeviceID == "" || len(r.OwnerDeviceID) > 256 || strings.ContainsAny(r.OwnerDeviceID, "\x00\r\n/\\") ||
		r.StorageDeviceID == "" || len(r.StorageDeviceID) > 256 || strings.ContainsAny(r.StorageDeviceID, "\x00\r\n/\\") ||
		r.Kind != "image" && r.Kind != "audio" && r.Kind != "video" && r.Kind != "file" ||
		r.Name == "" || len(r.Name) > 256 || strings.ContainsAny(r.Name, "\x00\r\n") ||
		r.MIMEType == "" || len(r.MIMEType) > 256 || strings.ContainsAny(r.MIMEType, "\x00\r\n") ||
		r.SizeBytes < 0 || len(r.SHA256) != 64 || r.SHA256 != strings.ToLower(r.SHA256) {
		return fmt.Errorf("invalid chat attachment reference: %w", ErrWrongOwner)
	}
	if _, err := hex.DecodeString(r.SHA256); err != nil {
		return fmt.Errorf("invalid chat attachment digest: %w", err)
	}
	return nil
}

type ChatAttachmentRead struct {
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
	ArtifactID     string `json:"artifactId"`
	Offset         int64  `json:"offset"`
	Length         int64  `json:"length"`
}

type ChatAttachmentChunk struct {
	ArtifactID string `json:"artifactId"`
	Digest     string `json:"digest"`
	Offset     int64  `json:"offset"`
	Size       int64  `json:"size"`
	Data       []byte `json:"data"`
	EOF        bool   `json:"eof"`
}

type ChatAttachmentPort interface {
	ValidateChatAttachment(context.Context, ExecutionScope, ChatAttachmentReference) error
	ReadChatAttachmentStorageChunk(context.Context, ExecutionScope, ChatAttachmentReference, int64, int64) (ChatAttachmentChunk, error)
	SetChatAttachmentReference(context.Context, ExecutionScope, ChatAttachmentReference, string, ChatAttachmentReferenceAction) error
}

type ChatAttachmentValidator interface {
	ValidateChatAttachment(context.Context, ExecutionScope, ChatAttachmentReference) error
}

type ChatAttachmentReader interface {
	ReadChatAttachmentChunk(context.Context, ExecutionScope, ChatAttachmentRead) (ChatAttachmentChunk, error)
}

type ChatAttachmentReferenceWriter interface {
	SetChatAttachmentReference(context.Context, ExecutionScope, ChatAttachmentReference, string, ChatAttachmentReferenceAction) error
}
