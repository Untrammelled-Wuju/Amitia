// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

const maxReferenceImageSize = 10 * 1024 * 1024

var allowedReferenceImageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
}

type ReferenceImageInfo struct {
	Path         string
	OriginalName string
	MimeType     string
	Size         int
	Width        int
	Height       int
	Hash         string
}

func ValidateAndSaveReferenceImage(fileHeader *multipart.FileHeader, taskDir string, relativeBase string) (*ReferenceImageInfo, error) {
	if fileHeader == nil {
		return nil, NewBusinessError(400, ErrCodeReferenceImageRequired, "缺少参考图片")
	}
	if fileHeader.Size <= 0 {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "参考图片为空")
	}
	if fileHeader.Size > maxReferenceImageSize {
		return nil, NewBusinessError(400, ErrCodeReferenceImageTooLarge, "参考图片大小不能超过 10MB")
	}
	originalName := fileHeader.Filename
	if strings.Contains(originalName, "..") || strings.ContainsAny(originalName, `/\`) {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "参考图片文件名非法")
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	expectedMime, ok := allowedReferenceImageExts[ext]
	if !ok {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "不支持的参考图片格式,仅允许 png/jpg/jpeg/webp")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "读取参考图片失败")
	}
	defer src.Close()

	content, err := io.ReadAll(io.LimitReader(src, maxReferenceImageSize+1))
	if err != nil {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "读取参考图片内容失败")
	}
	if int64(len(content)) > maxReferenceImageSize {
		return nil, NewBusinessError(400, ErrCodeReferenceImageTooLarge, "参考图片大小不能超过 10MB")
	}

	detectedMime := http.DetectContentType(content)
	if !strings.HasPrefix(detectedMime, "image/") {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "参考图片内容不是有效的图片")
	}
	if detectedMime != expectedMime {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "参考图片实际类型与扩展名不匹配")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "解析参考图片失败")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, NewBusinessError(400, ErrCodeReferenceImageInvalid, "参考图片尺寸无效")
	}

	sourceDir := filepath.Join(taskDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return nil, NewBusinessError(500, ErrCodeReferenceImageInvalid, "创建参考图片目录失败")
	}
	saveName := "reference" + ext
	savePath := filepath.Join(sourceDir, saveName)
	if err := os.WriteFile(savePath, content, 0644); err != nil {
		return nil, NewBusinessError(500, ErrCodeReferenceImageInvalid, "保存参考图片失败")
	}

	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])

	relPath := filepath.ToSlash(filepath.Join(relativeBase, "source", saveName))

	return &ReferenceImageInfo{
		Path:         relPath,
		OriginalName: originalName,
		MimeType:     detectedMime,
		Size:         len(content),
		Width:        cfg.Width,
		Height:       cfg.Height,
		Hash:         hash,
	}, nil
}
