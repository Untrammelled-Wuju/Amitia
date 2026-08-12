//go:build linux && !android

package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEndpointValidator_PublicIPv4(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	sec, err := validator.ResolveAndClassify(nil, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, EndpointPublic, sec.Class)
}

func TestNewEndpointValidator_PublicIPv4Literal(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	sec, err := validator.ResolveAndClassify(nil, "https://93.184.216.34")
	require.NoError(t, err)
	assert.Equal(t, EndpointPublic, sec.Class)
}

func TestNewEndpointValidator_PrivateDenied(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowPrivateNetwork = false
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "https://192.168.1.1")
	require.Error(t, err)
}

func TestNewEndpointValidator_PrivateAllowed(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowPrivateNetwork = true
	validator := NewEndpointValidator(policy)

	sec, err := validator.ResolveAndClassify(nil, "https://192.168.1.1")
	require.NoError(t, err)
	assert.Equal(t, EndpointPrivate, sec.Class)
}

func TestNewEndpointValidator_Loopback(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	sec, err := validator.ResolveAndClassify(nil, "https://127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, EndpointLoopback, sec.Class)
}

func TestNewEndpointValidator_LoopbackDenied(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowLoopback = false
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "https://127.0.0.1")
	require.Error(t, err)
}

func TestNewEndpointValidator_MetadataDenied(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "https://169.254.169.254")
	require.Error(t, err)
}

func TestNewEndpointValidator_InvalidScheme(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "ftp://example.com")
	require.Error(t, err)
}

func TestNewEndpointValidator_EmptyURL(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "")
	require.Error(t, err)
}

func TestNewEndpointValidator_Credentials(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "https://user:pass@example.com")
	require.Error(t, err)
}

func TestNewEndpointValidator_CRLFInjection(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "https://example\r.com")
	require.Error(t, err)
}

func TestNewEndpointValidator_LinkLocalDenied(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "https://169.254.1.1")
	require.Error(t, err)
}

func TestNewEndpointValidator_InvalidPort(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	_, err := validator.ResolveAndClassify(nil, "https://example.com:99999")
	require.Error(t, err)
}

func TestClassifySingleIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected EndpointClass
	}{
		{"loopback", "127.0.0.1", EndpointLoopback},
		{"private 10.x", "10.0.0.1", EndpointPrivate},
		{"private 192.168.x", "192.168.0.1", EndpointPrivate},
		{"private 172.16.x", "172.16.0.1", EndpointPrivate},
		{"link-local", "169.254.1.1", EndpointLinkLocal},
		{"unspecified", "0.0.0.0", EndpointUnspecified},
		{"ULA", "fc00::1", EndpointPrivate},
		{"public", "93.184.216.34", EndpointPublic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifySingleIP("", mustParseIP(tt.ip))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseLiteralIPs(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"127.0.0.1", 1},
		{"::1", 1},
		{"[::1]", 1},
		{"example.com", 0},
	}
	for _, tt := range tests {
		result := parseLiteralIPs(tt.input)
		assert.Len(t, result, tt.expected, "parseLiteralIPs(%q)", tt.input)
	}
}

func TestParseIPv4Literal(t *testing.T) {
	policy := DefaultPolicy()
	validator := NewEndpointValidator(policy)

	sec, err := validator.ResolveAndClassify(nil, "https://8.8.8.8")
	require.NoError(t, err)
	assert.Equal(t, EndpointPublic, sec.Class)
}

func mustParseIP(s string) []byte {
	ip := parseIP(s)
	if ip == nil {
		panic("invalid IP: " + s)
	}
	return ip
}

func parseIP(s string) []byte {
	ipBytes := make([]byte, 4)
	parts := splitIP(s)
	if len(parts) != 4 {
		return nil
	}
	for i, p := range parts {
		val := 0
		for _, c := range p {
			val = val*10 + int(c-'0')
		}
		ipBytes[i] = byte(val)
	}
	return ipBytes
}

func splitIP(s string) []string {
	parts := make([]string, 0, 4)
	current := ""
	for _, c := range s {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}
