package artifact

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
)

const previewPolicy = "sandbox allow-scripts; default-src 'none'; img-src https: http: data: blob:; style-src 'unsafe-inline' https: http:; script-src 'unsafe-inline' https: http:; font-src https: http: data:; connect-src 'none'; frame-src https: http: data: blob:; media-src https: http: data: blob:; base-uri https: http:; form-action 'none'"

type previewDocument struct {
	source    string
	owner     string
	expiresAt time.Time
}

func (h *Handler) CreatePreview(c *gin.Context) {
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "artifact.unauthorized"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 5*1024*1024)
	var input struct {
		HTML string `json:"html"`
	}
	if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.HTML) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "artifact.invalid_preview"})
		return
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "artifact.preview_failed"})
		return
	}
	id := hex.EncodeToString(nonce[:])
	owner := string(actor.SpaceID)
	ticket, expiresAt, err := h.ticketSigner.Issue(ID(id), owner)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "artifact.preview_failed"})
		return
	}
	h.previewMu.Lock()
	retainedBytes := 0
	for key, value := range h.previews {
		if !value.expiresAt.After(time.Now()) {
			delete(h.previews, key)
		} else {
			retainedBytes += len(value.source)
		}
	}
	if len(h.previews) >= 128 || retainedBytes+len(input.HTML) > 32*1024*1024 {
		h.previewMu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "artifact.preview_limit"})
		return
	}
	h.previews[id] = previewDocument{source: input.HTML, owner: owner, expiresAt: expiresAt}
	h.previewMu.Unlock()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"previewId": id, "url": fmt.Sprintf("/media/artifact-previews/%s/%s", id, ticket), "expiresAt": expiresAt.Format(time.RFC3339)})
}

func (h *Handler) DeletePreview(c *gin.Context) {
	actor, err := middleware.GetActorFromContext(c)
	if err != nil || actor == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "artifact.unauthorized"})
		return
	}
	id := c.Param("previewId")
	h.previewMu.Lock()
	if document, ok := h.previews[id]; ok && document.owner == string(actor.SpaceID) {
		delete(h.previews, id)
	}
	h.previewMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) GetPreview(c *gin.Context) {
	id := c.Param("previewId")
	owner, err := h.ticketSigner.Validate(c.Param("ticket"), ID(id))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "artifact.unauthorized"})
		return
	}
	h.previewMu.Lock()
	document, found := h.previews[id]
	h.previewMu.Unlock()
	if !found || document.owner != owner || !document.expiresAt.After(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact.preview_expired"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Security-Policy", previewPolicy)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Disposition", "inline")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(document.source))
}
