package artifact

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
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
		artifacts.POST("/previews", h.CreatePreview)
		artifacts.DELETE("/previews/:previewId", h.DeletePreview)
		artifacts.GET("/:artifactId", h.GetMetadata)
		artifacts.GET("/:artifactId/content", h.GetContent)
		artifacts.GET("/:artifactId/media-ticket", h.GetMediaTicket)
		artifacts.DELETE("/:artifactId", h.Delete)
	}
}

func (h *Handler) RegisterPublicMedia(r *gin.Engine) {
	r.GET("/media/artifacts/:artifactId/:ticket", h.GetMediaContent)
	r.GET("/media/artifact-previews/:previewId/:ticket", h.GetPreview)
}

func (h *Handler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "artifact.invalid_upload", "message": "missing file"})
		return
	}
	defer file.Close()

	kindVal := c.PostForm("kind")
	sourceVal := c.PostForm("source")
	if sourceVal == "" {
		sourceVal = string(SourceUserUpload)
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(401, gin.H{"error": "artifact.unauthorized", "message": "authentication required"})
		return
	}
	owner := string(actor.SpaceID)
	req := CreateRequest{
		OwnerSpaceID: owner,
		Kind:         Kind(kindVal),
		Filename:     header.Filename,
		Source:       Source(sourceVal),
		Reader:       file,
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
		c.JSON(401, gin.H{"error": "artifact.unauthorized"})
		return
	}
	owner := string(actor.SpaceID)
	art, err := h.svc.GetOwned(c.Request.Context(), owner, id)
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
		c.JSON(401, gin.H{"error": "artifact.unauthorized"})
		return
	}
	owner := string(actor.SpaceID)
	art, err := h.svc.GetOwned(c.Request.Context(), owner, id)
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
		c.JSON(401, gin.H{"error": "artifact.unauthorized"})
		return
	}
	owner := string(actor.SpaceID)
	art, err := h.svc.GetOwned(c.Request.Context(), owner, id)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
	ticket, expiresAt, err := h.ticketSigner.Issue(art.ID, art.OwnerSpaceID)
	if err != nil {
		c.JSON(500, gin.H{"error": "artifact.media_ticket_failed", "message": err.Error()})
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
		c.JSON(401, gin.H{"error": "artifact.unauthorized", "message": err.Error()})
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
		c.JSON(401, gin.H{"error": "artifact.unauthorized"})
		return
	}
	owner := string(actor.SpaceID)
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
		switch e.Code {
		case "invalid_upload":
			c.JSON(400, gin.H{"error": "artifact.invalid_upload", "message": e.Msg})
		case "too_large":
			c.JSON(413, gin.H{"error": "artifact.too_large", "message": e.Msg})
		case "unsupported_mime":
			c.JSON(400, gin.H{"error": "artifact.unsupported_mime", "message": e.Msg})
		case "not_found", "not_owned":
			c.JSON(404, gin.H{"error": "artifact.not_found", "message": e.Msg})
		case "deleted":
			c.JSON(404, gin.H{"error": "artifact.deleted", "message": e.Msg})
		case "in_use":
			c.JSON(409, gin.H{"error": "artifact.in_use", "message": e.Msg})
		case "invalid_reference":
			c.JSON(400, gin.H{"error": "artifact.invalid_reference", "message": e.Msg})
		default:
			c.JSON(500, gin.H{"error": "artifact.server_error", "message": e.Msg})
		}
	default:
		c.JSON(500, gin.H{"error": "artifact.server_error", "message": err.Error()})
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
