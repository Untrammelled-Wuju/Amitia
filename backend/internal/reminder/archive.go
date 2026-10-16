package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/system/dataportability"
)

const TaskArchiveFormat = "amitia-owned-tasks-v1"

type TaskArchive struct {
	Format   string         `json:"format"`
	OwnerID  string         `json:"ownerId"`
	Exporter Caller         `json:"exporter"`
	Tasks    []ArchivedTask `json:"tasks"`
}

type ArchivedTask struct {
	ID                   string                `json:"id"`
	OwnerID              string                `json:"ownerId"`
	CreatedBy            Caller                `json:"createdBy"`
	Origin               Caller                `json:"origin"`
	Source               string                `json:"source"`
	State                string                `json:"state"`
	CreatedAt            string                `json:"createdAt"`
	ScheduledOccurrences int                   `json:"scheduledOccurrences"`
	Configuration        CreateReminderRequest `json:"configuration"`
	History              []TriggerHistory      `json:"history"`
}

func (s *Service) ExportTaskArchive(ctx context.Context, caller Caller, characterID string) (TaskArchive, error) {
	if caller.PluginID != "" {
		return TaskArchive{}, ErrForbidden
	}
	items, err := s.list(ctx, caller, "", true)
	if err != nil {
		return TaskArchive{}, err
	}
	archive := TaskArchive{Format: TaskArchiveFormat, OwnerID: s.deps.OwnerID, Exporter: Caller{SpaceID: caller.SpaceID, DeviceID: caller.DeviceID}, Tasks: []ArchivedTask{}}
	for _, item := range items {
		if characterID != "" && item.CharacterID != characterID {
			continue
		}
		history, err := s.historyFor(ctx, &item, 0)
		if err != nil {
			return TaskArchive{}, err
		}
		count, err := s.deps.Scheduler.CountScheduledTriggers(ctx, item.ID)
		if err != nil {
			return TaskArchive{}, err
		}
		notify, enabled := item.SystemNotification, item.Enabled == 1
		request := CreateReminderRequest{Title: item.Title, Content: item.Content, CharacterID: item.CharacterID, ConversationID: item.ConversationID, TargetDeviceID: item.TargetDeviceID, Channel: item.Channel, Channels: item.Channels, ChannelTargets: item.ChannelTargets, SystemNotification: &notify, RemindAt: item.AnchorAt, Recurrence: item.Recurrence, Timezone: item.Timezone, EndAt: item.EndAt, MaxOccurrences: item.MaxOccurrences, Enabled: &enabled, Kind: item.Kind, Policy: item.Policy, RequestID: item.RequestID}
		archive.Tasks = append(archive.Tasks, ArchivedTask{ID: item.ID, OwnerID: item.OwnerID, CreatedBy: item.CreatedBy, Origin: item.Caller, Source: item.Source, State: item.State, CreatedAt: item.CreatedAt, Configuration: request, ScheduledOccurrences: count, History: history})
	}
	return archive, nil
}

func validateTaskArchive(archive TaskArchive) error {
	if archive.Format != TaskArchiveFormat || archive.OwnerID == "" || archive.Exporter.SpaceID == "" || archive.Exporter.DeviceID == "" || archive.Exporter.PluginID != "" || len(archive.Tasks) > 32768 {
		return errors.New("统一任务备份格式无效")
	}
	seen := map[string]bool{}
	for _, task := range archive.Tasks {
		if task.ID == "" || seen[task.ID] || task.OwnerID != archive.OwnerID || task.Origin.SpaceID != archive.Exporter.SpaceID || task.Origin.DeviceID == "" || task.ScheduledOccurrences < 0 || task.Configuration.CharacterID == "" || task.Configuration.RequestID == "" || len(task.History) > 100000 {
			return errors.New("统一任务备份记录无效")
		}
		seen[task.ID] = true
		if task.Configuration.Kind != UserReminder && task.Configuration.Kind != ProactiveTask {
			return errors.New("统一任务备份类型无效")
		}
		if (task.Origin.PluginID == "") != (task.Origin.ModuleID == "") || (task.Source == "plugin") != (task.Origin.PluginID != "") {
			return errors.New("统一任务备份插件来源不一致")
		}
		historyIDs := map[string]bool{}
		for _, entry := range task.History {
			if entry.ID == "" || historyIDs[entry.ID] || entry.ReminderID != task.ID || entry.TriggerType != task.Configuration.Kind || entry.AttemptCount < 0 || entry.CreatedAt.IsZero() || len(entry.Result) > 1<<20 || len(entry.Result) > 0 && !json.Valid(entry.Result) {
				return errors.New("统一任务备份历史无效")
			}
			historyIDs[entry.ID] = true
		}
	}
	return nil
}

func (s *Service) RestoreTaskArchive(ctx context.Context, caller Caller, req dataportability.ImportRequest, archive TaskArchive) error {
	if !s.Ready() {
		return ErrUnavailable
	}
	if err := validateTaskArchive(archive); err != nil {
		return err
	}
	if caller.PluginID != "" || caller.SpaceID != archive.Exporter.SpaceID || caller.DeviceID != archive.Exporter.DeviceID || archive.OwnerID != s.deps.OwnerID {
		return ErrForbidden
	}
	type prepared struct {
		item       *Reminder
		def        *schedule.ScheduleContributionDefinition
		generation int64
		cancelled  bool
	}
	entries := make([]prepared, 0, len(archive.Tasks))
	for _, archived := range archive.Tasks {
		request := archived.Configuration
		id := archived.ID
		if req.IdentityMap != nil {
			if mapped, ok := req.IdentityMap.GetCharacter(request.CharacterID); ok {
				request.CharacterID = mapped
			}
			if mapped, ok := req.IdentityMap.GetConversation(request.ConversationID); ok {
				request.ConversationID = mapped
			}
		}
		if request.CharacterID != archived.Configuration.CharacterID || request.ConversationID != archived.Configuration.ConversationID {
			id = "reminder-restored-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(archived.ID+"\x00"+request.CharacterID+"\x00"+request.ConversationID)).String()
			request.RequestID = "restored-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(id)).String()
		}
		origin := Caller{SpaceID: caller.SpaceID, DeviceID: caller.DeviceID, PluginID: archived.Origin.PluginID, ModuleID: archived.Origin.ModuleID}
		if origin.PluginID != "" && s.deps.PluginActive != nil {
			if err := s.deps.PluginActive(ctx, origin.PluginID, origin.ModuleID); err != nil {
				return err
			}
		}
		disabled := false
		request.Enabled = &disabled
		item, def, err := s.build(ctx, origin, request, id, archived.Source, true)
		if err != nil {
			return err
		}
		item.CreatedBy, item.CreatedAt = archived.CreatedBy, archived.CreatedAt
		item.ArchiveHistory = append([]TriggerHistory(nil), archived.History...)
		for index := range item.ArchiveHistory {
			entry := &item.ArchiveHistory[index]
			entry.ReminderID, entry.Archived = id, true
			if strings.Contains("waiting leased running triggering due retry_wait", entry.State) && entry.State != "" {
				entry.State = "unknown"
				entry.Reason = "备份中的未结束执行仅保留为历史，恢复不会重新触发"
			}
		}
		def.Trigger.RestoredOccurrences = archived.ScheduledOccurrences
		var generation int64
		if old, state, err := s.deps.Scheduler.GetSchedule(ctx, id); err == nil {
			stored, err := decode(old)
			if err != nil || s.checkOwnership(caller, stored) != nil || stored.Caller.PluginID != origin.PluginID || stored.Caller.ModuleID != origin.ModuleID {
				return ErrForbidden
			}
			if req.CharacterPolicy == dataportability.CollisionSkip {
				continue
			}
			if state.Status == schedule.DefinitionStatusUninstalled {
				return errors.New("已取消任务不能通过备份恢复重新启用")
			}
			generation = state.Generation
		} else if !errors.Is(err, schedule.ErrScheduleNotFound) {
			return err
		}
		def.Target.InputTemplate, err = json.Marshal(struct {
			*Reminder
			CreateHash string `json:"createHash"`
		}{item, requestHash(origin, request)})
		if err != nil {
			return err
		}
		entries = append(entries, prepared{item: item, def: def, generation: generation, cancelled: archived.State == string(schedule.DefinitionStatusUninstalled)})
	}
	return s.deps.Scheduler.WithArchiveTransaction(ctx, func(ctx context.Context) error {
		for _, entry := range entries {
			var err error
			if entry.generation == 0 {
				err = s.deps.Scheduler.InstallArchivedHostDefinition(ctx, entry.def)
			} else {
				err = s.deps.Scheduler.RestoreHostDefinition(ctx, entry.item.ID, entry.generation, entry.def)
			}
			if err != nil {
				return err
			}
			if entry.cancelled {
				_, state, err := s.deps.Scheduler.GetSchedule(ctx, entry.item.ID)
				if err != nil {
					return err
				}
				if err := s.deps.Scheduler.CancelHostDefinition(ctx, entry.item.ID, state.Generation); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func mergeArchivedHistory(item *Reminder, history []TriggerHistory) []TriggerHistory {
	seen := map[string]bool{}
	for _, entry := range history {
		seen[entry.ID] = true
	}
	for _, entry := range item.ArchiveHistory {
		if !seen[entry.ID] {
			entry.Archived = true
			history = append(history, entry)
			seen[entry.ID] = true
		}
	}
	sort.Slice(history, func(i, j int) bool { return history[i].CreatedAt.After(history[j].CreatedAt) })
	return history
}
