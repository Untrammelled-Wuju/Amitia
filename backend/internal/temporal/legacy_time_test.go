package temporal

import (
	"testing"
	"time"
)

func TestLegacyTimeParserPreservesExplicitOffset(t *testing.T) {
	parsed := (DefaultLegacyTimeParser{}).Parse("2026-07-18T09:30:00-07:00", "Asia/Shanghai")
	want := time.Date(2026, 7, 18, 16, 30, 0, 0, time.UTC)
	if parsed.TimeUTC == nil || !parsed.TimeUTC.Equal(want) || parsed.Confidence != 100 || parsed.AssumedTimezone != "" {
		t.Fatalf("unexpected explicit result %#v", parsed)
	}
}

func TestLegacyTimeParserRequiresConfirmedAssumptionForNaiveTime(t *testing.T) {
	parsed := (DefaultLegacyTimeParser{}).Parse("2026-07-18 09:30:00", "America/Los_Angeles")
	want := time.Date(2026, 7, 18, 16, 30, 0, 0, time.UTC)
	if parsed.TimeUTC == nil || !parsed.TimeUTC.Equal(want) || parsed.AssumedTimezone != "America/Los_Angeles" || parsed.Warning == "" || parsed.Confidence >= 100 {
		t.Fatalf("unexpected assumed result %#v", parsed)
	}
}

func TestLegacyTimeParserRejectsInvalidTimezoneWithoutUTCGuess(t *testing.T) {
	parsed := (DefaultLegacyTimeParser{}).Parse("2026-07-18 09:30:00", "invalid/zone")
	if parsed.TimeUTC != nil || parsed.Warning != "invalid_assumed_timezone" {
		t.Fatalf("legacy parser silently guessed UTC: %#v", parsed)
	}
}
