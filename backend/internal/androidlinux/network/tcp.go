//go:build linux && !android

package network

import (
	"context"
	"net"
	"strconv"
	"time"
)

func performTCPProbe(ctx context.Context, host string, port int, timeout time.Duration) (TCPProbeResult, error) {
	result := TCPProbeResult{
		Host: host,
		Port: port,
	}

	if port < 1 || port > 65535 {
		return result, ErrEndpointDenied("invalid port number")
	}

	if ip := net.ParseIP(host); ip != nil {
		result.ResolvedIP =	ip.String()
		return probeIP(ctx, result, timeout)
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return result, err
	}
	if len(ips) == 0 {
		return result, ErrEndpointDenied("no addresses resolved")
	}

	result.ResolvedIP = ips[0].String()
	return probeIP(ctx, result, timeout)
}

func probeIP(ctx context.Context, result TCPProbeResult, timeout time.Duration) (TCPProbeResult, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	addr := net.JoinHostPort(result.ResolvedIP, strconv.Itoa(result.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	result.ConnectTimeMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Reachable = false
		return result, nil
	}
	defer conn.Close()
	result.Reachable = true
	return result, nil
}
