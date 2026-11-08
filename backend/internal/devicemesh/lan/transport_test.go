package lan

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPinnedTLSBindsIdentityAddressAndValidity(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate, fingerprint, err := Certificate(key, "core-b", []net.IP{net.ParseIP("192.168.1.23")}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := certificate.Leaf.PublicKey.(*ecdsa.PublicKey); !ok {
		t.Fatalf("LAN TLS certificate must use ECDSA, got %T", certificate.Leaf.PublicKey)
	}
	if certificate.Leaf.SignatureAlgorithm != x509.ECDSAWithSHA256 {
		t.Fatalf("LAN TLS certificate signature algorithm = %v, want ECDSAWithSHA256", certificate.Leaf.SignatureAlgorithm)
	}
	endpoint := Endpoint{URL: "https://192.168.1.23:18899", Fingerprint: fingerprint, CoreID: "core-b"}
	config, err := PinnedTLS(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	state := tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate.Leaf}}
	if err := config.VerifyConnection(state); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []Endpoint{
		{URL: endpoint.URL, Fingerprint: strings.Repeat("0", 64), CoreID: endpoint.CoreID},
		{URL: endpoint.URL, Fingerprint: fingerprint, CoreID: "core-c"},
		{URL: "https://192.168.1.24:18899", Fingerprint: fingerprint, CoreID: endpoint.CoreID},
	} {
		config, err := PinnedTLS(invalid)
		if err == nil && config.VerifyConnection(state) == nil {
			t.Fatal("错误身份或地址获得信任")
		}
	}
	expired, _, err := Certificate(key, "core-b", []net.IP{net.ParseIP("192.168.1.23")}, time.Now().Add(-100*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := config.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{expired.Leaf}}); err == nil {
		t.Fatal("接受过期证书")
	}
}

func TestTLSKeyPersistsAcrossCertificateRotation(t *testing.T) {
	dataDir := t.TempDir()
	firstKey, err := loadOrCreateTLSKey(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin, err := Certificate(firstKey, "core-b", []net.IP{net.ParseIP("192.168.1.23")}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	secondKey, err := loadOrCreateTLSKey(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	second, secondPin, err := Certificate(secondKey, "core-b", []net.IP{net.ParseIP("192.168.1.24")}, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	firstPublic := first.Leaf.PublicKey.(*ecdsa.PublicKey)
	secondPublic := second.Leaf.PublicKey.(*ecdsa.PublicKey)
	if firstPublic.X.Cmp(secondPublic.X) != 0 || firstPublic.Y.Cmp(secondPublic.Y) != 0 || firstPin != secondPin {
		t.Fatal("局域网 TLS 证书轮换改变了公钥指纹")
	}
	if first.Leaf.SerialNumber.Cmp(second.Leaf.SerialNumber) == 0 {
		t.Fatal("局域网 TLS 证书轮换未生成新证书")
	}
	info, err := os.Stat(filepath.Join(dataDir, "device-mesh", "lan-tls-key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("局域网 TLS 私钥权限 = %o, want 600", info.Mode().Perm())
	}
}

func TestTLSKeyCorruptionReturnsOriginalParseError(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "device-mesh", "lan-tls-key")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := loadOrCreateTLSKey(dataDir)
	_, expected := x509.ParsePKCS8PrivateKey([]byte("broken"))
	if err == nil || expected == nil || err.Error() != expected.Error() {
		t.Fatalf("损坏私钥未返回原始解析错误: actual=%v expected=%v", err, expected)
	}
}

func TestPinnedTLSReconnectsAfterServerRestart(t *testing.T) {
	addresses, err := PrivateAddresses()
	if err != nil || len(addresses) == 0 {
		t.Skip("局域网重启测试需要私有网络接口")
	}
	address := addresses[0]
	probe, err := net.Listen("tcp", net.JoinHostPort(address.String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ready")
	})
	first, err := Start(handler, dataDir, "core-b", port, []net.IP{address})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := first.Endpoints[0]
	client := pinnedHTTPClient(t, endpoint)
	assertPinnedResponse(t, client, endpoint.URL)
	if err := first.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	client.CloseIdleConnections()

	second, err := Start(handler, dataDir, "core-b", port, []net.IP{address})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close(t.Context()) })
	if second.Endpoints[0].Fingerprint != endpoint.Fingerprint {
		t.Fatal("服务重启后局域网 TLS 指纹发生变化")
	}
	assertPinnedResponse(t, client, endpoint.URL)
}

func pinnedHTTPClient(t *testing.T, endpoint Endpoint) *http.Client {
	t.Helper()
	configuration, err := PinnedTLS(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = configuration
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}

func assertPinnedResponse(t *testing.T, client *http.Client, endpoint string) {
	t.Helper()
	response, err := client.Get(endpoint + "/livez")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "ready" {
		t.Fatalf("固定指纹请求失败: status=%d body=%q", response.StatusCode, body)
	}
}

func TestPinnedTLSRejectsUnscopedTrust(t *testing.T) {
	for _, endpoint := range []Endpoint{
		{URL: "http://192.168.1.2:18899"},
		{URL: "https://example.com:18899"},
		{URL: "https://192.168.1.2:18899/path"},
		{URL: "https://user@192.168.1.2:18899"},
		{URL: "https://192.168.1.2:18899?token=x"},
	} {
		if _, err := PinnedTLS(endpoint); err == nil {
			t.Fatal("接受未限定信任的地址")
		}
	}
}
