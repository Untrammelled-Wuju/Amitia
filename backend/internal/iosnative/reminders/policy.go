package reminders

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

	DefaultQueryTimeout = 5 * time.Second
	MaxQueryTimeout     = 15 * time.Second
	WriteTimeout        = 5 * time.Second
	StatusTimeout       = 5 * time.Second

	MaxCompletionDateSkewDays = 1
)

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

var validPriorities = map[string]bool{
	string(PriorityNone):   true,
	string(PriorityLow):    true,
	string(PriorityMedium): true,
	string(PriorityHigh):   true,
}

var validQueryStatuses = map[string]bool{
	"all":        true,
	"incomplete": true,
	"completed":  true,
}

var validURLSchemes = map[string]bool{
	"http":  true,
	"https": true,
}

func IsValidDayOfWeek(day string) bool {
	return validDaysOfWeek[day]
}

func IsValidFrequency(freq string) bool {
	return validFrequencies[freq]
}

func IsValidPriority(priority string) bool {
	return validPriorities[priority]
}

func IsValidQueryStatus(status string) bool {
	return validQueryStatuses[status]
}

func IsValidURLScheme(scheme string) bool {
	return validURLSchemes[scheme]
}
