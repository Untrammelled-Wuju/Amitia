package schedule

import (
	"strings"
	"testing"
	"time"
)

func TestParseRRuleDefinitionNormalizesBoundedRule(t *testing.T) {
	parsed, err := parseRRuleDefinition("DTSTART;TZID=Asia/Shanghai:20261114T090000\nRRULE:FREQ=DAILY;COUNT=3;BYDAY=MO,TU,WE,TH,FR")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.location.String() != "Asia/Shanghai" {
		t.Fatalf("unexpected location: %s", parsed.location)
	}
	if !strings.Contains(parsed.normalized, "DTSTART;TZID=Asia/Shanghai:20261114T090000") {
		t.Fatalf("unexpected normalized start: %s", parsed.normalized)
	}
	if !strings.Contains(parsed.normalized, "RRULE:FREQ=DAILY") {
		t.Fatalf("unexpected normalized rule: %s", parsed.normalized)
	}
}

func TestParseRRuleDefinitionRejectsUnsupportedOrInvalidInput(t *testing.T) {
	cases := []string{
		"RRULE:FREQ=DAILY",
		"DTSTART:20261114T090000\nRRULE:FREQ=DAILY",
		"DTSTART;TZID=Asia/Shanghai:20261114T090000Z\nRRULE:FREQ=DAILY",
		"DTSTART:20261114T090000Z\nRRULE:FREQ=SECONDLY",
		"DTSTART:20261114T090000Z\nRRULE:FREQ=DAILY;COUNT=3;UNTIL=20261120T000000Z",
		"DTSTART:20261114T090000Z\nRRULE:FREQ=DAILY;BYSECOND=0,30",
		"DTSTART:20261114T090000Z\nRRULE:FREQ=WEEKLY;BYMONTHDAY=1",
		"DTSTART:20261114T090000Z\nRRULE:FREQ=MONTHLY;BYDAY=1MO;BYWEEKNO=3",
		"DTSTART:20261114T090000Z\nRRULE:FREQ=DAILY;BYSETPOS=1",
		"DTSTART:20261114T090000Z\nRRULE:FREQ=DAILY;BYEASTER=1",
	}
	for _, value := range cases {
		if _, err := parseRRuleDefinition(value); err == nil {
			t.Errorf("expected rejection for %q", value)
		}
	}
}

func TestCalculateNextRRuleUsesCommittedOccurrence(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	calc := NewScheduleCalculator(clock)
	def := &ScheduleContributionDefinition{
		ScheduleID: "rrule-next",
		Timezone:   "Asia/Shanghai",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeRRule,
			RRule: &RRuleTriggerDefinition{
				Rule: "DTSTART;TZID=Asia/Shanghai:20261114T090000\nRRULE:FREQ=DAILY;COUNT=2",
			},
		},
	}
	first, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.NextScheduledAt == nil || !first.NextScheduledAt.Equal(time.Date(2026, 11, 14, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected first occurrence: %+v", first.NextScheduledAt)
	}
	second, err := calc.CalculateNext(def, &ScheduleState{LastScheduledAt: first.NextScheduledAt})
	if err != nil {
		t.Fatal(err)
	}
	if second.NextScheduledAt == nil || !second.NextScheduledAt.Equal(time.Date(2026, 11, 15, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected second occurrence: %+v", second.NextScheduledAt)
	}
	ended, err := calc.CalculateNext(def, &ScheduleState{LastScheduledAt: second.NextScheduledAt})
	if err != nil {
		t.Fatal(err)
	}
	if ended.NextScheduledAt != nil {
		t.Fatalf("expected exhausted rule, got %v", ended.NextScheduledAt)
	}
}

func TestCalculateNextRRuleRejectsTimezoneMismatch(t *testing.T) {
	calc := NewScheduleCalculator(NewFakeClock(time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)))
	_, err := calc.CalculateNext(&ScheduleContributionDefinition{
		Timezone: "UTC",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeRRule,
			RRule: &RRuleTriggerDefinition{
				Rule: "DTSTART;TZID=Asia/Shanghai:20261114T090000\nRRULE:FREQ=DAILY",
			},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "does not match schedule timezone") {
		t.Fatalf("expected timezone mismatch, got %v", err)
	}
}
