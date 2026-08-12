package network

import "time"

type Policy struct {
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration

	MaxDNSResults int

	MaxPingCount int

	MaxHTTPBodyBytes     int64
	MaxHTTPResponseBytes int64
	MaxDownloadBytes     int64

	MaxRedirects int

	AllowPublicInternet bool
	AllowPrivateNetwork bool
	AllowLoopback       bool

	DenyLinkLocal bool
	DenyMulticast bool
	DenyUnspecified bool

	UserAgent             string
	MaxConcurrentHTTP     int
	MaxConcurrentDownload int
}

func DefaultPolicy() Policy {
	return Policy{
		DefaultTimeout: 15 * time.Second,
		MaxTimeout:     60 * time.Second,

		MaxDNSResults: 32,

		MaxPingCount: 5,

		MaxHTTPBodyBytes:     4 * 1024 * 1024,
		MaxHTTPResponseBytes: 16 * 1024 * 1024,
		MaxDownloadBytes:     256 * 1024 * 1024,

		MaxRedirects: 5,

		AllowPublicInternet: true,
		AllowPrivateNetwork: false,
		AllowLoopback:       true,

		DenyLinkLocal:   true,
		DenyMulticast:   true,
		DenyUnspecified: true,

		UserAgent:             "Amitia/1.0",
		MaxConcurrentHTTP:     4,
		MaxConcurrentDownload: 2,
	}
}
