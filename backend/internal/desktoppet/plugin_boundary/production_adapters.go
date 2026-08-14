package plugin_boundary

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/integration"
)

type productionAdapters struct {
	resource       *integration.ResourceProductionAdapter
	action         *integration.ActionProductionAdapter
	runtime        *integration.RuntimeProductionAdapter
	floatingWindow *integration.FloatingWindowProductionAdapter
}

func newProductionAdapters(caps integration.DesktopPetPluginCapabilities) (*productionAdapters, error) {
	if err := caps.Validate(); err != nil {
		return nil, err
	}
	return &productionAdapters{
		resource:       integration.NewResourceProductionAdapter(caps.Resource),
		action:         integration.NewActionProductionAdapter(caps.Action, caps.ActionTarget),
		runtime:        integration.NewRuntimeProductionAdapter(caps.Runtime),
		floatingWindow: integration.NewFloatingWindowProductionAdapter(caps.FloatingWindow),
	}, nil
}

func (p *productionAdapters) toAdapters() []ContributionAdapter {
	return []ContributionAdapter{
		&productionResourceAdapter{adapt: p.resource},
		&productionActionAdapter{adapt: p.action},
		&productionRuntimeAdapter{adapt: p.runtime},
		&productionFloatingWindowAdapter{adapt: p.floatingWindow},
	}
}

type productionResourceAdapter struct {
	adapt *integration.ResourceProductionAdapter
}

func (a *productionResourceAdapter) Kind() ContributionKind { return KindResource }

func (a *productionResourceAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
	displayName, _ := def["displayName"].(string)
	if displayName == "" {
		return fmt.Errorf("%w: resource.displayName required", ErrValidationFailed)
	}
	assetKind, _ := def["assetKind"].(string)
	if assetKind != "" && !isValidAssetKind(assetKind) {
		return fmt.Errorf("%w: resource.assetKind invalid: %s", ErrValidationFailed, assetKind)
	}
	if refRef, ok := def["resourceRef"].(string); ok {
		if err := validateResourceRef(refRef); err != nil {
			return fmt.Errorf("%w: %v", ErrValidationFailed, err)
		}
	}
	return nil
}

func (a *productionResourceAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	displayName, _ := def["displayName"].(string)
	description, _ := def["description"].(string)
	assetKind, _ := def["assetKind"].(string)
	mimeCategory, _ := def["mimeCategory"].(string)
	contentType, _ := def["contentType"].(string)
	resourceRef, _ := def["resourceRef"].(string)

	req := integration.PluginResourceAttachRequest{
		ExtensionID:    string(ref.ExtensionID),
		PluginID:       string(ref.PluginID),
		ContributionID: ref.ContributionID,
		Revision:       rev,
		Definition:     cloneMap(def),
	}

	handle, err := a.adapt.Attach(ctx, req)
	if err != nil {
		return ContributionRegistration{}, fmt.Errorf("%w: attach resource: %v", ErrUnavailable, err)
	}

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	return ContributionRegistration{
		Ref:             ref,
		Kind:            KindResource,
		Revision:        rev,
		Status:          status,
		Definition:      cloneMap(def),
		Resource:        &ResourceDescriptor{ContributionRefJSON: ref.ToJSON(), DisplayName: displayName, Description: description, AssetKind: assetKind, MimeCategory: mimeCategory, ContentType: contentType, ResourceRef: resourceRef},
		CanonicalHandle: handle.String(),
	}, nil
}

func (a *productionResourceAdapter) Detach(ctx context.Context, reg ContributionRegistration) error {
	if reg.CanonicalHandle == "" {
		return fmt.Errorf("%w: resource %s has no canonical handle", ErrDetachFailed, reg.Ref.Key())
	}
	return a.adapt.Detach(ctx, integration.PetResourceHandle(reg.CanonicalHandle))
}

type productionActionAdapter struct {
	adapt *integration.ActionProductionAdapter
}

func (a *productionActionAdapter) Kind() ContributionKind { return KindAction }

func (a *productionActionAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
	actionKey, _ := def["actionKey"].(string)
	if actionKey == "" {
		return fmt.Errorf("%w: action.actionKey required", ErrValidationFailed)
	}
	if !isValidActionKey(actionKey) {
		return fmt.Errorf("%w: action.actionKey invalid format: %s", ErrValidationFailed, actionKey)
	}
	displayName, _ := def["displayName"].(string)
	if displayName == "" {
		return fmt.Errorf("%w: action.displayName required", ErrValidationFailed)
	}
	if playbackMode, ok := def["playbackMode"].(string); ok && playbackMode != "" {
		if !isValidPlaybackMode(playbackMode) {
			return fmt.Errorf("%w: action.playbackMode invalid: %s", ErrValidationFailed, playbackMode)
		}
	}
	return nil
}

func (a *productionActionAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	actionKey, _ := def["actionKey"].(string)
	displayName, _ := def["displayName"].(string)
	description, _ := def["description"].(string)
	categoryKey, _ := def["categoryKey"].(string)
	playbackMode, _ := def["playbackMode"].(string)
	requiredResource, _ := def["requiredResource"].(string)

	target, err := a.adapt.ResolveTarget(ctx, string(ref.ExtensionID), ref.ContributionID, rev)
	if err != nil {
		return ContributionRegistration{}, fmt.Errorf("%w: resolve action target: %v", ErrUnavailable, err)
	}

	req := integration.PluginActionAttachRequest{
		ExtensionID:    string(ref.ExtensionID),
		PluginID:       string(ref.PluginID),
		ContributionID: ref.ContributionID,
		Revision:       rev,
		Target:         target,
		Definition:     cloneMap(def),
	}

	handle, err := a.adapt.Attach(ctx, req)
	if err != nil {
		return ContributionRegistration{}, fmt.Errorf("%w: attach action: %v", ErrUnavailable, err)
	}

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	return ContributionRegistration{
		Ref:             ref,
		Kind:            KindAction,
		Revision:        rev,
		Status:          status,
		Definition:      cloneMap(def),
		Action:          &ActionDescriptor{ContributionRefJSON: ref.ToJSON(), ActionKey: actionKey, DisplayName: displayName, Description: description, CategoryKey: categoryKey, PlaybackMode: playbackMode, RequiredResource: requiredResource, Interruptible: true},
		CanonicalHandle: handle.String(),
	}, nil
}

func (a *productionActionAdapter) Detach(ctx context.Context, reg ContributionRegistration) error {
	if reg.CanonicalHandle == "" {
		return fmt.Errorf("%w: action %s has no canonical handle", ErrDetachFailed, reg.Ref.Key())
	}
	return a.adapt.Detach(ctx, integration.PetActionHandle(reg.CanonicalHandle))
}

type productionRuntimeAdapter struct {
	adapt *integration.RuntimeProductionAdapter
}

func (a *productionRuntimeAdapter) Kind() ContributionKind { return KindRuntime }

func (a *productionRuntimeAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
	capabilityID, _ := def["capabilityId"].(string)
	if capabilityID == "" {
		return fmt.Errorf("%w: runtime.capabilityId required", ErrValidationFailed)
	}
	capabilityKind, _ := def["capabilityKind"].(string)
	if capabilityKind == "" {
		return fmt.Errorf("%w: runtime.capabilityKind required", ErrValidationFailed)
	}
	if !isValidRuntimeCapabilityKind(capabilityKind) {
		return fmt.Errorf("%w: runtime.capabilityKind invalid: %s", ErrValidationFailed, capabilityKind)
	}
	return nil
}

func (a *productionRuntimeAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	capabilityID, _ := def["capabilityId"].(string)
	capabilityKind, _ := def["capabilityKind"].(string)
	version, _ := def["version"].(string)

	req := integration.PluginRuntimeAttachRequest{
		ExtensionID:    string(ref.ExtensionID),
		PluginID:       string(ref.PluginID),
		ContributionID: ref.ContributionID,
		Revision:       rev,
		Definition:     cloneMap(def),
	}

	handle, err := a.adapt.Attach(ctx, req)
	if err != nil {
		return ContributionRegistration{}, fmt.Errorf("%w: attach runtime capability: %v", ErrUnavailable, err)
	}

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	return ContributionRegistration{
		Ref:             ref,
		Kind:            KindRuntime,
		Revision:        rev,
		Status:          status,
		Definition:      cloneMap(def),
		Runtime:         &RuntimeCapabilityDescriptor{ContributionRefJSON: ref.ToJSON(), CapabilityID: capabilityID, CapabilityKind: capabilityKind, Version: version},
		CanonicalHandle: handle.String(),
	}, nil
}

func (a *productionRuntimeAdapter) Detach(ctx context.Context, reg ContributionRegistration) error {
	if reg.CanonicalHandle == "" {
		return fmt.Errorf("%w: runtime %s has no canonical handle", ErrDetachFailed, reg.Ref.Key())
	}
	return a.adapt.Detach(ctx, integration.PetRuntimeHandle(reg.CanonicalHandle))
}

type productionFloatingWindowAdapter struct {
	adapt *integration.FloatingWindowProductionAdapter
}

func (a *productionFloatingWindowAdapter) Kind() ContributionKind { return KindFloatingWindow }

func (a *productionFloatingWindowAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
	return nil
}

func (a *productionFloatingWindowAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	req := integration.PluginFloatingWindowAttachRequest{
		ExtensionID:    string(ref.ExtensionID),
		PluginID:       string(ref.PluginID),
		ContributionID: ref.ContributionID,
		Revision:       rev,
		Definition:     cloneMap(def),
	}

	handle, err := a.adapt.Attach(ctx, req)
	if err != nil {
		return ContributionRegistration{}, fmt.Errorf("%w: attach floating window capability: %v", ErrUnavailable, err)
	}

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	fd := FloatingWindowCapabilityDescriptor{
		ContributionRefJSON: ref.ToJSON(),
		SupportsShow:        boolDef(def, "supportsShow", true),
		SupportsHide:        boolDef(def, "supportsHide", true),
		SupportsPosition:    boolDef(def, "supportsPosition", false),
		SupportsSize:        boolDef(def, "supportsSize", false),
		SupportsOpacity:     boolDef(def, "supportsOpacity", false),
		SupportsContent:     boolDef(def, "supportsContent", false),
	}

	return ContributionRegistration{
		Ref:             ref,
		Kind:            KindFloatingWindow,
		Revision:        rev,
		Status:          status,
		Definition:      cloneMap(def),
		Window:          &fd,
		CanonicalHandle: handle.String(),
	}, nil
}

func (a *productionFloatingWindowAdapter) Detach(ctx context.Context, reg ContributionRegistration) error {
	if reg.CanonicalHandle == "" {
		return fmt.Errorf("%w: floating window %s has no canonical handle", ErrDetachFailed, reg.Ref.Key())
	}
	return a.adapt.Detach(ctx, integration.PetFloatingWindowHandle(reg.CanonicalHandle))
}
