package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	"github.com/u-ai/backend/pkg/util"
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
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	client, err := NewPinnedBootstrapClient(request.Endpoint)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	identity, err := h.identity.Load()
	if err != nil {
		c.JSON(503, gin.H{"message": err.Error()})
		return
	}
	h.mu.RLock()
	localCore := h.localCoreID
	version := h.bindingVersion
	credential, credentialErr := h.credStore.LoadCredential()
	h.mu.RUnlock()
	if credentialErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": credentialErr.Error()})
		return
	}
	if localCore == "" {
		localCore = identity.DeviceID.String()
	}
	if request.Endpoint.CoreID == localCore || request.Endpoint.CoreID == identity.DeviceID.String() {
		c.JSON(http.StatusConflict, gin.H{"code": "pairing.self", "message": "不能添加当前设备自身"})
		return
	}
	if activeCredentialForCenter(credential, request.Endpoint.CoreID, time.Now()) {
		c.JSON(http.StatusConflict, gin.H{"code": "pairing.already_paired", "message": "该设备已添加，无需重复扫码"})
		return
	}
	if _, err := client.ProviderPath(c.Request.Context(), request.Endpoint.URL, request.Endpoint.CoreID, localCore); err != nil {
		if errors.Is(err, coordination.ErrProviderCycle) {
			c.JSON(http.StatusConflict, gin.H{"code": "pairing.provider_topology_invalid", "message": err.Error()})
		} else {
			c.JSON(http.StatusBadGateway, gin.H{"code": "pairing.provider_unavailable", "message": err.Error()})
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
	signed := proof.New(identity.PublicKey, request.Endpoint.CoreID, uuid.NewString(), body, time.Now())
	signed.Signature, err = h.identity.Sign(signed.SigningBytes())
	if err != nil {
		c.JSON(503, gin.H{"message": err.Error()})
		return
	}
	encoded, err := json.Marshal(struct {
		proof.ClaimBody
		Proof *proof.Proof `json:"proof"`
	}{ClaimBody: body, Proof: &signed})
	if err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	upstream, err := http.NewRequestWithContext(c.Request.Context(), "POST", strings.TrimRight(request.Endpoint.URL, "/")+"/api/public/device-mesh/v1/pairing/claim", bytes.NewReader(encoded))
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(upstream)
	if err != nil {
		c.JSON(502, gin.H{"message": err.Error()})
		return
	}
	defer response.Body.Close()
	result, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(result) > 1<<20 || !json.Valid(result) {
		c.JSON(502, gin.H{"message": util.OriginalErrorMessage(err, "配对服务响应无效")})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(response.StatusCode, "application/json", result)
}

func activeCredentialForCenter(credential *StoredCredential, coreID string, now time.Time) bool {
	return credential != nil &&
		strings.TrimSpace(credential.Credential) != "" &&
		credential.SpaceID.String() == strings.TrimSpace(coreID) &&
		credential.ExpiresAt.After(now)
}

func (h *LocalHandler) handlePinnedPairingEvents(c *gin.Context) {
	endpoint := lan.Endpoint{
		URL:         strings.TrimSpace(c.Query("url")),
		Fingerprint: strings.TrimSpace(c.Query("fingerprint")),
		CoreID:      strings.TrimSpace(c.Query("coreId")),
	}
	client, err := NewPinnedBootstrapClient(endpoint)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	requestID := strings.TrimSpace(c.Param("requestId"))
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "requestId is required"})
		return
	}
	targetURL := strings.TrimRight(endpoint.URL, "/") + "/api/public/device-mesh/v1/pairing/requests/" + url.PathEscape(requestID) + "/events"
	upstream, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	upstream.Header.Set("Accept", "text/event-stream")
	client.httpClient.Timeout = 125 * time.Second
	response, err := client.httpClient.Do(upstream)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		c.Data(response.StatusCode, "application/json", body)
		if readErr != nil {
			c.Header("X-Amitia-Original-Error", readErr.Error())
		}
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	buffer := make([]byte, 4096)
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if _, writeErr := c.Writer.Write(buffer[:count]); writeErr != nil {
				return
			}
			c.Writer.Flush()
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				c.Header("X-Amitia-Original-Error", readErr.Error())
			}
			return
		}
	}
}
