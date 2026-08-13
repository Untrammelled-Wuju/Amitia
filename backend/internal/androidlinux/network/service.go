//go:build linux && !android

package network

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/androidlinux/fileops"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/util"
)

type FileWriter interface {
	Write(ctx context.Context, path string, data []byte, opts fileops.WriteOptions) (fileops.StatResult, error)
}

type fileOpsWriter struct {
	svc *fileops.Service
}

func (w *fileOpsWriter) Write(ctx context.Context, path string, data []byte, opts fileops.WriteOptions) (fileops.StatResult, error) {
	return w.svc.Write(path, data, opts)
}

type Service struct {
	host       runtimehost.RuntimeHost
	paths      util.RuntimePaths
	policy     Policy
	fileWriter FileWriter
	pingBackend PingBackend
}

func NewService(host runtimehost.RuntimeHost, paths util.RuntimePaths, policy Policy, fileWriter FileWriter) *Service {
	svc := &Service{
		host:       host,
		paths:      paths,
		policy:     policy,
		fileWriter: fileWriter,
	}
	if host.Capabilities().Supports(runtimehost.CapNetworkLoopback) {
		svc.pingBackend = NewICMPPingBackend()
	} else {
		svc.pingBackend = &ShellPingFallback{}
	}
	return svc
}

func NewServiceFromFileOps(host runtimehost.RuntimeHost, paths util.RuntimePaths, policy Policy, fileSvc *fileops.Service) *Service {
	wrapper := &fileOpsWriter{svc: fileSvc}
	return NewService(host, paths, policy, wrapper)
}

func (s *Service) SetPingBackend(backend PingBackend) {
	s.pingBackend = backend
}

func (s *Service) Close() {
}

func (s *Service) Status(ctx context.Context) Status {
	caps := s.host.Capabilities()
	status := Status{
		Available:         true,
		DNSAvailable:      caps.Support(runtimehost.CapNetworkDNS) != runtimehost.SupportUnsupported,
		OutboundAvailable: caps.Support(runtimehost.CapNetworkOutbound) != runtimehost.SupportUnsupported,
		LoopbackAvailable: caps.Supports(runtimehost.CapNetworkLoopback),
		TCPAvailable:      caps.Support(runtimehost.CapNetworkTCP) != runtimehost.SupportUnsupported,
		HTTPAvailable:     caps.Support(runtimehost.CapNetworkHTTP) != runtimehost.SupportUnsupported,
	}

	ifaces, err := net.Interfaces()
	if err == nil {
		status.InterfaceCount = len(ifaces)
	}

	return status
}

func (s *Service) Interfaces(ctx context.Context) []Interface {
	return collectInterfaces()
}

func (s *Service) Routes(ctx context.Context) []Route {
	return collectRoutes()
}

func (s *Service) DNSLookup(ctx context.Context, req DNSLookupRequest) (DNSLookupResult, error) {
	maxResults := s.policy.MaxDNSResults
	if req.Host == "" {
		return DNSLookupResult{}, ErrInvalidURL("", "host is required")
	}
	return performDNSLookup(ctx, req.Host, req.Type, maxResults)
}

func (s *Service) Ping(ctx context.Context, req PingRequest) (PingResult, error) {
	if s.pingBackend == nil {
		return PingResult{Host: req.Host}, ErrICMPUnavailable()
	}

	host := req.Host
	if host == "" {
		return PingResult{}, ErrInvalidURL("", "host is required")
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > s.policy.MaxPingCount {
		count = s.policy.MaxPingCount
	}

	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = s.policy.DefaultTimeout
	}
	if timeout > s.policy.MaxTimeout {
		timeout = s.policy.MaxTimeout
	}

	if ip := net.ParseIP(host); ip == nil {
		validator := NewEndpointValidator(s.policy)
		_, err := validator.ResolveAndClassify(ctx, "https://"+host)
		if err != nil {
			if ipAddr := net.ParseIP(host); ipAddr != nil {
				return PingResult{Host: host}, ErrEndpointDenied("not allowed")
			}
			return PingResult{Host: host}, err
		}
	}

	return s.pingBackend.Ping(ctx, host, count, timeout)
}

func (s *Service) TCPProbe(ctx context.Context, req TCPProbeRequest) (TCPProbeResult, error) {
	if req.Host == "" {
		return TCPProbeResult{}, ErrInvalidURL("", "host is required")
	}
	if req.Port < 1 || req.Port > 65535 {
		return TCPProbeResult{Port: req.Port}, ErrEndpointDenied("invalid port")
	}

	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = s.policy.DefaultTimeout
	}
	if timeout > s.policy.MaxTimeout {
		timeout = s.policy.MaxTimeout
	}

	validator := NewEndpointValidator(s.policy)
	_, err := validator.ResolveAndClassify(ctx, "tcp://"+req.Host+":"+strconv.Itoa(req.Port))
	if err != nil {
		return TCPProbeResult{Host: req.Host, Port: req.Port}, err
	}

	return performTCPProbe(ctx, req.Host, req.Port, timeout)
}

func (s *Service) HTTPRequest(ctx context.Context, req HTTPRequest) (HTTPResponse, error) {
	if req.URL == "" {
		return HTTPResponse{}, ErrInvalidURL("", "URL is required")
	}
	return performHTTPRequest(ctx, req, s.policy)
}

func (s *Service) Download(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	if req.URL == "" {
		return DownloadResult{}, ErrInvalidURL("", "URL is required")
	}
	if req.Target == "" {
		return DownloadResult{}, ErrDownloadDenied("target path is required")
	}
	return performDownload(ctx, req, s.policy, s.fileWriter)
}
