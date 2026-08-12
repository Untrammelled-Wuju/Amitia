//go:build linux && !android

package ssh


type HostKeyPolicy string

const (
	HostKeyPolicyReject  HostKeyPolicy = "reject"
	HostKeyPolicyAcceptNew HostKeyPolicy = "accept_new"
)

type Policy struct {
	Enabled              bool          `json:"enabled"`
	MaxSessionIdleSecond int           `json:"maxSessionIdleSecond"`
	MaxSessions          int           `json:"maxSessions"`
	MaxOutputBytes       int64         `json:"maxOutputBytes"`
	DefaultTimeoutSecond int           `json:"defaultTimeoutSecond"`
	MaxTimeoutSecond     int           `json:"maxTimeoutSecond"`
	MaxStdinBytes        int64         `json:"maxStdinBytes"`
	AllowedPortPolicy    []int         `json:"allowedPortPolicy"`
	DeniedPortList       []int         `json:"deniedPortList"`
	EnableAgentAuth      bool          `json:"enableAgentAuth"`
	DefaultHostKeyPolicy HostKeyPolicy `json:"defaultHostKeyPolicy"`
	MaxSessionCount      int           `json:"maxSessionCount"`
}

func DefaultPolicy() Policy {
	return Policy{
		Enabled:              true,
		MaxSessionIdleSecond: 120,
		MaxSessions:          10,
		MaxOutputBytes:       2 * 1024 * 1024,
		DefaultTimeoutSecond: 30,
		MaxTimeoutSecond:     600,
		MaxStdinBytes:        1 * 1024 * 1024,
		DeniedPortList:       []int{0},
		EnableAgentAuth:      false,
		DefaultHostKeyPolicy: HostKeyPolicyReject,
		MaxSessionCount:      10,
	}
}

func (p Policy) IsPortAllowed(port int, endpointClass string) bool {
	for _, denied := range p.DeniedPortList {
		if denied == port {
			return false
		}
	}
	return true
}
