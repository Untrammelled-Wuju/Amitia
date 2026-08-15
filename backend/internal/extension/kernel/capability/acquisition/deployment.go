package acquisition

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

// DeploymentPlanner decides where a candidate should be executed. The planner
// honours explicit placement constraints from the request and falls back to a
// safe default ("core") when no specific target is required.
type DeploymentPlanner struct{}

// NewDeploymentPlanner returns a new DeploymentPlanner.
func NewDeploymentPlanner() *DeploymentPlanner {
	return &DeploymentPlanner{}
}

// PlanTarget resolves a DeploymentTarget for the given candidate based on the
// placement constraints expressed in the acquisition request.
//
// Priority:
//  1. RequiredPlacement=device -> Target.DeviceID = RequiredDeviceID
//  2. RequiredPlacement=core   -> Target.Placement=core
//  3. RequiredDeviceID!=""     -> Target.DeviceID = RequiredDeviceID
//  4. RequiredRuntimeID!=""    -> Target.RuntimeID = RequiredRuntimeID
//  5. Fallback                 -> Placement=core
func (p *DeploymentPlanner) PlanTarget(
	ctx context.Context,
	candidate CapabilityCandidate,
	request AcquisitionRequest,
) (DeploymentTarget, error) {
	target := DeploymentTarget{
		UserID: request.UserID,
	}

	switch {
	case request.RequiredPlacement == capability.ProviderPlacementDevice:
		target.Placement = capability.ProviderPlacementDevice
		target.DeviceID = request.RequiredDeviceID
		target.RuntimeID = request.RequiredRuntimeID
	case request.RequiredPlacement == capability.ProviderPlacementCore:
		target.Placement = capability.ProviderPlacementCore
		target.RuntimeID = request.RequiredRuntimeID
	case request.RequiredDeviceID != "":
		target.Placement = capability.ProviderPlacementDevice
		target.DeviceID = request.RequiredDeviceID
		target.RuntimeID = request.RequiredRuntimeID
	case request.RequiredRuntimeID != "":
		target.RuntimeID = request.RequiredRuntimeID
		if deviceFromRuntimeID(target.RuntimeID) != "" {
			target.Placement = capability.ProviderPlacementDevice
		} else {
			target.Placement = capability.ProviderPlacementCore
		}
	case request.PreferredPlacement == capability.ProviderPlacementDevice:
		target.Placement = capability.ProviderPlacementDevice
		target.DeviceID = request.PreferredDeviceID
	default:
		target.Placement = capability.ProviderPlacementCore
	}

	if target.RuntimeID == "" && request.PreferredRuntimeID != "" {
		target.RuntimeID = request.PreferredRuntimeID
	}

	return target, nil
}

// deviceFromRuntimeID is a placeholder resolver that extracts a device ID
// reference from a runtime identifier. The real implementation would resolve
// the runtime binding; here we return empty so the core fallback is used
// unless an explicit device is already set.
func deviceFromRuntimeID(runtimeID runtimeidentity.RuntimeID) string {
	return ""
}
