package domain

import "strings"

// ControlSinkDeclaration is package-declared metadata for an opaque control
// effect endpoint. GameHost validates identity/authorization but never parses
// game-specific effect payloads.
type ControlSinkDeclaration struct {
	ID          string
	ServiceID   ServiceID
	Kind        string
	Description string
}

func (s ControlSinkDeclaration) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return NewHostError(ErrInvalidArgument, "control sink id must not be empty")
	}
	if s.ServiceID == "" {
		return NewHostError(ErrInvalidArgument, "control sink service id must not be empty")
	}
	if strings.TrimSpace(s.Kind) != "effect" {
		return NewHostError(ErrInvalidArgument, "control sink kind must be effect")
	}
	if strings.ContainsAny(s.ID, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "control sink id contains control characters")
	}
	return nil
}
