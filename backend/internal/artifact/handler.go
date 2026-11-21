package artifact

import (
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	svc          *Service
	ticketSigner *mediaTicketSigner
	previewMu    sync.Mutex
	previews     map[string]previewDocument
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, ticketSigner: newMediaTicketSigner(), previews: make(map[string]previewDocument)}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	artifacts := r.Group("/artifacts/v1")
	{
		artifacts.POST("", h.Upload)
		artifacts.POST("/chat-uploads", h.UploadChatStream)
		artifacts.POST("/previews", h.CreatePreview)
		artifacts.DELETE("/previews/:previewId", h.DeletePreview)
		artifacts.GET("/:artifactId", h.GetMetadata)
		artifacts.GET("/:artifactId/content", h.GetContent)
		artifacts.GET("/:artifactId/media-ticket", h.GetMediaTicket)
		artifacts.DELETE("/:artifactId", h.Delete)
	}
}

func (h *Handler) UploadChatStream(c *gin.Context) {
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "message": err.Error()})
		return
	}
	if actor == nil || actor.SpaceID == "" {
		err := fmt.Errorf("artifact: authenticated space is missing")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "message": err.Error()})
		return
	}
	filename := SanitizeFilename(c.Query("filename"))
	if filename == "" || filename == "." {
		err := ErrInvalidUpload("filename is required")
		handleArtifactError(c, err)
		return
	}
	kind := Kind(strings.TrimSpace(c.Query("kind")))
	if kind != "" && kind != KindImage && kind != KindAudio && kind != KindVideo && kind != KindFile {
		err := ErrInvalidUpload("invalid attachment kind")
		handleArtifactError(c, err)
		return
	}
	art, err := h.svc.Create(c.Request.Context(), CreateRequest{
		OwnerSpaceID:  string(actor.SpaceID),
		OwnerDeviceID: string(actor.DeviceID),
		Kind:          kind,
		Filename:      filename,
		MIMEType:      chatUploadMIMEType(c.GetHeader("Content-Type"), filename),
		Source:        SourceChatUpload,
		Reader:        c.Request.Body,
		Unbounded:     true,
	})
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"artifact": art})
}

func chatUploadMIMEType(declared, filename string) string {
	declared = normalizeMIME(strings.ToLower(strings.TrimSpace(declared)))
	if declared != "" && declared != "application/octet-stream" && declared != "binary/octet-stream" && declared != "application/unknown" && declared != "*/*" {
		return declared
	}
	byExtension := normalizeMIME(strings.ToLower(strings.TrimSpace(mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))))))
	if byExtension == "" || strings.HasPrefix(byExtension, "application/x-") {
		return ""
	}
	return byExtension
}

func (h *Handler) RegisterPublicMedia(r *gin.Engine) {
	r.GET("/media/artifacts/:artifactId/:ticket", h.GetMediaContent)
	r.GET("/media/artifact-previews/:previewId/:ticket", h.GetPreview)
}

func (h *Handler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error(), "message": err.Error()})
		return
	}
	defer file.Close()

	kindVal := c.PostForm("kind")
	sourceVal := c.PostForm("source")
	if sourceVal == "" {
		sourceVal = string(SourceUserUpload)
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error(), "message": err.Error()})
		return
	}
	if actor == nil {
		c.JSON(401, gin.H{"error": "middleware.GetActorFromContext returned nil actor", "message": "middleware.GetActorFromContext returned nil actor"})
		return
	}
	owner := string(actor.SpaceID)
	req := CreateRequest{
		OwnerSpaceID:  owner,
		OwnerDeviceID: string(actor.DeviceID),
		Kind:          Kind(kindVal),
		Filename:      header.Filename,
		Source:        Source(sourceVal),
		Reader:        file,
	}
	if header.Size > 0 {
		req.MaxBytes = 0
	}
	art, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	c.JSON(200, gin.H{
		"artifact": art,
	})
}

func (h *Handler) GetMetadata(c *gin.Context) {
	id := ID(c.Param("artifactId"))
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(401, gin.H{"error": util.OriginalErrorMessage(err, "authentication context unavailable")})
		return
	}
	owner := string(actor.SpaceID)
	art, err := h.svc.GetOwnedForDevice(c.Request.Context(), owner, string(actor.DeviceID), id)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	c.JSON(200, gin.H{"artifact": art})
}

func (h *Handler) GetContent(c *gin.Context) {
	id := ID(c.Param("artifactId"))
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(401, gin.H{"error": util.OriginalErrorMessage(err, "authentication context unavailable")})
		return
	}
	owner := string(actor.SpaceID)
	art, err := h.svc.GetOwnedForDevice(c.Request.Context(), owner, string(actor.DeviceID), id)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	h.serveContent(c, art)
}

func (h *Handler) GetMediaTicket(c *gin.Context) {
	id := ID(c.Param("artifactId"))
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(401, gin.H{"error": util.OriginalErrorMessage(err, "authentication context unavailable")})
		return
	}
	owner := string(actor.SpaceID)
	art, err := h.svc.GetOwnedForDevice(c.Request.Context(), owner, string(actor.DeviceID), id)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	ticket, expiresAt, err := h.ticketSigner.Issue(art.ID, art.OwnerSpaceID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error(), "message": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{
		"url":       fmt.Sprintf("/media/artifacts/%s/%s", art.ID, ticket),
		"expiresAt": expiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) GetMediaContent(c *gin.Context) {
	id := ID(c.Param("artifactId"))
	owner, err := h.ticketSigner.Validate(c.Param("ticket"), id)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error(), "message": err.Error()})
		return
	}
	art, err := h.svc.GetOwned(c.Request.Context(), owner, id)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	h.serveContent(c, art)
}

func (h *Handler) serveContent(c *gin.Context, art Artifact) {
	rc, info, err := h.svc.OpenBlob(c.Request.Context(), art.BlobDigest)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	defer rc.Close()
	filename := art.Filename
	if filename == "" {
		filename = string(art.ID) + art.Extension
	}
	disposition := "inline"
	if art.Kind == KindFile || art.Kind == KindArchive || isDownloadRequest(c) {
		disposition = "attachment"
	}
	c.Header("Content-Type", art.MIMEType)
	c.Header("Content-Length", fmt.Sprintf("%d", info.SizeBytes))
	c.Header("ETag", fmt.Sprintf(`"%s"`, art.BlobDigest))
	if value := contentDisposition(disposition, filename); value != "" {
		c.Header("Content-Disposition", value)
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, filename, art.UpdatedAt, rc)
	c.Status(200)
}

func (h *Handler) Delete(c *gin.Context) {
	id := ID(c.Param("artifactId"))
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(401, gin.H{"error": util.OriginalErrorMessage(err, "authentication context unavailable")})
		return
	}
	owner := string(actor.SpaceID)
	if _, err := h.svc.GetOwnedForDevice(c.Request.Context(), owner, string(actor.DeviceID), id); err != nil {
		handleArtifactError(c, err)
		return
	}
	err = h.svc.Delete(c.Request.Context(), owner, id)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func handleArtifactError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *ArtifactError:
		payload := gin.H{"error": "artifact." + e.Code, "code": "artifact." + e.Code, "message": e.Msg, "rawError": fmt.Sprintf("%+v", err)}
		if e.Cause != nil {
			payload["cause"] = e.Cause.Error()
			payload["detail"] = fmt.Sprintf("%+v", e.Cause)
		}
		switch e.Code {
		case "invalid_upload":
			c.JSON(400, payload)
		case "too_large":
			c.JSON(413, payload)
		case "unsupported_mime":
			c.JSON(400, payload)
		case "not_found", "not_owned":
			c.JSON(404, payload)
		case "deleted":
			c.JSON(404, payload)
		case "in_use":
			c.JSON(409, payload)
		case "invalid_reference":
			c.JSON(400, payload)
		default:
			c.JSON(500, payload)
		}
	default:
		c.JSON(500, gin.H{"error": "artifact.server_error", "code": "artifact.server_error", "message": err.Error(), "rawError": fmt.Sprintf("%+v", err)})
	}
}

func sanitizeDispositionFilename(name string) string {
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, `"`, "")
	return name
}

func contentDisposition(disposition, filename string) string {
	filename = sanitizeDispositionFilename(filename)
	if filename == "" {
		return disposition
	}
	value := mime.FormatMediaType(disposition, map[string]string{"filename": filename})
	if value != "" {
		return value
	}
	return fmt.Sprintf(`%s; filename="%s"`, disposition, filename)
}

func isDownloadRequest(c *gin.Context) bool {
	value := strings.TrimSpace(strings.ToLower(c.Query("download")))
	return value == "1" || value == "true" || value == "yes"
}
