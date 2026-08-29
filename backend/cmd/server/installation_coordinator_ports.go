package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
)

type coordinatorReleaseValidator struct {
	releases release.ReleaseRepository
}

func (p *coordinatorReleaseValidator) ValidateRelease(ctx context.Context, userID, releaseID string) (*coordinator.ReleaseValidationResult, error) {
	item, err := p.releases.GetRelease(releaseID)
	if err != nil {
		return nil, err
	}
	if item.OwnerUserID != "" && item.OwnerUserID != userID {
		return &coordinator.ReleaseValidationResult{
			ReleaseID:     item.ID,
			IsInstallable: false,
			ErrorMessage:  "release does not belong to user",
		}, nil
	}
	var manifest packageformat.Manifest
	if err := json.Unmarshal([]byte(item.ManifestJSON), &manifest); err != nil {
		return nil, fmt.Errorf("decode release manifest: %w", err)
	}
	manifestValid := item.ManifestHash != "" && item.ContentRootHash != "" && manifest.ReleaseID == item.ID && manifest.PetID == item.PetID && manifest.DefaultAction != ""
	actionKeys := make([]string, 0, len(manifest.Actions))
	seenActionKeys := make(map[string]struct{}, len(manifest.Actions))
	for _, action := range manifest.Actions {
		key := strings.TrimSpace(action.Key)
		if key == "" {
			continue
		}
		if _, exists := seenActionKeys[key]; exists {
			continue
		}
		seenActionKeys[key] = struct{}{}
		actionKeys = append(actionKeys, key)
	}
	return &coordinator.ReleaseValidationResult{
		ReleaseID:        item.ID,
		PetID:            item.PetID,
		IsInstallable:    release.IsInstallable(item.Lifecycle, item.IntegrityStatus, item.CompatibilityStatus) && manifestValid,
		HasPublishedCopy: item.StorageKey != "",
		ManifestValid:    manifestValid,
		PublishedPathKey: item.StorageKey,
		DefaultActionKey: manifest.DefaultAction,
		ActionKeys:       actionKeys,
	}, nil
}

type coordinatorInstallationLookup interface {
	GetInstallation(ctx context.Context, userID, deviceID, installationID string) (*coordinator.InstallationRecord, error)
}

type coordinatorRuntimePublisher struct {
	facade        *runtimev2.RuntimeFacade
	installations coordinatorInstallationLookup
}

func (p *coordinatorRuntimePublisher) PublishDesiredState(ctx context.Context, deviceCtx device.DeviceContext, snapshot *coordinator.DesiredStateSnapshot) error {
	if p.facade == nil {
		return fmt.Errorf("runtime v2 unavailable")
	}
	if !deviceCtx.IsValid() {
		return fmt.Errorf("invalid device context")
	}
	seq, err := p.facade.Commands().AllocateDeviceSequence(nil, deviceCtx.UserID, deviceCtx.DeviceID, time.Now())
	if err != nil {
		return fmt.Errorf("allocate sequence: %w", err)
	}
	var settingsSnapshot json.RawMessage
	if snapshot.SettingsSnapshotJSON != "" {
		if !json.Valid([]byte(snapshot.SettingsSnapshotJSON)) {
			return fmt.Errorf("persisted settings snapshot is invalid JSON")
		}
		settingsSnapshot = json.RawMessage(snapshot.SettingsSnapshotJSON)
	}
	payload := runtimev2.SyncDesiredStatePayload{
		DesiredRevision:        snapshot.DesiredRevision,
		DesiredHash:            snapshot.DesiredHash,
		EnsureAbsent:           snapshot.EnsureAbsent,
		InstallationID:         snapshot.InstallationID,
		PetID:                  snapshot.PetID,
		CharacterID:            "",
		ReleaseID:              snapshot.ReleaseID,
		RuntimeContractVersion: runtimev2.CurrentSchemaVersion,
		DefaultActionKey:       snapshot.DefaultActionKey,
		SettingsRevision:       snapshot.SettingsRevision,
		SettingsSnapshot:       settingsSnapshot,
	}
	commandType := runtimev2.CommandTypeSyncDesiredState
	if snapshot.EnsureAbsent {
		commandType = runtimev2.CommandTypeEnsureAbsent
	}
	_, err = p.facade.Commands().CreateDurableCommand(
		deviceCtx.UserID,
		deviceCtx.DeviceID,
		string(commandType),
		fmt.Sprintf("desired:%s:%d", deviceCtx.DeviceID, snapshot.DesiredRevision),
		fmt.Sprintf("desired:%s", deviceCtx.DeviceID),
		seq,
		payload,
	)
	if err != nil {
		if err == runtimev2.ErrCommandDuplication {
			return nil
		}
		return fmt.Errorf("create durable command: %w", err)
	}
	return nil
}

func (p *coordinatorRuntimePublisher) activeRuntimeConnection(deviceCtx device.DeviceContext) (*runtimev2.Connection, string, int64, error) {
	if p.facade == nil {
		return nil, "", 0, fmt.Errorf("runtime v2 unavailable")
	}
	targetRuntimeID := strings.TrimSpace(deviceCtx.RuntimeID)
	for _, conn := range p.facade.ListConnections(deviceCtx.UserID) {
		if conn == nil || conn.GetState() != runtimev2.ConnStateConnected {
			continue
		}
		if string(conn.DeviceID) != deviceCtx.DeviceID {
			continue
		}
		if targetRuntimeID != "" && string(conn.RuntimeID) != targetRuntimeID {
			continue
		}
		sessionID, generation := conn.SessionSnapshot()
		if sessionID == "" || generation <= 0 {
			continue
		}
		return conn, sessionID, generation, nil
	}
	return nil, "", 0, fmt.Errorf("%w: device=%s runtime=%s", coordinator.ErrRuntimeUnavailable, deviceCtx.DeviceID, targetRuntimeID)
}

func (p *coordinatorRuntimePublisher) PublishRecenter(ctx context.Context, deviceCtx device.DeviceContext, installationID, operationID string) (string, error) {
	if p.facade == nil {
		return "", fmt.Errorf("runtime v2 unavailable")
	}
	if !deviceCtx.IsValid() {
		return "", fmt.Errorf("invalid device context")
	}
	if operationID == "" {
		return "", fmt.Errorf("operation id is required")
	}
	targetConn, targetSessionID, targetGeneration, err := p.activeRuntimeConnection(deviceCtx)
	if err != nil {
		return "", err
	}
	payload := map[string]interface{}{
		"installationId": installationID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal recenter payload: %w", err)
	}
	cmd, err := p.facade.Commands().CreateEphemeralCommandForSession(
		deviceCtx.UserID,
		deviceCtx.DeviceID,
		string(targetConn.RuntimeID),
		targetSessionID,
		installationID,
		string(runtimev2.CommandTypeRecenterOnce),
		fmt.Sprintf("recenter:%s:%s", operationID, targetSessionID),
		payloadBytes,
	)
	if err != nil {
		if err == runtimev2.ErrCommandDuplication && cmd != nil {
			return cmd.ID, nil
		}
		return "", fmt.Errorf("create recenter command: %w", err)
	}
	if cmd == nil || cmd.ID == "" {
		return "", fmt.Errorf("create recenter command: empty command id")
	}
	currentSessionID, currentGeneration := targetConn.SessionSnapshot()
	if targetConn.GetState() != runtimev2.ConnStateConnected || currentSessionID != targetSessionID || currentGeneration != targetGeneration {
		_ = p.facade.Commands().MarkSuperseded(cmd.ID, "runtime session changed during recenter creation", time.Now().UTC())
		return "", fmt.Errorf("%w: runtime session changed while scheduling recenter", coordinator.ErrRuntimeUnavailable)
	}
	if cmd.RuntimeSessionID != targetSessionID {
		return "", fmt.Errorf("%w: duplicate recenter belongs to stale runtime session", coordinator.ErrRuntimeUnavailable)
	}
	return cmd.ID, nil
}

func (p *coordinatorRuntimePublisher) PublishPlayAction(ctx context.Context, deviceCtx device.DeviceContext, installationID, actionKey string) error {
	if p.facade == nil {
		return fmt.Errorf("%w: runtime v2 facade unavailable", coordinator.ErrRuntimeUnavailable)
	}
	if !deviceCtx.IsValid() || installationID == "" || actionKey == "" {
		return fmt.Errorf("invalid play action request")
	}
	targetConn, targetSessionID, targetGeneration, err := p.activeRuntimeConnection(deviceCtx)
	if err != nil {
		return err
	}
	targetRuntimeID := string(targetConn.RuntimeID)
	if p.installations == nil {
		return fmt.Errorf("play action installation lookup unavailable")
	}
	inst, err := p.installations.GetInstallation(ctx, deviceCtx.UserID, deviceCtx.DeviceID, installationID)
	if err != nil {
		return fmt.Errorf("resolve play action installation: %w", err)
	}
	if inst == nil || strings.TrimSpace(inst.CharacterID) == "" {
		return fmt.Errorf("play action installation has no character identity")
	}
	payload := runtimev2.PlayActionPayload{
		RuntimeID:        targetRuntimeID,
		ActionKey:        actionKey,
		CharacterID:      strings.TrimSpace(inst.CharacterID),
		PetInstanceID:    targetRuntimeID,
		InstallationID:   installationID,
		PlaybackMode:     "once",
		Priority:         0,
		QueuePolicy:      runtimev2.PlayActionQueueReplaceCurrent,
		Interruptible:    true,
		ReturnTo:         "default",
		PlaybackRate:     1.0,
		CompletionPolicy: runtimev2.PlayActionCompletionOnStarted,
		Semantic:         "manual",
		ReasonCode:       "manual_play_request",
		ExpiresAt:        time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal play action payload: %w", err)
	}
	created, err := p.facade.Commands().CreateEphemeralCommandForSession(
		deviceCtx.UserID,
		deviceCtx.DeviceID,
		targetRuntimeID,
		targetSessionID,
		installationID,
		string(runtimev2.CommandTypePlayAction),
		fmt.Sprintf("play:%s:%s:%s:%s", deviceCtx.DeviceID, installationID, actionKey, uuid.NewString()),
		payloadBytes,
	)
	if err != nil {
		if err == runtimev2.ErrCommandDuplication {
			return nil
		}
		return fmt.Errorf("create play action command: %w", err)
	}
	if created == nil {
		return fmt.Errorf("create play action command: empty command")
	}
	currentSessionID, currentGeneration := targetConn.SessionSnapshot()
	if targetConn.GetState() != runtimev2.ConnStateConnected || currentSessionID != targetSessionID || currentGeneration != targetGeneration {
		_ = p.facade.Commands().MarkSuperseded(created.ID, "runtime session changed during play action creation", time.Now().UTC())
		return fmt.Errorf("%w: runtime session changed while scheduling play action", coordinator.ErrRuntimeUnavailable)
	}
	return nil
}
