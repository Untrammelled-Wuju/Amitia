package browser

import (
	"net"
	"net/url"
	"strings"
	"time"
)

type NavigationSecurityClass string

const (
	NavClassPublic   NavigationSecurityClass = "public"
	NavClassPrivate  NavigationSecurityClass = "private"
	NavClassLoopback NavigationSecurityClass = "loopback"
	NavClassMetadata NavigationSecurityClass = "metadata"
	NavClassLinkLocal NavigationSecurityClass = "link_local"
	NavClassUnknown  NavigationSecurityClass = "unknown"
)

type NavigationPolicy struct {
	AllowedSchemes   map[string]struct{}
	AllowPublic      bool
	AllowPrivate     bool
	AllowLoopback    bool
	MaxURLBytes      int
	DefaultTimeout   time.Duration
	MaxTimeout       time.Duration
}

func NewNavigationPolicy(config BrowserConfig) *NavigationPolicy {
	schemes := make(map[string]struct{})
	for _, s := range config.AllowedSchemes {
		schemes[strings.ToLower(s)] = struct{}{}
	}

	return &NavigationPolicy{
		AllowedSchemes: schemes,
		AllowPublic:    true,
		AllowPrivate:   false,
		AllowLoopback:  false,
		MaxURLBytes:    16 * 1024,
		DefaultTimeout: config.NavigationTimeout,
		MaxTimeout:     config.MaxNavigationTimeout,
	}
}

func (p *NavigationPolicy) ValidateURL(rawURL string) (NavigationSecurityClass, *BrowserError) {
	if len(rawURL) > p.MaxURLBytes {
		return NavClassUnknown, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "url exceeds maximum allowed length",
		}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return NavClassUnknown, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "invalid url format",
			Cause:   err,
		}
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "" {
		return NavClassUnknown, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "url must have a scheme",
		}
	}

	if !p.isSchemeAllowed(scheme) {
		return NavClassUnknown, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "url scheme is not allowed: " + scheme,
		}
	}

	if parsed.User != nil {
		return NavClassUnknown, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "url must not contain userinfo",
		}
	}

	host := parsed.Hostname()
	if host == "" {
		return NavClassUnknown, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "url must have a host",
		}
	}

	host = strings.ToLower(host)
	if strings.HasPrefix(host, "chrome") || strings.HasPrefix(host, "devtools") || strings.HasPrefix(host, "extension") {
		if _, ok := p.AllowedSchemes[host]; !ok {
			return NavClassUnknown, &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "chrome/devtools urls are not allowed",
			}
		}
	}

	if host == "169.254.169.254" || host == "100.100.100.200" || host == "metadata.google.internal" {
		return NavClassMetadata, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "metadata endpoints are blocked",
		}
	}

	return p.classifyHost(host), nil
}

func (p *NavigationPolicy) classifyHost(host string) NavigationSecurityClass {
	if strings.HasPrefix(host, "127.") || host == "localhost" || host == "::1" || host == "0:0:0:0:0:0:0:1" {
		return NavClassLoopback
	}

	if strings.HasPrefix(host, "169.254.") || strings.HasPrefix(host, "fe80:") {
		return NavClassLinkLocal
	}

	ips := net.ParseIP(host)
	if ips != nil {
		if ips.IsLoopback() {
			return NavClassLoopback
		}
		if ips.IsLinkLocalUnicast() || ips.IsLinkLocalMulticast() {
			return NavClassLinkLocal
		}
		if ips.IsPrivate() {
			return NavClassPrivate
		}
		if ips.IsUnspecified() {
			return NavClassUnknown
		}
		return NavClassPublic
	}

	return NavClassPublic
}

func (p *NavigationPolicy) isSchemeAllowed(scheme string) bool {
	if _, ok := p.AllowedSchemes[scheme]; ok {
		return true
	}
	return false
}

func (p *NavigationPolicy) CheckPermission(class NavigationSecurityClass) *BrowserError {
	switch class {
	case NavClassPublic:
		if !p.AllowPublic {
			return &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "public navigation is not permitted",
			}
		}
	case NavClassPrivate:
		if !p.AllowPrivate {
			return &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "private network navigation is not permitted",
			}
		}
	case NavClassLoopback:
		if !p.AllowLoopback {
			return &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "loopback navigation is not permitted",
			}
		}
	case NavClassMetadata:
		return &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "metadata endpoints are blocked",
		}
	case NavClassLinkLocal:
		return &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "link-local addresses are blocked",
		}
	default:
		return &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "navigation to this address class is not permitted",
		}
	}
	return nil
}

func (p *NavigationPolicy) ResolveTimeout(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		return p.DefaultTimeout
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout > p.MaxTimeout {
		return p.MaxTimeout
	}
	return timeout
}

func (p *NavigationPolicy) CanNavigate(rawURL string) (NavigationSecurityClass, *BrowserError) {
	class, err := p.ValidateURL(rawURL)
	if err != nil {
		return NavClassUnknown, err
	}
	if class == NavClassUnknown {
		return class, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "could not determine security class",
		}
	}
	return class, p.CheckPermission(class)
}
