package reminder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

const UserReminder = "user_reminder"
const ProactiveTask = "proactive_task"
const TargetID = "builtin.reminder.fire"

var ErrUnavailable = errors.New("提醒服务未就绪")
var ErrForbidden = errors.New("无权访问该任务")

type Dependencies struct {
	Scheduler     *schedule.ScheduleService
	OwnerID       string
	Mode          string
	Authorize     func(context.Context, Caller, *Reminder) (json.RawMessage, error)
	Deliver       func(context.Context, *Reminder, string) (DeliveryResult, error)
	PluginActive  func(context.Context, string, string) error
	CheckPolicy   func(context.Context, *Reminder, []TriggerHistory) (string, error)
	Settings      SettingStore
	ResolveTarget func(context.Context, TargetResult) (TargetResult, error)
	Channels      func(context.Context, Caller) ([]string, error)
}

type pendingCreate struct {
	done chan struct{}
	item *Reminder
	err  error
}

type Service struct {
	deps        Dependencies
	createMu    sync.Mutex
	createCalls map[string]*pendingCreate
}

func NewService(deps Dependencies) *Service {
	s := &Service{deps: deps, createCalls: make(map[string]*pendingCreate)}
	if deps.Scheduler != nil {
		deps.Scheduler.GetExecutor().RegisterTargetAdapter(schedule.NewHostWorkloadAdapter(s.Execute))
	}
	return s
}

func (s *Service) Ready() bool {
	return s != nil && s.deps.Scheduler != nil && s.deps.Authorize != nil && s.deps.Deliver != nil && s.deps.OwnerID != ""
}
func (s *Service) Running() bool { return s.Ready() && s.deps.Scheduler.IsRunning() }

func (s *Service) authorize(ctx context.Context, caller Caller, item *Reminder) error {
	if !s.Ready() {
		return ErrUnavailable
	}
	if err := s.checkOwnership(caller, item); err != nil {
		return err
	}
	if item.Caller.PluginID != "" && s.deps.PluginActive != nil {
		if err := s.deps.PluginActive(ctx, item.Caller.PluginID, item.Caller.ModuleID); err != nil {
			return err
		}
	}
	_, err := s.deps.Authorize(ctx, caller, item)
	return err
}

func (s *Service) checkOwnership(caller Caller, item *Reminder) error {
	if caller.RoleScope != "" && caller.RoleScope != item.CharacterID || caller.ConversationScope != "" && caller.ConversationScope != item.ConversationID {
		return ErrForbidden
	}
	if caller.SpaceID == "" || caller.DeviceID == "" || item.Caller.SpaceID != caller.SpaceID || item.OwnerID != s.deps.OwnerID {
		return ErrForbidden
	}
	if caller.PluginID != "" && (item.Caller.PluginID != caller.PluginID || item.Caller.ModuleID != caller.ModuleID || item.Caller.DeviceID != caller.DeviceID) {
		return ErrForbidden
	}
	return nil
}

func decode(def *schedule.ScheduleContributionDefinition) (*Reminder, error) {
	if def == nil || def.Target.Type != schedule.TargetTypeHostWorkload || def.Target.TargetID != TargetID {
		return nil, schedule.ErrScheduleNotFound
	}
	var item Reminder
	if err := json.Unmarshal(def.Target.InputTemplate, &item); err != nil {
		return nil, err
	}
	if item.ID != def.ScheduleID || (item.Kind != UserReminder && item.Kind != ProactiveTask) {
		return nil, ErrForbidden
	}
	return &item, nil
}

func (s *Service) Find(ctx context.Context, caller Caller, id string) (*Reminder, error) {
	if !s.Ready() {
		return nil, ErrUnavailable
	}
	def, state, err := s.deps.Scheduler.GetSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	item, err := decode(def)
	if err != nil {
		return nil, err
	}
	if err := s.checkOwnership(caller, item); err != nil {
		return nil, err
	}
	if item.Caller.DeviceID != caller.DeviceID {
		if _, err := s.deps.Authorize(ctx, caller, item); err != nil {
			return nil, ErrForbidden
		}
	}
	if state == nil {
		return nil, ErrUnavailable
	}
	item.Generation, item.State, item.Paused = state.Generation, string(state.Status), state.Paused
	item.Enabled = 0
	if state.Enabled && !state.Paused && state.Status == schedule.DefinitionStatusEnabled {
		item.Enabled = 1
	}
	item.RemindAt = ""
	if state.NextScheduledAt != nil {
		item.RemindAt = state.NextScheduledAt.UTC().Format(time.RFC3339Nano)
		if loc, err := time.LoadLocation(item.Timezone); err == nil {
			item.LocalRemindAt = state.NextScheduledAt.In(loc).Format("2006-01-02 15:04:05")
		}
	}
	if state.LastTriggeredAt != nil {
		value := state.LastTriggeredAt.UTC().Format(time.RFC3339Nano)
		item.LastTriggeredAt = &value
	}
	item.UpdatedAt = state.UpdatedAt.UTC().Format(time.RFC3339Nano)
	item.Saved = true
	return item, nil
}

func (s *Service) List(ctx context.Context, caller Caller, kind string) ([]Reminder, error) {
	return s.list(ctx, caller, kind, false)
}

func (s *Service) list(ctx context.Context, caller Caller, kind string, cancelled bool) ([]Reminder, error) {
	if !s.Ready() {
		return nil, ErrUnavailable
	}
	if caller.SpaceID == "" || caller.DeviceID == "" {
		return nil, ErrForbidden
	}
	if kind != "" && kind != UserReminder && kind != ProactiveTask {
		return nil, errors.New("任务类型无效")
	}
	defs, err := s.deps.Scheduler.ListAllSchedules(ctx)
	if err != nil {
		return nil, err
	}
	items := []Reminder{}
	for _, def := range defs {
		item, err := decode(def)
		if err != nil || (kind != "" && item.Kind != kind) || item.Caller.SpaceID != caller.SpaceID {
			continue
		}
		if caller.RoleScope != "" && caller.RoleScope != item.CharacterID || caller.ConversationScope != "" && caller.ConversationScope != item.ConversationID {
			continue
		}
		if caller.PluginID != "" && (item.Caller.PluginID != caller.PluginID || item.Caller.ModuleID != caller.ModuleID) {
			continue
		}
		visible, err := s.Find(ctx, caller, item.ID)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				continue
			}
			return nil, err
		}
		if cancelled || visible.State != string(schedule.DefinitionStatusUninstalled) {
			items = append(items, *visible)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RemindAt < items[j].RemindAt })
	return items, nil
}

func requestHash(caller Caller, request CreateReminderRequest) (string, error) {
	encoded, err := json.Marshal(struct {
		Caller  Caller
		Request CreateReminderRequest
	}{caller, request})
	if err != nil {
		return "", fmt.Errorf("计算任务请求摘要失败: %w", err)
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func normalizeCreateRequest(request CreateReminderRequest) CreateReminderRequest {
	if request.Kind == "" {
		request.Kind = UserReminder
	}
	if request.Timezone == "" {
		request.Timezone = "Asia/Shanghai"
	}
	return request
}

func (s *Service) existingCreate(ctx context.Context, caller Caller, id, hash string) (*Reminder, bool, error) {
	if existing, _, err := s.deps.Scheduler.GetSchedule(ctx, id); err == nil {
		item, err := decode(existing)
		if err != nil {
			return nil, false, err
		}
		var stored struct {
			CreateHash string `json:"createHash"`
		}
		if err := json.Unmarshal(existing.Target.InputTemplate, &stored); err != nil {
			return nil, false, fmt.Errorf("读取任务幂等摘要失败: %w", err)
		}
		if stored.CreateHash != hash {
			return nil, true, schedule.ErrIdempotencyConflict
		}
		result, err := s.Find(ctx, caller, item.ID)
		return result, true, err
	} else if !errors.Is(err, schedule.ErrScheduleNotFound) {
		return nil, false, err
	}
	return nil, false, nil
}

func (s *Service) Create(ctx context.Context, caller Caller, request CreateReminderRequest, source string) (*Reminder, error) {
	if !s.Ready() {
		return nil, ErrUnavailable
	}
	if caller.SpaceID == "" || caller.DeviceID == "" {
		return nil, ErrForbidden
	}
	if request.RequestID == "" || len(request.RequestID) > 128 {
		return nil, errors.New("创建任务必须提供有效请求编号")
	}
	request = normalizeCreateRequest(request)
	hash, err := requestHash(caller, request)
	if err != nil {
		return nil, err
	}
	id := "reminder-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(caller.SpaceID+"\x00"+caller.DeviceID+"\x00"+caller.PluginID+"\x00"+caller.ModuleID+"\x00"+request.RequestID)).String()
	key := id + "\x00" + hash
	s.createMu.Lock()
	if pending := s.createCalls[key]; pending != nil {
		s.createMu.Unlock()
		select {
		case <-pending.done:
			return pending.item, pending.err
		case <-ctx.Done():
			return nil, fmt.Errorf("等待任务创建结果失败: %w", ctx.Err())
		}
	}
	pending := &pendingCreate{done: make(chan struct{})}
	s.createCalls[key] = pending
	s.createMu.Unlock()
	item, createErr := s.createOnce(ctx, caller, request, id, hash, source)
	pending.item = item
	pending.err = createErr
	s.createMu.Lock()
	delete(s.createCalls, key)
	s.createMu.Unlock()
	close(pending.done)
	return item, createErr
}

func (s *Service) createOnce(ctx context.Context, caller Caller, request CreateReminderRequest, id, hash, source string) (*Reminder, error) {
	if item, found, err := s.existingCreate(ctx, caller, id, hash); found {
		return item, err
	}
	item, def, err := s.build(ctx, caller, request, id, source, false)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(struct {
		*Reminder
		CreateHash string `json:"createHash"`
	}{item, hash})
	if err != nil {
		return nil, err
	}
	def.Target.InputTemplate = encoded
	if err := s.deps.Scheduler.InstallHostDefinition(ctx, def); err != nil {
		if errors.Is(err, schedule.ErrIdempotencyConflict) {
			return s.waitForCreateResult(ctx, caller, id, hash, err)
		}
		return nil, err
	}
	return s.Find(ctx, caller, id)
}

func (s *Service) waitForCreateResult(ctx context.Context, caller Caller, id, hash string, conflict error) (*Reminder, error) {
	for attempt := 0; attempt < 50; attempt++ {
		item, found, err := s.existingCreate(ctx, caller, id, hash)
		if found {
			return item, err
		}
		if err != nil {
			return nil, err
		}
		if attempt == 49 {
			break
		}
		timer := time.NewTimer(5 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, fmt.Errorf("等待任务创建结果失败: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("任务调度定义安装冲突且未读取到幂等结果: %w", conflict)
}

func (s *Service) build(ctx context.Context, caller Caller, request CreateReminderRequest, id, source string, updating bool) (*Reminder, *schedule.ScheduleContributionDefinition, error) {
	if strings.TrimSpace(request.Title) == "" || len(request.Title) > 512 || len(request.Content) > 128<<10 || strings.TrimSpace(request.CharacterID) == "" {
		return nil, nil, errors.New("标题、所属角色或内容无效")
	}
	if request.Kind == "" {
		request.Kind = UserReminder
	}
	if request.Kind != UserReminder && request.Kind != ProactiveTask {
		return nil, nil, errors.New("任务类型无效")
	}
	if request.Timezone == "" {
		request.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(request.Timezone)
	if err != nil {
		return nil, nil, errors.New("时区必须为有效 IANA 名称")
	}
	anchor, err := parseTime(request.RemindAt, loc)
	if err != nil || (!updating && !anchor.After(time.Now())) {
		return nil, nil, errors.New("提醒时间必须为未来的有效时间")
	}
	wallAnchor := anchor.In(loc).Format("2006-01-02 15:04:05")
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if wall, err := time.Parse(layout, request.RemindAt); err == nil {
			wallAnchor = wall.Format("2006-01-02 15:04:05")
			break
		}
	}
	rule := request.Recurrence
	if rule.Type == "" {
		rule.Type = request.RepeatRule
	}
	if rule.Type == "" {
		rule.Type = "none"
	}
	trigger := schedule.ScheduleTriggerDefinition{MaxOccurrences: request.MaxOccurrences}
	if request.MaxOccurrences < 0 {
		return nil, nil, errors.New("结束次数不能小于零")
	}
	switch rule.Type {
	case "none":
		trigger.Type = schedule.TriggerTypeOneShot
		trigger.OneShot = &schedule.OneShotTriggerDefinition{RunAt: anchor}
	case "daily", "weekly", "monthly", "weekdays":
		trigger.Type = schedule.TriggerTypeCalendar
		trigger.Calendar = &schedule.CalendarTriggerDefinition{Frequency: rule.Type, AnchorAt: anchor, Weekdays: rule.Weekdays, WallTime: wallAnchor[11:]}
	case "interval":
		if rule.IntervalSeconds < 60 || rule.IntervalSeconds > 365*86400 {
			return nil, nil, errors.New("间隔须为 60 秒至 365 天")
		}
		trigger.Type = schedule.TriggerTypeInterval
		trigger.Interval = &schedule.IntervalTriggerDefinition{AnchorAt: anchor, Interval: time.Duration(rule.IntervalSeconds) * time.Second}
	case "cron":
		if request.Kind != ProactiveTask {
			return nil, nil, errors.New("Cron 仅用于主动任务规则")
		}
		trigger.Type = schedule.TriggerTypeCron
		trigger.Cron = &schedule.CronTriggerDefinition{Expression: rule.Cron}
	default:
		return nil, nil, errors.New("重复规则无效")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item := &Reminder{ID: id, Caller: caller, OwnerID: s.deps.OwnerID, ExecutionMode: s.deps.Mode, Kind: request.Kind, Source: source, Title: strings.TrimSpace(request.Title), Content: strings.TrimSpace(request.Content), ConversationID: request.ConversationID, CharacterID: request.CharacterID, TargetDeviceID: request.TargetDeviceID, AnchorAt: anchor.UTC().Format(time.RFC3339Nano), RemindAt: anchor.UTC().Format(time.RFC3339Nano), RepeatRule: rule.Type, Recurrence: rule, Timezone: request.Timezone, EndAt: request.EndAt, MaxOccurrences: request.MaxOccurrences, Channel: strings.TrimSpace(request.Channel), Channels: request.Channels, SystemNotification: true, Policy: request.Policy, RequestID: request.RequestID, CreatedAt: now, UpdatedAt: now, Enabled: 1}
	if item.Channel == "" || item.Channel == "all" {
		item.Channel = "web"
	}
	item.CreatedBy = caller
	item.LocalAnchorAt = wallAnchor
	item.ChannelTargets = request.ChannelTargets
	if request.SystemNotification != nil {
		item.SystemNotification = *request.SystemNotification
	}
	if request.Enabled != nil && !*request.Enabled {
		item.Enabled = 0
	}
	if item.Kind == ProactiveTask && item.Policy == nil {
		return nil, nil, errors.New("主动任务必须提供执行策略")
	}
	if item.Policy != nil {
		policy := item.Policy
		if policy.ActiveLevel < 0 || policy.ActiveLevel > 100 || policy.MinInterval < 0 || policy.MinInterval > 525600 || policy.MaxPerDay < 0 || policy.MaxPerDay > 100000 || policy.MaxDailyCalls < 0 || policy.MaxDailyCalls > 100000 || policy.RuleMaxPerDay < 0 || policy.RuleMaxPerDay > 100000 || policy.UnrepliedSlowdownAfter < 0 || policy.UnrepliedSlowdownAfter > 100000 || policy.UnrepliedCooldownMultiplier < 0 || policy.UnrepliedCooldownMultiplier > 100 {
			return nil, nil, errors.New("主动任务频率策略超出允许范围")
		}
		for _, value := range []string{policy.QuietStart, policy.QuietEnd, policy.RuleQuietStart, policy.RuleQuietEnd} {
			if value == "" {
				continue
			}
			if _, err := time.Parse("15:04", value); err != nil {
				return nil, nil, errors.New("安静时段须使用 HH:mm 格式")
			}
		}
	}
	if caller.PluginID != "" && s.deps.PluginActive != nil {
		if err := s.deps.PluginActive(ctx, caller.PluginID, caller.ModuleID); err != nil {
			return nil, nil, err
		}
	}
	binding, err := s.deps.Authorize(ctx, caller, item)
	if err != nil {
		return nil, nil, err
	}
	item.Binding = binding
	def := &schedule.ScheduleContributionDefinition{ScheduleID: id, ContributionID: id, ExtensionID: caller.PluginID, ModuleID: caller.ModuleID, Name: item.Title, Description: item.Kind, Trigger: trigger, Target: schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeHostWorkload, TargetID: TargetID, IdempotencyMode: schedule.IdempotencyModeConditionallyIdem}, Timezone: item.Timezone, EnabledByDefault: item.Enabled == 1, MisfirePolicy: schedule.ScheduleMisfirePolicy{Policy: schedule.MisfirePolicyFireOnce, MaxCatchUp: 1, GracePeriod: 24 * time.Hour}, JitterPolicy: schedule.ScheduleJitterPolicy{SeedMode: "disabled"}, DSTSpringPolicy: schedule.DSTSpringNextValidTime, DSTFallPolicy: schedule.DSTFallFireOnceFirst, ExecutionOwner: schedule.ExecutionOwnerBackend, ScopeRule: schedule.ScopeRule{ScopeType: "host_user"}}
	if request.EndAt != "" {
		end, err := parseTime(request.EndAt, loc)
		if err != nil || end.Before(anchor) {
			return nil, nil, errors.New("结束时间不能早于开始时间")
		}
		def.EndAt = &end
		item.EndAt = end.UTC().Format(time.RFC3339Nano)
	}
	if item.Kind == ProactiveTask && item.Policy != nil {
		key := sha256.Sum256([]byte(item.Caller.SpaceID + "\x00" + item.CharacterID))
		def.ConcurrencyPolicy = schedule.DefaultConcurrencyPolicy()
		def.ConcurrencyPolicy.SerializationKey = "proactive:" + hex.EncodeToString(key[:])
		if item.Policy.RandomMinutes < 0 || item.Policy.RandomMinutes > 120 {
			return nil, nil, errors.New("随机延迟须为 0 至 120 分钟")
		}
		if item.Policy.RandomMinutes > 0 {
			def.JitterPolicy = schedule.ScheduleJitterPolicy{Enabled: true, MaxDelay: time.Duration(item.Policy.RandomMinutes) * time.Minute, SeedMode: "schedule_id_scheduled_at"}
		}
	}
	if rule.Type == "cron" {
		def.StartAt = &anchor
	}
	return item, def, nil
}

func parseTime(value string, loc *time.Location) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			normalized, _ := schedule.NormalizeCalendarWallTime(parsed, loc)
			return normalized, nil
		}
	}
	return time.Time{}, errors.New("时间格式无效")
}
