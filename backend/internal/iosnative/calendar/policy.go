package calendar

import "time"

const (
	DefaultLimit = 100
	MaxLimit     = 500

	MaxQueryRangeDays = 366

	TitleMaxLengthRunes = 1024
	NotesMaxLength     = 16 * 1024

	MinRecurrenceInterval = 1
	MaxRecurrenceInterval = 999

	MinAlarmOffsetSeconds = -30 * 24 * 60 * 60
	MaxAlarmOffsetSeconds = 30 * 24 * 60 * 60

	QueryTimeout  = 10 * time.Second
	WriteTimeout  = 5 * time.Second
	StatusTimeout = 5 * time.Second
)

var validAccessLevels = map[string]bool{
	"write_only": true,
	"full_access": true,
}

var validDaysOfWeek = map[string]bool{
	"monday":    true,
	"tuesday":   true,
	"wednesday": true,
	"thursday":  true,
	"friday":    true,
	"saturday":  true,
	"sunday":    true,
}

var validFrequencies = map[string]bool{
	"daily":   true,
	"weekly":  true,
	"monthly": true,
	"yearly":  true,
}

var validAvailability = map[string]bool{
	"not_supported": true,
	"busy":          true,
	"free":          true,
	"tentative":     true,
	"unavailable":   true,
}

var validURLSchemes = map[string]bool{
	"http":  true,
	"https": true,
}

func IsValidAccessLevel(level string) bool {
	return validAccessLevels[level]
}

func IsValidDayOfWeek(day string) bool {
	return validDaysOfWeek[day]
}

func IsValidFrequency(freq string) bool {
	return validFrequencies[freq]
}

func IsValidAvailability(avail string) bool {
	return validAvailability[avail]
}

func IsValidURLScheme(scheme string) bool {
	return validURLSchemes[scheme]
}
