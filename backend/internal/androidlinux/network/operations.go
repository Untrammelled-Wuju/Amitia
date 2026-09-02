//go:build linux && !android

package network

const (
	OpStatus     = "network.status"
	OpInterfaces = "network.interfaces"
	OpRoutes     = "network.routes"

	OpDNSLookup = "network.dns.lookup"
	OpPing      = "network.ping"
	OpTCPProbe  = "network.tcp.probe"

	OpHTTPRequest      = "network.http.request"
	OpMultipartRequest = "network.http.multipart"
	OpDownload         = "network.download"
)
