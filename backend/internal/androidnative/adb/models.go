package adb

import (
	"context"
	"time"
)

type ADBStatus struct {
	Supported             bool   `json:"supported"`
	Backend               string `json:"backend"`
	ServerAvailable       bool   `json:"serverAvailable"`
	DeviceCount           int    `json:"deviceCount"`
	AuthorizedDeviceCount int    `json:"authorizedDeviceCount"`
	DefaultDeviceReady    bool   `json:"defaultDeviceReady"`
	State                 string `json:"state"`
}

type ADBDevice struct {
	Serial    string `json:"serial"`
	State     string `json:"state"`
	Transport string `json:"transport,omitempty"`
	Product   string `json:"product,omitempty"`
	Model     string `json:"model,omitempty"`
	Device    string `json:"device,omitempty"`
	IsDefault bool   `json:"isDefault"`
}

type ADBConfig struct {
	Enabled        bool
	Backend        string
	ExecutablePath string
	DefaultDevice  string
	Timeout        time.Duration
	MaxOutputBytes int64
	MaxCommandArgs int
	MaxArgBytes    int64
}

type ADBExecuteRequest struct {
	DeviceSerial string   `json:"deviceSerial,omitempty"`
	Executable   string   `json:"executable"`
	Args         []string `json:"args,omitempty"`
	Stdin        string   `json:"stdin,omitempty"`
	TimeoutMs    int64    `json:"timeoutMs,omitempty"`
}

type ADBExecuteResult struct {
	DeviceSerial      string `json:"deviceSerial"`
	ExitCode          int    `json:"exitCode"`
	Stdout            string `json:"stdout,omitempty"`
	Stderr            string `json:"stderr,omitempty"`
	DurationMs        int64  `json:"durationMs"`
	TimedOut          bool   `json:"timedOut,omitempty"`
	ExitCodeAvailable bool   `json:"exitCodeAvailable"`
}

type InternalADBExecuteOptions struct {
	Timeout      time.Duration
	MaxOutput    int64
	StdinEnabled bool
}

type InternalADBExecutor interface {
	ExecuteArgs(
		ctx context.Context,
		deviceSerial string,
		args []string,
		opts InternalADBExecuteOptions,
	) (ADBExecuteResult, error)
}

func DefaultConfig() *ADBConfig {
	return &ADBConfig{
		Enabled:        true,
		Backend:        "cli",
		ExecutablePath: "adb",
		Timeout:        defaultTimeoutSeconds * time.Second,
		MaxOutputBytes: maxCombinedBytes,
		MaxCommandArgs: maxArgCount,
		MaxArgBytes:    maxTotalArgBytes,
	}
}
