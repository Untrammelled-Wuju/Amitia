package adb

import (
	"testing"
)

func TestParseDevicesOutput_SingleDevice(t *testing.T) {
	output := `List of devices attached
ABC123	device
`
	devices := parseDevicesOutput(output, "")
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Serial != "ABC123" {
		t.Fatalf("expected serial ABC123, got %s", devices[0].Serial)
	}
	if devices[0].State != DeviceStateDevice {
		t.Fatalf("expected state device, got %s", devices[0].State)
	}
}

func TestParseDevicesOutput_MultipleDevices(t *testing.T) {
	output := `List of devices attached
ABC123	device
DEF456	offline
GHI789	unauthorized
`
	devices := parseDevicesOutput(output, "")
	if len(devices) != 3 {
		t.Fatalf("expected 3 devices, got %d", len(devices))
	}
	if devices[0].State != DeviceStateDevice {
		t.Fatalf("device 0 expected device state, got %s", devices[0].State)
	}
	if devices[1].State != DeviceStateOffline {
		t.Fatalf("device 1 expected offline state, got %s", devices[1].State)
	}
	if devices[2].State != DeviceStateUnauthorized {
		t.Fatalf("device 2 expected unauthorized state, got %s", devices[2].State)
	}
}

func TestParseDevicesOutput_WithDetails(t *testing.T) {
	output := `List of devices attached
192.168.1.100:5555	device	product:walleye	model:Pixel2
`
	devices := parseDevicesOutput(output, "192.168.1.100:5555")
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Serial != "192.168.1.100:5555" {
		t.Fatalf("expected serial 192.168.1.100:5555, got %s", devices[0].Serial)
	}
	if devices[0].Product != "walleye" {
		t.Fatalf("expected product walleye, got %s", devices[0].Product)
	}
	if devices[0].Model != "Pixel2" {
		t.Fatalf("expected model Pixel2, got %s", devices[0].Model)
	}
	if !devices[0].IsDefault {
		t.Fatalf("expected IsDefault=true for default device")
	}
	if devices[0].Transport != TransportNetwork {
		t.Fatalf("expected transport network, got %s", devices[0].Transport)
	}
}

func TestParseDevicesOutput_SerialValidation(t *testing.T) {
	output := "List of devices attached\nABC\tdevice\n"
	devices := parseDevicesOutput(output, "")
	if len(devices) != 1 {
		t.Fatalf("expected 1 valid device, got %d", len(devices))
	}
}

func TestParseDevicesOutput_InvalidSerial(t *testing.T) {
	longSerial := make([]byte, 300)
	for i := range longSerial {
		longSerial[i] = 'A'
	}
	output := string(longSerial) + "\tdevice\n"
	output = "List of devices attached\n" + output
	devices := parseDevicesOutput(output, "")
	if len(devices) != 0 {
		t.Fatalf("expected 0 valid devices (long serial rejected), got %d", len(devices))
	}
}

func TestParseDevicesOutput_Empty(t *testing.T) {
	output := "List of devices attached\n"
	devices := parseDevicesOutput(output, "")
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices from empty output, got %d", len(devices))
	}
}

func TestParseDevicesOutput_WithDaemonLines(t *testing.T) {
	output := `* daemon not running. starting it now on port 5037 *
* daemon started successfully *
List of devices attached
ABC123	device
`
	devices := parseDevicesOutput(output, "")
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Serial != "ABC123" {
		t.Fatalf("expected serial ABC123, got %s", devices[0].Serial)
	}
}

func TestMapDeviceState(t *testing.T) {
	tests := map[string]string{
		"device":        DeviceStateDevice,
		"offline":       DeviceStateOffline,
		"unauthorized":  DeviceStateUnauthorized,
		"no permissions": DeviceStateNoPermissions,
		"unknown":       DeviceStateUnknown,
		"":              DeviceStateUnknown,
	}
	for raw, expected := range tests {
		actual := mapDeviceState(raw)
		if actual != expected {
			t.Errorf("mapDeviceState(%q) = %q, expected %q", raw, actual, expected)
		}
	}
}

func TestIsValidSerial(t *testing.T) {
	tests := map[string]bool{
		"ABC-123_456": true,
		"":             false,
		"192.168.1.1:5555": true,
	}
	for serial, expected := range tests {
		actual := isValidSerial(serial)
		if actual != expected {
			t.Errorf("isValidSerial(%q) = %v, expected %v", serial, actual, expected)
		}
	}
}

func TestSanitizeDeviceField(t *testing.T) {
	input := "Pixel 2\x00\x01\x02 with special chars"
	sanitized := sanitizeDeviceField(input)
	if sanitized != "Pixel 2 with special chars" {
		t.Errorf("sanitizeDeviceField returned %q, expected only printable chars", sanitized)
	}
}
