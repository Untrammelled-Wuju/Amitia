// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
)

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func SaveImageFromDataURI(imageUrl string) string {
	if !strings.HasPrefix(imageUrl, "data:") {
		return imageUrl
	}
	imgDir := filepath.Join(config.AppCfg.Storage.DataDir, "images")
	os.MkdirAll(imgDir, 0755)
	idx := strings.Index(imageUrl, ";base64,")
	if idx <= 0 {
		return imageUrl
	}
	mimePart := imageUrl[5:idx]
	ext := ".png"
	if strings.Contains(mimePart, "jpeg") || strings.Contains(mimePart, "jpg") {
		ext = ".jpg"
	}
	fname := uuid.New().String() + ext
	data, err := base64.StdEncoding.DecodeString(imageUrl[idx+8:])
	if err != nil {
		return imageUrl
	}
	os.WriteFile(filepath.Join(imgDir, fname), data, 0644)
	return "/images/" + fname
}
