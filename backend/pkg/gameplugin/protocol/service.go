package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type ServiceID string
type ServiceKind string

const (
	ServiceKindProcess ServiceKind = "process"
)

type ServiceDescriptor struct {
	ID   ServiceID   `json:"id"`
	Name string      `json:"name,omitempty"`
	Kind ServiceKind `json:"kind"`

	Required bool `json:"required,omitempty"`

	DependsOn []ServiceID `json:"dependsOn,omitempty"`

	Capabilities []Capability `json:"capabilities,omitempty"`

	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

func ValidateServiceID(id ServiceID) error {
	if id == "" {
		return fmt.Errorf("service id must not be empty")
	}
	const maxLength = 256
	if len(id) > maxLength {
		return fmt.Errorf("service id exceeds maximum length of %d", maxLength)
	}
	for _, r := range string(id) {
		if unicode.IsControl(r) {
			return fmt.Errorf("service id contains control character")
		}
		if r == ' ' {
			return fmt.Errorf("service id must not contain spaces")
		}
	}
	return nil
}

func ValidateServiceKind(kind ServiceKind) error {
	switch kind {
	case ServiceKindProcess:
		return nil
	default:
		return fmt.Errorf("invalid service kind: %s", kind)
	}
}

func (s ServiceDescriptor) Validate() error {
	if err := ValidateServiceID(s.ID); err != nil {
		return fmt.Errorf("invalid service id: %w", err)
	}
	if err := ValidateServiceKind(s.Kind); err != nil {
		return err
	}
	for i, dep := range s.DependsOn {
		if dep == s.ID {
			return fmt.Errorf("service '%s' depends on itself", s.ID)
		}
		for j := i + 1; j < len(s.DependsOn); j++ {
			if s.DependsOn[j] == dep {
				return fmt.Errorf("duplicate dependency '%s'", dep)
			}
		}
	}
	seenCaps := make(map[Capability]bool)
	for _, cap := range s.Capabilities {
		if seenCaps[cap] {
			return fmt.Errorf("duplicate capability '%s'", cap)
		}
		seenCaps[cap] = true
	}
	return nil
}

func ValidateServices(services []ServiceDescriptor) error {
	seen := make(map[ServiceID]bool)
	for i := range services {
		if err := services[i].Validate(); err != nil {
			return fmt.Errorf("service[%d]: %w", i, err)
		}
		if seen[services[i].ID] {
			return fmt.Errorf("duplicate service id '%s'", services[i].ID)
		}
		seen[services[i].ID] = true
	}
	return nil
}

func IsServiceIDValid(id ServiceID) bool {
	return ValidateServiceID(id) == nil
}

func NormalizeServiceID(id ServiceID) ServiceID {
	return ServiceID(strings.ToLower(string(id)))
}
