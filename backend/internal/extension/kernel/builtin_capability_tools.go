package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

const UsePackageToolName = "use_package"

func buildUsePackageTool() tool.Tool {
	return tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        UsePackageToolName,
			Description: "Activate or acquire a dynamic Amitia capability by package name. Skills are activated directly; Package/MCP/capability candidates are resolved through Amitia capability acquisition.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"package_name": {
						Type:        "string",
						Description: "Package, Skill, MCP, or canonical capability name to activate.",
					},
				},
				Required: []string{"package_name"},
			},
		},
	}
}

type usePackageInput struct {
	PackageName string `json:"package_name"`
}

func (f *ToolFacade) handleUsePackage(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req usePackageInput
	if err := json.Unmarshal(input, &req); err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid use_package input: %v", err),
			Error:       &LegacyToolError{Code: "INVALID_INPUT", Message: err.Error()},
		}, err
	}
	name := strings.TrimSpace(req.PackageName)
	if name == "" {
		err := fmt.Errorf("package_name is required")
		return LegacyToolResult{Status: "FAILED", VisibleText: err.Error(), Error: &LegacyToolError{Code: "INVALID_INPUT", Message: err.Error()}}, err
	}

	// Skills are the highest-confidence package-name lookup because the skill
	// catalog is explicitly name-addressable and activation is conversation scoped.
	var skillErr error
	if f.agentSkillBackend != nil {
		if result, err := f.agentSkillBackend.Activate(ctx, scope, name, true); err == nil {
			output, _ := json.Marshal(map[string]any{
				"status":              "activated",
				"kind":                "skill",
				"packageName":         name,
				"activationId":        result.ActivationID,
				"extensionId":         result.ExtensionID,
				"scope":               result.Scope,
				"compatibilityStatus": result.CompatibilityStatus,
				"contentHash":         result.ContentHash,
			})
			return LegacyToolResult{RunID: result.ActivationID, Status: "SUCCESS", Output: output, VisibleText: fmt.Sprintf("Dynamic package activated as Skill: %s", result.Name)}, nil
		} else {
			skillErr = err
		}
	}

	var activationErr error
	matchedCandidate := false
	if f.acquisitionBridge != nil {
		// Search by the package name first. Extension and MCP sources are name-aware;
		// the additional canonical forms cover callers that provide a short package ID.
		queries := []string{name}
		if !strings.Contains(name, ".") {
			queries = append(queries,
				"skill."+name,
				"mcp.server."+name,
				"package."+name,
			)
		}
		seen := map[string]struct{}{}
		for _, query := range queries {
			if _, ok := seen[query]; ok {
				continue
			}
			seen[query] = struct{}{}
			found, findErr := f.acquisitionBridge.FindCapabilities(ctx, acquisition.FindCapabilitiesInput{
				CapabilityID: query,
				Description:  "Activate dynamic package " + name,
			}, scope.UserID)
			if findErr != nil || found == nil || found.TotalFound == 0 {
				continue
			}

			candidate, capabilityID := selectUsePackageCandidate(found.Candidates, name, query)
			if candidate == nil || capabilityID == "" {
				continue
			}
			matchedCandidate = true
			acquired, acquireErr := f.acquisitionBridge.AcquireCapability(ctx, acquisition.AcquireInput{
				CapabilityID: capabilityID,
				CandidateID:  candidate.ID,
			}, scope.UserID, scope.ExecContext)
			if acquireErr != nil {
				activationErr = acquireErr
				continue
			}
			output, _ := json.Marshal(map[string]any{
				"status":        acquired.State,
				"kind":          candidate.Kind,
				"packageName":   name,
				"candidateId":   candidate.ID,
				"candidateName": candidate.Name,
				"capabilityId":  capabilityID,
				"success":       acquired.Success,
				"needsApproval": acquired.NeedsApproval,
				"resumeToken":   acquired.ResumeToken,
				"errorMessage":  acquired.ErrorMessage,
			})
			if acquired.NeedsApproval {
				visible := fmt.Sprintf("Dynamic package %s requires user approval before activation", name)
				return LegacyToolResult{
					Status:      "WAITING_APPROVAL",
					Output:      output,
					VisibleText: visible,
					Error: &LegacyToolError{
						Code:      "PACKAGE_APPROVAL_REQUIRED",
						Message:   name,
						Detail:    acquired.ResumeToken,
						Retryable: true,
					},
				}, nil
			}
			if acquired.Success {
				visible := fmt.Sprintf("Dynamic package %s activated as %s %s", name, candidate.Kind, candidate.Name)
				return LegacyToolResult{Status: "SUCCESS", Output: output, VisibleText: visible}, nil
			}

			message := fmt.Sprintf("dynamic package %q is not ready after activation attempt (state: %s)", name, acquired.State)
			if strings.TrimSpace(acquired.ErrorMessage) != "" {
				message += ": " + acquired.ErrorMessage
			}
			err := fmt.Errorf("%s", message)
			return LegacyToolResult{
				Status:      "FAILED",
				Output:      output,
				VisibleText: message,
				Error: &LegacyToolError{
					Code:      "PACKAGE_NOT_READY",
					Message:   name,
					Detail:    string(acquired.State),
					Retryable: acquired.State == acquisition.StateReconciling,
				},
			}, err
		}
	}

	if matchedCandidate && activationErr != nil {
		message := fmt.Sprintf("dynamic package %q was found but activation failed: %v", name, activationErr)
		err := fmt.Errorf("%s", message)
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: message,
			Error:       &LegacyToolError{Code: "PACKAGE_ACTIVATION_FAILED", Message: name, Detail: activationErr.Error(), Retryable: true},
		}, err
	}

	message := fmt.Sprintf("dynamic package %q was not found as a Skill, MCP, Package, or capability", name)
	if skillErr != nil {
		message += fmt.Sprintf(" (skill activation: %v)", skillErr)
	}
	err := fmt.Errorf("%s", message)
	return LegacyToolResult{Status: "FAILED", VisibleText: message, Error: &LegacyToolError{Code: "PACKAGE_NOT_FOUND", Message: message}}, err
}

func selectUsePackageCandidate(candidates []acquisition.CapabilityCandidate, packageName, query string) (*acquisition.CapabilityCandidate, string) {
	if len(candidates) == 0 {
		return nil, ""
	}

	// A package-name call must never activate an unrelated fuzzy search result.
	// Prefer an exact package identity (display name, ID, extension ID, MCP
	// server/binding ID). Once the concrete package is known, acquire one of the
	// capabilities actually declared by that candidate, preferring the queried
	// capability when it is present.
	for i := range candidates {
		candidate := &candidates[i]
		if !usePackageIdentityMatches(*candidate, packageName) {
			continue
		}
		if capabilityID := matchingCandidateCapability(*candidate, query); capabilityID != "" {
			return candidate, capabilityID
		}
		if len(candidate.Capabilities) > 0 {
			return candidate, string(candidate.Capabilities[0])
		}
	}

	// The caller may pass a canonical capability ID rather than a package name.
	// In that case an exact capability match is sufficient and preserves normal
	// Amitia acquisition ranking without accepting semantic/fuzzy-only matches.
	for i := range candidates {
		candidate := &candidates[i]
		if capabilityID := matchingCandidateCapability(*candidate, query); capabilityID != "" {
			return candidate, capabilityID
		}
	}
	return nil, ""
}

func usePackageIdentityMatches(candidate acquisition.CapabilityCandidate, packageName string) bool {
	wanted := strings.TrimSpace(packageName)
	if wanted == "" {
		return false
	}
	equals := func(value string) bool {
		value = strings.TrimSpace(value)
		return value != "" && strings.EqualFold(value, wanted)
	}
	if equals(candidate.Name) || equals(candidate.ID) {
		return true
	}
	if candidate.Install.MCP != nil && equals(candidate.Install.MCP.ServerName) {
		return true
	}
	if candidate.Metadata != nil {
		for _, key := range []string{"extensionId", "serverName", "bindingId", "bindingID", "packageName"} {
			if value, ok := candidate.Metadata[key].(string); ok && equals(value) {
				return true
			}
		}
	}
	return false
}

func matchingCandidateCapability(candidate acquisition.CapabilityCandidate, query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	for _, capID := range candidate.Capabilities {
		if strings.EqualFold(strings.TrimSpace(string(capID)), query) {
			return string(capID)
		}
	}
	return ""
}
