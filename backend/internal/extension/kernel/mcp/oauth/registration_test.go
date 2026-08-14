package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRegistrationStrategy_PreRegisteredPriority(t *testing.T) {
	mock := &mockHTTPClient{}
	strategy := NewMCPRegistrationStrategy(mock)

	preReg := MCPOAuthClientRegistration{
		RegistrationMethod: "pre-registered",
		ClientID:           "pre-client-id",
		Issuer:             "https://auth.example.com",
	}
	strategy.SetPreRegistered("https://auth.example.com", preReg)

	reg, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err != nil {
		t.Fatalf("expected no error for pre-registered, got: %v", err)
	}
	if reg.RegistrationMethod != "pre-registered" {
		t.Fatalf("expected pre-registered, got %s", reg.RegistrationMethod)
	}
	if reg.ClientID != "pre-client-id" {
		t.Fatalf("client_id mismatch: %s", reg.ClientID)
	}
}

func TestRegistrationStrategy_CIMDPriority(t *testing.T) {
	cimdDoc := map[string]any{
		"client_id":                  "https://amitia.example.com/client-metadata.json",
		"client_name":                "Amitia",
		"redirect_uris":              []string{"http://127.0.0.1:8080/callback"},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	}
	cimdJSON, _ := json.Marshal(cimdDoc)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(cimdJSON)),
		},
	}

	strategy := NewMCPRegistrationStrategy(mock)
	strategy.SetCIMDURL("https://auth.example.com", "https://amitia.example.com/client-metadata.json")

	reg, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err != nil {
		t.Fatalf("expected no error for CIMD, got: %v", err)
	}
	if reg.RegistrationMethod != "cimd" {
		t.Fatalf("expected cimd, got %s", reg.RegistrationMethod)
	}
	if reg.ClientID != "https://amitia.example.com/client-metadata.json" {
		t.Fatalf("client_id mismatch: %s", reg.ClientID)
	}
}

func TestRegistrationStrategy_CIMD_ClientIDMustMatchURL(t *testing.T) {
	cimdDoc := map[string]any{
		"client_id":     "https://different-url.example.com/metadata",
		"client_name":   "Amitia",
		"redirect_uris": []string{"http://127.0.0.1:8080/callback"},
	}
	cimdJSON, _ := json.Marshal(cimdDoc)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(cimdJSON)),
		},
	}

	strategy := NewMCPRegistrationStrategy(mock)
	strategy.SetCIMDURL("https://auth.example.com", "https://amitia.example.com/client-metadata.json")

	_, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err == nil {
		t.Fatal("expected error when CIMD client_id != document URL")
	}
}

func TestRegistrationStrategy_DCRFallback(t *testing.T) {
	dcrResp := map[string]any{
		"client_id":                  "dcr-client-id",
		"client_secret":              "dcr-secret",
		"redirect_uris":              []string{"http://127.0.0.1:8080/callback"},
		"token_endpoint_auth_method": "none",
	}
	dcrJSON, _ := json.Marshal(dcrResp)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(201, string(dcrJSON)),
		},
	}

	strategy := NewMCPRegistrationStrategy(mock)
	strategy.SetDCREndpoint("https://auth.example.com/register")

	reg, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err != nil {
		t.Fatalf("expected no error for DCR, got: %v", err)
	}
	if reg.RegistrationMethod != "dcr" {
		t.Fatalf("expected dcr, got %s", reg.RegistrationMethod)
	}
	if reg.ClientID != "dcr-client-id" {
		t.Fatalf("client_id mismatch: %s", reg.ClientID)
	}
}

func TestRegistrationStrategy_ManualFallback(t *testing.T) {
	mock := &mockHTTPClient{}
	strategy := NewMCPRegistrationStrategy(mock)

	_, err := strategy.ResolveClient(context.Background(), "https://unknown.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err == nil {
		t.Fatal("expected error when no registration method available")
	}
	if !strings.Contains(err.Error(), "MCP_OAUTH_REGISTRATION_UNAVAILABLE") {
		t.Fatalf("expected registration unavailable error, got: %v", err)
	}
}

func TestRegistrationStrategy_DCR_MissingClientID(t *testing.T) {
	dcrResp := map[string]any{
		"redirect_uris": []string{"http://127.0.0.1:8080/callback"},
	}
	dcrJSON, _ := json.Marshal(dcrResp)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(201, string(dcrJSON)),
		},
	}

	strategy := NewMCPRegistrationStrategy(mock)
	strategy.SetDCREndpoint("https://auth.example.com/register")

	_, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err == nil {
		t.Fatal("expected error when DCR response missing client_id")
	}
}

func TestRegistrationStrategy_CIMDOverDCR(t *testing.T) {
	cimdDoc := map[string]any{
		"client_id":     "https://amitia.example.com/metadata",
		"client_name":   "Amitia",
		"redirect_uris": []string{"http://127.0.0.1:8080/callback"},
	}
	cimdJSON, _ := json.Marshal(cimdDoc)

	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(200, string(cimdJSON)),
		},
	}

	strategy := NewMCPRegistrationStrategy(mock)
	strategy.SetCIMDURL("https://auth.example.com", "https://amitia.example.com/metadata")
	strategy.SetDCREndpoint("https://auth.example.com/register")

	reg, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if reg.RegistrationMethod != "cimd" {
		t.Fatalf("CIMD should take priority over DCR, got: %s", reg.RegistrationMethod)
	}
}

func TestRegistrationStrategy_CIMD_InvalidURL(t *testing.T) {
	mock := &mockHTTPClient{}
	strategy := NewMCPRegistrationStrategy(mock)
	strategy.SetCIMDURL("https://auth.example.com", "http://insecure-metadata.example.com/doc")

	_, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err == nil {
		t.Fatal("expected error for non-HTTPS CIMD URL")
	}
}

func TestRegistrationStrategy_DCR_ApplicationTypeNative(t *testing.T) {
	mock := &mockHTTPClient{
		responses: []*http.Response{
			jsonResponse(201, `{"client_id":"test","redirect_uris":["http://127.0.0.1:8080/callback"]}`),
		},
	}

	strategy := NewMCPRegistrationStrategy(mock)
	strategy.SetDCREndpoint("https://auth.example.com/register")

	_, err := strategy.ResolveClient(context.Background(), "https://auth.example.com", MCPRedirectProfile{
		URIs: []string{"http://127.0.0.1:8080/callback"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.reqs) == 0 {
		t.Fatal("expected a request to be made")
	}

	body, _ := io.ReadAll(mock.reqs[0].Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, `"application_type":"native"`) {
		t.Fatalf("expected application_type=native in DCR payload, got: %s", bodyStr)
	}
}
