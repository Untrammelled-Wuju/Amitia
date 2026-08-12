//go:build linux && !android

package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPolicy(t *testing.T) {
	policy := DefaultPolicy()

	assert.Equal(t, 15*time.Second, policy.DefaultTimeout)
	assert.Equal(t, 60*time.Second, policy.MaxTimeout)
	assert.Equal(t, 32, policy.MaxDNSResults)
	assert.Equal(t, 5, policy.MaxPingCount)
	assert.Equal(t, int64(4*1024*1024), policy.MaxHTTPBodyBytes)
	assert.Equal(t, int64(16*1024*1024), policy.MaxHTTPResponseBytes)
	assert.Equal(t, int64(256*1024*1024), policy.MaxDownloadBytes)
	assert.Equal(t, 5, policy.MaxRedirects)
	assert.True(t, policy.AllowPublicInternet)
	assert.False(t, policy.AllowPrivateNetwork)
	assert.True(t, policy.AllowLoopback)
	assert.True(t, policy.DenyLinkLocal)
	assert.True(t, policy.DenyMulticast)
	assert.True(t, policy.DenyUnspecified)
	assert.Equal(t, "Amitia/1.0", policy.UserAgent)
	assert.Equal(t, 4, policy.MaxConcurrentHTTP)
	assert.Equal(t, 2, policy.MaxConcurrentDownload)
}
