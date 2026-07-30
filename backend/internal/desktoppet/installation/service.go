// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

const (
	runtimeSettingsIDPrefix                      = "rts_"
	runtimeSettingsDefaultScale                  = 1.0
	runtimeSettingsDefaultIdleIntervalMinSeconds = 30
	runtimeSettingsDefaultIdleIntervalMaxSeconds = 120
	runtimeSettingsDefaultClickThroughMode       = "off"
)

type RuntimeNotifier interface {
	NotifyInstallationEnabled(userId, installationId string, settings *RuntimeSettings) error
	NotifyInstallationDisabled(userId, installationId string) error
	NotifyActionPlayed(userId, installationId, actionKey string) error
	NotifyRecenter(installationId string) error
	NotifyDefaultActionChanged(installationId, actionKey string) error
	NotifyRuntimeSettingsUpdated(installationId string, settings *RuntimeSettings) error
}

type Service interface {
	InstallPackage(packageId, userId, characterId string) (*Installation, error)
	Uninstall(userId, installationId string) error
	EnableInstallation(userId, installationId string) error
	DisableInstallation(userId, installationId string) error
	SwitchInstallation(userId, installationId string) error
	UpdateDefaultAction(installationId, actionKey string) error
	UpdateRuntimeSettings(userId, installationId string, req *UpdateRuntimeSettingsRequest) (*RuntimeSettings, error)
	Recenter(userId, installationId string) error
	PlayAction(userId, installationId, actionKey string) error
	ListInstallations(userId string) ([]*Installation, error)
	GetInstallation(installationId string) (*Installation, error)
	GetRuntimeSettings(installationId string) (*RuntimeSettings, error)
}

type service struct {
	repo        Repository
	installer   Installer
	uninstaller Uninstaller
	packageRepo processing.Repository
	charRepo    character.Repository
	dataDir     string
	notifier    RuntimeNotifier
}

type ServiceOption func(*service)

func WithRuntimeNotifier(notifier RuntimeNotifier) ServiceOption {
	return func(s *service) {
		s.notifier = notifier
	}
}

func NewService(repo Repository, installer Installer, uninstaller Uninstaller, packageRepo processing.Repository, charRepo character.Repository, dataDir string, opts ...ServiceOption) Service {
	s := &service{
		repo:        repo,
		installer:   installer,
		uninstaller: uninstaller,
		packageRepo: packageRepo,
		charRepo:    charRepo,
		dataDir:     dataDir,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Deprecated: 使用 WithRuntimeNotifier 选项通过 NewService 注入。
func SetRuntimeNotifier(svc Service, notifier RuntimeNotifier) bool {
	s, ok := svc.(*service)
	if !ok {
		return false
	}
	s.notifier = notifier
	return true
}

func (s *service) InstallPackage(packageId, userId, characterId string) (*Installation, error) {
	return s.installer.InstallPackage(packageId, userId, characterId)
}

func (s *service) Uninstall(userId, installationId string) error {
	return s.uninstaller.Uninstall(userId, installationId)
}

func (s *service) EnableInstallation(userId, installationId string) error {
	if userId == "" || installationId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 或安装 ID 为空", ErrInstallationInvalid)
	}
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if inst.UserID != userId {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if inst.Status == StatusEnabled {
		settings, err := s.ensureRuntimeSettings(installationId)
		if err != nil {
			return err
		}
		if err := s.notifyEnabled(userId, installationId, settings); err != nil {
			if isRuntimeOfflineError(err) {
				log.Logger.Warnf("installation: NotifyInstallationEnabled 运行时离线 pending_sync installationId=%s", installationId)
			} else {
				return NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
			}
		}
		return nil
	}
	if !inst.CanEnable() {
		return NewInstallationError(ErrCodeInstallationInvalid,
			fmt.Sprintf("安装状态 %s 不可启用", inst.Status), ErrInstallationInvalid)
	}
	if err := s.validateEnablePrerequisites(inst); err != nil {
		return err
	}
	if err := s.repo.SetActiveInstallation(userId, installationId); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "设置活跃安装失败", err)
	}
	if err := s.repo.UpdateInstallationStatus(installationId, StatusEnabled); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "更新安装状态失败", err)
	}
	settings, err := s.ensureRuntimeSettings(installationId)
	if err != nil {
		return err
	}
	if err := s.notifyEnabled(userId, installationId, settings); err != nil {
		if isRuntimeOfflineError(err) {
			log.Logger.Warnf("installation: NotifyInstallationEnabled 运行时离线 pending_sync installationId=%s", installationId)
		} else {
			return NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
		}
	}
	return nil
}

func (s *service) validateEnablePrerequisites(inst *Installation) error {
	installDir := s.absPath(inst.InstallPath)
	if installDir == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装路径为空", ErrInstallationInvalid)
	}
	info, err := os.Stat(installDir)
	if err != nil || !info.IsDir() {
		return NewInstallationError(ErrCodeInstallationFailed, "安装目录不存在", err)
	}
	manifest, err := s.readManifest(inst.ManifestPath)
	if err != nil {
		return err
	}
	defaultFound := false
	for _, action := range manifest.Actions {
		if action.Key == manifest.DefaultAction {
			defaultFound = true
			break
		}
	}
	if !defaultFound {
		return NewInstallationError(ErrCodePackageDefaultActionInvalid,
			fmt.Sprintf("默认动作 %s 不在 actions 列表中", manifest.DefaultAction),
			ErrPackageDefaultActionInvalid)
	}
	pkg, err := s.packageRepo.GetPackage(inst.PackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewInstallationError(ErrCodePackageNotReady, "源资源包记录不存在", ErrPackageNotReady)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询源资源包失败", err)
	}
	if pkg.Status != "ready" && pkg.Status != "succeeded" {
		return NewInstallationError(ErrCodePackageNotReady,
			fmt.Sprintf("源资源包状态为 %s，非 ready", pkg.Status), ErrPackageNotReady)
	}
	actualHash, err := s.computePackageHash(installDir)
	if err != nil {
		return NewInstallationError(ErrCodePackageHashMismatch, "重新计算包哈希失败", err)
	}
	if actualHash != inst.PackageHash {
		return NewInstallationError(ErrCodePackageHashMismatch,
			fmt.Sprintf("包哈希不匹配: 期望 %s, 实际 %s", inst.PackageHash, actualHash),
			ErrPackageHashMismatch)
	}
	if _, err := s.charRepo.FindByID(inst.CharacterID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewInstallationError(ErrCodeCharacterNotFound, "角色不存在", ErrCharacterNotFound)
		}
		return NewInstallationError(ErrCodeCharacterNotFound, "校验角色失败", err)
	}
	return nil
}

func (s *service) ensureRuntimeSettings(installationId string) (*RuntimeSettings, error) {
	existing, err := s.repo.GetRuntimeSettings(installationId)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrRuntimeSettingsNotFound) {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "查询运行时设置失败", err)
	}
	now := time.Now().Format(installationTimeFormat)
	settings := &RuntimeSettings{
		ID:                     runtimeSettingsIDPrefix + uuid.New().String(),
		InstallationID:         installationId,
		AlwaysOnTop:            1,
		LaunchOnStartup:        0,
		Scale:                  runtimeSettingsDefaultScale,
		PositionX:              0,
		PositionY:              0,
		ScreenID:               "",
		IdleEnabled:            1,
		IdleIntervalMinSeconds: runtimeSettingsDefaultIdleIntervalMinSeconds,
		IdleIntervalMaxSeconds: runtimeSettingsDefaultIdleIntervalMaxSeconds,
		ClickThroughMode:       runtimeSettingsDefaultClickThroughMode,
		SoundEnabled:           0,
		SettingsRevision:       0,
		RestoreOnAppStart:      1,
		PositionMode:           positionModeAbsolute,
		DisplayFingerprint:     "",
		RelativeX:              0.5,
		RelativeY:              0.5,
		LastWindowWidth:        0,
		LastWindowHeight:       0,
		PositionUpdatedAt:      "",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := s.repo.CreateRuntimeSettings(settings); err != nil {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "创建运行时设置失败", err)
	}
	return settings, nil
}

func (s *service) DisableInstallation(userId, installationId string) error {
	if userId == "" || installationId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 或安装 ID 为空", ErrInstallationInvalid)
	}
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if inst.UserID != userId {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if inst.Status == StatusUninstalled || inst.Status == StatusUninstalling {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录已卸载", ErrInstallationInvalid)
	}
	if inst.Status == StatusDisabled {
		if inst.IsActivated() {
			if err := s.repo.SetActiveInstallation(userId, ""); err != nil {
				log.Logger.Errorf("installation: 取消活跃标记失败 installationId=%s err=%v", installationId, err)
			}
		}
		if err := s.notifyDisabled(userId, installationId); err != nil {
			if isRuntimeOfflineError(err) {
				log.Logger.Warnf("installation: NotifyInstallationDisabled 运行时离线 pending_sync installationId=%s", installationId)
			} else {
				return NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
			}
		}
		return nil
	}
	if err := s.repo.UpdateInstallationStatus(installationId, StatusDisabled); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "更新安装状态失败", err)
	}
	if err := s.repo.SetActiveInstallation(userId, ""); err != nil {
		log.Logger.Errorf("installation: 取消活跃标记失败 installationId=%s err=%v", installationId, err)
	}
	if err := s.notifyDisabled(userId, installationId); err != nil {
		if isRuntimeOfflineError(err) {
			log.Logger.Warnf("installation: NotifyInstallationDisabled 运行时离线 pending_sync installationId=%s", installationId)
		} else {
			return NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
		}
	}
	return nil
}

func (s *service) SwitchInstallation(userId, installationId string) error {
	if userId == "" || installationId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 或安装 ID 为空", ErrInstallationInvalid)
	}
	target, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if target.UserID != userId {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if target.Status == StatusUninstalled || target.Status == StatusUninstalling {
		return NewInstallationError(ErrCodeInstallationInvalid, "目标安装已卸载", ErrInstallationInvalid)
	}
	current, err := s.repo.GetActiveInstallation(userId)
	if err == nil && current.ID == installationId {
		if current.Status == StatusEnabled {
			return nil
		}
	}
	if err == nil && current.ID != installationId {
		if derr := s.DisableInstallation(userId, current.ID); derr != nil {
			log.Logger.Errorf("installation: 切换时停用旧安装失败 oldId=%s err=%v", current.ID, derr)
		}
	} else if err != nil && !errors.Is(err, ErrInstallationNotFound) {
		return NewInstallationError(ErrCodeInstallationFailed, "查询当前活跃安装失败", err)
	}
	return s.EnableInstallation(userId, installationId)
}

func (s *service) UpdateDefaultAction(installationId, actionKey string) error {
	if installationId == "" || actionKey == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装 ID 或动作 Key 为空", ErrInstallationInvalid)
	}
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if inst.Status == StatusUninstalled || inst.Status == StatusUninstalling {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录已卸载", ErrInstallationInvalid)
	}
	manifest, err := s.readManifest(inst.ManifestPath)
	if err != nil {
		return err
	}
	targetAction, found := s.findManifestAction(manifest, actionKey)
	if !found {
		return NewInstallationError(ErrCodeActionNotFound,
			fmt.Sprintf("动作 %s 不在 manifest 中", actionKey), ErrActionNotFound)
	}
	supports, err := s.actionSupportsDefaultIdle(inst.InstallPath, targetAction)
	if err != nil {
		return NewInstallationError(ErrCodeDefaultActionNotIdle, "校验默认待机失败", err)
	}
	if !supports {
		return NewInstallationError(ErrCodeDefaultActionNotIdle,
			fmt.Sprintf("动作 %s 不支持默认待机", actionKey), ErrDefaultActionNotIdle)
	}
	now := time.Now().Format(installationTimeFormat)
	if err := s.repo.DB().Model(&Installation{}).Where("id = ?", installationId).
		Updates(map[string]interface{}{
			"default_action_key": actionKey,
			"updated_at":         now,
		}).Error; err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "更新默认动作失败", err)
	}
	if err := s.notifyDefaultActionChanged(installationId, actionKey); err != nil {
		if isRuntimeOfflineError(err) {
			log.Logger.Warnf("installation: NotifyDefaultActionChanged 运行时离线 pending_sync installationId=%s", installationId)
		} else {
			return NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
		}
	}
	return nil
}

func (s *service) UpdateRuntimeSettings(userId, installationId string, req *UpdateRuntimeSettingsRequest) (*RuntimeSettings, error) {
	if userId == "" || installationId == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 或安装 ID 为空", ErrInstallationInvalid)
	}
	if req == nil {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "请求体为空", ErrInstallationInvalid)
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.IsEmpty() {
		return s.repo.GetRuntimeSettings(installationId)
	}
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return nil, NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if inst.UserID != userId {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	existing, err := s.repo.GetRuntimeSettings(installationId)
	if err != nil {
		if errors.Is(err, ErrRuntimeSettingsNotFound) {
			return nil, NewInstallationError(ErrCodeRuntimeSettingsNotFound, "运行时设置不存在，请先启用", err)
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "查询运行时设置失败", err)
	}
	updates := req.ToUpdates()
	now := time.Now().Format(installationTimeFormat)
	updates["updated_at"] = now
	if req.HasPositionChange() {
		updates["position_updated_at"] = now
	}
	var updated *RuntimeSettings
	if req.ExpectedRevision != nil {
		updated, err = s.repo.UpdateRuntimeSettingsWithCAS(installationId, *req.ExpectedRevision, updates)
		if err != nil {
			var rce *RevisionConflictError
			if errors.As(err, &rce) {
				return nil, NewInstallationError(ErrCodeRevisionConflict,
					fmt.Sprintf("设置版本冲突: 期望 %d, 实际 %d", rce.Expected, rce.Actual), err)
			}
			if errors.Is(err, ErrRuntimeSettingsNotFound) {
				return nil, NewInstallationError(ErrCodeRuntimeSettingsNotFound, "运行时设置不存在", err)
			}
			return nil, NewInstallationError(ErrCodeInstallationFailed, "更新运行时设置失败", err)
		}
	} else {
		updates["settings_revision"] = existing.SettingsRevision + 1
		if err := s.repo.UpdateRuntimeSettings(installationId, updates); err != nil {
			return nil, NewInstallationError(ErrCodeInstallationFailed, "更新运行时设置失败", err)
		}
		updated, err = s.repo.GetRuntimeSettings(installationId)
		if err != nil {
			return nil, NewInstallationError(ErrCodeInstallationFailed, "查询更新后的运行时设置失败", err)
		}
	}
	if err := s.notifyRuntimeSettingsUpdated(installationId, updated); err != nil {
		if isRuntimeOfflineError(err) {
			log.Logger.Warnf("installation: NotifyRuntimeSettingsUpdated 运行时离线 pending_sync installationId=%s", installationId)
		} else {
			return nil, NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
		}
	}
	return updated, nil
}

func (s *service) Recenter(userId, installationId string) error {
	if userId == "" || installationId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 或安装 ID 为空", ErrInstallationInvalid)
	}
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if inst.UserID != userId {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if inst.Status != StatusEnabled {
		return NewInstallationError(ErrCodePetNotEnabled, "桌宠未启用", ErrPetNotEnabled)
	}
	existing, err := s.repo.GetRuntimeSettings(installationId)
	if err != nil {
		if errors.Is(err, ErrRuntimeSettingsNotFound) {
			return NewInstallationError(ErrCodeRuntimeSettingsNotFound, "运行时设置不存在，请先启用", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询运行时设置失败", err)
	}
	now := time.Now().Format(installationTimeFormat)
	updates := map[string]interface{}{
		"position_mode":      positionModeRecenter,
		"settings_revision":  existing.SettingsRevision + 1,
		"position_updated_at": now,
		"updated_at":         now,
	}
	if err := s.repo.UpdateRuntimeSettings(installationId, updates); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "重置位置失败", err)
	}
	if err := s.notifyRecenter(installationId); err != nil {
		if isRuntimeOfflineError(err) {
			log.Logger.Warnf("installation: NotifyRecenter 运行时离线 pending_sync installationId=%s", installationId)
		} else {
			return NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
		}
	}
	return nil
}

func (s *service) PlayAction(userId, installationId, actionKey string) error {
	if userId == "" || installationId == "" || actionKey == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "用户 ID、安装 ID 或动作 Key 为空", ErrInstallationInvalid)
	}
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if inst.UserID != userId {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if inst.Status != StatusEnabled {
		return NewInstallationError(ErrCodePetNotEnabled, "桌宠未启用", ErrPetNotEnabled)
	}
	manifest, err := s.readManifest(inst.ManifestPath)
	if err != nil {
		return err
	}
	if _, found := s.findManifestAction(manifest, actionKey); !found {
		return NewInstallationError(ErrCodeActionNotFound,
			fmt.Sprintf("动作 %s 不在 manifest 中", actionKey), ErrActionNotFound)
	}
	if err := s.notifyActionPlayed(userId, installationId, actionKey); err != nil {
		if isRuntimeOfflineError(err) {
			log.Logger.Warnf("installation: NotifyActionPlayed 运行时离线 pending_sync installationId=%s", installationId)
		} else {
			return NewInstallationError(ErrCodeRuntimeDeliveryFailed, "运行时通知投递失败", err)
		}
	}
	return nil
}

func (s *service) ListInstallations(userId string) ([]*Installation, error) {
	return s.repo.ListInstallationsByUser(userId)
}

func (s *service) GetInstallation(installationId string) (*Installation, error) {
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return nil, NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	return inst, nil
}

func (s *service) GetRuntimeSettings(installationId string) (*RuntimeSettings, error) {
	return s.repo.GetRuntimeSettings(installationId)
}

func (s *service) absPath(relPath string) string {
	if relPath == "" {
		return ""
	}
	return filepath.Join(s.dataDir, filepath.FromSlash(relPath))
}

func (s *service) readManifest(relPath string) (*processing.Manifest, error) {
	absPath := s.absPath(relPath)
	if absPath == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "manifest 路径为空", ErrInstallationInvalid)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "manifest.json 不可读", err)
	}
	var manifest processing.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "manifest.json 解析失败", err)
	}
	if manifest.SchemaVersion != processing.ManifestSchemaVersion {
		return nil, NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("schemaVersion %d 不受支持", manifest.SchemaVersion), nil)
	}
	return &manifest, nil
}

func (s *service) findManifestAction(manifest *processing.Manifest, actionKey string) (processing.ManifestAction, bool) {
	for _, action := range manifest.Actions {
		if action.Key == actionKey {
			return action, true
		}
	}
	return processing.ManifestAction{}, false
}

func (s *service) actionSupportsDefaultIdle(installRelPath string, action processing.ManifestAction) (bool, error) {
	installDir := s.absPath(installRelPath)
	if installDir == "" {
		return false, fmt.Errorf("安装路径为空")
	}
	actionJSONPath := filepath.Join(installDir, filepath.FromSlash(action.Config))
	data, err := os.ReadFile(actionJSONPath)
	if err != nil {
		return false, fmt.Errorf("读取 action.json 失败: %w", err)
	}
	var probe struct {
		SupportsDefaultIdle *bool `json:"supportsDefaultIdle,omitempty"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false, fmt.Errorf("解析 action.json 失败: %w", err)
	}
	if probe.SupportsDefaultIdle != nil {
		return *probe.SupportsDefaultIdle, nil
	}
	return processing.IsLoopAction(action.Key), nil
}

func (s *service) computePackageHash(installDir string) (string, error) {
	files, err := listInstallationFiles(installDir)
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	hasher := sha256.New()
	for _, relPath := range files {
		hasher.Write([]byte(relPath))
		hasher.Write([]byte{0})
		absPath := filepath.Join(installDir, filepath.FromSlash(relPath))
		f, err := os.Open(absPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hasher, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *service) notifyEnabled(userId, installationId string, settings *RuntimeSettings) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.NotifyInstallationEnabled(userId, installationId, settings)
}

func (s *service) notifyDisabled(userId, installationId string) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.NotifyInstallationDisabled(userId, installationId)
}

func (s *service) notifyActionPlayed(userId, installationId, actionKey string) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.NotifyActionPlayed(userId, installationId, actionKey)
}

func (s *service) notifyRecenter(installationId string) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.NotifyRecenter(installationId)
}

func (s *service) notifyDefaultActionChanged(installationId, actionKey string) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.NotifyDefaultActionChanged(installationId, actionKey)
}

func (s *service) notifyRuntimeSettingsUpdated(installationId string, settings *RuntimeSettings) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.NotifyRuntimeSettingsUpdated(installationId, settings)
}

type runtimeErrorCodeProvider interface {
	GetCode() string
}

func isRuntimeOfflineError(err error) bool {
	var p runtimeErrorCodeProvider
	if errors.As(err, &p) {
		switch p.GetCode() {
		case "RUNTIME_OFFLINE", "RUNTIME_DISCONNECTED", "RUNTIME_NOT_READY":
			return true
		}
	}
	return false
}
