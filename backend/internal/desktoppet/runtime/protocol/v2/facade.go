// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package v2

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type FacadeConfig struct {
	Enabled            bool
	Path               string
	LoopbackOnly       bool
	HeartbeatInterval  time.Duration
	HeartbeatTimeout   time.Duration
	MaxMessageBytes    int64
	CommandTimeoutSec  int64
	CommandRetentionHr int64
}

func DefaultFacadeConfig() *FacadeConfig {
	return &FacadeConfig{
		Enabled:            true,
		Path:               "/internal/desktop-pet/runtime/ws",
		LoopbackOnly:       true,
		HeartbeatInterval:  10 * time.Second,
		HeartbeatTimeout:   30 * time.Second,
		MaxMessageBytes:    1048576,
		CommandTimeoutSec:  30,
		CommandRetentionHr: 24,
	}
}

type RuntimeFacade struct {
	db         *gorm.DB
	config     *FacadeConfig
	services   *Services
	handler    *Handler
	states     ActualStateService
	reconciler *Reconciler

	deviceRuntimeSessions *deviceruntime.Service

	started     atomic.Bool
	cancel      context.CancelFunc
	lifecycleMu sync.Mutex
}

func NewRuntimeFacade(db *gorm.DB, config *FacadeConfig) *RuntimeFacade {
	return NewRuntimeFacadeWithDeviceRuntime(db, config, nil)
}

func NewRuntimeFacadeWithDeviceRuntime(db *gorm.DB, config *FacadeConfig, deviceRuntimeSessions *deviceruntime.Service) *RuntimeFacade {
	if config == nil {
		config = DefaultFacadeConfig()
	}
	svc := NewServices(db)
	handler := NewHandler(svc)
	if deviceRuntimeSessions != nil {
		handler = NewHandlerWithDeviceRuntime(svc, deviceRuntimeSessions)
	}
	states := svc.ActualStates
	reconciler := NewReconciler(svc, handler)
	return &RuntimeFacade{
		db:         db,
		config:     config,
		services:   svc,
		handler:    handler,
		states:     states,
		reconciler: reconciler,
		deviceRuntimeSessions: deviceRuntimeSessions,
	}
}

func (f *RuntimeFacade) DeviceRuntimeSessions() *deviceruntime.Service {
	return f.deviceRuntimeSessions
}

func (f *RuntimeFacade) Start(ctx context.Context) error {
	f.lifecycleMu.Lock()
	defer f.lifecycleMu.Unlock()

	if f.started.Load() {
		return nil
	}
	if !f.config.Enabled {
		return nil
	}

	fctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	f.started.Store(true)
	log.Info("[v2-runtime-facade] started")

	go f.runReconciler(fctx)
	go f.runRetentionGC(fctx)
	return nil
}

func (f *RuntimeFacade) Close(ctx context.Context) error {
	f.lifecycleMu.Lock()
	defer f.lifecycleMu.Unlock()

	if !f.started.Load() {
		return nil
	}
	f.started.Store(false)
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
	log.Info("[v2-runtime-facade] stopped")
	return nil
}

func (f *RuntimeFacade) IsStarted() bool {
	return f != nil && f.started.Load()
}

func (f *RuntimeFacade) Config() *FacadeConfig {
	return f.config
}

func (f *RuntimeFacade) Handler() *Handler {
	return f.handler
}

func (f *RuntimeFacade) StateService() ActualStateService {
	return f.states
}

func (f *RuntimeFacade) Sessions() SessionService {
	return f.services.Sessions
}

func (f *RuntimeFacade) Commands() CommandService {
	return f.services.Commands
}

func (f *RuntimeFacade) Events() EventService {
	return f.services.Events
}

func (f *RuntimeFacade) HandlerServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !f.config.Enabled {
		http.Error(w, "runtime disabled", http.StatusServiceUnavailable)
		return
	}
	http.Error(w, "v2 handler does not implement WS directly", http.StatusNotImplemented)
}

func (f *RuntimeFacade) runReconciler(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expired, err := f.reconciler.ExpireCommands(time.Now())
			if err != nil {
				log.Warn("[v2-runtime-facade] reconciler expire failed: ", err)
			} else if expired > 0 {
				log.Infof("[v2-runtime-facade] expired %d commands", expired)
			}
		}
	}
}

func (f *RuntimeFacade) runRetentionGC(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := f.states.FailAbandonedEvents(time.Now().Add(-10 * time.Minute))
			if err != nil {
				log.Warn("[v2-runtime-facade] retention gc failed: ", err)
			} else if count > 0 {
				log.Infof("[v2-runtime-facade] retention gc removed %d events", count)
			}
		}
	}
}

func (f *RuntimeFacade) ListConnections(userID string) []*Connection {
	f.handler.mu.RLock()
	defer f.handler.mu.RUnlock()
	var result []*Connection
	for _, conn := range f.handler.connections {
		if conn != nil && conn.UserID == userID {
			result = append(result, conn)
		}
	}
	return result
}

func (f *RuntimeFacade) GetConnection(userID, deviceID, runtimeID string) *Connection {
	return f.handler.GetConnection(
		runtimeidentity.ParseUserID(userID),
		runtimeidentity.ParseDeviceID(deviceID),
		runtimeidentity.ParseRuntimeID(runtimeID),
	)
}

type V2RuntimeNotifier struct {
	states ActualStateService
	events EventService
}

func NewV2RuntimeNotifier(states ActualStateService, events EventService) *V2RuntimeNotifier {
	return &V2RuntimeNotifier{states: states, events: events}
}

func (n *V2RuntimeNotifier) NotifyInstallationEnabled(userId, installationId string, settings *installation.RuntimeSettings) error {
	payload := map[string]interface{}{
		"type":            "installation.enabled",
		"userId":          userId,
		"installationId":  installationId,
		"settings":        settings,
		"desiredRevision": time.Now().UnixNano(),
	}
	return n.emit("installation.enabled", installationId, payload)
}

func (n *V2RuntimeNotifier) NotifyInstallationDisabled(userId, installationId string) error {
	payload := map[string]interface{}{
		"type":            "installation.disabled",
		"userId":          userId,
		"installationId":  installationId,
		"desiredRevision": time.Now().UnixNano(),
	}
	return n.emit("installation.disabled", installationId, payload)
}

func (n *V2RuntimeNotifier) NotifyActionPlayed(userId, installationId, actionKey string) error {
	payload := map[string]interface{}{
		"type":           "action.played",
		"userId":         userId,
		"installationId": installationId,
		"actionKey":      actionKey,
	}
	return n.emit("action.played", installationId, payload)
}

func (n *V2RuntimeNotifier) NotifyRecenter(installationId string) error {
	payload := map[string]interface{}{
		"type":           "installation.recenter",
		"installationId": installationId,
	}
	return n.emit("installation.recenter", installationId, payload)
}

func (n *V2RuntimeNotifier) NotifyDefaultActionChanged(installationId, actionKey string) error {
	payload := map[string]interface{}{
		"type":           "installation.defaultActionChanged",
		"installationId": installationId,
		"actionKey":      actionKey,
	}
	return n.emit("installation.defaultActionChanged", installationId, payload)
}

func (n *V2RuntimeNotifier) NotifyRuntimeSettingsUpdated(installationId string, settings *installation.RuntimeSettings) error {
	payload := map[string]interface{}{
		"type":           "installation.settingsUpdated",
		"installationId": installationId,
		"settings":       settings,
	}
	return n.emit("installation.settingsUpdated", installationId, payload)
}

func (n *V2RuntimeNotifier) emit(eventType, aggregateID string, payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	idemKey := eventType + "_" + aggregateID + "_" + uuid.NewString()
	_, err = n.states.AppendDomainEvent(eventType, aggregateID, body, time.Now(), &idemKey)
	if err != nil {
		log.Warn("[v2-notifier] emit failed: ", err)
	}
	return err
}
