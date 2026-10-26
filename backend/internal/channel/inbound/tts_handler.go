package inbound

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxChannelTTSTextBytes = 8192

type TTSProvider interface {
	Synthesize(ctx context.Context, text string) (string, error)
}

type SynthesizeFunc func(ctx context.Context, text string) (string, error)

func (f SynthesizeFunc) Synthesize(ctx context.Context, text string) (string, error) {
	return f(ctx, text)
}

type TTSHandler struct {
	providers ProviderDefinitionSource
	tts       TTSProvider
}

func RegisterTTSRouter(router gin.IRouter, providers ProviderDefinitionSource, tts TTSProvider) {
	if router == nil || providers == nil || tts == nil {
		return
	}
	handler := &TTSHandler{providers: providers, tts: tts}
	router.POST("/api/channels/tts/preview", handler.Handle)
}

func (h *TTSHandler) Handle(c *gin.Context) {
	if h == nil || h.providers == nil || h.tts == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": http.StatusServiceUnavailable, "msg": "channel tts service unavailable"})
		return
	}
	if !isLoopbackRequest(c.Request.RemoteAddr) {
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "channel tts requires loopback access"})
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
	var body struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "请求格式错误"})
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" || len([]byte(text)) > maxChannelTTSTextBytes {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "语音文本为空或超过上限"})
		return
	}
	audioURL, err := h.tts.Synthesize(c.Request.Context(), text)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": http.StatusInternalServerError, "msg": err.Error()})
		return
	}
	if strings.TrimSpace(audioURL) == "" {
		c.JSON(http.StatusOK, gin.H{"code": http.StatusInternalServerError, "msg": "语音合成未返回音频"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "操作成功", "data": gin.H{"audioUrl": audioURL}})
}
