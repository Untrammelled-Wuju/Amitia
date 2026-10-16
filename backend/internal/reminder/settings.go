package reminder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

type SettingStore interface {
	Read(context.Context, string) (string, error)
	Write(context.Context, string, string) error
}

func settingKey(caller Caller) string {
	hash := sha256.Sum256([]byte(caller.SpaceID + "\x00" + caller.DeviceID))
	return "reminder.v2.cleanup." + hex.EncodeToString(hash[:])
}

func (s *Service) GetCleanupDays(ctx context.Context, caller Caller) (string, error) {
	if caller.SpaceID == "" || caller.DeviceID == "" {
		return "", ErrForbidden
	}
	if s.deps.Settings == nil {
		return "", ErrUnavailable
	}
	value, err := s.deps.Settings.Read(ctx, settingKey(caller))
	if err != nil {
		return "", err
	}
	if value == "" {
		value = "0"
	}
	return value, nil
}

func (s *Service) SetCleanupDays(ctx context.Context, caller Caller, value string) error {
	if caller.SpaceID == "" || caller.DeviceID == "" {
		return ErrForbidden
	}
	days, err := strconv.Atoi(value)
	if err != nil || days < 0 || days > 3650 {
		return errors.New("清理天数须为 0 至 3650")
	}
	if s.deps.Settings == nil {
		return ErrUnavailable
	}
	if err := s.deps.Settings.Write(ctx, settingKey(caller), strconv.Itoa(days)); err != nil {
		return err
	}
	return s.cleanupHistory(ctx, caller)
}

func (s *Service) historyCutoff(ctx context.Context, caller Caller) (*time.Time, error) {
	if s.deps.Settings == nil {
		return nil, nil
	}
	value, err := s.GetCleanupDays(ctx, caller)
	if err != nil {
		return nil, err
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	if days == 0 {
		return nil, nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	items, err := s.list(ctx, caller, ProactiveTask, true)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Policy == nil {
			continue
		}
		gap := ProactiveMinimumInterval(item.Policy)
		if item.Policy.UnrepliedSlowdownEnabled && item.Policy.UnrepliedCooldownMultiplier > 1 {
			gap = ScaleCooldown(gap, item.Policy.UnrepliedCooldownMultiplier)
		}
		policyCutoff := time.Now().Add(-gap)
		if policyCutoff.Before(cutoff) {
			cutoff = policyCutoff
		}
	}
	return &cutoff, nil
}

func (s *Service) CleanupHistory(ctx context.Context, caller Caller) error {
	if !s.Ready() {
		return ErrUnavailable
	}
	return s.cleanupHistory(ctx, caller)
}

func (s *Service) CleanupAllHistory(ctx context.Context) error {
	if !s.Ready() {
		return ErrUnavailable
	}
	defs, err := s.deps.Scheduler.ListAllSchedules(ctx)
	if err != nil {
		return err
	}
	callers := map[string]Caller{}
	for _, def := range defs {
		item, err := decode(def)
		if err != nil || item.OwnerID != s.deps.OwnerID {
			continue
		}
		caller := Caller{SpaceID: item.Caller.SpaceID, DeviceID: item.Caller.DeviceID}
		callers[settingKey(caller)] = caller
	}
	for _, caller := range callers {
		if err := s.cleanupHistory(ctx, caller); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) cleanupHistory(ctx context.Context, caller Caller) error {
	cutoff, err := s.historyCutoff(ctx, caller)
	if err != nil || cutoff == nil {
		return err
	}
	items, err := s.list(ctx, caller, "", true)
	if err != nil {
		return err
	}
	for index := range items {
		removeIDs := map[string]bool{}
		entries, err := s.historyFor(ctx, &items[index], -1)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.UpdatedAt.Before(*cutoff) && (entry.State == "completed" || entry.State == "cancelled" || entry.State == "expired" || entry.State == "skipped") {
				if entry.Archived {
					removeIDs[entry.TriggerID] = true
					continue
				}
				if err := s.deps.Scheduler.CleanTriggerRunHistory(ctx, entry.TriggerID); err != nil {
					return err
				}
			}
		}
		if len(removeIDs) > 0 {
			if err := s.deps.Scheduler.PruneHostArchiveHistory(ctx, items[index].ID, items[index].Generation, removeIDs); err != nil {
				return err
			}
		}
	}
	return nil
}
