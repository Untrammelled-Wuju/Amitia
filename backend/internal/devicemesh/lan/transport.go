package lan

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/u-ai/backend/internal/ioshostbridge"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Endpoint struct {
	URL         string `json:"url"`
	Fingerprint string `json:"fingerprint"`
	CoreID      string `json:"coreId"`
}

func PrivateAddresses() ([]net.IP, error) {
	bridge, err := ioshostbridge.FromEnvironment()
	if err != nil {
		return nil, err
	}
	if bridge != nil {
		var result struct {
			Addresses []string `json:"addresses"`
		}
		if err := bridge.Call("network.privateAddresses", map[string]any{}, &result); err != nil {
			return nil, err
		}
		return validatedHostAddresses(result.Addresses)
	}
	if configured := strings.TrimSpace(os.Getenv("AMITIA_LAN_ADDRESSES")); configured != "" {
		seen := map[string]bool{}
		result := []net.IP{}
		for _, address := range strings.Split(configured, ",") {
			ip := net.ParseIP(strings.TrimSpace(address))
			if ip == nil || ip.To4() == nil || !ip.IsPrivate() || ip.IsLoopback() {
				return nil, errors.New("宿主提供的局域网地址无效")
			}
			if !seen[ip.String()] {
				result = append(result, ip.To4())
				seen[ip.String()] = true
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
		return result, nil
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	seen := map[string]net.IP{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && ip.IsPrivate() {
				seen[ip.String()] = ip.To4()
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]net.IP, 0, len(keys))
	for _, key := range keys {
		result = append(result, seen[key])
	}
	return result, nil
}

func validatedHostAddresses(addresses []string) ([]net.IP, error) {
	if len(addresses) > 16 {
		return nil, errors.New("宿主提供的局域网地址超过上限")
	}
	seen := map[string]bool{}
	result := []net.IP{}
	for _, address := range addresses {
		ip := net.ParseIP(address)
		if ip == nil || ip.To4() == nil || !ip.IsPrivate() || !ip.IsGlobalUnicast() || ip.IsLoopback() {
			return nil, errors.New("宿主提供的局域网地址无效")
		}
		if !seen[ip.String()] {
			seen[ip.String()] = true
			result = append(result, ip.To4())
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result, nil
}

func Certificate(signer crypto.Signer, coreID string, addresses []net.IP, now time.Time) (tls.Certificate, string, error) {
	if signer == nil || strings.TrimSpace(coreID) == "" || len(addresses) == 0 {
		return tls.Certificate{}, "", errors.New("局域网证书缺少设备身份或地址")
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, "", err
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: coreID}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(90 * 24 * time.Hour), IPAddresses: addresses, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, signer.Public(), signer)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	hash := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: signer, Leaf: leaf}, hex.EncodeToString(hash[:]), nil
}

func PinnedTLS(endpoint Endpoint) (*tls.Config, error) {
	parsed, err := url.Parse(endpoint.URL)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, errors.New("局域网服务地址无效")
	}
	ip := net.ParseIP(parsed.Hostname())
	pin, err := hex.DecodeString(endpoint.Fingerprint)
	if ip == nil || !ip.IsPrivate() || err != nil || len(pin) != sha256.Size || endpoint.CoreID == "" {
		return nil, errors.New("局域网身份指纹无效")
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, ServerName: parsed.Hostname(), InsecureSkipVerify: true, VerifyConnection: func(connection tls.ConnectionState) error {
		if len(connection.PeerCertificates) != 1 {
			return errors.New("局域网证书链无效")
		}
		certificate := connection.PeerCertificates[0]
		hash := sha256.Sum256(certificate.RawSubjectPublicKeyInfo)
		if subtle.ConstantTimeCompare(hash[:], pin) != 1 || certificate.Subject.CommonName != endpoint.CoreID {
			return errors.New("局域网服务身份已改变，请重新扫码配对")
		}
		if err := certificate.VerifyHostname(parsed.Hostname()); err != nil {
			return err
		}
		pool := x509.NewCertPool()
		pool.AddCert(certificate)
		_, err := certificate.Verify(x509.VerifyOptions{Roots: pool, DNSName: parsed.Hostname(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		return err
	}}, nil
}

type Server struct {
	Endpoints []Endpoint
	servers   []*http.Server
	listeners []net.Listener
	errors    chan error
}

func (s *Server) Errors() <-chan error { return s.errors }

func Start(handler http.Handler, signer crypto.Signer, coreID string, port int, addresses []net.IP) (*Server, error) {
	if port < 1 || port > 65535 || port == 3000 || handler == nil {
		return nil, errors.New("局域网服务配置无效")
	}
	certificate, fingerprint, err := Certificate(signer, coreID, addresses, time.Now())
	if err != nil {
		return nil, err
	}
	result := &Server{errors: make(chan error, len(addresses)+1)}
	for _, address := range addresses {
		if !address.IsPrivate() || address.IsLoopback() {
			result.Close(context.Background())
			return nil, errors.New("局域网地址必须为私有网络地址")
		}
		listener, err := net.Listen("tcp", net.JoinHostPort(address.String(), strconv.Itoa(port)))
		if err != nil {
			result.Close(context.Background())
			return nil, fmt.Errorf("局域网监听失败: %w", err)
		}
		server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32768}
		result.listeners = append(result.listeners, listener)
		result.servers = append(result.servers, server)
		result.Endpoints = append(result.Endpoints, Endpoint{URL: "https://" + listener.Addr().String(), Fingerprint: fingerprint, CoreID: coreID})
		secured := tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}})
		go func() {
			if err := server.Serve(secured); err != nil && !errors.Is(err, http.ErrServerClosed) {
				result.errors <- err
			}
		}()
	}
	return result, nil
}

func (s *Server) Close(ctx context.Context) error {
	var failures []error
	for _, server := range s.servers {
		if err := server.Shutdown(ctx); err != nil {
			failures = append(failures, err)
		}
	}
	for _, listener := range s.listeners {
		_ = listener.Close()
	}
	return errors.Join(failures...)
}
