package alarms

import "fmt"

const (
	CtxKeyAlarmID  = "alarm_id"
	CtxKeyPlatform = "platform"
	DefaultSoundID = "system-default"
	MaxAlertTitle  = 128
)

var AllowedWeekdays = []string{
	"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
}

var SupportedActions = []string{"dismiss", "repeat", "open", "pause", "resume"}

var AllowedAlarmIntentActions = []string{
	"open_alarm_details",
	"mark_alarm_acknowledged",
	"open_chat",
}

var AlarmKitTintColors = []string{
	"system",
	"amitia-blue",
	"amitia-green",
	"amitia-orange",
}

var SupportedSecondaryBehaviors = []string{"countdown", "custom"}

var SupportedKinds = []string{"alarm"}

func IsValidWeekday(w string) bool {
	for _, v := range AllowedWeekdays {
		if v == w {
			return true
		}
	}
	return false
}

func IsValidAction(a string) bool {
	for _, v := range SupportedActions {
		if v == a {
			return true
		}
	}
	return false
}

func IsValidAlarmIntentAction(a string) bool {
	for _, v := range AllowedAlarmIntentActions {
		if v == a {
			return true
		}
	}
	return false
}

func IsValidTintColor(c string) bool {
	for _, v := range AlarmKitTintColors {
		if v == c {
			return true
		}
	}
	return false
}

func IsValidSecondaryBehavior(b string) bool {
	for _, v := range SupportedSecondaryBehaviors {
		if v == b {
			return true
		}
	}
	return false
}

func IsValidKind(k string) bool {
	for _, v := range SupportedKinds {
		if v == k {
			return true
		}
	}
	return false
}

func ValidateWeekdays(recurrence Recurrence, weekdays []string) error {
	if recurrence == RecurrenceWeekly {
		if len(weekdays) == 0 {
			return fmt.Errorf("%v: weekly recurrence requires at least one weekday", ErrAlarmsScheduleInvalid)
		}
		for _, w := range weekdays {
			if !IsValidWeekday(w) {
				return fmt.Errorf("%v: invalid weekday %q", ErrAlarmsScheduleInvalid, w)
			}
		}
	}
	return nil
}

func ValidateSchedule(s *IOSAlarmSchedule) error {
	if s == nil {
		return fmt.Errorf("%v: schedule is required for alarm kind", ErrAlarmsScheduleInvalid)
	}
	if s.Recurrence != "" && s.Recurrence != string(RecurrenceNever) && s.Recurrence != string(RecurrenceWeekly) {
		return fmt.Errorf("%v: invalid recurrence %q", ErrAlarmsScheduleInvalid, s.Recurrence)
	}
	return ValidateWeekdays(Recurrence(s.Recurrence), s.Weekdays)
}

func ValidateCountdown(c *IOSAlarmCountdown) error {
	if c == nil {
		return nil
	}
	if c.PreAlertSeconds != nil && *c.PreAlertSeconds <= 0 {
		return fmt.Errorf("%v: preAlertSeconds must be positive", ErrAlarmsCountdownInvalid)
	}
	if c.PostAlertSeconds != nil && *c.PostAlertSeconds <= 0 {
		return fmt.Errorf("%v: postAlertSeconds must be positive", ErrAlarmsCountdownInvalid)
	}
	return nil
}

func ValidatePresentation(p IOSAlarmPresentation) error {
	if p.AlertTitle == "" {
		return fmt.Errorf("%v: alertTitle is required", ErrAlarmsPresentationInvalid)
	}
	if len(p.AlertTitle) > MaxAlertTitle {
		return fmt.Errorf("%v: alertTitle exceeds maximum length %d", ErrAlarmsPresentationInvalid, MaxAlertTitle)
	}
	if p.TintColor != "" && !IsValidTintColor(p.TintColor) {
		return fmt.Errorf("%v: invalid tintColor %q", ErrAlarmsPresentationInvalid, p.TintColor)
	}
	if p.SecondaryAction != "" && !IsValidSecondaryBehavior(p.SecondaryAction) {
		return fmt.Errorf("%v: invalid secondaryAction %q", ErrAlarmsSecondaryActionInvalid, p.SecondaryAction)
	}
	return nil
}

func ValidateSound(s IOSAlarmSound) error {
	if s.Kind == "" {
		return nil
	}
	if s.Kind != "default" && s.Kind != "named" {
		return fmt.Errorf("%v: invalid sound kind %q", ErrAlarmsSoundInvalid, s.Kind)
	}
	if s.Kind == "named" && s.SoundID == "" {
		return fmt.Errorf("%v", ErrAlarmsSoundNotRegistered)
	}
	return nil
}

func ValidateScheduleRequest(req IOSAlarmScheduleRequest) error {
	if !IsValidKind(req.Kind) {
		return fmt.Errorf("%v: invalid kind %q", ErrAlarmsScheduleInvalid, req.Kind)
	}
	if req.Title == "" {
		return fmt.Errorf("%v: title is required", ErrAlarmsScheduleInvalid)
	}
	if req.Kind == "alarm" || req.Kind == "countdown_alarm" {
		if err := ValidateSchedule(req.Schedule); err != nil {
			return err
		}
	}
	if req.Kind == "timer" || req.Kind == "countdown_alarm" {
		if req.Countdown == nil || (req.Countdown.PreAlertSeconds == nil && req.Countdown.PostAlertSeconds == nil) {
			return fmt.Errorf("%v: timer requires at least one countdown duration", ErrAlarmsCountdownInvalid)
		}
	}
	if err := ValidateCountdown(req.Countdown); err != nil {
		return err
	}
	if err := ValidatePresentation(req.Presentation); err != nil {
		return err
	}
	if err := ValidateSound(req.Sound); err != nil {
		return err
	}
	if req.Action != "" && !IsValidAlarmIntentAction(req.Action) {
		return fmt.Errorf("%v: invalid action %q", ErrAlarmsActionInvalid, req.Action)
	}
	return nil
}
