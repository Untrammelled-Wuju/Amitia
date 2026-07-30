// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/log"
)

type RuntimeNotifierAdapter struct {
	dispatcher *Dispatcher
	registry  *RuntimeRegistry
}

func NewRuntimeNotifierAdapter(dispatcher *Dispatcher, registry *RuntimeRegistry) *RuntimeNotifierAdapter {
	return &RuntimeNotifierAdapter{
		dispatcher: dispatcher,
		registry:   registry,
	}
}

func (n *RuntimeNotifierAdapter) NotifyInstallationEnabled(userId, installationId string, settings *installation.RuntimeSettings) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	settingsSnapshot := n.toSettingsSnapshot(settings, 0)
	installationSnapshot := contracts.InstallationSnapshot{
		InstallationID:   installationId,
	}

	payload := contracts.SpawnPayload{
		DesiredRevision: n.generateRevision(),
		Installation:    installationSnapshot,
		Settings:        settingsSnapshot,
	}

	req := CommandRequest{
		Name:           contracts.MsgPetSpawn,
		RuntimeID:      n.selectRuntimeID(userId),
		UserID:         userId,
		InstallationID: installationId,
		Payload:        payload,
		Durability:      contracts.DurabilityDurable,
		CoalesceKey:    "pet_present_" + installationId,
		IdempotencyKey: "enable_" + installationId + "_" + uuid.NewString(),
		DesiredRevision: payload.DesiredRevision,
		Deadline:       30 * time.Second,
	}

	receipt, err := n.dispatcher.Dispatch(ctx, req)
	if err != nil {
		log.Logger.Errorf("runtime notifier: NotifyInstallationEnabled failed installationId=%s err=%v", installationId, err)
		return err
	}

	log.Logger.Infof("runtime notifier: enabled dispatched installationId=%s status=%s", installationId, receipt.DeliveryStatus)
	return nil
}

func (n *RuntimeNotifierAdapter) NotifyInstallationDisabled(userId, installationId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload := contracts.DestroyPayload{
		DesiredRevision: n.generateRevision(),
		Reason:          "disabled by user",
	}

	req := CommandRequest{
		Name:           contracts.MsgPetDestroy,
		RuntimeID:      n.selectRuntimeID(userId),
		UserID:         userId,
		InstallationID: installationId,
		Payload:        payload,
		Durability:      contracts.DurabilityDurable,
		CoalesceKey:    "pet_present_" + installationId,
		IdempotencyKey: "disable_" + installationId + "_" + uuid.NewString(),
		DesiredRevision: payload.DesiredRevision,
		Deadline:       30 * time.Second,
	}

	receipt, err := n.dispatcher.Dispatch(ctx, req)
	if err != nil {
		log.Logger.Errorf("runtime notifier: NotifyInstallationDisabled failed installationId=%s err=%v", installationId, err)
		return err
	}

	log.Logger.Infof("runtime notifier: disabled dispatched installationId=%s status=%s", installationId, receipt.DeliveryStatus)
	return nil
}

func (n *RuntimeNotifierAdapter) NotifyActionPlayed(userId, installationId, actionKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payload := contracts.PlayActionPayload{
		ActionKey: actionKey,
		ExpiresAt: time.Now().Add(15 * time.Second),
	}

	req := CommandRequest{
		Name:           contracts.MsgPetPlayAction,
		RuntimeID:      n.selectRuntimeID(userId),
		UserID:         userId,
		InstallationID: installationId,
		Payload:        payload,
		Durability:      contracts.DurabilityEphemeral,
		IdempotencyKey: "play_" + installationId + "_" + actionKey + "_" + uuid.NewString(),
		Deadline:       15 * time.Second,
	}

	receipt, err := n.dispatcher.Dispatch(ctx, req)
	if err != nil {
		log.Logger.Errorf("runtime notifier: NotifyActionPlayed failed installationId=%s actionKey=%s err=%v", installationId, actionKey, err)
		return err
	}

	log.Logger.Infof("runtime notifier: action played dispatched installationId=%s actionKey=%s status=%s", installationId, actionKey, receipt.DeliveryStatus)
	return nil
}

func (n *RuntimeNotifierAdapter) NotifyRecenter(installationId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payload := contracts.RecenterPayload{
		SettingsRevision: n.generateRevision(),
	}

	req := CommandRequest{
		Name:           contracts.MsgPetRecenter,
		RuntimeID:      n.selectRuntimeIDByInstallation(installationId),
		InstallationID: installationId,
		Payload:        payload,
		Durability:      contracts.DurabilityDurableImmediate,
		CoalesceKey:    "recenter_" + installationId,
		IdempotencyKey: "recenter_" + installationId + "_" + uuid.NewString(),
		DesiredRevision: payload.SettingsRevision,
		Deadline:       15 * time.Second,
	}

	receipt, err := n.dispatcher.Dispatch(ctx, req)
	if err != nil {
		log.Logger.Errorf("runtime notifier: NotifyRecenter failed installationId=%s err=%v", installationId, err)
		return err
	}

	log.Logger.Infof("runtime notifier: recenter dispatched installationId=%s status=%s", installationId, receipt.DeliveryStatus)
	return nil
}

func (n *RuntimeNotifierAdapter) NotifyDefaultActionChanged(installationId, actionKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payload := contracts.DefaultActionChangedPayload{
		ActionKey: actionKey,
		Revision:  n.generateRevision(),
	}

	req := CommandRequest{
		Name:           contracts.MsgPetDefaultActionChanged,
		RuntimeID:      n.selectRuntimeIDByInstallation(installationId),
		InstallationID: installationId,
		Payload:        payload,
		Durability:      contracts.DurabilityDurableCoalescing,
		CoalesceKey:    "default_action_" + installationId,
		IdempotencyKey: "default_action_" + installationId + "_" + uuid.NewString(),
		DesiredRevision: payload.Revision,
		Deadline:       15 * time.Second,
	}

	receipt, err := n.dispatcher.Dispatch(ctx, req)
	if err != nil {
		log.Logger.Errorf("runtime notifier: NotifyDefaultActionChanged failed installationId=%s actionKey=%s err=%v", installationId, actionKey, err)
		return err
	}

	log.Logger.Infof("runtime notifier: default action changed dispatched installationId=%s actionKey=%s status=%s", installationId, actionKey, receipt.DeliveryStatus)
	return nil
}

func (n *RuntimeNotifierAdapter) NotifyRuntimeSettingsUpdated(installationId string, settings *installation.RuntimeSettings) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	settingsMap := map[string]interface{}{
		"always_on_top":      settings.AlwaysOnTop,
		"scale":              settings.Scale,
		"position_x":         settings.PositionX,
		"position_y":         settings.PositionY,
		"screen_id":          settings.ScreenID,
		"click_through_mode": settings.ClickThroughMode,
		"sound_enabled":      settings.SoundEnabled,
	}
	settingsSnapshot := n.mapToSettingsSnapshot(settingsMap, 0)
	payload := contracts.UpdateSettingsPayload{
		SettingsRevision: n.generateRevision(),
		Settings:         settingsSnapshot,
	}

	req := CommandRequest{
		Name:           contracts.MsgPetUpdateSettings,
		RuntimeID:      n.selectRuntimeIDByInstallation(installationId),
		InstallationID: installationId,
		Payload:        payload,
		Durability:      contracts.DurabilityDurableCoalescing,
		CoalesceKey:    "settings_" + installationId,
		IdempotencyKey: "settings_" + installationId + "_" + uuid.NewString(),
		DesiredRevision: payload.SettingsRevision,
		Deadline:       15 * time.Second,
	}

	receipt, err := n.dispatcher.Dispatch(ctx, req)
	if err != nil {
		log.Logger.Errorf("runtime notifier: NotifyRuntimeSettingsUpdated failed installationId=%s err=%v", installationId, err)
		return err
	}

	log.Logger.Infof("runtime notifier: settings updated dispatched installationId=%s status=%s", installationId, receipt.DeliveryStatus)
	return nil
}

func (n *RuntimeNotifierAdapter) EnsureUninstalled(userId, installationId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload := contracts.DestroyPayload{
		DesiredRevision: n.generateRevision(),
		Reason:          "uninstalled",
	}

	req := CommandRequest{
		Name:           contracts.MsgPetDestroy,
		RuntimeID:      n.selectRuntimeID(userId),
		UserID:         userId,
		InstallationID: installationId,
		Payload:        payload,
		Durability:      contracts.DurabilityDurable,
		CoalesceKey:    "pet_present_" + installationId,
		IdempotencyKey: "uninstall_" + installationId + "_" + uuid.NewString(),
		DesiredRevision: payload.DesiredRevision,
		Deadline:       30 * time.Second,
	}

	receipt, err := n.dispatcher.Dispatch(ctx, req)
	if err != nil {
		if isOfflineError(err) {
			log.Logger.Infof("runtime notifier: ensure uninstalled pending (offline) installationId=%s", installationId)
			return nil
		}
		log.Logger.Errorf("runtime notifier: EnsureUninstalled failed installationId=%s err=%v", installationId, err)
		return err
	}

	log.Logger.Infof("runtime notifier: ensure uninstalled dispatched installationId=%s status=%s", installationId, receipt.DeliveryStatus)
	return nil
}

func (n *RuntimeNotifierAdapter) selectRuntimeID(userID string) string {
	conn, err := n.registry.SelectRuntime(userID)
	if err != nil || conn == nil {
		return ""
	}
	return conn.RuntimeID()
}

func (n *RuntimeNotifierAdapter) selectRuntimeIDByInstallation(installationID string) string {
	conns := n.registry.ListAll()
	for _, conn := range conns {
		if conn.State() == SessionStateReady || conn.State() == SessionStateSyncing {
			return conn.RuntimeID()
		}
	}
	return ""
}

func (n *RuntimeNotifierAdapter) toSettingsSnapshot(settings *installation.RuntimeSettings, revision int64) contracts.SettingsSnapshot {
	if settings == nil {
		return contracts.SettingsSnapshot{Revision: revision}
	}
	return contracts.SettingsSnapshot{
		Revision:        revision,
		AlwaysOnTop:     settings.AlwaysOnTop != 0,
		Scale:           settings.Scale,
		PositionX:       settings.PositionX,
		PositionY:       settings.PositionY,
		ScreenID:        settings.ScreenID,
		ClickThroughMode: settings.ClickThroughMode,
		SoundEnabled:    settings.SoundEnabled != 0,
	}
}

func (n *RuntimeNotifierAdapter) mapToSettingsSnapshot(settings map[string]interface{}, revision int64) contracts.SettingsSnapshot {
	snapshot := contracts.SettingsSnapshot{Revision: revision}
	if v, ok := settings["always_on_top"]; ok {
		if b, ok := v.(bool); ok {
			snapshot.AlwaysOnTop = b
		} else if i, ok := v.(int); ok {
			snapshot.AlwaysOnTop = i != 0
		} else if f, ok := v.(float64); ok {
			snapshot.AlwaysOnTop = f != 0
		}
	}
	if v, ok := settings["scale"]; ok {
		if f, ok := v.(float64); ok {
			snapshot.Scale = f
		}
	}
	if v, ok := settings["position_x"]; ok {
		if i, ok := v.(int); ok {
			snapshot.PositionX = i
		} else if f, ok := v.(float64); ok {
			snapshot.PositionX = int(f)
		}
	}
	if v, ok := settings["position_y"]; ok {
		if i, ok := v.(int); ok {
			snapshot.PositionY = i
		} else if f, ok := v.(float64); ok {
			snapshot.PositionY = int(f)
		}
	}
	if v, ok := settings["screen_id"]; ok {
		if s, ok := v.(string); ok {
			snapshot.ScreenID = s
		}
	}
	if v, ok := settings["click_through_mode"]; ok {
		if s, ok := v.(string); ok {
			snapshot.ClickThroughMode = s
		}
	}
	if v, ok := settings["sound_enabled"]; ok {
		if b, ok := v.(bool); ok {
			snapshot.SoundEnabled = b
		} else if i, ok := v.(int); ok {
			snapshot.SoundEnabled = i != 0
		} else if f, ok := v.(float64); ok {
			snapshot.SoundEnabled = f != 0
		}
	}
	return snapshot
}

func (n *RuntimeNotifierAdapter) generateRevision() int64 {
	return time.Now().UnixNano()
}

func isOfflineError(err error) bool {
	if re, ok := err.(*RuntimeError); ok {
		switch re.Code {
		case ErrCodeRuntimeOffline, ErrCodeRuntimeDisconnected, ErrCodeRuntimeNotReady:
			return true
		}
	}
	return false
}

var _ installation.RuntimeNotifier = (*RuntimeNotifierAdapter)(nil)

func init() {
	_ = json.Marshal
}
