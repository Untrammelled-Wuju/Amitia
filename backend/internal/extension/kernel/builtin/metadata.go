package builtin

import "github.com/u-ai/backend/internal/extension/kernel/domain"

func IsBuiltinExtension(def domain.ExtensionDefinition) bool {
	if def.Metadata == nil {
		return false
	}
	v, ok := def.Metadata[MetadataKeyBuiltin]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func IsSystemRequired(def domain.ExtensionDefinition) bool {
	if def.Metadata == nil {
		return false
	}
	v, ok := def.Metadata[MetadataKeyRequired]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func IsDisableAllowed(def domain.ExtensionDefinition) bool {
	if def.Metadata == nil {
		return true
	}
	v, ok := def.Metadata[MetadataKeyDisableAllowed]
	if !ok {
		return true
	}
	b, ok := v.(bool)
	return ok && b
}

func GetComponentName(def domain.ExtensionDefinition) string {
	if def.Metadata == nil {
		return ""
	}
	v, ok := def.Metadata[MetadataKeyComponent]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
