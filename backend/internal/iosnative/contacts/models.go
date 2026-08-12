package contacts

type AuthorizationLevel string

const (
	AuthorizationNotDetermined     AuthorizationLevel = "not_determined"
	AuthorizationRestricted        AuthorizationLevel = "restricted"
	AuthorizationDenied            AuthorizationLevel = "denied"
	AuthorizationAuthorized        AuthorizationLevel = "authorized"
	AuthorizationLimited           AuthorizationLevel = "limited"
)

type CapabilityState struct {
	Supported bool `json:"supported"`

	Authorization string `json:"authorization"`

	Limited bool `json:"limited"`

	CanRead   bool `json:"canRead"`
	CanCreate bool `json:"canCreate"`
	CanUpdate bool `json:"canUpdate"`
	CanDelete bool `json:"canDelete"`

	CanManageLimitedAccess bool `json:"canManageLimitedAccess"`

	CanReadNotes bool `json:"canReadNotes"`

	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type ContactName struct {
	Prefix string `json:"prefix,omitempty"`

	Given  string `json:"given,omitempty"`
	Middle string `json:"middle,omitempty"`
	Family string `json:"family,omitempty"`

	Suffix   string `json:"suffix,omitempty"`
	Nickname string `json:"nickname,omitempty"`

	PhoneticGiven  string `json:"phoneticGiven,omitempty"`
	PhoneticMiddle string `json:"phoneticMiddle,omitempty"`
	PhoneticFamily string `json:"phoneticFamily,omitempty"`

	Display string `json:"display,omitempty"`
}

type LabeledString struct {
	Label string `json:"label,omitempty"`
	Value string `json:"value"`
}

type ContactPostalAddress struct {
	Label string `json:"label,omitempty"`

	Street string `json:"street,omitempty"`
	City   string `json:"city,omitempty"`
	State  string `json:"state,omitempty"`

	PostalCode      string `json:"postalCode,omitempty"`
	Country         string `json:"country,omitempty"`
	ISOCountryCode  string `json:"isoCountryCode,omitempty"`

	SubLocality          string `json:"subLocality,omitempty"`
	SubAdministrativeArea string `json:"subAdministrativeArea,omitempty"`
}

type ContactDate struct {
	Year  *int   `json:"year,omitempty"`
	Month int    `json:"month"`
	Day   int    `json:"day"`
	Label string `json:"label,omitempty"`
}

type ContactSocialProfile struct {
	Label   string `json:"label,omitempty"`
	URL     string `json:"url,omitempty"`
	Service string `json:"service,omitempty"`
	Username string `json:"username,omitempty"`
	UserIdentifier string `json:"userIdentifier,omitempty"`
}

type ContactInstantMessage struct {
	Label   string `json:"label,omitempty"`
	Service string `json:"service,omitempty"`
	Username string `json:"username,omitempty"`
}

type ContactInfo struct {
	ContactID string `json:"contactId"`

	Name ContactName `json:"name"`

	Organization string `json:"organization,omitempty"`
	Department   string `json:"department,omitempty"`
	JobTitle     string `json:"jobTitle,omitempty"`

	PhoneNumbers      []LabeledString        `json:"phoneNumbers,omitempty"`
	EmailAddresses     []LabeledString        `json:"emailAddresses,omitempty"`
	PostalAddresses    []ContactPostalAddress `json:"postalAddresses,omitempty"`
	URLs               []LabeledString        `json:"urls,omitempty"`

	Birthday *ContactDate `json:"birthday,omitempty"`

	Dates          []ContactDate          `json:"dates,omitempty"`
	SocialProfiles []ContactSocialProfile `json:"socialProfiles,omitempty"`
	InstantMessages []ContactInstantMessage `json:"instantMessages,omitempty"`

	HasImage bool `json:"hasImage"`

	Editable bool `json:"editable"`

	ContainerID string `json:"containerId,omitempty"`
}

type ContactContainerInfo struct {
	ContainerID string `json:"containerId"`

	Name string `json:"name"`

	Type string `json:"type,omitempty"`

	IsDefault bool `json:"isDefault"`
}

type ContactGroupInfo struct {
	GroupID string `json:"groupId"`

	Name string `json:"name"`

	ContainerID string `json:"containerId,omitempty"`
}

type PhotoResourceInfo struct {
	ResourceURI string `json:"resourceUri"`
	MimeType    string `json:"mimeType"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	ContentHash string  `json:"contentHash,omitempty"`
}

type ContactNameInput struct {
	Prefix string `json:"prefix,omitempty"`

	Given  string `json:"given,omitempty"`
	Middle string `json:"middle,omitempty"`
	Family string `json:"family,omitempty"`

	Suffix   string `json:"suffix,omitempty"`
	Nickname string `json:"nickname,omitempty"`

	PhoneticGiven  string `json:"phoneticGiven,omitempty"`
	PhoneticMiddle string `json:"phoneticMiddle,omitempty"`
	PhoneticFamily string `json:"phoneticFamily,omitempty"`
}

type PatchField[T any] struct {
	Set   bool `json:"set"`
	Value *T   `json:"value,omitempty"`
}

type SearchContactsRequest struct {
	Query string `json:"query"`

	Field string `json:"field,omitempty"`

	Limit int `json:"limit,omitempty"`

	IncludePhones bool `json:"includePhones,omitempty"`
	IncludeEmails bool `json:"includeEmails,omitempty"`
}

type ListContactsRequest struct {
	Limit int `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`

	IncludeOrganization bool `json:"includeOrganization,omitempty"`
	IncludePhones       bool `json:"includePhones,omitempty"`
	IncludeEmails       bool `json:"includeEmails,omitempty"`
}

type GetContactRequest struct {
	ContactID string `json:"contactId"`

	IncludePhones      bool `json:"includePhones,omitempty"`
	IncludeEmails      bool `json:"includeEmails,omitempty"`
	IncludeAddresses   bool `json:"includeAddresses,omitempty"`
	IncludeDates       bool `json:"includeDates,omitempty"`
	IncludeSocial      bool `json:"includeSocial,omitempty"`
	IncludePhoto       bool `json:"includePhoto,omitempty"`
}

type CreateContactRequest struct {
	ContainerID string `json:"containerId,omitempty"`

	Name ContactNameInput `json:"name"`

	Organization string `json:"organization,omitempty"`
	Department   string `json:"department,omitempty"`
	JobTitle     string `json:"jobTitle,omitempty"`

	PhoneNumbers      []LabeledString        `json:"phoneNumbers,omitempty"`
	EmailAddresses     []LabeledString        `json:"emailAddresses,omitempty"`
	PostalAddresses    []ContactPostalAddress `json:"postalAddresses,omitempty"`
	URLs               []LabeledString        `json:"urls,omitempty"`
	Birthday           *ContactDate           `json:"birthday,omitempty"`
	Dates              []ContactDate          `json:"dates,omitempty"`
	SocialProfiles     []ContactSocialProfile `json:"socialProfiles,omitempty"`
	InstantMessages    []ContactInstantMessage `json:"instantMessages,omitempty"`
	PhotoResourceURI   string                 `json:"photoResourceUri,omitempty"`
}

type UpdateContactRequest struct {
	ContactID string `json:"contactId"`

	ContainerID *string `json:"containerId,omitempty"`

	Name *ContactNameInput `json:"name,omitempty"`

	Organization *string `json:"organization,omitempty"`
	Department   *string `json:"department,omitempty"`
	JobTitle     *string `json:"jobTitle,omitempty"`

	PhoneNumbers      *[]LabeledString        `json:"phoneNumbers,omitempty"`
	EmailAddresses     *[]LabeledString        `json:"emailAddresses,omitempty"`
	PostalAddresses    *[]ContactPostalAddress `json:"postalAddresses,omitempty"`
	URLs               *[]LabeledString        `json:"urls,omitempty"`
	Birthday           *ContactDate            `json:"birthday,omitempty"`
	Dates              *[]ContactDate          `json:"dates,omitempty"`
	SocialProfiles     *[]ContactSocialProfile `json:"socialProfiles,omitempty"`
	InstantMessages    *[]ContactInstantMessage `json:"instantMessages,omitempty"`
	PhotoResourceURI   *string                 `json:"photoResourceUri,omitempty"`
}

type DeleteContactRequest struct {
	ContactID string `json:"contactId"`
}

type GetPhotoRequest struct {
	ContactID string `json:"contactId"`
}

type SetPhotoRequest struct {
	ContactID    string `json:"contactId"`
	ResourceURI  string `json:"resourceUri"`
}

type RemovePhotoRequest struct {
	ContactID string `json:"contactId"`
}

type SearchContactsResult struct {
	Contacts  []ContactInfo `json:"contacts"`
	Count     int           `json:"count"`
	Truncated bool          `json:"truncated"`
}

type ListContactsResult struct {
	Contacts  []ContactInfo `json:"contacts"`
	Count     int           `json:"count"`
	Truncated bool          `json:"truncated"`
	Cursor    string        `json:"cursor,omitempty"`
}

type CreateContactResult struct {
	ContactID string `json:"contactId"`
	ContactInfo
}

type UpdateContactResult struct {
	ContactID string `json:"contactId"`
	ContactInfo
}

type DeleteContactResult struct {
	ContactID string `json:"contactId"`
	Deleted   bool   `json:"deleted"`
}

type ContainersListResult struct {
	Containers []ContactContainerInfo `json:"containers"`
	Count      int                    `json:"count"`
}

type GroupsListResult struct {
	Groups []ContactGroupInfo `json:"groups"`
	Count  int                `json:"count"`
}

type GetPhotoResult struct {
	HasImage     bool             `json:"hasImage"`
	Photo        *PhotoResourceInfo `json:"photo,omitempty"`
}

type SetPhotoResult struct {
	ContactID string `json:"contactId"`
	HasImage  bool   `json:"hasImage"`
}

type RemovePhotoResult struct {
	ContactID string `json:"contactId"`
	HasImage  bool   `json:"hasImage"`
}

type AuthorizationStatusResult struct {
	Level                 string `json:"level"`
	EffectiveLevel        string `json:"effectiveLevel"`
	Limited               bool   `json:"limited"`
	CanRead               bool   `json:"canRead"`
	CanCreate             bool   `json:"canCreate"`
	CanUpdate             bool   `json:"canUpdate"`
	CanDelete             bool   `json:"canDelete"`
	CanManageLimitedAccess bool  `json:"canManageLimitedAccess"`
	CanReadNotes          bool   `json:"canReadNotes"`
}

type AuthorizationRequestResult struct {
	Level          string `json:"level"`
	EffectiveLevel string `json:"effectiveLevel"`
	Limited        bool   `json:"limited"`
	Granted        bool   `json:"granted"`
}

func (n ContactNameInput) IsEmpty() bool {
	return n.Prefix == "" && n.Given == "" && n.Middle == "" &&
		n.Family == "" && n.Suffix == "" && n.Nickname == ""
}

func (r CreateContactRequest) HasMinimalIdentity() bool {
	return !r.Name.IsEmpty() || r.Organization != "" ||
		len(r.PhoneNumbers) > 0 || len(r.EmailAddresses) > 0
}

func PatchFieldClear[T any]() PatchField[T] {
	return PatchField[T]{Set: true, Value: nil}
}

func PatchFieldSet[T any](value T) PatchField[T] {
	return PatchField[T]{Set: true, Value: &value}
}

func PatchFieldUnmodified[T any]() PatchField[T] {
	return PatchField[T]{Set: false}
}

