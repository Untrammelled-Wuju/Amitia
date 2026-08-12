package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type MCPDiscoveryHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type MCPDiscoveryClient struct {
	client MCPDiscoveryHTTPClient
	clock  func() time.Time
}

func NewMCPDiscoveryClient(client MCPDiscoveryHTTPClient) *MCPDiscoveryClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &MCPDiscoveryClient{client: client, clock: func() time.Time { return time.Now().UTC() }}
}

func (d *MCPDiscoveryClient) Discover(ctx context.Context, resourceURL string) (MCPProtectedResourceMetadata, MCPAuthorizationServerMetadata, error) {
	resource, err := url.Parse(resourceURL)
	if err != nil || resource.Scheme == "" || resource.Host == "" {
		return MCPProtectedResourceMetadata{}, MCPAuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: invalid resource URL")
	}

	protected, err := d.fetchProtectedResource(ctx, resource)
	if err != nil {
		return MCPProtectedResourceMetadata{}, MCPAuthorizationServerMetadata{}, err
	}

	if protected.Resource != "" && !sameOrigin(protected.Resource, resourceURL) {
		return protected, MCPAuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: resource mismatch")
	}

	if len(protected.AuthorizationServers) == 0 {
		return protected, MCPAuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: no authorization server")
	}

	issuer := protected.AuthorizationServers[0]
	asMeta, err := d.fetchAuthorizationServer(ctx, issuer, resource)
	if err != nil {
		return protected, MCPAuthorizationServerMetadata{}, err
	}

	return protected, asMeta, nil
}

func (d *MCPDiscoveryClient) fetchProtectedResource(ctx context.Context, resource *url.URL) (MCPProtectedResourceMetadata, error) {
	paths := []string{
		resource.Scheme + "://" + resource.Host + resource.Path + "/.well-known/oauth-protected-resource",
		resource.Scheme + "://" + resource.Host + "/.well-known/oauth-protected-resource",
	}

	var lastErr error
	for _, endpoint := range paths {
		var meta MCPProtectedResourceMetadata
		if err := d.getJSON(ctx, endpoint, resource.Host, &meta); err == nil {
			return meta, nil
		} else {
			lastErr = err
		}
	}
	return MCPProtectedResourceMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: %w", lastErr)
}

func (d *MCPDiscoveryClient) fetchAuthorizationServer(ctx context.Context, issuerURL string, resource *url.URL) (MCPAuthorizationServerMetadata, error) {
	issuer, err := url.Parse(issuerURL)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
		return MCPAuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: invalid issuer")
	}

	paths := []string{
		strings.TrimRight(issuerURL, "/") + resource.Path + "/.well-known/oauth-authorization-server",
		strings.TrimRight(issuerURL, "/") + "/.well-known/oauth-authorization-server",
		strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration",
	}

	var lastErr error
	for _, endpoint := range paths {
		var meta MCPAuthorizationServerMetadata
		if err := d.getJSON(ctx, endpoint, issuer.Host, &meta); err == nil {
			if meta.Issuer != "" && !strings.EqualFold(strings.TrimRight(meta.Issuer, "/"), strings.TrimRight(issuerURL, "/")) {
				return MCPAuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: issuer mismatch")
			}
			if err := validateASMetadata(meta, issuer.Host); err != nil {
				return MCPAuthorizationServerMetadata{}, err
			}
			return meta, nil
		} else {
			lastErr = err
		}
	}
	return MCPAuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: %w", lastErr)
}

func (d *MCPDiscoveryClient) getJSON(ctx context.Context, endpoint, expectedHost string, target interface{}) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), expectedHost) {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: endpoint host mismatch")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: decode error")
	}
	return nil
}

func validateASMetadata(meta MCPAuthorizationServerMetadata, issuerHost string) error {
	if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: missing required endpoints")
	}

	for _, endpoint := range []string{meta.AuthorizationEndpoint, meta.TokenEndpoint, meta.RegistrationEndpoint, meta.RevocationEndpoint} {
		if endpoint == "" {
			continue
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), issuerHost) {
			return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: endpoint security violation")
		}
	}

	found := false
	for _, method := range meta.CodeChallengeMethodsSupported {
		if method == "S256" {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: PKCE S256 required")
	}
	return nil
}

func sameOrigin(a, b string) bool {
	u1, err1 := url.Parse(a)
	u2, err2 := url.Parse(b)
	return err1 == nil && err2 == nil && strings.EqualFold(u1.Scheme, u2.Scheme) && strings.EqualFold(u1.Host, u2.Host)
}

func NormalizeScopes(scopes []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		result = append(result, s)
	}
	sort.Strings(result)
	return result
}

type WWWAuthenticateChallenge struct {
	ResourceMetadata string
	Scope            string
	Error            string
	ErrorDescription string
	Issuer           string
}

func ParseWWWAuthenticate(header string) WWWAuthenticateChallenge {
	result := WWWAuthenticateChallenge{}
	if header == "" {
		return result
	}

	if i := strings.Index(strings.ToLower(header), "bearer"); i >= 0 {
		params := header[i+6:]
		for _, part := range strings.Split(params, ",") {
			part = strings.TrimSpace(part)
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.Trim(strings.TrimSpace(kv[1]), "\"")
			switch key {
			case "resource_metadata":
				result.ResourceMetadata = val
			case "scope":
				result.Scope = val
			case "error":
				result.Error = val
			case "error_description":
				result.ErrorDescription = val
			case "issuer":
				result.Issuer = val
			}
		}
	}
	return result
}
