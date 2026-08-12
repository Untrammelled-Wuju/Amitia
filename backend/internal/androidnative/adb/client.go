package adb

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ADBClient struct {
	config    *ADBConfig
	policy    *CommandPolicy
}

func NewADBClient(config *ADBConfig) *ADBClient {
	return &ADBClient{
		config: config,
		policy: NewCommandPolicy(),
	}
}

func (c *ADBClient) IsAvailable(ctx context.Context) bool {
	if !c.config.Enabled || c.config.ExecutablePath == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, c.config.ExecutablePath, "version")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "Android Debug Bridge")
}

func (c *ADBClient) GetStatus(ctx context.Context) ADBStatus {
	status := ADBStatus{
		Supported: false,
		Backend:   "unavailable",
		State:     BackendUnavailable,
	}

	if !c.config.Enabled || c.config.ExecutablePath == "" {
		return status
	}

	if !c.IsAvailable(ctx) {
		return status
	}

	serverAvailable := c.isServerAvailable(ctx)
	if !serverAvailable {
		status.Supported = true
		status.Backend = c.config.Backend
		status.State = BackendNoServer
		return status
	}

	devices, err := c.listDevices(ctx)
	if err != nil {
		status.Supported = true
		status.Backend = c.config.Backend
		status.State = BackendUnavailable
		return status
	}

	status.Supported = true
	status.Backend = c.config.Backend
	status.ServerAvailable = true
	status.DeviceCount = len(devices)

	authorizedCount := 0
	hasDefaultOnline := false
	for _, d := range devices {
		if d.State == DeviceStateDevice {
			authorizedCount++
			if d.IsDefault {
				hasDefaultOnline = true
			}
		}
	}
	status.AuthorizedDeviceCount = authorizedCount
	status.DefaultDeviceReady = hasDefaultOnline

	switch {
	case authorizedCount == 0:
		for _, d := range devices {
			if d.State == DeviceStateUnauthorized {
				status.State = BackendUnauthorized
				return status
			}
			if d.State == DeviceStateOffline {
				status.State = BackendOffline
				return status
			}
		}
		status.State = BackendNoDevice
	case authorizedCount == 1:
		status.State = BackendReady
	default:
		hasDefault := false
		onlineCount := 0
		for _, d := range devices {
			if d.State == DeviceStateDevice {
				onlineCount++
				if c.config.DefaultDevice != "" && d.Serial == c.config.DefaultDevice {
					hasDefault = true
				}
			}
		}
		if hasDefault || onlineCount == 1 {
			status.State = BackendReady
		} else {
			status.State = BackendAmbiguous
		}
	}

	return status
}

func (c *ADBClient) isServerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, c.config.ExecutablePath, "devices")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return bytes.Contains(output, []byte("List of devices attached"))
}

func (c *ADBClient) listDevices(ctx context.Context) ([]ADBDevice, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.config.ExecutablePath, "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseDevicesOutput(string(output), c.config.DefaultDevice), nil
}

func (c *ADBClient) Execute(ctx context.Context, req ADBExecuteRequest) (ADBExecuteResult, error) {
	if !c.config.Enabled {
		return ADBExecuteResult{ExitCodeAvailable: false}, fmt.Errorf("%s: adb backend not enabled", ADB_UNAVAILABLE)
	}

	if !c.policy.IsAllowed(req.Executable) {
		return ADBExecuteResult{ExitCodeAvailable: false}, &PolicyError{Code: ADB_COMMAND_NOT_ALLOWED, Message: "command not allowed: " + req.Executable}
	}

	if err := c.policy.Validate(req.Executable, req.Args); err != nil {
		return ADBExecuteResult{ExitCodeAvailable: false}, err
	}

	deviceSerial, err := c.resolveDevice(req.DeviceSerial)
	if err != nil {
		return ADBExecuteResult{ExitCodeAvailable: false}, err
	}

	timeout := c.config.Timeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	if timeout > maxTimeoutSeconds*time.Second {
		timeout = maxTimeoutSeconds * time.Second
	}

	args := []string{"-s", deviceSerial, "shell", req.Executable}
	args = append(args, req.Args...)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()
	cmd := exec.CommandContext(ctx, c.config.ExecutablePath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(startTime)

	result := ADBExecuteResult{
		DeviceSerial:      deviceSerial,
		DurationMs:        duration.Milliseconds(),
		ExitCodeAvailable: true,
		Stdout:            truncateBytes(stdout.Bytes(), maxStdoutBytes),
		Stderr:            truncateBytes(stderr.Bytes(), maxStderrBytes),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			return result, &PolicyError{Code: ADB_TIMEOUT, Message: "adb execution timed out"}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return result, &PolicyError{Code: ADB_EXECUTION_FAILED, Message: err.Error()}
	}

	result.ExitCode = 0
	return result, nil
}

func (c *ADBClient) resolveDevice(requestedSerial string) (string, error) {
	devices, err := c.listDevices(context.Background())
	if err != nil {
		return "", &PolicyError{Code: ADB_DEVICE_LIST_FAILED, Message: err.Error()}
	}

	var authorizedDevices []ADBDevice
	for _, d := range devices {
		if d.State == DeviceStateDevice {
			authorizedDevices = append(authorizedDevices, d)
		}
	}

	if requestedSerial != "" {
		for _, d := range devices {
			if d.Serial == requestedSerial {
				if d.State == DeviceStateDevice {
					return d.Serial, nil
				}
				if d.State == DeviceStateUnauthorized {
					return "", &PolicyError{Code: ADB_DEVICE_UNAUTHORIZED, Message: "device unauthorized: " + requestedSerial}
				}
				if d.State == DeviceStateOffline {
					return "", &PolicyError{Code: ADB_DEVICE_OFFLINE, Message: "device offline: " + requestedSerial}
				}
				return "", &PolicyError{Code: ADB_DEVICE_NOT_FOUND, Message: "device state invalid: " + d.State}
			}
		}
		return "", &PolicyError{Code: ADB_DEVICE_NOT_FOUND, Message: "device not found: " + requestedSerial}
	}

	if c.config.DefaultDevice != "" {
		for _, d := range authorizedDevices {
			if d.Serial == c.config.DefaultDevice {
				return d.Serial, nil
			}
		}
	}

	if len(authorizedDevices) == 0 {
		for _, d := range devices {
			if d.State == DeviceStateUnauthorized {
				return "", &PolicyError{Code: ADB_DEVICE_UNAUTHORIZED, Message: "device unauthorized"}
			}
			if d.State == DeviceStateOffline {
				return "", &PolicyError{Code: ADB_DEVICE_OFFLINE, Message: "device offline"}
			}
		}
		return "", &PolicyError{Code: ADB_NO_DEVICE, Message: "no device connected"}
	}

	if len(authorizedDevices) > 1 {
		return "", &PolicyError{Code: ADB_DEVICE_AMBIGUOUS, Message: "multiple devices connected, specify deviceSerial"}
	}

	return authorizedDevices[0].Serial, nil
}

func (c *ADBClient) ExecuteArgs(ctx context.Context, deviceSerial string, args []string, opts InternalADBExecuteOptions) (ADBExecuteResult, error) {
	if !c.config.Enabled {
		return ADBExecuteResult{ExitCodeAvailable: false}, fmt.Errorf("%s: adb backend not enabled", ADB_UNAVAILABLE)
	}

	adbArgs := []string{"-s", deviceSerial, "shell"}
	adbArgs = append(adbArgs, args...)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = c.config.Timeout
	}
	if timeout > maxTimeoutSeconds*time.Second {
		timeout = maxTimeoutSeconds * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()
	cmd := exec.CommandContext(ctx, c.config.ExecutablePath, adbArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	result := ADBExecuteResult{
		DeviceSerial:      deviceSerial,
		DurationMs:        duration.Milliseconds(),
		ExitCodeAvailable: true,
		Stdout:            truncateBytes(stdout.Bytes(), maxStdoutBytes),
		Stderr:            truncateBytes(stderr.Bytes(), maxStderrBytes),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			return result, &PolicyError{Code: ADB_TIMEOUT, Message: "adb execution timed out"}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return result, &PolicyError{Code: ADB_EXECUTION_FAILED, Message: err.Error()}
	}

	result.ExitCode = 0
	return result, nil
}

func truncateBytes(data []byte, maxBytes int64) string {
	if int64(len(data)) > maxBytes {
		return string(data[:maxBytes])
	}
	return string(data)
}
