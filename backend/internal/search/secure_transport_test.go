package search

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestValidateEndpoint_EmptyURL(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "")
	if err == nil {
		t.Fatal("empty URL should be rejected")
	}
	sErr, ok := err.(*Error)
	if !ok || sErr.Code != SEARCH_BLOCKED_BY_NETWORK {
		t.Fatalf("expected SEARCH_BLOCKED_BY_NETWORK, got %v", err)
	}
}

func TestValidateEndpoint_WhitespaceURL(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "   ")
	if err == nil {
		t.Fatal("whitespace URL should be rejected")
	}
}

func TestValidateEndpoint_RejectHTTP(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "http://example.com")
	if err == nil {
		t.Fatal("http should be rejected")
	}
}

func TestValidateEndpoint_RejectNoHost(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "https:///path")
	if err == nil {
		t.Fatal("no host should be rejected")
	}
}

func TestValidateEndpoint_RejectUserinfo(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "https://user:pass@example.com/")
	if err == nil {
		t.Fatal("userinfo should be rejected")
	}
}

func TestValidateEndpoint_RejectCRLF(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "https://evil.com/\r\nInject: header")
	if err == nil {
		t.Fatal("CRLF in host should be rejected")
	}
}

func TestValidateEndpoint_RejectBadPort(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "https://example.com:99999/")
	if err == nil {
		t.Fatal("invalid port should be rejected")
	}
}

func TestValidateEndpoint_RejectLoopback(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "https://127.0.0.1/")
	if err == nil {
		t.Fatal("loopback should be rejected")
	}
}

func TestValidateEndpoint_RejectLinkLocal(t *testing.T) {
	st := NewSecureTransport()
	_, err := st.ValidateEndpoint(context.Background(), "https://169.254.1.1/")
	if err == nil {
		t.Fatal("link-local should be rejected")
	}
}

func TestValidateEndpoint_PublicHost(t *testing.T) {
	st := NewSecureTransport()
	st.resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("1.1.1.1")}, nil
	}
	ep, err := st.ValidateEndpoint(context.Background(), "https://api.search.brave.com/res/v1/web/search")
	if err != nil {
		t.Fatalf("public host should validate: %v", err)
	}
	if ep == nil || !ep.public {
		t.Fatal("expected public endpoint")
	}
}

func TestDenyIP_Loopback(t *testing.T) {
	st := NewSecureTransport()
	if !st.deniedIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("127.0.0.1 should be denied")
	}
	if !st.deniedIP(net.ParseIP("::1")) {
		t.Fatal("::1 should be denied")
	}
}

func TestDenyIP_Private(t *testing.T) {
	st := NewSecureTransport()
	if !st.deniedIP(net.ParseIP("10.0.0.1")) {
		t.Fatal("10.0.0.1 should be denied")
	}
	if !st.deniedIP(net.ParseIP("192.168.1.1")) {
		t.Fatal("192.168.1.1 should be denied")
	}
	if !st.deniedIP(net.ParseIP("172.16.0.1")) {
		t.Fatal("172.16.0.1 should be denied")
	}
}

func TestDenyIP_Metadata(t *testing.T) {
	st := NewSecureTransport()
	if !st.deniedIP(net.ParseIP("169.254.169.254")) {
		t.Fatal("AWS metadata IP should be denied")
	}
}

func TestDenyIP_Public(t *testing.T) {
	st := NewSecureTransport()
	if st.deniedIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP should be allowed")
	}
	if st.deniedIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("public IP should be allowed")
	}
}

func TestNewHTTPClient_DefaultTimeout(t *testing.T) {
	st := NewSecureTransport()
	c := st.NewHTTPClient(0)
	if c.Timeout == 0 {
		t.Fatal("default timeout should be set")
	}
}

func TestPinHTTPClient_HostPinning(t *testing.T) {
	st := NewSecureTransport()
	u, _ := url.Parse("https://api.example.com")
	ep := &validatedEndpoint{
		url:       u,
		addresses: []net.IP{net.ParseIP("93.184.216.34")},
	}
	client := st.PinHTTPClient(ep, 0)
	if client.Timeout == 0 {
		t.Fatal("timeout should be set")
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("transport not *http.Transport")
	}
	if tr.DialContext == nil {
		t.Fatal("pin dialer missing")
	}
}

func TestSecureTransport_ResolveDirectIP(t *testing.T) {
	st := NewSecureTransport()
	ips, err := st.resolve(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("direct IP resolve failed: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("1.1.1.1")) {
		t.Fatalf("wrong IP: %v", ips)
	}
}

func TestSecureTransportRejectsObfuscatedNumericIPHosts(t *testing.T) {
	transport := NewConfiguredTransport(true, false, false)
	for _, rawURL := range []string{
		"http://2130706433/",
		"http://0x7f000001/",
		"http://0177.0.0.1/",
		"http://[fe80::1%25eth0]/",
	} {
		if _, err := transport.ValidateEndpoint(context.Background(), rawURL); err == nil {
			t.Fatalf("expected URL to be rejected: %s", rawURL)
		}
	}
}

func TestValidateEndpointRejectsMixedPublicPrivateDNSAnswers(t *testing.T) {
	st := NewSecureTransport()
	st.resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("10.0.0.2")}, nil
	}
	if _, err := st.ValidateEndpoint(context.Background(), "https://mixed.example/"); err == nil {
		t.Fatal("mixed public/private DNS answer must be rejected")
	}
}

func TestDenyIPIPv4MappedIPv6AndIPv6PrivateRanges(t *testing.T) {
	st := NewSecureTransport()
	for _, raw := range []string{"::ffff:127.0.0.1", "fc00::1", "fe80::1", "::"} {
		if !st.deniedIP(net.ParseIP(raw)) {
			t.Fatalf("%s should be denied", raw)
		}
	}
}

func TestPinHTTPClientRejectsDialHostMutation(t *testing.T) {
	st := NewSecureTransport()
	u, _ := url.Parse("https://example.com")
	client := st.PinHTTPClient(&validatedEndpoint{url: u, addresses: []net.IP{net.ParseIP("1.1.1.1")}}, time.Second)
	transport := client.Transport.(*http.Transport)
	if _, err := transport.DialContext(context.Background(), "tcp", "other.example:443"); err == nil {
		t.Fatal("pinned transport must reject host mutation before dialing")
	}
}
