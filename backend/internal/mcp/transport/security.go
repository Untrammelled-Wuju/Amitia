package transport

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

type EndpointClass string

const (
	EndpointRemoteHTTPS  EndpointClass = "remote_https"
	EndpointLoopbackHTTP EndpointClass = "loopback_http"
	EndpointPrivateHTTP  EndpointClass = "private_network_http"
	EndpointPublicHTTP   EndpointClass = "public_http"
)

type EndpointPolicy struct {
	AllowLoopback      bool
	AllowPrivate       bool
	AllowPublicHTTP    bool
	AllowHostRedirects bool
	MaxRedirects       int
	Resolver           *net.Resolver
}

type EndpointSecurity struct {
	URL       *url.URL
	Class     EndpointClass
	Addresses []net.IP
}

func ValidateEndpoint(ctx context.Context, rawURL string, policy EndpointPolicy) (EndpointSecurity, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: invalid endpoint")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: unsupported endpoint scheme")
	}
	if parsed.User != nil {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: endpoint credentials are forbidden")
	}
	if strings.ContainsAny(parsed.Host, "\r\n") {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: invalid endpoint host")
	}
	if port := parsed.Port(); port != "" {
		value, portErr := strconv.Atoi(port)
		if portErr != nil || value < 1 || value > 65535 {
			return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: invalid endpoint port")
		}
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	addresses, err := resolveEndpoint(ctx, host, policy.Resolver)
	if err != nil {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: resolve endpoint: %w", err)
	}
	class, err := classifyEndpoint(parsed.Scheme, host, addresses)
	if err != nil {
		return EndpointSecurity{}, err
	}
	if class == EndpointLoopbackHTTP && !policy.AllowLoopback {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: loopback endpoint requires confirmation")
	}
	if class == EndpointPrivateHTTP && !policy.AllowPrivate {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: private network endpoint requires confirmation")
	}
	if class == EndpointPublicHTTP && !policy.AllowPublicHTTP {
		return EndpointSecurity{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: public HTTP endpoint is forbidden")
	}
	return EndpointSecurity{URL: parsed, Class: class, Addresses: addresses}, nil
}

func NewSecureHTTPClient(security EndpointSecurity, policy EndpointPolicy, timeout time.Duration) *http.Client {
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
			return nil, fmt.Errorf("MCP transport host changed")
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
				return fmt.Errorf("MCP redirect limit exceeded")
			}
			if !policy.AllowHostRedirects && !strings.EqualFold(request.URL.Host, security.URL.Host) {
				return fmt.Errorf("MCP cross-host redirect rejected")
			}
			if request.URL.Scheme != security.URL.Scheme {
				return fmt.Errorf("MCP redirect security level changed")
			}
			return nil
		},
	}
}

func resolveEndpoint(ctx context.Context, host string, resolver *net.Resolver) ([]net.IP, error) {
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

func classifyEndpoint(scheme, host string, addresses []net.IP) (EndpointClass, error) {
	var classification EndpointClass
	for _, ip := range addresses {
		current := EndpointPublicHTTP
		if ip.IsLoopback() || host == "localhost" {
			current = EndpointLoopbackHTTP
		} else if isPrivateNetworkIP(ip) {
			current = EndpointPrivateHTTP
		}
		if prohibitedMetadataIP(ip) {
			return "", fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: metadata endpoint is forbidden")
		}
		if classification != "" && classification != current {
			return "", fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: mixed DNS security classes")
		}
		classification = current
	}
	if classification == EndpointPublicHTTP && scheme == "https" {
		return EndpointRemoteHTTPS, nil
	}
	return classification, nil
}

func isPrivateNetworkIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func prohibitedMetadataIP(ip net.IP) bool {
	return ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("100.100.100.200"))
}
