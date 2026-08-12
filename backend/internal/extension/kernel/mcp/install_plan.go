// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type MCPRuntimeDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
}

type MCPInstallScriptPreview struct {
	HasScripts   bool     `json:"hasScripts"`
	ScriptTypes  []string `json:"scriptTypes,omitempty"`
	ScriptRisk   string   `json:"scriptRisk"`
}

type MCPInstallPlan struct {
	PlanID            string                   `json:"planId"`
	BindingID         string                   `json:"bindingId"`
	Source            string                   `json:"source"`
	Transport         string                   `json:"transport"`
	Launcher          string                   `json:"launcher"`
	RequestedPackage  string                   `json:"requestedPackage"`
	RequestedVersion  string                   `json:"requestedVersion"`
	RuntimeDependencies []MCPRuntimeDependency `json:"runtimeDependencies"`
	NetworkEndpoints  []string                 `json:"networkEndpoints"`
	InstallScripts    MCPInstallScriptPreview  `json:"installScripts"`
	Permissions       []string                 `json:"permissions"`
	Risk              string                   `json:"risk"`
	RequiresApproval  bool                     `json:"requiresApproval"`
	PlanDigest        string                   `json:"planDigest"`
	ExpiresAt         time.Time                `json:"expiresAt"`
}

func (p MCPInstallPlan) ComputeDigest() string {
	var sb strings.Builder
	sb.WriteString(p.PlanID)
	sb.WriteString("|")
	sb.WriteString(p.BindingID)
	sb.WriteString("|")
	sb.WriteString(p.Source)
	sb.WriteString("|")
	sb.WriteString(p.Transport)
	sb.WriteString("|")
	sb.WriteString(p.Launcher)
	sb.WriteString("|")
	sb.WriteString(p.RequestedPackage)
	sb.WriteString("|")
	sb.WriteString(p.RequestedVersion)
	sb.WriteString("|")
	sb.WriteString(p.Risk)
	for _, perm := range p.Permissions {
		sb.WriteString("|")
		sb.WriteString(perm)
	}
	for _, dep := range p.RuntimeDependencies {
		sb.WriteString("|")
		sb.WriteString(dep.Name)
		sb.WriteString("@")
		sb.WriteString(dep.Version)
	}
	for _, ep := range p.NetworkEndpoints {
		sb.WriteString("|")
		sb.WriteString(ep)
	}
	if p.InstallScripts.HasScripts {
		sb.WriteString("|scripts:")
		sb.WriteString(p.InstallScripts.ScriptRisk)
	}

	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:])
}

func (p MCPInstallPlan) VerifyDigest() bool {
	return p.PlanDigest == p.ComputeDigest()
}

func (p MCPInstallPlan) IsExpired(now time.Time) bool {
	return !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt)
}

type PlanInvalidError struct {
	PlanID string
	Reason string
}

func (e *PlanInvalidError) Error() string {
	return fmt.Errorf("MCP_INSTALL_PLAN_INVALID: plan %s: %s", e.PlanID, e.Reason).Error()
}

type PlanExpiredError struct {
	PlanID string
}

func (e *PlanExpiredError) Error() string {
	return fmt.Errorf("MCP_INSTALL_PLAN_EXPIRED: plan %s has expired", e.PlanID).Error()
}

type PlanChangedError struct {
	PlanID string
}

func (e *PlanChangedError) Error() string {
	return fmt.Errorf("MCP_INSTALL_PLAN_CHANGED: plan %s digest mismatch", e.PlanID).Error()
}

type ApprovalRequiredError struct {
	PlanID string
}

func (e *ApprovalRequiredError) Error() string {
	return fmt.Errorf("MCP_INSTALL_APPROVAL_REQUIRED: plan %s requires approval", e.PlanID).Error()
}
