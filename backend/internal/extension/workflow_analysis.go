package extension

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type WorkflowRiskItem struct {
	Level   string `json:"level"`
	NodeID  string `json:"nodeId,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WorkflowNestedDependency struct {
	NodeID         string `json:"nodeId"`
	WorkflowID     string `json:"workflowId"`
	Name           string `json:"name,omitempty"`
	Status         string `json:"status"`
	DefinitionHash string `json:"definitionHash,omitempty"`
}

type WorkflowSafetyAnalysis struct {
	RiskLevel           string                     `json:"riskLevel"`
	DeclaredPermissions []string                   `json:"declaredPermissions"`
	SecretReferences    []string                   `json:"secretReferences"`
	Risks               []WorkflowRiskItem         `json:"risks"`
	NestedDependencies  []WorkflowNestedDependency `json:"nestedDependencies"`
	HasSideEffects      bool                       `json:"hasSideEffects"`
}

func (api *WorkflowAPI) prepareValidatedUserWorkflow(def workflow.WorkflowDefinition, userID, existingID string) (workflow.WorkflowDefinition, error) {
	prepared, err := prepareUserWorkflow(def, userID, existingID)
	if err != nil {
		return def, err
	}
	if err := api.validateNestedWorkflowTargets(prepared, userID); err != nil {
		return def, err
	}
	return prepared, nil
}

func nestedWorkflowTarget(node workflow.WorkflowNode) string {
	if node.Type != "nested_workflow" {
		return ""
	}
	if strings.TrimSpace(node.TargetID) != "" {
		return strings.TrimSpace(node.TargetID)
	}
	return strings.TrimSpace(node.Runtime.RuntimeID)
}

func remoteDeviceNestedWorkflow(node workflow.WorkflowNode) bool {
	return node.Type == "nested_workflow" && node.ExecutionTarget.Placement == workflow.WorkflowExecutionDevice
}

func ownedWorkflowUserID(def workflow.WorkflowDefinition) string {
	if def.Metadata == nil {
		return ""
	}
	value, ok := def.Metadata["ownerUserId"]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func (api *WorkflowAPI) validateNestedWorkflowTargets(def workflow.WorkflowDefinition, userID string) error {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || api.runtime.Kernel.Container().WorkflowRegistry == nil {
		return fmt.Errorf("workflow registry unavailable")
	}
	registry := api.runtime.Kernel.Container().WorkflowRegistry
	for _, node := range def.Nodes {
		targetID := nestedWorkflowTarget(node)
		if targetID == "" {
			continue
		}
		// A device-targeted nested workflow belongs to that Device Agent's local
		// Workflow Registry, not this Core's registry. Existence/ownership is
		// authoritatively checked by Cloud Core's device ownership gate and the
		// target Device Agent's meshOwned() call at execution time. Treating it as
		// a local dependency here would make Cloud -> Device Local Workflow
		// impossible to save. Distributed recursion is enforced by CallStack.
		if remoteDeviceNestedWorkflow(node) {
			continue
		}
		if targetID == def.ID {
			return fmt.Errorf("nested workflow node %s cannot target itself", node.ID)
		}
		target, ok := registry.Get(targetID)
		if !ok {
			return fmt.Errorf("nested workflow node %s target %s not found", node.ID, targetID)
		}
		if target.Source != userWorkflowSource || ownedWorkflowUserID(target) != userID {
			return fmt.Errorf("nested workflow node %s target %s is not owned by the current user", node.ID, targetID)
		}
	}

	visiting := map[string]bool{}
	visited := map[string]bool{}
	var walk func(workflow.WorkflowDefinition) error
	walk = func(current workflow.WorkflowDefinition) error {
		if visiting[current.ID] {
			return fmt.Errorf("nested workflow dependency cycle detected at %s", current.ID)
		}
		if visited[current.ID] {
			return nil
		}
		visiting[current.ID] = true
		defer delete(visiting, current.ID)
		for _, node := range current.Nodes {
			targetID := nestedWorkflowTarget(node)
			if targetID == "" || remoteDeviceNestedWorkflow(node) {
				continue
			}
			var target workflow.WorkflowDefinition
			if targetID == def.ID {
				target = def
			} else {
				item, ok := registry.Get(targetID)
				if !ok || item.Source != userWorkflowSource || ownedWorkflowUserID(item) != userID {
					continue
				}
				target = item
			}
			if err := walk(target); err != nil {
				return err
			}
		}
		visited[current.ID] = true
		return nil
	}
	return walk(def)
}

func analyzeWorkflowRisk(def workflow.WorkflowDefinition, registry *workflow.WorkflowRegistry, userID string) WorkflowSafetyAnalysis {
	analysis := WorkflowSafetyAnalysis{
		DeclaredPermissions: []string{},
		SecretReferences:    []string{},
		Risks:               []WorkflowRiskItem{},
		NestedDependencies:  []WorkflowNestedDependency{},
		HasSideEffects:      def.HasSideEffects,
		RiskLevel:           "low",
	}
	permissionSet := map[string]struct{}{}
	for _, permission := range def.Permissions {
		permission = strings.TrimSpace(permission)
		if permission != "" {
			permissionSet[permission] = struct{}{}
		}
	}
	secretSet := map[string]struct{}{}
	for _, node := range def.Nodes {
		for _, permission := range node.Permissions {
			permission = strings.TrimSpace(permission)
			if permission != "" {
				permissionSet[permission] = struct{}{}
			}
		}
		switch strings.ToLower(node.Type) {
		case "javascript", "wasm", "trusted_service":
			analysis.Risks = append(analysis.Risks, WorkflowRiskItem{Level: "high", NodeID: node.ID, Code: "executable_runtime", Message: "节点可执行代码或受信任运行时，启用前应确认来源和权限。"})
		case "mcp", "tool", "task":
			analysis.Risks = append(analysis.Risks, WorkflowRiskItem{Level: "medium", NodeID: node.ID, Code: "external_capability", Message: "节点会调用外部能力，实际权限由运行时和 Capability 策略决定。"})
		}
		if targetID := nestedWorkflowTarget(node); targetID != "" {
			dep := WorkflowNestedDependency{NodeID: node.ID, WorkflowID: targetID, Status: "missing"}
			if remoteDeviceNestedWorkflow(node) {
				// The device catalog is intentionally not mirrored into the Core's
				// Workflow Registry. Report a remote dependency rather than a false
				// "missing" finding; runtime control-plane checks remain authoritative.
				dep.Status = "remote_device"
			} else if registry != nil {
				if target, ok := registry.Get(targetID); ok {
					dep.Name = target.Name
					dep.DefinitionHash = target.DefinitionHash
					if target.Source == userWorkflowSource && ownedWorkflowUserID(target) == userID {
						dep.Status = "ok"
					} else {
						dep.Status = "forbidden"
					}
				}
			}
			analysis.NestedDependencies = append(analysis.NestedDependencies, dep)
		}
		collectWorkflowSecretRefs(node.Step.Input, secretSet)
		if node.Step.When != nil {
			collectWorkflowSecretRefs(*node.Step.When, secretSet)
		}
		collectWorkflowSecretRefs(node.Step.OnError.Default, secretSet)
		if node.Runtime.Metadata != nil {
			if raw, err := json.Marshal(node.Runtime.Metadata); err == nil {
				collectWorkflowSecretRefs(raw, secretSet)
			}
		}
	}
	for _, edge := range def.Edges {
		collectWorkflowSecretRefs(edge.Condition, secretSet)
	}
	for _, trigger := range def.Triggers {
		collectWorkflowSecretRefs(trigger.Config, secretSet)
		collectWorkflowSecretRefs(trigger.Input, secretSet)
	}
	for permission := range permissionSet {
		analysis.DeclaredPermissions = append(analysis.DeclaredPermissions, permission)
		lower := strings.ToLower(permission)
		if strings.Contains(lower, "write") || strings.Contains(lower, "execute") || strings.Contains(lower, "device") || strings.Contains(lower, "financial") {
			analysis.Risks = append(analysis.Risks, WorkflowRiskItem{Level: "high", Code: "declared_permission", Message: "工作流声明了高影响权限：" + permission})
		} else if strings.Contains(lower, "network") || strings.Contains(lower, "read") || strings.Contains(lower, "mcp") {
			analysis.Risks = append(analysis.Risks, WorkflowRiskItem{Level: "medium", Code: "declared_permission", Message: "工作流声明了权限：" + permission})
		}
	}
	for ref := range secretSet {
		analysis.SecretReferences = append(analysis.SecretReferences, ref)
	}
	sort.Strings(analysis.DeclaredPermissions)
	sort.Strings(analysis.SecretReferences)
	for _, risk := range analysis.Risks {
		if risk.Level == "high" {
			analysis.RiskLevel = "high"
			break
		}
		if risk.Level == "medium" && analysis.RiskLevel == "low" {
			analysis.RiskLevel = "medium"
		}
	}
	return analysis
}

func collectWorkflowSecretRefs(raw json.RawMessage, refs map[string]struct{}) {
	if len(raw) == 0 || refs == nil {
		return
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return
	}
	var walk func(any)
	walk = func(v any) {
		switch item := v.(type) {
		case map[string]any:
			for _, child := range item {
				walk(child)
			}
		case []any:
			for _, child := range item {
				walk(child)
			}
		case string:
			trimmed := strings.TrimSpace(item)
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "credential://") || strings.HasPrefix(lower, "secrets.") {
				refs[trimmed] = struct{}{}
			}
		}
	}
	walk(value)
}
