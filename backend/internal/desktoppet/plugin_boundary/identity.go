package plugin_boundary

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ContributionRef struct {
	ExtensionID    domain.ExtensionID
	PluginID       domain.ContributionID
	ContributionID string
}

func (r ContributionRef) Key() string {
	return fmt.Sprintf("%s/%s/%s", r.ExtensionID, r.PluginID, r.ContributionID)
}

func (r ContributionRef) Validate() error {
	if r.ExtensionID == "" {
		return fmt.Errorf("plugin_boundary: extensionId required")
	}
	if r.PluginID == "" {
		return fmt.Errorf("plugin_boundary: pluginId required")
	}
	if r.ContributionID == "" {
		return fmt.Errorf("plugin_boundary: contributionId required")
	}
	return nil
}

func ContributionRefFromDefinition(contrib domain.ContributionDefinition) ContributionRef {
	id := string(contrib.ID)
	return ContributionRef{
		ExtensionID:    contrib.ExtensionID,
		PluginID:       contrib.ID,
		ContributionID: id,
	}
}

type ContributionRefJSON struct {
	ExtensionID    string `json:"extensionId"`
	PluginID       string `json:"pluginId"`
	ContributionID string `json:"contributionId"`
}

func (r ContributionRef) ToJSON() ContributionRefJSON {
	return ContributionRefJSON{
		ExtensionID:    string(r.ExtensionID),
		PluginID:       string(r.PluginID),
		ContributionID: r.ContributionID,
	}
}

func ContributionRefFromJSON(j ContributionRefJSON) (ContributionRef, error) {
	r := ContributionRef{
		ExtensionID:    domain.ExtensionID(j.ExtensionID),
		PluginID:       domain.ContributionID(j.PluginID),
		ContributionID: j.ContributionID,
	}
	return r, r.Validate()
}

func ParseContributionRef(s string) (ContributionRef, error) {
	parts := strings.SplitN(s, "/", 3)
	if len(parts) != 3 {
		return ContributionRef{}, fmt.Errorf("plugin_boundary: invalid contribution ref: %s", s)
	}
	return ContributionRefFromJSON(ContributionRefJSON{
		ExtensionID:    parts[0],
		PluginID:       parts[1],
		ContributionID: parts[2],
	})
}

type ContributionOwnership struct {
	Ref        ContributionRef
	OwnerExtID domain.ExtensionID
}

func (o ContributionOwnership) BelongsTo(extID domain.ExtensionID) bool {
	return o.OwnerExtID == extID
}

type ContributionKind string

const (
	KindUnknown        ContributionKind = ""
	KindResource       ContributionKind = "pet_resource"
	KindAction         ContributionKind = "pet_action"
	KindRuntime        ContributionKind = "pet_runtime_capability"
	KindFloatingWindow ContributionKind = "pet_floating_window_capability"
)

func ParseContributionKind(s string) ContributionKind {
	switch s {
	case string(KindResource):
		return KindResource
	case string(KindAction):
		return KindAction
	case string(KindRuntime):
		return KindRuntime
	case string(KindFloatingWindow):
		return KindFloatingWindow
	default:
		return KindUnknown
	}
}

func (k ContributionKind) String() string {
	return string(k)
}
