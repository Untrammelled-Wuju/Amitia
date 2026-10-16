package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	agenttool "github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/reminder"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"gorm.io/gorm"
)

type reminderSettings struct{ db *gorm.DB }

func (s reminderSettings) Read(ctx context.Context, key string) (string, error) {
	var rows []struct{ Value string }
	err := s.db.WithContext(ctx).Table("app_settings").Select("value").Where("key = ? AND deleted_at IS NULL", key).Limit(1).Scan(&rows).Error
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0].Value, nil
}

func (s reminderSettings) Write(ctx context.Context, key, value string) error {
	return s.db.WithContext(ctx).Exec(`INSERT INTO app_settings(key,value,revision,deleted_at,updated_at) VALUES(?,?,1,NULL,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,revision=app_settings.revision+1,deleted_at=NULL,updated_at=excluded.updated_at`, key, value, time.Now().UTC().Format(time.RFC3339Nano)).Error
}

func wireReminderService(services *AppServices) error {
	if services.OwnedBusiness == nil || services.KernelContainer == nil || services.DeviceMesh == nil {
		return nil
	}
	container := services.KernelContainer
	if container.ScheduleService == nil {
		return errors.New("统一提醒调度底座未接通")
	}
	coreID := requestidentity.CanonicalSpaceID()
	container.ContextualScopeSnapshotCreator = func(ctx context.Context, extension, module string, generation int64, role, conversation string) (string, error) {
		actor, ok := auth.FromContext(ctx)
		if !ok || actor == nil {
			return "", reminder.ErrForbidden
		}
		device := actor.DeviceID.String()
		if device == "" && actor.IsLocalTrusted && services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
			device = services.DeviceMesh.LocalDeviceID
		}
		if device == "" || actor.SpaceID == "" {
			return "", reminder.ErrForbidden
		}
		invocation := "ui-" + fmt.Sprint(time.Now().UnixNano())
		child, authority, finish, err := services.DeviceMesh.Coordination.Begin(ctx, actor.SpaceID.String(), device, "", coreID, role, invocation)
		if err != nil {
			return "", err
		}
		defer finish()
		scopes := []scope.ScopeRef{scope.NewExtensionScope(extension), scope.NewModuleScope(extension, module), scope.NewInvocationScope(invocation), scope.NewSessionScope(invocation)}
		if role != "" {
			scopes = append(scopes, scope.NewCharacterScope(role))
		}
		if conversation != "" {
			scopes = append(scopes, scope.NewConversationScope(conversation))
		}
		snapshot := scope.CreateSnapshotWithOwner(invocation, scopes, actor.SpaceID.String(), role, conversation, extension, module, generation)
		snapshot.OwnedExecutionScope, err = json.Marshal(authority)
		if err != nil {
			return "", err
		}
		if err := scope.NewSQLiteScopeStore(container.DeviceRegistry.Database()).SaveSnapshot(child, snapshot); err != nil {
			return "", err
		}
		return snapshot.SnapshotID, nil
	}
	pluginActive := func(ctx context.Context, plugin, module string) error {
		if !container.IsExtensionActive(plugin) {
			return errors.New("任务来源插件已停用或卸载")
		}
		if _, err := container.ModuleRepository.GetModule(ctx, domain.ExtensionID(plugin), domain.ModuleID(module)); err != nil {
			return err
		}
		state, err := container.EnablementStore.Get(ctx, enablement.StateSubject{Kind: enablement.SubjectModule, ID: module, ParentID: plugin})
		if err != nil || state.Enablement != enablement.EnablementEnabled {
			return errors.New("任务来源模块未启用")
		}
		return nil
	}
	authorize := func(ctx context.Context, caller reminder.Caller, item *reminder.Reminder) (json.RawMessage, error) {
		if actor, ok := auth.FromContext(ctx); ok && actor.DeviceID != "" && actor.DeviceID.String() != caller.DeviceID {
			return nil, reminder.ErrForbidden
		}
		if caller.PluginID != "" {
			decision := container.ScopeManager.Evaluate(ctx, scope.ScopeEvaluationRequest{SubjectType: scope.SubjectModule, SubjectID: caller.ModuleID, ExtensionID: caller.PluginID, ModuleID: caller.ModuleID, CharacterID: item.CharacterID, ConversationID: item.ConversationID})
			if !decision.Allowed {
				return nil, reminder.ErrForbidden
			}
			createPermission := "reminder.create"
			if item.Kind == reminder.ProactiveTask {
				createPermission = "proactive_task.create"
			}
			requirements := []permission.PermissionRequirement{{PermissionID: createPermission, Scope: permission.ScopeForExtension(caller.PluginID)}, {PermissionID: "message.send", Scope: permission.ScopeForExtension(caller.PluginID)}}
			if item.SystemNotification {
				requirements = append(requirements, permission.PermissionRequirement{PermissionID: "ui.notify", Scope: permission.ScopeForExtension(caller.PluginID)})
			}
			result := container.PermissionBroker.Evaluate(ctx, permission.PermissionEvaluationRequest{Subject: permission.PermissionSubject{Type: permission.SubjectModule, ID: caller.ModuleID, ExtensionID: caller.PluginID, ModuleID: caller.ModuleID}, Requirements: requirements, IsBackground: true})
			if result.Decision != permission.DecisionAllow && result.Decision != permission.DecisionAllowPersistent && result.Decision != permission.DecisionAllowOnce && result.Decision != permission.DecisionAllowSession {
				return nil, reminder.ErrForbidden
			}
		}
		request := business.Request{SpaceID: caller.SpaceID, DeviceID: caller.DeviceID, CoreID: coreID, TargetDeviceID: item.TargetDeviceID, RoleID: item.CharacterID, ConversationID: item.ConversationID, RequestID: item.ID + "/prepare"}
		id, binding, err := services.OwnedBusiness.PrepareScheduledConversation(ctx, request, item.Title, item.Binding)
		if err != nil {
			return nil, err
		}
		item.ConversationID = id
		for _, target := range item.ChannelTargets {
			if target.ChannelID == "" || target.PeerID == "" || container.ChannelResolver == nil || !container.ChannelResolver.Has(target.ChannelID) {
				return nil, errors.New("指定渠道未接入或缺少接收对象")
			}
		}
		for _, channel := range append(item.Channels, item.Channel) {
			if channel == "" || channel == "web" {
				continue
			}
			found := false
			for _, target := range item.ChannelTargets {
				if target.ChannelID == channel {
					found = true
				}
			}
			if !found {
				return nil, errors.New("指定渠道必须提供明确接收对象")
			}
		}
		return json.Marshal(binding)
	}
	deliver := func(ctx context.Context, item *reminder.Reminder, triggerID string) (reminder.DeliveryResult, error) {
		requestID := "reminder:" + triggerID
		fallback := item.Title
		if item.Content != "" {
			fallback += "\n" + item.Content
		}
		instruction := "请用所属角色的自然语气提醒用户：" + fallback
		source := "reminder"
		if item.Kind == reminder.ProactiveTask {
			instruction = item.Content
			fallback = ""
			source = "proactive"
		}
		message, err := services.OwnedBusiness.SendScheduledMessage(ctx, business.Request{SpaceID: item.Caller.SpaceID, DeviceID: item.Caller.DeviceID, CoreID: coreID, TargetDeviceID: item.TargetDeviceID, RoleID: item.CharacterID, ConversationID: item.ConversationID, RequestID: requestID, Message: instruction}, item.Binding, source, fallback, func(ctx context.Context, commit func(context.Context) error) error {
			return container.ScheduleService.GuardHostExecution(ctx, item.ID, item.Generation, commit)
		})
		if err != nil {
			return reminder.DeliveryResult{}, err
		}
		result := reminder.DeliveryResult{MessageID: message.MessageID, ConversationID: message.ConversationID, MessageSaved: message.Saved, GenerationDegraded: message.Degraded, Targets: []reminder.TargetResult{{Kind: "conversation", ID: message.ConversationID, State: "saved"}}}
		for _, target := range item.ChannelTargets {
			if services.DeliveryStore == nil {
				return result, errors.New("渠道投递服务未接通")
			}
			payload, _ := json.Marshal(map[string]any{"messageId": message.MessageID, "conversationId": message.ConversationID, "characterId": item.CharacterID, "content": message.Content, "deliveryGroupId": triggerID, "deliverySequence": 1})
			id := delivery.GenerateDeliveryID(triggerID, target.ChannelID, target.PeerID, "reminder-message")
			_, err := services.DeliveryStore.SubmitIntent(delivery.DeliveryIntent{ID: id, InteractionID: triggerID, DeliveryGroupID: triggerID, DeliverySequence: 1, Channel: target.ChannelID, PeerID: target.PeerID, ContentType: "text", Payload: payload, Status: delivery.DeliveryStatusPending, CreatedAt: time.Now().UTC(), MaxRetries: 5})
			if err != nil {
				return result, err
			}
			intent, err := services.DeliveryStore.GetIntent(id)
			if err != nil || intent == nil {
				if err == nil {
					err = errors.New("渠道投递未确认保存")
				}
				return result, err
			}
			result.Targets = append(result.Targets, reminder.TargetResult{Kind: "channel", ID: id, State: string(intent.Status), Error: intent.LastError})
		}
		if item.SystemNotification {
			if services.NotificationRuntime == nil {
				result.Targets = append(result.Targets, reminder.TargetResult{Kind: "notification", ID: item.Caller.DeviceID, State: "blocked", Error: "系统通知服务未接通"})
			} else {
				kind := "reminder.triggered"
				if item.Kind == reminder.ProactiveTask {
					kind = "proactive.message"
				}
				targets, err := services.NotificationRuntime.QueueScheduled(ctx, item.Caller.SpaceID, item.Caller.DeviceID, triggerID, item.ConversationID, item.CharacterID, message.MessageID, item.Title, message.Content, kind)
				if err != nil {
					return result, err
				}
				for _, target := range targets {
					result.Targets = append(result.Targets, reminder.TargetResult{Kind: "notification", ID: target.ID, State: target.State, Error: target.Error})
				}
			}
		}
		_, err = conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{ConversationID: item.ConversationID, MessageID: message.MessageID, RequestID: requestID, Type: "message_created", Status: "completed", Payload: map[string]any{"source": source, "content": message.Content, "characterId": item.CharacterID, "spaceId": item.Caller.SpaceID, "ownerId": message.Scope.ResourceOwnerID, "reminderId": item.ID, "executionScope": message.Scope}}, true)
		return result, err
	}
	services.Reminders = reminder.NewService(reminder.Dependencies{Scheduler: container.ScheduleService, OwnerID: coreID, Mode: services.RuntimeProfile.String(), Authorize: authorize, Deliver: deliver, PluginActive: pluginActive, Settings: reminderSettings{services.DB}, Channels: func(_ context.Context, _ reminder.Caller) ([]string, error) {
		if container.ChannelResolver == nil {
			return []string{}, nil
		}
		return container.ChannelResolver.Channels(), nil
	}, ResolveTarget: func(ctx context.Context, target reminder.TargetResult) (reminder.TargetResult, error) {
		if target.Kind == "channel" && services.DeliveryStore != nil {
			intent, err := services.DeliveryStore.GetIntent(target.ID)
			if err != nil {
				return target, err
			}
			if intent != nil {
				target.State, target.Error = string(intent.Status), intent.LastError
			}
		}
		if target.Kind == "notification" && services.NotificationRuntime != nil && target.State != "blocked" && target.State != "suppressed" {
			current, err := services.NotificationRuntime.ScheduledState(ctx, target.ID)
			if err != nil {
				return target, err
			}
			target.State, target.Error = current.State, current.Error
		}
		return target, nil
	}, CheckPolicy: func(ctx context.Context, item *reminder.Reminder, history []reminder.TriggerHistory) (string, error) {
		return checkProactivePolicy(ctx, services, item, history)
	}})
	container.ScheduleService.SetMaintenance(services.Reminders.CleanupAllHistory)
	resolve := func(ctx context.Context, request host_api.CallRequest) (reminder.Caller, string, string, error) {
		caller := reminder.Caller{PluginID: string(request.RuntimeIdentity.ExtensionID), ModuleID: string(request.RuntimeIdentity.ModuleID)}
		role, conversation := "", ""
		if request.ScopeSnapshotID != "" {
			store := scope.NewSQLiteScopeStore(container.DeviceRegistry.Database())
			snapshot, err := store.GetSnapshot(ctx, request.ScopeSnapshotID)
			if err != nil {
				return caller, "", "", err
			}
			role, conversation = snapshot.CharacterID, snapshot.ConversationID
			if len(snapshot.OwnedExecutionScope) > 0 {
				var owned coordination.ExecutionScope
				if json.Unmarshal(snapshot.OwnedExecutionScope, &owned) != nil {
					return caller, "", "", reminder.ErrForbidden
				}
				caller.SpaceID = owned.SpaceID
				caller.DeviceID = owned.InitiatorDeviceID
			}
		}
		if actor, ok := auth.FromContext(ctx); ok && actor.DeviceID != "" {
			caller.SpaceID = actor.SpaceID.String()
			caller.DeviceID = actor.DeviceID.String()
		}
		if caller.DeviceID == "" && services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
			caller.SpaceID = coreID
			caller.DeviceID = services.DeviceMesh.LocalDeviceID
		}
		if caller.DeviceID == "" || caller.SpaceID == "" || caller.PluginID == "" || caller.ModuleID == "" {
			return caller, "", "", reminder.ErrForbidden
		}
		return caller, role, conversation, nil
	}
	if err := kernel.RegisterReminderHostRoutes(container.HostAPIGateway, services.Reminders, resolve); err != nil {
		return err
	}
	agenttool.SetScheduleCreator(func(ctx context.Context, execution agenttool.ToolExecutionContext, args map[string]interface{}) (string, error) {
		caller := reminder.Caller{SpaceID: execution.SpaceID, DeviceID: services.DeviceMesh.LocalDeviceID}
		trustedDevice := false
		if owned, ok := coordination.FromContext(ctx); ok {
			caller.SpaceID = owned.SpaceID
			caller.DeviceID = owned.InitiatorDeviceID
			trustedDevice = owned.InitiatorDeviceID != "" && owned.SpaceID != ""
		}
		if actor, ok := auth.FromContext(ctx); ok && actor.DeviceID != "" {
			caller.SpaceID = actor.SpaceID.String()
			caller.DeviceID = actor.DeviceID.String()
			trustedDevice = actor.SpaceID != ""
		}
		if services.RuntimeProfile == runtimeprofile.ProfileCloudCore && !trustedDevice {
			return "", reminder.ErrForbidden
		}
		text := func(key string) string { value, _ := args[key].(string); return strings.TrimSpace(value) }
		request := reminder.CreateReminderRequest{Title: text("title"), Content: text("description"), RemindAt: text("due_time"), RepeatRule: text("repeat"), Timezone: text("timezone"), Channel: text("channel"), CharacterID: execution.CharacterID, ConversationID: execution.ConversationID, RequestID: execution.IdempotencyKey}
		if request.RequestID == "" {
			request.RequestID = execution.RequestID + "/" + execution.ToolCallID
		}
		if request.RequestID == "/" {
			return "", errors.New("聊天工具缺少请求及调用编号")
		}
		if raw, ok := args["recurrence"]; ok {
			encoded, _ := json.Marshal(raw)
			if json.Unmarshal(encoded, &request.Recurrence) != nil {
				return "", errors.New("重复规则参数无效")
			}
		}
		if raw, ok := args["channel_targets"]; ok {
			encoded, _ := json.Marshal(raw)
			if json.Unmarshal(encoded, &request.ChannelTargets) != nil {
				return "", errors.New("渠道接收对象参数无效")
			}
		}
		if value, ok := args["end_at"].(string); ok {
			request.EndAt = value
		}
		if value, ok := args["max_occurrences"].(float64); ok {
			request.MaxOccurrences = int(value)
		}
		item, err := services.Reminders.Create(ctx, caller, request, "chat")
		if err != nil {
			return "", err
		}
		if !item.Saved {
			return "", errors.New("提醒未确认保存")
		}
		return item.ID, nil
	})
	return nil
}

func checkProactivePolicy(ctx context.Context, services *AppServices, item *reminder.Reminder, history []reminder.TriggerHistory) (string, error) {
	policy := item.Policy
	if policy == nil || !policy.Enabled || policy.ActiveLevel <= 0 {
		return "主动任务策略已关闭", nil
	}
	loc, err := time.LoadLocation(item.Timezone)
	if err != nil {
		return "", err
	}
	now := time.Now().In(loc)
	minute := func(value string) (int, error) {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return 0, errors.New("免打扰时间无效")
		}
		var hour, min int
		if _, err := fmt.Sscanf(value, "%d:%d", &hour, &min); err != nil || hour < 0 || hour > 23 || min < 0 || min > 59 {
			return 0, errors.New("免打扰时间无效")
		}
		return hour*60 + min, nil
	}
	quietStart, quietEnd := policy.QuietStart, policy.QuietEnd
	if policy.RuleQuietStart != "" {
		quietStart = policy.RuleQuietStart
	}
	if policy.RuleQuietEnd != "" {
		quietEnd = policy.RuleQuietEnd
	}
	if quietStart != "" && quietEnd != "" {
		start, err := minute(quietStart)
		if err != nil {
			return "", err
		}
		end, err := minute(quietEnd)
		if err != nil {
			return "", err
		}
		current := now.Hour()*60 + now.Minute()
		if start < end && current >= start && current < end || start > end && (current >= start || current < end) {
			return "免打扰时段", nil
		}
	}
	sent := 0
	for _, entry := range history {
		if entry.CreatedAt.In(loc).Format("2006-01-02") != now.Format("2006-01-02") {
			continue
		}
		if entry.State == "completed" || entry.State == "partial" {
			sent++
		}
	}
	if policy.RuleMaxPerDay > 0 && sent >= policy.RuleMaxPerDay {
		return "规则今日发送上限", nil
	}
	caller := item.Caller
	caller.RoleScope = item.CharacterID
	all, err := services.Reminders.PolicyHistory(ctx, caller)
	if err != nil {
		return "", err
	}
	globalSent, calls := 0, 0
	var lastSent time.Time
	for _, entry := range all {
		if entry.CreatedAt.In(loc).Format("2006-01-02") == now.Format("2006-01-02") && entry.State != "skipped" {
			calls += entry.AttemptCount
		}
		if entry.State == "completed" || entry.State == "partial" {
			if entry.UpdatedAt.After(lastSent) {
				lastSent = entry.UpdatedAt
			}
			if entry.CreatedAt.In(loc).Format("2006-01-02") == now.Format("2006-01-02") {
				globalSent++
			}
		}
	}
	if policy.MaxPerDay > 0 && globalSent >= policy.MaxPerDay {
		return "今日主动任务发送上限", nil
	}
	if policy.MaxDailyCalls > 0 && calls >= policy.MaxDailyCalls {
		return "今日调用上限", nil
	}
	gap := proactiveMinimumInterval(policy)
	if policy.UnrepliedSlowdownEnabled && policy.UnrepliedSlowdownAfter > 0 {
		activity, err := services.OwnedBusiness.ScheduledConversationActivity(ctx, business.Request{SpaceID: item.Caller.SpaceID, DeviceID: item.Caller.DeviceID, CoreID: item.OwnerID, TargetDeviceID: item.TargetDeviceID, RoleID: item.CharacterID, ConversationID: item.ConversationID, RequestID: item.ID + "/activity"}, item.Binding)
		if err != nil {
			return "", err
		}
		unreplied := activity.Unreplied
		if !policy.UnrepliedRecoveryOnReply {
			unreplied = globalSent
		}
		if unreplied >= policy.UnrepliedSlowdownAfter && policy.UnrepliedCooldownMultiplier > 1 {
			gap = reminder.ScaleCooldown(gap, policy.UnrepliedCooldownMultiplier)
		}
	}
	if !lastSent.IsZero() && time.Since(lastSent) < gap {
		return "尚在主动任务冷却时段", nil
	}
	return "", nil
}

func proactiveMinimumInterval(policy *reminder.ProactivePolicy) time.Duration {
	return reminder.ProactiveMinimumInterval(policy)
}
