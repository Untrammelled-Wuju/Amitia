package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	maxStorageKeyLength = 128
	hashHexLength       = 16
)

const (
	pluginKeyPrefix  = "plg"
	runtimeKeyPrefix = "run"
	serviceKeyPrefix = "svc"
)

type StorageKey string

func (k StorageKey) String() string {
	return string(k)
}

func (k StorageKey) IsEmpty() bool {
	return string(k) == ""
}

func StorageKeyForPluginID(pluginID domain.PluginID) (StorageKey, error) {
	if pluginID == "" {
		return "", domain.NewHostError(domain.ErrInvalidArgument, "plugin id must not be empty")
	}
	return generateStorageKey(pluginKeyPrefix, string(pluginID))
}

func StorageKeyForRuntimeID(runtimeID domain.RuntimeInstanceID) (StorageKey, error) {
	if runtimeID == "" {
		return "", domain.NewHostError(domain.ErrInvalidArgument, "runtime instance id must not be empty")
	}
	return generateStorageKey(runtimeKeyPrefix, string(runtimeID))
}

func StorageKeyForServiceID(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (StorageKey, error) {
	if runtimeID == "" {
		return "", domain.NewHostError(domain.ErrInvalidArgument, "runtime instance id must not be empty")
	}
	if serviceID == "" {
		return "", domain.NewHostError(domain.ErrInvalidArgument, "service id must not be empty")
	}
	composite := string(runtimeID) + "/" + string(serviceID)
	return generateStorageKey(serviceKeyPrefix, composite)
}

func generateStorageKey(prefix, id string) (StorageKey, error) {
	normalized := normalizeID(id)
	if normalized == "" {
		return "", domain.NewHostError(domain.ErrInvalidArgument, "id produces empty storage key")
	}

	hash := sha256.Sum256([]byte(normalized))
	hashHex := hex.EncodeToString(hash[:])[:hashHexLength]

	readable := buildReadableSegment(id)
	maxReadable := maxStorageKeyLength - len(prefix) - len(hashHex) - 2
	if maxReadable < 0 {
		maxReadable = 0
	}
	if len(readable) > maxReadable {
		readable = readable[:maxReadable]
	}

	var builder strings.Builder
	builder.Grow(len(prefix) + len(readable) + len(hashHex) + 2)
	builder.WriteString(prefix)
	builder.WriteString("-")
	builder.WriteString(readable)
	builder.WriteString("-")
	builder.WriteString(hashHex)

	return StorageKey(builder.String()), nil
}

func buildReadableSegment(id string) string {
	if id == "" {
		return "empty"
	}

	var builder strings.Builder
	builder.Grow(len(id))

	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune('-')
		default:
			builder.WriteRune('-')
		}
	}

	result := builder.String()
	result = strings.Trim(result, "-")
	result = collapseDashes(result)

	if result == "" {
		return "x"
	}
	return result
}

func collapseDashes(s string) string {
	if !strings.Contains(s, "--") {
		return s
	}

	var builder strings.Builder
	builder.Grow(len(s))
	prevDash := false
	for _, r := range s {
		if r == '-' {
			if !prevDash {
				builder.WriteRune(r)
				prevDash = true
			}
			continue
		}
		builder.WriteRune(r)
		prevDash = false
	}
	return builder.String()
}

func normalizeID(id string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(id)
	}
	return id
}

func ValidateStorageKey(key StorageKey) error {
	if key.IsEmpty() {
		return domain.NewHostError(domain.ErrInvalidArgument, "storage key must not be empty")
	}
	raw := string(key)
	if filepath.IsAbs(raw) {
		return domain.NewHostError(domain.ErrInvalidArgument, "storage key must not be absolute path")
	}
	if strings.ContainsAny(raw, "/\\:*\x00") {
		return domain.NewHostError(domain.ErrInvalidArgument, "storage key contains invalid characters")
	}
	if strings.Contains(raw, "..") {
		return domain.NewHostError(domain.ErrInvalidArgument, "storage key must not contain path traversal")
	}
	if len(raw) > maxStorageKeyLength {
		return domain.NewHostError(domain.ErrInvalidArgument, "storage key exceeds maximum length")
	}
	return nil
}
