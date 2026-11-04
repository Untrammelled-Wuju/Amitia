package kernel

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

func privateNetworkGuardedHTTPClient(base *http.Client) (*http.Client, error) {
	if base == nil {
		return nil, fmt.Errorf("http client is required")
	}
	client := *base
	var transport *http.Transport
	switch current := base.Transport.(type) {
	case nil:
		transport = http.DefaultTransport.(*http.Transport).Clone()
	case *http.Transport:
		transport = current.Clone()
	default:
		return nil, fmt.Errorf("private network guard requires an *http.Transport, got %T", base.Transport)
	}
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("parse network destination %q: %w", address, err)
			}
			ip := net.ParseIP(strings.Trim(host, "[]"))
			if ip == nil {
				return fmt.Errorf("network destination %q did not resolve to an IP address", host)
			}
			if isPrivateNetworkIP(ip) {
				return fmt.Errorf("network destination resolved to private or local address %s", ip)
			}
			return nil
		},
	}
	transport.DialContext = dialer.DialContext
	client.Transport = transport
	return &client, nil
}
