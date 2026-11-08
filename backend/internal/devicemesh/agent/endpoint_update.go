package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/lan"
)

type endpointUpdateRequest struct {
	CloudBaseURL string `json:"cloudBaseUrl" binding:"required"`
	Fingerprint  string `json:"fingerprint"`
	CoreID       string `json:"coreId" binding:"required"`
}

func (h *LocalHandler) handleUpdateEndpoint(c *gin.Context) {
	var request endpointUpdateRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.UpdateEndpoint(c.Request.Context(), request)
	if err != nil {
		if failure, ok := err.(*BindingError); ok {
			c.JSON(failure.Status, gin.H{"code": failure.Code, "message": failure.Message})
		} else {
			c.JSON(500, gin.H{"message": err.Error()})
		}
		return
	}
	c.JSON(200, result)
}

func (h *LocalHandler) UpdateEndpoint(ctx context.Context, request endpointUpdateRequest) (gin.H, error) {
	request.CloudBaseURL = strings.TrimRight(strings.TrimSpace(request.CloudBaseURL), "/")
	request.Fingerprint = strings.ToLower(strings.TrimSpace(request.Fingerprint))
	request.CoreID = strings.TrimSpace(request.CoreID)
	parsed, err := url.Parse(request.CloudBaseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || request.CoreID == "" {
		return nil, bindingFailure(400, gin.H{"code": "invalid_provider_endpoint", "message": "统筹中心地址无效"})
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.coreMode {
		return nil, bindingFailure(409, gin.H{"code": "mesh.agent_mode_required", "message": "当前 Runtime 是统筹中心节点，不能修改子设备统筹中心地址"})
	}
	previous, err := h.credStore.LoadCredential()
	if err != nil {
		return nil, err
	}
	if previous == nil || previous.Credential == "" {
		return nil, bindingFailure(409, gin.H{"code": "pairing_required", "message": "当前设备尚未绑定统筹中心"})
	}
	if previous.SpaceID.String() != request.CoreID {
		return nil, bindingFailure(409, gin.H{"code": "core_identity_changed", "message": "新地址不是当前已绑定的统筹中心，请重新配对"})
	}
	identity, err := h.identity.Load()
	if err != nil {
		return nil, err
	}
	if previous.DeviceID != identity.DeviceID || previous.RuntimeID != identity.RuntimeID {
		return nil, bindingFailure(403, gin.H{"code": "provider_identity_mismatch", "message": "本机身份与已绑定凭据不一致"})
	}

	endpoint := lan.Endpoint{URL: request.CloudBaseURL, Fingerprint: request.Fingerprint, CoreID: request.CoreID}
	var client *BootstrapClient
	var tlsConfig *tls.Config
	if request.Fingerprint != "" {
		client, err = NewPinnedBootstrapClient(endpoint)
		if err != nil {
			return nil, bindingFailure(400, gin.H{"code": "invalid_provider_identity", "message": err.Error()})
		}
		tlsConfig, err = lan.PinnedTLS(endpoint)
		if err != nil {
			return nil, bindingFailure(400, gin.H{"code": "invalid_provider_identity", "message": err.Error()})
		}
	} else {
		client = NewBootstrapClient()
	}
	localCore := h.localCoreID
	if localCore == "" {
		localCore = identity.DeviceID.String()
	}
	providerPath, err := client.ProviderPath(ctx, request.CloudBaseURL, request.CoreID, localCore)
	if err != nil {
		return nil, bindingFailure(409, gin.H{"code": "provider_topology_invalid", "message": err.Error()})
	}

	updated := *previous
	updated.CloudBaseUrl = request.CloudBaseURL
	updated.Fingerprint = request.Fingerprint
	updated.ProviderPath = providerPath
	result, err := h.activateProvider(ctx, &updated, identity, tlsConfig)
	if err == nil {
		return result, nil
	}
	activationErr := err

	oldEndpoint := lan.Endpoint{URL: previous.CloudBaseUrl, Fingerprint: previous.Fingerprint, CoreID: previous.SpaceID.String()}
	var oldTLS *tls.Config
	var oldTLSErr error
	if previous.Fingerprint != "" {
		oldTLS, oldTLSErr = lan.PinnedTLS(oldEndpoint)
	}
	if oldTLSErr == nil {
		if _, rollbackErr := h.activateProvider(ctx, previous, identity, oldTLS); rollbackErr == nil {
			return nil, bindingFailure(503, gin.H{"code": "endpoint_update_failed", "message": "新中心地址不可用，已恢复原连接"})
		} else if rollbackErr != nil {
			return nil, errors.Join(activationErr, rollbackErr)
		}
	}
	return nil, errors.Join(activationErr, oldTLSErr, errors.New("恢复原统筹中心连接失败"))
}
