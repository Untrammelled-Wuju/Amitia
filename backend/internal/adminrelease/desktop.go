// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type desktopLatestFile struct {
	URL    string `yaml:"url"`
	SHA512 string `yaml:"sha512"`
	Size   int64  `yaml:"size"`
}

type desktopLatest struct {
	Version      string              `yaml:"version"`
	Files        []desktopLatestFile `yaml:"files"`
	Path         string              `yaml:"path"`
	SHA512       string              `yaml:"sha512"`
	ReleaseDate  string              `yaml:"releaseDate"`
	ReleaseName  string              `yaml:"releaseName"`
	ReleaseNotes string              `yaml:"releaseNotes"`
}

func (s *Server) publishDesktopRelease(release *UpdateRelease) error {
	var artifacts []UpdateReleaseArtifact
	if err := s.db.Where("release_id = ?", release.ID).Find(&artifacts).Error; err != nil {
		return err
	}
	byKind := map[string]UpdateReleaseArtifact{}
	for _, artifact := range artifacts {
		byKind[artifact.Kind] = artifact
	}
	installer, ok := byKind["installer"]
	if !ok {
		return errors.New("缺少桌面安装包")
	}
	blockmap, ok := byKind["blockmap"]
	if !ok {
		return errors.New("缺少桌面差分文件")
	}
	latestBytes, err := buildDesktopLatestYAML(release, installer)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.cfg.DesktopPublishDir, 0o755); err != nil {
		return err
	}
	installerTarget := filepath.Join(s.cfg.DesktopPublishDir, installer.OriginalName)
	blockmapTarget := filepath.Join(s.cfg.DesktopPublishDir, blockmap.OriginalName)
	if err := copyFileAtomic(installer.StoredPath, installerTarget); err != nil {
		return err
	}
	if err := copyFileAtomic(blockmap.StoredPath, blockmapTarget); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(s.cfg.DesktopPublishDir, "latest.yml"), latestBytes, 0o644); err != nil {
		return err
	}
	release.PublishPath = installerTarget
	return nil
}

func buildDesktopLatestYAML(release *UpdateRelease, installer UpdateReleaseArtifact) ([]byte, error) {
	if installer.SHA512Base64 == "" {
		return nil, errors.New("桌面安装包缺少 SHA-512")
	}
	latest := desktopLatest{
		Version: release.Version,
		Files: []desktopLatestFile{{
			URL:    installer.OriginalName,
			SHA512: installer.SHA512Base64,
			Size:   installer.Size,
		}},
		Path:         installer.OriginalName,
		SHA512:       installer.SHA512Base64,
		ReleaseDate:  time.Now().UTC().Format(time.RFC3339),
		ReleaseName:  release.ReleaseName,
		ReleaseNotes: release.ReleaseNotes,
	}
	content, err := yaml.Marshal(latest)
	if err != nil {
		return nil, fmt.Errorf("生成 latest.yml 失败: %w", err)
	}
	return content, nil
}
