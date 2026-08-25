package domain

import (
	"strings"
)

const (
	maxServiceNameLength = 256
	maxServiceIDLength   = 256
)

type ServiceID string

type ServiceKind string

const (
	ServiceKindProcess  ServiceKind = "process"
	ServiceKindExternal ServiceKind = "external"
)

var validServiceKinds = map[ServiceKind]struct{}{
	ServiceKindProcess:  {},
	ServiceKindExternal: {},
}

type ServiceDescriptor struct {
	ID       ServiceID
	Name     string
	Kind     ServiceKind
	Required bool

	DependsOn []ServiceID

	Metadata map[string]string
}

func (s ServiceDescriptor) Validate() error {
	if s.ID == "" {
		return NewHostError(ErrInvalidArgument, "service id must not be empty")
	}
	if len(s.ID) > maxServiceIDLength {
		return NewHostError(ErrInvalidArgument, "service id exceeds maximum length")
	}
	if s.Name == "" {
		return NewHostError(ErrInvalidArgument, "service name must not be empty")
	}
	if len(s.Name) > maxServiceNameLength {
		return NewHostError(ErrInvalidArgument, "service name exceeds maximum length")
	}
	if _, ok := validServiceKinds[s.Kind]; !ok {
		return NewHostErrorWithCause(ErrInvalidArgument, "invalid service kind",
			NewHostError(ErrInvalidArgument, string(s.Kind)))
	}
	if strings.ContainsAny(string(s.ID), "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "service id contains control characters")
	}
	if strings.ContainsRune(string(s.ID), ' ') {
		return NewHostError(ErrInvalidArgument, "service id must not contain spaces")
	}
	if strings.ContainsAny(s.Name, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "service name contains control characters")
	}

	for _, depID := range s.DependsOn {
		if depID == "" {
			return NewHostError(ErrInvalidArgument, "service dependency id must not be empty")
		}
		if len(depID) > maxServiceIDLength {
			return NewHostError(ErrInvalidArgument, "service dependency id exceeds maximum length")
		}
		if strings.ContainsAny(string(depID), "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") || strings.ContainsRune(string(depID), ' ') {
			return NewHostError(ErrInvalidArgument, "service dependency id is invalid")
		}
		if depID == s.ID {
			return NewHostError(ErrInvalidArgument, "service cannot depend on itself")
		}
	}

	if s.Metadata != nil {
		if err := validateMetadata(s.Metadata); err != nil {
			return NewHostErrorWithCause(ErrInvalidArgument, "service metadata validation failed", err)
		}
	}

	return nil
}

func IsValidServiceKind(kind ServiceKind) bool {
	_, ok := validServiceKinds[kind]
	return ok
}
