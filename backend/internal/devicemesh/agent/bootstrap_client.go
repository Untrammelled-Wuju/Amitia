package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type BootstrapClient struct {
	httpClient *http.Client
}

func NewBootstrapClient() *BootstrapClient {
	return &BootstrapClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *BootstrapClient) Exchange(ctx context.Context, cloudBaseURL, rawTicket, deviceID, runtimeID, platform, runtimeVersion string) (*ExchangeResponse, error) {
	cloudBaseURL = strings.TrimRight(cloudBaseURL, "/")

	reqBody := map[string]interface{}{
		"deviceId":        deviceID,
		"runtimeId":       runtimeID,
		"platform":        platform,
		"runtimeVersion":  runtimeVersion,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("bootstrap client: marshal request: %w", err)
	}

	url := cloudBaseURL + "/api/device-mesh/v1/bootstrap/exchange"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("bootstrap client: create request: %w", err)
	}
	req.Header.Set("Authorization", "AmitiaBootstrap "+rawTicket)
	req.Header.Set("Content-Type", "application/json")

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

	return &result, nil
}

type ExchangeResponse struct {
	CredentialID    string `json:"credentialId"`
	Credential      string `json:"credential"`
	UserID          string `json:"userId"`
	DeviceID        string `json:"deviceId"`
	RuntimeID       string `json:"runtimeId"`
	ExpiresAt       string `json:"expiresAt"`
	Protocol        string `json:"protocol"`
	EnvelopeVersion int    `json:"envelopeVersion"`
	SchemaVersion   string `json:"schemaVersion"`
	WebSocketPath   string `json:"websocketPath"`
}
