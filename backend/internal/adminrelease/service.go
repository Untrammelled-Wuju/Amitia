// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	versionPattern       = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._-]{0,63}$`)
	desktopInstallerName = regexp.MustCompile(`^AmitiaSetup-[0-9A-Za-z._-]+-x64\.exe$`)
	desktopBlockmapName  = regexp.MustCompile(`^AmitiaSetup-[0-9A-Za-z._-]+-x64\.exe\.blockmap$`)
	apkNamePattern       = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._-]{0,255}\.apk$`)
)

type CreateReleaseInput struct {
	Product           string `json:"product"`
	Channel           string `json:"channel"`
	Version           string `json:"version"`
	VersionCode       int64  `json:"versionCode"`
	ReleaseName       string `json:"releaseName"`
	ReleaseNotes      string `json:"releaseNotes"`
	Mandatory         bool   `json:"mandatory"`
	MinVersionCode    int64  `json:"minSupportedVersionCode"`
	RolloutPercentage int    `json:"rolloutPercentage"`
	ABI               string `json:"abi"`
}

type releaseValidation struct {
	Ready     bool     `json:"ready"`
	Errors    []string `json:"errors"`
	Artifacts []string `json:"artifacts"`
}

func normalizeProduct(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProductDesktop:
		return ProductDesktop, nil
	case ProductAndroid:
		return ProductAndroid, nil
	default:
		return "", errors.New("更新产品类型无效")
	}
}

func normalizeChannel(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ChannelStable:
		return ChannelStable, nil
	case ChannelBeta:
		return ChannelBeta, nil
	case ChannelAlpha:
		return ChannelAlpha, nil
	default:
		return "", errors.New("更新通道无效")
	}
}

func (s *Server) createRelease(actor adminActor, input CreateReleaseInput) (*UpdateRelease, error) {
	product, err := normalizeProduct(input.Product)
	if err != nil {
		return nil, err
	}
	channel, err := normalizeChannel(input.Channel)
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(input.Version)
	if !versionPattern.MatchString(version) {
		return nil, errors.New("版本号格式无效")
	}
	if input.RolloutPercentage < 0 || input.RolloutPercentage > 100 {
		return nil, errors.New("灰度比例必须在 0 到 100 之间")
	}
	if product == ProductDesktop && channel != ChannelStable {
		return nil, errors.New("桌面端当前仅支持 stable 通道")
	}
	release := UpdateRelease{
		Product:           product,
		Channel:           channel,
		Version:           version,
		Status:            StatusDraft,
		ReleaseName:       strings.TrimSpace(input.ReleaseName),
		ReleaseNotes:      strings.TrimSpace(input.ReleaseNotes),
		Mandatory:         input.Mandatory,
		MinVersionCode:    input.MinVersionCode,
		RolloutPercentage: input.RolloutPercentage,
		CreatedBy:         actor.UserID,
	}
	if release.ReleaseName == "" {
		release.ReleaseName = "Amitia " + version
	}
	if product == ProductAndroid {
		release.VersionCode = input.VersionCode
		release.ABI = strings.TrimSpace(input.ABI)
		if release.ABI == "" {
			release.ABI = "arm64-v8a"
		}
		release.PackageName = s.cfg.AndroidPackageName
		if release.VersionCode <= 0 {
			return nil, errors.New("手机端 versionCode 必须大于 0")
		}
		if release.ABI != "arm64-v8a" {
			return nil, errors.New("手机端更新只允许 arm64-v8a")
		}
		if release.MinVersionCode < 0 || release.MinVersionCode > release.VersionCode {
			return nil, errors.New("最低支持 versionCode 无效")
		}
	}
	if err := s.db.Create(&release).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, errors.New("该产品和通道下版本已存在")
		}
		return nil, err
	}
	s.writeAudit(actor, "release.create", &release, "success", "创建发布草稿")
	return &release, nil
}

func (s *Server) listReleases(product, channel, status string) ([]releaseResponse, error) {
	query := s.db.Model(&UpdateRelease{}).Order("created_at DESC")
	if product != "" {
		normalized, err := normalizeProduct(product)
		if err != nil {
			return nil, err
		}
		query = query.Where("product = ?", normalized)
	}
	if channel != "" {
		normalized, err := normalizeChannel(channel)
		if err != nil {
			return nil, err
		}
		query = query.Where("channel = ?", normalized)
	}
	if status != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	var releases []UpdateRelease
	if err := query.Find(&releases).Error; err != nil {
		return nil, err
	}
	responses := make([]releaseResponse, 0, len(releases))
	for _, release := range releases {
		response, err := s.releaseWithArtifacts(release)
		if err != nil {
			return nil, err
		}
		responses = append(responses, *response)
	}
	return responses, nil
}

func (s *Server) getRelease(id uint) (*releaseResponse, error) {
	var release UpdateRelease
	if err := s.db.First(&release, id).Error; err != nil {
		return nil, err
	}
	return s.releaseWithArtifacts(release)
}

func (s *Server) releaseWithArtifacts(release UpdateRelease) (*releaseResponse, error) {
	var artifacts []UpdateReleaseArtifact
	if err := s.db.Where("release_id = ?", release.ID).Order("kind ASC").Find(&artifacts).Error; err != nil {
		return nil, err
	}
	return &releaseResponse{UpdateRelease: release, Artifacts: artifacts}, nil
}

func (s *Server) deleteDraft(actor adminActor, id uint) error {
	var release UpdateRelease
	if err := s.db.First(&release, id).Error; err != nil {
		return err
	}
	if release.Status != StatusDraft && release.Status != StatusFailed {
		return errors.New("只有草稿或失败记录可以删除")
	}
	var artifacts []UpdateReleaseArtifact
	if err := s.db.Where("release_id = ?", id).Find(&artifacts).Error; err != nil {
		return err
	}
	for _, artifact := range artifacts {
		_ = os.Remove(artifact.StoredPath)
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("release_id = ?", id).Delete(&UpdateReleaseArtifact{}).Error; err != nil {
			return err
		}
		return tx.Delete(&UpdateRelease{}, id).Error
	}); err != nil {
		return err
	}
	s.writeAudit(actor, "release.delete", &release, "success", "删除发布草稿")
	return nil
}

func (s *Server) saveArtifact(actor adminActor, releaseID uint, kind, originalName string, source multipart.File) (*UpdateReleaseArtifact, error) {
	var release UpdateRelease
	if err := s.db.First(&release, releaseID).Error; err != nil {
		return nil, err
	}
	if release.Status == StatusPublished || release.Status == StatusPaused {
		return nil, errors.New("已发布记录不能替换制品")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	originalName = filepath.Base(strings.TrimSpace(originalName))
	if originalName == "" {
		return nil, errors.New("文件名不能为空")
	}
	if err := validateArtifactName(release.Product, kind, originalName); err != nil {
		return nil, err
	}
	artifactDir := filepath.Join(s.cfg.DataDir, "artifacts", strconv.FormatUint(uint64(releaseID), 10))
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		return nil, err
	}
	storedName := kind + filepath.Ext(originalName)
	storedPath := filepath.Join(artifactDir, storedName)
	tempPath := storedPath + ".uploading"
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	sha256Hash := sha256.New()
	sha512Hash := sha512.New()
	size, copyErr := io.Copy(io.MultiWriter(tempFile, sha256Hash, sha512Hash), io.LimitReader(source, s.cfg.MaxUploadBytes+1))
	closeErr := tempFile.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return nil, closeErr
	}
	if size <= 0 {
		_ = os.Remove(tempPath)
		return nil, errors.New("上传文件为空")
	}
	if size > s.cfg.MaxUploadBytes {
		_ = os.Remove(tempPath)
		return nil, errors.New("上传文件超过大小限制")
	}
	if err := os.Rename(tempPath, storedPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	sha256Value := hex.EncodeToString(sha256Hash.Sum(nil))
	sha512Value := hex.EncodeToString(sha512Hash.Sum(nil))
	sha512Base64 := base64.StdEncoding.EncodeToString(sha512Hash.Sum(nil))
	artifact := UpdateReleaseArtifact{
		ReleaseID:    release.ID,
		Kind:         kind,
		OriginalName: originalName,
		StoredPath:   storedPath,
		Size:         size,
		SHA256:       sha256Value,
		SHA512:       sha512Value,
		SHA512Base64: sha512Base64,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing UpdateReleaseArtifact
		if err := tx.Where("release_id = ? AND kind = ?", release.ID, kind).First(&existing).Error; err == nil {
			artifact.ID = existing.ID
			artifact.CreatedAt = existing.CreatedAt
			if err := tx.Save(&artifact).Error; err != nil {
				return err
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&artifact).Error; err != nil {
				return err
			}
		} else {
			return err
		}
		return tx.Model(&UpdateRelease{}).Where("id = ?", release.ID).Updates(map[string]interface{}{
			"status":     StatusDraft,
			"updated_at": time.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	s.writeAudit(actor, "release.artifact.upload", &release, "success", fmt.Sprintf("%s %s", kind, originalName))
	return &artifact, nil
}

func validateArtifactName(product, kind, name string) error {
	switch product {
	case ProductDesktop:
		switch kind {
		case "installer":
			if !desktopInstallerName.MatchString(name) {
				return errors.New("桌面安装包文件名不符合 AmitiaSetup-*-x64.exe")
			}
		case "blockmap":
			if !desktopBlockmapName.MatchString(name) {
				return errors.New("桌面差分文件名不符合 AmitiaSetup-*-x64.exe.blockmap")
			}
		default:
			return errors.New("桌面端制品类型无效")
		}
	case ProductAndroid:
		if kind != "apk" || !apkNamePattern.MatchString(name) {
			return errors.New("手机端必须上传 APK 文件")
		}
	default:
		return errors.New("更新产品类型无效")
	}
	return nil
}

func (s *Server) validateRelease(releaseID uint) (*releaseValidation, error) {
	var release UpdateRelease
	if err := s.db.First(&release, releaseID).Error; err != nil {
		return nil, err
	}
	var artifacts []UpdateReleaseArtifact
	if err := s.db.Where("release_id = ?", release.ID).Find(&artifacts).Error; err != nil {
		return nil, err
	}
	byKind := map[string]UpdateReleaseArtifact{}
	for _, artifact := range artifacts {
		byKind[artifact.Kind] = artifact
	}
	result := releaseValidation{Ready: true, Artifacts: make([]string, 0, len(artifacts))}
	for _, artifact := range artifacts {
		if _, err := os.Stat(artifact.StoredPath); err != nil {
			result.Ready = false
			result.Errors = append(result.Errors, artifact.Kind+" 文件不存在")
			continue
		}
		result.Artifacts = append(result.Artifacts, artifact.OriginalName)
	}
	switch release.Product {
	case ProductDesktop:
		installer, hasInstaller := byKind["installer"]
		blockmap, hasBlockmap := byKind["blockmap"]
		if !hasInstaller {
			result.Ready = false
			result.Errors = append(result.Errors, "缺少桌面安装包")
		}
		if !hasBlockmap {
			result.Ready = false
			result.Errors = append(result.Errors, "缺少桌面差分文件")
		}
		if hasInstaller && hasBlockmap {
			expectedInstaller := "AmitiaSetup-" + release.Version + "-x64.exe"
			expectedBlockmap := expectedInstaller + ".blockmap"
			if installer.OriginalName != expectedInstaller {
				result.Ready = false
				result.Errors = append(result.Errors, "安装包文件名与版本号不匹配")
			}
			if blockmap.OriginalName != expectedBlockmap {
				result.Ready = false
				result.Errors = append(result.Errors, "差分文件与安装包不匹配")
			}
		}
	case ProductAndroid:
		apk, hasAPK := byKind["apk"]
		if !hasAPK {
			result.Ready = false
			result.Errors = append(result.Errors, "缺少 APK")
		} else {
			if err := validateAPKArchive(apk.StoredPath); err != nil {
				result.Ready = false
				result.Errors = append(result.Errors, err.Error())
			}
			if release.VersionCode <= 0 {
				result.Ready = false
				result.Errors = append(result.Errors, "versionCode 无效")
			}
			if release.RolloutPercentage < 0 || release.RolloutPercentage > 100 {
				result.Ready = false
				result.Errors = append(result.Errors, "灰度比例无效")
			}
		}
	default:
		result.Ready = false
		result.Errors = append(result.Errors, "产品类型无效")
	}
	status := StatusDraft
	if result.Ready {
		status = StatusReady
	}
	if err := s.db.Model(&UpdateRelease{}).Where("id = ?", release.ID).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func validateAPKArchive(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return errors.New("APK 不是有效的 ZIP 文件")
	}
	defer reader.Close()
	hasManifest := false
	hasArm64 := false
	for _, entry := range reader.File {
		if entry.Name == "AndroidManifest.xml" {
			hasManifest = true
		}
		if strings.HasPrefix(entry.Name, "lib/arm64-v8a/") {
			hasArm64 = true
		}
	}
	if !hasManifest {
		return errors.New("APK 缺少 AndroidManifest.xml")
	}
	if !hasArm64 {
		return errors.New("APK 缺少 arm64-v8a 原生库")
	}
	return nil
}

func (s *Server) publishRelease(actor adminActor, releaseID uint) (*UpdateRelease, error) {
	var release UpdateRelease
	if err := s.db.First(&release, releaseID).Error; err != nil {
		return nil, err
	}
	var result *UpdateRelease
	err := s.withChannelLock(context.Background(), release.Product, release.Channel, func() error {
		validation, err := s.validateRelease(releaseID)
		if err != nil {
			return err
		}
		if !validation.Ready {
			return errors.New(strings.Join(validation.Errors, "；"))
		}
		if err := s.db.First(&release, releaseID).Error; err != nil {
			return err
		}
		release.Status = StatusReady
		result, err = s.publishValidatedRelease(actor, &release)
		return err
	})
	return result, err
}

func (s *Server) publishValidatedRelease(actor adminActor, release *UpdateRelease) (*UpdateRelease, error) {
	var err error
	switch release.Product {
	case ProductDesktop:
		err = s.publishDesktopRelease(release)
	case ProductAndroid:
		err = s.publishAndroidRelease(release)
	default:
		err = errors.New("产品类型无效")
	}
	if err != nil {
		_ = s.db.Model(&UpdateRelease{}).Where("id = ?", release.ID).Updates(map[string]interface{}{
			"status":     StatusFailed,
			"updated_at": time.Now(),
		}).Error
		s.writeAudit(actor, "release.publish", release, "failed", err.Error())
		return nil, err
	}
	now := time.Now()
	if err := s.db.Model(&UpdateRelease{}).Where("id = ?", release.ID).Updates(map[string]interface{}{
		"status":       StatusPublished,
		"published_at": now,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, err
	}
	release.Status = StatusPublished
	release.PublishedAt = &now
	release.UpdatedAt = now
	s.writeAudit(actor, "release.publish", release, "success", "发布完成")
	return release, nil
}

func (s *Server) pauseRelease(actor adminActor, releaseID uint, rollout int) (*UpdateRelease, error) {
	if rollout < 0 || rollout > 100 {
		return nil, errors.New("灰度比例必须在 0 到 100 之间")
	}
	var release UpdateRelease
	if err := s.db.First(&release, releaseID).Error; err != nil {
		return nil, err
	}
	if release.Product != ProductAndroid {
		return nil, errors.New("只有手机端支持暂停灰度")
	}
	if release.Status != StatusPublished && release.Status != StatusPaused {
		return nil, errors.New("只有已发布的手机端版本可以调整灰度")
	}
	var result *UpdateRelease
	err := s.withChannelLock(context.Background(), release.Product, release.Channel, func() error {
		if err := s.db.First(&release, releaseID).Error; err != nil {
			return err
		}
		release.RolloutPercentage = rollout
		release.Status = StatusPaused
		if rollout == 100 {
			release.Status = StatusPublished
		}
		if err := s.publishAndroidRelease(&release); err != nil {
			return err
		}
		if err := s.db.Model(&UpdateRelease{}).Where("id = ?", release.ID).Updates(map[string]interface{}{
			"rollout_percentage": rollout,
			"status":             release.Status,
			"updated_at":         time.Now(),
		}).Error; err != nil {
			return err
		}
		s.writeAudit(actor, "release.rollout", &release, "success", fmt.Sprintf("灰度调整为 %d%%", rollout))
		result = &release
		return nil
	})
	return result, err
}

func (s *Server) rollbackDesktop(actor adminActor, releaseID uint) (*UpdateRelease, error) {
	var release UpdateRelease
	if err := s.db.First(&release, releaseID).Error; err != nil {
		return nil, err
	}
	if release.Product != ProductDesktop || release.Status != StatusPublished {
		return nil, errors.New("只有已发布的桌面端版本可以切换最新指针")
	}
	var result *UpdateRelease
	err := s.withChannelLock(context.Background(), release.Product, release.Channel, func() error {
		if err := s.db.First(&release, releaseID).Error; err != nil {
			return err
		}
		release.Status = StatusReady
		var publishErr error
		result, publishErr = s.publishValidatedRelease(actor, &release)
		if publishErr != nil {
			return publishErr
		}
		s.writeAudit(actor, "release.rollback.pointer", result, "success", "已切换桌面端最新指针")
		return nil
	})
	return result, err
}

func (s *Server) overview() (map[string]interface{}, error) {
	var releaseCount int64
	var publishedCount int64
	var androidCount int64
	var auditCount int64
	if err := s.db.Model(&UpdateRelease{}).Count(&releaseCount).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&UpdateRelease{}).Where("status = ?", StatusPublished).Count(&publishedCount).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&UpdateRelease{}).Where("product = ? AND status = ?", ProductAndroid, StatusPublished).Count(&androidCount).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&AdminAuditLog{}).Count(&auditCount).Error; err != nil {
		return nil, err
	}
	var latest []UpdateRelease
	if err := s.db.Where("status IN ?", []string{StatusPublished, StatusPaused}).Order("published_at DESC").Limit(8).Find(&latest).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"releaseCount":    releaseCount,
		"publishedCount":  publishedCount,
		"androidCount":    androidCount,
		"auditCount":      auditCount,
		"latestReleases":  latest,
		"desktopPublish":  s.cfg.DesktopPublishDir,
		"androidPublish":  s.cfg.AndroidPublishDir,
		"publicBaseURL":   s.cfg.PublicBaseURL,
		"signingKeyReady": s.cfg.AndroidManifestKeyPath != "",
	}, nil
}

func (s *Server) listAuditLogs(limit int) ([]AdminAuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var logs []AdminAuditLog
	if err := s.db.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *Server) writeAudit(actor adminActor, action string, release *UpdateRelease, result, message string) {
	log := AdminAuditLog{
		ActorID:   actor.UserID,
		ActorName: actor.Username,
		Action:    action,
		Result:    result,
		Message:   message,
		CreatedAt: time.Now(),
	}
	if release != nil {
		log.Product = release.Product
		log.Channel = release.Channel
		log.ReleaseID = release.ID
		log.Target = release.Version
	}
	_ = s.db.Create(&log).Error
}
