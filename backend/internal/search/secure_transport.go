package search

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type endpointPolicy struct {
	allowLoopback      bool
	allowPrivate       bool
	allowHTTP          bool
	maxRedirects       int
	allowHostRedirects bool
}

type SecureTransport struct {
	policy   endpointPolicy
	resolver func(context.Context, string) ([]net.IP, error)
}

type validatedEndpoint struct {
	url       *url.URL
	addresses []net.IP
	public    bool
}

func NewSecureTransport() *SecureTransport {
	return &SecureTransport{
		policy: endpointPolicy{
			allowLoopback:      false,
			allowPrivate:       false,
			allowHTTP:          false,
			maxRedirects:       3,
			allowHostRedirects: false,
		},
	}
}

func NewConfiguredTransport(allowHTTP, allowPrivate, allowHostRedirects bool) *SecureTransport {
	return &SecureTransport{policy: endpointPolicy{
		allowLoopback:      allowPrivate,
		allowPrivate:       allowPrivate,
		allowHTTP:          allowHTTP,
		maxRedirects:       3,
		allowHostRedirects: allowHostRedirects,
	}}
}

func (t *SecureTransport) ValidateEndpoint(ctx context.Context, rawURL string) (*validatedEndpoint, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
	}
	if parsed.Scheme != "https" && !(t.policy.allowHTTP && parsed.Scheme == "http") {
		return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
	}
	if parsed.User != nil {
		return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
	}
	if strings.ContainsAny(parsed.Host, "\r\n") {
		return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
	}
	if port := parsed.Port(); port != "" {
		v, perr := strconv.Atoi(port)
		if perr != nil || v < 1 || v > 65535 {
			return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
		}
	}
	host := parsed.Hostname()
	addresses, err := t.resolve(ctx, host)
	if err != nil {
		return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
	}
	for _, ip := range addresses {
		if t.deniedIP(ip) {
			return nil, &Error{Code: SEARCH_BLOCKED_BY_NETWORK}
		}
	}
	return &validatedEndpoint{
		url:       parsed,
		addresses: addresses,
		public:    true,
	}, nil
}

func (t *SecureTransport) NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		return d.DialContext(ctx, network, address)
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= t.policy.maxRedirects {
				return fmt.Errorf("search redirect limit exceeded")
			}
			if !t.policy.allowHostRedirects && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("search cross-host redirect rejected")
			}
			if req.URL.Scheme != "https" && !(t.policy.allowHTTP && req.URL.Scheme == "http") {
				return fmt.Errorf("search redirect scheme downgrade rejected")
			}
			return nil
		},
	}
}

func (t *SecureTransport) PinHTTPClient(endpoint *validatedEndpoint, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	approved := append([]net.IP(nil), endpoint.addresses...)
	hostname := strings.ToLower(endpoint.url.Hostname())
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if strings.ToLower(strings.TrimSuffix(host, ".")) != strings.TrimSuffix(hostname, ".") {
			return nil, fmt.Errorf("search transport host changed")
		}
		var lastErr error
		for _, ip := range approved {
			conn, derr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if derr == nil {
				return conn, nil
			}
			lastErr = derr
		}
		return nil, lastErr
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= t.policy.maxRedirects {
				return fmt.Errorf("search redirect limit exceeded")
			}
			if !t.policy.allowHostRedirects && !strings.EqualFold(req.URL.Host, endpoint.url.Host) {
				return fmt.Errorf("search cross-host redirect rejected")
			}
			if req.URL.Scheme != "https" && !(t.policy.allowHTTP && req.URL.Scheme == "http") {
				return fmt.Errorf("search redirect scheme downgrade rejected")
			}
			return nil
		},
	}
}

func (t *SecureTransport) resolve(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" || strings.Contains(host, "%") {
		return nil, fmt.Errorf("invalid host")
	}
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	if isObfuscatedNumericHost(host) {
		return nil, fmt.Errorf("obfuscated numeric IP host is not allowed")
	}
	if t != nil && t.resolver != nil {
		ips, err := t.resolver(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses for host")
		}
		return ips, nil
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses for host")
	}
	return ips, nil
}

func isObfuscatedNumericHost(host string) bool {
	lower := strings.ToLower(strings.TrimSpace(host))
	if lower == "" {
		return false
	}
	if strings.HasPrefix(lower, "0x") {
		return true
	}
	allDigits := true
	for _, r := range lower {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}
	parts := strings.Split(lower, ".")
	if len(parts) > 1 {
		suspicious := true
		for _, part := range parts {
			if part == "" {
				return true
			}
			if strings.HasPrefix(part, "0x") || (len(part) > 1 && part[0] == '0') {
				continue
			}
			for _, r := range part {
				if r < '0' || r > '9' {
					suspicious = false
					break
				}
			}
		}
		return suspicious
	}
	return false
}

func (t *SecureTransport) deniedIP(ip net.IP) bool {
	if ip.IsLoopback() && !t.policy.allowLoopback {
		return true
	}
	if ip.IsPrivate() && !t.policy.allowPrivate {
		return true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("100.100.100.200")) {
		return true
	}
	if isReservedNetworkIP(ip) {
		return true
	}
	return false
}

func isReservedNetworkIP(ip net.IP) bool {
	for _, cidr := range []string{
		"100.64.0.0/10",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"240.0.0.0/4",
		"2001:db8::/32",
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
