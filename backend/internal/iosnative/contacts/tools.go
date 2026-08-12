package contacts

const (
	ToolIDStatus                = "ios.contacts.status"
	ToolIDAuthorizationStatus   = "ios.contacts.authorization.status"
	ToolIDAuthorizationRequest  = "ios.contacts.authorization.request"
	ToolIDSearch                = "ios.contacts.search"
	ToolIDList                  = "ios.contacts.list"
	ToolIDGet                   = "ios.contacts.get"
	ToolIDCreate                = "ios.contacts.create"
	ToolIDUpdate                = "ios.contacts.update"
	ToolIDDelete                = "ios.contacts.delete"
	ToolIDContainersList        = "ios.contacts.containers.list"
	ToolIDGroupsList            = "ios.contacts.groups.list"
	ToolIDPhotoGet              = "ios.contacts.photo.get"
	ToolIDPhotoSet              = "ios.contacts.photo.set"
	ToolIDPhotoRemove           = "ios.contacts.photo.remove"
)

const (
	ModelNameContacts = "ios_native_contacts"
)

var SupportedSearchFields = map[string]bool{
	"name":       true,
	"phone":      true,
	"email":      true,
	"organization": true,
	"any":        true,
}

var SupportedContainerTypes = map[string]bool{
	"local":        true,
	"carddav":      true,
	"exchange":     true,
	"ldap":         true,
	"subscribed":   true,
	"unnamed":      true,
}

var SupportedGroupTypes = map[string]bool{
	"smart":        true,
	"subscription": true,
	"group":        true,
	"folder":       true,
}

var ModelNames = []string{ModelNameContacts}
