package extension

import (
	"encoding/json"
	"strings"
)

var surfaceComponents = map[string]bool{
	"text": true, "number": true, "switch": true, "select": true, "textarea": true,
	"secret": true, "action": true, "status": true, "table": true,
}

var surfaceSectionTypes = map[string]bool{"form": true, "action": true, "status": true, "table": true}

func validateSurface(manifest PluginManifest, raw json.RawMessage) error {
	if len(raw) > 65536 {
		return NewExtensionError(ErrPluginSurfaceInvalid, "Plugin surface is too large", manifest.Metadata.ID, false, nil)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"<script", "javascript:", "v-html", "<style", "eval(", "import("} {
		if strings.Contains(lower, forbidden) {
			return NewExtensionError(ErrPluginSurfaceInvalid, "Executable surface content is forbidden", forbidden, false, nil)
		}
	}
	var document SurfaceDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return NewExtensionError(ErrPluginSurfaceInvalid, "Plugin surface is invalid", err.Error(), false, err)
	}
	if document.Schema != "https://schemas.amitia.dev/extensions/v1/surface.schema.json" || document.Version != "1.0" || strings.TrimSpace(document.Title) == "" || len(document.Sections) == 0 {
		return NewExtensionError(ErrPluginSurfaceInvalid, "Plugin surface metadata is invalid", manifest.Metadata.ID, false, nil)
	}
	sectionIDs := map[string]bool{}
	registeredSkills := map[string]bool{}
	for _, skillID := range manifest.RegisteredSkills {
		registeredSkills[skillID] = true
	}
	for _, section := range document.Sections {
		if strings.TrimSpace(section.ID) == "" || sectionIDs[section.ID] || !surfaceSectionTypes[section.Type] {
			return NewExtensionError(ErrPluginSurfaceInvalid, "Plugin surface section is invalid", section.ID, false, nil)
		}
		sectionIDs[section.ID] = true
		if section.Type == "action" && !registeredSkills[section.Skill] {
			return NewExtensionError(ErrPluginSurfaceInvalid, "Surface action skill is not declared", section.Skill, false, nil)
		}
		for _, field := range append(append([]SurfaceField(nil), section.Fields...), section.Columns...) {
			if strings.TrimSpace(field.Key) == "" || strings.TrimSpace(field.Label) == "" || !surfaceComponents[field.Component] {
				return NewExtensionError(ErrPluginSurfaceInvalid, "Plugin surface field is invalid", field.Key, false, nil)
			}
		}
	}
	return nil
}

func surfaceActionSkill(manifest PluginManifest, actionID string) (string, error) {
	var document SurfaceDocument
	if err := json.Unmarshal(manifest.Surface, &document); err != nil {
		return "", NewExtensionError(ErrPluginSurfaceInvalid, "Plugin surface is invalid", manifest.Metadata.ID, false, err)
	}
	for _, section := range document.Sections {
		if section.ID == actionID && section.Type == "action" && section.Skill != "" {
			return section.Skill, nil
		}
	}
	return "", NewExtensionError(ErrPluginActionNotAllowed, "Plugin action is not declared", actionID, false, nil)
}
