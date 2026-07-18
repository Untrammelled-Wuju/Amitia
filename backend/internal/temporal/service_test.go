package temporal

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func temporalTestService(t *testing.T, now time.Time) (*Service, *SQLiteRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE characters (id TEXT PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO characters (id, name) VALUES (?, ?)", "character-a", "A").Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	service := NewServiceWithFlags(repo, NewFakeClock(now), FeatureFlags{TemporalCoreEnabled: true, RelationshipTimeEnabled: true})
	if err := service.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return service, repo
}

func saveTemporalProfile(t *testing.T, service *Service, ownerType, ownerID, zone, mode string) {
	t.Helper()
	_, err := service.SaveProfile(context.Background(), ownerType, ownerID, Profile{Timezone: zone, TimezoneMode: mode, Locale: "zh-CN", CalendarSystem: "gregorian", Hemisphere: "north", DaypartConfigJSON: "{}", QuietHoursJSON: `{"enabled":true,"start":"23:00","end":"07:00"}`, AwarenessLevel: 70, Confidence: 100, Enabled: true, HolidayAwareness: true, DaypartAwareness: true, AnniversaryAwareness: true, MemoryResonance: true, AllowSharedDateMention: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotResolvesUserAndCharacterAcrossDate(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 7, 18, 16, 30, 0, 0, time.UTC))
	saveTemporalProfile(t, service, OwnerUser, DefaultUserOwnerID, "America/Los_Angeles", TimezoneFixed)
	saveTemporalProfile(t, service, OwnerCharacter, "character-a", "Asia/Shanghai", TimezoneFixed)
	snapshot, err := service.ResolveSnapshot(context.Background(), SnapshotInput{CharacterID: "character-a", Channel: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.UserTime.LocalTime.Format("2006-01-02 15:04") != "2026-07-18 09:30" {
		t.Fatalf("unexpected user time %s", snapshot.UserTime.LocalTime)
	}
	if snapshot.CharacterTime.LocalTime.Format("2006-01-02 15:04") != "2026-07-19 00:30" {
		t.Fatalf("unexpected character time %s", snapshot.CharacterTime.LocalTime)
	}
	if !snapshot.Signals.TimezoneDiffers {
		t.Fatal("expected timezone difference")
	}
}

func TestSnapshotFollowUserTimezone(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	saveTemporalProfile(t, service, OwnerUser, DefaultUserOwnerID, "Asia/Tokyo", TimezoneFixed)
	saveTemporalProfile(t, service, OwnerCharacter, "character-a", "Europe/London", TimezoneFollowUser)
	snapshot, err := service.ResolveSnapshot(context.Background(), SnapshotInput{CharacterID: "character-a"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.UserTime.Timezone != snapshot.CharacterTime.Timezone || snapshot.Signals.TimezoneDiffers {
		t.Fatalf("follow user failed: %#v", snapshot)
	}
}

func TestPatchProfilePreservesOmittedBooleans(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	timezone := "Europe/Paris"
	profile, err := service.PatchProfile(context.Background(), OwnerUser, DefaultUserOwnerID, ProfilePatch{Timezone: &timezone})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Enabled || !profile.HolidayAwareness || !profile.MemoryResonance || profile.Timezone != timezone {
		t.Fatalf("patch erased omitted values %#v", profile)
	}
}

func TestPatchProfileCanDisableBoolean(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	disabled := false
	profile, err := service.PatchProfile(context.Background(), OwnerUser, DefaultUserOwnerID, ProfilePatch{MemoryResonance: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MemoryResonance {
		t.Fatalf("explicit false was ignored %#v", profile)
	}
}

func TestSnapshotUsesSessionDeviceTimezone(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC))
	snapshot, err := service.ResolveSnapshot(context.Background(), SnapshotInput{CharacterID: "character-a", DeviceTimezone: "America/Los_Angeles"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.UserTime.Timezone != "America/Los_Angeles" || snapshot.Signals.UserTimezoneSource != "device_session" || snapshot.Signals.UserTimezoneConfidence != 80 {
		t.Fatalf("device timezone not applied %#v", snapshot)
	}
	if strings.Contains(service.RenderSnapshot(snapshot), "用户时区未确认") {
		t.Fatalf("valid device timezone rendered as fallback %s", service.RenderSnapshot(snapshot))
	}
}

func TestSnapshotLabelsFallbackTimezoneAsUnconfirmed(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC))
	snapshot, err := service.ResolveSnapshot(context.Background(), SnapshotInput{CharacterID: "character-a"})
	if err != nil {
		t.Fatal(err)
	}
	rendered := service.RenderSnapshot(snapshot)
	if !strings.Contains(rendered, "系统参考时间") || !strings.Contains(rendered, "用户时区未确认") || strings.Contains(rendered, "用户当地时间") {
		t.Fatalf("fallback timezone was overstated %#v %s", snapshot.Signals, rendered)
	}
}

type relationshipTimeProbe struct{ calls int }

func (p *relationshipTimeProbe) Resolve(_ context.Context, _ SnapshotInput, _ time.Time) (*RelationshipTimeContext, error) {
	p.calls++
	return &RelationshipTimeContext{Version: RelationshipTimeVersion}, nil
}

func TestFeatureFlagCombinationsKeepRelationshipProviderIndependent(t *testing.T) {
	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	for _, combination := range []struct {
		name                             string
		core, relationship, wantProvider bool
	}{
		{"both-disabled", false, false, false},
		{"core-only", true, false, false},
		{"relationship-only", false, true, true},
		{"both-enabled", true, true, true},
	} {
		t.Run(combination.name, func(t *testing.T) {
			service, _ := temporalTestService(t, now)
			service.flags = FeatureFlags{TemporalCoreEnabled: combination.core, RelationshipTimeEnabled: combination.relationship}
			probe := &relationshipTimeProbe{}
			service.SetRelationshipTimeProvider(probe)
			if _, err := service.ResolveSnapshot(context.Background(), SnapshotInput{}); err != nil {
				t.Fatal(err)
			}
			if (probe.calls == 1) != combination.wantProvider {
				t.Fatalf("core=%v relationship=%v calls=%d", combination.core, combination.relationship, probe.calls)
			}
		})
	}
}

func TestDailyRecurrenceKeepsLocalTimeAcrossDSTStart(t *testing.T) {
	anchor := Anchor{TimeKind: "recurring", LocalDate: "2026-03-07", LocalTime: "09:00", Timezone: "America/New_York", RRule: "FREQ=DAILY"}
	next := nextAnchorOccurrence(anchor, time.Date(2026, 3, 7, 15, 0, 0, 0, time.UTC))
	if next == nil || !next.Equal(time.Date(2026, 3, 8, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected next occurrence %v", next)
	}
}

func TestPluginAnchorAlwaysRequiresConfirmation(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC))
	anchor, err := service.SaveAnchor(context.Background(), DefaultUserOwnerID, "", Anchor{Source: "plugin", SourceRef: "calendar-plugin", AnchorType: "custom", Title: "插件日期", TimeKind: "local_date", LocalDate: "2026-08-01", Timezone: "Asia/Shanghai", AllowProactiveMention: true})
	if err != nil {
		t.Fatal(err)
	}
	if anchor.Status != "candidate" || !anchor.RequiresConfirmation || anchor.AllowProactiveMention {
		t.Fatalf("plugin anchor bypassed confirmation: %#v", anchor)
	}
}

func TestDailyRecurrenceKeepsLocalTimeAcrossDSTEnd(t *testing.T) {
	anchor := Anchor{TimeKind: "recurring", LocalDate: "2026-10-31", LocalTime: "09:00", Timezone: "America/New_York", RRule: "FREQ=DAILY"}
	next := nextAnchorOccurrence(anchor, time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC))
	if next == nil || !next.Equal(time.Date(2026, 11, 1, 14, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected next occurrence %v", next)
	}
}

func TestAnnualLeapDayUsesFebruaryTwentyEight(t *testing.T) {
	anchor := Anchor{TimeKind: "annual_date", LocalDate: "02-29", LocalTime: "09:00", Timezone: "Asia/Shanghai"}
	next := nextAnchorOccurrence(anchor, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	location, _ := time.LoadLocation("Asia/Shanghai")
	if next == nil || next.In(location).Format("2006-01-02 15:04") != "2027-02-28 09:00" {
		t.Fatalf("unexpected leap occurrence %v", next)
	}
}

func TestQuietHoursUseUserCivilTime(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 7, 18, 16, 0, 0, 0, time.UTC))
	saveTemporalProfile(t, service, OwnerUser, DefaultUserOwnerID, "Asia/Shanghai", TimezoneFixed)
	snapshot, err := service.ResolveSnapshot(context.Background(), SnapshotInput{CharacterID: "character-a"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Signals.QuietHours || snapshot.Policy.AllowProactive {
		t.Fatalf("quiet hours gate failed %#v", snapshot.Policy)
	}
}

func TestCandidateAnchorDoesNotEnterSnapshot(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC))
	anchor, err := service.SaveAnchor(context.Background(), DefaultUserOwnerID, "character-a", Anchor{AnchorType: "birthday", Title: "候选生日", TimeKind: "annual_date", LocalDate: "07-18", Timezone: "Asia/Shanghai", Importance: 100, Confidence: 100, AllowPromptMention: true, RequiresConfirmation: true})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.ResolveSnapshot(context.Background(), SnapshotInput{CharacterID: "character-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.SalientAnchors) != 0 {
		t.Fatal("candidate anchor must not enter snapshot")
	}
	if _, err = service.ConfirmAnchor(context.Background(), DefaultUserOwnerID, "character-a", anchor.ID); err != nil {
		t.Fatal(err)
	}
	snapshot, err = service.ResolveSnapshot(context.Background(), SnapshotInput{CharacterID: "character-a"})
	if err != nil || len(snapshot.SalientAnchors) != 1 {
		t.Fatalf("confirmed anchor missing %#v %v", snapshot.SalientAnchors, err)
	}
}

func TestTemporalCoreDisabledUsesUTCFallback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	service := NewServiceWithFlags(repo, NewFakeClock(time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)), FeatureFlags{TemporalCoreEnabled: false})
	if err := service.InitSchema(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.ResolveSnapshot(context.Background(), SnapshotInput{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.UserTime.Timezone != "UTC" || snapshot.Policy.MentionTime != "none" || service.RenderSnapshot(snapshot) != "" {
		t.Fatalf("unexpected disabled fallback %#v", snapshot)
	}
}

func TestCustomDaypartAndSouthernSeason(t *testing.T) {
	location, _ := time.LoadLocation("Australia/Sydney")
	local := time.Date(2026, 7, 18, 6, 30, 0, 0, location)
	snapshot := civilSnapshot(local.UTC(), location, `{"morning":{"start":"06:00","end":"10:00"}}`, "south")
	if snapshot.Daypart != "morning" || snapshot.Season != "winter" {
		t.Fatalf("unexpected civil snapshot %#v", snapshot)
	}
}

func TestMemoryTemporalMetadataRoundTrip(t *testing.T) {
	_, repo := temporalTestService(t, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC))
	occurred := time.Date(2025, 7, 18, 1, 2, 3, 0, time.UTC)
	metadata := &MemoryTemporalMetadata{MemoryID: "memory-1", OccurredAtUTC: &occurred, Timezone: "Asia/Shanghai", TemporalPrecision: "exact", CreatedAtUTC: occurred, UpdatedAtUTC: occurred}
	if err := repo.SaveMemoryTemporalMetadata(metadata); err != nil {
		t.Fatal(err)
	}
	items, err := repo.GetMemoryTemporalMetadata([]string{"memory-1"})
	if err != nil || items["memory-1"].OccurredAtUTC == nil || !items["memory-1"].OccurredAtUTC.Equal(occurred) {
		t.Fatalf("metadata mismatch %#v %v", items, err)
	}
}

func TestMemoryRerankUsesOccurredAtAndBoundsBoost(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	service, _ := temporalTestService(t, now)
	occurred := time.Date(2025, 7, 18, 9, 0, 0, 0, time.UTC)
	if _, err := service.SaveMemoryTemporalMetadata(context.Background(), MemoryTemporalMetadata{MemoryID: "memory-old", OccurredAtUTC: &occurred, TemporalPrecision: "exact"}); err != nil {
		t.Fatal(err)
	}
	results, err := service.RerankMemoryScores(context.Background(), "去年今天发生了什么", []MemoryScoreCandidate{{MemoryID: "memory-old", BaseScore: 1, CreatedAt: now.Format(time.RFC3339), MemoryType: "worldbook"}})
	if err != nil {
		t.Fatal(err)
	}
	result := results["memory-old"]
	if result.ReferenceSource != "occurred_at_utc" {
		t.Fatalf("wrong reference %#v", result)
	}
	if result.TemporalBoost > .18 {
		t.Fatalf("boost exceeded bound %#v", result)
	}
	if result.FinalScore >= 1.18 || result.FinalScore <= 0 {
		t.Fatalf("event-time decay not applied %#v", result)
	}
}

func TestMemoryRerankAppliesValidityPenalty(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	service, _ := temporalTestService(t, now)
	expired := now.Add(-time.Hour)
	if _, err := service.SaveMemoryTemporalMetadata(context.Background(), MemoryTemporalMetadata{MemoryID: "memory-expired", ValidToUTC: &expired}); err != nil {
		t.Fatal(err)
	}
	results, err := service.RerankMemoryScores(context.Background(), "这个事实什么时候失效", []MemoryScoreCandidate{{MemoryID: "memory-expired", BaseScore: 1, CreatedAt: now.Format(time.RFC3339), MemoryType: "fact"}})
	if err != nil {
		t.Fatal(err)
	}
	if results["memory-expired"].ValidityPenalty != .65 {
		t.Fatalf("validity penalty missing %#v", results["memory-expired"])
	}
}

func TestMemoryRerankLeavesNonTemporalQueryUnchanged(t *testing.T) {
	service, _ := temporalTestService(t, time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC))
	results, err := service.RerankMemoryScores(context.Background(), "她喜欢什么颜色", []MemoryScoreCandidate{{MemoryID: "memory-old", BaseScore: .83, CreatedAt: "2020-01-01T00:00:00Z", MemoryType: "episodic"}})
	if err != nil {
		t.Fatal(err)
	}
	result := results["memory-old"]
	if result.FinalScore != .83 || result.TemporalBoost != 0 || result.ValidityPenalty != 1 || result.ReferenceSource != "none" {
		t.Fatalf("non-temporal query changed score %#v", result)
	}
}
