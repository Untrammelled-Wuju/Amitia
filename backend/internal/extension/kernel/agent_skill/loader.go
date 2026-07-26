package agent_skill

import (
	"fmt"
)

type SkillDefinitionBuilder struct {
	parser      *SkillParser
	scanner     *ResourceScanner
	indexer     *ResourceIndexer
	compat      *CompatibilityValidator
	hostVersion string
	platform    string
}

func NewSkillDefinitionBuilder(hostVersion, platform string) *SkillDefinitionBuilder {
	return &SkillDefinitionBuilder{
		parser:      NewSkillParser(),
		scanner:     NewResourceScanner(),
		indexer:     NewResourceIndexer(),
		compat:      NewCompatibilityValidator(),
		hostVersion: hostVersion,
		platform:    platform,
	}
}

func (b *SkillDefinitionBuilder) Build(raw []byte, extensionID string) (*AgentSkillDefinition, error) {
	parsed, err := b.parser.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	fields := parsed.Fields

	name := getStringField(fields, "name", "")
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	description := getStringField(fields, "description", "")
	version := getStringField(fields, "version", "0.0.0")
	schemaVersion := getIntField(fields, "schemaVersion", 2)

	instructionText := SanitizeInstructionText(parsed.InstructionsRaw)
	instructionTokens := EstimateTokens(instructionText)

	activation := ActivationRule{
		Mode:     ActivationManual,
		Priority: 0,
	}
	if actRaw, ok := extractField(fields, "activation"); ok {
		if actMap, ok := actRaw.(map[string]any); ok {
			if mode, ok := actMap["mode"].(string); ok {
				activation.Mode = parseActivationMode(mode)
			}
			if keywords, ok := actMap["keywords"].([]any); ok {
				for _, kw := range keywords {
					if s, ok := kw.(string); ok {
						activation.Keywords = append(activation.Keywords, s)
					}
				}
			}
			if priority, ok := actMap["priority"].(int); ok {
				activation.Priority = priority
			}
		}
	}

	var requiredTools []ToolReference
	if toolsRaw, ok := extractField(fields, "requiredTools"); ok {
		if toolsArr, ok := toolsRaw.([]any); ok {
			for _, t := range toolsArr {
				if toolID, ok := t.(string); ok {
					requiredTools = append(requiredTools, ToolReference{
						ToolID:   toolID,
						Required: true,
					})
				} else if toolMap, ok := t.(map[string]any); ok {
					ref := ToolReference{Required: true}
					if id, ok := toolMap["id"].(string); ok {
						ref.ToolID = id
					}
					if req, ok := toolMap["required"].(bool); ok {
						ref.Required = req
					}
					requiredTools = append(requiredTools, ref)
				}
			}
		}
	}

	var requiredMCP []MCPReference
	if mcpRaw, ok := extractField(fields, "requiredMCP"); ok {
		if mcpArr, ok := mcpRaw.([]any); ok {
			for _, m := range mcpArr {
				if serverID, ok := m.(string); ok {
					requiredMCP = append(requiredMCP, MCPReference{ServerID: serverID})
				} else if mcpMap, ok := m.(map[string]any); ok {
					ref := MCPReference{}
					if id, ok := mcpMap["serverId"].(string); ok {
						ref.ServerID = id
					}
					if opt, ok := mcpMap["optional"].(bool); ok {
						ref.Optional = opt
					}
					if auto, ok := mcpMap["autoInstall"].(bool); ok {
						ref.AutoInstall = auto
					}
					requiredMCP = append(requiredMCP, ref)
				}
			}
		}
	}

	var resourcePaths []string
	if resRaw, ok := extractField(fields, "resources"); ok {
		if resArr, ok := resRaw.([]any); ok {
			for _, r := range resArr {
				if path, ok := r.(string); ok {
					resourcePaths = append(resourcePaths, path)
				} else if resMap, ok := r.(map[string]any); ok {
					if path, ok := resMap["path"].(string); ok {
						resourcePaths = append(resourcePaths, path)
					}
				}
			}
		}
	}

	resources, err := b.scanner.ScanPaths(resourcePaths)
	if err != nil {
		return nil, fmt.Errorf("resource scan failed: %w", err)
	}

	tokenPolicy := SkillTokenPolicy{
		MaxInstructionTokens: 2000,
	}
	if tpRaw, ok := extractField(fields, "tokenPolicy"); ok {
		if tpMap, ok := tpRaw.(map[string]any); ok {
			if mit, ok := tpMap["maxInstructionTokens"].(int); ok {
				tokenPolicy.MaxInstructionTokens = mit
			}
			if mrt, ok := tpMap["maxResourceTokensPerTurn"].(int); ok {
				tokenPolicy.MaxResourceTokensPerTurn = mrt
			}
			if mtp, ok := tpMap["maxTotalResources"].(int); ok {
				tokenPolicy.MaxTotalResources = mtp
			}
			if ts, ok := tpMap["truncationStrategy"].(string); ok {
				tokenPolicy.TruncationStrategy = ts
			}
		}
	}

	compatibility := b.compat.Validate(fields, b.hostVersion, b.platform)

	scope := AgentSkillScopeGlobal
	scopeID := ""
	if scopeRaw, ok := extractField(fields, "scope"); ok {
		if scopeStr, ok := scopeRaw.(string); ok {
			scope = mapScopeString(scopeStr)
		} else if scopeMap, ok := scopeRaw.(map[string]any); ok {
			if s, ok := scopeMap["type"].(string); ok {
				scope = mapScopeString(s)
			}
			if id, ok := scopeMap["id"].(string); ok {
				scopeID = id
			}
		}
	}

	author := getStringField(fields, "author", "")
	license := getStringField(fields, "license", "")
	displayName := getStringField(fields, "displayName", "")
	if displayName == "" {
		displayName = name
	}

	integrity := SkillIntegrity{
		Algorithm:       "sha256",
		FrontmatterHash: ComputeFrontmatterHash(parsed.FrontmatterRaw),
		ContentHash:     ComputeInstructionHash(instructionText),
	}

	warnings := ValidateFrontmatterFields(fields)
	if len(warnings) > 0 && compatibility.Status == "compatible" {
		compatibility.Messages = append(compatibility.Messages, warnings...)
	}

	return &AgentSkillDefinition{
		ID:            extensionID,
		ExtensionID:   extensionID,
		Name:          name,
		Description:   description,
		DisplayName:   displayName,
		Version:       version,
		SchemaVersion: schemaVersion,
		Instructions: SkillInstructionRef{
			Text:        instructionText,
			TokenCount:  instructionTokens,
			ContentHash: integrity.ContentHash,
		},
		Activation:    activation,
		Resources:     resources,
		RequiredTools: requiredTools,
		RequiredMCP:   requiredMCP,
		TokenPolicy:   tokenPolicy,
		Compatibility: compatibility,
		Integrity:     integrity,
		Scope:         scope,
		ScopeID:       scopeID,
		Enabled:       false,
		Compatible:    compatibility.Status == "compatible" || compatibility.Status == "legacy",
		Source:        "agent_skill",
		License:       license,
		Author:        author,
		Metadata:      map[string]any{},
	}, nil
}

func extractField(fields map[string]any, key string) (any, bool) {
	val, ok := fields[key]
	return val, ok
}

func parseActivationMode(mode string) ActivationMode {
	switch mode {
	case "auto":
		return ActivationAuto
	case "explicit":
		return ActivationExplicit
	default:
		return ActivationManual
	}
}

func mapScopeString(s string) AgentSkillScope {
	switch s {
	case "character", "role":
		return AgentSkillScopeCharacter
	default:
		return AgentSkillScopeGlobal
	}
}
