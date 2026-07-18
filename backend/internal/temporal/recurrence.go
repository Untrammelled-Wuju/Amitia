package temporal

import (
	"strconv"
	"strings"
	"time"
)

type recurrenceRule struct {
	Frequency  string
	Interval   int
	ByDay      map[time.Weekday]bool
	ByMonth    map[time.Month]bool
	ByMonthDay map[int]bool
	Count      int
	Until      *time.Time
}

func nextRecurringOccurrence(anchor Anchor, from time.Time) *time.Time {
	location, err := loadLocation(anchor.Timezone)
	if err != nil {
		return nil
	}
	rule, ok := parseRecurrenceRule(anchor.RRule, location)
	if !ok {
		return nil
	}
	start, ok := recurrenceStart(anchor, location)
	if !ok {
		return nil
	}
	localFrom := from.In(location)
	candidate := time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), start.Minute(), start.Second(), 0, location)
	matched := 0
	limit := candidate.AddDate(10, 0, 0)
	for !candidate.After(limit) {
		if recurrenceMatches(candidate, start, rule) {
			matched++
			if rule.Count > 0 && matched > rule.Count {
				return nil
			}
			if rule.Until != nil && candidate.After(rule.Until.In(location)) {
				return nil
			}
			if !candidate.Before(localFrom) {
				result := utc(candidate)
				return &result
			}
		}
		candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day()+1, start.Hour(), start.Minute(), start.Second(), 0, location)
	}
	return nil
}

func parseRecurrenceRule(value string, location *time.Location) (recurrenceRule, bool) {
	rule := recurrenceRule{Interval: 1, ByDay: map[time.Weekday]bool{}, ByMonth: map[time.Month]bool{}, ByMonthDay: map[int]bool{}}
	value = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(value), "RRULE:"))
	if value == "" {
		return rule, false
	}
	for _, part := range strings.Split(value, ";") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 {
			continue
		}
		switch pair[0] {
		case "FREQ":
			rule.Frequency = pair[1]
		case "INTERVAL":
			if parsed, err := strconv.Atoi(pair[1]); err == nil && parsed > 0 {
				rule.Interval = parsed
			}
		case "BYDAY":
			for _, day := range strings.Split(pair[1], ",") {
				if weekday, exists := recurrenceWeekdays[day]; exists {
					rule.ByDay[weekday] = true
				}
			}
		case "BYMONTH":
			for _, month := range strings.Split(pair[1], ",") {
				if parsed, err := strconv.Atoi(month); err == nil && parsed >= 1 && parsed <= 12 {
					rule.ByMonth[time.Month(parsed)] = true
				}
			}
		case "BYMONTHDAY":
			for _, day := range strings.Split(pair[1], ",") {
				if parsed, err := strconv.Atoi(day); err == nil && parsed >= 1 && parsed <= 31 {
					rule.ByMonthDay[parsed] = true
				}
			}
		case "COUNT":
			rule.Count, _ = strconv.Atoi(pair[1])
		case "UNTIL":
			if parsed, err := time.Parse("20060102T150405Z", pair[1]); err == nil {
				rule.Until = &parsed
			} else if parsed, err := time.ParseInLocation("20060102T150405", pair[1], location); err == nil {
				rule.Until = &parsed
			} else if parsed, err := time.ParseInLocation("20060102", pair[1], location); err == nil {
				parsed = parsed.Add(24*time.Hour - time.Nanosecond)
				rule.Until = &parsed
			}
		}
	}
	switch rule.Frequency {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
		return rule, true
	default:
		return rule, false
	}
}

func recurrenceStart(anchor Anchor, location *time.Location) (time.Time, bool) {
	date := anchor.LocalDate
	if len(date) == 5 {
		date = "2000-" + date
	}
	parsedDate, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		return time.Time{}, false
	}
	hour, minute := 0, 0
	if parsed, ok := clockMinutes(anchor.LocalTime); ok {
		hour, minute = parsed/60, parsed%60
	}
	return time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), hour, minute, 0, 0, location), true
}

func recurrenceMatches(candidate, start time.Time, rule recurrenceRule) bool {
	if candidate.Before(start) {
		return false
	}
	if len(rule.ByDay) > 0 && !rule.ByDay[candidate.Weekday()] {
		return false
	}
	if len(rule.ByMonth) > 0 && !rule.ByMonth[candidate.Month()] {
		return false
	}
	if len(rule.ByMonthDay) > 0 && !rule.ByMonthDay[candidate.Day()] {
		return false
	}
	days := civilDaysBetween(start, candidate)
	switch rule.Frequency {
	case "DAILY":
		return days%rule.Interval == 0
	case "WEEKLY":
		if len(rule.ByDay) == 0 && candidate.Weekday() != start.Weekday() {
			return false
		}
		return (days/7)%rule.Interval == 0
	case "MONTHLY":
		months := (candidate.Year()-start.Year())*12 + int(candidate.Month()-start.Month())
		if months%rule.Interval != 0 {
			return false
		}
		if len(rule.ByMonthDay) == 0 {
			return candidate.Day() == start.Day()
		}
		return true
	case "YEARLY":
		if (candidate.Year()-start.Year())%rule.Interval != 0 {
			return false
		}
		if len(rule.ByMonth) == 0 && candidate.Month() != start.Month() {
			return false
		}
		if len(rule.ByMonthDay) == 0 && candidate.Day() != start.Day() {
			return false
		}
		return true
	}
	return false
}

func civilDaysBetween(start, end time.Time) int {
	startUTC := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endUTC := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(endUTC.Sub(startUTC).Hours() / 24)
}

var recurrenceWeekdays = map[string]time.Weekday{
	"SU": time.Sunday,
	"MO": time.Monday,
	"TU": time.Tuesday,
	"WE": time.Wednesday,
	"TH": time.Thursday,
	"FR": time.Friday,
	"SA": time.Saturday,
}
