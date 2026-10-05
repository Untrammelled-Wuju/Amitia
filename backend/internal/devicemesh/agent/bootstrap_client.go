package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"io"
	"net/http"
	"strings"
	"time"
)

type BootstrapClient struct {
	httpClient *http.Client
	identity   *IdentityStore
	core       string
}

func (c *BootstrapClient) SetIdentity(identity *IdentityStore, core string) {
	c.identity = identity
	c.core = core
}

func NewBootstrapClient() *BootstrapClient {
	return &BootstrapClient{
		httpClient: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

func NewPinnedBootstrapClient(endpoint lan.Endpoint) (*BootstrapClient, error) {
	configuration, err := lan.PinnedTLS(endpoint)
	if err != nil {
		return nil, err
	}
	return &BootstrapClient{httpClient: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: configuration}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *BootstrapClient) Exchange(ctx context.Context, cloudBaseURL, rawTicket, deviceID, runtimeID, platform, runtimeVersion string) (*ExchangeResponse, error) {
	cloudBaseURL = strings.TrimRight(cloudBaseURL, "/")

	reqBody := map[string]interface{}{
		"deviceId":       deviceID,
		"runtimeId":      runtimeID,
		"platform":       platform,
		"runtimeVersion": runtimeVersion,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("bootstrap client: marshal request: %w", err)
	}

	url := cloudBaseURL + "/api/public/device-mesh/v1/bootstrap/exchange"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("bootstrap client: create request: %w", err)
	}
	req.Header.Set("Authorization", "AmitiaBootstrap "+rawTicket)
	req.Header.Set("Content-Type", "application/json")
	if c.identity != nil {
		core := c.core
		if core == "" {
			statusRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, cloudBaseURL+"/api/public/device-mesh/v1/pairing/status", nil)
			if err != nil {
				return nil, err
			}
			statusResponse, err := c.httpClient.Do(statusRequest)
			if err != nil {
				return nil, err
			}
			var status struct {
				SpaceID string `json:"spaceId"`
			}
			err = json.NewDecoder(io.LimitReader(statusResponse.Body, 1<<20)).Decode(&status)
			_ = statusResponse.Body.Close()
			if err != nil || statusResponse.StatusCode != http.StatusOK || status.SpaceID == "" {
				return nil, fmt.Errorf("无法验证服务提供者身份")
			}
			core = status.SpaceID
		}
		if err := c.identity.SignRequest(req, core); err != nil {
			return nil, err
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bootstrap client: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("bootstrap client: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bootstrap client: exchange failed status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var result ExchangeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("bootstrap client: parse response: %w", err)
	}
	if result.Protocol != meshprotocol.ProtocolName || result.EnvelopeVersion != meshprotocol.EnvelopeVersion || result.SchemaVersion != meshprotocol.SchemaVersion || result.WebSocketPath != meshprotocol.WebSocketPath {
		return nil, fmt.Errorf("服务提供者协议不兼容，拒绝切换")
	}

	return &result, nil
}

type ExchangeResponse struct {
	CredentialID    string `json:"credentialId"`
	Credential      string `json:"credential"`
	SpaceID         string `json:"spaceId"`
	DeviceID        string `json:"deviceId"`
	RuntimeID       string `json:"runtimeId"`
	ExpiresAt       string `json:"expiresAt"`
	Protocol        string `json:"protocol"`
	EnvelopeVersion int    `json:"envelopeVersion"`
	SchemaVersion   string `json:"schemaVersion"`
	WebSocketPath   string `json:"websocketPath"`
}
