//go:build linux && !android

package ssh

import "time"

type SSHStatus struct {
	Enabled         bool     `json:"enabled"`
	ReachableHosts  []string `json:"reachableHosts,omitempty"`
	DefaultUser     string   `json:"defaultUser,omitempty"`
	KnownHostsCount int      `json:"knownHostsCount"`
	MaxSessions     int      `json:"maxSessions"`
	ActiveSessions  int      `json:"activeSessions"`
}

type SSHExecRequest struct {
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	User           string            `json:"user"`
	Command        string            `json:"command"`
	Stdin          string            `json:"stdin,omitempty"`
	TimeoutMs      int64             `json:"timeoutMs,omitempty"`
	MaxOutputBytes int64             `json:"maxOutputBytes,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	WorkingDir     string            `json:"workingDir,omitempty"`
	HostKey        string            `json:"hostKey,omitempty"`
	PrivateKey     string            `json:"privateKey,omitempty"`
	Password       string            `json:"password,omitempty"`
	HostKeyPolicy  string            `json:"hostKeyPolicy,omitempty"`
	AgentAuth      bool              `json:"agentAuth,omitempty"`
	AgentSocket    string            `json:"agentSocket,omitempty"`
}

type SSHExecResult struct {
	ExitCode        int    `json:"exitCode"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdoutTruncated"`
	StderrTruncated bool   `json:"stderrTruncated"`
	StdoutBytes     int64  `json:"stdoutBytes"`
	StderrBytes     int64  `json:"stderrBytes"`
	DurationMs      int64  `json:"durationMs"`
	SessionID       string `json:"sessionId"`
}

type HostKeyScanRequest struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	TimeoutMs int64 `json:"timeoutMs,omitempty"`
}

type HostKeyScanResult struct {
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	Algorithms []string `json:"algorithms"`
	RawKeys    []string `json:"rawKeys"`
	Fingerprints []string `json:"fingerprints"`
}

type KnownHost struct {
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Algorithm   string    `json:"algorithm"`
	Fingerprint string    `json:"fingerprint"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
	Notes       string    `json:"notes,omitempty"`
}

type SSHSession struct {
	ID         string
	Host       string
	Port       int
	User       string
	Client     interface{}
	LastUsed   time.Time
	CreatedAt  time.Time
}
