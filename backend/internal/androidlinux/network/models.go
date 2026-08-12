package network

type Status struct {
	Available         bool `json:"available"`
	DNSAvailable      bool `json:"dnsAvailable"`
	OutboundAvailable bool `json:"outboundAvailable"`
	LoopbackAvailable bool `json:"loopbackAvailable"`
	HTTPAvailable     bool `json:"httpAvailable"`
	TCPAvailable      bool `json:"tcpAvailable"`
	InterfaceCount    int  `json:"interfaceCount"`
}

type Interface struct {
	Name         string   `json:"name"`
	Index        int      `json:"index"`
	MTU          int      `json:"mtu"`
	HardwareAddr string   `json:"hardwareAddr,omitempty"`
	Flags        []string `json:"flags"`
	Addresses    []string `json:"addresses"`
}

type Route struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Prefix      int    `json:"prefix"`
	Metric      int    `json:"metric,omitempty"`
	Family      string `json:"family"`
}

type DNSLookupRequest struct {
	Host string `json:"host"`
	Type string `json:"type,omitempty"`
}

type MX struct {
	Host string `json:"host"`
	Pref uint16 `json:"pref"`
}

type DNSLookupResult struct {
	Host      string   `json:"host"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses,omitempty"`
	CNAME     string   `json:"cname,omitempty"`
	TXT       []string `json:"txt,omitempty"`
	MX        []MX     `json:"mx,omitempty"`
}

type PingRequest struct {
	Host      string `json:"host"`
	Count     int    `json:"count,omitempty"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
}

type PingResult struct {
	Host           string  `json:"host"`
	ResolvedIP     string  `json:"resolvedIp"`
	Sent           int     `json:"sent"`
	Received       int     `json:"received"`
	LossPercent    float64 `json:"lossPercent"`
	MinMs          float64 `json:"minMs"`
	AvgMs          float64 `json:"avgMs"`
	MaxMs          float64 `json:"maxMs"`
	Mode           string  `json:"mode"`
}

type TCPProbeRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
}

type TCPProbeResult struct {
	Host          string `json:"host"`
	ResolvedIP    string `json:"resolvedIp"`
	Port          int    `json:"port"`
	Reachable     bool   `json:"reachable"`
	ConnectTimeMS int64  `json:"connectTimeMs"`
}

type HTTPRequest struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	BodyBase64      string            `json:"bodyBase64,omitempty"`
	TimeoutMS       int               `json:"timeoutMs,omitempty"`
	FollowRedirects *bool             `json:"followRedirects,omitempty"`
	MaxResponseBytes int64            `json:"maxResponseBytes,omitempty"`
}

type HTTPResponse struct {
	StatusCode int                 `json:"statusCode"`
	Status     string              `json:"status"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body,omitempty"`
	BodyBase64 string              `json:"bodyBase64,omitempty"`
	MIMEType   string              `json:"mimeType,omitempty"`
	Charset    string              `json:"charset,omitempty"`
	BytesRead  int64               `json:"bytesRead"`
	Truncated  bool                `json:"truncated"`
	FinalURL   string              `json:"finalUrl"`
	DurationMS int64               `json:"durationMs"`
}

type DownloadRequest struct {
	URL       string `json:"url"`
	Target    string `json:"target"`
	Overwrite bool   `json:"overwrite,omitempty"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
	MaxBytes  int64  `json:"maxBytes,omitempty"`
}

type DownloadResult struct {
	Path         string `json:"path"`
	BytesWritten int64  `json:"bytesWritten"`
	MIMEType     string `json:"mimeType"`
	SHA256       string `json:"sha256"`
	StatusCode   int    `json:"statusCode"`
	FinalURL     string `json:"finalUrl"`
}
