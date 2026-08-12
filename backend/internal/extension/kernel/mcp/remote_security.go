// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

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

type RemoteEndpointClass string

const (
	RemoteEndpointRemoteHTTPS  RemoteEndpointClass = "remote_https"
	RemoteEndpointLoopbackHTTP RemoteEndpointClass = "loopback_http"
	RemoteEndpointPrivateHTTP  RemoteEndpointClass = "private_network_http"
	RemoteEndpointPublicHTTP   RemoteEndpointClass = "public_http"
)

type RemoteEndpointPolicy struct {
	AllowLoopback      bool
	AllowPrivate       bool
	AllowPublicHTTP    bool
	AllowHostRedirects bool
	MaxRedirects       int
	Resolver           *net.Resolver
}

type RemoteEndpointSecurity struct {
	URL       *url.URL
	Class     RemoteEndpointClass
	Addresses []net.IP
}

func ValidateRemoteEndpoint(ctx context.Context, rawURL string, policy RemoteEndpointPolicy) (RemoteEndpointSecurity, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_INVALID_ENDPOINT: invalid endpoint")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_INVALID_ENDPOINT: unsupported endpoint scheme")
	}
	if parsed.User != nil {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_INVALID_ENDPOINT: endpoint credentials are forbidden")
	}
	if strings.ContainsAny(parsed.Host, "\r\n") {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_INVALID_ENDPOINT: invalid endpoint host")
	}
	if port := parsed.Port(); port != "" {
		value, portErr := strconv.Atoi(port)
		if portErr != nil || value < 1 || value > 65535 {
			return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_INVALID_ENDPOINT: invalid endpoint port")
		}
	}

	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	addresses, err := resolveRemoteEndpoint(ctx, host, policy.Resolver)
	if err != nil {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_DNS_FAILED: %w", err)
	}

	class, err := classifyRemoteEndpoint(parsed.Scheme, host, addresses)
	if err != nil {
		return RemoteEndpointSecurity{}, err
	}

	if class == RemoteEndpointLoopbackHTTP && !policy.AllowLoopback {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: loopback endpoint requires confirmation")
	}
	if class == RemoteEndpointPrivateHTTP && !policy.AllowPrivate {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: private network endpoint requires confirmation")
	}
	if class == RemoteEndpointPublicHTTP && !policy.AllowPublicHTTP {
		return RemoteEndpointSecurity{}, fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: public HTTP endpoint is forbidden")
	}

	return RemoteEndpointSecurity{URL: parsed, Class: class, Addresses: addresses}, nil
}

func NewRemoteSecureHTTPClient(security RemoteEndpointSecurity, policy RemoteEndpointPolicy, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxRedirects := policy.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 3
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	approved := append([]net.IP(nil), security.Addresses...)
	hostname := strings.ToLower(security.URL.Hostname())

	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if strings.ToLower(strings.TrimSuffix(host, ".")) != strings.TrimSuffix(hostname, ".") {
			return nil, fmt.Errorf("MCP remote transport host changed")
		}
		var lastErr error
		for _, ip := range approved {
			connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return connection, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: redirect limit exceeded")
			}
			if !policy.AllowHostRedirects && !strings.EqualFold(request.URL.Host, security.URL.Host) {
				return fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: cross-host redirect rejected")
			}
			if request.URL.Scheme != security.URL.Scheme {
				return fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: redirect security level changed")
			}
			return nil
		},
	}
}

func resolveRemoteEndpoint(ctx context.Context, host string, resolver *net.Resolver) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	values, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("endpoint has no addresses")
	}
	return values, nil
}

func classifyRemoteEndpoint(scheme, host string, addresses []net.IP) (RemoteEndpointClass, error) {
	var classification RemoteEndpointClass
	for _, ip := range addresses {
		current := RemoteEndpointPublicHTTP
		if ip.IsLoopback() || host == "localhost" {
			current = RemoteEndpointLoopbackHTTP
		} else if isRemotePrivateNetworkIP(ip) {
			current = RemoteEndpointPrivateHTTP
		}
		if prohibitedRemoteMetadataIP(ip) {
			return "", fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: metadata endpoint is forbidden")
		}
		if classification != "" && classification != current {
			return "", fmt.Errorf("MCP_REMOTE_ENDPOINT_FORBIDDEN: mixed DNS security classes")
		}
		classification = current
	}
	if classification == RemoteEndpointPublicHTTP && scheme == "https" {
		return RemoteEndpointRemoteHTTPS, nil
	}
	return classification, nil
}

func isRemotePrivateNetworkIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func prohibitedRemoteMetadataIP(ip net.IP) bool {
	return ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("100.100.100.200"))
}
