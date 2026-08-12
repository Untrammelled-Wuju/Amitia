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
	maxRedirects       int
	allowHostRedirects bool
}

type SecureTransport struct {
	policy endpointPolicy
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
			maxRedirects:       3,
			allowHostRedirects: false,
		},
	}
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
	if parsed.Scheme != "https" {
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
			if req.URL.Scheme != "https" {
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
	hostname := strings.ToLower(endpoint.url.Host)
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
			if req.URL.Scheme != "https" {
				return fmt.Errorf("search redirect scheme downgrade rejected")
			}
			return nil
		},
	}
}

func (t *SecureTransport) resolve(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
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

func (t *SecureTransport) deniedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("100.100.100.200")) {
		return true
	}
	return false
}

