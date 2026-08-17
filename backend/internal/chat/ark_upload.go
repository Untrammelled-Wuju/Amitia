// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func uploadFileToArk(baseURL, apiKey, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()
	return uploadStreamToArk(baseURL, apiKey, file, filepath.Base(filePath))
}

func uploadStreamToArk(baseURL, apiKey string, reader io.Reader, fileName string) (string, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	_ = writer.WriteField("purpose", "user_data")

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return "", err
	}
	writer.Close()

	req, _ := http.NewRequest("POST", strings.TrimRight(baseURL, "/")+"/files", &requestBody)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("文件上传失败 (status=%d): %s", resp.StatusCode, string(body[:min(len(body), 300)]))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析上传响应失败: %w", err)
	}
	fileID, _ := result["id"].(string)
	if fileID == "" {
		return "", fmt.Errorf("未获取到file_id: %s", string(body[:min(len(body), 300)]))
	}
	return fileID, nil
}
