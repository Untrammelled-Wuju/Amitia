package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	return &coordinator.ReleaseValidationResult{
		ReleaseID:        item.ID,
		PetID:            item.PetID,
		IsInstallable:    release.IsInstallable(item.Lifecycle, item.IntegrityStatus, item.CompatibilityStatus) && manifestValid,
		HasPublishedCopy: item.StorageKey != "",
		ManifestValid:    manifestValid,
		PublishedPathKey: item.StorageKey,
		DefaultActionKey: manifest.DefaultAction,
	}, nil
}

type coordinatorRuntimePublisher struct {
	facade *runtimev2.RuntimeFacade
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

func (p *coordinatorRuntimePublisher) PublishRecenter(ctx context.Context, deviceCtx device.DeviceContext, installationID string) error {
	if p.facade == nil {
		return fmt.Errorf("runtime v2 unavailable")
	}
	if !deviceCtx.IsValid() {
		return fmt.Errorf("invalid device context")
	}
	payload := map[string]interface{}{
		"installationId": installationID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal recenter payload: %w", err)
	}
	_, err = p.facade.Commands().CreateEphemeralCommand(
		deviceCtx.UserID,
		deviceCtx.DeviceID,
		string(runtimev2.CommandTypeRecenterOnce),
		fmt.Sprintf("recenter:%s:%s", deviceCtx.DeviceID, installationID),
		payloadBytes,
	)
	if err != nil {
		if err == runtimev2.ErrCommandDuplication {
			return nil
		}
		return fmt.Errorf("create recenter command: %w", err)
	}
	return nil
}

func (p *coordinatorRuntimePublisher) PublishPlayAction(ctx context.Context, deviceCtx device.DeviceContext, installationID, actionKey string) error {
	if p.facade == nil {
		return fmt.Errorf("runtime v2 unavailable")
	}
	if !deviceCtx.IsValid() || installationID == "" || actionKey == "" {
		return fmt.Errorf("invalid play action request")
	}
	payload := runtimev2.PlayActionPayload{
		ActionKey:        actionKey,
		PlaybackMode:     "once",
		Priority:         0,
		QueuePolicy:      "replace",
		Interruptible:    true,
		PlaybackRate:     1.0,
		CompletionPolicy: "started",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal play action payload: %w", err)
	}
	_, err = p.facade.Commands().CreateEphemeralCommand(
		deviceCtx.UserID,
		deviceCtx.DeviceID,
		string(runtimev2.CommandTypePlayAction),
		fmt.Sprintf("play:%s:%s:%s:%d", deviceCtx.DeviceID, installationID, actionKey, time.Now().UnixNano()),
		payloadBytes,
	)
	if err != nil {
		if err == runtimev2.ErrCommandDuplication {
			return nil
		}
		return fmt.Errorf("create play action command: %w", err)
	}
	return nil
}
