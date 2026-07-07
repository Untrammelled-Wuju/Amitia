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
