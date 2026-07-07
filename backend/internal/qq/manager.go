// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
)

type Manager struct {
	appID      string
	token      string
	sandbox    bool
	sidecarURL string
	httpClient *http.Client
	mu         sync.RWMutex
	status     Status
	accountID  string
	lastError  string
	startedAt  string
}

func NewManager(sidecarURL string) *Manager {
	return &Manager{
		sidecarURL: sidecarURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		status:     StatusDisconnected,
	}
}

func (m *Manager) IsOnline() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status == StatusConnected
}

func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) GetAccountID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accountID
}

func (m *Manager) GetLastError() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastError
}

func (m *Manager) GetAppID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.appID
}

func (m *Manager) GetSandbox() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sandbox
}

func (m *Manager) GetStartedAt() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.startedAt
}

func (m *Manager) FetchSidecarConfig() map[string]interface{} {
	resp, err := m.httpClient.Get(m.sidecarURL + "/api/config")
	if err != nil {
		return map[string]interface{}{"appId": "", "token": "", "sandbox": false}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if result == nil {
		result = map[string]interface{}{"appId": "", "token": "", "sandbox": false}
	}
	return result
}

func (m *Manager) FetchSidecarStatus() (bool, Status, string, string, string) {
	resp, err := m.httpClient.Get(m.sidecarURL + "/api/status")
	if err != nil {
		return false, StatusDisconnected, "", err.Error(), ""
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	data, _ := result["data"].(map[string]interface{})
	if data == nil {
		return false, StatusDisconnected, "", "", ""
	}
	online, _ := data["qqOnline"].(bool)
	statusStr, _ := data["status"].(string)
	accountID, _ := data["accountId"].(string)
	errStr, _ := data["error"].(string)
	var s Status
	switch statusStr {
	case "online":
		s = StatusConnected
	case "connecting":
		s = StatusConnecting
	default:
		s = StatusDisconnected
	}
	startedAt, _ := data["startedAt"].(string)
	return online, s, accountID, errStr, startedAt
}

func (m *Manager) SetOnline(accountID string, startedAt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = StatusConnected
	m.accountID = accountID
	m.startedAt = startedAt
}

func (m *Manager) Connect(appID, token string, sandbox bool) error {
	m.mu.Lock()
	m.appID = appID
	m.token = token
	m.sandbox = sandbox
	m.status = StatusConnecting
	m.lastError = ""
	m.mu.Unlock()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"appId":   appID,
		"token":   token,
		"sandbox": sandbox,
	})
	resp, err := m.httpClient.Post(m.sidecarURL+"/api/connect", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		m.mu.Lock()
		m.status = StatusDisconnected
		m.lastError = err.Error()
		m.mu.Unlock()
		logrus.Errorf("[QQ] 连接失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		m.mu.Lock()
		m.status = StatusDisconnected
		m.lastError = fmt.Sprintf("连接返回 %d", resp.StatusCode)
		m.mu.Unlock()
		return fmt.Errorf("QQBot连接返回 %d", resp.StatusCode)
	}

	m.mu.Lock()
	m.status = StatusConnected
	m.mu.Unlock()

	logrus.Infof("[QQ] 连接成功 appId=%s sandbox=%v", appID, sandbox)
	return nil
}

func (m *Manager) Disconnect() {
	reqBody, _ := json.Marshal(map[string]string{})
	resp, err := m.httpClient.Post(m.sidecarURL+"/api/disconnect", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		logrus.Errorf("[QQ] 断开连接失败: %v", err)
		return
	}
	defer resp.Body.Close()

	m.mu.Lock()
	m.status = StatusDisconnected
	m.mu.Unlock()
}

func (m *Manager) SendPrivateMsg(userID string, text string) error {
	if !m.IsOnline() {
		return fmt.Errorf("QQBot未连接")
	}

	reqBody, _ := json.Marshal(map[string]string{
		"toUserId": userID,
		"text":     text,
	})

	resp, err := m.httpClient.Post(m.sidecarURL+"/api/send", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		logrus.Errorf("[QQ] 发送私聊失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("发送失败 (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	logrus.Infof("[QQ] 私聊已发送 to=%s", userID)
	return nil
}

func (m *Manager) SendGroupMsg(groupID string, text string) error {
	if !m.IsOnline() {
		return fmt.Errorf("QQBot未连接")
	}

	reqBody, _ := json.Marshal(map[string]string{
		"groupId": groupID,
		"text":    text,
	})

	resp, err := m.httpClient.Post(m.sidecarURL+"/api/send", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		logrus.Errorf("[QQ] 发送群消息失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("发送失败 (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	logrus.Infof("[QQ] 群消息已发送 group=%s", groupID)
	return nil
}
