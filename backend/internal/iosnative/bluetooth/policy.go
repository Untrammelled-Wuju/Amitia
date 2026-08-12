package bluetooth

import (
	"fmt"
	"regexp"
)

const (
	DefaultScanDurationMs = 10000
	MaxScanDurationMs     = 30000
	DefaultMaxResults     = 50
	MaxResultsLimit       = 200
	MaxScanTTLSeconds     = 120
	MaxWriteBytes         = 512
	DefaultConnectTimeoutMs = 15000
	MaxConnectTimeoutMs   = 60000
	MinConnectTimeoutMs   = 1000
	MaxConcurrentConnectionsDefault = 4
	DefaultEventRateLimit = 10
)

var uuidRegex = regexp.MustCompile(`(?i)^[0-9a-f]{4}(?:[0-9a-f]{4})?(?:[0-9a-f]{12})?$|^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

func ClampScanDuration(ms int) int {
	if ms <= 0 {
		return DefaultScanDurationMs
	}
	if ms > MaxScanDurationMs {
		return MaxScanDurationMs
	}
	return ms
}

func ClampMaxResults(n int) int {
	if n <= 0 {
		return DefaultMaxResults
	}
	if n > MaxResultsLimit {
		return MaxResultsLimit
	}
	return n
}

func ClampConnectTimeout(ms int) int {
	if ms <= 0 {
		return DefaultConnectTimeoutMs
	}
	if ms > MaxConnectTimeoutMs {
		return MaxConnectTimeoutMs
	}
	if ms < MinConnectTimeoutMs {
		return MinConnectTimeoutMs
	}
	return ms
}

func NormalizeUUID(s string) string {
	if !IsValidUUID(s) {
		return ""
	}
	if len(s) == 4 {
		return "0000" + s + "-0000-1000-8000-00805f9b34fb"
	}
	if len(s) == 8 {
		return s + "-0000-1000-8000-00805f9b34fb"
	}
	return s
}

func ValidateScanRequest(req BluetoothScanRequest) error {
	if req.DurationMs > MaxScanDurationMs {
		return fmt.Errorf("%v: duration %d exceeds max %d", ErrInvalidScanRequest, req.DurationMs, MaxScanDurationMs)
	}
	if req.MaxResults < 0 {
		return fmt.Errorf("%v: maxResults cannot be negative", ErrInvalidScanRequest)
	}
	for _, uuid := range req.ServiceUUIDs {
		if !IsValidUUID(uuid) {
			return fmt.Errorf("%v: invalid service UUID %q", ErrInvalidServiceUUID, uuid)
		}
	}
	return nil
}

func ValidateConnectRequest(req BluetoothConnectRequest) error {
	if req.PeripheralID == "" {
		return fmt.Errorf("%v: peripheralId is required", ErrInvalidPeripheralID)
	}
	if req.TimeoutMs > MaxConnectTimeoutMs {
		return fmt.Errorf("%v: timeout %d exceeds max %d", ErrConnectionTimeout, req.TimeoutMs, MaxConnectTimeoutMs)
	}
	for _, uuid := range req.ExpectedServiceUUIDs {
		if !IsValidUUID(uuid) {
			return fmt.Errorf("%v: invalid service UUID %q", ErrInvalidServiceUUID, uuid)
		}
	}
	return nil
}

func ValidateWriteRequest(req BluetoothWriteRequest) error {
	if req.CharacteristicRef == "" {
		return fmt.Errorf("%v: characteristicRef is required", ErrInvalidCharacteristicUUID)
	}
	if req.Mode != "with_response" && req.Mode != "without_response" {
		return fmt.Errorf("%v: invalid write mode %q", ErrWriteValueInvalid, req.Mode)
	}
	if req.Value.Base64 == "" && req.Value.Hex == "" {
		return fmt.Errorf("%v: value must provide base64 or hex", ErrWriteValueInvalid)
	}
	return nil
}

func ValidateUUID(uuid string) error {
	if !IsValidUUID(uuid) {
		return fmt.Errorf("%v: %q", ErrInvalidCharacteristicUUID, uuid)
	}
	return nil
}
