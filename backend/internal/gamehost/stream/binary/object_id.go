package binary

import (
	"strings"

	"github.com/google/uuid"
)

type BinaryObjectID string

func NewBinaryObjectID() BinaryObjectID {
	return BinaryObjectID("bin_" + uuid.New().String())
}

func (id BinaryObjectID) String() string {
	return string(id)
}

func (id BinaryObjectID) IsEmpty() bool {
	return string(id) == ""
}

func ValidateBinaryObjectID(id BinaryObjectID) error {
	if id.IsEmpty() {
		return ErrIDFormat
	}
	s := string(id)
	if len(s) < 5 || len(s) > 512 {
		return ErrIDFormat
	}
	if !strings.HasPrefix(s, "bin_") {
		return ErrIDFormat
	}
	core := s[4:]
	if len(core) == 0 {
		return ErrIDFormat
	}
	for _, r := range core {
		if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9') || r == '-') {
			return ErrIDFormat
		}
	}
	if strings.ContainsAny(s, "/\\") {
		return ErrIDFormat
	}
	return nil
}
