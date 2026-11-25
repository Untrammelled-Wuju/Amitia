package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	DefaultSchemaMediaType = "application/schema+json"
	descriptorSummaryLimit = 280
	descriptorTagLimit     = 32
	descriptorTagsMax      = 16
)

var (
	ErrInvalidDescriptor = errors.New("plugincontract: invalid descriptor")
	ErrUnknownSchemaRef  = errors.New("plugincontract: unknown schema ref")
)

type SchemaRef struct {
	MediaType string `json:"mediaType"`
	Hash      string `json:"hash"`
}

func (r SchemaRef) String() string {
	return r.MediaType + "#" + r.Hash
}

func ParseSchemaRef(raw string) (SchemaRef, error) {
	index := strings.LastIndex(raw, "#")
	if index <= 0 || index == len(raw)-1 {
		return SchemaRef{}, fmt.Errorf("plugincontract: schema ref %q must be <mediaType>#<sha256>", raw)
	}
	ref := SchemaRef{MediaType: raw[:index], Hash: raw[index+1:]}
	if err := ref.Validate(); err != nil {
		return SchemaRef{}, err
	}
	return ref, nil
}

func (r SchemaRef) Validate() error {
	if strings.TrimSpace(r.MediaType) == "" {
		return fmt.Errorf("plugincontract: schema ref media type is required")
	}
	if !hashPattern.MatchString(r.Hash) {
		return fmt.Errorf("plugincontract: schema ref hash %q must be 64 hex characters", r.Hash)
	}
	return nil
}

type SchemaStore struct {
	mu      sync.RWMutex
	schemas map[string]json.RawMessage
}

func NewSchemaStore() *SchemaStore {
	return &SchemaStore{schemas: make(map[string]json.RawMessage)}
}

func (s *SchemaStore) Put(raw []byte) (SchemaRef, error) {
	if s == nil {
		return SchemaRef{}, fmt.Errorf("plugincontract: schema store is nil")
	}
	canonical, err := CanonicalizeJSONValue(raw)
	if err != nil {
		return SchemaRef{}, fmt.Errorf("plugincontract: canonicalize schema: %w", err)
	}
	sum := sha256.Sum256(canonical)
	ref := SchemaRef{MediaType: DefaultSchemaMediaType, Hash: hex.EncodeToString(sum[:])}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.schemas[ref.Hash]; !exists {
		stored := make([]byte, len(canonical))
		copy(stored, canonical)
		s.schemas[ref.Hash] = stored
	}
	return ref, nil
}

func (s *SchemaStore) Resolve(ref SchemaRef) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("plugincontract: schema store is nil")
	}
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	stored, exists := s.schemas[ref.Hash]
	s.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSchemaRef, ref.String())
	}
	out := make([]byte, len(stored))
	copy(out, stored)
	return out, nil
}

func (s *SchemaStore) Count() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.schemas)
}

type DescriptorPermission struct {
	Capability string `json:"capability"`
	Risk       string `json:"risk,omitempty"`
}

type Descriptor struct {
	ID          string                 `json:"id"`
	Summary     string                 `json:"summary"`
	Tags        []string               `json:"tags,omitempty"`
	Scopes      []ServiceScope         `json:"scopes,omitempty"`
	Permissions []DescriptorPermission `json:"permissions,omitempty"`
	Placement   ServicePlacement       `json:"placement"`
	InputRef    *SchemaRef             `json:"inputSchemaRef,omitempty"`
	OutputRef   *SchemaRef             `json:"outputSchemaRef,omitempty"`
}

func (d Descriptor) Validate() error {
	if err := ValidateCapabilityIdentity(strings.TrimSpace(d.ID)); err != nil {
		return &DescriptorError{Field: "id", Cause: err}
	}
	summary := strings.TrimSpace(d.Summary)
	if summary == "" {
		return &DescriptorError{Field: "summary", Reason: "required"}
	}
	if len(summary) > descriptorSummaryLimit {
		return &DescriptorError{Field: "summary", Reason: fmt.Sprintf("longer than %d bytes", descriptorSummaryLimit)}
	}
	if strings.ContainsAny(summary, "\r\n\t") {
		return &DescriptorError{Field: "summary", Reason: "must be single line"}
	}
	if _, err := canonicalScopes(d.Scopes); err != nil {
		return &DescriptorError{Field: "scopes", Cause: err}
	}
	if !d.Placement.Valid() {
		return &DescriptorError{Field: "placement", Reason: fmt.Sprintf("unsupported value %q", d.Placement)}
	}
	if _, err := normalizeDescriptorTags(d.Tags); err != nil {
		return &DescriptorError{Field: "tags", Cause: err}
	}
	for _, permission := range d.Permissions {
		if err := validateDescriptorPermission(permission); err != nil {
			return &DescriptorError{Field: "permissions", Cause: err}
		}
	}
	if d.InputRef != nil {
		if err := d.InputRef.Validate(); err != nil {
			return &DescriptorError{Field: "inputSchemaRef", Cause: err}
		}
	}
	if d.OutputRef != nil {
		if err := d.OutputRef.Validate(); err != nil {
			return &DescriptorError{Field: "outputSchemaRef", Cause: err}
		}
	}
	return nil
}

func validateDescriptorPermission(permission DescriptorPermission) error {
	capability := strings.TrimSpace(permission.Capability)
	if capability == "" {
		return fmt.Errorf("permission capability is required")
	}
	if err := ValidateServiceID(strings.ToLower(capability)); err != nil {
		return fmt.Errorf("permission capability %q: %w", capability, err)
	}
	risk := strings.TrimSpace(permission.Risk)
	if risk == "" {
		return nil
	}
	switch risk {
	case "low", "medium", "high", "critical":
		return nil
	}
	return fmt.Errorf("permission risk %q must be one of low/medium/high/critical", risk)
}

func normalizeDescriptorTags(tags []string) ([]string, error) {
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			return nil, fmt.Errorf("empty tag")
		}
		if len(normalized) > descriptorTagLimit {
			return nil, fmt.Errorf("tag %q longer than %d bytes", normalized, descriptorTagLimit)
		}
		if err := validateIdentitySegment(normalized); err != nil {
			return nil, fmt.Errorf("tag %q: %w", normalized, err)
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	if len(out) > descriptorTagsMax {
		return nil, fmt.Errorf("more than %d tags", descriptorTagsMax)
	}
	sort.Strings(out)
	return out, nil
}

type DescriptorError struct {
	Field  string
	Reason string
	Cause  error
}

func (e *DescriptorError) Error() string {
	base := fmt.Sprintf("plugincontract: invalid descriptor field %s", e.Field)
	if e.Reason != "" {
		base += ": " + e.Reason
	}
	if e.Cause != nil {
		base += ": " + e.Cause.Error()
	}
	return base
}

func (e *DescriptorError) Unwrap() []error {
	if e.Cause != nil {
		return []error{ErrInvalidDescriptor, e.Cause}
	}
	return []error{ErrInvalidDescriptor}
}

type DescriptorIndex struct {
	mu      sync.RWMutex
	schemas *SchemaStore
	entries map[string]Descriptor
}

func NewDescriptorIndex() *DescriptorIndex {
	return &DescriptorIndex{schemas: NewSchemaStore(), entries: make(map[string]Descriptor)}
}

func (i *DescriptorIndex) Schemas() *SchemaStore {
	if i == nil {
		return nil
	}
	return i.schemas
}

func (i *DescriptorIndex) PutSchema(raw []byte) (SchemaRef, error) {
	if i == nil {
		return SchemaRef{}, fmt.Errorf("plugincontract: descriptor index is nil")
	}
	return i.schemas.Put(raw)
}

func (i *DescriptorIndex) Resolve(ref SchemaRef) ([]byte, error) {
	if i == nil {
		return nil, fmt.Errorf("plugincontract: descriptor index is nil")
	}
	return i.schemas.Resolve(ref)
}

func (i *DescriptorIndex) Add(descriptor Descriptor) (Descriptor, error) {
	if i == nil {
		return Descriptor{}, fmt.Errorf("plugincontract: descriptor index is nil")
	}
	normalized, err := i.normalize(descriptor)
	if err != nil {
		return Descriptor{}, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, exists := i.entries[normalized.ID]; exists {
		return Descriptor{}, &DescriptorError{Field: "id", Reason: fmt.Sprintf("duplicate descriptor %s", normalized.ID)}
	}
	i.entries[normalized.ID] = normalized
	return normalized, nil
}

func (i *DescriptorIndex) Replace(descriptor Descriptor) (Descriptor, error) {
	if i == nil {
		return Descriptor{}, fmt.Errorf("plugincontract: descriptor index is nil")
	}
	normalized, err := i.normalize(descriptor)
	if err != nil {
		return Descriptor{}, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.entries[normalized.ID] = normalized
	return normalized, nil
}

func (i *DescriptorIndex) normalize(descriptor Descriptor) (Descriptor, error) {
	if err := descriptor.Validate(); err != nil {
		return Descriptor{}, err
	}
	scopes, err := canonicalScopes(descriptor.Scopes)
	if err != nil {
		return Descriptor{}, &DescriptorError{Field: "scopes", Cause: err}
	}
	tags, err := normalizeDescriptorTags(descriptor.Tags)
	if err != nil {
		return Descriptor{}, &DescriptorError{Field: "tags", Cause: err}
	}
	permissions := make([]DescriptorPermission, 0, len(descriptor.Permissions))
	seen := make(map[string]bool, len(descriptor.Permissions))
	for _, permission := range descriptor.Permissions {
		item := DescriptorPermission{
			Capability: strings.ToLower(strings.TrimSpace(permission.Capability)),
			Risk:       strings.ToLower(strings.TrimSpace(permission.Risk)),
		}
		key := item.Capability + "\x00" + item.Risk
		if seen[key] {
			continue
		}
		seen[key] = true
		permissions = append(permissions, item)
	}
	sort.Slice(permissions, func(left, right int) bool {
		if permissions[left].Capability != permissions[right].Capability {
			return permissions[left].Capability < permissions[right].Capability
		}
		return permissions[left].Risk < permissions[right].Risk
	})
	if descriptor.InputRef != nil {
		if _, err := i.schemas.Resolve(*descriptor.InputRef); err != nil {
			return Descriptor{}, &DescriptorError{Field: "inputSchemaRef", Cause: err}
		}
	}
	if descriptor.OutputRef != nil {
		if _, err := i.schemas.Resolve(*descriptor.OutputRef); err != nil {
			return Descriptor{}, &DescriptorError{Field: "outputSchemaRef", Cause: err}
		}
	}
	return Descriptor{
		ID:          strings.TrimSpace(descriptor.ID),
		Summary:     strings.TrimSpace(descriptor.Summary),
		Tags:        tags,
		Scopes:      scopes,
		Permissions: permissions,
		Placement:   descriptor.Placement,
		InputRef:    descriptor.InputRef,
		OutputRef:   descriptor.OutputRef,
	}, nil
}

func (i *DescriptorIndex) Snapshot() []Descriptor {
	if i == nil {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make([]Descriptor, 0, len(i.entries))
	for _, entry := range i.entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(left, right int) bool { return out[left].ID < out[right].ID })
	return out
}

func (i *DescriptorIndex) Get(id string) (Descriptor, bool) {
	if i == nil {
		return Descriptor{}, false
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	entry, exists := i.entries[strings.TrimSpace(id)]
	return entry, exists
}

func (i *DescriptorIndex) Remove(id string) bool {
	if i == nil {
		return false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	key := strings.TrimSpace(id)
	if _, exists := i.entries[key]; !exists {
		return false
	}
	delete(i.entries, key)
	return true
}

func (i *DescriptorIndex) Len() int {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.entries)
}

func (i *DescriptorIndex) MarshalSnapshot() ([]byte, error) {
	return json.Marshal(i.Snapshot())
}
