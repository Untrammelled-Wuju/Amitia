// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"bytes"
	"context"
	"encoding/base64"
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

const uploadTimeout = 45 * time.Second

type uploadResult struct {
	RemoteID  string
	RemoteURL string
}

func screenshotExtension(mediaType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/png":
		return ".png", true
	case "image/jpeg", "image/jpg":
		return ".jpg", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

func (s *Service) saveScreenshot(ticketID string, input *ScreenshotInput) (name, path, mediaType string, size int64, err error) {
	if input == nil {
		return "", "", "", 0, nil
	}
	extension, ok := screenshotExtension(input.MediaType)
	if !ok {
		return "", "", "", 0, fmt.Errorf("不支持的截图格式")
	}
	data, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(input.DataBase64))
	if decodeErr != nil {
		return "", "", "", 0, fmt.Errorf("截图数据无效")
	}
	if len(data) == 0 {
		return "", "", "", 0, fmt.Errorf("截图数据为空")
	}
	if len(data) > maxScreenshotBytes {
		return "", "", "", 0, fmt.Errorf("截图过大，最大支持 %d MB", maxScreenshotBytes/1024/1024)
	}
	directory := screenshotDirectory()
	if mkdirErr := os.MkdirAll(directory, 0o700); mkdirErr != nil {
		return "", "", "", 0, mkdirErr
	}
	target := filepath.Join(directory, ticketID+extension)
	if writeErr := os.WriteFile(target, data, 0o600); writeErr != nil {
		return "", "", "", 0, writeErr
	}
	displayName := strings.TrimSpace(input.Name)
	if displayName == "" {
		displayName = "screenshot" + extension
	}
	return displayName, target, strings.ToLower(strings.TrimSpace(input.MediaType)), int64(len(data)), nil
}

func (s *Service) uploadTicket(ctx context.Context, ticket *Ticket, snapshot *DiagnosticsSnapshot) (*uploadResult, error) {
	uploadURL := strings.TrimSpace(ticket.UploadURL)
	if uploadURL == "" {
		uploadURL = s.uploadURL()
	}
	if !strings.HasPrefix(uploadURL, "http://") && !strings.HasPrefix(uploadURL, "https://") {
		return nil, fmt.Errorf("上报地址无效，必须以 http:// 或 https:// 开头")
	}

	payload := buildPayload(ticket, snapshot)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	encoded, contentType, err := encodeUpload(ticket, body)
	if err != nil {
		return nil, err
	}

	clients := uploadClients(uploadURL)
	var lastErr error
	for _, client := range clients {
		result, sendErr := sendUpload(ctx, client, uploadURL, contentType, encoded)
		if sendErr == nil {
			return result, nil
		}
		lastErr = sendErr
	}
	return nil, lastErr
}

func encodeUpload(ticket *Ticket, payload []byte) ([]byte, string, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	if err := writer.WriteField("payload", string(payload)); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("source", "amitia-core"); err != nil {
		return nil, "", err
	}
	if ticket.HasScreenshot && ticket.ScreenshotPath != "" {
		file, openErr := os.Open(ticket.ScreenshotPath)
		if openErr == nil {
			part, createErr := writer.CreateFormFile("screenshot", ticket.ScreenshotName)
			if createErr == nil {
				_, _ = io.Copy(part, file)
			}
			file.Close()
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func uploadClients(uploadURL string) []*http.Client {
	directTransport := http.DefaultTransport.(*http.Transport).Clone()
	directTransport.Proxy = nil
	clients := []*http.Client{{Transport: directTransport, Timeout: uploadTimeout}}
	probe, err := http.NewRequest(http.MethodPost, uploadURL, nil)
	if err != nil {
		return clients
	}
	if proxyURL, proxyErr := http.ProxyFromEnvironment(probe); proxyErr == nil && proxyURL != nil {
		clients = append(clients, &http.Client{Timeout: uploadTimeout})
	}
	return clients
}

func sendUpload(ctx context.Context, client *http.Client, uploadURL, contentType string, body []byte) (*uploadResult, error) {
	requestCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, uploadURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("User-Agent", "Amitia-Core-Feedback/1.0")
	if token := strings.TrimSpace(os.Getenv("AMITIA_FEEDBACK_TOKEN")); token != "" {
		request.Header.Set("X-Amitia-Feedback-Token", token)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("上报失败，状态码 %d: %s", response.StatusCode, strings.TrimSpace(string(raw)))
	}

	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	result := &uploadResult{}
	if jsonErr := json.Unmarshal(raw, &envelope); jsonErr == nil && len(envelope.Data) > 0 {
		var data struct {
			ID  any    `json:"id"`
			URL string `json:"url"`
		}
		if dataErr := json.Unmarshal(envelope.Data, &data); dataErr == nil {
			result.RemoteID = stringifyID(data.ID)
			result.RemoteURL = strings.TrimSpace(data.URL)
		}
	}
	return result, nil
}

func buildPayload(ticket *Ticket, snapshot *DiagnosticsSnapshot) map[string]any {
	if snapshot == nil {
		snapshot = &DiagnosticsSnapshot{}
	}
	logLines := snapshot.RecentLines
	errorLines := snapshot.ErrorLines
	clientErrors := snapshot.ClientErrors
	if logLines == nil {
		logLines = []LogLine{}
	}
	if errorLines == nil {
		errorLines = []LogLine{}
	}
	if clientErrors == nil {
		clientErrors = []ClientError{}
	}
	if snapshot.LogFiles == nil {
		snapshot.LogFiles = []string{}
	}
	if snapshot.DataSummary == nil {
		snapshot.DataSummary = map[string]string{}
	}
	coreVersion := snapshot.CoreVersion
	redactedCount := snapshot.RedactedCount
	return map[string]any{
		"ticketId":       ticket.ID,
		"category":       ticket.Category,
		"title":          ticket.Title,
		"description":    ticket.Description,
		"contact":        ticket.Contact,
		"createdAt":      ticket.CreatedAt,
		"platform":       ticket.Platform,
		"clientType":     ticket.ClientType,
		"appVersion":     ticket.AppVersion,
		"coreVersion":    coreVersion,
		"runtimeMode":    ticket.RuntimeMode,
		"osInfo":         ticket.OSInfo,
		"deviceHash":     ticket.DeviceHash,
		"locale":         ticket.Locale,
		"timezone":       ticket.Timezone,
		"includeLogs":    ticket.IncludeLogs,
		"logLineCount":   ticket.LogLineCount,
		"redactedCount":  redactedCount,
		"screenshotName": ticket.ScreenshotName,
		"screenshotSize": ticket.ScreenshotSize,
		"hasScreenshot":  ticket.HasScreenshot,
		"logFiles":       snapshot.LogFiles,
		"errorLines":     errorLines,
		"recentLines":    logLines,
		"clientErrors":   clientErrors,
		"dataSummary":    snapshot.DataSummary,
	}
}

func stringifyID(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}
