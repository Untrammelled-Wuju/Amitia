package contacts

import (
	"strings"
	"unicode/utf8"
)

const (
	DefaultLimitList    = 50
	MaxLimitList        = 200
	DefaultLimitSearch  = 20
	MaxLimitSearch      = 100
	DefaultLimitPhoto   = 1
	MaxLimitPhoto       = 10

	TitleMaxLengthRunes = 1024
	NotesMaxLength     = 16 * 1024

	OrganizationMaxLength = 1024
	DepartmentMaxLength   = 1024
	JobTitleMaxLength     = 1024

	MaxPhoneNumbers   = 50
	MaxEmailAddresses = 50
	MaxPostalAddresses = 20
	MaxURLs            = 50
	MaxDates           = 50
	MaxSocialProfiles  = 50
	MaxInstantMessages = 50

	MaxPhotoBytes = 10 * 1024 * 1024
	MaxPhotoPixels = 20_000_000
	MaxPhotoDimension = 8192

	PhoneLabelMaxLength   = 128
	EmailLabelMaxLength   = 128
	URLLabelMaxLength     = 128
	AddressLabelMaxLength = 128
	SocialLabelMaxLength  = 128
	DateLabelMaxLength    = 128

	TimeoutStatusAuth       = 2000000000
	TimeoutSearchGetList    = 5000000000
	TimeoutContainersGroups = 5000000000
	TimeoutCreateUpdateDelete = 5000000000
	TimeoutPhoto            = 10000000000
)

func ClampListLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimitList
	}
	if limit > MaxLimitList {
		return MaxLimitList
	}
	return limit
}

func ClampSearchLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimitSearch
	}
	if limit > MaxLimitSearch {
		return MaxLimitSearch
	}
	return limit
}

func TruncateStringRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes])
}

func IsValidContainerType(t string) bool {
	switch t {
	case "local", "carddav", "exchange", "ldap", "subscribed", "unnamed":
		return true
	default:
		return false
	}
}

func IsValidPhoneLabel(label string) bool {
	if label == "" {
		return true
	}
	return utf8.RuneCountInString(label) <= PhoneLabelMaxLength
}

func IsValidEmailLabel(label string) bool {
	if label == "" {
		return true
	}
	return utf8.RuneCountInString(label) <= EmailLabelMaxLength
}

func IsValidURLLabel(label string) bool {
	if label == "" {
		return true
	}
	return utf8.RuneCountInString(label) <= URLLabelMaxLength
}

func IsValidAddressLabel(label string) bool {
	if label == "" {
		return true
	}
	return utf8.RuneCountInString(label) <= AddressLabelMaxLength
}

func IsValidSocialLabel(label string) bool {
	if label == "" {
		return true
	}
	return utf8.RuneCountInString(label) <= SocialLabelMaxLength
}

func IsValidDateLabel(label string) bool {
	if label == "" {
		return true
	}
	return utf8.RuneCountInString(label) <= DateLabelMaxLength
}

func IsValidSocialService(service string) bool {
	switch strings.ToLower(service) {
	case "twitter", "facebook", "linkedin", "flickr", "myspace", "sinaweibo", "gamecenter", "custom":
		return true
	default:
		return false
	}
}

func IsValidInstantMessageService(service string) bool {
	switch strings.ToLower(service) {
	case "aim", "icq", "irc", "jabber", "msn", "qq", "skype", "yahoo", "googletalk", "gadugadu", "custom":
		return true
	default:
		return false
	}
}
