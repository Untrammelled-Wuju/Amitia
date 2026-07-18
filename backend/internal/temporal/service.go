package temporal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidTimezone = errors.New("invalid IANA timezone")
	ErrInvalidOwner    = errors.New("invalid temporal profile owner")
	ErrAnchorNotFound  = errors.New("temporal anchor not found")
	ErrScopeMismatch   = errors.New("temporal scope mismatch")
)

type ScheduleProvider interface {
	CurrentState(characterID string, at time.Time) (ScheduleTemporalSnapshot, error)
}

type RelationshipTimeProvider interface {
	Resolve(ctx context.Context, input SnapshotInput, nowUTC time.Time) (*RelationshipTimeContext, error)
}

type Service struct {
	repo               Repository
	clock              Clock
	schedule           ScheduleProvider
	relationshipTime   RelationshipTimeProvider
	flags              FeatureFlags
	calendars          []CalendarProvider
	candidatePublisher CandidatePublisher
	metrics            *temporalMetrics
}

func NewService(repo Repository, clock Clock) *Service {
	return NewServiceWithFlags(repo, clock, FeatureFlagsFromEnvironment())
}

func NewServiceWithFlags(repo Repository, clock Clock, flags FeatureFlags) *Service {
	if clock == nil {
		clock = SystemClock{}
	}
	return &Service{repo: repo, clock: clock, flags: flags, calendars: []CalendarProvider{NewStaticCalendarProvider()}, metrics: &temporalMetrics{}}
}

func (s *Service) SetScheduleProvider(provider ScheduleProvider) { s.schedule = provider }
func (s *Service) SetRelationshipTimeProvider(provider RelationshipTimeProvider) {
	s.relationshipTime = provider
}
func (s *Service) FeatureFlags() FeatureFlags                         { return s.flags }
func (s *Service) SetCalendarProviders(providers ...CalendarProvider) { s.calendars = providers }

func (s *Service) InitSchema() error { return s.repo.InitSchema() }

func defaultProfile(ownerType, ownerID string, now time.Time) *Profile {
	mode := TimezoneFollowDevice
	if ownerType == OwnerCharacter {
		mode = TimezoneFollowUser
	}
	return &Profile{
		ID: uuid.NewString(), OwnerType: ownerType, OwnerID: ownerID, TimezoneMode: mode,
		Timezone: DefaultTimezone, Locale: "zh-CN", CalendarSystem: "gregorian", WeekStart: 1,
		Hemisphere: "unknown", DaypartConfigJSON: "{}", QuietHoursJSON: `{"enabled":true,"start":"23:00","end":"07:00"}`,
		AutoDetectTimezone: true, AwarenessLevel: 70, Source: "fallback", Confidence: 30,
		Enabled: true, HolidayAwareness: true, DaypartAwareness: true, AnniversaryAwareness: true,
		MemoryResonance: true, AllowSharedDateMention: true, Version: 1,
		CreatedAtUTC: utc(now), UpdatedAtUTC: utc(now),
	}
}

func normalizeUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DefaultUserOwnerID
	}
	return userID
}

func (s *Service) GetProfile(ctx context.Context, ownerType, ownerID string) (*Profile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ownerType = strings.TrimSpace(ownerType)
	ownerID = strings.TrimSpace(ownerID)
	if ownerType == OwnerUser {
		ownerID = normalizeUserID(ownerID)
	}
	if ownerType != OwnerUser && ownerType != OwnerCharacter || ownerID == "" {
		return nil, ErrInvalidOwner
	}
	profile, err := s.repo.GetProfile(ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	if profile != nil {
		return profile, nil
	}
	return defaultProfile(ownerType, ownerID, s.clock.Now()), nil
}

func (s *Service) SaveProfile(ctx context.Context, ownerType, ownerID string, input Profile) (*Profile, error) {
	if ownerType == OwnerUser {
		ownerID = normalizeUserID(ownerID)
	}
	current, err := s.GetProfile(ctx, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	if ownerType == OwnerCharacter {
		exists, existsErr := s.repo.CharacterExists(ownerID)
		if existsErr != nil {
			return nil, existsErr
		}
		if !exists {
			return nil, ErrScopeMismatch
		}
	}
	if input.Timezone == "" {
		input.Timezone = current.Timezone
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return nil, ErrInvalidTimezone
	}
	input.ID = current.ID
	input.OwnerType = ownerType
	input.OwnerID = ownerID
	input.CreatedAtUTC = current.CreatedAtUTC
	input.UpdatedAtUTC = utc(s.clock.Now())
	input.Version = current.Version + 1
	if input.TimezoneMode == "" {
		input.TimezoneMode = current.TimezoneMode
	}
	if input.Locale == "" {
		input.Locale = current.Locale
	}
	if input.CalendarSystem == "" {
		input.CalendarSystem = "gregorian"
	}
	if input.Source == "" {
		input.Source = "explicit"
	}
	input.Confidence = clampInt(input.Confidence, 0, 100)
	input.AwarenessLevel = clampInt(input.AwarenessLevel, 0, 100)
	if err := s.repo.SaveProfile(&input); err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *Service) PatchProfile(ctx context.Context, ownerType, ownerID string, patch ProfilePatch) (*Profile, error) {
	current, err := s.GetProfile(ctx, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	if patch.TimezoneMode != nil {
		current.TimezoneMode = *patch.TimezoneMode
	}
	if patch.Timezone != nil {
		current.Timezone = *patch.Timezone
	}
	if patch.Locale != nil {
		current.Locale = *patch.Locale
	}
	if patch.CalendarSystem != nil {
		current.CalendarSystem = *patch.CalendarSystem
	}
	if patch.WeekStart != nil {
		current.WeekStart = *patch.WeekStart
	}
	if patch.HolidayRegion != nil {
		current.HolidayRegion = *patch.HolidayRegion
	}
	if patch.Hemisphere != nil {
		current.Hemisphere = *patch.Hemisphere
	}
	if patch.DaypartConfigJSON != nil {
		current.DaypartConfigJSON = *patch.DaypartConfigJSON
	}
	if patch.QuietHoursJSON != nil {
		current.QuietHoursJSON = *patch.QuietHoursJSON
	}
	if patch.AutoDetectTimezone != nil {
		current.AutoDetectTimezone = *patch.AutoDetectTimezone
	}
	if patch.TravelMode != nil {
		current.TravelMode = *patch.TravelMode
	}
	if patch.AwarenessLevel != nil {
		current.AwarenessLevel = *patch.AwarenessLevel
	}
	if patch.Source != nil {
		current.Source = *patch.Source
	}
	if patch.Confidence != nil {
		current.Confidence = *patch.Confidence
	}
	if patch.Enabled != nil {
		current.Enabled = *patch.Enabled
	}
	if patch.HolidayAwareness != nil {
		current.HolidayAwareness = *patch.HolidayAwareness
	}
	if patch.DaypartAwareness != nil {
		current.DaypartAwareness = *patch.DaypartAwareness
	}
	if patch.AnniversaryAwareness != nil {
		current.AnniversaryAwareness = *patch.AnniversaryAwareness
	}
	if patch.MemoryResonance != nil {
		current.MemoryResonance = *patch.MemoryResonance
	}
	if patch.AllowSharedDateMention != nil {
		current.AllowSharedDateMention = *patch.AllowSharedDateMention
	}
	return s.SaveProfile(ctx, ownerType, ownerID, *current)
}

func (s *Service) ResolveSnapshot(ctx context.Context, input SnapshotInput) (snapshot Snapshot, err error) {
	started := s.clock.Now()
	defer func() { s.metrics.recordSnapshot(s.clock.Since(started), err) }()
	return s.resolveSnapshot(ctx, input)
}

func (s *Service) resolveSnapshot(ctx context.Context, input SnapshotInput) (Snapshot, error) {
	now := utc(s.clock.Now())
	input.UserID = normalizeUserID(input.UserID)
	if input.DeviceTimezone == "" {
		input.DeviceTimezone = DeviceTimezoneFromContext(ctx)
	}
	if !s.flags.TemporalCoreEnabled {
		fallback := civilSnapshot(now, time.UTC, "", "unknown")
		var relationshipTime *RelationshipTimeContext
		if s.flags.RelationshipTimeEnabled && s.relationshipTime != nil {
			relationshipTime, _ = s.relationshipTime.Resolve(ctx, input, now)
		}
		return Snapshot{Version: SnapshotVersion, NowUTC: now, UserTime: fallback, CharacterTime: fallback, RelationshipTime: relationshipTime, Policy: TemporalBehaviorPolicy{MentionTime: "none", AllowProactive: true}, GeneratedAt: now}, nil
	}
	userProfile, err := s.GetProfile(ctx, OwnerUser, input.UserID)
	if err != nil {
		return Snapshot{}, err
	}
	characterProfile := defaultProfile(OwnerCharacter, input.CharacterID, now)
	if input.CharacterID != "" {
		characterProfile, err = s.GetProfile(ctx, OwnerCharacter, input.CharacterID)
		if err != nil {
			return Snapshot{}, err
		}
	}
	userTimezone := userProfile.Timezone
	userTimezoneSource := userProfile.Source
	userTimezoneConfidence := userProfile.Confidence
	if userProfile.TimezoneMode == TimezoneFollowDevice && userProfile.AutoDetectTimezone && strings.TrimSpace(input.DeviceTimezone) != "" {
		if deviceLocation, deviceErr := loadLocation(input.DeviceTimezone); deviceErr == nil {
			userTimezone = deviceLocation.String()
			userTimezoneSource = "device_session"
			userTimezoneConfidence = 80
		}
	}
	userLocation, err := loadLocation(userTimezone)
	if err != nil {
		return Snapshot{}, err
	}
	characterTimezone := characterProfile.Timezone
	if characterProfile.TimezoneMode == TimezoneFollowUser || characterTimezone == "" {
		characterTimezone = userTimezone
	}
	characterLocation, err := loadLocation(characterTimezone)
	if err != nil {
		return Snapshot{}, err
	}
	userCivil := civilSnapshot(now, userLocation, userProfile.DaypartConfigJSON, userProfile.Hemisphere)
	characterCivil := civilSnapshot(now, characterLocation, characterProfile.DaypartConfigJSON, characterProfile.Hemisphere)
	if !userProfile.DaypartAwareness {
		userCivil.Daypart = ""
	}
	if !characterProfile.DaypartAwareness {
		characterCivil.Daypart = ""
	}
	anchors, err := s.resolveSalientAnchors(input.UserID, input.CharacterID, now, userLocation)
	if err != nil {
		return Snapshot{}, err
	}
	schedule := ScheduleTemporalSnapshot{}
	if s.schedule != nil && input.CharacterID != "" {
		schedule, _ = s.schedule.CurrentState(input.CharacterID, now)
	}
	quiet := inQuietHours(userCivil.LocalTime, userProfile.QuietHoursJSON)
	calendarEvents := s.resolveCalendarEvents(ctx, *userProfile, userCivil.LocalTime)
	policy := TemporalBehaviorPolicy{MentionTime: "subtle", AllowProactive: userProfile.Enabled && !quiet, MaxTemporalMentions: 1}
	if !userProfile.Enabled || !characterProfile.Enabled {
		policy.MentionTime = "none"
		anchors = nil
	}
	var relationshipTime *RelationshipTimeContext
	if s.flags.RelationshipTimeEnabled && s.relationshipTime != nil {
		relationshipTime, _ = s.relationshipTime.Resolve(ctx, input, now)
	}
	return Snapshot{
		Version: SnapshotVersion, NowUTC: now, UserTime: userCivil, CharacterTime: characterCivil,
		RelationshipTime: relationshipTime, Schedule: schedule, CalendarEvents: calendarEvents, SalientAnchors: anchors,
		Signals: TemporalSignals{TimezoneDiffers: userTimezone != characterTimezone, QuietHours: quiet, UserTimezoneSource: userTimezoneSource, UserTimezoneConfidence: userTimezoneConfidence, UserTimezoneConfirmed: userTimezoneSource != "fallback" && userTimezoneConfidence >= 60},
		Policy:  policy, GeneratedAt: now,
	}, nil
}

func (s *Service) RenderSnapshot(snapshot Snapshot) string {
	return RenderSnapshot(snapshot)
}

func RenderSnapshot(snapshot Snapshot) string {
	if snapshot.Policy.MentionTime == "none" {
		return ""
	}
	userTimeLabel := "用户当地时间"
	if !snapshot.Signals.UserTimezoneConfirmed || snapshot.Signals.UserTimezoneConfidence < 60 {
		userTimeLabel = "系统参考时间"
	}
	userTimeLine := fmt.Sprintf("%s：%s，%s，%s", userTimeLabel, snapshot.UserTime.LocalTime.Format("2006-01-02 15:04"), weekdayCN(snapshot.UserTime.LocalTime.Weekday()), daypartCN(snapshot.UserTime.Daypart))
	if userTimeLabel == "系统参考时间" {
		userTimeLine += "（用户时区未确认）"
	}
	lines := []string{"【当前时间上下文】", userTimeLine, fmt.Sprintf("角色当地时间：%s，%s，%s", snapshot.CharacterTime.LocalTime.Format("2006-01-02 15:04"), weekdayCN(snapshot.CharacterTime.LocalTime.Weekday()), daypartCN(snapshot.CharacterTime.Daypart))}
	if snapshot.Schedule.CurrentState != "" {
		lines = append(lines, "当前角色状态："+snapshot.Schedule.CurrentState)
	}
	for _, anchor := range snapshot.SalientAnchors {
		lines = append(lines, fmt.Sprintf("相关时间锚点：%s（显著性 %.2f）", anchor.Title, anchor.Salience))
	}
	lines = append(lines, "表达策略：仅在相关时自然使用时间事实；不要臆测用户正在睡觉、工作或吃饭")
	return strings.Join(lines, "\n")
}

func (s *Service) ListAnchors(ctx context.Context, query AnchorQuery) ([]Anchor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query.UserID = normalizeUserID(query.UserID)
	return s.repo.ListAnchors(query)
}

func (s *Service) SaveAnchor(ctx context.Context, userID, characterID string, anchor Anchor) (*Anchor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	userID = normalizeUserID(userID)
	if characterID != "" {
		exists, err := s.repo.CharacterExists(characterID)
		if err != nil || !exists {
			if err != nil {
				return nil, err
			}
			return nil, ErrScopeMismatch
		}
	}
	now := utc(s.clock.Now())
	if anchor.ID == "" {
		anchor.ID = uuid.NewString()
		anchor.CreatedAtUTC = now
	} else {
		current, err := s.repo.GetAnchor(anchor.ID)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, ErrAnchorNotFound
		}
		if current.UserID != userID || current.CharacterID != characterID {
			return nil, ErrScopeMismatch
		}
		anchor.CreatedAtUTC = current.CreatedAtUTC
	}
	anchor.UserID = userID
	anchor.CharacterID = characterID
	anchor.UpdatedAtUTC = now
	if anchor.Source == "plugin" || anchor.Source == "model" {
		anchor.RequiresConfirmation = true
		anchor.Status = "candidate"
		anchor.AllowProactiveMention = false
	}
	if anchor.ScopeType == "" {
		if characterID == "" {
			anchor.ScopeType = OwnerUser
		} else {
			anchor.ScopeType = "relationship"
		}
	}
	if anchor.Status == "" {
		if anchor.RequiresConfirmation {
			anchor.Status = "candidate"
		} else {
			anchor.Status = "active"
		}
	}
	if anchor.Timezone == "" {
		anchor.Timezone = DefaultTimezone
	}
	if _, err := loadLocation(anchor.Timezone); err != nil {
		return nil, err
	}
	anchor.Importance = clampInt(anchor.Importance, 0, 100)
	anchor.Confidence = clampInt(anchor.Confidence, 0, 100)
	anchor.NextOccurrenceAtUTC = nextAnchorOccurrence(anchor, now)
	if err := s.repo.SaveAnchor(&anchor); err != nil {
		return nil, err
	}
	return &anchor, nil
}

func (s *Service) ConfirmAnchor(ctx context.Context, userID, characterID, id string) (*Anchor, error) {
	anchor, err := s.repo.GetAnchor(id)
	if err != nil {
		return nil, err
	}
	if anchor == nil {
		return nil, ErrAnchorNotFound
	}
	if anchor.UserID != normalizeUserID(userID) || anchor.CharacterID != characterID {
		return nil, ErrScopeMismatch
	}
	anchor.Status = "active"
	anchor.RequiresConfirmation = false
	anchor.UpdatedAtUTC = utc(s.clock.Now())
	if err := s.repo.SaveAnchor(anchor); err != nil {
		return nil, err
	}
	return anchor, nil
}

func (s *Service) DeleteAnchor(ctx context.Context, userID, characterID, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.repo.DeleteAnchor(id, normalizeUserID(userID), characterID)
}

func (s *Service) ListEvents(ctx context.Context, userID, characterID string, limit int) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.repo.ListEvents(normalizeUserID(userID), characterID, limit)
}

func (s *Service) SuggestTimezone(ctx context.Context, userID, timezone string) (*Profile, error) {
	if _, err := loadLocation(timezone); err != nil {
		return nil, err
	}
	profile, err := s.GetProfile(ctx, OwnerUser, userID)
	if err != nil {
		return nil, err
	}
	if profile.Timezone != timezone {
		profile.PendingTimezone = timezone
	}
	profile.UpdatedAtUTC = utc(s.clock.Now())
	if err := s.repo.SaveProfile(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *Service) ResolveTimezoneSuggestion(ctx context.Context, userID string, accept bool) (*Profile, error) {
	profile, err := s.GetProfile(ctx, OwnerUser, userID)
	if err != nil {
		return nil, err
	}
	if accept && profile.PendingTimezone != "" {
		profile.Timezone = profile.PendingTimezone
		profile.Source = "device"
		profile.Confidence = 80
	}
	profile.PendingTimezone = ""
	profile.Version++
	profile.UpdatedAtUTC = utc(s.clock.Now())
	if err := s.repo.SaveProfile(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func loadLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidTimezone
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, ErrInvalidTimezone
	}
	return location, nil
}

func civilSnapshot(now time.Time, location *time.Location, rawConfig, hemisphere string) CivilTimeSnapshot {
	local := now.In(location)
	_, offset := local.Zone()
	weekend := local.Weekday() == time.Saturday || local.Weekday() == time.Sunday
	return CivilTimeSnapshot{Timezone: location.String(), LocalTime: local, Weekday: local.Weekday().String(), Daypart: resolveDaypart(local, rawConfig), Season: resolveSeason(local, hemisphere), OffsetSeconds: offset, IsWeekend: weekend, IsWorkday: !weekend}
}

func resolveDaypart(local time.Time, rawConfig string) string {
	minutes := local.Hour()*60 + local.Minute()
	boundaries := []struct {
		name       string
		start, end int
	}{{"dawn", 300, 420}, {"morning", 420, 690}, {"noon", 690, 810}, {"afternoon", 810, 1080}, {"evening", 1080, 1350}, {"late_night", 1350, 120}, {"deep_night", 120, 300}}
	overridden := map[string]bool{}
	var overrides map[string]struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if json.Unmarshal([]byte(rawConfig), &overrides) == nil {
		for index, item := range boundaries {
			override, exists := overrides[item.name]
			if !exists {
				continue
			}
			start, startOK := clockMinutes(override.Start)
			end, endOK := clockMinutes(override.End)
			if startOK && endOK {
				boundaries[index].start, boundaries[index].end = start, end
				overridden[item.name] = true
			}
		}
	}
	for _, item := range boundaries {
		if overridden[item.name] && (item.start < item.end && minutes >= item.start && minutes < item.end || item.start > item.end && (minutes >= item.start || minutes < item.end)) {
			return item.name
		}
	}
	for _, item := range boundaries {
		if item.start < item.end && minutes >= item.start && minutes < item.end || item.start > item.end && (minutes >= item.start || minutes < item.end) {
			return item.name
		}
	}
	return "unknown"
}

func resolveSeason(local time.Time, hemisphere string) string {
	month := int(local.Month())
	season := "winter"
	switch {
	case month >= 3 && month <= 5:
		season = "spring"
	case month >= 6 && month <= 8:
		season = "summer"
	case month >= 9 && month <= 11:
		season = "autumn"
	}
	if hemisphere == "south" {
		opposite := map[string]string{"spring": "autumn", "summer": "winter", "autumn": "spring", "winter": "summer"}
		return opposite[season]
	}
	if hemisphere == "unknown" {
		return "unknown"
	}
	return season
}

func inQuietHours(local time.Time, raw string) bool {
	var config struct {
		Enabled bool   `json:"enabled"`
		Start   string `json:"start"`
		End     string `json:"end"`
	}
	if json.Unmarshal([]byte(raw), &config) != nil || !config.Enabled {
		return false
	}
	start, okStart := clockMinutes(config.Start)
	end, okEnd := clockMinutes(config.End)
	if !okStart || !okEnd {
		return false
	}
	now := local.Hour()*60 + local.Minute()
	if start < end {
		return now >= start && now < end
	}
	return now >= start || now < end
}

func clockMinutes(value string) (int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, errHour := strconv.Atoi(parts[0])
	minute, errMinute := strconv.Atoi(parts[1])
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func (s *Service) resolveSalientAnchors(userID, characterID string, now time.Time, userLocation *time.Location) ([]AnchorOccurrence, error) {
	anchors, err := s.repo.ListAnchors(AnchorQuery{UserID: userID, CharacterID: characterID, Status: "active", Limit: 200})
	if err != nil {
		return nil, err
	}
	result := []AnchorOccurrence{}
	for _, anchor := range anchors {
		if !anchor.AllowPromptMention || anchor.RequiresConfirmation || anchor.Confidence < 60 {
			continue
		}
		occurrence := nextAnchorOccurrence(anchor, now.Add(-48*time.Hour))
		if occurrence == nil {
			continue
		}
		distance := occurrence.Sub(now)
		window := time.Duration(anchor.PreWindowSeconds) * time.Second
		if window <= 0 {
			window = 72 * time.Hour
		}
		post := time.Duration(anchor.PostWindowSeconds) * time.Second
		if post <= 0 {
			post = 24 * time.Hour
		}
		if distance > window || distance < -post {
			continue
		}
		proximity := 1 - math.Min(1, math.Abs(distance.Hours())/math.Max(24, window.Hours()))
		salience := float64(anchor.Importance) / 100 * float64(anchor.Confidence) / 100 * proximity
		result = append(result, AnchorOccurrence{ID: anchor.ID, Type: anchor.AnchorType, Title: anchor.Title, DistanceDays: int(math.Round(distance.Hours() / 24)), Salience: math.Round(salience*100) / 100})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Salience > result[j].Salience })
	if len(result) > 2 {
		result = result[:2]
	}
	return result, nil
}

func nextAnchorOccurrence(anchor Anchor, from time.Time) *time.Time {
	if anchor.TimeKind == "instant" || anchor.TimeKind == "range" {
		if anchor.InstantAtUTC != nil {
			value := utc(*anchor.InstantAtUTC)
			return &value
		}
		return nil
	}
	location, err := loadLocation(anchor.Timezone)
	if err != nil {
		return nil
	}
	localFrom := from.In(location)
	if anchor.TimeKind == "local_datetime" {
		value, err := time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(anchor.LocalDate+" "+anchor.LocalTime), location)
		if err != nil {
			return nil
		}
		result := utc(value)
		return &result
	}
	if anchor.TimeKind == "local_date" {
		value, err := time.ParseInLocation("2006-01-02", anchor.LocalDate, location)
		if err != nil {
			return nil
		}
		hour, minute := 0, 0
		if parsed, ok := clockMinutes(anchor.LocalTime); ok {
			hour, minute = parsed/60, parsed%60
		}
		value = time.Date(value.Year(), value.Month(), value.Day(), hour, minute, 0, 0, location)
		result := utc(value)
		return &result
	}
	if anchor.TimeKind == "annual_date" {
		month, day, ok := parseMonthDay(anchor.LocalDate)
		if !ok {
			return nil
		}
		hour, minute := 0, 0
		if parsed, ok := clockMinutes(anchor.LocalTime); ok {
			hour, minute = parsed/60, parsed%60
		}
		year := localFrom.Year()
		value := safeLocalDate(year, month, day, hour, minute, location)
		if value.Before(localFrom) {
			value = safeLocalDate(year+1, month, day, hour, minute, location)
		}
		result := utc(value)
		return &result
	}
	if anchor.TimeKind == "recurring" {
		return nextRecurringOccurrence(anchor, from)
	}
	return nil
}

func parseMonthDay(value string) (time.Month, int, bool) {
	value = strings.TrimSpace(value)
	if len(value) >= 10 {
		value = value[len(value)-5:]
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	month, errM := strconv.Atoi(parts[0])
	day, errD := strconv.Atoi(parts[1])
	if errM != nil || errD != nil || month < 1 || month > 12 || day < 1 || day > 31 {
		return 0, 0, false
	}
	return time.Month(month), day, true
}

func safeLocalDate(year int, month time.Month, day, hour, minute int, location *time.Location) time.Time {
	value := time.Date(year, month, day, hour, minute, 0, 0, location)
	if month == time.February && day == 29 && value.Month() != time.February {
		return time.Date(year, time.February, 28, hour, minute, 0, 0, location)
	}
	return value
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
func weekdayCN(day time.Weekday) string {
	names := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	return names[int(day)]
}
func daypartCN(value string) string {
	names := map[string]string{"dawn": "黎明", "morning": "早晨", "noon": "中午", "afternoon": "下午", "evening": "傍晚", "late_night": "深夜", "deep_night": "凌晨"}
	if result := names[value]; result != "" {
		return result
	}
	return value
}
