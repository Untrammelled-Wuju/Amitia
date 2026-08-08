package domain

import "strings"

const (
	maxMetadataKeyLength   = 128
	maxMetadataValueLength = 1024
)

func validateMetadata(metadata map[string]string) error {
	for key, value := range metadata {
		if key == "" {
			return NewHostError(ErrInvalidArgument, "metadata key must not be empty")
		}
		if len(key) > maxMetadataKeyLength {
			return NewHostError(ErrInvalidArgument, "metadata key exceeds maximum length")
		}
		if len(value) > maxMetadataValueLength {
			return NewHostError(ErrInvalidArgument, "metadata value exceeds maximum length")
		}
		if strings.ContainsAny(key, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
			return NewHostError(ErrInvalidArgument, "metadata key contains control characters")
		}
	}
	return nil
}
