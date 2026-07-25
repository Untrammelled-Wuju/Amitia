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
	"strings"
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
	NotifyRuntimeSettingsUpdated(installationId string, settings map[string]interface{}) error
}

type Service interface {
	InstallPackage(packageId, userId, characterId string) (*Installation, error)
	Uninstall(installationId string) error
	EnableInstallation(userId, installationId string) error
	DisableInstallation(userId, installationId string) error
	SwitchInstallation(userId, installationId string) error
	UpdateDefaultAction(installationId, actionKey string) error
	UpdateRuntimeSettings(installationId string, settings map[string]interface{}) error
	Recenter(installationId string) error
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

func NewService(repo Repository, installer Installer, uninstaller Uninstaller, packageRepo processing.Repository, charRepo character.Repository, dataDir string) Service {
	return &service{
		repo:        repo,
		installer:   installer,
		uninstaller: uninstaller,
		packageRepo: packageRepo,
		charRepo:    charRepo,
		dataDir:     dataDir,
	}
}

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

func (s *service) Uninstall(installationId string) error {
	return s.uninstaller.Uninstall(installationId)
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
	s.notifyEnabled(userId, installationId, settings)
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
	if pkg.Status != "ready" {
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
		s.notifyDisabled(userId, installationId)
		return nil
	}
	if err := s.repo.UpdateInstallationStatus(installationId, StatusDisabled); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "更新安装状态失败", err)
	}
	if err := s.repo.SetActiveInstallation(userId, ""); err != nil {
		log.Logger.Errorf("installation: 取消活跃标记失败 installationId=%s err=%v", installationId, err)
	}
	s.notifyDisabled(userId, installationId)
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
	s.notifyDefaultActionChanged(installationId, actionKey)
	return nil
}

func (s *service) UpdateRuntimeSettings(installationId string, settings map[string]interface{}) error {
	if installationId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装 ID 为空", ErrInstallationInvalid)
	}
	if _, err := s.repo.GetInstallation(installationId); err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if len(settings) == 0 {
		return nil
	}
	if _, err := s.repo.GetRuntimeSettings(installationId); err != nil {
		if errors.Is(err, ErrRuntimeSettingsNotFound) {
			return NewInstallationError(ErrCodeRuntimeSettingsNotFound, "运行时设置不存在，请先启用", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询运行时设置失败", err)
	}
	filtered, err := filterRuntimeSettingsUpdates(settings)
	if err != nil {
		return err
	}
	if len(filtered) == 0 {
		return nil
	}
	filtered["updated_at"] = time.Now().Format(installationTimeFormat)
	if err := s.repo.UpdateRuntimeSettings(installationId, filtered); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "更新运行时设置失败", err)
	}
	s.notifyRuntimeSettingsUpdated(installationId, filtered)
	return nil
}

func (s *service) Recenter(installationId string) error {
	if installationId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装 ID 为空", ErrInstallationInvalid)
	}
	inst, err := s.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}
	if inst.Status != StatusEnabled {
		return NewInstallationError(ErrCodePetNotEnabled, "桌宠未启用", ErrPetNotEnabled)
	}
	if _, err := s.repo.GetRuntimeSettings(installationId); err != nil {
		if errors.Is(err, ErrRuntimeSettingsNotFound) {
			return NewInstallationError(ErrCodeRuntimeSettingsNotFound, "运行时设置不存在，请先启用", err)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询运行时设置失败", err)
	}
	now := time.Now().Format(installationTimeFormat)
	updates := map[string]interface{}{
		"position_x": 0,
		"position_y": 0,
		"screen_id":  "",
		"updated_at": now,
	}
	if err := s.repo.UpdateRuntimeSettings(installationId, updates); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "重置位置失败", err)
	}
	s.notifyRecenter(installationId)
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
	s.notifyActionPlayed(userId, installationId, actionKey)
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

func (s *service) notifyEnabled(userId, installationId string, settings *RuntimeSettings) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.NotifyInstallationEnabled(userId, installationId, settings); err != nil {
		log.Logger.Errorf("installation: NotifyInstallationEnabled 失败 installationId=%s err=%v", installationId, err)
	}
}

func (s *service) notifyDisabled(userId, installationId string) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.NotifyInstallationDisabled(userId, installationId); err != nil {
		log.Logger.Errorf("installation: NotifyInstallationDisabled 失败 installationId=%s err=%v", installationId, err)
	}
}

func (s *service) notifyActionPlayed(userId, installationId, actionKey string) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.NotifyActionPlayed(userId, installationId, actionKey); err != nil {
		log.Logger.Errorf("installation: NotifyActionPlayed 失败 installationId=%s err=%v", installationId, err)
	}
}

func (s *service) notifyRecenter(installationId string) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.NotifyRecenter(installationId); err != nil {
		log.Logger.Errorf("installation: NotifyRecenter 失败 installationId=%s err=%v", installationId, err)
	}
}

func (s *service) notifyDefaultActionChanged(installationId, actionKey string) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.NotifyDefaultActionChanged(installationId, actionKey); err != nil {
		log.Logger.Errorf("installation: NotifyDefaultActionChanged 失败 installationId=%s err=%v", installationId, err)
	}
}

func (s *service) notifyRuntimeSettingsUpdated(installationId string, settings map[string]interface{}) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.NotifyRuntimeSettingsUpdated(installationId, settings); err != nil {
		log.Logger.Errorf("installation: NotifyRuntimeSettingsUpdated 失败 installationId=%s err=%v", installationId, err)
	}
}

var runtimeSettingsAllowedColumns = map[string]bool{
	"always_on_top":             true,
	"launch_on_startup":         true,
	"scale":                     true,
	"position_x":                true,
	"position_y":                true,
	"screen_id":                 true,
	"idle_enabled":              true,
	"idle_interval_min_seconds": true,
	"idle_interval_max_seconds": true,
	"click_through_mode":        true,
	"sound_enabled":             true,
}

func filterRuntimeSettingsUpdates(input map[string]interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(input))
	for k, v := range input {
		column := strings.ToLower(k)
		if !runtimeSettingsAllowedColumns[column] {
			return nil, NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("不支持更新运行时字段: %s", k), ErrInstallationInvalid)
		}
		out[column] = v
	}
	return out, nil
}
