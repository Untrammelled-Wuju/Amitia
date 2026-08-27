package hostapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

var errNetworkPolicyDenied = errors.New("host-mediated network policy denied")

func networkPolicyDenied(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", errNetworkPolicyDenied, err)
}

const (
	defaultNetworkResponseLimit int64 = 4 << 20
	maxNetworkResponseLimit     int64 = 16 << 20
	maxNetworkRequestBody             = 4 << 20
	maxNetworkRedirects               = 5
)

type NetworkRequestInput struct {
	Method           string            `json:"method"`
	URL              string            `json:"url"`
	Headers          map[string]string `json:"headers,omitempty"`
	BodyBase64       string            `json:"bodyBase64,omitempty"`
	TimeoutMs        int               `json:"timeoutMs,omitempty"`
	MaxResponseBytes int64             `json:"maxResponseBytes,omitempty"`
}

type NetworkRequestOutput struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"bodyBase64"`
	FinalURL   string              `json:"finalUrl"`
}

type restrictedHTTPClient struct {
	policy   trusted_service.ServiceNetworkPolicy
	resolver *net.Resolver
}

func newRestrictedHTTPClient(policy trusted_service.ServiceNetworkPolicy) (*restrictedHTTPClient, error) {
	if strings.ToLower(strings.TrimSpace(policy.Mode)) != "restricted" || !policy.Enforce || !policy.RequireProxy || policy.AllowOutbound || policy.AllowInbound || policy.LoopbackOnly {
		return nil, fmt.Errorf("restricted host networking requires an enforced host-mediated policy")
	}
	if err := trusted_service.ValidateNetworkPolicySupport(policy); err != nil {
		return nil, fmt.Errorf("restricted network policy is invalid: %w", err)
	}
	return &restrictedHTTPClient{policy: policy, resolver: net.DefaultResolver}, nil
}

func (c *restrictedHTTPClient) Do(ctx context.Context, in NetworkRequestInput) (NetworkRequestOutput, error) {
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
	default:
		return NetworkRequestOutput{}, fmt.Errorf("unsupported HTTP method %q", method)
	}
	if len(strings.TrimSpace(in.URL)) == 0 || len(in.URL) > 8192 {
		return NetworkRequestOutput{}, fmt.Errorf("request URL must contain 1..8192 bytes")
	}
	parsed, err := url.Parse(strings.TrimSpace(in.URL))
	if err != nil || parsed == nil || parsed.Host == "" {
		return NetworkRequestOutput{}, fmt.Errorf("invalid request URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return NetworkRequestOutput{}, fmt.Errorf("URL userinfo and fragments are not allowed")
	}
	if err := c.validateURL(ctx, parsed); err != nil {
		return NetworkRequestOutput{}, err
	}

	var body []byte
	if in.BodyBase64 != "" {
		body, err = base64.StdEncoding.DecodeString(in.BodyBase64)
		if err != nil {
			return NetworkRequestOutput{}, fmt.Errorf("bodyBase64 is invalid")
		}
		if len(body) > maxNetworkRequestBody {
			return NetworkRequestOutput{}, fmt.Errorf("request body exceeds %d bytes", maxNetworkRequestBody)
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, parsed.String(), bytes.NewReader(body))
	if err != nil {
		return NetworkRequestOutput{}, err
	}
	if len(in.Headers) > 64 {
		return NetworkRequestOutput{}, fmt.Errorf("too many request headers")
	}
	for name, value := range in.Headers {
		canonical := http.CanonicalHeaderKey(strings.TrimSpace(name))
		if len(canonical) > 128 || len(value) > 8192 || !allowedForwardHeader(canonical) {
			return NetworkRequestOutput{}, fmt.Errorf("header %q is not allowed", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return NetworkRequestOutput{}, fmt.Errorf("header %q contains a line break", name)
		}
		request.Header.Set(canonical, value)
	}

	responseLimit := in.MaxResponseBytes
	if responseLimit == 0 {
		responseLimit = defaultNetworkResponseLimit
	}
	if responseLimit < 1 || responseLimit > maxNetworkResponseLimit {
		return NetworkRequestOutput{}, fmt.Errorf("maxResponseBytes must be between 1 and %d", maxNetworkResponseLimit)
	}
	timeout := 30 * time.Second
	if in.TimeoutMs > 0 {
		if in.TimeoutMs < 100 || in.TimeoutMs > 120000 {
			return NetworkRequestOutput{}, fmt.Errorf("timeoutMs must be between 100 and 120000")
		}
		timeout = time.Duration(in.TimeoutMs) * time.Millisecond
	}

	transport := &http.Transport{
		Proxy:                  nil,
		DisableCompression:     false,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           8,
		MaxIdleConnsPerHost:    2,
		IdleConnTimeout:        30 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  timeout,
		MaxResponseHeaderBytes: 1 << 20,
	}
	transport.DialContext = c.dialContext
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxNetworkRedirects {
				return fmt.Errorf("too many redirects")
			}
			return c.validateURL(req.Context(), req.URL)
		},
	}
	defer transport.CloseIdleConnections()

	resp, err := client.Do(request)
	if err != nil {
		return NetworkRequestOutput{}, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, responseLimit+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return NetworkRequestOutput{}, err
	}
	if int64(len(responseBody)) > responseLimit {
		return NetworkRequestOutput{}, fmt.Errorf("response exceeds maxResponseBytes %d", responseLimit)
	}
	return NetworkRequestOutput{
		StatusCode: resp.StatusCode,
		Headers:    sanitizeResponseHeaders(resp.Header),
		BodyBase64: base64.StdEncoding.EncodeToString(responseBody),
		FinalURL:   resp.Request.URL.String(),
	}, nil
}

func (c *restrictedHTTPClient) validateURL(ctx context.Context, target *url.URL) error {
	scheme := strings.ToLower(target.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only http and https are allowed")
	}
	if !c.transportAllowed(scheme) {
		return fmt.Errorf("network transport %q is not allowed", scheme)
	}
	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	if host == "" {
		return fmt.Errorf("destination host is required")
	}
	port := target.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || !c.portAllowed(portNumber) {
		return fmt.Errorf("destination port %q is not allowed", port)
	}
	if _, err := c.resolveAllowedAddresses(ctx, host); err != nil {
		return err
	}
	return nil
}

func (c *restrictedHTTPClient) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || !c.portAllowed(portNumber) {
		return nil, fmt.Errorf("destination port %q is not allowed", port)
	}
	addresses, err := c.resolveAllowedAddresses(ctx, strings.ToLower(strings.TrimSuffix(host, ".")))
	if err != nil {
		return nil, err
	}
	var lastErr error
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	for _, item := range addresses {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(item.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no approved destination address")
	}
	return nil, lastErr
}

// resolveAllowedAddresses is deliberately called both before request creation
// and again at the actual DialContext boundary. This closes the DNS-rebinding
// gap: a hostname may change between validation and connect, but every address
// used for the socket must independently satisfy the current policy.
func (c *restrictedHTTPClient) resolveAllowedAddresses(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "host-loopback" {
		if !c.policy.AllowHostLoopback {
			return nil, fmt.Errorf("host-loopback destination is not allowed")
		}
		return []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}, nil
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		literal = literal.Unmap()
		if literal.Zone() != "" || !c.ipAllowed(literal) {
			return nil, fmt.Errorf("destination IP %q is not explicitly allowed", host)
		}
		return []netip.Addr{literal}, nil
	}
	if !c.domainAllowed(host) {
		return nil, fmt.Errorf("destination host %q is not allowed", host)
	}
	resolved, err := c.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve destination %q: %w", host, err)
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("resolve destination %q: no addresses", host)
	}
	addresses := make([]netip.Addr, 0, len(resolved))
	seen := make(map[netip.Addr]struct{}, len(resolved))
	for _, item := range resolved {
		addr, ok := netip.AddrFromSlice(item.IP)
		if !ok {
			return nil, fmt.Errorf("destination %q resolved to an invalid address", host)
		}
		addr = addr.Unmap()
		if !isPublicDestinationAddr(addr) && !c.ipAllowed(addr) {
			return nil, fmt.Errorf("destination %q resolves to non-public IP %s that is not explicitly allowed", host, addr)
		}
		if _, exists := seen[addr]; exists {
			continue
		}
		seen[addr] = struct{}{}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

func (c *restrictedHTTPClient) domainAllowed(host string) bool {
	for _, raw := range c.policy.AllowedDomains {
		pattern := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		if strings.HasPrefix(pattern, "*.") {
			base := strings.TrimPrefix(pattern, "*.")
			if host != base && strings.HasSuffix(host, "."+base) {
				return true
			}
			continue
		}
		if host == pattern {
			return true
		}
	}
	return false
}

func (c *restrictedHTTPClient) ipAllowed(addr netip.Addr) bool {
	if !addr.IsValid() || addr.Zone() != "" {
		return false
	}
	addr = addr.Unmap()
	for _, raw := range c.policy.AllowedIPs {
		allowed, err := netip.ParseAddr(strings.TrimSpace(raw))
		if err != nil || allowed.Zone() != "" {
			continue
		}
		if addr == allowed.Unmap() {
			return true
		}
	}
	return false
}

func (c *restrictedHTTPClient) portAllowed(port int) bool {
	for _, allowed := range c.policy.AllowedPorts {
		if port == allowed {
			return true
		}
	}
	return false
}

func (c *restrictedHTTPClient) transportAllowed(transport string) bool {
	transport = strings.ToLower(strings.TrimSpace(transport))
	if len(c.policy.AllowedTransports) == 0 {
		// Compatibility for protocol-v1 restricted manifests created before
		// mediated raw transports existed. They remain HTTP/HTTPS-only.
		return transport == "http" || transport == "https"
	}
	for _, raw := range c.policy.AllowedTransports {
		if strings.ToLower(strings.TrimSpace(raw)) == transport {
			return true
		}
	}
	return false
}

func (c *restrictedHTTPClient) maxConnections() int {
	if c.policy.MaxConnections > 0 {
		return c.policy.MaxConnections
	}
	return 16
}

func (c *restrictedHTTPClient) resolveSocketAddresses(ctx context.Context, transport, target string, port int) ([]netip.Addr, error) {
	transport = strings.ToLower(strings.TrimSpace(transport))
	if !c.transportAllowed(transport) {
		return nil, fmt.Errorf("network transport %q is not allowed", transport)
	}
	if port < 1 || port > 65535 || !c.portAllowed(port) {
		return nil, fmt.Errorf("destination port %d is not allowed", port)
	}
	target = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(target), "."))
	if target == "" {
		return nil, fmt.Errorf("destination target is required")
	}
	return c.resolveAllowedAddresses(ctx, target)
}

func (c *restrictedHTTPClient) dialApproved(ctx context.Context, transport, network, target string, port int, timeout time.Duration) (net.Conn, error) {
	addresses, err := c.resolveSocketAddresses(ctx, transport, target, port)
	if err != nil {
		return nil, networkPolicyDenied(err)
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, addr := range addresses {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), strconv.Itoa(port)))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no approved destination address")
	}
	return nil, lastErr
}

func isPublicDestinationIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	return ok && isPublicDestinationAddr(addr)
}

func isPublicDestinationAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.Zone() != "" || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, prefix := range nonPublicNetworkPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var nonPublicNetworkPrefixes = mustNetworkPrefixes(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
	// IPv6 special-use ranges that can otherwise tunnel or synthesize access to
	// non-public IPv4 destinations (NAT64/6to4) or are not ordinary public
	// Internet endpoints. Exact manifest IP entries may still opt into them.
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"100::/64",
	"2001::/23",
	"2001:db8::/32",
	"2002::/16",
)

func mustNetworkPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

func allowedForwardHeader(name string) bool {
	switch strings.ToLower(name) {
	case "host", "connection", "proxy-connection", "proxy-authorization", "proxy-authenticate", "te", "trailer", "transfer-encoding", "upgrade", "cookie", "set-cookie":
		return false
	default:
		return name != ""
	}
}

func sanitizeResponseHeaders(headers http.Header) map[string][]string {
	out := make(map[string][]string, len(headers))
	for name, values := range headers {
		switch strings.ToLower(name) {
		case "set-cookie", "proxy-authenticate", "proxy-authorization":
			continue
		}
		out[name] = append([]string(nil), values...)
	}
	return out
}
