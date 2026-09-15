package resource

type OwnerType string

const (
	OwnerSystem    OwnerType = "system"
	OwnerSpace     OwnerType = "space"
	OwnerExtension OwnerType = "extension"
	OwnerModule    OwnerType = "module"
	OwnerShared    OwnerType = "shared"
	OwnerTemporary OwnerType = "temporary"
	OwnerMigration OwnerType = "migration"
)

func (ot OwnerType) IsValid() bool {
	switch ot {
	case OwnerSystem, OwnerSpace, OwnerExtension, OwnerModule, OwnerShared, OwnerTemporary, OwnerMigration:
		return true
	}
	return false
}

type ResourceOwner struct {
	OwnerType   OwnerType `json:"owner_type"`
	OwnerID     string    `json:"owner_id"`
	ExtensionID string    `json:"extension_id,omitempty"`
	ModuleID    string    `json:"module_id,omitempty"`
}

func NewSystemOwner() ResourceOwner {
	return ResourceOwner{OwnerType: OwnerSystem, OwnerID: "system"}
}

func NewSpaceOwner(spaceID string) ResourceOwner {
	return ResourceOwner{OwnerType: OwnerSpace, OwnerID: spaceID}
}

func NewExtensionOwner(extensionID string) ResourceOwner {
	return ResourceOwner{OwnerType: OwnerExtension, OwnerID: extensionID, ExtensionID: extensionID}
}

func NewModuleOwner(extensionID, moduleID string) ResourceOwner {
	return ResourceOwner{OwnerType: OwnerModule, OwnerID: moduleID, ExtensionID: extensionID, ModuleID: moduleID}
}

func NewSharedOwner(ownerID string) ResourceOwner {
	return ResourceOwner{OwnerType: OwnerShared, OwnerID: ownerID}
}

func NewTemporaryOwner(ownerID string) ResourceOwner {
	return ResourceOwner{OwnerType: OwnerTemporary, OwnerID: ownerID}
}

func NewMigrationOwner(ownerID string) ResourceOwner {
	return ResourceOwner{OwnerType: OwnerMigration, OwnerID: ownerID}
}

func (ro ResourceOwner) IsSystem() bool    { return ro.OwnerType == OwnerSystem }
func (ro ResourceOwner) IsSpace() bool     { return ro.OwnerType == OwnerSpace }
func (ro ResourceOwner) IsExtension() bool { return ro.OwnerType == OwnerExtension }
func (ro ResourceOwner) IsModule() bool    { return ro.OwnerType == OwnerModule }
func (ro ResourceOwner) IsShared() bool    { return ro.OwnerType == OwnerShared }
func (ro ResourceOwner) IsTemporary() bool { return ro.OwnerType == OwnerTemporary }
func (ro ResourceOwner) IsMigration() bool { return ro.OwnerType == OwnerMigration }

func (ro ResourceOwner) Equals(other ResourceOwner) bool {
	return ro.OwnerType == other.OwnerType && ro.OwnerID == other.OwnerID
}
