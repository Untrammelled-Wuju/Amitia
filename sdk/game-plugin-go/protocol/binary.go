package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

type BinaryStorageKind string

const (
	// BinaryStorageFile is the protocol v1 production transport. Plugins upload
	// chunks through binary.write; the host owns the backing file and never
	// exposes an ambient filesystem path to the plugin.
	BinaryStorageFile BinaryStorageKind = "file"
)

type BinaryLifetime string

const (
	// BinaryLifetimeMessage is automatically expired by the host after a bounded
	// TTL. It is appropriate for frames, audio packets and other transient data.
	BinaryLifetimeMessage BinaryLifetime = "message"
	// BinaryLifetimeRuntime remains available until explicit release or runtime
	// teardown. It is appropriate for reusable maps/models/buffers.
	BinaryLifetimeRuntime BinaryLifetime = "runtime"
)

type BinaryChecksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type BinaryReference struct {
	ID        string                     `json:"id"`
	Kind      BinaryStorageKind          `json:"kind"`
	Size      int64                      `json:"size"`
	MediaType string                     `json:"mediaType,omitempty"`
	Checksum  *BinaryChecksum            `json:"checksum,omitempty"`
	Lifetime  BinaryLifetime             `json:"lifetime"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (r BinaryReference) Validate() error {
	if !validBinaryReferenceID(r.ID) {
		return fmt.Errorf("binary reference id is invalid")
	}
	if r.Kind != BinaryStorageFile {
		return fmt.Errorf("binary storage kind %q is not supported by %s", r.Kind, ProtocolVersion)
	}
	if r.Size < 0 {
		return fmt.Errorf("binary reference size must not be negative")
	}
	if len(r.MediaType) > 256 {
		return fmt.Errorf("binary media type exceeds 256 bytes")
	}
	if r.Lifetime != BinaryLifetimeMessage && r.Lifetime != BinaryLifetimeRuntime {
		return fmt.Errorf("binary lifetime %q is invalid", r.Lifetime)
	}
	if r.Checksum != nil {
		value := strings.TrimSpace(r.Checksum.Value)
		if !strings.EqualFold(strings.TrimSpace(r.Checksum.Algorithm), "sha256") || len(value) != 64 {
			return fmt.Errorf("binary checksum is invalid")
		}
		for _, ch := range value {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return fmt.Errorf("binary checksum is invalid")
			}
		}
	}
	if r.Metadata != nil {
		total := 0
		for key, value := range r.Metadata {
			total += len(key) + len(value)
		}
		if total > 1<<20 {
			return fmt.Errorf("binary metadata exceeds 1 MiB")
		}
	}
	return nil
}

type BinaryFrameHeader struct {
	Protocol   string `json:"protocol"`
	ID         string `json:"id"`
	RuntimeID  string `json:"runtimeId,omitempty"`
	PluginID   string `json:"pluginId,omitempty"`
	ServiceID  string `json:"serviceId,omitempty"`
	Generation uint64 `json:"generation,omitempty"`
	ObjectID   string `json:"objectId"`
	Offset     int64  `json:"offset"`
}

func (h BinaryFrameHeader) Validate() error {
	if h.Protocol != ProtocolVersion {
		return fmt.Errorf("binary frame protocol %q is invalid", h.Protocol)
	}
	if err := ValidateMessageID(h.ID); err != nil {
		return fmt.Errorf("binary frame id is invalid: %w", err)
	}
	if !validBinaryReferenceID(h.ObjectID) {
		return fmt.Errorf("binary frame object id is invalid")
	}
	if h.Offset < 0 {
		return fmt.Errorf("binary frame offset must not be negative")
	}
	return nil
}

func validBinaryReferenceID(id string) bool {
	id = strings.TrimSpace(id)
	if len(id) < 5 || len(id) > 512 || !strings.HasPrefix(id, "bin_") {
		return false
	}
	for _, ch := range id[4:] {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || ch == '-') {
			return false
		}
	}
	return true
}
