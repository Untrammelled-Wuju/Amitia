package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (p *meshLocalDataPort) ValidateChatAttachment(ctx context.Context, scope coordination.ExecutionScope, reference coordination.ChatAttachmentReference) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	if reference.StorageDeviceID != p.ownerID {
		return fmt.Errorf("attachment storage does not match owner: %w", coordination.ErrWrongOwner)
	}
	if p.services == nil || p.services.Artifact == nil || p.services.Artifact.Service == nil {
		return fmt.Errorf("attachment artifact service unavailable")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if !(scope.RoleOwnerID == scope.CoreID && scope.CoreID != p.ownerID) {
		if err := coordination.ValidateRoleRevision(ctx, p, scope); err != nil {
			return err
		}
	}
	stored, err := p.services.Artifact.Service.GetChatUploadReference(ctx, p.spaceID, reference.OwnerDeviceID, reference.StorageDeviceID, artifact.ID(reference.ArtifactID))
	if err != nil {
		return fmt.Errorf("validate uploaded attachment: %w", err)
	}
	if stored.Source != artifact.SourceChatUpload || stored.OwnerDeviceID != reference.OwnerDeviceID || stored.StorageDeviceID != reference.StorageDeviceID || string(stored.BlobDigest) != "sha256:"+reference.SHA256 || stored.SizeBytes != reference.SizeBytes || string(stored.Kind) != reference.Kind || stored.Filename != reference.Name || stored.MIMEType != reference.MIMEType {
		return fmt.Errorf("uploaded attachment metadata mismatch: %w", coordination.ErrWrongOwner)
	}
	return coordination.ValidateCurrent(ctx)
}

func (p *meshLocalDataPort) ReadChatAttachmentStorageChunk(ctx context.Context, scope coordination.ExecutionScope, reference coordination.ChatAttachmentReference, offset, length int64) (coordination.ChatAttachmentChunk, error) {
	var result coordination.ChatAttachmentChunk
	if err := reference.Validate(); err != nil {
		return result, err
	}
	if reference.StorageDeviceID != p.ownerID || offset < 0 || length < 1 || length > coordination.ChatAttachmentChunkMaxBytes {
		return result, fmt.Errorf("attachment storage request outside owner scope: %w", coordination.ErrWrongOwner)
	}
	if p.services == nil || p.services.Artifact == nil || p.services.Artifact.Service == nil {
		return result, fmt.Errorf("attachment artifact service unavailable")
	}
	if !(scope.RoleOwnerID == scope.CoreID && scope.CoreID != p.ownerID) {
		if err := coordination.ValidateRoleRevision(ctx, p, scope); err != nil {
			return result, err
		}
	}
	chunk, err := p.services.Artifact.Service.ReadChatUploadChunk(ctx, p.spaceID, reference.OwnerDeviceID, reference.StorageDeviceID, artifact.ID(reference.ArtifactID), offset, length)
	if err != nil {
		return result, fmt.Errorf("read attachment content: %w", err)
	}
	if string(chunk.Digest) != "sha256:"+strings.ToLower(reference.SHA256) || chunk.Size != reference.SizeBytes {
		return result, fmt.Errorf("attachment content does not match saved reference: %w", coordination.ErrWrongOwner)
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return result, err
	}
	return coordination.ChatAttachmentChunk{ArtifactID: string(chunk.ArtifactID), Digest: string(chunk.Digest), Offset: chunk.Offset, Size: chunk.Size, Data: chunk.Data, EOF: chunk.EOF}, nil
}

func (p *meshLocalDataPort) SetChatAttachmentReference(ctx context.Context, scope coordination.ExecutionScope, reference coordination.ChatAttachmentReference, messageID string, action coordination.ChatAttachmentReferenceAction) error {
	if err := p.ValidateChatAttachment(ctx, scope, reference); err != nil {
		return err
	}
	binding := coordination.ChatAttachmentReferenceBinding{Scope: scope, Reference: reference, MessageID: messageID}
	token, err := binding.Token()
	if err != nil {
		return err
	}
	switch action {
	case coordination.ChatAttachmentReferencePrepare:
		if err := p.services.Artifact.Service.PrepareChatAttachmentReference(ctx, artifact.ID(reference.ArtifactID), token, messageID); err != nil {
			return fmt.Errorf("prepare chat attachment reference: %w", err)
		}
	case coordination.ChatAttachmentReferenceCommit:
		if err := p.services.Artifact.Service.CommitChatAttachmentReference(ctx, artifact.ID(reference.ArtifactID), token, messageID); err != nil {
			return fmt.Errorf("commit chat attachment reference: %w", err)
		}
	case coordination.ChatAttachmentReferenceAbort:
		if err := p.services.Artifact.Service.AbortChatAttachmentReference(ctx, artifact.ID(reference.ArtifactID), token); err != nil {
			return fmt.Errorf("abort chat attachment reference: %w", err)
		}
	case coordination.ChatAttachmentReferenceRemove:
		if err := p.services.Artifact.Service.RemoveChatAttachmentReference(ctx, artifact.ID(reference.ArtifactID), token, messageID); err != nil {
			return fmt.Errorf("remove chat attachment reference: %w", err)
		}
	default:
		return fmt.Errorf("invalid chat attachment reference action %q: %w", action, coordination.ErrWrongOwner)
	}
	return coordination.ValidateCurrent(ctx)
}
