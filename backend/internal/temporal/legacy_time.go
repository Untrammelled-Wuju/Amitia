package temporal

import (
	"strings"
	"time"
)

type ParsedLegacyTime struct {
	TimeUTC         *time.Time `json:"timeUtc,omitempty"`
	Confidence      int        `json:"confidence"`
	AssumedTimezone string     `json:"assumedTimezone,omitempty"`
	Warning         string     `json:"warning,omitempty"`
}

type LegacyTimeParser interface {
	Parse(value string, assumedTimezone string) ParsedLegacyTime
}

type DefaultLegacyTimeParser struct{}

func (DefaultLegacyTimeParser) Parse(value string, assumedTimezone string) ParsedLegacyTime {
	value = strings.TrimSpace(value)
	if value == "" {
		return ParsedLegacyTime{Warning: "empty_time"}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			result := utc(parsed)
			return ParsedLegacyTime{TimeUTC: &result, Confidence: 100}
		}
	}
	location, err := loadLocation(assumedTimezone)
	if err != nil {
		return ParsedLegacyTime{Warning: "invalid_assumed_timezone"}
	}
	for _, candidate := range []struct {
		layout     string
		confidence int
	}{
		{layout: "2006-01-02 15:04:05.999999999", confidence: 75},
		{layout: "2006-01-02 15:04:05", confidence: 75},
		{layout: "2006-01-02 15:04", confidence: 70},
		{layout: "2006-01-02", confidence: 55},
	} {
		if parsed, parseErr := time.ParseInLocation(candidate.layout, value, location); parseErr == nil {
			result := utc(parsed)
			return ParsedLegacyTime{TimeUTC: &result, Confidence: candidate.confidence, AssumedTimezone: assumedTimezone, Warning: "timezone_assumed_requires_confirmation"}
		}
	}
	return ParsedLegacyTime{AssumedTimezone: assumedTimezone, Warning: "unrecognized_time_format"}
}
