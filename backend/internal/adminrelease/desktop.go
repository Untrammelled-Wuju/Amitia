// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

type desktopVersion struct {
	major      int
	minor      int
	patch      int
	prerelease []string
}

func parseDesktopVersion(raw string) (desktopVersion, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return desktopVersion{}, false
	}
	withoutBuild := strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(withoutBuild, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return desktopVersion{}, false
	}
	numbers := make([]int, 3)
	for index, part := range core {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return desktopVersion{}, false
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return desktopVersion{}, false
		}
		numbers[index] = number
	}
	var prerelease []string
	if len(parts) == 2 {
		if parts[1] == "" {
			return desktopVersion{}, false
		}
		prerelease = strings.Split(parts[1], ".")
		for _, identifier := range prerelease {
			if identifier == "" {
				return desktopVersion{}, false
			}
			numeric := true
			for _, character := range identifier {
				if !((character >= '0' && character <= '9') ||
					(character >= 'A' && character <= 'Z') ||
					(character >= 'a' && character <= 'z') ||
					character == '-') {
					return desktopVersion{}, false
				}
				if character < '0' || character > '9' {
					numeric = false
				}
			}
			if numeric && len(identifier) > 1 && identifier[0] == '0' {
				return desktopVersion{}, false
			}
		}
	}
	return desktopVersion{
		major:      numbers[0],
		minor:      numbers[1],
		patch:      numbers[2],
		prerelease: prerelease,
	}, true
}

func compareDesktopVersions(left, right string) int {
	leftVersion, leftOK := parseDesktopVersion(left)
	rightVersion, rightOK := parseDesktopVersion(right)
	if !leftOK || !rightOK {
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	for _, pair := range [][2]int{
		{leftVersion.major, rightVersion.major},
		{leftVersion.minor, rightVersion.minor},
		{leftVersion.patch, rightVersion.patch},
	} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(leftVersion.prerelease) == 0 && len(rightVersion.prerelease) == 0 {
		return 0
	}
	if len(leftVersion.prerelease) == 0 {
		return 1
	}
	if len(rightVersion.prerelease) == 0 {
		return -1
	}
	count := len(leftVersion.prerelease)
	if len(rightVersion.prerelease) < count {
		count = len(rightVersion.prerelease)
	}
	for index := 0; index < count; index++ {
		leftIdentifier := leftVersion.prerelease[index]
		rightIdentifier := rightVersion.prerelease[index]
		leftNumber, leftErr := strconv.Atoi(leftIdentifier)
		rightNumber, rightErr := strconv.Atoi(rightIdentifier)
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		default:
			if comparison := strings.Compare(leftIdentifier, rightIdentifier); comparison != 0 {
				return comparison
			}
		}
	}
	switch {
	case len(leftVersion.prerelease) < len(rightVersion.prerelease):
		return -1
	case len(leftVersion.prerelease) > len(rightVersion.prerelease):
		return 1
	default:
		return 0
	}
}

func selectHighestDesktopRelease(candidates []UpdateRelease) *UpdateRelease {
	var best *UpdateRelease
	for index := range candidates {
		candidate := &candidates[index]
		if best == nil {
			best = candidate
			continue
		}
		comparison := compareDesktopVersions(candidate.Version, best.Version)
		if comparison > 0 || (comparison == 0 && candidate.ID > best.ID) {
			best = candidate
		}
	}
	return best
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
	release.PublishPath = installerTarget
	if release.Channel == ChannelBeta {
		return writeFileAtomic(filepath.Join(s.cfg.DesktopPublishDir, "beta.yml"), latestBytes, 0o644)
	}
	if err := writeFileAtomic(filepath.Join(s.cfg.DesktopPublishDir, "latest.yml"), latestBytes, 0o644); err != nil {
		return err
	}
	return s.refreshDesktopBetaManifest(release, installer)
}

func (s *Server) refreshDesktopBetaManifest(stableRelease *UpdateRelease, stableInstaller UpdateReleaseArtifact) error {
	var betaReleases []UpdateRelease
	if err := s.db.Where(
		"product = ? AND channel = ? AND status = ?",
		ProductDesktop,
		ChannelBeta,
		StatusPublished,
	).Find(&betaReleases).Error; err != nil {
		return err
	}
	selected := selectHighestDesktopRelease(betaReleases)
	if selected != nil && compareDesktopVersions(selected.Version, stableRelease.Version) > 0 {
		var installer UpdateReleaseArtifact
		if err := s.db.Where("release_id = ? AND kind = ?", selected.ID, "installer").First(&installer).Error; err != nil {
			return fmt.Errorf("读取桌面 beta 安装包失败: %w", err)
		}
		content, err := buildDesktopLatestYAML(selected, installer)
		if err != nil {
			return err
		}
		return writeFileAtomic(filepath.Join(s.cfg.DesktopPublishDir, "beta.yml"), content, 0o644)
	}
	content, err := buildDesktopLatestYAML(stableRelease, stableInstaller)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.cfg.DesktopPublishDir, "beta.yml"), content, 0o644)
}

func (s *Server) ensureDesktopBetaManifest() error {
	betaPath := filepath.Join(s.cfg.DesktopPublishDir, "beta.yml")
	if _, err := os.Stat(betaPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	content, err := os.ReadFile(filepath.Join(s.cfg.DesktopPublishDir, "latest.yml"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return writeFileAtomic(betaPath, content, 0o644)
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
		return nil, fmt.Errorf("生成桌面更新清单失败: %w", err)
	}
	return content, nil
}
