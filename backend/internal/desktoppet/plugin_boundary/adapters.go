package plugin_boundary

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type noopAdapter struct {
	KindField ContributionKind
}

func (a *noopAdapter) Kind() ContributionKind { return a.KindField }
func (a *noopAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	return ContributionRegistration{
		Ref:        ref,
		Kind:       a.KindField,
		Revision:   rev,
		Status:     ContributionStatusRegistered,
		Definition: cloneMap(def),
	}, nil
}
func (a *noopAdapter) Detach(ctx context.Context, ref ContributionRef) error  { return nil }
func (a *noopAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
	return nil
}

type resourceContributionAdapter struct {
}

func (a *resourceContributionAdapter) Kind() ContributionKind { return KindResource }

func (a *resourceContributionAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
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

func (a *resourceContributionAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	displayName, _ := def["displayName"].(string)
	description, _ := def["description"].(string)
	assetKind, _ := def["assetKind"].(string)
	mimeCategory, _ := def["mimeCategory"].(string)
	contentType, _ := def["contentType"].(string)
	resourceRef, _ := def["resourceRef"].(string)

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	return ContributionRegistration{
		Ref:        ref,
		Kind:       KindResource,
		Revision:   rev,
		Status:     status,
		Definition: cloneMap(def),
		Resource: &ResourceDescriptor{
			ContributionRefJSON: ref.ToJSON(),
			DisplayName:         displayName,
			Description:         description,
			AssetKind:           assetKind,
			MimeCategory:        mimeCategory,
			ContentType:         contentType,
			ResourceRef:         resourceRef,
		},
	}, nil
}

func (a *resourceContributionAdapter) Detach(ctx context.Context, ref ContributionRef) error {
	return nil
}

type actionContributionAdapter struct {
	resolveTarget petActionResolver
}

type petActionResolver interface {
	ResolveActionTarget(ctx context.Context, ref ContributionRef) (PetActionTarget, error)
	InvalidateAction(ctx context.Context, ref ContributionRef) error
}

type PetActionTarget struct {
	InstallationID string
	DeviceID       string
	UserID         string
}

func (a *actionContributionAdapter) Kind() ContributionKind { return KindAction }

func (a *actionContributionAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
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

func (a *actionContributionAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	actionKey, _ := def["actionKey"].(string)
	displayName, _ := def["displayName"].(string)
	description, _ := def["description"].(string)
	categoryKey, _ := def["categoryKey"].(string)
	playbackMode, _ := def["playbackMode"].(string)
	requiredResource, _ := def["requiredResource"].(string)

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	if a.resolveTarget != nil {
		if _, err := a.resolveTarget.ResolveActionTarget(ctx, ref); err != nil {
			return ContributionRegistration{}, fmt.Errorf("%w: resolve action target: %v", ErrUnavailable, err)
		}
	}

	return ContributionRegistration{
		Ref:        ref,
		Kind:       KindAction,
		Revision:   rev,
		Status:     status,
		Definition: cloneMap(def),
		Action: &ActionDescriptor{
			ContributionRefJSON: ref.ToJSON(),
			ActionKey:           actionKey,
			DisplayName:         displayName,
			Description:         description,
			CategoryKey:         categoryKey,
			PlaybackMode:        playbackMode,
			RequiredResource:    requiredResource,
			Interruptible:       true,
		},
	}, nil
}

func (a *actionContributionAdapter) Detach(ctx context.Context, ref ContributionRef) error {
	if a.resolveTarget == nil {
		return nil
	}
	return a.resolveTarget.InvalidateAction(ctx, ref)
}

type runtimeCapabilityContributionAdapter struct {
}

func (a *runtimeCapabilityContributionAdapter) Kind() ContributionKind { return KindRuntime }

func (a *runtimeCapabilityContributionAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
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

func (a *runtimeCapabilityContributionAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	capabilityID, _ := def["capabilityId"].(string)
	capabilityKind, _ := def["capabilityKind"].(string)
	version, _ := def["version"].(string)

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	return ContributionRegistration{
		Ref:        ref,
		Kind:       KindRuntime,
		Revision:   rev,
		Status:     status,
		Definition: cloneMap(def),
		Runtime: &RuntimeCapabilityDescriptor{
			ContributionRefJSON: ref.ToJSON(),
			CapabilityID:        capabilityID,
			CapabilityKind:      capabilityKind,
			Version:             version,
		},
	}, nil
}

func (a *runtimeCapabilityContributionAdapter) Detach(ctx context.Context, ref ContributionRef) error {
	return nil
}

type floatingWindowCapabilityContributionAdapter struct {
}

func (a *floatingWindowCapabilityContributionAdapter) Kind() ContributionKind {
	return KindFloatingWindow
}

func (a *floatingWindowCapabilityContributionAdapter) Validate(ctx context.Context, ref ContributionRef, def map[string]any) error {
	return nil
}

func (a *floatingWindowCapabilityContributionAdapter) Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
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
		Ref:        ref,
		Kind:       KindFloatingWindow,
		Revision:   rev,
		Status:     status,
		Definition: cloneMap(def),
		Window:     &fd,
	}, nil
}

func (a *floatingWindowCapabilityContributionAdapter) Detach(ctx context.Context, ref ContributionRef) error {
	return nil
}

func boolDef(def map[string]any, key string, defaultVal bool) bool {
	if v, ok := def[key].(bool); ok {
		return v
	}
	return defaultVal
}

func isValidPlaybackMode(mode string) bool {
	switch mode {
	case "once", "loop", "hold", "ping_pong":
		return true
	}
	return false
}

func isValidRuntimeCapabilityKind(kind string) bool {
	switch kind {
	case "activate_pet", "apply_resource", "trigger_action", "refresh_state", "idle_interval":
		return true
	}
	return false
}

func validateResourceRef(ref string) error {
	if ref == "" {
		return nil
	}
	clean := path.Clean(ref)
	abs := path.IsAbs(clean)
	if abs {
		return fmt.Errorf("absolute resource path not allowed: %s", ref)
	}
	if strings.Contains(clean, "..") {
		return fmt.Errorf("traversal sequence not allowed: %s", ref)
	}
	if strings.HasPrefix(clean, "~") {
		return fmt.Errorf("home shortcut not allowed: %s", ref)
	}
	return nil
}

func defaultAdapters() []ContributionAdapter {
	return []ContributionAdapter{
		&noopAdapter{KindField: KindUnknown},
		&resourceContributionAdapter{},
		&actionContributionAdapter{},
		&runtimeCapabilityContributionAdapter{},
		&floatingWindowCapabilityContributionAdapter{},
	}
}

var _ KernelContributionSource = (*adapterBridge)(nil)

type adapterBridge struct {
	inner func(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error)
}

func (b *adapterBridge) ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	if b.inner != nil {
		return b.inner(ctx, extID)
	}
	return nil, nil
}

func (b *adapterBridge) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	return domain.ExtensionInstallation{}, fmt.Errorf("not implemented")
}

var _ = KernelContributionSource(nil)

type containerSource struct {
	contribs sqlite.ContributionRepository
	installs domain.InstallationRepository
}

func NewContainerSource(contribs sqlite.ContributionRepository, installs domain.InstallationRepository) KernelContributionSource {
	return &containerSource{contribs: contribs, installs: installs}
}

func (s *containerSource) ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	if s.contribs == nil {
		return nil, nil
	}
	return s.contribs.ListContributions(ctx, extID)
}

func (s *containerSource) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	if s.installs == nil {
		return domain.ExtensionInstallation{}, fmt.Errorf("not installed")
	}
	return s.installs.GetInstallation(ctx, id)
}

var _ KernelContributionSource = (*containerSource)(nil)
