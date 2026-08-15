package acquisition

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// Installer is the abstraction that executes a specific install method for a
// capability candidate. Each Installer is responsible for one installation
// strategy (extension package, MCP server, skill, etc.) and supports rollback
// when the overall acquisition transaction fails.
type Installer interface {
	// Method returns the InstallMethod this installer handles.
	Method() InstallMethod

	// Install executes the installation of candidate into target, returning an
	// InstalledCapability describing what was materialized.
	Install(ctx context.Context, candidate CapabilityCandidate, target DeploymentTarget) (InstalledCapability, error)

	// Rollback undoes a previously successful Install. It is called by the
	// orchestrator when a later stage of the acquisition transaction fails.
	Rollback(ctx context.Context, installed InstalledCapability) error
}

// InstalledCapability records the concrete artifacts produced by an Installer.
// It is the internal representation passed between install stages and is later
// translated into a user-facing AcquisitionResult by the orchestrator.
type InstalledCapability struct {
	Candidate           CapabilityCandidate       `json:"candidate"`
	Target              DeploymentTarget          `json:"target"`
	ExtensionIDs        []string                  `json:"extensionIds,omitempty"`
	ProviderIDs         []capability.ProviderID   `json:"providerIds,omitempty"`
	ProviderInstanceIDs []capability.ProviderInstanceID `json:"providerInstanceIds,omitempty"`
	CapabilityIDs       []capability.CapabilityID `json:"capabilityIds,omitempty"`
	TransactionID       string                    `json:"transactionId"`
	InstalledAt         time.Time                 `json:"installedAt"`
}

// ---------------------------------------------------------------------------
// ExtensionPackageInstaller
// ---------------------------------------------------------------------------

// ExtensionPackageInstaller handles the InstallExtension method. It installs
// extension packages and reconciles the resulting providers. The actual
// package install code delegates to a PackageInstallSaga (not yet
// implemented); this installer fills in the InstalledCapability metadata
// and handles already-installed / disabled-extension edge cases.
type ExtensionPackageInstaller struct{}

func (*ExtensionPackageInstaller) Method() InstallMethod { return InstallExtension }

func (i *ExtensionPackageInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	extID, _ := candidate.Metadata["extensionId"].(string)

	// Detect already-installed extension.
	if extID != "" {
		disabled, _ := candidate.Metadata["disabled"].(bool)
		if disabled {
			// Re-enabling a disabled extension rather than reinstalling.
			if candidate.Metadata == nil {
				candidate.Metadata = make(map[string]any)
			}
			candidate.Metadata["requiresEnable"] = true
		}
	}

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		ExtensionIDs:  []string{extID},
		CapabilityIDs: candidate.Capabilities,
		InstalledAt:   time.Now().UTC(),
	}

	if candidate.Metadata == nil {
		installed.Candidate.Metadata = make(map[string]any)
	}
	installed.Candidate.Metadata["newInstall"] = true

	// Placeholder: actual install code would invoke PackageInstallSaga here.
	// The saga would download the extension package, validate its signature,
	// register the extension, and reconcile providers. We record that this
	// branch should have been taken via metadata so downstream stages can
	// differentiate a fresh install from a re-enable.
	_ = ctx

	return installed, nil
}

func (i *ExtensionPackageInstaller) Rollback(
	ctx context.Context,
	installed InstalledCapability,
) error {
	// Only uninstall if this was a fresh install (not a re-enable).
	newInstall, _ := installed.Candidate.Metadata["newInstall"].(bool)
	if !newInstall {
		return nil
	}

	// Placeholder: actual code would call UninstallExtension here.
	// e.g., return extensionStore.Uninstall(ctx, installed.ExtensionIDs[0])
	_ = ctx
	return nil
}

// ---------------------------------------------------------------------------
// MCPInstaller
// ---------------------------------------------------------------------------

// MCPInstaller handles the InstallMCP method. It converts an
// MCPInstallDescriptor into a ServerConfig and registers the MCP server.
type MCPInstaller struct{}

func (*MCPInstaller) Method() InstallMethod { return InstallMCP }

func (i *MCPInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	if candidate.Install.MCP == nil {
		return InstalledCapability{}, fmt.Errorf("candidate %s: MCP install descriptor is nil", candidate.ID)
	}

	desc := candidate.Install.MCP

	// Convert MCPInstallDescriptor into a ServerConfig (placeholder type).
	// In the real implementation this would be registered with the MCP
	// runtime / server registry.
	serverConfig := map[string]any{
		"serverName": desc.ServerName,
		"transport":  desc.Transport,
		"command":    desc.Command,
		"args":       desc.Args,
		"env":        desc.Env,
		"registry":   desc.Registry,
	}

	// Placeholder: actual registration would happen here.
	// e.g., serverID, err := mcpRegistry.Register(ctx, serverConfig)
	_ = serverConfig
	_ = ctx

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		CapabilityIDs: candidate.Capabilities,
		InstalledAt:   time.Now().UTC(),
	}

	return installed, nil
}

func (i *MCPInstaller) Rollback(
	ctx context.Context,
	installed InstalledCapability,
) error {
	desc := installed.Candidate.Install.MCP
	if desc == nil {
		return nil
	}

	// Placeholder: actual code would call RemoveServer here.
	// e.g., return mcpRegistry.RemoveServer(ctx, desc.ServerName)
	_ = ctx
	return nil
}

// ---------------------------------------------------------------------------
// EnableExistingInstaller
// ---------------------------------------------------------------------------

// EnableExistingInstaller handles the InstallEnableExisting method. It does
// not install anything new; it only flips the disabled flag on an existing
// ProviderDefinition to false so the capability becomes usable again.
type EnableExistingInstaller struct{}

func (*EnableExistingInstaller) Method() InstallMethod { return InstallEnableExisting }

func (i *EnableExistingInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	// Mark this as an enable-only operation so the orchestrator knows not to
	// modify core/metadata or attempt a fresh install.
	if candidate.Metadata == nil {
		candidate.Metadata = make(map[string]any)
	}
	candidate.Metadata["enableOnly"] = true

	// Placeholder: actual code would set ProviderDefinition.disabled = false.
	// e.g., err := providerStore.Enable(ctx, candidate.ID)
	_ = ctx

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		CapabilityIDs: candidate.Capabilities,
		InstalledAt:   time.Now().UTC(),
	}

	return installed, nil
}

func (i *EnableExistingInstaller) Rollback(
	_ context.Context,
	_ InstalledCapability,
) error {
	// Deliberately a no-op: we do not want to re-disable a provider that was
	// previously enabled by the user outside this transaction.
	return nil
}

// ---------------------------------------------------------------------------
// SkillInstaller
// ---------------------------------------------------------------------------

// SkillInstaller handles the InstallSkill method. It writes the skill source
// into a dedicated skill directory and creates a manifest.json describing
// the skill.
type SkillInstaller struct{}

func (*SkillInstaller) Method() InstallMethod { return InstallSkill }

func (i *SkillInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	if candidate.Install.Skill == nil {
		return InstalledCapability{}, fmt.Errorf("candidate %s: skill install descriptor is nil", candidate.ID)
	}

	desc := candidate.Install.Skill

	// Determine the skill directory path.
	skillDir := skillDirectoryPath(candidate, desc)

	// Placeholder: actual code would write the skill source files into
	// skillDir here.
	// e.g., err := skillStore.WriteSource(ctx, skillDir, desc.SourceURI)
	_ = ctx

	// Write manifest.json describing the skill.
	manifest := map[string]any{
		"name":        firstNonEmpty(desc.SkillName, candidate.Name),
		"version":     candidate.Version,
		"description": candidate.Description,
		"sourceUri":   desc.SourceURI,
		"hash":        desc.Hash,
		"installedAt": time.Now().UTC().Format(time.RFC3339),
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return InstalledCapability{}, fmt.Errorf("marshal skill manifest: %w", err)
	}

	// Placeholder: actual code would persist manifestJSON to
	// filepath.Join(skillDir, "manifest.json").
	_ = manifestJSON

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		CapabilityIDs: candidate.Capabilities,
		InstalledAt:   time.Now().UTC(),
	}
	if installed.Candidate.Metadata == nil {
		installed.Candidate.Metadata = make(map[string]any)
	}
	installed.Candidate.Metadata["skillDir"] = skillDir

	return installed, nil
}

func (i *SkillInstaller) Rollback(
	ctx context.Context,
	installed InstalledCapability,
) error {
	skillDir, _ := installed.Candidate.Metadata["skillDir"].(string)
	if skillDir == "" {
		return nil
	}

	// Placeholder: actual code would delete the skill directory here.
	// e.g., return os.RemoveAll(skillDir)
	_ = ctx
	return nil
}

// ---------------------------------------------------------------------------
// GeneratedSkillInstaller (placeholder)
// ---------------------------------------------------------------------------

// GeneratedSkillInstaller is a placeholder for the InstallGeneratedSkill
// method. The full implementation will generate a skill from a prompt stub
// and register it.
type GeneratedSkillInstaller struct{}

func (*GeneratedSkillInstaller) Method() InstallMethod { return InstallGeneratedSkill }

func (i *GeneratedSkillInstaller) Install(
	_ context.Context,
	candidate CapabilityCandidate,
	_ DeploymentTarget,
) (InstalledCapability, error) {
	return InstalledCapability{
		Candidate:   candidate,
		InstalledAt: time.Now().UTC(),
	}, nil
}

func (i *GeneratedSkillInstaller) Rollback(
	_ context.Context,
	_ InstalledCapability,
) error {
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func skillDirectoryPath(candidate CapabilityCandidate, desc *SkillInstallDescriptor) string {
	name := firstNonEmpty(desc.SkillName, candidate.Name, "unnamed-skill")
	// In the real implementation this would resolve to a proper skills
	// directory under the user's workspace.
	return "/skills/" + name
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
