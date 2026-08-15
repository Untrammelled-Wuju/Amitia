package acquisition

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// InstallOrchestrator selects the appropriate Installer for a candidate and
// drives the install stage of an acquisition transaction. On success it
// returns an AcquisitionResult in StateReady; on failure it invokes the
// installer's Rollback and returns an AcquisitionResult in StateRolledBack
// with the Error field populated.
type InstallOrchestrator struct {
	installers map[InstallMethod]Installer
}

// NewInstallOrchestrator creates a fully initialized InstallOrchestrator
// with all built-in installers registered.
func NewInstallOrchestrator() *InstallOrchestrator {
	return &InstallOrchestrator{
		installers: map[InstallMethod]Installer{
			InstallExtension:      &ExtensionPackageInstaller{},
			InstallMCP:            &MCPInstaller{},
			InstallSkill:          &SkillInstaller{},
			InstallEnableExisting: &EnableExistingInstaller{},
		},
	}
}

// Install selects the correct Installer based on the candidate's install
// method, executes the install, and translates the result. If the install
// fails, the installed capability is rolled back (if any) and the returned
// AcquisitionResult reflects StateRolledBack.
func (o *InstallOrchestrator) Install(
	ctx context.Context,
	candidate CapabilityCandidate,
	target DeploymentTarget,
) (*AcquisitionResult, error) {
	method := resolveInstallMethod(candidate)
	installer, ok := o.installers[method]
	if !ok {
		return nil, fmt.Errorf("no installer registered for method %q", method)
	}

	installed, err := installer.Install(ctx, candidate, target)
	if err != nil {
		// Best-effort rollback even on partial install (InstalledCapability
		// may be zero-valued, in which case rollback is a no-op).
		_ = installer.Rollback(ctx, installed)
		result := &AcquisitionResult{
			State:       StateRolledBack,
			CandidateID: candidate.ID,
			Target:      target,
			Error:       fmt.Sprintf("install failed (%s): %v", method, err),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if installed.TransactionID != "" {
			result.TransactionID = installed.TransactionID
		}
		return result, err
	}

	result := toAcquisitionResult(installed)
	return result, nil
}

// resolveInstallMethod determines which InstallMethod should be used for the
// given candidate. The candidate's explicit Install.Method takes priority;
// otherwise the method is inferred from the candidate Kind.
func resolveInstallMethod(candidate CapabilityCandidate) InstallMethod {
	if candidate.Install.Method != "" {
		return candidate.Install.Method
	}

	switch candidate.Kind {
	case CandidateExtensionPackage:
		return InstallExtension
	case CandidateMCP:
		return InstallMCP
	case CandidateAgentSkill:
		return InstallSkill
	case CandidateGeneratedSkill:
		return InstallGeneratedSkill
	case CandidateInstalledExtension:
		return InstallEnableExisting
	default:
		return InstallEnableExisting
	}
}

// toAcquisitionResult translates an InstalledCapability produced by an
// Installer into the user-facing AcquisitionResult type with State set to
// StateReady.
func toAcquisitionResult(installed InstalledCapability) *AcquisitionResult {
	now := time.Now().UTC()
	result := &AcquisitionResult{
		State:                  StateReady,
		TransactionID:          installed.TransactionID,
		CandidateID:            installed.Candidate.ID,
		Installed:              true,
		Enabled:                true,
		Target:                 installed.Target,
		CapabilityIDs:          installed.CapabilityIDs,
		ProviderIDs:            installed.ProviderIDs,
		ProviderInstanceIDs:    installed.ProviderInstanceIDs,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	// If the installer set the enableOnly flag, the capability was not
	// freshly installed - only re-enabled.
	if enableOnly, _ := installed.Candidate.Metadata["enableOnly"].(bool); enableOnly {
		result.Installed = false
	}

	return result
}

// Ensure capability types are referenced even if not directly used in this
// file. This makes the import explicit for future maintainers.
var _ capability.CapabilityID
