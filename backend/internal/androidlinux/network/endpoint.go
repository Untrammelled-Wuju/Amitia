package network

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type EndpointClass string

const (
	EndpointPublic    EndpointClass = "public"
	EndpointPrivate   EndpointClass = "private"
	EndpointLoopback  EndpointClass = "loopback"
	EndpointLinkLocal EndpointClass = "link-local"
	EndpointMulticast EndpointClass = "multicast"
	EndpointUnspecified EndpointClass = "unspecified"
)

type EndpointSecurity struct {
	URL       *url.URL
	Class     EndpointClass
	Host      string
	Addresses []net.IP
}

type Validator struct {
	resolver *net.Resolver
	policy   Policy
}

func NewEndpointValidator(policy Policy) *Resolver {
	return &Resolver{policy: policy}
}
type Resolver struct {
	policy Policy
}

func (r *Resolver) ResolveAndClassify(ctx context.Context, rawURL string) (EndpointSecurity, error) {
	parsed, err := r.parseURL(rawURL)
	if err != nil {
		return EndpointSecurity{}, err
	}

	host := strings.ToLower(parsed.Hostname())
	addresses, err := r.resolveAddresses(ctx, host)
	if err != nil {
		return EndpointSecurity{}, err
	}

	class := classifyAddresses(host, addresses)

	if class == EndpointLinkLocal && r.policy.DenyLinkLocal {
		return EndpointSecurity{}, ErrEndpointDenied("link-local addresses are forbidden")
	}
	if class == EndpointMulticast && r.policy.DenyMulticast {
		return EndpointSecurity{}, ErrEndpointDenied("multicast addresses are forbidden")
	}
	if class == EndpointUnspecified && r.policy.DenyUnspecified {
		return EndpointSecurity{}, ErrEndpointDenied("unspecified addresses are forbidden")
	}
	if class == EndpointPrivate && !r.policy.AllowPrivateNetwork {
		return EndpointSecurity{}, ErrEndpointDenied("private network access denied")
	}
	if class == EndpointLoopback && !r.policy.AllowLoopback {
		return EndpointSecurity{}, ErrEndpointDenied("loopback access denied")
	}
	if class == EndpointPublic && !r.policy.AllowPublicInternet {
		return EndpointSecurity{}, ErrEndpointDenied("public internet access denied")
	}

	if isMetadataEndpoint(addresses) {
		return EndpointSecurity{}, ErrEndpointDenied("cloud metadata endpoint is forbidden")
	}

	return EndpointSecurity{
		URL:       parsed,
		Class:     class,
		Host:      host,
		Addresses: addresses,
	}, nil
}

func (r *Resolver) parseURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, ErrInvalidURL(rawURL, "empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrInvalidURL(rawURL, err.Error())
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidURL(parsed.String(), "only http/https schemes are allowed")
	}

	if parsed.Host == "" {
		return nil, ErrInvalidURL(parsed.String(), "missing host")
	}

	if parsed.User != nil {
		return nil, ErrInvalidURL(parsed.String(), "credentials in URL are forbidden")
	}

	if strings.ContainsAny(parsed.Host, "\r\n") {
		return nil, ErrCRLFInjection("host contains CRLF characters")
	}

	if port := parsed.Port(); port != "" {
		value, perr := strconv.Atoi(port)
		if perr != nil || value < 1 || value > 65535 {
			return nil, ErrInvalidURL(parsed.String(), "invalid port")
		}
	}

	return parsed, nil
}

func (r *Resolver) resolveAddresses(ctx context.Context, host string) ([]net.IP, error) {
	ips := parseLiteralIPs(host)
	if ips != nil {
		return ips, nil
	}

	resolver := net.DefaultResolver
	found, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	ips = make([]net.IP, 0, len(found))
	for _, addr := range found {
		ips = append(ips, addr.IP)
	}

	if len(ips) == 0 {
		return nil, ErrEndpointDenied("no addresses resolved for host")
	}

	return ips, nil
}

func parseLiteralIPs(host string) []net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}
	}
	if host != "" && host[0] == '[' && host[len(host)-1] == '}' {
		ip := net.ParseIP(host[1 : len(host)-1])
		if ip != nil {
			return []net.IP{ip}
		}
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		ip := net.ParseIP(host[1 : len(host)-1])
		if ip != nil {
			return []net.IP{ip}
		}
	}
	return nil
}

func classifyAddresses(host string, addresses []net.IP) EndpointClass {
	var result EndpointClass
	for _, ip := range addresses {
		class := classifySingleIP(host, ip)
		if result == "" || classOrder(class) > classOrder(result) {
			result = class
		}
	}
	return result
}

func classifySingleIP(host string, ip net.IP) EndpointClass {
	if ip.IsUnspecified() {
		return EndpointUnspecified
	}
	if ip.IsMulticast() {
		return EndpointMulticast
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return EndpointLinkLocal
	}
	if ip.IsLoopback() || host == "localhost" {
		return EndpointLoopback
	}
	if isPrivateIP(ip) {
		return EndpointPrivate
	}
	return EndpointPublic
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsPrivate() {
		return true
	}
	ranges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}
	for _, cidr := range ranges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func isMetadataEndpoint(addresses []net.IP) bool {
	metadata := []string{
		"169.254.169.254",
		"100.100.100.200",
		"fd00:ec2::254",
	}
	for _, ip := range addresses {
		for _, m := range metadata {
			if ip.Equal(net.ParseIP(m)) {
				return true
			}
		}
	}
	return false
}

func classOrder(class EndpointClass) int {
	switch class {
	case EndpointUnspecified:
		return 6
	case EndpointMulticast:
		return 5
	case EndpointLinkLocal:
		return 4
	case EndpointLoopback:
		return 3
	case EndpointPrivate:
		return 2
	case EndpointPublic:
		return 1
	}
	return 0
}
