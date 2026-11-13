package lan

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
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
	"github.com/u-ai/backend/internal/secretstore"
	amitialog "log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Endpoint struct {
	URL         string `json:"url"`
	Fingerprint string `json:"fingerprint"`
	CoreID      string `json:"coreId"`
}

var tlsKeyLocks sync.Map

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

func Certificate(privateKey *ecdsa.PrivateKey, coreID string, addresses []net.IP, now time.Time) (tls.Certificate, string, error) {
	if privateKey == nil || privateKey.D == nil || privateKey.Curve != elliptic.P256() || strings.TrimSpace(coreID) == "" || len(addresses) == 0 {
		return tls.Certificate{}, "", errors.New("局域网证书缺少设备身份或地址")
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, "", err
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: coreID}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(90 * 24 * time.Hour), IPAddresses: addresses, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	hash := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: privateKey, Leaf: leaf}, hex.EncodeToString(hash[:]), nil
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

func Start(handler http.Handler, dataDir string, coreID string, port int, addresses []net.IP) (*Server, error) {
	if port < 1 || port > 65535 || port == 3000 || handler == nil {
		return nil, errors.New("局域网服务配置无效")
	}
	privateKey, err := loadOrCreateTLSKey(dataDir)
	if err != nil {
		return nil, err
	}
	certificate, fingerprint, err := Certificate(privateKey, coreID, addresses, time.Now())
	if err != nil {
		return nil, err
	}
	result := &Server{errors: make(chan error, len(addresses)+1)}
	for _, address := range addresses {
		if !address.IsPrivate() || address.IsLoopback() {
			result.Close(context.Background())
			return nil, errors.New("局域网地址必须为私有网络地址")
		}
	}
	listeners, boundPort, listenErr := listenOnPort(addresses, port)
	if listenErr != nil {
		if !isAddressInUse(listenErr) {
			result.Close(context.Background())
			return nil, fmt.Errorf("局域网监听失败: %w", listenErr)
		}
		fallbackListeners, fallbackPort, fallbackErr := listenOnEphemeralPort(addresses)
		if fallbackErr != nil {
			result.Close(context.Background())
			return nil, fmt.Errorf("局域网监听失败: %w (固定端口 %d 被占用且备用端口不可用: %v)", fallbackErr, port, listenErr)
		}
		amitialog.Printf("[lan] 局域网固定端口 %d 已被占用，改用端口 %d", port, fallbackPort)
		listeners = fallbackListeners
		boundPort = fallbackPort
	}
	if boundPort != port {
		amitialog.Printf("[lan] 局域网服务实际监听端口 %d", boundPort)
	}
	for _, listener := range listeners {
		server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32768}
		result.listeners = append(result.listeners, listener)
		result.servers = append(result.servers, server)
		result.Endpoints = append(result.Endpoints, Endpoint{URL: "https://" + listener.Addr().String(), Fingerprint: fingerprint, CoreID: coreID})
		secured := tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}})
		go func(server *http.Server, secured net.Listener) {
			if err := server.Serve(secured); err != nil && !errors.Is(err, http.ErrServerClosed) {
				result.errors <- err
			}
		}(server, secured)
	}
	return result, nil
}

func listenOnPort(addresses []net.IP, port int) ([]net.Listener, int, error) {
	listeners := make([]net.Listener, 0, len(addresses))
	for _, address := range addresses {
		listener, err := net.Listen("tcp", net.JoinHostPort(address.String(), strconv.Itoa(port)))
		if err != nil {
			for _, opened := range listeners {
				if closeErr := opened.Close(); closeErr != nil {
					amitialog.Printf("[original-error] %v", closeErr)
				}
			}
			return nil, 0, err
		}
		listeners = append(listeners, listener)
	}
	return listeners, port, nil
}

func listenOnEphemeralPort(addresses []net.IP) ([]net.Listener, int, error) {
	if len(addresses) == 0 {
		return nil, 0, errors.New("局域网地址为空")
	}
	first, err := net.Listen("tcp", net.JoinHostPort(addresses[0].String(), "0"))
	if err != nil {
		return nil, 0, err
	}
	_, portText, err := net.SplitHostPort(first.Addr().String())
	if err != nil {
		first.Close()
		return nil, 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		first.Close()
		return nil, 0, err
	}
	listeners := []net.Listener{first}
	for _, address := range addresses[1:] {
		listener, err := net.Listen("tcp", net.JoinHostPort(address.String(), portText))
		if err != nil {
			for _, opened := range listeners {
				if closeErr := opened.Close(); closeErr != nil {
					amitialog.Printf("[original-error] %v", closeErr)
				}
			}
			return nil, 0, err
		}
		listeners = append(listeners, listener)
	}
	return listeners, port, nil
}

func isAddressInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

func loadOrCreateTLSKey(dataDir string) (*ecdsa.PrivateKey, error) {
	dataDir = filepath.Clean(strings.TrimSpace(dataDir))
	if dataDir == "" || dataDir == "." {
		return nil, errors.New("局域网 TLS 密钥缺少数据目录")
	}
	path := filepath.Join(dataDir, "device-mesh", "lan-tls-key")
	lockValue, _ := tlsKeyLocks.LoadOrStore(path, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	encoded, err := secretstore.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		encoded, err = x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return nil, err
		}
		if err := secretstore.Write(path, encoded); err != nil {
			return nil, err
		}
		return privateKey, nil
	}
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil && !errors.Is(err, os.ErrPermission) {
		return nil, err
	}
	parsed, err := x509.ParsePKCS8PrivateKey(encoded)
	if err != nil {
		return nil, err
	}
	privateKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || privateKey.D == nil || privateKey.Curve != elliptic.P256() {
		return nil, errors.New("局域网 TLS 私钥无效")
	}
	return privateKey, nil
}

func (s *Server) Close(ctx context.Context) error {
	var failures []error
	for _, server := range s.servers {
		if err := server.Shutdown(ctx); err != nil {
			failures = append(failures, err)
		}
	}
	for _, listener := range s.listeners {
		if err := listener.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	return errors.Join(failures...)
}
