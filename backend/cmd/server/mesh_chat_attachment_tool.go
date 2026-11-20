package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

const chatAttachmentReadTool = "chat_attachment_read"
const chatAttachmentExtractTool = "chat_attachment_extract"
const chatAttachmentToolChunkBytes int64 = 16 << 10

func chatAttachmentToolDefinition() tool.Tool {
	return tool.Tool{Type: "function", Function: tool.Function{
		Name:        chatAttachmentReadTool,
		Description: "Read a bounded byte range from an attachment referenced by a saved message in this conversation. Content is returned as UTF-8 text when valid, otherwise base64. Use the returned offset and eof to read subsequent ranges; verify file type and choose available authorized tools for binary formats.",
		Parameters: tool.Parameters{Type: "object", Properties: map[string]tool.Property{
			"messageId":  {Type: "string", Description: "ID of the saved conversation message containing the attachment"},
			"artifactId": {Type: "string", Description: "Attachment artifact ID from that message"},
			"offset":     {Type: "integer", Description: "Byte offset, starting at zero"},
			"length":     {Type: "integer", Description: "Maximum bytes to read, between 1 and 16384"},
		}, Required: []string{"messageId", "artifactId", "offset", "length"}},
	}}
}

func chatAttachmentExtractToolDefinition() tool.Tool {
	return tool.Tool{Type: "function", Function: tool.Function{
		Name:        chatAttachmentExtractTool,
		Description: "Process a document, image, or text attachment referenced by a saved message. Streams the attachment from its authorized storage owner, extracts DOCX/PDF/text content, or uses the configured vision model for images and scanned PDF pages. Returns unsupported or tool failures as data so the task can continue with another authorized capability.",
		Parameters: tool.Parameters{Type: "object", Properties: map[string]tool.Property{
			"messageId":  {Type: "string", Description: "ID of the saved conversation message containing the attachment"},
			"artifactId": {Type: "string", Description: "Attachment artifact ID from that message"},
			"maxChars":   {Type: "integer", Description: "Maximum extracted characters to return, between 1000 and 64000"},
			"maxPages":   {Type: "integer", Description: "Maximum scanned PDF pages to analyze, between 1 and 16"},
		}, Required: []string{"messageId", "artifactId"}},
	}}
}

func (r *meshOwnedToolRuntime) readChatAttachment(ctx context.Context, inference business.Inference, input json.RawMessage) (chat.ToolResult, error) {
	if r == nil || r.services == nil || r.services.DeviceMesh == nil || inference.ConversationID == "" {
		return chat.ToolResult{}, fmt.Errorf("chat attachment runtime unavailable")
	}
	var payload struct {
		MessageID  string `json:"messageId"`
		ArtifactID string `json:"artifactId"`
		Offset     *int64 `json:"offset"`
		Length     *int64 `json:"length"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return chat.ToolResult{}, fmt.Errorf("decode chat attachment request: %w", err)
	}
	if payload.MessageID == "" || payload.ArtifactID == "" || payload.Offset == nil || payload.Length == nil || *payload.Offset < 0 || *payload.Length < 1 || *payload.Length > chatAttachmentToolChunkBytes {
		return chat.ToolResult{}, fmt.Errorf("invalid chat attachment request: %w", coordination.ErrWrongOwner)
	}
	chunk, err := r.services.DeviceMesh.ReadChatAttachmentChunk(ctx, inference.Scope, coordination.ChatAttachmentRead{ConversationID: inference.ConversationID, MessageID: payload.MessageID, ArtifactID: payload.ArtifactID, Offset: *payload.Offset, Length: *payload.Length})
	if err != nil {
		return chat.ToolResult{}, fmt.Errorf("read chat attachment: %w", err)
	}
	content := base64.StdEncoding.EncodeToString(chunk.Data)
	encoding := "base64"
	if utf8.Valid(chunk.Data) && bytes.IndexByte(chunk.Data, 0) < 0 {
		content = string(chunk.Data)
		encoding = "utf-8"
	}
	encoded, err := json.Marshal(map[string]any{"artifactId": chunk.ArtifactID, "digest": chunk.Digest, "offset": chunk.Offset, "size": chunk.Size, "nextOffset": chunk.Offset + int64(len(chunk.Data)), "eof": chunk.EOF, "encoding": encoding, "content": content})
	if err != nil {
		return chat.ToolResult{}, fmt.Errorf("encode chat attachment result: %w", err)
	}
	return chat.ToolResult{Status: "SUCCESS", Output: encoded}, nil
}

func (r *meshOwnedToolRuntime) extractChatAttachment(ctx context.Context, inference business.Inference, input json.RawMessage) (chat.ToolResult, error) {
	if r == nil || r.services == nil || r.services.DeviceMesh == nil || inference.ConversationID == "" {
		return chat.ToolResult{}, fmt.Errorf("chat attachment runtime unavailable")
	}
	var payload struct {
		MessageID  string `json:"messageId"`
		ArtifactID string `json:"artifactId"`
		MaxChars   *int64 `json:"maxChars"`
		MaxPages   *int   `json:"maxPages"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return chat.ToolResult{}, fmt.Errorf("decode chat attachment extraction request: %w", err)
	}
	maxChars := int64(32000)
	if payload.MaxChars != nil {
		maxChars = *payload.MaxChars
	}
	maxPages := 8
	if payload.MaxPages != nil {
		maxPages = *payload.MaxPages
	}
	if payload.MessageID == "" || payload.ArtifactID == "" || maxChars < 1000 || maxChars > 64000 || maxPages < 1 || maxPages > 16 {
		return chat.ToolResult{}, fmt.Errorf("invalid chat attachment extraction request: %w", coordination.ErrWrongOwner)
	}
	reference, err := r.services.DeviceMesh.ChatAttachmentReference(ctx, inference.Scope, inference.ConversationID, payload.MessageID, payload.ArtifactID)
	if err != nil {
		return chat.ToolResult{}, fmt.Errorf("resolve chat attachment: %w", err)
	}
	dir, err := os.MkdirTemp("", "amitia-chat-attachment-")
	if err != nil {
		return chat.ToolResult{}, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, safeAttachmentTempName(reference))
	if err := r.materializeChatAttachment(ctx, inference, payload.MessageID, reference, path); err != nil {
		return chat.ToolResult{}, err
	}
	if reference.Kind == "image" || strings.HasPrefix(strings.ToLower(reference.MIMEType), "image/") {
		text, visionErr := chat.AnalyzeAttachmentImage(ctx, path, reference.MIMEType)
		return attachmentProcessingResult(reference, "vision", text, visionErr, nil, maxChars), nil
	}
	text, extractErr := chat.ExtractAttachmentText(ctx, path, strings.ToLower(reference.MIMEType))
	if extractErr == nil {
		return attachmentProcessingResult(reference, "text", text, nil, nil, maxChars), nil
	}
	if strings.EqualFold(reference.MIMEType, "application/pdf") {
		pagePaths, renderErr := chat.ExtractPDFImages(ctx, path, dir, maxPages)
		if renderErr != nil {
			externalPaths, externalErr := chat.RenderPDFPages(ctx, path, dir, maxPages)
			renderErr = errors.Join(renderErr, externalErr)
			if externalErr == nil {
				pagePaths = externalPaths
				renderErr = nil
			}
		}
		if renderErr == nil {
			pageTexts := make([]string, 0, len(pagePaths))
			visionErrors := make([]string, 0)
			for index, pagePath := range pagePaths {
				pageMIME := mime.TypeByExtension(filepath.Ext(pagePath))
				if pageMIME == "" {
					pageMIME = "application/octet-stream"
				}
				text, visionErr := chat.AnalyzeAttachmentImage(ctx, pagePath, pageMIME)
				if visionErr != nil {
					visionErrors = append(visionErrors, fmt.Sprintf("page %d: %v", index+1, visionErr))
					continue
				}
				pageTexts = append(pageTexts, fmt.Sprintf("第 %d 页：\n%s", index+1, text))
			}
			if len(pageTexts) > 0 {
				details := map[string]any{"pdfTextError": extractErr.Error(), "renderedPages": len(pagePaths)}
				if len(visionErrors) > 0 {
					details["visionErrors"] = visionErrors
				}
				return attachmentProcessingResult(reference, "vision", strings.Join(pageTexts, "\n\n"), nil, details, maxChars), nil
			}
			renderErr = errors.Join(renderErr, errors.New(strings.Join(visionErrors, "; ")))
		}
		details := map[string]any{"pdfTextError": extractErr.Error(), "pdfRenderError": renderErr.Error(), "requiresVision": true}
		return attachmentProcessingResult(reference, "unavailable", "", errors.Join(extractErr, renderErr), details, maxChars), nil
	}
	return attachmentProcessingResult(reference, "unsupported", "", extractErr, map[string]any{"requiresGenericTool": true}, maxChars), nil
}

func (r *meshOwnedToolRuntime) materializeChatAttachment(ctx context.Context, inference business.Inference, messageID string, reference coordination.ChatAttachmentReference, path string) (returnErr error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()
	hash := sha256.New()
	var offset int64
	for {
		chunk, err := r.services.DeviceMesh.ReadChatAttachmentChunk(ctx, inference.Scope, coordination.ChatAttachmentRead{
			ConversationID: inference.ConversationID,
			MessageID:      messageID,
			ArtifactID:     reference.ArtifactID,
			Offset:         offset,
			Length:         coordination.ChatAttachmentChunkMaxBytes,
		})
		if err != nil {
			return fmt.Errorf("read chat attachment at offset %d: %w", offset, err)
		}
		if _, err := file.Write(chunk.Data); err != nil {
			return fmt.Errorf("write chat attachment temp file: %w", err)
		}
		if _, err := hash.Write(chunk.Data); err != nil {
			return err
		}
		offset += int64(len(chunk.Data))
		if chunk.EOF {
			break
		}
	}
	if offset != reference.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != reference.SHA256 {
		return fmt.Errorf("chat attachment integrity mismatch: expected size=%d sha256=%s, got size=%d sha256=%s", reference.SizeBytes, reference.SHA256, offset, hex.EncodeToString(hash.Sum(nil)))
	}
	return nil
}

func attachmentProcessingResult(reference coordination.ChatAttachmentReference, source, text string, processingErr error, details map[string]any, maxChars int64) chat.ToolResult {
	truncated := false
	if int64(len(text)) > maxChars {
		text = text[:maxChars]
		truncated = true
	}
	document := map[string]any{
		"artifactId":      reference.ArtifactID,
		"ownerDeviceId":   reference.OwnerDeviceID,
		"storageDeviceId": reference.StorageDeviceID,
		"name":            reference.Name,
		"mimeType":        reference.MIMEType,
		"sizeBytes":       reference.SizeBytes,
		"sha256":          reference.SHA256,
		"source":          source,
		"truncated":       truncated,
	}
	if details != nil {
		document["details"] = details
	}
	if text != "" {
		document["content"] = text
	}
	if processingErr != nil {
		document["error"] = processingErr.Error()
		document["errorChain"] = fmt.Sprintf("%+v", processingErr)
	}
	encoded, _ := json.Marshal(document)
	if processingErr != nil {
		return chat.ToolResult{Status: "FAILED", Output: encoded, VisibleText: "附件处理未完成：" + processingErr.Error(), Error: &chat.ToolError{Code: "ATTACHMENT_PROCESSING_FAILED", Message: processingErr.Error()}}
	}
	return chat.ToolResult{Status: "SUCCESS", Output: encoded, VisibleText: "已处理附件 " + reference.Name}
}

func safeAttachmentTempName(reference coordination.ChatAttachmentReference) string {
	extension := filepath.Ext(reference.Name)
	if len(extension) > 16 || strings.ContainsAny(extension, "/\\\x00\r\n") {
		extension = ""
	}
	return "attachment" + extension
}
