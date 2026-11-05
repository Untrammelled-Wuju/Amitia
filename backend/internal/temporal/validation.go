package temporal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

var validAnchorTypes = map[string]bool{
	"birthday": true, "anniversary": true, "relationship_anniversary": true,
	"first_meeting": true, "shared_memory": true, "holiday": true,
	"deadline": true, "appointment": true, "exam": true, "travel": true,
	"work_event": true, "class_event": true, "custom": true,
}

func validateTemporalProfile(profile *Profile) error {
	switch profile.TimezoneMode {
	case TimezoneFollowDevice, TimezoneFixed, TimezoneFollowUser, TimezoneNarrative:
	default:
		return fmt.Errorf("%w: 时区模式 %q 无效", ErrInvalidConfig, profile.TimezoneMode)
	}
	if profile.WeekStart < 0 || profile.WeekStart > 6 {
		return fmt.Errorf("%w: weekStart 必须在 0 到 6 之间", ErrInvalidConfig)
	}
	switch profile.Hemisphere {
	case "north", "south", "unknown":
	default:
		return fmt.Errorf("%w: hemisphere %q 无效", ErrInvalidConfig, profile.Hemisphere)
	}
	if profile.CalendarSystem != "gregorian" {
		return fmt.Errorf("%w: calendarSystem %q 无效", ErrInvalidConfig, profile.CalendarSystem)
	}
	if profile.Locale == "" || len(profile.Locale) > 64 || strings.ContainsAny(profile.Locale, "\x00\r\n") {
		return fmt.Errorf("%w: locale 无效", ErrInvalidConfig)
	}
	if profile.Confidence < 0 || profile.Confidence > 100 || profile.AwarenessLevel < 0 || profile.AwarenessLevel > 100 {
		return fmt.Errorf("%w: confidence 和 awarenessLevel 必须在 0 到 100 之间", ErrInvalidConfig)
	}
	if profile.PendingTimezone != "" {
		if _, err := loadLocation(profile.PendingTimezone); err != nil {
			return err
		}
	}
	if profile.DaypartConfigJSON == "" {
		profile.DaypartConfigJSON = "{}"
	}
	if err := validateDaypartConfig(profile.DaypartConfigJSON); err != nil {
		return err
	}
	if profile.QuietHoursJSON == "" {
		profile.QuietHoursJSON = "{}"
	}
	if err := validateQuietHours(profile.QuietHoursJSON); err != nil {
		return err
	}
	return nil
}

func validateDaypartConfig(raw string) error {
	var values map[string]struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := decodeStrictJSON(raw, &values); err != nil {
		return fmt.Errorf("%w: daypartConfigJson: %v", ErrInvalidConfig, err)
	}
	allowed := map[string]bool{"dawn": true, "morning": true, "noon": true, "afternoon": true, "evening": true, "late_night": true, "deep_night": true}
	for name, value := range values {
		if !allowed[name] {
			return fmt.Errorf("%w: daypartConfigJson 包含未知时段 %q", ErrInvalidConfig, name)
		}
		start, startOK := clockMinutes(value.Start)
		end, endOK := clockMinutes(value.End)
		if !startOK || !endOK || start == end {
			return fmt.Errorf("%w: 时段 %s 的起止时间无效", ErrInvalidConfig, name)
		}
	}
	return nil
}

func validateQuietHours(raw string) error {
	var value struct {
		Enabled bool   `json:"enabled"`
		Start   string `json:"start"`
		End     string `json:"end"`
	}
	if err := decodeStrictJSON(raw, &value); err != nil {
		return fmt.Errorf("%w: quietHoursJson: %v", ErrInvalidConfig, err)
	}
	if !value.Enabled && value.Start == "" && value.End == "" {
		return nil
	}
	if _, startOK := clockMinutes(value.Start); !startOK {
		return fmt.Errorf("%w: quietHoursJson.start 无效", ErrInvalidConfig)
	}
	if _, endOK := clockMinutes(value.End); !endOK {
		return fmt.Errorf("%w: quietHoursJson.end 无效", ErrInvalidConfig)
	}
	return nil
}

func validateTemporalAnchor(anchor *Anchor) error {
	if strings.TrimSpace(anchor.Title) == "" || len(anchor.Title) > 512 || strings.ContainsAny(anchor.Title, "\x00\r\n") {
		return fmt.Errorf("%w: 锚点标题无效", ErrInvalidConfig)
	}
	if !validAnchorTypes[anchor.AnchorType] {
		return fmt.Errorf("%w: anchorType %q 无效", ErrInvalidConfig, anchor.AnchorType)
	}
	switch anchor.ScopeType {
	case OwnerSpace, OwnerCharacter, "relationship":
	default:
		return fmt.Errorf("%w: scopeType %q 无效", ErrInvalidConfig, anchor.ScopeType)
	}
	if !validAnchorStatus(anchor.Status) {
		return fmt.Errorf("%w: status %q 无效", ErrInvalidConfig, anchor.Status)
	}
	switch anchor.SensitivityLevel {
	case "", "internal", "private", "sensitive", "public":
	default:
		return fmt.Errorf("%w: sensitivityLevel %q 无效", ErrInvalidConfig, anchor.SensitivityLevel)
	}
	if anchor.Importance < 0 || anchor.Importance > 100 || anchor.Confidence < 0 || anchor.Confidence > 100 {
		return fmt.Errorf("%w: importance 和 confidence 必须在 0 到 100 之间", ErrInvalidConfig)
	}
	if anchor.DurationSeconds < 0 || anchor.PreWindowSeconds < 0 || anchor.PostWindowSeconds < 0 {
		return fmt.Errorf("%w: 时间窗口不能为负数", ErrInvalidConfig)
	}
	if anchor.PayloadJSON != "" && !json.Valid([]byte(anchor.PayloadJSON)) {
		return fmt.Errorf("%w: payloadJson 不是有效 JSON", ErrInvalidConfig)
	}
	if anchor.RRule != "" && anchor.TimeKind != "recurring" {
		return fmt.Errorf("%w: 只有 recurring 锚点可以设置 RRULE", ErrInvalidConfig)
	}
	switch anchor.TimeKind {
	case "instant":
		if anchor.InstantAtUTC == nil {
			return fmt.Errorf("%w: instant 锚点缺少 instantAtUtc", ErrInvalidConfig)
		}
		value := anchor.InstantAtUTC.UTC()
		anchor.InstantAtUTC = &value
		if anchor.EndAtUTC != nil {
			end := anchor.EndAtUTC.UTC()
			anchor.EndAtUTC = &end
			if !end.After(value) {
				return fmt.Errorf("%w: endAtUtc 必须晚于 instantAtUtc", ErrInvalidConfig)
			}
		}
	case "range":
		if anchor.InstantAtUTC == nil || anchor.EndAtUTC == nil {
			return fmt.Errorf("%w: range 锚点必须同时包含开始和结束时间", ErrInvalidConfig)
		}
		start, end := anchor.InstantAtUTC.UTC(), anchor.EndAtUTC.UTC()
		if !end.After(start) {
			return fmt.Errorf("%w: endAtUtc 必须晚于 instantAtUtc", ErrInvalidConfig)
		}
		anchor.InstantAtUTC, anchor.EndAtUTC = &start, &end
	case "local_date":
		if err := validateFullLocalDate(anchor.LocalDate); err != nil {
			return err
		}
		if anchor.LocalTime != "" {
			if _, ok := clockMinutes(anchor.LocalTime); !ok {
				return fmt.Errorf("%w: localTime 无效", ErrInvalidConfig)
			}
		}
	case "annual_date":
		if err := validateAnnualDate(anchor.LocalDate); err != nil {
			return err
		}
		if anchor.LocalTime != "" {
			if _, ok := clockMinutes(anchor.LocalTime); !ok {
				return fmt.Errorf("%w: localTime 无效", ErrInvalidConfig)
			}
		}
	case "local_datetime":
		if err := validateFullLocalDate(anchor.LocalDate); err != nil {
			return err
		}
		if _, ok := clockMinutes(anchor.LocalTime); !ok {
			return fmt.Errorf("%w: local_datetime 锚点缺少有效 localTime", ErrInvalidConfig)
		}
	case "recurring":
		if err := validateFullLocalDate(anchor.LocalDate); err != nil {
			return err
		}
		if _, ok := clockMinutes(anchor.LocalTime); !ok {
			return fmt.Errorf("%w: recurring 锚点缺少有效 localTime", ErrInvalidConfig)
		}
		location, _ := loadLocation(anchor.Timezone)
		if err := validateRecurrenceRule(anchor.RRule, location); err != nil {
			return err
		}
	case "derived":
	default:
		return fmt.Errorf("%w: timeKind %q 无效", ErrInvalidConfig, anchor.TimeKind)
	}
	return nil
}

func validateFullLocalDate(value string) error {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return fmt.Errorf("%w: localDate %q 无效", ErrInvalidConfig, value)
	}
	return nil
}

func validateAnnualDate(value string) error {
	layout := "2006-01-02"
	if len(value) == 5 {
		layout = "01-02"
	}
	if _, err := time.Parse(layout, value); err != nil {
		return fmt.Errorf("%w: annual_date %q 无效", ErrInvalidConfig, value)
	}
	return nil
}

func validAnchorStatus(value string) bool {
	switch value {
	case "candidate", "active", "disabled", "archived":
		return true
	default:
		return false
	}
}

func validateRecurrenceRule(value string, location *time.Location) error {
	value = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(value), "RRULE:"))
	if value == "" {
		return fmt.Errorf("%w: recurring 锚点缺少 RRULE", ErrInvalidConfig)
	}
	seen := map[string]bool{}
	frequency := ""
	for _, part := range strings.Split(value, ";") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 || strings.TrimSpace(pair[0]) == "" || strings.TrimSpace(pair[1]) == "" || seen[pair[0]] {
			return fmt.Errorf("%w: RRULE 片段 %q 无效或重复", ErrInvalidConfig, part)
		}
		key, item := pair[0], pair[1]
		seen[key] = true
		switch key {
		case "FREQ":
			switch item {
			case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
				frequency = item
			default:
				return fmt.Errorf("%w: RRULE FREQ=%s 无效", ErrInvalidConfig, item)
			}
		case "INTERVAL":
			parsed, err := strconv.Atoi(item)
			if err != nil || parsed < 1 || parsed > 10000 {
				return fmt.Errorf("%w: RRULE INTERVAL=%s 无效", ErrInvalidConfig, item)
			}
		case "BYDAY":
			for _, day := range strings.Split(item, ",") {
				if _, ok := recurrenceWeekdays[day]; !ok {
					return fmt.Errorf("%w: RRULE BYDAY=%s 无效", ErrInvalidConfig, item)
				}
			}
		case "BYMONTH":
			for _, month := range strings.Split(item, ",") {
				parsed, err := strconv.Atoi(month)
				if err != nil || parsed < 1 || parsed > 12 {
					return fmt.Errorf("%w: RRULE BYMONTH=%s 无效", ErrInvalidConfig, item)
				}
			}
		case "BYMONTHDAY":
			for _, day := range strings.Split(item, ",") {
				parsed, err := strconv.Atoi(day)
				if err != nil || parsed < 1 || parsed > 31 {
					return fmt.Errorf("%w: RRULE BYMONTHDAY=%s 无效", ErrInvalidConfig, item)
				}
			}
		case "COUNT":
			parsed, err := strconv.Atoi(item)
			if err != nil || parsed < 1 || parsed > 10000 {
				return fmt.Errorf("%w: RRULE COUNT=%s 无效", ErrInvalidConfig, item)
			}
		case "UNTIL":
			if !validRecurrenceUntil(item, location) {
				return fmt.Errorf("%w: RRULE UNTIL=%s 无效", ErrInvalidConfig, item)
			}
		default:
			return fmt.Errorf("%w: RRULE 参数 %s 不受支持", ErrInvalidConfig, key)
		}
	}
	if frequency == "" {
		return fmt.Errorf("%w: RRULE 缺少 FREQ", ErrInvalidConfig)
	}
	return nil
}

func validRecurrenceUntil(value string, location *time.Location) bool {
	if _, err := time.Parse("20060102T150405Z", value); err == nil {
		return true
	}
	if _, err := time.ParseInLocation("20060102T150405", value, location); err == nil {
		return true
	}
	_, err := time.ParseInLocation("20060102", value, location)
	return err == nil
}

func decodeStrictJSON(raw string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("包含额外内容")
		}
		return err
	}
	return nil
}
