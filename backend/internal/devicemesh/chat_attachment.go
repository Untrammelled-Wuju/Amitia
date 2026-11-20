package devicemesh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) ValidateChatAttachment(ctx context.Context, scope coordination.ExecutionScope, reference coordination.ChatAttachmentReference) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	if reference.OwnerDeviceID != scope.InitiatorDeviceID {
		return fmt.Errorf("attachment uploader does not match initiator: %w", coordination.ErrWrongOwner)
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return fmt.Errorf("validate attachment scope: %w", err)
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, scope); err != nil {
		return err
	}
	if err := rt.validateChatAttachmentStorage(ctx, scope, reference); err != nil {
		return err
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return err
	}
	return coordination.ValidateRoleRevision(ctx, rt, scope)
}

func (rt *Runtime) ReadChatAttachmentChunk(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChatAttachmentRead) (coordination.ChatAttachmentChunk, error) {
	var chunk coordination.ChatAttachmentChunk
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return chunk, fmt.Errorf("validate attachment scope: %w", err)
	}
	if request.ConversationID == "" || request.MessageID == "" || request.ArtifactID == "" || request.Offset < 0 || request.Length < 1 || request.Length > coordination.ChatAttachmentChunkMaxBytes {
		return chunk, fmt.Errorf("invalid attachment read request: %w", coordination.ErrWrongOwner)
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, scope); err != nil {
		return chunk, err
	}
	reference, err := rt.ChatAttachmentReference(ctx, scope, request.ConversationID, request.MessageID, request.ArtifactID)
	if err != nil {
		return chunk, err
	}
	chunk, err = rt.readChatAttachmentStorageChunk(ctx, scope, reference, request.Offset, request.Length)
	if err != nil {
		return chunk, err
	}
	if chunk.ArtifactID != request.ArtifactID || chunk.Offset != request.Offset || chunk.Size < chunk.Offset || int64(len(chunk.Data)) > request.Length || int64(len(chunk.Data)) > chunk.Size-chunk.Offset || chunk.EOF != (int64(len(chunk.Data)) == chunk.Size-chunk.Offset) || !strings.HasPrefix(chunk.Digest, "sha256:") || len(chunk.Digest) != 71 || len(chunk.Data) == 0 && !chunk.EOF {
		return coordination.ChatAttachmentChunk{}, fmt.Errorf("attachment owner returned invalid chunk: %w", coordination.ErrWrongOwner)
	}
	if chunk.Digest != "sha256:"+reference.SHA256 || chunk.Size != reference.SizeBytes {
		return coordination.ChatAttachmentChunk{}, fmt.Errorf("attachment content does not match saved message: %w", coordination.ErrWrongOwner)
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.ChatAttachmentChunk{}, err
	}
	return chunk, coordination.ValidateRoleRevision(ctx, rt, scope)
}

func (rt *Runtime) ChatAttachmentReference(ctx context.Context, scope coordination.ExecutionScope, conversationID, messageID, artifactID string) (coordination.ChatAttachmentReference, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("validate attachment scope: %w", err)
	}
	if conversationID == "" || messageID == "" || artifactID == "" {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("invalid attachment reference request: %w", coordination.ErrWrongOwner)
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, scope); err != nil {
		return coordination.ChatAttachmentReference{}, err
	}
	message, err := rt.Resource(ctx, scope, "message", messageID)
	if err != nil {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("read attachment message: %w", err)
	}
	if message == nil || message.Deleted || message.OwnerID != scope.ResourceOwnerID {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("attachment message is unavailable: %w", coordination.ErrWrongOwner)
	}
	var document struct {
		ConversationID string                                 `json:"conversationId"`
		Attachments    []coordination.ChatAttachmentReference `json:"attachments"`
	}
	if err := json.Unmarshal(message.Body, &document); err != nil {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("decode attachment message: %w", err)
	}
	if document.ConversationID != conversationID {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("attachment conversation does not match message: %w", coordination.ErrWrongOwner)
	}
	var reference coordination.ChatAttachmentReference
	found := false
	for _, item := range document.Attachments {
		if item.ArtifactID != artifactID {
			continue
		}
		if found {
			return coordination.ChatAttachmentReference{}, fmt.Errorf("duplicate attachment reference: %w", coordination.ErrWrongOwner)
		}
		reference, found = item, true
	}
	if !found {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("attachment is not referenced by message: %w", coordination.ErrWrongOwner)
	}
	if err := reference.Validate(); err != nil {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("attachment message reference is invalid: %w", err)
	}
	if reference.OwnerDeviceID != message.OwnerID {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("attachment uploader does not own message: %w", coordination.ErrWrongOwner)
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.ChatAttachmentReference{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, scope); err != nil {
		return coordination.ChatAttachmentReference{}, err
	}
	return reference, nil
}

func (rt *Runtime) validateChatAttachmentStorage(ctx context.Context, scope coordination.ExecutionScope, reference coordination.ChatAttachmentReference) error {
	var port coordination.DataPort
	if reference.StorageDeviceID == scope.CoreID {
		port = rt.CoreDataPort
	} else if reference.StorageDeviceID == rt.LocalDeviceID {
		port = rt.LocalDeviceDataPort
	}
	if port != nil {
		validator, ok := port.(coordination.ChatAttachmentPort)
		if !ok {
			return fmt.Errorf("attachment storage does not support validation")
		}
		return validator.ValidateChatAttachment(ctx, scope, reference)
	}
	var response struct {
		Valid bool `json:"valid"`
	}
	if err := rt.callChatAttachmentStorage(ctx, scope, reference.StorageDeviceID, "attachment-validate", coordination.DataQuery{AttachmentReference: &reference}, &response); err != nil {
		return fmt.Errorf("validate remote attachment: %w", err)
	}
	if !response.Valid {
		return fmt.Errorf("attachment storage did not confirm validation: %w", coordination.ErrWrongOwner)
	}
	return nil
}

func (rt *Runtime) readChatAttachmentStorageChunk(ctx context.Context, scope coordination.ExecutionScope, reference coordination.ChatAttachmentReference, offset, length int64) (coordination.ChatAttachmentChunk, error) {
	var port coordination.DataPort
	if reference.StorageDeviceID == scope.CoreID {
		port = rt.CoreDataPort
	} else if reference.StorageDeviceID == rt.LocalDeviceID {
		port = rt.LocalDeviceDataPort
	}
	if port != nil {
		reader, ok := port.(coordination.ChatAttachmentPort)
		if !ok {
			return coordination.ChatAttachmentChunk{}, fmt.Errorf("attachment storage does not support chunk reads")
		}
		return reader.ReadChatAttachmentStorageChunk(ctx, scope, reference, offset, length)
	}
	var chunk coordination.ChatAttachmentChunk
	if err := rt.callChatAttachmentStorage(ctx, scope, reference.StorageDeviceID, "attachment-storage-chunk", coordination.DataQuery{AttachmentReference: &reference, Attachment: &coordination.ChatAttachmentRead{Offset: offset, Length: length, ArtifactID: reference.ArtifactID}}, &chunk); err != nil {
		return chunk, fmt.Errorf("read remote attachment storage: %w", err)
	}
	return chunk, nil
}

func (rt *Runtime) SetChatAttachmentReference(ctx context.Context, scope coordination.ExecutionScope, reference coordination.ChatAttachmentReference, messageID string, action coordination.ChatAttachmentReferenceAction) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	if messageID == "" || len(messageID) > 512 || strings.ContainsAny(messageID, "\x00\r\n") {
		return fmt.Errorf("invalid attachment message identity: %w", coordination.ErrWrongOwner)
	}
	switch action {
	case coordination.ChatAttachmentReferencePrepare, coordination.ChatAttachmentReferenceCommit, coordination.ChatAttachmentReferenceAbort, coordination.ChatAttachmentReferenceRemove:
	default:
		return fmt.Errorf("invalid attachment reference action %q: %w", action, coordination.ErrWrongOwner)
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return err
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, scope); err != nil {
		return err
	}
	var port coordination.DataPort
	if reference.StorageDeviceID == scope.CoreID {
		port = rt.CoreDataPort
	} else if reference.StorageDeviceID == rt.LocalDeviceID {
		port = rt.LocalDeviceDataPort
	}
	if port != nil {
		writer, ok := port.(coordination.ChatAttachmentPort)
		if !ok {
			return fmt.Errorf("attachment storage does not support reference updates")
		}
		return writer.SetChatAttachmentReference(ctx, scope, reference, messageID, action)
	}
	operations := map[coordination.ChatAttachmentReferenceAction]string{
		coordination.ChatAttachmentReferencePrepare: "attachment-reference-prepare",
		coordination.ChatAttachmentReferenceCommit:  "attachment-reference-commit",
		coordination.ChatAttachmentReferenceAbort:   "attachment-reference-abort",
		coordination.ChatAttachmentReferenceRemove:  "attachment-reference-remove",
	}
	operation := operations[action]
	if err := rt.callChatAttachmentStorage(ctx, scope, reference.StorageDeviceID, operation, coordination.DataQuery{AttachmentReference: &reference, RequestID: messageID}, &struct{}{}); err != nil {
		return fmt.Errorf("update remote attachment reference: %w", err)
	}
	return rt.validateDataRoute(ctx, scope)
}

func (rt *Runtime) callChatAttachmentStorage(ctx context.Context, scope coordination.ExecutionScope, storageDeviceID, operation string, query coordination.DataQuery, result any) error {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return err
	}
	if rt.DeviceReg != nil {
		if err := rt.DeviceReg.RequireTrustedDevice(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(storageDeviceID)); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(map[string]any{"operation": operation, "scope": scope, "query": query})
	if err != nil {
		return err
	}
	reply, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(storageDeviceID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
	if err != nil {
		return err
	}
	if len(reply.Structured) > 4<<20 {
		return coordination.ErrPendingLimit
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return err
	}
	return json.Unmarshal(reply.Structured, result)
}
