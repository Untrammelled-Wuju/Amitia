package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/proof"
)

func (h *LocalHandler) handlePinnedPairing(c *gin.Context) {
	var request struct {
		Endpoint   lan.Endpoint `json:"endpoint"`
		OfferToken string       `json:"offerToken"`
		SetupCode  string       `json:"setupCode"`
		Label      string       `json:"label"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"message": "局域网配对参数无效"})
		return
	}
	client, err := NewPinnedBootstrapClient(request.Endpoint)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	identity, err := h.identity.Load()
	if err != nil {
		c.JSON(503, gin.H{"message": "当前设备身份不可用"})
		return
	}
	h.mu.RLock()
	localCore := h.localCoreID
	version := h.bindingVersion
	credential, credentialErr := h.credStore.LoadCredential()
	h.mu.RUnlock()
	if credentialErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "当前配对状态不可用，请稍后重试"})
		return
	}
	if localCore == "" {
		localCore = identity.DeviceID.String()
	}
	if request.Endpoint.CoreID == localCore || request.Endpoint.CoreID == identity.DeviceID.String() {
		c.JSON(http.StatusConflict, gin.H{"code": "pairing.self", "message": "不能添加当前设备自身"})
		return
	}
	if credential != nil && credential.SpaceID.String() == request.Endpoint.CoreID {
		c.JSON(http.StatusConflict, gin.H{"code": "pairing.already_paired", "message": "该设备已添加，无需重复扫码"})
		return
	}
	if _, err := client.ProviderPath(c.Request.Context(), request.Endpoint.URL, request.Endpoint.CoreID, localCore); err != nil {
		if errors.Is(err, coordination.ErrProviderCycle) {
			c.JSON(http.StatusConflict, gin.H{"code": "pairing.provider_topology_invalid", "message": "该设备已在当前服务连接链中，不能反向或重复扫码添加"})
		} else {
			c.JSON(http.StatusBadGateway, gin.H{"code": "pairing.provider_unavailable", "message": "无法验证局域网服务身份或连接链，请恢复连接后重试"})
		}
		return
	}
	h.mu.RLock()
	unchanged := version == h.bindingVersion
	h.mu.RUnlock()
	if !unchanged {
		c.JSON(http.StatusConflict, gin.H{"code": "pairing.binding_changed", "message": "设备绑定已变化，请刷新后重新扫码"})
		return
	}
	body := proof.ClaimBody{DeviceID: identity.DeviceID.String(), RuntimeID: identity.RuntimeID.String(), Platform: h.platform.String(), Label: strings.TrimSpace(request.Label), OfferToken: strings.TrimSpace(request.OfferToken), SetupCode: strings.TrimSpace(request.SetupCode)}
	if body.OfferToken == "" && body.SetupCode == "" {
		c.JSON(400, gin.H{"message": "缺少配对码"})
		return
	}
	signed := proof.New(identity.PublicKey, request.Endpoint.CoreID, uuid.NewString(), body, time.Now())
	signed.Signature, err = h.identity.Sign(signed.SigningBytes())
	if err != nil {
		c.JSON(503, gin.H{"message": "设备身份签名失败"})
		return
	}
	encoded, err := json.Marshal(struct {
		proof.ClaimBody
		Proof *proof.Proof `json:"proof"`
	}{ClaimBody: body, Proof: &signed})
	if err != nil {
		c.JSON(500, gin.H{"message": "设备配对请求编码失败"})
		return
	}
	upstream, err := http.NewRequestWithContext(c.Request.Context(), "POST", strings.TrimRight(request.Endpoint.URL, "/")+"/api/public/device-mesh/v1/pairing/claim", bytes.NewReader(encoded))
	if err != nil {
		c.JSON(400, gin.H{"message": "局域网配对地址无效"})
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(upstream)
	if err != nil {
		c.JSON(502, gin.H{"message": "局域网服务身份验证或连接失败"})
		return
	}
	defer response.Body.Close()
	result, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(result) > 1<<20 || !json.Valid(result) {
		c.JSON(502, gin.H{"message": "局域网配对响应无效"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(response.StatusCode, "application/json", result)
}
