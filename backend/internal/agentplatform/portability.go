package agentplatform

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

type AgentArchive struct {
	Path     string
	FileName string
	Size     int64
	Hash     string
}

func (s *Service) CreateAgentArchive(ctx context.Context, spaceID, agentID, version string) (AgentArchive, error) {
	pkg, err := s.ExportAgentPackage(ctx, spaceID, agentID, version)
	if err != nil {
		return AgentArchive{}, err
	}
	root := filepath.Join(s.dataDir, "agent-platform", "packages")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return AgentArchive{}, fmt.Errorf("创建 Agent 包目录: %w", err)
	}
	target := filepath.Join(root, pkg.ID+".zip")
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return AgentArchive{}, fmt.Errorf("创建 Agent 可携带包: %w", err)
	}
	writer := zip.NewWriter(file)
	writeErr := func() error {
		header := &zip.FileHeader{Name: "agent-package.json", Method: zip.Deflate}
		header.SetMode(0o600)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("创建 Agent 包清单条目: %w", err)
		}
		if err := json.NewEncoder(entry).Encode(pkg); err != nil {
			return fmt.Errorf("写入 Agent 包清单: %w", err)
		}
		var bundle AgentBundle
		if err := json.Unmarshal([]byte(pkg.BundleJSON), &bundle); err != nil {
			return fmt.Errorf("解析 Agent 包内容: %w", err)
		}
		snapshotManifest := make([]map[string]any, 0, len(bundle.Snapshots))
		for _, snapshot := range bundle.Snapshots {
			if err := validateSnapshotPath(snapshot.Path); err != nil {
				return err
			}
			hash, err := fileSHA256(snapshot.Path)
			if err != nil {
				return err
			}
			if snapshot.ContentHash != "" && snapshot.ContentHash != hash {
				return fmt.Errorf("Agent 快照 %s 哈希校验失败", snapshot.ID)
			}
			name := "snapshots/" + snapshot.ID + ".tar.gz"
			snapshotHeader := &zip.FileHeader{Name: name, Method: zip.Store}
			snapshotHeader.SetMode(0o600)
			entry, err := writer.CreateHeader(snapshotHeader)
			if err != nil {
				return fmt.Errorf("创建 Agent 快照条目: %w", err)
			}
			source, err := os.Open(snapshot.Path)
			if err != nil {
				return fmt.Errorf("读取 Agent 快照: %w", err)
			}
			_, copyErr := io.Copy(entry, source)
			closeErr := source.Close()
			if copyErr != nil {
				return fmt.Errorf("写入 Agent 快照: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("关闭 Agent 快照: %w", closeErr)
			}
			snapshotManifest = append(snapshotManifest, map[string]any{
				"id": snapshot.ID, "runtimeId": snapshot.RuntimeID, "name": snapshot.Name,
				"path": name, "sha256": snapshot.ContentHash,
			})
		}
		manifestHeader := &zip.FileHeader{Name: "snapshot-manifest.json", Method: zip.Deflate}
		manifestHeader.SetMode(0o600)
		manifestEntry, err := writer.CreateHeader(manifestHeader)
		if err != nil {
			return fmt.Errorf("创建 Agent 快照清单条目: %w", err)
		}
		if err := json.NewEncoder(manifestEntry).Encode(snapshotManifest); err != nil {
			return fmt.Errorf("写入 Agent 快照清单: %w", err)
		}
		return nil
	}()
	zipErr := writer.Close()
	closeErr := file.Close()
	if writeErr != nil {
		return AgentArchive{}, writeErr
	}
	if zipErr != nil {
		return AgentArchive{}, fmt.Errorf("完成 Agent 可携带包: %w", zipErr)
	}
	if closeErr != nil {
		return AgentArchive{}, fmt.Errorf("关闭 Agent 可携带包: %w", closeErr)
	}
	hash, err := fileSHA256(target)
	if err != nil {
		return AgentArchive{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return AgentArchive{}, fmt.Errorf("读取 Agent 可携带包信息: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&AgentPackage{}).Where("id = ?", pkg.ID).Updates(map[string]any{
		"archive_path": target, "archive_hash": hash, "archive_bytes": info.Size(),
	}).Error; err != nil {
		return AgentArchive{}, fmt.Errorf("保存 Agent 可携带包: %w", err)
	}
	safeName := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(strings.TrimSpace(pkg.Name))
	if safeName == "" {
		safeName = "agent"
	}
	return AgentArchive{Path: target, FileName: safeName + "-" + pkg.Version + ".amitia-agent.zip", Size: info.Size(), Hash: hash}, nil
}

func (s *Service) ImportAgentArchive(ctx context.Context, spaceID, ownerID, archivePath string) (AgentDetail, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return AgentDetail{}, fmt.Errorf("打开 Agent 可携带包: %w", err)
	}
	defer archive.Close()
	tempDir, err := os.MkdirTemp("", "amitia-agent-import-*")
	if err != nil {
		return AgentDetail{}, fmt.Errorf("创建 Agent 导入目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	var pkg AgentPackage
	foundPackage := false
	snapshotPaths := map[string]string{}
	seenEntries := map[string]bool{}
	var totalSize int64
	for _, file := range archive.File {
		totalSize += int64(file.UncompressedSize64)
		if totalSize > 20<<30 {
			return AgentDetail{}, errors.New("Agent 可携带包超过允许大小")
		}
		name := filepath.ToSlash(file.Name)
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || seenEntries[name] {
			return AgentDetail{}, fmt.Errorf("Agent 可携带包包含非法路径: %s", file.Name)
		}
		seenEntries[name] = true
		switch {
		case name == "agent-package.json":
			reader, err := file.Open()
			if err != nil {
				return AgentDetail{}, fmt.Errorf("读取 Agent 包清单: %w", err)
			}
			decodeErr := json.NewDecoder(io.LimitReader(reader, 20<<20)).Decode(&pkg)
			closeErr := reader.Close()
			if decodeErr != nil {
				return AgentDetail{}, fmt.Errorf("解析 Agent 包清单: %w", decodeErr)
			}
			if closeErr != nil {
				return AgentDetail{}, fmt.Errorf("关闭 Agent 包清单: %w", closeErr)
			}
			foundPackage = true
		case strings.HasPrefix(name, "snapshots/") && strings.HasSuffix(name, ".tar.gz"):
			target := filepath.Join(tempDir, "snapshots", filepath.Base(name))
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return AgentDetail{}, fmt.Errorf("创建 Agent 快照导入目录: %w", err)
			}
			reader, err := file.Open()
			if err != nil {
				return AgentDetail{}, fmt.Errorf("读取导入 Agent 快照: %w", err)
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				reader.Close()
				return AgentDetail{}, fmt.Errorf("创建导入 Agent 快照: %w", err)
			}
			_, copyErr := io.Copy(output, reader)
			readerCloseErr := reader.Close()
			outputCloseErr := output.Close()
			if copyErr != nil {
				return AgentDetail{}, fmt.Errorf("保存导入 Agent 快照: %w", copyErr)
			}
			if readerCloseErr != nil {
				return AgentDetail{}, fmt.Errorf("关闭导入 Agent 快照源: %w", readerCloseErr)
			}
			if outputCloseErr != nil {
				return AgentDetail{}, fmt.Errorf("关闭导入 Agent 快照目标: %w", outputCloseErr)
			}
			snapshotPaths[strings.TrimSuffix(filepath.Base(name), ".tar.gz")] = target
		}
	}
	if !foundPackage {
		return AgentDetail{}, errors.New("Agent 可携带包缺少 agent-package.json")
	}
	var bundle AgentBundle
	if err := json.Unmarshal([]byte(pkg.BundleJSON), &bundle); err != nil {
		return AgentDetail{}, fmt.Errorf("解析 Agent 包内容: %w", err)
	}
	_, computedHash, err := hashJSON(bundle)
	if err != nil {
		return AgentDetail{}, err
	}
	if pkg.ContentHash == "" || pkg.ContentHash != computedHash {
		return AgentDetail{}, errors.New("Agent 可携带包内容校验失败")
	}
	return s.importAgentBundle(ctx, spaceID, ownerID, bundle, snapshotPaths)
}

func (s *Service) importAgentBundle(ctx context.Context, spaceID, ownerID string, bundle AgentBundle, snapshotPaths map[string]string) (AgentDetail, error) {
	var config map[string]any
	if err := json.Unmarshal([]byte(bundle.Revision.ConfigJSON), &config); err != nil {
		return AgentDetail{}, fmt.Errorf("解析 Agent 包配置: %w", err)
	}
	teamID := ""
	if bundle.Team != nil {
		team, err := s.CreateTeam(ctx, spaceID, CreateTeamInput{
			Name: bundle.Team.Name, Description: bundle.Team.Description,
			OwnerID: ownerID, SharedMemory: bundle.Team.SharedMemory, SharedWorkspace: bundle.Team.SharedWorkspace,
		})
		if err != nil {
			return AgentDetail{}, err
		}
		teamID = team.ID
		for _, shared := range bundle.TeamContexts {
			if _, err := s.CreateTeamContext(ctx, spaceID, team.ID, CreateTeamContextInput{
				Kind: shared.Kind, Key: shared.Key, Content: shared.Content, Visibility: shared.Visibility,
			}); err != nil {
				return AgentDetail{}, err
			}
		}
	}
	detail, err := s.CreateAgent(ctx, spaceID, CreateAgentInput{
		Name: bundle.Agent.Name, Description: bundle.Agent.Description,
		Archetype: bundle.Agent.Archetype, OwnerID: ownerID, TeamID: teamID, Config: config,
	})
	if err != nil {
		return AgentDetail{}, err
	}
	now := time.Now().UTC()
	history := bundle.History
	if len(history) == 0 {
		history = []AgentRevision{bundle.Revision}
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ? AND space_id = ?", detail.Profile.ID, spaceID).Delete(&AgentRevision{}).Error; err != nil {
			return fmt.Errorf("清理导入 Agent 初始版本: %w", err)
		}
		revisionMap := map[string]string{}
		var latestRevisionID string
		var latestRevision int64
		for _, source := range history {
			next := source
			next.ID = newID("revision")
			next.SpaceID = spaceID
			next.AgentID = detail.Profile.ID
			next.CreatedBy = ownerID
			if next.CreatedAt.IsZero() {
				next.CreatedAt = now
			}
			if err := tx.Create(&next).Error; err != nil {
				return fmt.Errorf("导入 Agent 版本历史: %w", err)
			}
			revisionMap[source.ID] = next.ID
			if next.Revision >= latestRevision {
				latestRevision = next.Revision
				latestRevisionID = next.ID
			}
		}
		releaseMap := map[string]string{}
		for _, source := range bundle.Releases {
			revisionID := revisionMap[source.RevisionID]
			if revisionID == "" {
				continue
			}
			next := source
			next.ID = newID("release")
			next.SpaceID = spaceID
			next.AgentID = detail.Profile.ID
			next.RevisionID = revisionID
			if next.CreatedAt.IsZero() {
				next.CreatedAt = now
			}
			next.SupersededAt = nil
			if err := tx.Create(&next).Error; err != nil {
				return fmt.Errorf("导入 Agent 发布历史: %w", err)
			}
			releaseMap[source.ID] = next.ID
		}
		currentRevisionID := revisionMap[bundle.Agent.CurrentRevisionID]
		if currentRevisionID == "" {
			currentRevisionID = latestRevisionID
		}
		currentReleaseID := releaseMap[bundle.Agent.CurrentReleaseID]
		if err := tx.Model(&AgentProfile{}).Where("id = ? AND space_id = ?", detail.Profile.ID, spaceID).Updates(map[string]any{
			"current_revision_id": currentRevisionID, "current_release_id": currentReleaseID,
			"revision": latestRevision, "status": "draft", "updated_at": now,
		}).Error; err != nil {
			return fmt.Errorf("恢复导入 Agent 当前版本指针: %w", err)
		}
		return nil
	}); err != nil {
		return AgentDetail{}, err
	}
	for _, grant := range bundle.Credentials {
		var scopes []string
		if err := json.Unmarshal([]byte(grant.ScopeJSON), &scopes); err != nil {
			return AgentDetail{}, fmt.Errorf("解析导入 Agent 授权: %w", err)
		}
		scopeJSON, _, err := hashJSON(scopes)
		if err != nil {
			return AgentDetail{}, err
		}
		imported := CredentialGrant{
			ID: newID("grant"), SpaceID: spaceID, AgentID: detail.Profile.ID,
			ConnectionID: grant.ConnectionID, Provider: grant.Provider, AccountID: grant.AccountID,
			ScopeJSON: scopeJSON, Status: "reauth_required", CreatedAt: now, UpdatedAt: now,
		}
		if err := s.db.WithContext(ctx).Create(&imported).Error; err != nil {
			return AgentDetail{}, fmt.Errorf("导入 Agent 授权声明: %w", err)
		}
	}
	if bundle.Runtime != nil {
		provider := strings.ToLower(strings.TrimSpace(bundle.Runtime.Provider))
		if provider == "" || provider == "remote" || provider == "android" {
			provider = "local"
		}
		runtime, err := s.runtime.Create(ctx, spaceID, CreateRuntimeInput{
			AgentID: detail.Profile.ID, Provider: provider, Image: bundle.Runtime.Image,
			CPUQuota: bundle.Runtime.CPUQuota, MemoryBytes: bundle.Runtime.MemoryBytes,
			DiskBytes: bundle.Runtime.DiskBytes, ProcessLimit: bundle.Runtime.ProcessLimit,
			IdleTimeoutSeconds: bundle.Runtime.IdleTimeoutSeconds,
		})
		if err != nil {
			return AgentDetail{}, err
		}
		var restoredSnapshotID string
		for _, snapshot := range bundle.Snapshots {
			path := snapshotPaths[snapshot.ID]
			if path == "" {
				continue
			}
			imported, err := s.runtime.ImportSnapshot(ctx, spaceID, detail.Profile.ID, runtime.ID, snapshot.Name, path, snapshot.ContentHash)
			if err != nil {
				return AgentDetail{}, err
			}
			restoredSnapshotID = imported.ID
		}
		if restoredSnapshotID != "" {
			if _, err := s.runtime.Reset(ctx, spaceID, runtime.ID, restoredSnapshotID); err != nil {
				return AgentDetail{}, err
			}
			if _, err := s.runtime.Start(ctx, spaceID, runtime.ID); err != nil {
				return AgentDetail{}, err
			}
		}
	}
	var imported AgentProfile
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", detail.Profile.ID, spaceID).First(&imported).Error; err != nil {
		return AgentDetail{}, fmt.Errorf("读取导入 Agent: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{
		SpaceID: spaceID, AgentID: imported.ID, TeamID: imported.TeamID,
		Kind: "agent.portability", State: "imported", Title: "Agent 配置、团队与电脑已导入",
	}); err != nil {
		return AgentDetail{}, err
	}
	return s.GetAgent(ctx, spaceID, imported.ID)
}

func validateSnapshotPath(path string) error {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." || strings.Contains(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Agent 快照路径无效: %s", path)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return fmt.Errorf("读取 Agent 快照文件: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Agent 快照不是普通文件: %s", path)
	}
	return nil
}
