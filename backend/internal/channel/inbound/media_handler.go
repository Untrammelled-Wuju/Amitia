package inbound

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	amitialog "log"
)

const maxChannelResourceBytes = 50 << 20

type ResourceLinkResolver interface {
	ResolveResourceLink(token string) (extensionID string, scope string, fullPath string, err error)
}

type MediaHandler struct {
	providers ProviderDefinitionSource
	resolver  ResourceLinkResolver
}

func RegisterMediaRouter(router gin.IRouter, providers ProviderDefinitionSource, resolver ResourceLinkResolver) {
	if router == nil || providers == nil || resolver == nil {
		return
	}
	handler := &MediaHandler{providers: providers, resolver: resolver}
	router.GET("/api/channels/media/:token", handler.Handle)
}

func (h *MediaHandler) Handle(c *gin.Context) {
	if h == nil || h.providers == nil || h.resolver == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": http.StatusServiceUnavailable, "msg": "channel media service unavailable"})
		return
	}
	if !isLoopbackRequest(c.Request.RemoteAddr) {
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "channel media requires loopback access"})
		return
	}
	extensionID := strings.TrimSpace(c.GetHeader("X-Amitia-Extension-ID"))
	moduleID := strings.TrimSpace(c.GetHeader("X-Amitia-Module-ID"))
	channelID := strings.TrimSpace(c.GetHeader("X-Amitia-Channel-ID"))
	if extensionID == "" || moduleID == "" || channelID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "channel provider identity is required"})
		return
	}
	if !validInboundToken(c.GetHeader("Authorization"), extensionID, moduleID) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "invalid channel provider credential"})
		return
	}
	if !providerDefinitionMatches(h.providers.ListByCapability(inboundProviderCapability), extensionID, moduleID, channelID) {
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "channel provider identity does not match the declared provider"})
		return
	}
	_, _, fullPath, err := h.resolver.ResolveResourceLink(strings.TrimSpace(c.Param("token")))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "msg": err.Error()})
		return
	}
	file, err := os.Open(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "msg": err.Error()})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() || info.Size() <= 0 || info.Size() > maxChannelResourceBytes {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "msg": err.Error()})
		return
	}
	contentType := mediaContentType(path.Base(fullPath), file)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
}

func mediaContentType(name string, file *os.File) string {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(lowerName, ".png"):
		return "image/png"
	case strings.HasSuffix(lowerName, ".jpg"), strings.HasSuffix(lowerName, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lowerName, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lowerName, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lowerName, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lowerName, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lowerName, ".mp3"):
		return "audio/mpeg"
	case strings.HasSuffix(lowerName, ".wav"):
		return "audio/wav"
	case strings.HasSuffix(lowerName, ".ogg"):
		return "audio/ogg"
	case strings.HasSuffix(lowerName, ".pdf"):
		return "application/pdf"
	}
	header := make([]byte, 512)
	count, _ := file.Read(header)
	if _, err := file.Seek(0, 0); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	return http.DetectContentType(header[:count])
}
