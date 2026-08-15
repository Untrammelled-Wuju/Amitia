package uiagent

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	CapUIInspect         capability.CapabilityID = "ui.inspect"
	CapUISourceInspect   capability.CapabilityID = "ui.source.inspect"
	CapUISourcePatch     capability.CapabilityID = "ui.source.patch"
	CapUISourcePreview   capability.CapabilityID = "ui.source.preview"
	CapUISchemaGenerate  capability.CapabilityID = "ui.schema.generate"
	CapUISchemaValidate  capability.CapabilityID = "ui.schema.validate"
	CapUISchemaCompile   capability.CapabilityID = "ui.schema.compile"
	CapUISchemaRender    capability.CapabilityID = "ui.schema.render"
	CapUIContribRegister  capability.CapabilityID = "ui.contribution.register"
	CapUIContribUpdate    capability.CapabilityID = "ui.contribution.update"
	CapUIContribRemove    capability.CapabilityID = "ui.contribution.remove"
	CapUIPreviewCapture   capability.CapabilityID = "ui.preview.capture"
	CapUIPreviewObserve   capability.CapabilityID = "ui.preview.observe"
)

var RequiredCapabilitiesForMode = map[UITargetType][]capability.CapabilityID{
	UITargetSource: {
		CapUISourceInspect, CapUISourcePatch, CapUISourcePreview,
	},
	UITargetSchema: {
		CapUISchemaGenerate, CapUISchemaValidate, CapUISchemaCompile, CapUISchemaRender,
	},
	UITargetContribution: {
		CapUIContribRegister, CapUIContribUpdate,
	},
}
