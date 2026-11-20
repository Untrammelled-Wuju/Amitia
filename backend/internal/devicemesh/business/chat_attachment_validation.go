package business

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) validateChatAttachments(ctx context.Context, scope coordination.ExecutionScope, attachments []Attachment) error {
	if len(attachments) == 0 {
		return nil
	}
	if err := ValidateAttachments(attachments); err != nil {
		return err
	}
	validator, ok := e.data.(coordination.ChatAttachmentValidator)
	if !ok {
		return fmt.Errorf("attachment validation is unavailable")
	}
	for _, attachment := range attachments {
		if err := validator.ValidateChatAttachment(ctx, scope, attachment); err != nil {
			return fmt.Errorf("validate message attachment %s: %w", attachment.ArtifactID, err)
		}
	}
	return nil
}

func (e *Engine) updateChatAttachmentReferences(ctx context.Context, scope coordination.ExecutionScope, attachments []Attachment, messageID string, action coordination.ChatAttachmentReferenceAction) error {
	if len(attachments) == 0 {
		return nil
	}
	writer, ok := e.data.(coordination.ChatAttachmentReferenceWriter)
	if !ok {
		return fmt.Errorf("attachment reference updates are unavailable")
	}
	for _, attachment := range attachments {
		if err := writer.SetChatAttachmentReference(ctx, scope, attachment, messageID, action); err != nil {
			return fmt.Errorf("%s message attachment %s: %w", action, attachment.ArtifactID, err)
		}
	}
	return nil
}

func (e *Engine) prepareChatAttachmentReferences(ctx context.Context, scope coordination.ExecutionScope, attachments []Attachment, messageID string) error {
	return e.updateChatAttachmentReferences(ctx, scope, attachments, messageID, coordination.ChatAttachmentReferencePrepare)
}

func (e *Engine) commitChatAttachmentReferences(ctx context.Context, scope coordination.ExecutionScope, attachments []Attachment, messageID string) error {
	return e.updateChatAttachmentReferences(ctx, scope, attachments, messageID, coordination.ChatAttachmentReferenceCommit)
}

func (e *Engine) abortChatAttachmentReferences(ctx context.Context, scope coordination.ExecutionScope, attachments []Attachment, messageID string) error {
	return e.updateChatAttachmentReferences(ctx, scope, attachments, messageID, coordination.ChatAttachmentReferenceAbort)
}

func (e *Engine) removeChatAttachmentReferences(ctx context.Context, scope coordination.ExecutionScope, attachments []Attachment, messageID string) error {
	return e.updateChatAttachmentReferences(ctx, scope, attachments, messageID, coordination.ChatAttachmentReferenceRemove)
}
