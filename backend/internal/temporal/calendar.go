package temporal

import (
	"context"
	"strings"
	"time"
)

type CalendarQuery struct {
	From           time.Time `json:"from"`
	To             time.Time `json:"to"`
	Timezone       string    `json:"timezone"`
	HolidayRegion  string    `json:"holidayRegion"`
	CalendarSystem string    `json:"calendarSystem"`
}

type CalendarEvent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	LocalDate string `json:"localDate"`
	Kind      string `json:"kind"`
	Region    string `json:"region,omitempty"`
	Source    string `json:"source"`
}

type CalendarProvider interface {
	Events(ctx context.Context, query CalendarQuery) ([]CalendarEvent, error)
}

type StaticCalendarProvider struct{}

func NewStaticCalendarProvider() *StaticCalendarProvider { return &StaticCalendarProvider{} }

func (p *StaticCalendarProvider) Events(ctx context.Context, query CalendarQuery) ([]CalendarEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	region := strings.ToUpper(strings.TrimSpace(query.HolidayRegion))
	if region == "" {
		return nil, nil
	}
	definitions := staticHolidayDefinitions[region]
	result := []CalendarEvent{}
	for cursor := query.From; !cursor.After(query.To); cursor = cursor.AddDate(0, 0, 1) {
		key := cursor.Format("01-02")
		if name := definitions[key]; name != "" {
			result = append(result, CalendarEvent{ID: "static:" + region + ":" + cursor.Format("2006-01-02"), Name: name, LocalDate: cursor.Format("2006-01-02"), Kind: "holiday", Region: region, Source: "static-calendar"})
		}
	}
	return result, nil
}

var staticHolidayDefinitions = map[string]map[string]string{
	"CN": {"01-01": "元旦", "05-01": "劳动节", "10-01": "国庆节"},
	"US": {"01-01": "New Year's Day", "07-04": "Independence Day", "12-25": "Christmas Day"},
}

func (s *Service) resolveCalendarEvents(ctx context.Context, profile Profile, local time.Time) []CalendarEvent {
	if !profile.HolidayAwareness || profile.HolidayRegion == "" {
		return nil
	}
	query := CalendarQuery{From: time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()), To: time.Date(local.Year(), local.Month(), local.Day(), 23, 59, 59, 0, local.Location()), Timezone: profile.Timezone, HolidayRegion: profile.HolidayRegion, CalendarSystem: profile.CalendarSystem}
	result := []CalendarEvent{}
	for _, provider := range s.calendars {
		events, err := provider.Events(ctx, query)
		if err == nil {
			result = append(result, events...)
		}
	}
	return result
}
