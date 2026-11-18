package schedule

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"
)

var (
	rruleStartPattern = regexp.MustCompile(`^DTSTART(?:;TZID=([^:]+))?:(\d{8}T\d{6})(Z)?$`)
	rruleDayPattern   = regexp.MustCompile(`^([+-]?\d{1,2})?(MO|TU|WE|TH|FR|SA|SU)$`)
)

type parsedRRuleDefinition struct {
	rule       *rrule.RRule
	location   *time.Location
	normalized string
}

func parseRRuleDefinition(value string) (*parsedRRuleDefinition, error) {
	lines := make([]string, 0, 2)
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 2 {
		return nil, fmt.Errorf("RRULE must contain exactly DTSTART and RRULE lines")
	}
	startMatch := rruleStartPattern.FindStringSubmatch(lines[0])
	if startMatch == nil {
		return nil, fmt.Errorf("RRULE DTSTART must use a real YYYYMMDDTHHMMSS value with Z or an explicit IANA TZID")
	}
	timeZone := startMatch[1]
	stamp := startMatch[2]
	hasUTC := startMatch[3] == "Z"
	if hasUTC != (timeZone == "") {
		return nil, fmt.Errorf("RRULE DTSTART must use either TZID or Z")
	}
	var location *time.Location
	var start time.Time
	var err error
	if hasUTC {
		location = time.UTC
		start, err = time.Parse("20060102T150405Z", stamp+"Z")
	} else {
		location, err = time.LoadLocation(timeZone)
		if err == nil {
			start, err = time.ParseInLocation("20060102T150405", stamp, location)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("parse RRULE DTSTART: %w", err)
	}
	if start.Year() < 100 || start.Year() > 9999 || start.Format("20060102T150405") != stamp {
		return nil, fmt.Errorf("RRULE DTSTART is not a real local date-time")
	}
	if !strings.HasPrefix(lines[1], "RRULE:") {
		return nil, fmt.Errorf("RRULE input requires an RRULE line")
	}
	ruleText := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(lines[1], "RRULE:")))
	if ruleText == "" {
		return nil, fmt.Errorf("RRULE line is empty")
	}
	if err := validateRRuleText(ruleText); err != nil {
		return nil, err
	}
	options, err := rrule.StrToROptionInLocation("RRULE:"+ruleText, location)
	if err != nil {
		return nil, fmt.Errorf("parse RRULE: %w", err)
	}
	options.Dtstart = start
	rule, err := rrule.NewRRule(*options)
	if err != nil {
		return nil, fmt.Errorf("build RRULE: %w", err)
	}
	return &parsedRRuleDefinition{
		rule:       rule,
		location:   location,
		normalized: rule.String(),
	}, nil
}

func validateRRuleText(value string) error {
	parts := strings.Split(value, ";")
	seen := map[string]bool{}
	byParts := map[string][]string{}
	frequency := ""
	countText := ""
	untilText := ""
	for _, part := range parts {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 || pair[0] == "" || pair[1] == "" {
			return fmt.Errorf("RRULE field %q is invalid", part)
		}
		key, field := pair[0], pair[1]
		if seen[key] {
			return fmt.Errorf("RRULE field %s is duplicated", key)
		}
		seen[key] = true
		switch key {
		case "FREQ":
			if frequency != "" {
				return fmt.Errorf("RRULE FREQ is duplicated")
			}
			frequency = field
		case "INTERVAL":
			value, err := positiveRRuleInteger(field)
			if err != nil {
				return fmt.Errorf("RRULE INTERVAL: %w", err)
			}
			if value > int(^uint(0)>>1) {
				return fmt.Errorf("RRULE INTERVAL is too large")
			}
		case "COUNT":
			value, err := positiveRRuleInteger(field)
			if err != nil || value > 1000 {
				return fmt.Errorf("RRULE COUNT must be an integer from 1 through 1000")
			}
			countText = field
		case "UNTIL":
			if !strings.HasSuffix(field, "Z") {
				return fmt.Errorf("RRULE UNTIL must be an absolute UTC date-time ending in Z")
			}
			if _, err := time.Parse("20060102T150405Z", field); err != nil {
				return fmt.Errorf("RRULE UNTIL is invalid: %w", err)
			}
			untilText = field
		case "WKST":
			if _, _, err := parseRRuleWeekday(field); err != nil || len(field) != 2 {
				return fmt.Errorf("RRULE WKST must be MO through SU")
			}
		case "BYDAY":
			days := strings.Split(field, ",")
			unique := map[string]bool{}
			for _, day := range days {
				if unique[day] {
					return fmt.Errorf("RRULE BYDAY contains duplicate %s", day)
				}
				unique[day] = true
				if _, _, err := parseRRuleWeekday(day); err != nil {
					return err
				}
			}
			byParts[key] = days
		case "BYSETPOS":
			if err := validateRRuleIntegers(field, -366, 366, false); err != nil {
				return fmt.Errorf("RRULE BYSETPOS: %w", err)
			}
			if len(strings.Split(field, ",")) > 31 {
				return fmt.Errorf("RRULE BYSETPOS accepts at most 31 values")
			}
			byParts[key] = strings.Split(field, ",")
		case "BYMONTH":
			if err := validateRRuleIntegers(field, 1, 12, false); err != nil {
				return fmt.Errorf("RRULE BYMONTH: %w", err)
			}
			byParts[key] = strings.Split(field, ",")
		case "BYMONTHDAY":
			if err := validateRRuleIntegers(field, 1, 31, true); err != nil {
				return fmt.Errorf("RRULE BYMONTHDAY: %w", err)
			}
			byParts[key] = strings.Split(field, ",")
		case "BYYEARDAY":
			if err := validateRRuleIntegers(field, 1, 366, true); err != nil {
				return fmt.Errorf("RRULE BYYEARDAY: %w", err)
			}
			byParts[key] = strings.Split(field, ",")
		case "BYWEEKNO":
			if err := validateRRuleIntegers(field, 1, 53, true); err != nil {
				return fmt.Errorf("RRULE BYWEEKNO: %w", err)
			}
			byParts[key] = strings.Split(field, ",")
		case "BYHOUR":
			if err := validateRRuleIntegers(field, 0, 23, false); err != nil {
				return fmt.Errorf("RRULE BYHOUR: %w", err)
			}
			byParts[key] = strings.Split(field, ",")
		case "BYMINUTE":
			if err := validateRRuleIntegers(field, 0, 59, false); err != nil {
				return fmt.Errorf("RRULE BYMINUTE: %w", err)
			}
			byParts[key] = strings.Split(field, ",")
		case "BYSECOND":
			if err := validateRRuleIntegers(field, 0, 59, false); err != nil {
				return fmt.Errorf("RRULE BYSECOND: %w", err)
			}
			if len(strings.Split(field, ",")) != 1 {
				return fmt.Errorf("RRULE BYSECOND accepts exactly one value")
			}
			byParts[key] = strings.Split(field, ",")
		default:
			return fmt.Errorf("RRULE field %s is not supported", key)
		}
	}
	switch frequency {
	case "MINUTELY", "HOURLY", "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
	default:
		return fmt.Errorf("RRULE FREQ must be MINUTELY, HOURLY, DAILY, WEEKLY, MONTHLY, or YEARLY")
	}
	if countText != "" && untilText != "" {
		return fmt.Errorf("RRULE COUNT and UNTIL cannot be combined")
	}
	if frequency == "MINUTELY" || frequency == "HOURLY" {
		for key := range byParts {
			return fmt.Errorf("RRULE %s cannot be used with FREQ=%s", key, frequency)
		}
	}
	hasOrdinalWeekday := false
	for _, day := range byParts["BYDAY"] {
		_, ordinal, err := parseRRuleWeekday(day)
		if err != nil {
			return err
		}
		if ordinal != 0 && !(frequency == "MONTHLY" || frequency == "YEARLY") {
			return fmt.Errorf("RRULE ordinal BYDAY requires FREQ=MONTHLY or YEARLY")
		}
		hasOrdinalWeekday = hasOrdinalWeekday || ordinal != 0
	}
	if len(byParts["BYMONTHDAY"]) > 0 && !(frequency == "MONTHLY" || frequency == "YEARLY") {
		return fmt.Errorf("RRULE BYMONTHDAY requires FREQ=MONTHLY or YEARLY")
	}
	if len(byParts["BYYEARDAY"]) > 0 && frequency != "YEARLY" {
		return fmt.Errorf("RRULE BYYEARDAY requires FREQ=YEARLY")
	}
	if len(byParts["BYWEEKNO"]) > 0 && frequency != "YEARLY" {
		return fmt.Errorf("RRULE BYWEEKNO requires FREQ=YEARLY")
	}
	if len(byParts["BYWEEKNO"]) > 0 && hasOrdinalWeekday {
		return fmt.Errorf("RRULE ordinal BYDAY cannot be combined with BYWEEKNO")
	}
	if len(byParts["BYSETPOS"]) > 0 {
		hasOtherByPart := false
		for key := range byParts {
			hasOtherByPart = hasOtherByPart || key != "BYSETPOS"
		}
		if !hasOtherByPart {
			return fmt.Errorf("RRULE BYSETPOS requires another BY field")
		}
	}
	for _, value := range byParts["BYSETPOS"] {
		parsed, _ := strconv.Atoi(value)
		if parsed == 0 {
			return fmt.Errorf("RRULE BYSETPOS cannot contain zero")
		}
	}
	return nil
}

func positiveRRuleInteger(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("value must be a positive integer")
	}
	return parsed, nil
}

func validateRRuleIntegers(value string, minimum int, maximum int, negative bool) error {
	seen := map[int]bool{}
	for _, item := range strings.Split(value, ",") {
		parsed, err := strconv.Atoi(item)
		if err != nil || parsed == 0 {
			return fmt.Errorf("value %q is not a non-zero integer", item)
		}
		if seen[parsed] {
			return fmt.Errorf("value %d is duplicated", parsed)
		}
		seen[parsed] = true
		if parsed < minimum || parsed > maximum {
			if !negative || parsed > -minimum || parsed < -maximum {
				return fmt.Errorf("value %d is outside the supported range", parsed)
			}
		}
	}
	return nil
}

func parseRRuleWeekday(value string) (string, int, error) {
	match := rruleDayPattern.FindStringSubmatch(value)
	if match == nil {
		return "", 0, fmt.Errorf("RRULE weekday %q is invalid", value)
	}
	ordinal := 0
	if match[1] != "" {
		parsed, err := strconv.Atoi(match[1])
		if err != nil || parsed == 0 || parsed < -53 || parsed > 53 {
			return "", 0, fmt.Errorf("RRULE weekday ordinal %q is invalid", match[1])
		}
		ordinal = parsed
	}
	return match[2], ordinal, nil
}
