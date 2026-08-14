package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockHTTPClient struct {
	responses []*http.Response
	reqs      []*http.Request
	index     int
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.reqs = append(m.reqs, req)
	if m.index >= len(m.responses) {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestDiscover_InvalidResourceURL(t *testing.T) {
	client := NewMCPDiscoveryClient(nil)
	_, _, err := client.Discover(context.Background(), "not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "MCP_OAUTH_DISCOVERY_FAILED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscover_EmptyScheme(t *testing.T) {
	client := NewMCPDiscoveryClient(nil)
	_, _, err := client.Discover(context.Background(), "://missing-scheme")
	if err == nil {
		t.Fatal("expected error for empty scheme")
	}
}

func TestDiscover_ProtectedResourceSuccess(t *testing.T) {
	protected := MCPProtectedResourceMetadata{
		Resource:             "https://api.example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
		ScopesSupported:      []string{"read", "write"},
	}
	protectedJSON, _ := json.Marshal(protected)

	asMeta := MCPAuthorizationServerMetadata{
		Issuer:                        "https://auth.example.com",
		AuthorizationEndpoint:         "https://auth.example.com/authorize",
		TokenEndpoint:                 "https://auth.example.com/token",
		CodeChallengeMethodsSupported: []string{"S256"},
	}
	asJSON, _ := json.Marshal(asMeta)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(protectedJSON)),
			jsonResponse(200, string(asJSON)),
		},
	}

	client := NewMCPDiscoveryClient(mock)
	p, a, err := client.Discover(context.Background(), "https://api.example.com/mcp")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if p.Resource != "https://api.example.com/mcp" {
		t.Fatalf("resource mismatch: %s", p.Resource)
	}
	if len(p.AuthorizationServers) != 1 || p.AuthorizationServers[0] != "https://auth.example.com" {
		t.Fatalf("authorization servers mismatch: %v", p.AuthorizationServers)
	}
	if a.Issuer != "https://auth.example.com" {
		t.Fatalf("issuer mismatch: %s", a.Issuer)
	}
}

func TestDiscover_ResourceMismatch(t *testing.T) {
	protected := MCPProtectedResourceMetadata{
		Resource:             "https://other.example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
	}
	protectedJSON, _ := json.Marshal(protected)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(protectedJSON)),
		},
	}

	client := NewMCPDiscoveryClient(mock)
	_, _, err := client.Discover(context.Background(), "https://api.example.com/mcp")
	if err == nil {
		t.Fatal("expected error for resource mismatch")
	}
}

func TestDiscover_NoAuthorizationServer(t *testing.T) {
	protected := MCPProtectedResourceMetadata{
		Resource:             "https://api.example.com/mcp",
		AuthorizationServers: []string{},
	}
	protectedJSON, _ := json.Marshal(protected)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(protectedJSON)),
		},
	}

	client := NewMCPDiscoveryClient(mock)
	_, _, err := client.Discover(context.Background(), "https://api.example.com/mcp")
	if err == nil {
		t.Fatal("expected error for missing authorization server")
	}
}

func TestDiscover_IssuerMismatch(t *testing.T) {
	protected := MCPProtectedResourceMetadata{
		Resource:             "https://api.example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
	}
	protectedJSON, _ := json.Marshal(protected)

	asMeta := MCPAuthorizationServerMetadata{
		Issuer:                        "https://different.example.com",
		AuthorizationEndpoint:         "https://auth.example.com/authorize",
		TokenEndpoint:                 "https://auth.example.com/token",
		CodeChallengeMethodsSupported: []string{"S256"},
	}
	asJSON, _ := json.Marshal(asMeta)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(protectedJSON)),
			jsonResponse(200, string(asJSON)),
		},
	}

	client := NewMCPDiscoveryClient(mock)
	_, _, err := client.Discover(context.Background(), "https://api.example.com/mcp")
	if err == nil {
		t.Fatal("expected error for issuer mismatch")
	}
}

func TestDiscover_PKCE_S256Required(t *testing.T) {
	protected := MCPProtectedResourceMetadata{
		Resource:             "https://api.example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
	}
	protectedJSON, _ := json.Marshal(protected)

	asMeta := MCPAuthorizationServerMetadata{
		Issuer:                "https://auth.example.com",
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
	}
	asJSON, _ := json.Marshal(asMeta)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(protectedJSON)),
			jsonResponse(200, string(asJSON)),
		},
	}

	client := NewMCPDiscoveryClient(mock)
	_, _, err := client.Discover(context.Background(), "https://api.example.com/mcp")
	if err == nil {
		t.Fatal("expected error for missing PKCE S256")
	}
	if !strings.Contains(err.Error(), "S256") {
		t.Fatalf("expected PKCE S256 error, got: %v", err)
	}
}

func TestDiscover_PathScopedWellKnown(t *testing.T) {
	protected := MCPProtectedResourceMetadata{
		Resource:             "https://api.example.com/v1/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
	}
	protectedJSON, _ := json.Marshal(protected)

	asMeta := MCPAuthorizationServerMetadata{
		Issuer:                        "https://auth.example.com",
		AuthorizationEndpoint:         "https://auth.example.com/authorize",
		TokenEndpoint:                 "https://auth.example.com/token",
		CodeChallengeMethodsSupported: []string{"S256"},
	}
	asJSON, _ := json.Marshal(asMeta)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(404, ""),
			jsonResponse(200, string(protectedJSON)),
			jsonResponse(200, string(asJSON)),
		},
	}

	client := NewMCPDiscoveryClient(mock)
	p, _, err := client.Discover(context.Background(), "https://api.example.com/v1/mcp")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if p.Resource != "https://api.example.com/v1/mcp" {
		t.Fatalf("resource mismatch: %s", p.Resource)
	}
}

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://api.example.com/mcp", "https://api.example.com/other", true},
		{"https://api.example.com", "https://other.example.com", false},
		{"http://api.example.com", "https://api.example.com", false},
		{"invalid", "https://api.example.com", false},
	}
	for _, tt := range tests {
		got := sameOrigin(tt.a, tt.b)
		if got != tt.want {
			t.Fatalf("sameOrigin(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
