package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

const (
	CoreProviderPluginID  = "builtin.core"
	CoreProviderModuleID  = "core"
	CoreDescriptorVersion = "1.0.0"
)

var (
	ErrUnsupportedSchemaType = errors.New("plugincontract: unsupported go type for schema")
	ErrInvalidCoreDescriptor = errors.New("plugincontract: invalid core descriptor")
)

type CoreDescriptorError struct {
	ServiceID string
	Reason    string
	Cause     error
}

func (e *CoreDescriptorError) Error() string {
	base := "plugincontract: invalid core descriptor"
	if e.ServiceID != "" {
		base += " for service " + e.ServiceID
	}
	if e.Reason != "" {
		base += ": " + e.Reason
	}
	if e.Cause != nil {
		base += ": " + e.Cause.Error()
	}
	return base
}

func (e *CoreDescriptorError) Unwrap() []error {
	if e.Cause != nil {
		return []error{ErrInvalidCoreDescriptor, e.Cause}
	}
	return []error{ErrInvalidCoreDescriptor}
}

type CoreCapabilitySpec struct {
	ServiceID   string
	Version     string
	Summary     string
	Tags        []string
	Scopes      []ServiceScope
	Permissions []DescriptorPermission
	Placement   ServicePlacement
	Input       any
	Output      any
}

type CoreDescriptorEntry struct {
	ProviderID   string          `json:"providerId"`
	CapabilityID string          `json:"capabilityId"`
	ServiceID    string          `json:"serviceId"`
	Version      string          `json:"version"`
	ContractHash string          `json:"contractHash"`
	Descriptor   Descriptor      `json:"descriptor"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
}

func (e CoreDescriptorEntry) ResolveSchema(store map[string]json.RawMessage) error {
	if e.Descriptor.InputRef != nil {
		if _, err := resolveSchemaRef(store, *e.Descriptor.InputRef); err != nil {
			return &CoreDescriptorError{ServiceID: e.ServiceID, Reason: "unresolvable input schema", Cause: err}
		}
	}
	if e.Descriptor.OutputRef != nil {
		if _, err := resolveSchemaRef(store, *e.Descriptor.OutputRef); err != nil {
			return &CoreDescriptorError{ServiceID: e.ServiceID, Reason: "unresolvable output schema", Cause: err}
		}
	}
	return nil
}

type CoreDescriptorCatalog struct {
	ProviderModule ModuleRef                  `json:"providerModule"`
	Entries        []CoreDescriptorEntry      `json:"entries"`
	Schemas        map[string]json.RawMessage `json:"schemas"`
}

func (c CoreDescriptorCatalog) Find(serviceID string) (CoreDescriptorEntry, bool) {
	target := strings.TrimSpace(serviceID)
	for _, entry := range c.Entries {
		if entry.ServiceID == target {
			return entry, true
		}
	}
	return CoreDescriptorEntry{}, false
}

func (c CoreDescriptorCatalog) Listing() []Descriptor {
	out := make([]Descriptor, 0, len(c.Entries))
	for _, entry := range c.Entries {
		out = append(out, entry.Descriptor)
	}
	sort.Slice(out, func(left, right int) bool { return out[left].ID < out[right].ID })
	return out
}

func (c CoreDescriptorCatalog) Resolve(ref SchemaRef) ([]byte, error) {
	return resolveSchemaRef(c.Schemas, ref)
}

func (c CoreDescriptorCatalog) Snapshot() ([]byte, error) {
	canonical := CoreDescriptorCatalog{
		ProviderModule: c.ProviderModule,
		Entries:        make([]CoreDescriptorEntry, len(c.Entries)),
		Schemas:        make(map[string]json.RawMessage, len(c.Schemas)),
	}
	copy(canonical.Entries, c.Entries)
	sort.Slice(canonical.Entries, func(left, right int) bool {
		return canonical.Entries[left].ServiceID < canonical.Entries[right].ServiceID
	})
	for hash, schema := range c.Schemas {
		canonical.Schemas[hash] = schema
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: encode core descriptor snapshot: %w", err)
	}
	return encoded, nil
}

func BuildCoreDescriptorCatalog(specs []CoreCapabilitySpec) (CoreDescriptorCatalog, error) {
	catalog := CoreDescriptorCatalog{
		ProviderModule: ModuleRef{PluginID: CoreProviderPluginID, ModuleID: CoreProviderModuleID},
		Entries:        make([]CoreDescriptorEntry, 0, len(specs)),
		Schemas:        make(map[string]json.RawMessage, len(specs)*2),
	}
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		entry, err := buildCoreDescriptorEntry(spec, catalog.Schemas)
		if err != nil {
			return CoreDescriptorCatalog{}, err
		}
		if seen[entry.ServiceID] {
			return CoreDescriptorCatalog{}, &CoreDescriptorError{ServiceID: entry.ServiceID, Reason: "duplicated service id"}
		}
		seen[entry.ServiceID] = true
		catalog.Entries = append(catalog.Entries, entry)
	}
	sort.Slice(catalog.Entries, func(left, right int) bool { return catalog.Entries[left].ServiceID < catalog.Entries[right].ServiceID })
	return catalog, nil
}

func buildCoreDescriptorEntry(spec CoreCapabilitySpec, schemas map[string]json.RawMessage) (CoreDescriptorEntry, error) {
	serviceID := strings.TrimSpace(spec.ServiceID)
	if err := ValidateServiceID(serviceID); err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "invalid service id", Cause: err}
	}
	providerID, err := BuildProviderID(ModuleRef{PluginID: CoreProviderPluginID, ModuleID: CoreProviderModuleID}, serviceID)
	if err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "invalid provider id", Cause: err}
	}
	capabilityID := "builtin" + "/" + CoreProviderModuleID + "/" + serviceID
	if err := ValidateCapabilityIdentity(capabilityID); err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "invalid capability id", Cause: err}
	}
	version := strings.TrimSpace(spec.Version)
	if version == "" {
		version = CoreDescriptorVersion
	}
	if spec.Input == nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "input sample is required"}
	}
	if spec.Output == nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "output sample is required"}
	}
	inputSchema, err := SchemaForGoValue(spec.Input)
	if err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "input schema", Cause: err}
	}
	outputSchema, err := SchemaForGoValue(spec.Output)
	if err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "output schema", Cause: err}
	}
	inputRef, err := putCoreSchema(schemas, inputSchema)
	if err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "input schema ref", Cause: err}
	}
	outputRef, err := putCoreSchema(schemas, outputSchema)
	if err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "output schema ref", Cause: err}
	}
	contractHash, err := CanonicalContractHash(ContractDigest{
		ProtocolVersion: ContractProtocolVersion,
		ServiceID:       serviceID,
		Version:         version,
		Placement:       spec.Placement,
		Scopes:          spec.Scopes,
		InputSchema:     inputSchema,
		OutputSchema:    outputSchema,
	})
	if err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "contract hash", Cause: err}
	}
	descriptor, err := normalizeCoreDescriptor(Descriptor{
		ID:          capabilityID,
		Summary:     spec.Summary,
		Tags:        spec.Tags,
		Scopes:      spec.Scopes,
		Permissions: spec.Permissions,
		Placement:   spec.Placement,
		InputRef:    &inputRef,
		OutputRef:   &outputRef,
	})
	if err != nil {
		return CoreDescriptorEntry{}, &CoreDescriptorError{ServiceID: serviceID, Reason: "descriptor", Cause: err}
	}
	return CoreDescriptorEntry{
		ProviderID:   providerID,
		CapabilityID: capabilityID,
		ServiceID:    serviceID,
		Version:      version,
		ContractHash: contractHash,
		Descriptor:   descriptor,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}, nil
}

func normalizeCoreDescriptor(descriptor Descriptor) (Descriptor, error) {
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

func putCoreSchema(schemas map[string]json.RawMessage, raw json.RawMessage) (SchemaRef, error) {
	if schemas == nil {
		return SchemaRef{}, fmt.Errorf("plugincontract: core schema table is nil")
	}
	canonical, err := CanonicalizeJSONValue(raw)
	if err != nil {
		return SchemaRef{}, fmt.Errorf("plugincontract: canonicalize schema: %w", err)
	}
	sum := sha256.Sum256(canonical)
	ref := SchemaRef{MediaType: DefaultSchemaMediaType, Hash: hex.EncodeToString(sum[:])}
	if _, exists := schemas[ref.Hash]; !exists {
		stored := make([]byte, len(canonical))
		copy(stored, canonical)
		schemas[ref.Hash] = stored
	}
	return ref, nil
}

func resolveSchemaRef(schemas map[string]json.RawMessage, ref SchemaRef) ([]byte, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	stored, exists := schemas[ref.Hash]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSchemaRef, ref.String())
	}
	out := make([]byte, len(stored))
	copy(out, stored)
	return out, nil
}

func SchemaForGoValue(sample any) (json.RawMessage, error) {
	if sample == nil {
		return nil, fmt.Errorf("%w: nil sample", ErrUnsupportedSchemaType)
	}
	return SchemaForGoType(reflect.TypeOf(sample))
}

func SchemaForGoType(target reflect.Type) (json.RawMessage, error) {
	if target == nil {
		return nil, fmt.Errorf("%w: nil type", ErrUnsupportedSchemaType)
	}
	schema, err := schemaForType(target, nil)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: encode schema: %w", err)
	}
	return json.RawMessage(encoded), nil
}

var (
	schemaTimeType       = reflect.TypeOf(time.Time{})
	schemaRawMessageType = reflect.TypeOf(json.RawMessage(nil))
)

func schemaForType(target reflect.Type, stack []reflect.Type) (map[string]any, error) {
	if target == nil {
		return map[string]any{}, nil
	}
	if target == schemaRawMessageType {
		return map[string]any{}, nil
	}
	if target == schemaTimeType {
		return map[string]any{"type": "string", "format": "date-time"}, nil
	}
	switch target.Kind() {
	case reflect.Pointer:
		return schemaForType(target.Elem(), stack)
	case reflect.Interface:
		return map[string]any{}, nil
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer", "minimum": 0}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.Slice, reflect.Array:
		if target.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64"}, nil
		}
		items, err := schemaForType(target.Elem(), stack)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items}, nil
	case reflect.Map:
		if target.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("%w: map key %s", ErrUnsupportedSchemaType, target.Key())
		}
		value, err := schemaForType(target.Elem(), stack)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "object", "additionalProperties": value}, nil
	case reflect.Struct:
		for _, seen := range stack {
			if seen == target {
				return nil, fmt.Errorf("%w: recursive struct %s", ErrUnsupportedSchemaType, target)
			}
		}
		extended := make([]reflect.Type, len(stack), len(stack)+1)
		copy(extended, stack)
		return structSchemaForType(target, append(extended, target))
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSchemaType, target)
	}
}

type jsonFieldOptions struct {
	omitempty bool
	omitzero  bool
}

func parseJSONFieldTag(tag string) (string, jsonFieldOptions) {
	var options jsonFieldOptions
	if tag == "" {
		return "", options
	}
	segments := strings.Split(tag, ",")
	for _, segment := range segments[1:] {
		switch segment {
		case "omitempty":
			options.omitempty = true
		case "omitzero":
			options.omitzero = true
		}
	}
	return segments[0], options
}

func structSchemaForType(target reflect.Type, stack []reflect.Type) (map[string]any, error) {
	properties := make(map[string]any, target.NumField())
	required := make([]string, 0, target.NumField())
	for index := 0; index < target.NumField(); index++ {
		field := target.Field(index)
		name, options := parseJSONFieldTag(field.Tag.Get("json"))
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" && field.Type.Kind() == reflect.Struct {
			embedded, err := structSchemaForType(field.Type, stack)
			if err != nil {
				return nil, err
			}
			if inner, ok := embedded["properties"].(map[string]any); ok {
				for key, value := range inner {
					properties[key] = value
				}
			}
			if names, ok := embedded["required"].([]string); ok {
				required = append(required, names...)
			}
			continue
		}
		if field.PkgPath != "" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		property, err := schemaForType(field.Type, stack)
		if err != nil {
			return nil, fmt.Errorf("plugincontract: field %s.%s: %w", target.Name(), field.Name, err)
		}
		properties[name] = property
		if !options.omitempty && !options.omitzero && field.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema, nil
}
