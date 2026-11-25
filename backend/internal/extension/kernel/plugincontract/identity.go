package plugincontract

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/provideridentity"
)

const (
	identityMaxLength   = 190
	identityMaxSegments = 5
	identityMaxSegment  = 64
)

var ErrInvalidIdentity = errors.New("plugincontract: invalid identity")

type IdentityKind string

const (
	IdentityKindService    IdentityKind = "service"
	IdentityKindProvider   IdentityKind = "provider"
	IdentityKindInstance   IdentityKind = "instance"
	IdentityKindCapability IdentityKind = "capability"
)

func (k IdentityKind) IsValid() bool {
	switch k {
	case IdentityKindService, IdentityKindProvider, IdentityKindInstance, IdentityKindCapability:
		return true
	}
	return false
}

type IdentityRules struct {
	MinSegments int
	MaxSegments int
}

var identityRules = map[IdentityKind]IdentityRules{
	IdentityKindService:    {MinSegments: 1, MaxSegments: 3},
	IdentityKindProvider:   {MinSegments: 1, MaxSegments: 4},
	IdentityKindInstance:   {MinSegments: 2, MaxSegments: 5},
	IdentityKindCapability: {MinSegments: 3, MaxSegments: 4},
}

type IdentityError struct {
	Kind  IdentityKind
	Raw   string
	Rule  string
	Cause error
}

func (e *IdentityError) Error() string {
	base := fmt.Sprintf("plugincontract: invalid %s identity %q: %s", e.Kind, e.Raw, e.Rule)
	if e.Cause != nil {
		return base + ": " + e.Cause.Error()
	}
	return base
}

func (e *IdentityError) Unwrap() []error {
	if e.Cause != nil {
		return []error{ErrInvalidIdentity, e.Cause}
	}
	return []error{ErrInvalidIdentity}
}

type Identity struct {
	kind     IdentityKind
	segments []string
}

func (i Identity) Kind() IdentityKind {
	return i.kind
}

func (i Identity) Segments() []string {
	if len(i.segments) == 0 {
		return nil
	}
	out := make([]string, len(i.segments))
	copy(out, i.segments)
	return out
}

func (i Identity) String() string {
	return strings.Join(i.segments, "/")
}

func (i Identity) IsZero() bool {
	return len(i.segments) == 0
}

func ParseIdentity(kind IdentityKind, raw string) (Identity, error) {
	if !kind.IsValid() {
		return Identity{}, &IdentityError{Kind: kind, Raw: raw, Rule: "unknown identity kind"}
	}
	rules := identityRules[kind]
	if raw == "" {
		return Identity{}, &IdentityError{Kind: kind, Raw: raw, Rule: "empty"}
	}
	if strings.TrimSpace(raw) != raw {
		return Identity{}, &IdentityError{Kind: kind, Raw: raw, Rule: "surrounding whitespace"}
	}
	if len(raw) > identityMaxLength {
		return Identity{}, &IdentityError{Kind: kind, Raw: raw, Rule: fmt.Sprintf("longer than %d bytes", identityMaxLength)}
	}
	segments := strings.Split(raw, "/")
	if len(segments) > identityMaxSegments {
		return Identity{}, &IdentityError{Kind: kind, Raw: raw, Rule: fmt.Sprintf("more than %d segments", identityMaxSegments)}
	}
	if len(segments) < rules.MinSegments || len(segments) > rules.MaxSegments {
		return Identity{}, &IdentityError{Kind: kind, Raw: raw, Rule: fmt.Sprintf("segment count must be within [%d,%d]", rules.MinSegments, rules.MaxSegments)}
	}
	for _, segment := range segments {
		if err := validateIdentitySegment(segment); err != nil {
			return Identity{}, &IdentityError{Kind: kind, Raw: raw, Rule: "segment " + segment, Cause: err}
		}
	}
	out := make([]string, len(segments))
	copy(out, segments)
	return Identity{kind: kind, segments: out}, nil
}

func validateIdentitySegment(segment string) error {
	if segment == "" {
		return errors.New("empty segment")
	}
	if len(segment) > identityMaxSegment {
		return fmt.Errorf("longer than %d bytes", identityMaxSegment)
	}
	for index := 0; index < len(segment); index++ {
		char := segment[index]
		switch {
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9':
		case char == '.', char == '-', char == '_':
			if index == 0 || index == len(segment)-1 {
				return fmt.Errorf("separator %q at segment edge", string(char))
			}
			if isIdentitySeparator(segment[index-1]) {
				return fmt.Errorf("consecutive separators at offset %d", index)
			}
		default:
			return fmt.Errorf("unsupported character %q (lowercase ascii required)", string(char))
		}
	}
	return nil
}

func isIdentitySeparator(char byte) bool {
	return char == '.' || char == '-' || char == '_'
}

func ValidateIdentity(kind IdentityKind, raw string) error {
	_, err := ParseIdentity(kind, raw)
	return err
}

func ValidateServiceID(raw string) error {
	return ValidateIdentity(IdentityKindService, raw)
}

func ValidateProviderID(raw string) error {
	return ValidateIdentity(IdentityKindProvider, raw)
}

func ValidateInstanceID(raw string) error {
	return ValidateIdentity(IdentityKindInstance, raw)
}

func ValidateCapabilityIdentity(raw string) error {
	return ValidateIdentity(IdentityKindCapability, raw)
}

func BuildServiceID(namespace, name string) (string, error) {
	candidate := strings.ToLower(strings.TrimSpace(namespace)) + "." + strings.ToLower(strings.TrimSpace(name))
	if _, err := ParseIdentity(IdentityKindService, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func composeProviderID(module ModuleRef, serviceID string) string {
	return provideridentity.CatalogProviderKey(module.PluginID, module.ModuleID, serviceID)
}

func BuildProviderID(module ModuleRef, serviceID string) (string, error) {
	trimmedService := strings.TrimSpace(serviceID)
	if err := ValidateServiceID(trimmedService); err != nil {
		return "", err
	}
	candidate := composeProviderID(module, trimmedService)
	if _, err := ParseIdentity(IdentityKindProvider, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func BuildInstanceID(providerID, instance string) (string, error) {
	trimmedProvider := strings.TrimSpace(providerID)
	if err := ValidateProviderID(trimmedProvider); err != nil {
		return "", err
	}
	candidate := trimmedProvider + "/" + strings.ToLower(strings.TrimSpace(instance))
	if _, err := ParseIdentity(IdentityKindInstance, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

type DuplicateIdentityError struct {
	Kind       IdentityKind
	Duplicates []string
}

func (e *DuplicateIdentityError) Error() string {
	return fmt.Sprintf("plugincontract: duplicated %s identity: %s", e.Kind, strings.Join(e.Duplicates, ", "))
}

func (e *DuplicateIdentityError) Unwrap() []error {
	return []error{ErrInvalidIdentity}
}

func DetectDuplicateIdentities(kind IdentityKind, values []string) error {
	seen := make(map[string]int, len(values))
	duplicates := make([]string, 0)
	for _, value := range values {
		key := strings.TrimSpace(value)
		seen[key]++
		if seen[key] == 2 {
			duplicates = append(duplicates, key)
		}
	}
	if len(duplicates) == 0 {
		return nil
	}
	sort.Strings(duplicates)
	return &DuplicateIdentityError{Kind: kind, Duplicates: duplicates}
}
