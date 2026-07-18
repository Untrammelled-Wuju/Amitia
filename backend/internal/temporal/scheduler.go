package temporal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ProactiveCandidate struct {
	EventID         string    `json:"eventId"`
	AnchorID        string    `json:"anchorId"`
	UserID          string    `json:"userId"`
	CharacterID     string    `json:"characterId"`
	Kind            string    `json:"kind"`
	Title           string    `json:"title"`
	OccurrenceAtUTC time.Time `json:"occurrenceAtUtc"`
	ExpiresAtUTC    time.Time `json:"expiresAtUtc"`
	IdempotencyKey  string    `json:"idempotencyKey"`
}

type CandidatePublisher interface {
	PublishTemporalCandidate(context.Context, ProactiveCandidate) error
}

type ProcessDueResult struct {
	Processed      int `json:"processed"`
	Emitted        int `json:"emitted"`
	SkippedExpired int `json:"skippedExpired"`
	Deduplicated   int `json:"deduplicated"`
}

func (s *Service) SetCandidatePublisher(publisher CandidatePublisher) {
	s.candidatePublisher = publisher
}

func (s *Service) ProcessDueAnchors(ctx context.Context, recovery bool) (ProcessDueResult, error) {
	now := utc(s.clock.Now())
	anchors, err := s.repo.ListDueAnchors(now, 200)
	if err != nil {
		return ProcessDueResult{}, err
	}
	result := ProcessDueResult{}
	for index := range anchors {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		anchor := &anchors[index]
		result.Processed++
		occurrence := anchor.NextOccurrenceAtUTC
		if occurrence == nil {
			continue
		}
		window := time.Duration(anchor.DurationSeconds) * time.Second
		if window <= 0 {
			window = time.Hour
		}
		expired := now.Sub(*occurrence) > window
		if recovery && expired {
			result.SkippedExpired++
			s.metrics.anchorRecoveryExpired.Add(1)
		} else {
			localDate := occurrence.In(mustLocation(anchor.Timezone)).Format("2006-01-02")
			key := fmt.Sprintf("temporal:%s:%s:%s:anchor_occurred", anchor.ID, localDate, anchor.CharacterID)
			event := &Event{ID: uuid.NewString(), EventType: "anchor_occurred", UserID: anchor.UserID, CharacterID: anchor.CharacterID, AnchorID: anchor.ID, OccurredAtUTC: utc(*occurrence), EffectiveLocalDate: localDate, Timezone: anchor.Timezone, Salience: float64(anchor.Importance) / 100, Source: "temporal-scheduler", IdempotencyKey: key, PayloadJSON: anchor.PayloadJSON, CreatedAtUTC: now}
			created, createErr := s.repo.CreateEvent(event)
			if createErr != nil {
				return result, createErr
			}
			if !created {
				result.Deduplicated++
				s.metrics.anchorDeduplicated.Add(1)
			} else {
				result.Emitted++
				s.metrics.anchorEvents.Add(1)
				if anchor.AllowProactiveMention && s.candidatePublisher != nil {
					if publishErr := s.candidatePublisher.PublishTemporalCandidate(ctx, ProactiveCandidate{EventID: event.ID, AnchorID: anchor.ID, UserID: anchor.UserID, CharacterID: anchor.CharacterID, Kind: anchor.AnchorType, Title: anchor.Title, OccurrenceAtUTC: utc(*occurrence), ExpiresAtUTC: utc(occurrence.Add(window)), IdempotencyKey: key}); publishErr != nil {
						s.metrics.proactiveCandidateErrors.Add(1)
					} else {
						s.metrics.proactiveCandidates.Add(1)
					}
				}
			}
		}
		previous := utc(*occurrence)
		anchor.LastOccurrenceAtUTC = &previous
		advanceFrom := previous.Add(time.Second)
		if recovery && expired {
			advanceFrom = now
		}
		anchor.NextOccurrenceAtUTC = nextOccurrenceAfter(*anchor, advanceFrom)
		anchor.UpdatedAtUTC = now
		if anchor.NextOccurrenceAtUTC == nil && (anchor.TimeKind == "instant" || anchor.TimeKind == "local_date" || anchor.TimeKind == "local_datetime" || anchor.TimeKind == "range") {
			anchor.Status = "archived"
		}
		if err := s.repo.SaveAnchor(anchor); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *Service) RecomputeAnchorOccurrences(ctx context.Context) (int, error) {
	anchors, err := s.repo.ListAllAnchors("active", 5000)
	if err != nil {
		return 0, err
	}
	now := utc(s.clock.Now())
	count := 0
	for index := range anchors {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		anchors[index].NextOccurrenceAtUTC = nextOccurrenceAfter(anchors[index], now)
		anchors[index].UpdatedAtUTC = now
		if err := s.repo.SaveAnchor(&anchors[index]); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func nextOccurrenceAfter(anchor Anchor, from time.Time) *time.Time {
	if anchor.TimeKind == "instant" || anchor.TimeKind == "range" || anchor.TimeKind == "local_date" || anchor.TimeKind == "local_datetime" {
		candidate := nextAnchorOccurrence(anchor, from)
		if candidate != nil && candidate.Before(from) {
			return nil
		}
		return candidate
	}
	return nextAnchorOccurrence(anchor, from)
}

func mustLocation(name string) *time.Location {
	location, err := loadLocation(name)
	if err != nil {
		return time.UTC
	}
	return location
}

type Scheduler struct {
	service        *Service
	cancel         context.CancelFunc
	done           chan struct{}
	mu             sync.Mutex
	lastLocalDates map[string]string
}

func NewScheduler(service *Service) *Scheduler {
	return &Scheduler{service: service, lastLocalDates: map[string]string{}}
}

func (s *Scheduler) Start(parent context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil || s.service == nil || !s.service.flags.TemporalCoreEnabled {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	s.captureLocalDates()
	_, _ = s.service.ProcessDueAnchors(ctx, true)
	go s.run(ctx, done)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *Scheduler) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_, _ = s.service.ProcessDueAnchors(ctx, false)
			s.detectLocalDayChanges(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) captureLocalDates() {
	profiles, err := s.service.repo.ListProfiles()
	if err != nil {
		return
	}
	now := s.service.clock.Now()
	for _, profile := range profiles {
		location, err := loadLocation(profile.Timezone)
		if err == nil {
			s.lastLocalDates[profile.OwnerType+":"+profile.OwnerID] = now.In(location).Format("2006-01-02")
		}
	}
}

func (s *Scheduler) detectLocalDayChanges(ctx context.Context) {
	profiles, err := s.service.repo.ListProfiles()
	if err != nil {
		return
	}
	now := utc(s.service.clock.Now())
	for _, profile := range profiles {
		location, loadErr := loadLocation(profile.Timezone)
		if loadErr != nil {
			continue
		}
		key := profile.OwnerType + ":" + profile.OwnerID
		date := now.In(location).Format("2006-01-02")
		previous := s.lastLocalDates[key]
		s.lastLocalDates[key] = date
		if previous == "" || previous == date {
			continue
		}
		userID, characterID := profile.OwnerID, ""
		if profile.OwnerType == OwnerCharacter {
			userID = DefaultUserOwnerID
			characterID = profile.OwnerID
		}
		_, _ = s.service.repo.CreateEvent(&Event{ID: uuid.NewString(), EventType: "day_changed", UserID: userID, CharacterID: characterID, OccurredAtUTC: now, EffectiveLocalDate: date, Timezone: profile.Timezone, Source: "temporal-scheduler", IdempotencyKey: "temporal:day_changed:" + key + ":" + date, CreatedAtUTC: now})
		if ctx.Err() != nil {
			return
		}
	}
}
