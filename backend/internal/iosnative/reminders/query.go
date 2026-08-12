package reminders

import "time"

const (
	QueryStatusAll        = "all"
	QueryStatusIncomplete = "incomplete"
	QueryStatusCompleted  = "completed"
)

type ReminderPredicate struct {
	Status string `json:"status"`

	DueStart *time.Time `json:"dueStart,omitempty"`
	DueEnd   *time.Time `json:"dueEnd,omitempty"`

	CompletionStart *time.Time `json:"completionStart,omitempty"`
	CompletionEnd   *time.Time `json:"completionEnd,omitempty"`
}

type SortOrder struct {
	Field string `json:"field"`
	Asc   bool   `json:"asc"`
}

var defaultIncompleteSort = []SortOrder{
	{Field: "dueDate", Asc: true},
	{Field: "priority", Asc: false},
	{Field: "title", Asc: true},
	{Field: "reminderId", Asc: true},
}

var defaultCompletedSort = []SortOrder{
	{Field: "completionDate", Asc: false},
	{Field: "title", Asc: true},
}

func GetDefaultSort(status string) []SortOrder {
	switch status {
	case QueryStatusCompleted:
		return defaultCompletedSort
	default:
		return defaultIncompleteSort
	}
}

func IsValidSortField(field string) bool {
	switch field {
	case "dueDate", "completionDate", "priority", "title", "reminderId", "startDate":
		return true
	default:
		return false
	}
}
