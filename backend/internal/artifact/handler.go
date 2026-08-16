package artifact

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	artifacts := r.Group("/api/artifacts/v1")
	{
		artifacts.POST("", h.Upload)
		artifacts.GET("/:artifactId", h.GetMetadata)
		artifacts.GET("/:artifactId/content", h.GetContent)
		artifacts.DELETE("/:artifactId", h.Delete)
	}
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
	owner := string(actor.UserID)
	req := CreateRequest{
		OwnerUserID: owner,
		Kind:        Kind(kindVal),
		Filename:    header.Filename,
		Source:      Source(sourceVal),
		Reader:      file,
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
	owner := string(actor.UserID)
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
	owner := string(actor.UserID)
	art, err := h.svc.GetOwned(c.Request.Context(), owner, id)
	if err != nil {
		handleArtifactError(c, err)
		return
	}
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
	if art.Kind == KindFile || art.Kind == KindArchive {
		disposition = "attachment"
	}
	c.Header("Content-Type", art.MIMEType)
	c.Header("Content-Length", fmt.Sprintf("%d", info.SizeBytes))
	c.Header("ETag", fmt.Sprintf(`"%s"`, art.BlobDigest))
	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, sanitizeDispositionFilename(filename)))
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, filename, art.UpdatedAt, newReadSeeker(rc, info.SizeBytes))
	c.Status(200)
}

func (h *Handler) Delete(c *gin.Context) {
	id := ID(c.Param("artifactId"))
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(401, gin.H{"error": "artifact.unauthorized"})
		return
	}
	owner := string(actor.UserID)
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

func newReadSeeker(rc io.Reader, size int64) io.ReadSeeker {
	if rs, ok := rc.(io.ReadSeeker); ok {
		return rs
	}
	return &readSeekerAdapter{rc: rc, size: size}
}

type readSeekerAdapter struct {
	rc   io.Reader
	pos  int64
	size int64
}

func (a *readSeekerAdapter) Read(p []byte) (int, error) {
	n, err := a.rc.Read(p)
	a.pos += int64(n)
	return n, err
}

func (a *readSeekerAdapter) Seek(offset int64, whence int) (int64, error) {
	if rs, ok := a.rc.(io.ReadSeeker); ok {
		return rs.Seek(offset, whence)
	}
	if whence == io.SeekStart && offset == 0 && a.pos == 0 {
		return 0, nil
	}
	if whence == io.SeekEnd {
		return a.size, nil
	}
	return a.pos, fmt.Errorf("artifact: seek not fully supported")
}

func init() {
	_ = filepath.Base
	_ = time.Now
}
