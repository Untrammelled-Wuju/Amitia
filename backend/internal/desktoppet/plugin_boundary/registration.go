package plugin_boundary

import (
	"fmt"
	"strings"
)

type ContributionStatus string

const (
	ContributionStatusUnknown    ContributionStatus = ""
	ContributionStatusRegistered ContributionStatus = "registered"
	ContributionStatusActive     ContributionStatus = "active"
	ContributionStatusDisabled   ContributionStatus = "disabled"
	ContributionStatusDetached   ContributionStatus = "detached"
	ContributionStatusInvalid    ContributionStatus = "invalid"
)

type ResourceDescriptor struct {
	ContributionRefJSON ContributionRefJSON `json:"contributionRef"`
	DisplayName         string              `json:"displayName"`
	Description         string              `json:"description,omitempty"`
	AssetKind           string              `json:"assetKind,omitempty"`
	MimeCategory        string              `json:"mimeCategory,omitempty"`
	SizeBytes           int64               `json:"sizeBytes,omitempty"`
	ContentType         string              `json:"contentType,omitempty"`
	ResourceRef         string              `json:"resourceRef,omitempty"`
}

func (d ResourceDescriptor) Validate() error {
	if d.DisplayName == "" {
		return fmt.Errorf("plugin_boundary: resource descriptor display name required")
	}
	if d.AssetKind != "" && !isValidAssetKind(d.AssetKind) {
		return fmt.Errorf("plugin_boundary: invalid asset kind: %s", d.AssetKind)
	}
	return nil
}

func isValidAssetKind(kind string) bool {
	switch kind {
	case "sprite", "animation", "frame_set", "config_template", "manifest", "preview":
		return true
	}
	return false
}

type ActionDescriptor struct {
	ContributionRefJSON ContributionRefJSON `json:"contributionRef"`
	ActionKey           string              `json:"actionKey"`
	DisplayName         string              `json:"displayName"`
	Description         string              `json:"description,omitempty"`
	CategoryKey         string              `json:"categoryKey,omitempty"`
	PlaybackMode        string              `json:"playbackMode,omitempty"`
	Interruptible       bool                `json:"interruptible"`
	CooldownMS          int                 `json:"cooldownMs,omitempty"`
	RequiredResource    string              `json:"requiredResource,omitempty"`
}

func (d ActionDescriptor) Validate() error {
	if d.ActionKey == "" {
		return fmt.Errorf("plugin_boundary: action descriptor actionKey required")
	}
	if d.DisplayName == "" {
		return fmt.Errorf("plugin_boundary: action descriptor display name required")
	}
	if !isValidActionKey(d.ActionKey) {
		return fmt.Errorf("plugin_boundary: invalid action key format: %s", d.ActionKey)
	}
	return nil
}

func isValidActionKey(key string) bool {
	if key == "" || len(key) > 128 {
		return false
	}
	for _, r := range key {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' && r != '.' {
			return false
		}
	}
	return true
}

type RuntimeCapabilityDescriptor struct {
	ContributionRefJSON ContributionRefJSON `json:"contributionRef"`
	CapabilityID        string              `json:"capabilityId"`
	CapabilityKind      string              `json:"capabilityKind"`
	Version             string              `json:"version,omitempty"`
}

func (d RuntimeCapabilityDescriptor) Validate() error {
	if d.CapabilityID == "" {
		return fmt.Errorf("plugin_boundary: runtime capability id required")
	}
	if d.CapabilityKind == "" {
		return fmt.Errorf("plugin_boundary: runtime capability kind required")
	}
	return nil
}

type FloatingWindowCapabilityDescriptor struct {
	ContributionRefJSON ContributionRefJSON `json:"contributionRef"`
	SupportsShow        bool                `json:"supportsShow"`
	SupportsHide        bool                `json:"supportsHide"`
	SupportsPosition    bool                `json:"supportsPosition"`
	SupportsSize        bool                `json:"supportsSize"`
	SupportsOpacity     bool                `json:"supportsOpacity"`
	SupportsContent     bool                `json:"supportsContent"`
}

type ContributionRegistration struct {
	Ref             ContributionRef
	Kind            ContributionKind
	Revision        int
	Status          ContributionStatus
	Definition      map[string]any
	RegisteredAt    string
	UpdatedAt       string
	Resource        *ResourceDescriptor
	Action          *ActionDescriptor
	Runtime         *RuntimeCapabilityDescriptor
	Window          *FloatingWindowCapabilityDescriptor
	ErrorMessage    string
	CanonicalHandle string
}

func (r ContributionRegistration) IsExecutable() bool {
	return r.Status == ContributionStatusActive
}

func (r ContributionRegistration) IsStatic() bool {
	if r.Status == ContributionStatusDetached || r.Status == ContributionStatusDisabled || r.Status == ContributionStatusInvalid {
		return true
	}
	return false
}

var hostReservedFields = []string{"extensionId", "pluginId", "contributionId", "installState", "permission", "ownership"}

func isHostReservedField(key string) bool {
	for _, rf := range hostReservedFields {
		if strings.EqualFold(rf, key) {
			return true
		}
	}
	return false
}

type ContributionRegistryView struct {
	ByKey    map[string]ContributionRegistration
	ByRef    map[string]ContributionRegistration
	ByExt    map[string][]ContributionRegistration
	ByPlugin map[string][]ContributionRegistration
}

func NewContributionRegistryView(regs []ContributionRegistration) ContributionRegistryView {
	v := ContributionRegistryView{
		ByKey:    make(map[string]ContributionRegistration),
		ByRef:    make(map[string]ContributionRegistration),
		ByExt:    make(map[string][]ContributionRegistration),
		ByPlugin: make(map[string][]ContributionRegistration),
	}
	for _, r := range regs {
		v.ByKey[r.Ref.Key()] = r
		v.ByRef[r.Ref.Key()] = r
		extKey := string(r.Ref.ExtensionID)
		v.ByExt[extKey] = append(v.ByExt[extKey], r)
		plugKey := string(r.Ref.ExtensionID) + "/" + string(r.Ref.PluginID)
		v.ByPlugin[plugKey] = append(v.ByPlugin[plugKey], r)
	}
	return v
}

func (v ContributionRegistryView) FindByRef(ref ContributionRef) (ContributionRegistration, bool) {
	r, ok := v.ByRef[ref.Key()]
	return r, ok
}

func (v ContributionRegistryView) FindByExt(extID string) []ContributionRegistration {
	return v.ByExt[extID]
}

func (v ContributionRegistryView) FindByPlugin(extID, pluginID string) []ContributionRegistration {
	return v.ByPlugin[extID+"/"+pluginID]
}
