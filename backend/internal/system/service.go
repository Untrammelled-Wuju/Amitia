// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"encoding/json"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/internal/temporal"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	UsageContext(context.Context, mindruntime.UsageFilter, string) (map[string]any, error)
	ClearUsageContext(context.Context, mindruntime.UsageFilter) (map[string]any, error)
	UpdateTimeoutSettings(timeoutpolicy.Settings) error
	AppConfig() map[string]interface{}
	CheckDBIntegrity() map[string]interface{}
	CheckStorageMigrations() map[string]interface{}
	CheckUpdate() map[string]interface{}
	CleanupTemp() map[string]interface{}
	ConfigExport() map[string]interface{}
	ConfigImportPreviewService(body map[string]interface{}) map[string]interface{}
	ConfigImportConfirmService(body map[string]interface{}) map[string]interface{}
	ConfigSettings() map[string]interface{}
	DeleteLogs() map[string]interface{}
	DeleteLogsModelErrors() map[string]interface{}
	Diagnostics() map[string]interface{}
	ExportReleaseCheck() map[string]interface{}
	GetAuditActions() []string
	GetAuditLogs(limit int) []auditLogRecord
	ClearAuditLogs() int64
	GetAuditSettings() map[string]interface{}
	GetAuditStats() map[string]interface{}
	GetAbout() map[string]interface{}
	GetLogsFileContent(name string) string
	GetLogsFiles() map[string]interface{}
	GetLogsModelErrors() map[string]interface{}
	GetLogsPromptTraces(limit int) map[string]interface{}
	GetLogsRecent(limit int) map[string]interface{}
	GetLogsRecentErrors(limit int) map[string]interface{}
	GetLongRunningConfig() map[string]interface{}
	GetLongRunningStatus() map[string]interface{}
	GetMaintenanceStatus() map[string]interface{}
	GetPrivacyScanResult(id string) map[string]interface{}
	GetReleaseCheckHistory() map[string]interface{}
	GetReleaseCheckLatest() map[string]interface{}
	GetRuntimeHealth() map[string]interface{}
	GetRuntimeHealthHistory() map[string]interface{}
	GetIdentityCore(characterID string) map[string]interface{}
	GetRuntimeMode() map[string]interface{}
	GetRuntimeStatus() map[string]interface{}
	GetSecurityAccessConfig() map[string]interface{}
	GetSecurityAccessStatus() map[string]interface{}
	GetSecurityStatus() map[string]interface{}
	GetStorageInfo() map[string]interface{}
	GetStorageMigrations() map[string]interface{}
	GetTheme() map[string]interface{}
	GetThemePresets() map[string]interface{}
	GetUpdateConfig() map[string]interface{}
	GetVersion() map[string]interface{}
	Health() map[string]interface{}
	MaintenanceDiagnose() map[string]interface{}
	MaintenanceExportDiagnostic() map[string]interface{}
	MaintenanceReloadConfig() map[string]interface{}
	MoodDetectionConfig() map[string]interface{}
	UpdateMoodDetectionConfig(body map[string]interface{}) map[string]interface{}
	OnboardingComplete() map[string]interface{}
	OnboardingReset() map[string]interface{}
	OnboardingStatus() map[string]interface{}
	PrivacyScanResults() map[string]interface{}
	RotateLogs() map[string]interface{}
	RunDiagnostics() map[string]interface{}
	RunNow() map[string]interface{}
	RunReleaseCheck() map[string]interface{}
	SafetyEvents(context.Context, safety.PolicyScope, int, int, string) ([]safety.Incident, int64, error)
	DeleteSafetyEvents(context.Context, safety.PolicyScope) (int64, error)
	HandleSafetyEvent(context.Context, safety.PolicyScope, string, int64) (safety.Incident, error)
	SecurityIdentityCheck() map[string]interface{}
	SecurityExposureCheck() map[string]interface{}
	SetupChecks() map[string]interface{}
	SetupFinish() map[string]interface{}
	SetupReset() map[string]interface{}
	SetupStatus() map[string]interface{}
	SetupStep(step string) map[string]interface{}
	ToolRoute(body map[string]interface{}) map[string]interface{}
	UpdateAppConfig(body map[string]interface{}) map[string]interface{}
	UpdateAuditSettings(body map[string]interface{}) map[string]interface{}
	UpdateLongRunningConfig(body map[string]interface{}) map[string]interface{}
	ValidateIdentityCorePatch(characterID string, body map[string]interface{}) map[string]interface{}
	UpdateRuntimeMode(body map[string]interface{}) map[string]interface{}
	UpdateSecurityAccessConfig(body map[string]interface{}) map[string]interface{}
	UpdateTheme(body map[string]interface{}) map[string]interface{}
	UpdateUpdateConfig(body map[string]interface{}) map[string]interface{}
	ValidateMode() map[string]interface{}
	AttachTemporalService(temporalSvc *temporal.Service)
	SetDataPortabilityCoordinator(coord *dataportability.Coordinator)
	GetDataPortabilityCoordinator() (*dataportability.Coordinator, bool)
}

type service struct {
	usageStore           *mindruntime.ModelUsageStore
	memoryConsumer       business.MemoryConsumerPort
	db                   *gorm.DB
	startTime            time.Time
	healthLog            []map[string]interface{}
	dataDir              string
	temporalSvc          *temporal.Service
	coordinator          *dataportability.Coordinator
	runtimeProfile       runtimeprofile.Profile
	privacyMu            sync.RWMutex
	privacyScans         []map[string]interface{}
	privacyMessages      map[string]coordination.PrivacyMessage
	privacyMemoryEntries map[string]business.MemoryConsumerEntry
	safetyRepository     *safety.Repository
}

func NewService(ctx *app.AppContext, profile runtimeprofile.Profile) Service {
	dataDir := "data"
	if config.AppCfg != nil && config.AppCfg.Storage.DataDir != "" {
		dataDir = config.AppCfg.Storage.DataDir
	}
	s := &service{db: ctx.DB, startTime: time.Now(), dataDir: dataDir, runtimeProfile: profile, safetyRepository: safety.NewRepository(ctx.DB)}
	s.loadTimeoutSettings()
	return s
}

func (s *service) AttachTemporalService(temporalSvc *temporal.Service) {
	s.temporalSvc = temporalSvc
}

func (s *service) SetDataPortabilityCoordinator(coord *dataportability.Coordinator) {
	s.coordinator = coord
	if coord != nil && coord.DataDir != "" {
		s.dataDir = coord.DataDir
	}
}

func (s *service) GetDataPortabilityCoordinator() (*dataportability.Coordinator, bool) {
	if s.coordinator == nil {
		return nil, false
	}
	return s.coordinator, true
}
func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case json.Number:
		f, _ := val.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case json.Number:
		i, _ := val.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(val)
		return i
	}
	return 0
}

func readEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
