package binary

import (
	"encoding/json"
	"strings"
)

type BinaryStorageKind string

const (
	BinaryStorageFile         BinaryStorageKind = "file"
	BinaryStorageSharedMemory BinaryStorageKind = "shared_memory"
)

func (k BinaryStorageKind) Validate() error {
	switch k {
	case BinaryStorageFile, BinaryStorageSharedMemory:
		return nil
	default:
		return ErrKindUnknown
	}
}

type BinaryLifetime string

const (
	BinaryLifetimeMessage BinaryLifetime = "message"
	BinaryLifetimeRuntime BinaryLifetime = "runtime"
)

func (l BinaryLifetime) Validate() error {
	switch l {
	case BinaryLifetimeMessage, BinaryLifetimeRuntime:
		return nil
	default:
		return ErrLifetimeUnknown
	}
}

type Checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

func (c *Checksum) Validate() error {
	if c.Algorithm == "" {
		return ErrChecksumInvalid
	}
	c.Algorithm = strings.ToLower(c.Algorithm)
	if c.Algorithm != "sha256" {
		return ErrChecksumInvalid
	}
	value := strings.TrimSpace(c.Value)
	if len(value) != 64 {
		return ErrChecksumInvalid
	}
	for _, ch := range value {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return ErrChecksumInvalid
		}
	}
	c.Value = strings.ToLower(value)
	return nil
}

type BinaryReference struct {
	ID        BinaryObjectID             `json:"id"`
	Kind      BinaryStorageKind          `json:"kind"`
	Size      int64                      `json:"size"`
	MediaType string                     `json:"mediaType,omitempty"`
	Checksum  *Checksum                  `json:"checksum,omitempty"`
	Lifetime  BinaryLifetime             `json:"lifetime"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

const (
	maxMediaTypeLength = 256
	maxMetadataBytes   = 1 << 20
)

func (r *BinaryReference) Validate() error {
	if err := ValidateBinaryObjectID(r.ID); err != nil {
		return err
	}
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if r.Size < 0 {
		return ErrSizeNegative
	}
	if len(r.MediaType) > maxMediaTypeLength {
		return ErrMediaTypeTooLong
	}
	if r.Checksum != nil {
		if err := r.Checksum.Validate(); err != nil {
			return err
		}
	}
	if err := r.Lifetime.Validate(); err != nil {
		return err
	}
	if r.Metadata != nil {
		total := 0
		for k, v := range r.Metadata {
			total += len(k) + len(v)
		}
		if total > maxMetadataBytes {
			return ErrMetadataTooLarge
		}
	}
	return nil
}
