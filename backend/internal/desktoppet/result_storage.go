// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	_ "golang.org/x/image/webp"
)

const (
	maxGeneratedImageSize = 10 * 1024 * 1024
	resultMetadataName    = "metadata.json"
	resultRawDirName      = "raw"
	resultAttemptDirFmt   = "attempt-%d"
	resultFrameFileFmt    = "frame-%04d.png"
)

var allowedGeneratedMimes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
}

type ResultDownloader struct{}

func NewResultDownloader() *ResultDownloader { return &ResultDownloader{} }

func (d *ResultDownloader) DownloadAndSave(data []byte, mimeType string, taskID, actionKey string, attempt int, frameIndex int) (path string, width int, height int, size int, hash string, err error) {
	if len(data) == 0 {
		return "", 0, 0, 0, "", NewBusinessError(400, ErrCodeImageResultInvalidFormat, "生成图片内容为空")
	}
	if len(data) > maxGeneratedImageSize {
		return "", 0, 0, 0, "", NewBusinessError(400, ErrCodeImageResultTooLarge, "生成图片大小不能超过 10MB")
	}

	detected := http.DetectContentType(data)
	normalized := strings.TrimSpace(strings.Split(detected, ";")[0])
	if !allowedGeneratedMimes[normalized] {
		return "", 0, 0, 0, "", NewBusinessError(400, ErrCodeImageResultInvalidFormat, "生成图片格式无效,仅允许 png/jpeg/webp")
	}

	img, _, decErr := image.Decode(bytes.NewReader(data))
	if decErr != nil {
		return "", 0, 0, 0, "", NewBusinessError(400, ErrCodeImageResultDecodeFailed, fmt.Sprintf("解析生成图片失败: %v", decErr))
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return "", 0, 0, 0, "", NewBusinessError(400, ErrCodeImageResultDecodeFailed, "生成图片尺寸无效")
	}

	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])

	attemptDir, err := d.EnsureAttemptDir(taskID, actionKey, attempt)
	if err != nil {
		return "", 0, 0, 0, "", NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("创建结果目录失败: %v", err))
	}

	rawDir := filepath.Join(attemptDir, resultRawDirName)
	if mkErr := os.MkdirAll(rawDir, 0755); mkErr != nil {
		return "", 0, 0, 0, "", NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("创建结果原始目录失败: %v", mkErr))
	}

	fileName := fmt.Sprintf(resultFrameFileFmt, frameIndex)
	finalPath := filepath.Join(rawDir, fileName)
	tmpPath := finalPath + ".tmp"
	if writeErr := os.WriteFile(tmpPath, data, 0644); writeErr != nil {
		_ = os.Remove(tmpPath)
		return "", 0, 0, 0, "", NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("写入生成图片失败: %v", writeErr))
	}
	if renameErr := os.Rename(tmpPath, finalPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return "", 0, 0, 0, "", NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("保存生成图片失败: %v", renameErr))
	}

	relPath := buildResultRelativePath(taskID, actionKey, attempt, fileName)
	return relPath, w, h, len(data), digest, nil
}

func (d *ResultDownloader) WriteMetadata(taskID, actionKey string, attempt int, metadata map[string]interface{}) error {
	attemptDir, err := d.EnsureAttemptDir(taskID, actionKey, attempt)
	if err != nil {
		return NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("创建元数据目录失败: %v", err))
	}
	sanitized := sanitizeMetadata(metadata)
	sanitized["writtenAt"] = time.Now().Format("2006-01-02 15:04:05")
	sanitized["taskId"] = taskID
	sanitized["actionKey"] = actionKey
	sanitized["attempt"] = attempt
	payload, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("序列化元数据失败: %v", err))
	}
	metaPath := filepath.Join(attemptDir, resultMetadataName)
	tmpPath := metaPath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("写入元数据失败: %v", err))
	}
	if err := os.Rename(tmpPath, metaPath); err != nil {
		_ = os.Remove(tmpPath)
		return NewBusinessError(500, ErrCodeImageResultSaveFailed, fmt.Sprintf("保存元数据失败: %v", err))
	}
	return nil
}

func (d *ResultDownloader) EnsureAttemptDir(taskID, actionKey string, attempt int) (string, error) {
	attemptName := fmt.Sprintf(resultAttemptDirFmt, attempt)
	dir := filepath.Join(
		config.AppCfg.Storage.DataDir,
		"desktop-pets",
		"generation-tasks",
		taskID,
		"generated",
		actionKey,
		attemptName,
	)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func buildResultRelativePath(taskID, actionKey string, attempt int, fileName string) string {
	attemptName := fmt.Sprintf(resultAttemptDirFmt, attempt)
	return filepath.ToSlash(filepath.Join(
		"desktop-pets",
		"generation-tasks",
		taskID,
		"generated",
		actionKey,
		attemptName,
		resultRawDirName,
		fileName,
	))
}

func sanitizeMetadata(value map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(value))
	for k, v := range value {
		lowerKey := strings.ToLower(k)
		if isSensitiveKey(lowerKey) {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = sanitizeValue(v)
	}
	return out
}

func sanitizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return sanitizeMetadata(val)
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, item := range val {
			out[i] = sanitizeValue(item)
		}
		return out
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	sensitiveFragments := []string{
		"api_key", "apikey", "authorization", "token", "secret", "password", "credential", "private_key",
	}
	for _, frag := range sensitiveFragments {
		if strings.Contains(key, frag) {
			return true
		}
	}
	return false
}
