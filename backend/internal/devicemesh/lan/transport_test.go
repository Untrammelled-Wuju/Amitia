package lan

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"net"
	"strings"
	"testing"
	"time"
)

func TestPinnedTLSBindsIdentityAddressAndValidity(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate, fingerprint, err := Certificate(key, "core-b", []net.IP{net.ParseIP("192.168.1.23")}, time.Now())
	if err != nil {
		t.Fatal(err)
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
