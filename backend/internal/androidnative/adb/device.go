package adb

import (
	"regexp"
	"strings"
)

var headerLinePattern = regexp.MustCompile(`(?i)^List of devices attached`)
var daemonLinePattern = regexp.MustCompile(`(?i)^(\* daemon (?:started|not running)|.+daemon .+ successfully\s*$)`)

func parseDevicesOutput(output string, defaultDevice string) []ADBDevice {
	lines := strings.Split(output, "\n")
	var devices []ADBDevice

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if headerLinePattern.MatchString(line) {
			continue
		}
		if daemonLinePattern.MatchString(line) {
			continue
		}

		device := parseDeviceLine(line, defaultDevice)
		if device != nil {
			devices = append(devices, *device)
		}
	}
	return devices
}

func parseDeviceLine(line string, defaultDevice string) *ADBDevice {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	serial := parts[0]
	stateStr := strings.ToLower(parts[1])

	if !isValidSerial(serial) {
		return nil
	}

	device := &ADBDevice{
		Serial:    serial,
		State:     mapDeviceState(stateStr),
		Transport: TransportUnknown,
		IsDefault: serial == defaultDevice,
	}

	for i := 2; i < len(parts); i++ {
		key, value := splitFirstColon(parts[i])
		switch key {
		case "product":
			device.Product = value
		case "model":
			device.Model = sanitizeDeviceField(value)
		case "device":
			device.Device = value
		case "transport_id":
			device.Transport = TransportUSB
		}
	}

	if strings.Contains(serial, ":") {
		device.Transport = TransportNetwork
	}

	return device
}

func splitFirstColon(s string) (string, string) {
	for i, r := range s {
		if r == ':' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func isValidSerial(serial string) bool {
	if len(serial) == 0 || len(serial) > maxSerialLength {
		return false
	}
	for _, r := range serial {
		if r == 0 || r == '\n' || r == '\r' || r == '\t' {
			return false
		}
		if r >= 0x00 && r <= 0x1F && r != 0x20 {
			return false
		}
	}
	return true
}

func sanitizeDeviceField(s string) string {
	if len(s) > 128 {
		s = s[:128]
	}
	var result strings.Builder
	for _, r := range s {
		if r >= 0x20 && r <= 0x7E {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func mapDeviceState(rawState string) string {
	switch rawState {
	case "device":
		return DeviceStateDevice
	case "offline":
		return DeviceStateOffline
	case "unauthorized":
		return DeviceStateUnauthorized
	case "no permissions":
		return DeviceStateNoPermissions
	default:
		return DeviceStateUnknown
	}
}
