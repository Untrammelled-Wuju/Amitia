package skill

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

type ClaudeCodeProfileAdapter struct{}

func NewClaudeCodeProfileAdapter() *ClaudeCodeProfileAdapter {
	return &ClaudeCodeProfileAdapter{}
}

func (a *ClaudeCodeProfileAdapter) ID() string      { return ProfileIDClaudeCode }
func (a *ClaudeCodeProfileAdapter) Version() string { return AdapterVersionClaudeCode }

var claudeCodeExtraFields = []string{
	"when_to_use", "argument-hint", "arguments", "disable-model-invocation",
	"user-invocable", "disallowed-tools", "model", "effort", "context",
	"agent", "background", "hooks", "paths", "shell",
}

func (a *ClaudeCodeProfileAdapter) Detect(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillProfileDetection, error) {
	evidence := detectExtraFields(parsed.ExtraFrontmatter, claudeCodeExtraFields...)
	if len(evidence) > 0 {
		return SkillProfileDetection{
			Detected: []SkillEcosystemProfile{
				{ID: ProfileIDClaudeCode, Version: AdapterVersionClaudeCode, Evidence: evidence},
			},
		}, nil
	}
	return SkillProfileDetection{Detected: []SkillEcosystemProfile{}}, nil
}

var dynamicShellPatterns = []*regexp.Regexp{
	regexp.MustCompile("!`[^`]*`"),
	regexp.MustCompile("```!\n"),
}

func (a *ClaudeCodeProfileAdapter) Analyze(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillCompatibilityOverlay, error) {
	overlay := SkillCompatibilityOverlay{
		Profile:        ProfileIDClaudeCode,
		AdapterVersion: AdapterVersionClaudeCode,
		FieldMappings:  []SkillFieldMapping{},
		Features:       []SkillFeatureResult{},
		Warnings:       []SkillWarning{},
		Errors:         []SkillError{},
	}

	policy := DefaultInvocationPolicy
	hasPolicyOverride := false

	if v, ok := parsed.ExtraFrontmatter["when_to_use"]; ok {
		if hint, ok := v.(string); ok && hint != "" {
			overlay.ActivationHints = append(overlay.ActivationHints, hint)
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "when_to_use", Target: "activationHints",
				State: FeatureStateMapped, Reason: "mapped to canonical activation hint",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "when_to_use", State: FeatureStateMapped,
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["argument-hint"]; ok {
		if hint, ok := v.(string); ok && hint != "" {
			overlay.UI = &SkillUIHints{ArgumentHint: hint}
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "argument-hint", Target: "ui.argumentHint",
				State: FeatureStateIgnoredDisplayOnly, Reason: "display-only UI hint, Amitia UI may not render",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "argument-hint", State: FeatureStateIgnoredDisplayOnly,
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "CLAUDE_ARGUMENT_HINT_DISPLAY_ONLY",
				Message: "argument-hint is a display-only UI hint and does not affect execution permissions",
				Path:    "SKILL.md",
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["arguments"]; ok {
		if args, ok := v.(string); ok && args != "" {
			overlay.ArgumentSchema = &SkillArgumentSchema{Raw: args}
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "arguments", Target: "argumentSchema",
				State: FeatureStateMapped, Reason: "mapped to canonical argument schema for invocation",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "arguments", State: FeatureStateMapped,
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["disable-model-invocation"]; ok {
		if disable, ok := v.(bool); ok && disable {
			policy.ImplicitInvocationAllowed = false
			hasPolicyOverride = true
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "disable-model-invocation", Target: "invocationPolicy.implicitInvocationAllowed",
				State: FeatureStateMapped, Reason: "suppresses automatic model invocation; user invocation still allowed",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "disable-model-invocation", State: FeatureStateMapped,
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["user-invocable"]; ok {
		if invocable, ok := v.(bool); ok && !invocable {
			policy.UserInvocationAllowed = false
			hasPolicyOverride = true
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "user-invocable", Target: "invocationPolicy.userInvocationAllowed",
				State: FeatureStateMapped, Reason: "hides from user skill picker/slash UI",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "user-invocable", State: FeatureStateMapped,
			})
		}
	}

	if hasPolicyOverride && !policy.UserInvocationAllowed && !policy.ImplicitInvocationAllowed {
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDClaudeCode, Feature: "invocation", State: FeatureStateDegraded,
			Reason: "both model and user invocation are disabled; skill has no valid invoker",
		})
		overlay.Warnings = append(overlay.Warnings, SkillWarning{
			Code:    "CLAUDE_NO_VALID_INVOKER",
			Message: "disable-model-invocation and user-invocable=false leave skill with no valid invocation path",
			Path:    "SKILL.md",
		})
	}

	if v, ok := parsed.ExtraFrontmatter["disallowed-tools"]; ok {
		tools := parseToolList(v)
		if len(tools) > 0 {
			overlay.ToolDenyRules = tools
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "disallowed-tools", Target: "toolDenyRules",
				State: FeatureStateMappedWithPolicy, Reason: "mapped as tool deny overlay; only reduces permissions",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "disallowed-tools", State: FeatureStateMappedWithPolicy,
				Reason: "tool deny rules only reduce permissions and never grant them",
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["model"]; ok {
		if model, ok := v.(string); ok && model != "" {
			if strings.ToLower(model) == "inherit" {
				overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
					Profile: ProfileIDClaudeCode, Source: "model", Target: "preferredModelHint",
					State: FeatureStateMapped, Reason: "inherit means use current model",
				})
				overlay.Features = append(overlay.Features, SkillFeatureResult{
					Profile: ProfileIDClaudeCode, Feature: "model", State: FeatureStateMapped,
				})
			} else {
				overlay.PreferredModelHint = model
				overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
					Profile: ProfileIDClaudeCode, Source: "model", Target: "preferredModelHint",
					State: FeatureStateDegraded, Reason: "model hint recorded but Amitia model policy decides",
				})
				overlay.Features = append(overlay.Features, SkillFeatureResult{
					Profile: ProfileIDClaudeCode, Feature: "model", State: FeatureStateDegraded,
					Reason: "model preference cannot override Amitia model policy",
				})
				overlay.Warnings = append(overlay.Warnings, SkillWarning{
					Code:    "CLAUDE_MODEL_HINT_DEGRADED",
					Message: "model hint is a preference only; Amitia model policy decides the actual model",
					Path:    "SKILL.md",
				})
			}
		}
	}

	if v, ok := parsed.ExtraFrontmatter["effort"]; ok {
		if effort, ok := v.(string); ok && effort != "" {
			overlay.PreferredEffort = effort
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "effort", Target: "preferredEffort",
				State: FeatureStateIgnoredDisplayOnly, Reason: "reasoning effort hint recorded but may not be honored",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "effort", State: FeatureStateIgnoredDisplayOnly,
				Reason: "reasoning effort is a hint; Amitia may not support it",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "CLAUDE_EFFORT_DISPLAY_ONLY",
				Message: "effort hint is advisory and may not affect model behavior",
				Path:    "SKILL.md",
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["paths"]; ok {
		patterns := parseStringList(v)
		if len(patterns) > 0 {
			overlay.WorkspacePathPatterns = patterns
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "paths", Target: "workspacePathPatterns",
				State: FeatureStateDegraded, Reason: "activation filter only; no automatic file read or workspace mount",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "paths", State: FeatureStateDegraded,
				Reason: "paths is an activation filter, not a permission grant",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "CLAUDE_PATHS_FILTER_ONLY",
				Message: "paths only affects catalog visibility and implicit activation, not file permissions",
				Path:    "SKILL.md",
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["context"]; ok {
		if ctx, ok := v.(string); ok && ctx == "fork" {
			policy.IsolatedExecutionRequested = true
			hasPolicyOverride = true
			overlay.ExecutionMode = ExecutionModeIsolated
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "context", Target: "executionMode",
				State: FeatureStateMappedWithPolicy, Reason: "mapped to canonical isolated execution if available",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "context=fork", State: FeatureStateMappedWithPolicy,
				Reason: "requires Amitia MultiAgent/Task Runtime which may not be fully equivalent",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "CLAUDE_CONTEXT_FORK_LIMITED",
				Message: "context=fork maps to canonical isolated execution but isolation semantics may differ",
				Path:    "SKILL.md",
			})
		} else if ctx != "" {
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "context", Target: "executionMode",
				State: FeatureStateUnsupported, Reason: "non-fork context value is not supported",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "context", State: FeatureStateUnsupported,
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["agent"]; ok {
		if agent, ok := v.(string); ok && agent != "" {
			if overlay.PreferredModelHint == "" {
				overlay.PreferredModelHint = agent
			}
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "agent", Target: "preferredModelHint",
				State: FeatureStateDegraded, Reason: "agent profile hint requires matching canonical agent",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "agent", State: FeatureStateDegraded,
				Reason: "agent profile may not exist in Amitia",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "CLAUDE_AGENT_PROFILE_LIMITED",
				Message: "agent profile hint may not match any canonical agent",
				Path:    "SKILL.md",
			})
		}
	}

	if v, ok := parsed.ExtraFrontmatter["background"]; ok {
		if bg, ok := v.(bool); ok && bg {
			policy.BackgroundAllowed = true
			policy.IsolatedExecutionRequested = true
			hasPolicyOverride = true
			if overlay.ExecutionMode == "" {
				overlay.ExecutionMode = ExecutionModeBackground
			}
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCode, Source: "background", Target: "invocationPolicy.backgroundAllowed",
				State: FeatureStateMappedWithPolicy, Reason: "requires Amitia BackgroundTaskCoordinator",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCode, Feature: "background", State: FeatureStateMappedWithPolicy,
				Reason: "background execution capability is platform-dependent",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "CLAUDE_BACKGROUND_PLATFORM_LIMITED",
				Message: "background execution depends on platform capability",
				Path:    "SKILL.md",
			})
		}
	}

	if _, ok := parsed.ExtraFrontmatter["hooks"]; ok {
		overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
			Profile: ProfileIDClaudeCode, Source: "hooks", Target: "unsupported",
			State: FeatureStateUnsupported, Reason: "hooks requires Step39 Hook Runtime which is not yet available",
		})
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDClaudeCode, Feature: "hooks", State: FeatureStateUnsupported,
			Reason: "hooks runtime not yet implemented",
		})
		overlay.UnsupportedFeatures = append(overlay.UnsupportedFeatures, "hooks")
		overlay.Warnings = append(overlay.Warnings, SkillWarning{
			Code:    "CLAUDE_HOOKS_UNSUPPORTED",
			Message: "hooks are preserved but not executed until Step39 completes",
			Path:    "SKILL.md",
		})
	}

	if _, ok := parsed.ExtraFrontmatter["shell"]; ok {
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDClaudeCode, Feature: "shell", State: FeatureStateBlocked,
			Reason: "Claude dynamic shell injection is never executed by Amitia",
		})
		overlay.Warnings = append(overlay.Warnings, SkillWarning{
			Code:    "CLAUDE_SHELL_BLOCKED",
			Message: "shell field indicates dependency on Claude dynamic shell injection which Amitia does not execute",
			Path:    "SKILL.md",
		})
	}

	if hasShellInjection(parsed.Body) {
		overlay.Features = append(overlay.Features, SkillFeatureResult{
			Profile: ProfileIDClaudeCode, Feature: "dynamic_shell_injection", State: FeatureStateBlocked,
			Reason: "body contains !`command` or ```! constructs that Amitia does not execute",
		})
		overlay.UnsupportedFeatures = append(overlay.UnsupportedFeatures, "claude.dynamic_shell_injection")
		overlay.Warnings = append(overlay.Warnings, SkillWarning{
			Code:    "CLAUDE_DYNAMIC_SHELL_INJECTION",
			Message: "body contains dynamic shell injection syntax that will not be executed",
			Path:    "SKILL.md",
		})
	}

	if hasPolicyOverride || !policy.UserInvocationAllowed || !policy.ImplicitInvocationAllowed || policy.IsolatedExecutionRequested {
		overlay.InvocationPolicy = &policy
	}

	unknownFields := detectUnknownExtraFields(parsed.ExtraFrontmatter, claudeCodeExtraFields)
	for _, field := range unknownFields {
		overlay.Warnings = append(overlay.Warnings, SkillWarning{
			Code:    "UNKNOWN_CLIUDE_EXTENSION_FIELD",
			Message: "unknown Claude Code extension field is preserved but not executed",
			Path:    field,
		})
	}

	return overlay, nil
}

func hasShellInjection(body string) bool {
	for _, re := range dynamicShellPatterns {
		if re.MatchString(body) {
			return true
		}
	}
	return false
}

func parseToolList(v interface{}) []string {
	switch val := v.(type) {
	case string:
		var result []string
		for _, tok := range strings.Fields(val) {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				result = append(result, tok)
			}
		}
		return result
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func parseStringList(v interface{}) []string {
	switch val := v.(type) {
	case string:
		var result []string
		for _, tok := range strings.Fields(val) {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				result = append(result, tok)
			}
		}
		return result
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func detectUnknownExtraFields(extra map[string]interface{}, known []string) []string {
	knownSet := make(map[string]bool, len(known))
	for _, k := range known {
		knownSet[k] = true
	}
	var unknown []string
	for k := range extra {
		if !knownSet[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	return unknown
}
