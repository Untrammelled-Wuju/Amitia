package capability

type OwnerType string

const (
	OwnerTypeSystem    OwnerType = "system"
	OwnerTypeUser      OwnerType = "user"
	OwnerTypeExtension OwnerType = "extension"
	OwnerTypeShared    OwnerType = "shared"
	OwnerTypeTemporary OwnerType = "temporary"
)

type ResourceOwner struct {
	OwnerType   OwnerType `json:"ownerType"`
	OwnerID     string    `json:"ownerId"`
	ExtensionID string    `json:"extensionId,omitempty"`
	ModuleID    string    `json:"moduleId,omitempty"`
}
