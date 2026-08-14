package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type MCPRedirectProfile struct {
	URIs []string
}

type MCPClientRegistrationStrategy interface {
	ResolveClient(ctx context.Context, issuer string, redirect MCPRedirectProfile) (MCPOAuthClientRegistration, error)
}

type MCPRegistrationStrategy struct {
	preRegistered map[string]MCPOAuthClientRegistration
	cimdURLs      map[string]string
	dcrEndpoint   string
	client        MCPDiscoveryHTTPClient
}

func NewMCPRegistrationStrategy(client MCPDiscoveryHTTPClient) *MCPRegistrationStrategy {
	if client == nil {
		client = http.DefaultClient
	}
	return &MCPRegistrationStrategy{
		preRegistered: make(map[string]MCPOAuthClientRegistration),
		cimdURLs:      make(map[string]string),
		client:        client,
	}
}

func (s *MCPRegistrationStrategy) SetPreRegistered(issuer string, reg MCPOAuthClientRegistration) {
	s.preRegistered[issuer] = reg
}

func (s *MCPRegistrationStrategy) SetCIMDURL(issuer, cimdURL string) {
	s.cimdURLs[issuer] = cimdURL
}

func (s *MCPRegistrationStrategy) SetDCREndpoint(endpoint string) {
	s.dcrEndpoint = endpoint
}

func (s *MCPRegistrationStrategy) ResolveClient(ctx context.Context, issuer string, redirect MCPRedirectProfile) (MCPOAuthClientRegistration, error) {
	if reg, ok := s.preRegistered[issuer]; ok {
		return reg, nil
	}

	if cimdURL, ok := s.cimdURLs[issuer]; ok {
		if reg, err := s.resolveCIMD(ctx, cimdURL, redirect); err == nil {
			return reg, nil
		}
	}

	if s.dcrEndpoint != "" {
		if reg, err := s.resolveDCR(ctx, s.dcrEndpoint, redirect); err == nil {
			return reg, nil
		}
	}

	return MCPOAuthClientRegistration{
		RegistrationMethod:      "manual",
		Issuer:                  issuer,
		RedirectURIs:            redirect.URIs,
		TokenEndpointAuthMethod: "none",
	}, fmt.Errorf("MCP_OAUTH_REGISTRATION_UNAVAILABLE: manual configuration required for issuer %s", issuer)
}

func (s *MCPRegistrationStrategy) resolveCIMD(ctx context.Context, cimdURL string, redirect MCPRedirectProfile) (MCPOAuthClientRegistration, error) {
	parsed, err := url.Parse(cimdURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return MCPOAuthClientRegistration{}, fmt.Errorf("MCP_OAUTH_CIMD_INVALID")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cimdURL, nil)
	if err != nil {
		return MCPOAuthClientRegistration{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return MCPOAuthClientRegistration{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return MCPOAuthClientRegistration{}, fmt.Errorf("MCP_OAUTH_CIMD_FAILED")
	}

	var meta struct {
		ClientID                string   `json:"client_id"`
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&meta); err != nil {
		return MCPOAuthClientRegistration{}, fmt.Errorf("MCP_OAUTH_CIMD_INVALID")
	}

	if meta.ClientID != cimdURL {
		return MCPOAuthClientRegistration{}, fmt.Errorf("MCP_OAUTH_CIMD_MISMATCH")
	}

	return MCPOAuthClientRegistration{
		RegistrationMethod:      "cimd",
		ClientID:                meta.ClientID,
		RedirectURIs:            meta.RedirectURIs,
		TokenEndpointAuthMethod: meta.TokenEndpointAuthMethod,
	}, nil
}

func (s *MCPRegistrationStrategy) resolveDCR(ctx context.Context, endpoint string, redirect MCPRedirectProfile) (MCPOAuthClientRegistration, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return MCPOAuthClientRegistration{}, fmt.Errorf("MCP_OAUTH_DCR_INVALID")
	}

	payload := map[string]any{
		"client_name":                "Amitia",
		"redirect_uris":              redirect.URIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"application_type":           "native",
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return MCPOAuthClientRegistration{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return MCPOAuthClientRegistration{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MCPOAuthClientRegistration{}, fmt.Errorf("MCP_OAUTH_DCR_FAILED")
	}

	var result struct {
		ClientID                string   `json:"client_id"`
		ClientSecret            string   `json:"client_secret"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil || result.ClientID == "" {
		return MCPOAuthClientRegistration{}, fmt.Errorf("MCP_OAUTH_DCR_INVALID")
	}

	return MCPOAuthClientRegistration{
		RegistrationMethod:      "dcr",
		ClientID:                result.ClientID,
		ClientSecretRef:         result.ClientSecret,
		RedirectURIs:            result.RedirectURIs,
		TokenEndpointAuthMethod: result.TokenEndpointAuthMethod,
	}, nil
}
