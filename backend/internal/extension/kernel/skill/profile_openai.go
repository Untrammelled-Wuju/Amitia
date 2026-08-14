package skill

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

type OpenAIProfileAdapter struct{}

func NewOpenAIProfileAdapter() *OpenAIProfileAdapter {
	return &OpenAIProfileAdapter{}
}

func (a *OpenAIProfileAdapter) ID() string      { return ProfileIDOpenAI }
func (a *OpenAIProfileAdapter) Version() string { return AdapterVersionOpenAI }

var brandColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (a *OpenAIProfileAdapter) Detect(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillProfileDetection, error) {
	if hasFileType(pkg.Files, "agents/openai.yaml") {
		return SkillProfileDetection{
			Detected: []SkillEcosystemProfile{
				{ID: ProfileIDOpenAI, Version: AdapterVersionOpenAI, Evidence: []string{"file:agents/openai.yaml"}},
			},
		}, nil
	}
	return SkillProfileDetection{Detected: []SkillEcosystemProfile{}}, nil
}

type openAIVerbatimYAML struct {
	Interface struct {
		DisplayName      string `yaml:"display_name"`
		ShortDescription string `yaml:"short_description"`
		IconSmall        string `yaml:"icon_small"`
		IconLarge        string `yaml:"icon_large"`
		BrandColor       string `yaml:"brand_color"`
		DefaultPrompt    string `yaml:"default_prompt"`
	} `yaml:"interface"`
	Policy struct {
		AllowImplicitInvocation *bool `yaml:"allow_implicit_invocation"`
	} `yaml:"policy"`
	Dependencies struct {
		Tools []map[string]interface{} `yaml:"tools"`
	} `yaml:"dependencies"`
}

func (a *OpenAIProfileAdapter) Analyze(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillCompatibilityOverlay, error) {
	overlay := SkillCompatibilityOverlay{
		Profile:        ProfileIDOpenAI,
		AdapterVersion: AdapterVersionOpenAI,
		FieldMappings:  []SkillFieldMapping{},
		Features:       []SkillFeatureResult{},
		Warnings:       []SkillWarning{},
		Errors:         []SkillError{},
	}

	raw, ok := pkg.Files["agents/openai.yaml"]
	if !ok {
		return overlay, nil
	}

	var node yaml.Node
	if err := decodeSafeOpenAIYAML(raw, &node); err != nil {
		overlay.Errors = append(overlay.Errors, SkillError{
			Code: "OPENAI_YAML_INVALID", Message: "agents/openai.yaml is invalid: " + err.Error(), Path: "agents/openai.yaml",
		})
		return overlay, nil
	}

	var value openAIVerbatimYAML
	var rawMap map[string]interface{}
	_ = node.Decode(&rawMap)
	if err := node.Decode(&value); err != nil {
		overlay.Errors = append(overlay.Errors, SkillError{
			Code: "OPENAI_YAML_INVALID", Message: "agents/openai.yaml fields are invalid: " + err.Error(), Path: "agents/openai.yaml",
		})
		return overlay, nil
	}

	ui := &SkillUIHints{}

	if value.Interface.DisplayName != "" {
		ui.DisplayName = value.Interface.DisplayName
		overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
			Profile: ProfileIDOpenAI, Source: "interface.display_name", Target: "ui.displayName",
			State: FeatureStateMapped, Reason: "display name mapped to canonical UI hint",
		})
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDOpenAI, Feature: "display_name", State: FeatureStateMapped,
		})
	}

	if value.Interface.ShortDescription != "" {
		ui.ShortDescription = value.Interface.ShortDescription
		overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
			Profile: ProfileIDOpenAI, Source: "interface.short_description", Target: "ui.shortDescription",
			State: FeatureStateMapped, Reason: "short description mapped to canonical UI hint",
		})
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDOpenAI, Feature: "short_description", State: FeatureStateMapped,
		})
	}

	if value.Interface.BrandColor != "" {
		if brandColorPattern.MatchString(value.Interface.BrandColor) {
			ui.BrandColor = value.Interface.BrandColor
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDOpenAI, Source: "interface.brand_color", Target: "ui.brandColor",
				State: FeatureStateMapped, Reason: "brand color is UI only and does not affect execution",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDOpenAI, Feature: "brand_color", State: FeatureStateMapped,
			})
		} else {
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDOpenAI, Source: "interface.brand_color", Target: "ui.brandColor",
				State: FeatureStateUnsupported, Reason: "invalid brand_color format",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDOpenAI, Feature: "brand_color", State: FeatureStateUnsupported,
				Reason: "must match ^#[0-9A-Fa-f]{6}$",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "OPENAI_BRAND_COLOR_INVALID",
				Message: "brand_color format is invalid and was ignored",
				Path:    "agents/openai.yaml",
			})
		}
	}

	if value.Interface.DefaultPrompt != "" {
		ui.DefaultPrompt = value.Interface.DefaultPrompt
		overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
			Profile: ProfileIDOpenAI, Source: "interface.default_prompt", Target: "ui.defaultPrompt",
			State: FeatureStateIgnoredDisplayOnly, Reason: "default_prompt is UI convenience only, not system prompt",
		})
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDOpenAI, Feature: "default_prompt", State: FeatureStateIgnoredDisplayOnly,
			Reason: "default_prompt is a UI suggestion and never becomes system prompt",
		})
		overlay.Warnings = append(overlay.Warnings, SkillWarning{
			Code:    "OPENAI_DEFAULT_PROMPT_DISPLAY_ONLY",
			Message: "default_prompt is UI convenience only and does not become system prompt",
			Path:    "agents/openai.yaml",
		})
	}

	for _, icon := range []struct{ path, field string }{
		{value.Interface.IconSmall, "icon_small"},
		{value.Interface.IconLarge, "icon_large"},
	} {
		if icon.path == "" {
			continue
		}
		clean := strings.TrimPrefix(icon.path, "./")
		if strings.Contains(icon.path, ":") || !strings.HasPrefix(clean, "assets/") {
			overlay.Errors = append(overlay.Errors, SkillError{
				Code:    "OPENAI_ICON_PATH_INVALID",
				Message: "icon must be a local assets path: " + icon.path,
				Path:    "agents/openai.yaml",
			})
			continue
		}
		if _, exists := pkg.Files[clean]; !exists {
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "OPENAI_ICON_MISSING",
				Message: "icon file not found in artifact: " + clean,
				Path:    "agents/openai.yaml",
			})
		}
		if icon.field == "icon_small" {
			ui.IconSmall = clean
		} else {
			ui.IconLarge = clean
		}
		overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
			Profile: ProfileIDOpenAI, Source: "interface." + icon.field, Target: "ui." + icon.field,
			State: FeatureStateMapped, Reason: "icon path validated against artifact resource index",
		})
	}

	overlay.UI = ui

	if value.Policy.AllowImplicitInvocation != nil && !*value.Policy.AllowImplicitInvocation {
		policy := DefaultInvocationPolicy
		policy.ImplicitInvocationAllowed = false
		overlay.InvocationPolicy = &policy
		overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
			Profile: ProfileIDOpenAI, Source: "policy.allow_implicit_invocation", Target: "invocationPolicy.implicitInvocationAllowed",
			State: FeatureStateMapped, Reason: "false disables automatic model invocation",
		})
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDOpenAI, Feature: "allow_implicit_invocation", State: FeatureStateMapped,
		})
	}

	deps, depWarnings, depErrs := a.parseDependencies(value.Dependencies.Tools, pkg.Files)
	if len(depErrs) > 0 {
		overlay.Errors = append(overlay.Errors, depErrs...)
	}
	if len(depWarnings) > 0 {
		overlay.Warnings = append(overlay.Warnings, depWarnings...)
	}
	if len(deps) > 0 {
		overlay.MCPDependencies = deps
		overlay.DependencyMappings = append(overlay.DependencyMappings, SkillDependencyMapping{
			Profile:    ProfileIDOpenAI,
			ResolvedAs: "mcpDependencies",
			State:      FeatureStateMappedWithPolicy,
			Reason:     "MCP dependencies are metadata only and require user confirmation",
		})
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDOpenAI, Feature: "mcp_dependencies", State: FeatureStateMappedWithPolicy,
			Reason: "MCP dependencies require explicit user confirmation before installation",
		})
		overlay.Warnings = append(overlay.Warnings, SkillWarning{
			Code:    "OPENAI_MCP_REQUIRES_CONFIRMATION",
			Message: "MCP dependencies require explicit user confirmation before installation",
			Path:    "agents/openai.yaml",
		})
	}

	if len(rawMap) > 0 {
		if extra, ok := rawMap["interface"].(map[string]interface{}); ok {
			unknownFields := detectUnknownMapFields(extra, "display_name", "short_description", "icon_small", "icon_large", "brand_color", "default_prompt")
			for _, f := range unknownFields {
				overlay.Warnings = append(overlay.Warnings, SkillWarning{
					Code:    "OPENAI_UNKNOWN_INTERFACE_FIELD",
					Message: "unknown interface field: " + f,
					Path:    "agents/openai.yaml",
				})
			}
		}
		if extra, ok := rawMap["policy"].(map[string]interface{}); ok {
			unknownFields := detectUnknownMapFields(extra, "allow_implicit_invocation")
			for _, f := range unknownFields {
				overlay.Warnings = append(overlay.Warnings, SkillWarning{
					Code:    "OPENAI_UNKNOWN_POLICY_FIELD",
					Message: "unknown policy field: " + f,
					Path:    "agents/openai.yaml",
				})
			}
		}
	}

	return overlay, nil
}

func (a *OpenAIProfileAdapter) parseDependencies(tools []map[string]interface{}, files map[string][]byte) ([]SkillMCPDependency, []SkillWarning, []SkillError) {
	var deps []SkillMCPDependency
	var warnings []SkillWarning
	var errors []SkillError

	for _, item := range tools {
		typeName, _ := item["type"].(string)
		if strings.ToLower(strings.TrimSpace(typeName)) != "mcp" {
			errors = append(errors, SkillError{
				Code:    "OPENAI_DEPENDENCY_TYPE_UNSUPPORTED",
				Message: "only MCP tool dependencies are supported, got: " + typeName,
				Path:    "agents/openai.yaml",
			})
			continue
		}
		valueName, _ := item["value"].(string)
		description, _ := item["description"].(string)
		transportName, _ := item["transport"].(string)
		endpoint, _ := item["url"].(string)
		command, _ := item["command"].(string)
		args := parseYAMLStringList(item["args"])
		authType, _ := item["auth.type"].(string)
		toolAllowlist := parseYAMLStringList(item["tools.allow"])

		dep := SkillMCPDependency{
			ID:                         valueName,
			Description:                description,
			Required:                   true,
			Transport:                  strings.ToLower(transportName),
			URL:                        endpoint,
			Command:                    command,
			Args:                       args,
			AuthType:                   strings.ToLower(authType),
			ToolAllowlist:              toolAllowlist,
			DefaultScope:               "character",
			AutoConfigure:              false,
			AutoEnable:                 false,
			RequiresManualConfirmation: true,
		}
		if dep.Transport == "" && dep.URL != "" {
			dep.Transport = "streamable_http"
		}
		if dep.AuthType == "" {
			dep.AuthType = "none"
		}

		if err := validateSkillMCPDependency(&dep); err != nil {
			errors = append(errors, SkillError{
				Code:    "OPENAI_DEPENDENCY_INVALID",
				Message: err.Error(),
				Path:    "agents/openai.yaml",
			})
			continue
		}

		deps = append(deps, dep)
	}

	return deps, warnings, errors
}

func validateSkillMCPDependency(dep *SkillMCPDependency) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`).MatchString(dep.ID) {
		return fmt.Errorf("invalid MCP dependency id: %s", dep.ID)
	}
	if dep.Transport == "" && dep.URL == "" && dep.Command == "" {
		return nil
	}
	if dep.Transport != "streamable_http" && dep.Transport != "stdio" {
		return fmt.Errorf("invalid transport: %s", dep.Transport)
	}
	if dep.AuthType != "none" && dep.AuthType != "oauth" && dep.AuthType != "bearer_token" && dep.AuthType != "custom_headers" && dep.AuthType != "stdio_env" {
		return fmt.Errorf("invalid auth type: %s", dep.AuthType)
	}
	if dep.Transport == "streamable_http" {
		parsed, err := url.Parse(dep.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("invalid MCP URL: %s", dep.URL)
		}
		if parsed.Scheme != "https" {
			if parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || isLoopback(parsed.Hostname())) {
			} else {
				return fmt.Errorf("MCP URL must be https or localhost http: %s", dep.URL)
			}
		}
	}
	if dep.Transport == "stdio" && strings.TrimSpace(dep.Command) == "" {
		return fmt.Errorf("stdio transport requires command")
	}
	return nil
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func decodeSafeOpenAIYAML(raw []byte, node *yaml.Node) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(node); err != nil {
		return err
	}
	return nil
}

func parseYAMLStringList(v interface{}) []string {
	switch val := v.(type) {
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		return strings.Fields(val)
	}
	return nil
}

func detectUnknownMapFields(m map[string]interface{}, known ...string) []string {
	knownSet := make(map[string]bool, len(known))
	for _, k := range known {
		knownSet[k] = true
	}
	var unknown []string
	for k := range m {
		if !knownSet[k] {
			unknown = append(unknown, k)
		}
	}
	return unknown
}
