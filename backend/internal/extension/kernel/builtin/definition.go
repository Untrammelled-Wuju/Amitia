package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	MetadataKeyBuiltin       = "system.builtin"
	MetadataKeyManaged       = "system.managed"
	MetadataKeyRequired      = "system.required"
	MetadataKeyDisableAllowed = "system.disableAllowed"
	MetadataKeyComponent     = "system.component"
	MetadataKeyBootstrapRevision = "system.bootstrapRevision"
)

type Definition struct {
	Extension domain.ExtensionDefinition

	SystemManaged  bool
	Required       bool
DisableAllowed  bool
	BootstrapRevision int64
}

func (d Definition) ExtensionID() domain.ExtensionID {
	return d.Extension.ID
}

func (d Definition) Metadata() map[string]any {
	if d.Extension.Metadata == nil {
		d.Extension.Metadata = make(map[string]any)
	}
	d.Extension.Metadata[MetadataKeyBuiltin] = true
	d.Extension.Metadata[MetadataKeyManaged] = d.SystemManaged
	d.Extension.Metadata[MetadataKeyRequired] = d.Required
	d.Extension.Metadata[MetadataKeyDisableAllowed] = d.DisableAllowed
	d.Extension.Metadata[MetadataKeyBootstrapRevision] = d.BootstrapRevision
	return d.Extension.Metadata
}

func (d Definition) WithMetadata() domain.ExtensionDefinition {
	d.Extension.Metadata = d.Metadata()
	return d.Extension
}

func (d Definition) ShouldEnable() bool {
	if d.Required {
		return true
	}
	systemDefault, ok := d.Extension.Metadata["system.defaultEnabled"].(bool)
	if ok {
		return systemDefault
	}
	return !d.DisableAllowed
}

const (
	PrefixBuiltin = "com.amitia.builtin."
)
