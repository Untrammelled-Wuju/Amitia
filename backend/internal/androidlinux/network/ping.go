//go:build linux && !android

package network

import (
	"context"
	"fmt"
	"math"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type PingBackend interface {
	Ping(ctx context.Context, host string, count int, timeout time.Duration) (PingResult, error)
}

type ICMPPingBackend struct {
	resolveFunc func(ctx context.Context, host string) (net.IP, error)
}

func NewICMPPingBackend() *ICMPPingBackend {
	return &ICMPPingBackend{
		resolveFunc: func(ctx context.Context, host string) (net.IP, error) {
			if ip := net.ParseIP(host); ip != nil {
				return ip, nil
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, ErrEndpointDenied("no addresses resolved")
			}
			return ips[0], nil
		},
	}
}

func (b *ICMPPingBackend) Ping(ctx context.Context, host string, count int, timeout time.Duration) (PingResult, error) {
	result := PingResult{
		Host: host,
		Mode: "icmp",
	}

	ip, err := b.resolveFunc(ctx, host)
	if err != nil {
		return result, err
	}
	result.ResolvedIP = ip.String()

	for i := 0; i < count; i++ {
		result.Sent++
		rtt, err := b.sendSinglePing(ctx, ip, timeout)
		if err != nil {
			continue
		}
		result.Received++
		if result.Received == 1 || rtt < result.MinMs {
			result.MinMs = rtt
		}
		if result.Received == 1 || rtt > result.MaxMs {
			result.MaxMs = rtt
		}
		result.AvgMs += rtt
	}

	if result.Received > 0 {
		result.AvgMs = result.AvgMs / float64(result.Received)
	}
	if result.Sent > 0 {
		result.LossPercent = float64(result.Sent-result.Received) / float64(result.Sent) * 100
	}

	if result.MinMs == math.MaxFloat64 {
		result.MinMs = 0
	}
	if result.MaxMs == 0 && result.Received > 0 {
		result.MaxMs = result.MinMs
	}

	return result, nil
}

func (b *ICMPPingBackend) sendSinglePing(ctx context.Context, ip net.IP, timeout time.Duration) (float64, error) {
	addr := ip.String()
	if ip.To4() == nil {
		addr = "[" + addr + "]"
	}

	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", strconv.Itoa(int(timeout.Seconds())), addr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}

	return parsePingOutput(string(output))
}

func parsePingOutput(output string) (float64, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if idx := strings.Index(line, "time="); idx >= 0 {
			rest := line[idx+5:]
			end := strings.IndexAny(rest, " \t\r\n")
			if end < 0 {
				end = len(rest)
			}
			rttStr := rest[:end]
			rtt, err := strconv.ParseFloat(rttStr, 64)
			if err != nil {
				return 0, err
			}
			return rtt, nil
		}
	}
	return 0, fmt.Errorf("could not parse ping output")
}

type ShellPingFallback struct{}

func (s *ShellPingFallback) Ping(ctx context.Context, host string, count int, timeout time.Duration) (PingResult, error) {
	result := PingResult{
		Host: host,
		Mode: "tcp-fallback",
	}

	ip, err := resolveHost(ctx, host)
	if err != nil {
		return result, err
	}
	result.ResolvedIP = ip.String()

	for i := 0; i < count; i++ {
		result.Sent++
		start := time.Now()
		conn, cerr := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), "80"), timeout)
		if cerr != nil {
			continue
		}
		rtt := float64(time.Since(start).Microseconds()) / 1000.0
		conn.Close()
		result.Received++
		if result.Received == 1 || rtt < result.MinMs {
			result.MinMs = rtt
		}
		if result.Received == 1 || rtt > result.MaxMs {
			result.MaxMs = rtt
		}
		result.AvgMs += rtt
	}

	if result.Received > 0 {
		result.AvgMs = result.AvgMs / float64(result.Received)
	}
	if result.Sent > 0 {
		result.LossPercent = float64(result.Sent-result.Received) / float64(result.Sent) * 100
	}

	return result, nil
}

func resolveHost(ctx context.Context, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, ErrEndpointDenied("no addresses resolved")
	}
	return ips[0], nil
}
