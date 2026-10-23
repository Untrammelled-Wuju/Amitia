// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

const (
	artifactUploadChunkSize = int64(8 * 1024 * 1024)
	artifactUploadMaxChunks = int64(4096)
	artifactUploadTTL       = 24 * time.Hour
	artifactUploadSuffix    = ".part"
	artifactUploadHash      = ".sha256"
)

var (
	uploadIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
	sha256Pattern       = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	artifactUploadLocks sync.Map
)

type artifactUploadManifest struct {
	ID           string    `json:"id"`
	ReleaseID    uint      `json:"releaseId"`
	Kind         string    `json:"kind"`
	OriginalName string    `json:"originalName"`
	Size         int64     `json:"size"`
	ChunkSize    int64     `json:"chunkSize"`
	ChunkCount   int       `json:"chunkCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type artifactUploadSessionResponse struct {
	UploadID      string `json:"uploadId"`
	ChunkSize     int64  `json:"chunkSize"`
	ChunkCount    int    `json:"chunkCount"`
	Concurrency   int    `json:"concurrency"`
	UploadedParts []int  `json:"uploadedParts"`
	UploadedBytes int64  `json:"uploadedBytes"`
	Completed     bool   `json:"completed"`
}

type createArtifactUploadInput struct {
	Kind     string `json:"kind"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
}

func artifactUploadRoot(dataDir string, releaseID uint) string {
	return filepath.Join(dataDir, "artifact-uploads", strconv.FormatUint(uint64(releaseID), 10))
}

func artifactUploadSessionDir(dataDir string, releaseID uint, uploadID string) (string, error) {
	if !uploadIDPattern.MatchString(uploadID) {
		return "", errors.New("上传会话编号无效")
	}
	return filepath.Join(artifactUploadRoot(dataDir, releaseID), uploadID), nil
}

func artifactPartPath(sessionDir string, index int) string {
	return filepath.Join(sessionDir, fmt.Sprintf("%08d%s", index, artifactUploadSuffix))
}

func artifactPartHashPath(sessionDir string, index int) string {
	return fmt.Sprintf("%s%s", artifactPartPath(sessionDir, index), artifactUploadHash)
}

func expectedArtifactPartSize(manifest artifactUploadManifest, index int) int64 {
	start := int64(index) * manifest.ChunkSize
	end := start + manifest.ChunkSize
	if end > manifest.Size {
		end = manifest.Size
	}
	if end < start {
		return 0
	}
	return end - start
}

func artifactPartReady(sessionDir string, manifest artifactUploadManifest, index int) bool {
	info, err := os.Stat(artifactPartPath(sessionDir, index))
	if err != nil || info.Size() != expectedArtifactPartSize(manifest, index) {
		return false
	}
	content, err := os.ReadFile(artifactPartHashPath(sessionDir, index))
	if err != nil {
		return false
	}
	return sha256Pattern.MatchString(strings.TrimSpace(string(content)))
}

func writeArtifactUploadManifest(path string, manifest artifactUploadManifest) error {
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	tempPath := path + ".uploading"
	if err := os.WriteFile(tempPath, content, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func readArtifactUploadManifest(sessionDir string) (artifactUploadManifest, error) {
	var manifest artifactUploadManifest
	content, err := os.ReadFile(filepath.Join(sessionDir, "manifest.json"))
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return manifest, errors.New("上传会话元数据损坏")
	}
	if manifest.ID == "" || manifest.ReleaseID == 0 || manifest.Size <= 0 ||
		manifest.ChunkSize <= 0 || manifest.ChunkCount <= 0 {
		return manifest, errors.New("上传会话元数据无效")
	}
	return manifest, nil
}

func (s *Server) cleanupArtifactUploads(releaseID uint) {
	root := artifactUploadRoot(s.cfg.DataDir, releaseID)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-artifactUploadTTL)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, statErr := entry.Info()
		if statErr == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}

func (s *Server) validateArtifactUploadRelease(releaseID uint, kind, originalName string, size int64) (*UpdateRelease, error) {
	var release UpdateRelease
	if err := s.db.First(&release, releaseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("发布记录不存在")
		}
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
	if size <= 0 {
		return nil, errors.New("上传文件为空")
	}
	if size > s.cfg.MaxUploadBytes {
		return nil, errors.New("上传文件超过大小限制")
	}
	if err := validateArtifactName(release.Product, kind, originalName); err != nil {
		return nil, err
	}
	return &release, nil
}

func (s *Server) createArtifactUpload(releaseID uint, input createArtifactUploadInput) (*artifactUploadSessionResponse, error) {
	release, err := s.validateArtifactUploadRelease(releaseID, input.Kind, input.FileName, input.Size)
	if err != nil {
		return nil, err
	}
	chunkCount := int((input.Size + artifactUploadChunkSize - 1) / artifactUploadChunkSize)
	if chunkCount <= 0 || int64(chunkCount) > artifactUploadMaxChunks {
		return nil, errors.New("上传分片数量无效")
	}
	uploadID, err := randomToken(24)
	if err != nil {
		return nil, err
	}
	sessionDir, err := artifactUploadSessionDir(s.cfg.DataDir, release.ID, uploadID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return nil, err
	}
	now := time.Now()
	manifest := artifactUploadManifest{
		ID:           uploadID,
		ReleaseID:    release.ID,
		Kind:         strings.ToLower(strings.TrimSpace(input.Kind)),
		OriginalName: filepath.Base(strings.TrimSpace(input.FileName)),
		Size:         input.Size,
		ChunkSize:    artifactUploadChunkSize,
		ChunkCount:   chunkCount,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := writeArtifactUploadManifest(filepath.Join(sessionDir, "manifest.json"), manifest); err != nil {
		_ = os.RemoveAll(sessionDir)
		return nil, err
	}
	s.cleanupArtifactUploads(release.ID)
	return &artifactUploadSessionResponse{
		UploadID:      uploadID,
		ChunkSize:     manifest.ChunkSize,
		ChunkCount:    manifest.ChunkCount,
		Concurrency:   4,
		UploadedParts: []int{},
		UploadedBytes: 0,
		Completed:     false,
	}, nil
}

func (s *Server) readArtifactUpload(releaseID uint, uploadID string) (artifactUploadManifest, string, error) {
	sessionDir, err := artifactUploadSessionDir(s.cfg.DataDir, releaseID, uploadID)
	if err != nil {
		return artifactUploadManifest{}, "", err
	}
	manifest, err := readArtifactUploadManifest(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, "", errors.New("上传会话不存在")
		}
		return manifest, "", err
	}
	if manifest.ReleaseID != releaseID || manifest.ID != uploadID {
		return manifest, "", errors.New("上传会话与发布记录不匹配")
	}
	return manifest, sessionDir, nil
}

func (s *Server) artifactUploadStatus(releaseID uint, uploadID string) (*artifactUploadSessionResponse, error) {
	manifest, sessionDir, err := s.readArtifactUpload(releaseID, uploadID)
	if err != nil {
		return nil, err
	}
	uploadedParts := make([]int, 0, manifest.ChunkCount)
	var uploadedBytes int64
	for index := 0; index < manifest.ChunkCount; index++ {
		if !artifactPartReady(sessionDir, manifest, index) {
			continue
		}
		info, statErr := os.Stat(artifactPartPath(sessionDir, index))
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return nil, statErr
		}
		uploadedParts = append(uploadedParts, index)
		uploadedBytes += info.Size()
	}
	return &artifactUploadSessionResponse{
		UploadID:      manifest.ID,
		ChunkSize:     manifest.ChunkSize,
		ChunkCount:    manifest.ChunkCount,
		Concurrency:   4,
		UploadedParts: uploadedParts,
		UploadedBytes: uploadedBytes,
		Completed:     len(uploadedParts) == manifest.ChunkCount,
	}, nil
}

func (s *Server) uploadArtifactPart(releaseID uint, uploadID string, index int, source io.Reader, expectedHash string) error {
	manifest, sessionDir, err := s.readArtifactUpload(releaseID, uploadID)
	if err != nil {
		return err
	}
	if index < 0 || index >= manifest.ChunkCount {
		return errors.New("上传分片编号无效")
	}
	expectedHash = strings.ToLower(strings.TrimSpace(expectedHash))
	if !sha256Pattern.MatchString(expectedHash) {
		return errors.New("分片 SHA-256 无效")
	}
	expectedSize := expectedArtifactPartSize(manifest, index)
	partPath := artifactPartPath(sessionDir, index)
	hashPath := artifactPartHashPath(sessionDir, index)
	if info, statErr := os.Stat(partPath); statErr == nil && info.Size() == expectedSize {
		if content, readErr := os.ReadFile(hashPath); readErr == nil &&
			strings.EqualFold(strings.TrimSpace(string(content)), expectedHash) {
			return nil
		}
	}
	tempSuffix, err := randomToken(12)
	if err != nil {
		return err
	}
	tempPath := fmt.Sprintf("%s.%s.uploading", partPath, tempSuffix)
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tempFile, hasher), io.LimitReader(source, expectedSize+1))
	closeErr := tempFile.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tempPath)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if written != expectedSize {
		_ = os.Remove(tempPath)
		if written > expectedSize {
			return errors.New("上传分片超过预期大小")
		}
		return errors.New("上传分片大小不完整")
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualHash, expectedHash) {
		_ = os.Remove(tempPath)
		return errors.New("上传分片 SHA-256 校验失败")
	}
	if err := os.Rename(tempPath, partPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	hashTempPath := hashPath + "." + tempSuffix + ".uploading"
	if err := os.WriteFile(hashTempPath, []byte(actualHash), 0o600); err != nil {
		_ = os.Remove(partPath)
		return err
	}
	if err := os.Rename(hashTempPath, hashPath); err != nil {
		_ = os.Remove(hashTempPath)
		_ = os.Remove(partPath)
		return err
	}
	return nil
}

func (s *Server) uploadArtifactLock(releaseID uint, kind string) func() {
	key := fmt.Sprintf("%d:%s", releaseID, kind)
	lockValue, _ := artifactUploadLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := lockValue.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func (s *Server) completeArtifactUpload(actor adminActor, releaseID uint, uploadID string) (*UpdateReleaseArtifact, error) {
	manifest, sessionDir, err := s.readArtifactUpload(releaseID, uploadID)
	if err != nil {
		return nil, err
	}
	status, err := s.artifactUploadStatus(releaseID, uploadID)
	if err != nil {
		return nil, err
	}
	if !status.Completed {
		return nil, errors.New("上传分片尚未全部完成")
	}
	unlock := s.uploadArtifactLock(releaseID, manifest.Kind)
	defer unlock()
	release, err := s.validateArtifactUploadRelease(releaseID, manifest.Kind, manifest.OriginalName, manifest.Size)
	if err != nil {
		return nil, err
	}
	artifactDir := filepath.Join(s.cfg.DataDir, "artifacts", strconv.FormatUint(uint64(releaseID), 10))
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		return nil, err
	}
	storedPath := filepath.Join(artifactDir, manifest.Kind+filepath.Ext(manifest.OriginalName))
	assemblePath := storedPath + ".assembling." + uploadID
	output, err := os.OpenFile(assemblePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	sha256Hash := sha256.New()
	sha512Hash := sha512.New()
	writer := io.MultiWriter(output, sha256Hash, sha512Hash)
	var total int64
	for index := 0; index < manifest.ChunkCount; index++ {
		partPath := artifactPartPath(sessionDir, index)
		part, openErr := os.Open(partPath)
		if openErr != nil {
			_ = output.Close()
			_ = os.Remove(assemblePath)
			return nil, openErr
		}
		written, copyErr := io.Copy(writer, io.LimitReader(part, expectedArtifactPartSize(manifest, index)+1))
		closeErr := part.Close()
		if copyErr != nil || closeErr != nil {
			_ = output.Close()
			_ = os.Remove(assemblePath)
			if copyErr != nil {
				return nil, copyErr
			}
			return nil, closeErr
		}
		total += written
	}
	syncErr := output.Sync()
	closeErr := output.Close()
	if syncErr != nil || closeErr != nil {
		_ = os.Remove(assemblePath)
		if syncErr != nil {
			return nil, syncErr
		}
		return nil, closeErr
	}
	if total != manifest.Size {
		_ = os.Remove(assemblePath)
		return nil, errors.New("合并后的文件大小不匹配")
	}
	if err := os.Rename(assemblePath, storedPath); err != nil {
		_ = os.Remove(assemblePath)
		return nil, err
	}
	sha256Value := hex.EncodeToString(sha256Hash.Sum(nil))
	sha512Value := hex.EncodeToString(sha512Hash.Sum(nil))
	sha512Base64 := base64.StdEncoding.EncodeToString(sha512Hash.Sum(nil))
	artifact := UpdateReleaseArtifact{
		ReleaseID:    release.ID,
		Kind:         manifest.Kind,
		OriginalName: manifest.OriginalName,
		StoredPath:   storedPath,
		Size:         total,
		SHA256:       sha256Value,
		SHA512:       sha512Value,
		SHA512Base64: sha512Base64,
	}
	if err := s.persistArtifactRecord(actor, release, &artifact); err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	_ = os.RemoveAll(sessionDir)
	s.writeAudit(actor, "release.artifact.upload.complete", release, "success", fmt.Sprintf("%s %s", manifest.Kind, manifest.OriginalName))
	return &artifact, nil
}

func (s *Server) abortArtifactUpload(releaseID uint, uploadID string) error {
	_, sessionDir, err := s.readArtifactUpload(releaseID, uploadID)
	if err != nil {
		return err
	}
	return os.RemoveAll(sessionDir)
}

func (s *Server) handleCreateArtifactUpload(c *gin.Context) {
	actor := currentActor(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	var input createArtifactUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.ErrorResponse(c, 400, "上传参数无效", nil)
		return
	}
	session, err := s.createArtifactUpload(uint(id), input)
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	s.writeAudit(*actor, "release.artifact.upload.start", nil, "success", fmt.Sprintf("%s %s", input.Kind, input.FileName))
	util.SuccessResponse(c, session)
}

func (s *Server) handleGetArtifactUpload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	status, err := s.artifactUploadStatus(uint(id), c.Param("uploadId"))
	if err != nil {
		util.ErrorResponse(c, 404, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, status)
}

func (s *Server) handleUploadArtifactPart(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		util.ErrorResponse(c, 400, "分片编号无效", nil)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, artifactUploadChunkSize+1)
	if err := s.uploadArtifactPart(uint(id), c.Param("uploadId"), index, c.Request.Body, c.GetHeader("X-Chunk-SHA256")); err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "分片已上传", gin.H{"index": index})
}

func (s *Server) handleCompleteArtifactUpload(c *gin.Context) {
	actor := currentActor(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	artifact, err := s.completeArtifactUpload(*actor, uint(id), c.Param("uploadId"))
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, artifact)
}

func (s *Server) handleAbortArtifactUpload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "发布编号无效", nil)
		return
	}
	if err := s.abortArtifactUpload(uint(id), c.Param("uploadId")); err != nil {
		util.ErrorResponse(c, 404, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "上传已取消", nil)
}
