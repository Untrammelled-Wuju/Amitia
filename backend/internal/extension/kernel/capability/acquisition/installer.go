package acquisition

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// InstallerPort defines the external service interfaces that Installers need.
// These are minimal interfaces to avoid circular dependencies.
type (
	// PackageInstallPort invokes the real package install saga.
	PackageInstallPort interface {
		InstallPackage(ctx context.Context, extID string, version string, packageID string, hash string, userID string) (string, error)
		UninstallPackage(ctx context.Context, extID string, userID string) error
		ResolveArtifact(ctx context.Context, extID string, version string, packageURI string, hash string) (string, error)
	}

	// MCPInstallPort invokes the real MCP lifecycle.
	MCPInstallPort interface {
		InstallMCP(ctx context.Context, serverName string, transport string, command string, args []string, env map[string]string) (string, error)
		RemoveMCP(ctx context.Context, serverName string) error
	}

	// SkillInstallPort invokes the real AgentSkill service.
	SkillInstallPort interface {
		ImportSkill(ctx context.Context, sourceURI string, skillName string, hash string) (string, error)
		RemoveSkill(ctx context.Context, skillID string) error
	}

	// EnableExistingPort handles enablement of already-installed capabilities.
	EnableExistingPort interface {
		EnableExtension(ctx context.Context, extID string) error
		EnableSkill(ctx context.Context, skillID string) error
		EnableMCP(ctx context.Context, serverName string) error
	}
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
	Candidate           CapabilityCandidate             `json:"candidate"`
	Target              DeploymentTarget                `json:"target"`
	ExtensionIDs        []string                        `json:"extensionIds,omitempty"`
	ProviderIDs         []capability.ProviderID         `json:"providerIds,omitempty"`
	ProviderInstanceIDs []capability.ProviderInstanceID `json:"providerInstanceIds,omitempty"`
	CapabilityIDs       []capability.CapabilityID       `json:"capabilityIds,omitempty"`
	TransactionID       string                          `json:"transactionId"`
	InstalledAt         time.Time                       `json:"installedAt"`
}

// ---------------------------------------------------------------------------
// ExtensionPackageInstaller
// ---------------------------------------------------------------------------

// ExtensionPackageInstaller handles the InstallExtension method. It installs
// extension packages by delegating to the PackageInstallPort (real PackageInstallSaga).
type ExtensionPackageInstaller struct {
	packagePort PackageInstallPort
}

// NewExtensionPackageInstaller creates an ExtensionPackageInstaller with real dependencies.
func NewExtensionPackageInstaller(packagePort PackageInstallPort) *ExtensionPackageInstaller {
	return &ExtensionPackageInstaller{packagePort: packagePort}
}

func (*ExtensionPackageInstaller) Method() InstallMethod { return InstallExtension }

func (i *ExtensionPackageInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	if i.packagePort == nil {
		return InstalledCapability{}, fmt.Errorf("extension installer: package port not configured")
	}

	extID, _ := candidate.Metadata["extensionId"].(string)
	if extID == "" {
		return InstalledCapability{}, fmt.Errorf("extension installer: missing extensionId in candidate metadata")
	}

	version := candidate.Version
	packageURI := ""
	hash := ""
	if candidate.Install.ExtensionPackage != nil {
		packageURI = candidate.Install.ExtensionPackage.PackageURI
		hash = candidate.Install.ExtensionPackage.Hash
	}

	// Resolve PackageURI to ArtifactID before calling InstallPackage
	packageID := packageURI
	if packageURI != "" {
		artifactID, err := i.packagePort.ResolveArtifact(ctx, extID, version, packageURI, hash)
		if err != nil {
			return InstalledCapability{}, fmt.Errorf("extension installer: resolve artifact %s: %w", packageURI, err)
		}
		packageID = artifactID
	}

	installID, err := i.packagePort.InstallPackage(ctx, extID, version, packageID, hash, string(target.UserID))
	if err != nil {
		return InstalledCapability{}, fmt.Errorf("extension installer: install package %s: %w", extID, err)
	}

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		ExtensionIDs:  []string{extID},
		CapabilityIDs: candidate.Capabilities,
		TransactionID: installID,
		InstalledAt:   time.Now().UTC(),
	}

	if candidate.Metadata == nil {
		installed.Candidate.Metadata = make(map[string]any)
	}
	installed.Candidate.Metadata["newInstall"] = true
	installed.Candidate.Metadata["installTransactionID"] = installID

	return installed, nil
}

func (i *ExtensionPackageInstaller) Rollback(
	ctx context.Context,
	installed InstalledCapability,
) error {
	if i.packagePort == nil {
		return fmt.Errorf("extension installer rollback: package port not configured")
	}

	newInstall, _ := installed.Candidate.Metadata["newInstall"].(bool)
	if !newInstall {
		return nil
	}

	if len(installed.ExtensionIDs) == 0 {
		return nil
	}

	if err := i.packagePort.UninstallPackage(ctx, installed.ExtensionIDs[0], string(installed.Target.UserID)); err != nil {
		return fmt.Errorf("extension installer rollback: uninstall %s: %w", installed.ExtensionIDs[0], err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// MCPInstaller
// ---------------------------------------------------------------------------

// MCPInstaller handles the InstallMCP method. It delegates to MCPInstallPort.
type MCPInstaller struct {
	mcpPort MCPInstallPort
}

// NewMCPInstaller creates an MCPInstaller with real dependencies.
func NewMCPInstaller(mcpPort MCPInstallPort) *MCPInstaller {
	return &MCPInstaller{mcpPort: mcpPort}
}

func (*MCPInstaller) Method() InstallMethod { return InstallMCP }

func (i *MCPInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	if i.mcpPort == nil {
		return InstalledCapability{}, fmt.Errorf("MCP installer: MCP port not configured")
	}

	if candidate.Install.MCP == nil {
		return InstalledCapability{}, fmt.Errorf("candidate %s: MCP install descriptor is nil", candidate.ID)
	}

	desc := candidate.Install.MCP

	serverID, err := i.mcpPort.InstallMCP(ctx, desc.ServerName, desc.Transport, desc.Command, desc.Args, desc.Env)
	if err != nil {
		return InstalledCapability{}, fmt.Errorf("MCP installer: install %s: %w", desc.ServerName, err)
	}

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		CapabilityIDs: candidate.Capabilities,
		TransactionID: serverID,
		InstalledAt:   time.Now().UTC(),
	}

	return installed, nil
}

func (i *MCPInstaller) Rollback(
	ctx context.Context,
	installed InstalledCapability,
) error {
	if i.mcpPort == nil {
		return fmt.Errorf("MCP installer rollback: MCP port not configured")
	}

	desc := installed.Candidate.Install.MCP
	if desc == nil {
		return nil
	}

	if err := i.mcpPort.RemoveMCP(ctx, desc.ServerName); err != nil {
		return fmt.Errorf("MCP installer rollback: remove %s: %w", desc.ServerName, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// EnableExistingInstaller
// ---------------------------------------------------------------------------

// EnableExistingInstaller handles the InstallEnableExisting method. It delegates
// to EnableExistingPort to enable already-installed capabilities.
type EnableExistingInstaller struct {
	enablePort EnableExistingPort
}

// NewEnableExistingInstaller creates an EnableExistingInstaller with real dependencies.
func NewEnableExistingInstaller(enablePort EnableExistingPort) *EnableExistingInstaller {
	return &EnableExistingInstaller{enablePort: enablePort}
}

func (*EnableExistingInstaller) Method() InstallMethod { return InstallEnableExisting }

func (i *EnableExistingInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	if i.enablePort == nil {
		return InstalledCapability{}, fmt.Errorf("enable existing installer: enable port not configured")
	}

	if candidate.Metadata == nil {
		candidate.Metadata = make(map[string]any)
	}
	candidate.Metadata["enableOnly"] = true

	var err error
	var extID string
	var serverName string
	switch candidate.Kind {
	case CandidateExtensionPackage, CandidateInstalledExtension:
		extID, _ = candidate.Metadata["extensionId"].(string)
		if extID == "" {
			return InstalledCapability{}, fmt.Errorf("enable existing installer: missing extensionId")
		}
		err = i.enablePort.EnableExtension(ctx, extID)
	case CandidateAgentSkill:
		err = i.enablePort.EnableSkill(ctx, candidate.ID)
	case CandidateMCP:
		if candidate.Install.MCP != nil {
			serverName = candidate.Install.MCP.ServerName
		} else {
			serverName = candidate.ID
		}
		err = i.enablePort.EnableMCP(ctx, serverName)
	default:
		return InstalledCapability{}, fmt.Errorf("enable existing installer: unsupported candidate kind %s", candidate.Kind)
	}

	if err != nil {
		return InstalledCapability{}, fmt.Errorf("enable existing installer: enable %s: %w", candidate.ID, err)
	}

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		CapabilityIDs: candidate.Capabilities,
		InstalledAt:   time.Now().UTC(),
	}

	if extID != "" {
		installed.ExtensionIDs = []string{extID}
	}
	if serverName != "" {
		if installed.Candidate.Metadata == nil {
			installed.Candidate.Metadata = make(map[string]any)
		}
		installed.Candidate.Metadata["mcpServerName"] = serverName
	}

	return installed, nil
}

func (i *EnableExistingInstaller) Rollback(
	_ context.Context,
	_ InstalledCapability,
) error {
	return nil
}

// ---------------------------------------------------------------------------
// SkillInstaller
// ---------------------------------------------------------------------------

// SkillInstaller handles the InstallSkill method. It delegates to SkillInstallPort.
type SkillInstaller struct {
	skillPort SkillInstallPort
}

// NewSkillInstaller creates a SkillInstaller with real dependencies.
func NewSkillInstaller(skillPort SkillInstallPort) *SkillInstaller {
	return &SkillInstaller{skillPort: skillPort}
}

func (*SkillInstaller) Method() InstallMethod { return InstallSkill }

func (i *SkillInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	if i.skillPort == nil {
		return InstalledCapability{}, fmt.Errorf("skill installer: skill port not configured")
	}

	if candidate.Install.Skill == nil {
		return InstalledCapability{}, fmt.Errorf("candidate %s: skill install descriptor is nil", candidate.ID)
	}

	desc := candidate.Install.Skill

	skillID, err := i.skillPort.ImportSkill(ctx, desc.SourceURI, desc.SkillName, desc.Hash)
	if err != nil {
		return InstalledCapability{}, fmt.Errorf("skill installer: import skill %s: %w", desc.SkillName, err)
	}

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		CapabilityIDs: candidate.Capabilities,
		TransactionID: skillID,
		InstalledAt:   time.Now().UTC(),
	}
	if installed.Candidate.Metadata == nil {
		installed.Candidate.Metadata = make(map[string]any)
	}
	installed.Candidate.Metadata["skillId"] = skillID

	return installed, nil
}

func (i *SkillInstaller) Rollback(
	ctx context.Context,
	installed InstalledCapability,
) error {
	if i.skillPort == nil {
		return fmt.Errorf("skill installer rollback: skill port not configured")
	}

	skillID, _ := installed.Candidate.Metadata["skillId"].(string)
	if skillID == "" {
		return nil
	}

	if err := i.skillPort.RemoveSkill(ctx, skillID); err != nil {
		return fmt.Errorf("skill installer rollback: remove skill %s: %w", skillID, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// GeneratedSkillInstaller
// ---------------------------------------------------------------------------

// WorkshopGeneratePort defines the interface for generating skill content via Workshop.
// This avoids circular dependencies between acquisition and extension packages.
type WorkshopGeneratePort interface {
	GenerateInstruction(ctx context.Context, requirement string) (WorkshopInstructionDraft, error)
}

// WorkshopInstructionDraft represents a generated skill instruction draft.
type WorkshopInstructionDraft struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Body        string            `json:"body"`
	References  map[string]string `json:"references"`
	Assets      map[string]string `json:"assets"`
	DisplayName string            `json:"displayName"`
}

// GeneratedSkillInstaller handles the InstallGeneratedSkill method. It generates
// a skill via Workshop, validates it, then delegates to SkillInstallPort for registration.
type GeneratedSkillInstaller struct {
	skillPort    SkillInstallPort
	workshopPort WorkshopGeneratePort
}

// NewGeneratedSkillInstaller creates a GeneratedSkillInstaller with real dependencies.
// Both skillPort and workshopPort must be provided.
func NewGeneratedSkillInstaller(skillPort SkillInstallPort, workshopPort WorkshopGeneratePort) *GeneratedSkillInstaller {
	return &GeneratedSkillInstaller{skillPort: skillPort, workshopPort: workshopPort}
}

// NewGeneratedSkillInstallerWithWorkshop creates a GeneratedSkillInstaller with real dependencies.
// Both skillPort and workshopPort must be provided.
func NewGeneratedSkillInstallerWithWorkshop(skillPort SkillInstallPort, workshopPort WorkshopGeneratePort) *GeneratedSkillInstaller {
	return &GeneratedSkillInstaller{skillPort: skillPort, workshopPort: workshopPort}
}

func (*GeneratedSkillInstaller) Method() InstallMethod { return InstallGeneratedSkill }

// buildSkillMarkdown converts a WorkshopInstructionDraft into SKILL.md format.
func buildSkillMarkdown(draft WorkshopInstructionDraft) []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", draft.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", draft.Description))
	sb.WriteString(fmt.Sprintf("displayName: %s\n", draft.DisplayName))
	sb.WriteString("version: 1.0.0\n")
	sb.WriteString("schemaVersion: 2\n")
	sb.WriteString("---\n\n")
	sb.WriteString(draft.Body)
	sb.WriteString("\n")
	return []byte(sb.String())
}

func (i *GeneratedSkillInstaller) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (InstalledCapability, error) {
	if i.skillPort == nil {
		return InstalledCapability{}, fmt.Errorf("generated skill installer: skill port not configured")
	}

	if candidate.Install.GeneratedSkill == nil {
		return InstalledCapability{}, fmt.Errorf("candidate %s: generated skill descriptor is nil", candidate.ID)
	}

	desc := candidate.Install.GeneratedSkill

	// Step 1: Workshop Generate - must use real Workshop, not PromptStub directly
	if i.workshopPort == nil {
		return InstalledCapability{}, fmt.Errorf("generated skill installer: workshop port not configured, cannot generate skill")
	}

	requirement := desc.PromptStub
	if requirement == "" {
		requirement = desc.Description
	}
	if requirement == "" {
		return InstalledCapability{}, fmt.Errorf("generated skill installer: no requirement available for generation")
	}

	draft, err := i.workshopPort.GenerateInstruction(ctx, requirement)
	if err != nil {
		return InstalledCapability{}, fmt.Errorf("generated skill installer: workshop generate: %w", err)
	}

	// Step 2: Convert draft to SKILL.md format
	skillMarkdown := buildSkillMarkdown(draft)

	// Step 3: Write to temp file for source reading
	tmpFile, err := os.CreateTemp("", "generated_skill_*.md")
	if err != nil {
		return InstalledCapability{}, fmt.Errorf("generated skill installer: create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(skillMarkdown); err != nil {
		tmpFile.Close()
		return InstalledCapability{}, fmt.Errorf("generated skill installer: write temp file: %w", err)
	}
	tmpFile.Close()

	// Step 4: Import via SkillInstallPort (which now reads source → parser → validator → install)
	skillID, err := i.skillPort.ImportSkill(ctx, tmpFile.Name(), draft.Name, "")
	if err != nil {
		return InstalledCapability{}, fmt.Errorf("generated skill installer: import generated skill: %w", err)
	}

	installed := InstalledCapability{
		Candidate:     candidate,
		Target:        target,
		CapabilityIDs: candidate.Capabilities,
		TransactionID: skillID,
		InstalledAt:   time.Now().UTC(),
	}
	if installed.Candidate.Metadata == nil {
		installed.Candidate.Metadata = make(map[string]any)
	}
	installed.Candidate.Metadata["skillId"] = skillID
	installed.Candidate.Metadata["generated"] = true
	installed.Candidate.Metadata["generatedFrom"] = requirement

	return installed, nil
}

func (i *GeneratedSkillInstaller) Rollback(
	ctx context.Context,
	installed InstalledCapability,
) error {
	if i.skillPort == nil {
		return fmt.Errorf("generated skill installer rollback: skill port not configured")
	}

	skillID, _ := installed.Candidate.Metadata["skillId"].(string)
	if skillID == "" {
		return nil
	}

	if err := i.skillPort.RemoveSkill(ctx, skillID); err != nil {
		return fmt.Errorf("generated skill installer rollback: remove skill %s: %w", skillID, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
