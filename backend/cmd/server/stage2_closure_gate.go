package main

import (
	"context"
	"fmt"

	migrationcore "github.com/u-ai/backend/internal/migration"
)

// Stage2ClosureGateAdapter is the server-facing adapter for the single canonical
// migration Stage2 closure gate. It intentionally owns no independent
// manifest, evidence records, or validation rules.
type Stage2ClosureGateAdapter struct {
	services    *AppServices
	runtimeGate *migrationcore.RuntimeArchitectureGate
	closureGate *migrationcore.Stage2ClosureGate
}

func NewStage2ClosureGateAdapter(services *AppServices) *Stage2ClosureGateAdapter {
	var provider migrationcore.CanonicalAuthorityProvider
	if services != nil {
		provider = &kernelContainerAuthorityProvider{container: services.KernelContainer, services: services}
	}
	runtimeGate := migrationcore.NewRuntimeArchitectureGate(provider, currentRuntimePlatform())
	closureGate := migrationcore.NewStage2ClosureGate(
		migrationcore.NewEvidenceLoader(resolveStage2EvidencePath()),
		runtimeGate,
	)
	return NewStage2ClosureGateAdapterFromCanonical(services, runtimeGate, closureGate)
}

func NewStage2ClosureGateAdapterFromCanonical(
	services *AppServices,
	runtimeGate *migrationcore.RuntimeArchitectureGate,
	closureGate *migrationcore.Stage2ClosureGate,
) *Stage2ClosureGateAdapter {
	return &Stage2ClosureGateAdapter{services: services, runtimeGate: runtimeGate, closureGate: closureGate}
}

type ArchitectureReadiness struct {
	Ready   bool
	Reasons []string
}

func (g *Stage2ClosureGateAdapter) ArchitectureReady() ArchitectureReadiness {
	if g == nil || g.runtimeGate == nil {
		return ArchitectureReadiness{Ready: false, Reasons: []string{"RuntimeArchitectureGate: nil"}}
	}
	ready, reasons := g.runtimeGate.Check(context.Background())
	return ArchitectureReadiness{Ready: ready, Reasons: reasons}
}

func (g *Stage2ClosureGateAdapter) ValidateG0() (bool, []string) {
	if g == nil || g.closureGate == nil {
		return false, []string{"Stage2ClosureGate: nil"}
	}
	ok, reasons, err := g.closureGate.ValidateG0(context.Background())
	if err != nil {
		reasons = append(reasons, err.Error())
		return false, reasons
	}
	return ok, reasons
}

func (g *Stage2ClosureGateAdapter) CanRunCutover() (bool, []string) {
	ok, reasons := g.ValidateG0()
	if g == nil || g.services == nil {
		reasons = append(reasons, "services not initialized")
		return false, reasons
	}
	if g.services.DB == nil {
		reasons = append(reasons, "database not initialized")
		ok = false
	}
	return ok && len(reasons) == 0, reasons
}

func (g *Stage2ClosureGateAdapter) FailureMessage(reasons []string) string {
	msg := "closure gate: Stage 2 G0 not ready"
	for _, reason := range reasons {
		msg += "\n  - " + reason
	}
	return msg
}

// Kept for compatibility with existing tests/callers. Production startup no
// longer uses fresh-install status to bypass the closure gate.
func assertFreshInstallCannotBypassCutover(isFreshInstall, closureReady bool) error {
	if isFreshInstall && !closureReady {
		return fmt.Errorf("fresh install cannot bypass Stage 2 closure verification")
	}
	return nil
}
