package main

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
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/devicemesh"
	devicemeshagent "github.com/u-ai/backend/internal/devicemesh/agent"
	deviceruntimeprotocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/spaceidentity"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	desktopPetBehaviorMeshHandler        = "desktop_pet.behavior.submit_event"
	desktopPetBehaviorMeshResolveHandler = "desktop_pet.behavior.resolve_target"
	behaviorMeshPollInterval             = 2 * time.Second
	behaviorMeshBatchSize                = 50
	behaviorMeshInvokeTimeout            = 5 * time.Second
	behaviorMeshClaimLease               = 5 * time.Minute
)

// desktopPetOwnerMapping freezes the authority translation between a Cloud
// Space and the owner namespace used by the local Desktop Pet database. The
// mapping is created from the authenticated Device Mesh credential, not from
// caller-controlled behavior payloads.
type desktopPetOwnerMapping struct {
	CloudSpaceID string    `gorm:"column:cloud_space_id;primaryKey;size:128"`
	DeviceID     string    `gorm:"column:device_id;primaryKey;size:128"`
	LocalOwnerID string    `gorm:"column:local_owner_id;not null;size:128"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (desktopPetOwnerMapping) TableName() string { return "desktop_pet_owner_mappings" }

type desktopPetOwnerMapper struct {
	db *gorm.DB
}

func newDesktopPetOwnerMapper(db *gorm.DB) (*desktopPetOwnerMapper, error) {
	if db == nil {
		return nil, errors.New("desktop pet owner mapper: db is nil")
	}
	if err := db.AutoMigrate(&desktopPetOwnerMapping{}); err != nil {
		return nil, fmt.Errorf("desktop pet owner mapper schema: %w", err)
	}
	return &desktopPetOwnerMapper{db: db}, nil
}

func (m *desktopPetOwnerMapper) Bind(ctx context.Context, cloudSpaceID, deviceID, localOwnerID string) error {
	cloudSpaceID = strings.TrimSpace(cloudSpaceID)
	deviceID = strings.TrimSpace(deviceID)
	localOwnerID = strings.TrimSpace(localOwnerID)
	if m == nil || m.db == nil {
		return errors.New("desktop pet owner mapper unavailable")
	}
	if cloudSpaceID == "" || deviceID == "" || localOwnerID == "" {
		return errors.New("desktop pet owner mapping requires cloud Space, device and local owner")
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing desktopPetOwnerMapping
		err := tx.Where("cloud_space_id = ? AND device_id = ?", cloudSpaceID, deviceID).First(&existing).Error
		switch {
		case err == nil:
			if existing.LocalOwnerID != localOwnerID {
				return fmt.Errorf("desktop pet owner mapping conflict for device %s", deviceID)
			}
			return tx.Model(&existing).Update("updated_at", time.Now().UTC()).Error
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}
		now := time.Now().UTC()
		return tx.Create(&desktopPetOwnerMapping{
			CloudSpaceID: cloudSpaceID,
			DeviceID:     deviceID,
			LocalOwnerID: localOwnerID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}).Error
	})
}

func (m *desktopPetOwnerMapper) Resolve(ctx context.Context, cloudSpaceID, deviceID string) (string, error) {
	if m == nil || m.db == nil {
		return "", errors.New("desktop pet owner mapper unavailable")
	}
	var mapping desktopPetOwnerMapping
	if err := m.db.WithContext(ctx).
		Where("cloud_space_id = ? AND device_id = ?", strings.TrimSpace(cloudSpaceID), strings.TrimSpace(deviceID)).
		First(&mapping).Error; err != nil {
		return "", fmt.Errorf("desktop pet owner mapping not found: %w", err)
	}
	if strings.TrimSpace(mapping.LocalOwnerID) == "" {
		return "", errors.New("desktop pet owner mapping is empty")
	}
	return mapping.LocalOwnerID, nil
}

type desktopPetBehaviorMeshRequest struct {
	CloudSpaceID string                         `json:"cloudSpaceId"`
	DeviceID     string                         `json:"deviceId"`
	Event        behavior.BehaviorEventEnvelope `json:"event"`
}

type desktopPetBehaviorMeshResponse struct {
	Accepted       bool   `json:"accepted"`
	EventID        string `json:"eventId"`
	LocalOwnerID   string `json:"localOwnerId"`
	InstallationID string `json:"installationId,omitempty"`
}

type desktopPetBehaviorTargetRequest struct {
	CloudSpaceID   string `json:"cloudSpaceId"`
	DeviceID       string `json:"deviceId"`
	CharacterID    string `json:"characterId"`
	InstallationID string `json:"installationId,omitempty"`
}

type desktopPetBehaviorTargetResponse struct {
	Active          bool   `json:"active"`
	LocalOwnerID    string `json:"localOwnerId,omitempty"`
	DeviceID        string `json:"deviceId"`
	CharacterID     string `json:"characterId"`
	InstallationID  string `json:"installationId,omitempty"`
	BindingRevision int64  `json:"bindingRevision,omitempty"`
	LastEnabledAt   string `json:"lastEnabledAt,omitempty"`
}

// desktopPetBehaviorMeshAffinity is CloudCore's last verified character ->
// device routing fact. It is advisory until the device re-validates it; it is
// never derived from websocket freshness.
type desktopPetBehaviorMeshAffinity struct {
	CloudSpaceID    string    `gorm:"column:cloud_space_id;primaryKey;size:128"`
	CharacterID     string    `gorm:"column:character_id;primaryKey;size:128"`
	DeviceID        string    `gorm:"column:device_id;not null;size:128"`
	InstallationID  string    `gorm:"column:installation_id;size:128"`
	BindingRevision int64     `gorm:"column:binding_revision;not null;default:0"`
	VerifiedAt      time.Time `gorm:"column:verified_at;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null"`
}

func (desktopPetBehaviorMeshAffinity) TableName() string {
	return "desktop_pet_behavior_mesh_affinities"
}

// desktopPetBehaviorMeshOutbox closes the Cloud -> Device reliability gap.
// Durable/recoverable events are accepted only after this row is committed.
type desktopPetBehaviorMeshOutbox struct {
	CloudSpaceID         string     `gorm:"column:cloud_space_id;primaryKey;size:128"`
	EventID              string     `gorm:"column:event_id;primaryKey;size:128"`
	CharacterID          string     `gorm:"column:character_id;size:128"`
	TargetDeviceID       string     `gorm:"column:target_device_id;size:128"`
	TargetInstallationID string     `gorm:"column:target_installation_id;size:128"`
	Reliability          string     `gorm:"column:reliability;not null;size:32"`
	PayloadJSON          string     `gorm:"column:payload_json;type:text;not null"`
	PayloadHash          string     `gorm:"column:payload_hash;type:text;not null"`
	Status               string     `gorm:"column:status;not null;size:32"`
	AttemptCount         int        `gorm:"column:attempt_count;not null;default:0"`
	AvailableAt          time.Time  `gorm:"column:available_at;not null"`
	ClaimExpiresAt       *time.Time `gorm:"column:claim_expires_at"`
	ExpiresAt            *time.Time `gorm:"column:expires_at"`
	LastError            string     `gorm:"column:last_error;type:text"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null"`
	DeliveredAt          *time.Time `gorm:"column:delivered_at"`
}

func (desktopPetBehaviorMeshOutbox) TableName() string {
	return "desktop_pet_behavior_mesh_outbox"
}

// desktopPetBehaviorMeshPublisher is installed on CloudCore in place of the
// local engine publisher. Execution remains on a Device Agent, while CloudCore
// owns durable delivery and verified character/device affinity.
type desktopPetBehaviorMeshPublisher struct {
	mu      sync.RWMutex
	runtime *devicemesh.Runtime
	db      *gorm.DB

	workerMu sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  atomic.Bool
}

func newDesktopPetBehaviorMeshPublisher(db *gorm.DB) (*desktopPetBehaviorMeshPublisher, error) {
	if db == nil {
		return nil, errors.New("desktop pet behavior mesh: db is nil")
	}
	if err := db.AutoMigrate(&desktopPetBehaviorMeshAffinity{}, &desktopPetBehaviorMeshOutbox{}); err != nil {
		return nil, fmt.Errorf("desktop pet behavior mesh schema: %w", err)
	}
	return &desktopPetBehaviorMeshPublisher{db: db}, nil
}

func (p *desktopPetBehaviorMeshPublisher) SetRuntime(runtime *devicemesh.Runtime) {
	p.mu.Lock()
	p.runtime = runtime
	p.mu.Unlock()
}

func (p *desktopPetBehaviorMeshPublisher) Start(ctx context.Context) error {
	if p == nil || p.db == nil {
		return errors.New("desktop pet behavior mesh unavailable")
	}
	p.mu.RLock()
	mesh := p.runtime
	p.mu.RUnlock()
	if mesh == nil || mesh.Hub == nil {
		return errors.New("desktop pet behavior mesh runtime unavailable")
	}
	p.workerMu.Lock()
	if p.running.Load() {
		p.workerMu.Unlock()
		return nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running.Store(true)
	p.wg.Add(1)
	p.workerMu.Unlock()
	go func() {
		defer p.wg.Done()
		defer p.running.Store(false)
		p.run(workerCtx)
	}()
	return nil
}

func (p *desktopPetBehaviorMeshPublisher) Stop() {
	if p == nil {
		return
	}
	p.workerMu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.workerMu.Unlock()
	if cancel != nil {
		cancel()
	}
	p.wg.Wait()
}

func (p *desktopPetBehaviorMeshPublisher) IsRunning() bool {
	return p != nil && p.running.Load()
}

func (p *desktopPetBehaviorMeshPublisher) PublishBehaviorEvent(ctx context.Context, event behavior.BehaviorEventEnvelope) error {
	if p == nil || p.db == nil {
		return errors.New("desktop pet behavior mesh unavailable")
	}
	cloudSpaceID := strings.TrimSpace(event.SpaceID)
	if cloudSpaceID == "" {
		return errors.New("desktop pet behavior mesh: event Space is required")
	}
	if strings.TrimSpace(event.EventID) == "" {
		return errors.New("desktop pet behavior mesh: event id is required")
	}
	if strings.TrimSpace(event.CharacterID) == "" {
		return errors.New("desktop pet behavior mesh: event character is required")
	}
	now := time.Now().UTC()
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = now
	}
	if strings.TrimSpace(event.DedupKey) == "" {
		event.DedupKey = event.EventID
	}

	reliability := behavior.GetReliability(event.EventType)
	if reliability == behavior.ReliabilityEphemeral {
		_, err := p.dispatchEvent(ctx, cloudSpaceID, event, "", "", nil)
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("desktop pet behavior mesh encode outbox: %w", err)
	}
	payloadSum := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(payloadSum[:])
	expiresAt := event.ExpiresAt
	if expiresAt == nil {
		expiresAt = behavior.ComputeExpiresAt(event.EventType, event.OccurredAt)
	}
	row := desktopPetBehaviorMeshOutbox{
		EventID:      event.EventID,
		CloudSpaceID: cloudSpaceID,
		CharacterID:  strings.TrimSpace(event.CharacterID),
		Reliability:  string(reliability),
		PayloadJSON:  string(payload),
		PayloadHash:  payloadHash,
		Status:       "pending",
		AvailableAt:  now,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	create := p.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if create.Error != nil {
		return fmt.Errorf("desktop pet behavior mesh persist outbox: %w", create.Error)
	}
	if create.RowsAffected == 0 {
		var existing desktopPetBehaviorMeshOutbox
		if err := p.db.WithContext(ctx).
			Where("cloud_space_id = ? AND event_id = ?", cloudSpaceID, event.EventID).
			First(&existing).Error; err != nil {
			return fmt.Errorf("desktop pet behavior mesh reload idempotent outbox row: %w", err)
		}
		existingHash := strings.TrimSpace(existing.PayloadHash)
		if existingHash == "" && existing.PayloadJSON != "" {
			sum := sha256.Sum256([]byte(existing.PayloadJSON))
			existingHash = hex.EncodeToString(sum[:])
		}
		if existingHash != payloadHash ||
			strings.TrimSpace(existing.CharacterID) != strings.TrimSpace(event.CharacterID) ||
			strings.TrimSpace(existing.Reliability) != string(reliability) {
			return errors.New("desktop pet behavior mesh: event id reused with different payload")
		}
	}

	// Best effort immediate delivery keeps latency low. Failure is intentionally
	// not returned after durable persistence; the worker owns retry semantics.
	if err := p.processEventID(ctx, cloudSpaceID, event.EventID); err != nil {
		log.Warn("desktop pet behavior mesh: queued for retry", map[string]interface{}{
			"eventId": event.EventID,
			"error":   err.Error(),
		})
	}
	return nil
}

func (p *desktopPetBehaviorMeshPublisher) run(ctx context.Context) {
	if err := p.processBatch(ctx); err != nil && ctx.Err() == nil {
		log.Warn("desktop pet behavior mesh initial retry batch failed", map[string]interface{}{"error": err.Error()})
	}
	ticker := time.NewTicker(behaviorMeshPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil && ctx.Err() == nil {
				log.Warn("desktop pet behavior mesh retry batch failed", map[string]interface{}{"error": err.Error()})
			}
		}
	}
}

func (p *desktopPetBehaviorMeshPublisher) processBatch(ctx context.Context) error {
	if p == nil || p.db == nil {
		return errors.New("desktop pet behavior mesh unavailable")
	}
	now := time.Now().UTC()
	// A process crash may leave a claimed row behind. Only an expired claim can
	// return to pending; live claims are never stolen by another worker.
	if err := p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
		Where("status = ? AND claim_expires_at IS NOT NULL AND claim_expires_at <= ?", "processing", now).
		Updates(map[string]interface{}{
			"status":           "pending",
			"claim_expires_at": nil,
			"available_at":     now,
			"updated_at":       now,
		}).Error; err != nil {
		return err
	}
	var rows []desktopPetBehaviorMeshOutbox
	if err := p.db.WithContext(ctx).
		Where("status = ? AND available_at <= ? AND (expires_at IS NULL OR expires_at > ?)", "pending", now, now).
		Order("available_at ASC, created_at ASC").
		Limit(behaviorMeshBatchSize).
		Find(&rows).Error; err != nil {
		return err
	}
	for i := range rows {
		if err := p.processOutboxRow(ctx, &rows[i]); err != nil && ctx.Err() == nil {
			continue
		}
	}
	// Expired rows are terminal and must not spin forever.
	return p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", "pending", now).
		Updates(map[string]interface{}{
			"status":     "expired",
			"last_error": "event TTL expired before device delivery",
			"updated_at": now,
		}).Error
}

func (p *desktopPetBehaviorMeshPublisher) processEventID(ctx context.Context, cloudSpaceID, eventID string) error {
	var row desktopPetBehaviorMeshOutbox
	if err := p.db.WithContext(ctx).
		Where("cloud_space_id = ? AND event_id = ?", strings.TrimSpace(cloudSpaceID), strings.TrimSpace(eventID)).
		First(&row).Error; err != nil {
		return err
	}
	if row.Status != "pending" {
		return nil
	}
	return p.processOutboxRow(ctx, &row)
}

func (p *desktopPetBehaviorMeshPublisher) processOutboxRow(ctx context.Context, row *desktopPetBehaviorMeshOutbox) error {
	if row == nil {
		return nil
	}
	now := time.Now().UTC()
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
			Where("cloud_space_id = ? AND event_id = ? AND status = ?", row.CloudSpaceID, row.EventID, "pending").
			Updates(map[string]interface{}{
				"status":     "expired",
				"last_error": "event TTL expired before device delivery",
				"updated_at": now,
			}).Error
	}
	claimExpiresAt := now.Add(behaviorMeshClaimLease)
	claim := p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
		Where("cloud_space_id = ? AND event_id = ? AND status = ? AND available_at <= ?", row.CloudSpaceID, row.EventID, "pending", now).
		Updates(map[string]interface{}{
			"status":           "processing",
			"claim_expires_at": claimExpiresAt,
			"updated_at":       now,
		})
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected == 0 {
		// Another worker already owns this event, or it is no longer due.
		return nil
	}
	row.Status = "processing"
	row.ClaimExpiresAt = &claimExpiresAt
	var event behavior.BehaviorEventEnvelope
	if err := json.Unmarshal([]byte(row.PayloadJSON), &event); err != nil {
		_ = p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
			Where("cloud_space_id = ? AND event_id = ? AND status = ?", row.CloudSpaceID, row.EventID, "processing").
			Updates(map[string]interface{}{
				"status":           "failed",
				"claim_expires_at": nil,
				"last_error":       "invalid persisted event payload: " + err.Error(),
				"updated_at":       now,
			}).Error
		return err
	}
	target, err := p.dispatchEvent(
		ctx,
		row.CloudSpaceID,
		event,
		row.TargetDeviceID,
		row.TargetInstallationID,
		func(resolved *desktopPetBehaviorTargetResponse) error {
			if resolved == nil || strings.TrimSpace(resolved.DeviceID) == "" || strings.TrimSpace(resolved.InstallationID) == "" {
				return errors.New("desktop pet behavior mesh: resolved target is incomplete")
			}
			pinAt := time.Now().UTC()
			pin := p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
				Where("cloud_space_id = ? AND event_id = ? AND status = ?", row.CloudSpaceID, row.EventID, "processing").
				Updates(map[string]interface{}{
					"target_device_id":       resolved.DeviceID,
					"target_installation_id": resolved.InstallationID,
					"updated_at":             pinAt,
				})
			if pin.Error != nil {
				return fmt.Errorf("persist behavior mesh target fence: %w", pin.Error)
			}
			if pin.RowsAffected != 1 {
				return errors.New("desktop pet behavior mesh: outbox claim lost before target fence persistence")
			}
			row.TargetDeviceID = resolved.DeviceID
			row.TargetInstallationID = resolved.InstallationID
			return nil
		},
	)
	if err == nil {
		deliveredAt := time.Now().UTC()
		return p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
			Where("cloud_space_id = ? AND event_id = ? AND status = ?", row.CloudSpaceID, row.EventID, "processing").
			Updates(map[string]interface{}{
				"status":                 "delivered",
				"claim_expires_at":       nil,
				"target_device_id":       target.DeviceID,
				"target_installation_id": target.InstallationID,
				"last_error":             "",
				"delivered_at":           deliveredAt,
				"updated_at":             deliveredAt,
			}).Error
	}
	attempt := row.AttemptCount + 1
	delay := time.Duration(attempt*attempt) * time.Second
	if delay < 2*time.Second {
		delay = 2 * time.Second
	}
	if delay > time.Minute {
		delay = time.Minute
	}
	updates := map[string]interface{}{
		"status":           "pending",
		"claim_expires_at": nil,
		"attempt_count":    attempt,
		"available_at":     now.Add(delay),
		"last_error":       err.Error(),
		"updated_at":       now,
	}
	if target != nil {
		if strings.TrimSpace(target.DeviceID) != "" {
			updates["target_device_id"] = target.DeviceID
		}
		if strings.TrimSpace(target.InstallationID) != "" {
			updates["target_installation_id"] = target.InstallationID
		}
	}
	if persistErr := p.db.WithContext(ctx).Model(&desktopPetBehaviorMeshOutbox{}).
		Where("cloud_space_id = ? AND event_id = ? AND status = ?", row.CloudSpaceID, row.EventID, "processing").
		Updates(updates).Error; persistErr != nil {
		return fmt.Errorf("behavior mesh dispatch failed (%v), persist retry failed: %w", err, persistErr)
	}
	return err
}

func (p *desktopPetBehaviorMeshPublisher) dispatchEvent(
	ctx context.Context,
	cloudSpaceID string,
	event behavior.BehaviorEventEnvelope,
	targetHint string,
	installationHint string,
	onResolved func(*desktopPetBehaviorTargetResponse) error,
) (*desktopPetBehaviorTargetResponse, error) {
	p.mu.RLock()
	mesh := p.runtime
	p.mu.RUnlock()
	if mesh == nil || mesh.Hub == nil {
		return nil, errors.New("desktop pet behavior mesh runtime unavailable")
	}
	spaceID := runtimeidentity.SpaceID(strings.TrimSpace(cloudSpaceID))
	if spaceID == "" {
		return nil, errors.New("desktop pet behavior mesh: cloud Space is required")
	}

	target, err := p.resolveTarget(ctx, mesh, spaceID, event, targetHint, installationHint)
	if err != nil {
		return nil, err
	}
	if target == nil || strings.TrimSpace(target.DeviceID) == "" || strings.TrimSpace(target.InstallationID) == "" {
		return target, errors.New("desktop pet behavior mesh: resolved target is incomplete")
	}
	// For durable outbox delivery the concrete device+installation fence is
	// persisted before the invoke. A process crash or transport uncertainty can
	// therefore never cause the same event to drift to a replacement pet.
	if onResolved != nil {
		if err := onResolved(target); err != nil {
			return target, err
		}
	}
	request := desktopPetBehaviorMeshRequest{
		CloudSpaceID: cloudSpaceID,
		DeviceID:     target.DeviceID,
		Event:        event,
	}
	request.Event.InstallationID = target.InstallationID
	input, err := json.Marshal(request)
	if err != nil {
		return target, fmt.Errorf("desktop pet behavior mesh encode: %w", err)
	}
	_, err = mesh.InvokeDeviceHandlerWithRuntimeType(
		ctx,
		spaceID,
		runtimeidentity.DeviceID(target.DeviceID),
		capability.RuntimeTypeInternal,
		desktopPetBehaviorMeshHandler,
		input,
		behaviorMeshInvokeTimeout,
	)
	if err != nil {
		return target, fmt.Errorf("desktop pet behavior mesh dispatch: %w", err)
	}
	return target, nil
}

func (p *desktopPetBehaviorMeshPublisher) resolveTarget(ctx context.Context, mesh *devicemesh.Runtime, spaceID runtimeidentity.SpaceID, event behavior.BehaviorEventEnvelope, targetHint, installationHint string) (*desktopPetBehaviorTargetResponse, error) {
	characterID := strings.TrimSpace(event.CharacterID)
	installationID := strings.TrimSpace(event.InstallationID)
	if installationID == "" {
		installationID = strings.TrimSpace(installationHint)
	}

	// Explicit persisted target is tried first. If online, re-validate it against
	// the device-local active binding before dispatch. If offline, keep affinity;
	// do not reroute the event to another heartbeat-fresher device.
	if target := strings.TrimSpace(targetHint); target != "" {
		if _, ok := mesh.Hub.GetByDevice(spaceID, runtimeidentity.DeviceID(target)); !ok {
			return nil, fmt.Errorf("desktop pet behavior mesh: target device %s is offline", target)
		}
		resolved, err := p.resolveOnDevice(ctx, mesh, spaceID, target, characterID, installationID)
		if err != nil {
			// A concrete outbox target is an execution fence. Transport or resolver
			// failures must retry this same device; never fall through to discovery.
			return nil, fmt.Errorf("desktop pet behavior mesh: validate pinned target %s: %w", target, err)
		}
		if resolved.Active {
			_ = p.saveAffinity(ctx, spaceID.String(), resolved)
			return resolved, nil
		}
		// The device authoritatively reports that the old installation/character
		// binding is no longer active. Invalidate the advisory affinity for future
		// events, but keep this already-targeted event pinned until TTL expiry.
		_ = p.invalidateAffinity(ctx, spaceID.String(), characterID, target)
		return nil, fmt.Errorf("desktop pet behavior mesh: pinned target device %s no longer owns the active installation", target)
	}

	if characterID != "" {
		var affinity desktopPetBehaviorMeshAffinity
		err := p.db.WithContext(ctx).
			Where("cloud_space_id = ? AND character_id = ?", spaceID.String(), characterID).
			First(&affinity).Error
		if err == nil && strings.TrimSpace(affinity.DeviceID) != "" {
			if _, ok := mesh.Hub.GetByDevice(spaceID, runtimeidentity.DeviceID(affinity.DeviceID)); ok {
				resolved, resolveErr := p.resolveOnDevice(ctx, mesh, spaceID, affinity.DeviceID, characterID, installationID)
				if resolveErr != nil {
					// Do not erase a verified affinity because of a transient RPC failure;
					// retrying preserves device ownership instead of routing by availability.
					return nil, fmt.Errorf("desktop pet behavior mesh: validate affinity device %s: %w", affinity.DeviceID, resolveErr)
				}
				if resolved.Active {
					_ = p.saveAffinity(ctx, spaceID.String(), resolved)
					return resolved, nil
				}
				// An online device has authoritatively denied the old binding. It is
				// safe to discard this advisory affinity before discovery.
				_ = p.invalidateAffinity(ctx, spaceID.String(), characterID, affinity.DeviceID)
			}
			// An offline advisory affinity is not yet a physical event target. Keep
			// it for future evidence, but allow discovery of another online device
			// that now owns the character. Once an outbox row has target_device_id,
			// the targetHint branch above pins that concrete event and never reroutes.
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	connections := mesh.Hub.ListBySpace(spaceID)
	if len(connections) == 0 {
		return nil, errors.New("desktop pet behavior mesh: no online device agent")
	}
	candidates := make([]desktopPetBehaviorTargetResponse, 0, len(connections))
	for _, conn := range connections {
		if conn == nil {
			continue
		}
		resolved, err := p.resolveOnDevice(ctx, mesh, spaceID, conn.DeviceID.String(), characterID, installationID)
		if err != nil || !resolved.Active {
			continue
		}
		candidates = append(candidates, *resolved)
	}
	if len(candidates) == 0 {
		return nil, errors.New("desktop pet behavior mesh: no device has an active installation for the character")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		li, lj := candidates[i].LastEnabledAt, candidates[j].LastEnabledAt
		if li != lj {
			return li > lj
		}
		if candidates[i].BindingRevision != candidates[j].BindingRevision {
			return candidates[i].BindingRevision > candidates[j].BindingRevision
		}
		if candidates[i].DeviceID != candidates[j].DeviceID {
			return candidates[i].DeviceID < candidates[j].DeviceID
		}
		return candidates[i].InstallationID < candidates[j].InstallationID
	})
	selected := candidates[0]
	if err := p.saveAffinity(ctx, spaceID.String(), &selected); err != nil {
		return nil, err
	}
	return &selected, nil
}

func (p *desktopPetBehaviorMeshPublisher) resolveOnDevice(ctx context.Context, mesh *devicemesh.Runtime, spaceID runtimeidentity.SpaceID, deviceID, characterID, installationID string) (*desktopPetBehaviorTargetResponse, error) {
	request := desktopPetBehaviorTargetRequest{
		CloudSpaceID:   spaceID.String(),
		DeviceID:       strings.TrimSpace(deviceID),
		CharacterID:    strings.TrimSpace(characterID),
		InstallationID: strings.TrimSpace(installationID),
	}
	input, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	result, err := mesh.InvokeDeviceHandlerWithRuntimeType(
		ctx,
		spaceID,
		runtimeidentity.DeviceID(request.DeviceID),
		capability.RuntimeTypeInternal,
		desktopPetBehaviorMeshResolveHandler,
		input,
		behaviorMeshInvokeTimeout,
	)
	if err != nil {
		return nil, err
	}
	if len(result.Structured) == 0 {
		return nil, errors.New("desktop pet behavior mesh: target resolver returned empty result")
	}
	var response desktopPetBehaviorTargetResponse
	if err := json.Unmarshal(result.Structured, &response); err != nil {
		return nil, fmt.Errorf("desktop pet behavior mesh decode target: %w", err)
	}
	if response.DeviceID == "" {
		response.DeviceID = request.DeviceID
	}
	return &response, nil
}

func (p *desktopPetBehaviorMeshPublisher) saveAffinity(ctx context.Context, cloudSpaceID string, target *desktopPetBehaviorTargetResponse) error {
	if p == nil || p.db == nil || target == nil || !target.Active || strings.TrimSpace(target.CharacterID) == "" {
		return nil
	}
	now := time.Now().UTC()
	row := desktopPetBehaviorMeshAffinity{
		CloudSpaceID:    strings.TrimSpace(cloudSpaceID),
		CharacterID:     strings.TrimSpace(target.CharacterID),
		DeviceID:        strings.TrimSpace(target.DeviceID),
		InstallationID:  strings.TrimSpace(target.InstallationID),
		BindingRevision: target.BindingRevision,
		VerifiedAt:      now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return p.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cloud_space_id"}, {Name: "character_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"device_id":        row.DeviceID,
			"installation_id":  row.InstallationID,
			"binding_revision": row.BindingRevision,
			"verified_at":      now,
			"updated_at":       now,
		}),
	}).Create(&row).Error
}

func (p *desktopPetBehaviorMeshPublisher) invalidateAffinity(ctx context.Context, cloudSpaceID, characterID, deviceID string) error {
	if p == nil || p.db == nil || strings.TrimSpace(characterID) == "" {
		return nil
	}
	return p.db.WithContext(ctx).
		Where("cloud_space_id = ? AND character_id = ? AND device_id = ?", strings.TrimSpace(cloudSpaceID), strings.TrimSpace(characterID), strings.TrimSpace(deviceID)).
		Delete(&desktopPetBehaviorMeshAffinity{}).Error
}

func resolveDeviceActivePet(ctx context.Context, services *AppServices, cloudSpaceID, deviceID, characterID, installationHint string) (*desktopPetBehaviorTargetResponse, error) {
	if services == nil || services.DesktopPetOwnerMapper == nil || services.InstallationRepo == nil || services.DB == nil {
		return nil, errors.New("desktop pet behavior target resolver unavailable")
	}
	cloudSpaceID = strings.TrimSpace(cloudSpaceID)
	deviceID = strings.TrimSpace(deviceID)
	characterID = strings.TrimSpace(characterID)
	installationHint = strings.TrimSpace(installationHint)
	localOwnerID, err := services.DesktopPetOwnerMapper.Resolve(ctx, cloudSpaceID, deviceID)
	if err != nil {
		return nil, err
	}

	active := &desktopPetBehaviorTargetResponse{
		Active:       false,
		LocalOwnerID: localOwnerID,
		DeviceID:     deviceID,
		CharacterID:  characterID,
	}
	bindingEntry, err := services.InstallationRepo.GetActiveBindingForSpaceDeviceTx(services.DB.WithContext(ctx), localOwnerID, deviceID)
	if err != nil {
		if errors.Is(err, installation.ErrBindingNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return active, nil
		}
		return nil, err
	}
	if bindingEntry == nil || !bindingEntry.IsValid() {
		return active, nil
	}
	if installationHint != "" && bindingEntry.InstallationID != installationHint {
		return active, nil
	}
	inst, err := services.InstallationRepo.GetInstallationForSpaceDevice(localOwnerID, deviceID, bindingEntry.InstallationID)
	if err != nil {
		if errors.Is(err, installation.ErrInstallationNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return active, nil
		}
		return nil, err
	}
	if inst == nil || inst.IsActive != 1 || inst.DesiredState != installation.DesiredEnabled || inst.Status != installation.StatusEnabled {
		return active, nil
	}
	active.Active = true
	active.CharacterID = characterID
	active.InstallationID = inst.ID
	active.BindingRevision = bindingEntry.BindingRevision
	active.LastEnabledAt = strings.TrimSpace(inst.LastEnabledAt)
	return active, nil
}

func newDesktopPetBehaviorMeshResolveHandler(services *AppServices) devicemeshagent.CancellableRuntimeInvokeHandler {
	return func(ctx context.Context, invoke deviceruntimeprotocol.RuntimeInvokePayload) (*deviceruntimeprotocol.RuntimeResultPayload, error) {
		var request desktopPetBehaviorTargetRequest
		if err := json.Unmarshal(invoke.Input, &request); err != nil {
			return nil, fmt.Errorf("desktop pet behavior target decode: %w", err)
		}
		if strings.TrimSpace(request.CloudSpaceID) == "" || request.CloudSpaceID != invoke.SpaceID.String() {
			return nil, errors.New("desktop pet behavior target cloud owner mismatch")
		}
		if strings.TrimSpace(request.DeviceID) == "" || request.DeviceID != invoke.DeviceID.String() {
			return nil, errors.New("desktop pet behavior target device mismatch")
		}
		resolved, err := resolveDeviceActivePet(ctx, services, request.CloudSpaceID, request.DeviceID, request.CharacterID, request.InstallationID)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(resolved)
		if err != nil {
			return nil, err
		}
		return &deviceruntimeprotocol.RuntimeResultPayload{
			InvocationID:         invoke.InvocationID,
			RuntimeSessionID:     invoke.RuntimeSessionID,
			ConnectionGeneration: invoke.ConnectionGeneration,
			DeviceID:             invoke.DeviceID,
			RuntimeID:            invoke.RuntimeID,
			Status:               string(capability.ToolResultStatusSuccess),
			Result:               encoded,
			CompletedAt:          time.Now().UTC(),
		}, nil
	}
}

func newDesktopPetBehaviorMeshInvokeHandler(services *AppServices) devicemeshagent.CancellableRuntimeInvokeHandler {
	return func(ctx context.Context, invoke deviceruntimeprotocol.RuntimeInvokePayload) (*deviceruntimeprotocol.RuntimeResultPayload, error) {
		if services == nil || services.BehaviorService == nil || services.DesktopPetOwnerMapper == nil {
			return nil, errors.New("desktop pet behavior mesh handler unavailable")
		}
		var request desktopPetBehaviorMeshRequest
		if err := json.Unmarshal(invoke.Input, &request); err != nil {
			return nil, fmt.Errorf("desktop pet behavior mesh decode: %w", err)
		}
		if strings.TrimSpace(request.CloudSpaceID) == "" || request.CloudSpaceID != invoke.SpaceID.String() {
			return nil, errors.New("desktop pet behavior mesh cloud owner mismatch")
		}
		if strings.TrimSpace(request.DeviceID) == "" || request.DeviceID != invoke.DeviceID.String() {
			return nil, errors.New("desktop pet behavior mesh device mismatch")
		}
		resolved, err := resolveDeviceActivePet(ctx, services, request.CloudSpaceID, request.DeviceID, request.Event.CharacterID, request.Event.InstallationID)
		if err != nil {
			return nil, err
		}
		if resolved == nil || !resolved.Active {
			return nil, errors.New("desktop pet behavior mesh active installation affinity mismatch")
		}
		request.Event.SpaceID = resolved.LocalOwnerID
		request.Event.InstallationID = resolved.InstallationID
		if err := services.BehaviorService.SubmitEvent(ctx, request.Event); err != nil {
			return nil, fmt.Errorf("desktop pet behavior submit: %w", err)
		}
		encoded, err := json.Marshal(desktopPetBehaviorMeshResponse{
			Accepted:       true,
			EventID:        request.Event.EventID,
			LocalOwnerID:   resolved.LocalOwnerID,
			InstallationID: resolved.InstallationID,
		})
		if err != nil {
			return nil, err
		}
		return &deviceruntimeprotocol.RuntimeResultPayload{
			InvocationID:         invoke.InvocationID,
			RuntimeSessionID:     invoke.RuntimeSessionID,
			ConnectionGeneration: invoke.ConnectionGeneration,
			DeviceID:             invoke.DeviceID,
			RuntimeID:            invoke.RuntimeID,
			Status:               string(capability.ToolResultStatusSuccess),
			Result:               encoded,
			CompletedAt:          time.Now().UTC(),
		}, nil
	}
}

func bindDesktopPetOwnerFromCredential(ctx context.Context, services *AppServices, cloudSpaceID, deviceID string) error {
	if services == nil || services.DesktopPetOwnerMapper == nil {
		return errors.New("desktop pet owner mapper unavailable")
	}
	localOwnerID := strings.TrimSpace(spaceidentity.DefaultSpaceID())
	if localOwnerID == "" {
		return errors.New("desktop pet local owner id is not configured")
	}
	return services.DesktopPetOwnerMapper.Bind(ctx, cloudSpaceID, deviceID, localOwnerID)
}
