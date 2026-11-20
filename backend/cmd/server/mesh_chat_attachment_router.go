package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func chatAttachmentReference(item artifact.Artifact) coordination.ChatAttachmentReference {
	return coordination.ChatAttachmentReference{
		ArtifactID:      string(item.ID),
		OwnerDeviceID:   item.OwnerDeviceID,
		StorageDeviceID: item.StorageDeviceID,
		Kind:            string(item.Kind),
		Name:            item.Filename,
		MIMEType:        item.MIMEType,
		SHA256:          strings.TrimPrefix(string(item.BlobDigest), "sha256:"),
		SizeBytes:       item.SizeBytes,
	}
}

func registerMeshChatAttachmentRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	mesh.GET("/attachments/:artifactId/content", func(c *gin.Context) {
		target, err := chatAttachmentTargetFromActor(c, coreID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "mesh.invalid_attachment_read", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
			return
		}
		serveSavedChatAttachment(c, services, coreID, target)
	})
	mesh.GET("/attachments/:artifactId/ticket", func(c *gin.Context) {
		target, err := chatAttachmentTargetFromActor(c, coreID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "mesh.invalid_attachment_ticket", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
			return
		}
		if services == nil || services.ChatAttachmentTickets == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.attachment_ticket_unavailable", "message": "chat attachment media ticket signer unavailable"})
			return
		}
		if _, _, err := resolveSavedChatAttachment(c.Request.Context(), services, coreID, target); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": "mesh.attachment_unavailable", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
			return
		}
		ticket, expiresAt, err := services.ChatAttachmentTickets.Issue(chatAttachmentMediaTicketPayload{
			ArtifactID:          target.ArtifactID,
			SpaceID:             target.SpaceID,
			DeviceID:            target.DeviceID,
			CoreID:              coreID,
			ConversationID:      target.ConversationID,
			MessageID:           target.MessageID,
			CharacterID:         target.CharacterID,
			ConversationOwnerID: target.ConversationOwnerID,
			HistoricalRoleID:    target.HistoricalRoleID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "mesh.attachment_ticket_failed", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{
			"url":       fmt.Sprintf("/media/chat-attachments/%s/%s", target.ArtifactID, ticket),
			"expiresAt": expiresAt.Format(time.RFC3339),
		})
	})
}

func serveSavedChatAttachment(c *gin.Context, services *AppServices, coreID string, target chatAttachmentReadTarget) {
	if services == nil || services.DeviceMesh == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.attachment_unavailable", "message": "device mesh runtime unavailable"})
		return
	}
	view, reference, err := resolveSavedChatAttachment(c.Request.Context(), services, coreID, target)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "mesh.attachment_unavailable", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
		return
	}
	reader := &remoteChatAttachmentReader{
		ctx:            c.Request.Context(),
		runtime:        services.DeviceMesh,
		scope:          view.Scope,
		conversationID: target.ConversationID,
		messageID:      target.MessageID,
		reference:      reference,
	}
	if _, err := reader.Read(make([]byte, 1)); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.attachment_read_failed", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
		return
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.attachment_seek_failed", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
		return
	}
	disposition := "inline"
	if c.Query("download") == "1" || reference.Kind == "file" {
		disposition = "attachment"
	}
	c.Header("Content-Type", reference.MIMEType)
	c.Header("Content-Length", fmt.Sprintf("%d", reference.SizeBytes))
	c.Header("ETag", fmt.Sprintf(`"sha256:%s"`, reference.SHA256))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", attachmentContentDisposition(disposition, reference.Name))
	http.ServeContent(c.Writer, c.Request, reference.Name, time.Time{}, reader)
}

func savedChatAttachmentReference(view business.QueryResult, conversationID, messageID, artifactID string) (coordination.ChatAttachmentReference, error) {
	var resource *coordination.Resource
	for _, snapshot := range []coordination.DataSnapshot{view.Snapshot, dereferenceSnapshot(view.HistoricalSnapshot)} {
		for index := range snapshot.Resources {
			row := &snapshot.Resources[index]
			if row.Kind == "message" && row.ID == messageID && !row.Deleted {
				resource = row
				break
			}
		}
		if resource != nil {
			break
		}
	}
	if resource == nil {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("附件消息不存在: %w", coordination.ErrWrongOwner)
	}
	var document struct {
		ConversationID string                                 `json:"conversationId"`
		Attachments    []coordination.ChatAttachmentReference `json:"attachments"`
	}
	if err := json.Unmarshal(resource.Body, &document); err != nil {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("解析附件消息失败: %w", err)
	}
	if document.ConversationID != conversationID {
		return coordination.ChatAttachmentReference{}, fmt.Errorf("附件消息不属于当前会话: %w", coordination.ErrWrongOwner)
	}
	for _, reference := range document.Attachments {
		if reference.ArtifactID != artifactID {
			continue
		}
		if err := reference.Validate(); err != nil {
			return coordination.ChatAttachmentReference{}, err
		}
		if reference.OwnerDeviceID != resource.OwnerID {
			return coordination.ChatAttachmentReference{}, fmt.Errorf("附件上传设备与消息归属不一致: %w", coordination.ErrWrongOwner)
		}
		return reference, nil
	}
	return coordination.ChatAttachmentReference{}, fmt.Errorf("消息未引用该附件: %w", coordination.ErrWrongOwner)
}

func dereferenceSnapshot(snapshot *coordination.DataSnapshot) coordination.DataSnapshot {
	if snapshot == nil {
		return coordination.DataSnapshot{}
	}
	return *snapshot
}

func attachmentContentDisposition(disposition, filename string) string {
	if filename == "" {
		filename = "attachment"
	}
	quoted := strings.ReplaceAll(strings.ReplaceAll(filename, `\`, `\\`), `"`, `\"`)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, quoted, url.PathEscape(filename))
}

type remoteChatAttachmentReader struct {
	ctx            context.Context
	runtime        coordination.ChatAttachmentReader
	scope          coordination.ExecutionScope
	conversationID string
	messageID      string
	reference      coordination.ChatAttachmentReference
	position       int64
	buffer         []byte
	bufferOffset   int64
}

func (r *remoteChatAttachmentReader) Read(p []byte) (int, error) {
	if r.position >= r.reference.SizeBytes {
		return 0, io.EOF
	}
	if len(r.buffer) == 0 || r.position < r.bufferOffset || r.position >= r.bufferOffset+int64(len(r.buffer)) {
		chunk, err := r.runtime.ReadChatAttachmentChunk(r.ctx, r.scope, coordination.ChatAttachmentRead{
			ConversationID: r.conversationID,
			MessageID:      r.messageID,
			ArtifactID:     r.reference.ArtifactID,
			Offset:         r.position,
			Length:         coordination.ChatAttachmentChunkMaxBytes,
		})
		if err != nil {
			return 0, err
		}
		r.buffer = chunk.Data
		r.bufferOffset = chunk.Offset
	}
	start := r.position - r.bufferOffset
	if start < 0 || start >= int64(len(r.buffer)) {
		return 0, fmt.Errorf("附件流位置无效: position=%d bufferOffset=%d bufferSize=%d", r.position, r.bufferOffset, len(r.buffer))
	}
	n := copy(p, r.buffer[start:])
	r.position += int64(n)
	return n, nil
}

func (r *remoteChatAttachmentReader) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = r.position + offset
	case io.SeekEnd:
		next = r.reference.SizeBytes + offset
	default:
		return 0, errors.New("无效的附件定位方式")
	}
	if next < 0 {
		return 0, errors.New("附件定位不能为负数")
	}
	r.position = next
	return next, nil
}
