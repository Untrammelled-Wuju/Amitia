package hostapi

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestRestrictedNetworkDomainAndPortMatching(t *testing.T) {
	client, err := newRestrictedHTTPClient(trusted_service.ServiceNetworkPolicy{
		Mode: "restricted", Enforce: true, RequireProxy: true,
		AllowedDomains: []string{"api.example.com", "*.cdn.example.com"}, AllowedPorts: []int{443, 8443},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"api.example.com", "img.cdn.example.com", "deep.img.cdn.example.com"} {
		if !client.domainAllowed(host) {
			t.Fatalf("expected host %q to be allowed", host)
		}
	}
	for _, host := range []string{"example.com", "cdn.example.com", "evilcdn.example.com", "api.example.com.evil.test"} {
		if client.domainAllowed(host) {
			t.Fatalf("host %q bypassed allowlist", host)
		}
	}
	if !client.portAllowed(443) || client.portAllowed(80) {
		t.Fatal("port allowlist mismatch")
	}
}

func TestRestrictedNetworkExactIPAllowlist(t *testing.T) {
	client, err := newRestrictedHTTPClient(trusted_service.ServiceNetworkPolicy{
		Mode: "restricted", Enforce: true, RequireProxy: true,
		AllowedIPs: []string{"127.0.0.1", "2001:db8::10"}, AllowedPorts: []int{18080},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !client.ipAllowed(netip.MustParseAddr("127.0.0.1")) || !client.ipAllowed(netip.MustParseAddr("2001:db8::10")) {
		t.Fatal("explicit IP allowlist entry was not recognized")
	}
	if client.ipAllowed(netip.MustParseAddr("127.0.0.2")) {
		t.Fatal("undeclared loopback IP bypassed exact allowlist")
	}
}

func TestRestrictedNetworkRejectsAmbientOutboundPolicy(t *testing.T) {
	_, err := newRestrictedHTTPClient(trusted_service.ServiceNetworkPolicy{Mode: "restricted", Enforce: true, RequireProxy: true, AllowOutbound: true, AllowedDomains: []string{"example.com"}, AllowedPorts: []int{443}})
	if err == nil {
		t.Fatal("restricted policy with ambient outbound was accepted")
	}
}

func TestPublicDestinationIPRejectsSSRFAddressClasses(t *testing.T) {
	for _, raw := range []string{
		"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.1.1", "100.64.0.1", "0.0.0.0",
		"192.0.2.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "240.0.0.1",
		"::1", "fe80::1", "fc00::1", "64:ff9b::a00:1", "64:ff9b:1::1", "100::1", "2001::1", "2001:db8::1", "2002:a00:1::1",
	} {
		if isPublicDestinationIP(net.ParseIP(raw)) {
			t.Fatalf("non-public address %s accepted", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !isPublicDestinationIP(net.ParseIP(raw)) {
			t.Fatalf("public address %s rejected", raw)
		}
	}
}

func TestRestrictedNetworkHeaderBoundary(t *testing.T) {
	for _, name := range []string{"Host", "Connection", "Proxy-Authorization", "Cookie", "Set-Cookie"} {
		if allowedForwardHeader(name) {
			t.Fatalf("forbidden header %s accepted", name)
		}
	}
	if !allowedForwardHeader("Authorization") || !allowedForwardHeader("Content-Type") {
		t.Fatal("ordinary request headers rejected")
	}
}

func TestRestrictedNetworkRequestAllowsOnlyExactDeclaredLoopbackEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		_, _ = w.Write([]byte("mediated-ok"))
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	client, err := newRestrictedHTTPClient(trusted_service.ServiceNetworkPolicy{
		Mode: "restricted", Enforce: true, RequireProxy: true,
		AllowedIPs: []string{"127.0.0.1"}, AllowedPorts: []int{port},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := client.Do(context.Background(), NetworkRequestInput{Method: http.MethodGet, URL: server.URL})
	if err != nil {
		t.Fatalf("allowlisted request failed: %v", err)
	}
	if out.StatusCode != http.StatusOK || out.Headers["X-Test"][0] != "ok" {
		t.Fatalf("unexpected response metadata: %+v", out)
	}
	body, err := base64.StdEncoding.DecodeString(out.BodyBase64)
	if err != nil || string(body) != "mediated-ok" {
		t.Fatalf("unexpected response body %q (err=%v)", string(body), err)
	}

	blocked := *parsed
	blocked.Host = net.JoinHostPort("127.0.0.2", parsed.Port())
	if _, err := client.Do(context.Background(), NetworkRequestInput{URL: blocked.String(), TimeoutMs: 500}); err == nil {
		t.Fatal("undeclared loopback IP was allowed")
	}
}

func TestRestrictedNetworkRedirectCannotEscapePortAllowlist(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("should-not-be-reached"))
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	sourceURL, err := url.Parse(source.URL)
	if err != nil {
		t.Fatal(err)
	}
	sourcePort, err := strconv.Atoi(sourceURL.Port())
	if err != nil {
		t.Fatal(err)
	}
	client, err := newRestrictedHTTPClient(trusted_service.ServiceNetworkPolicy{
		Mode: "restricted", Enforce: true, RequireProxy: true,
		AllowedIPs: []string{"127.0.0.1"}, AllowedPorts: []int{sourcePort},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(context.Background(), NetworkRequestInput{URL: source.URL}); err == nil {
		t.Fatal("redirect to an undeclared port escaped the restricted allowlist")
	}
}
