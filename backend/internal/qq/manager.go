// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusOnline       Status = "online"
)

type Manager struct {
	sidecarURL   string
	httpClient   *http.Client
	mu           sync.RWMutex
	status       Status
	accountID    string
	appID        string
	token        string
	sandbox      bool
	lastError    string
	messageCount int64
}

type Msg struct {
	FromUserID string
	ToUserID   string
	MessageID  string
	Text       string
	GroupID    string
	CreatedAt  int64
}

func NewManager(sidecarURL string) *Manager {
	m := &Manager{
		sidecarURL: sidecarURL,
		status:     StatusDisconnected,
	}
	m.httpClient = &http.Client{
		Timeout: 15 * time.Second,
	}
	return m
}

func (m *Manager) SetMessageHandler(handler func(msg *Msg)) {}

func (m *Manager) GetStatus() Status      { m.refreshFromSidecar(); return m.status }
func (m *Manager) GetAccountID() string   { m.refreshFromSidecar(); return m.accountID }
func (m *Manager) GetLastError() string   { m.refreshFromSidecar(); return m.lastError }
func (m *Manager) GetMessageCount() int64 { m.refreshFromSidecar(); return m.messageCount }
func (m *Manager) IsOnline() bool         { m.refreshFromSidecar(); return m.status == StatusOnline }
func (m *Manager) GetAppID() string       { return m.appID }
func (m *Manager) GetSandbox() bool       { return m.sandbox }

func (m *Manager) Connect(appID, token string, sandbox bool) error {
	m.mu.Lock()
	m.appID = appID
	m.token = token
	m.sandbox = sandbox
	m.status = StatusConnecting
	m.mu.Unlock()

	body := fmt.Sprintf(`{"appId":"%s","token":"%s","sandbox":%v}`, appID, token, sandbox)
	resp, err := m.httpClient.Post(m.sidecarURL+"/api/connect", "application/json", strings.NewReader(body))
	if err != nil {
		logrus.Errorf("[QQ] 连接QQBot失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := string(bodyBytes)
		logrus.Errorf("[QQ] QQBot连接返回错误: %s", errMsg)
		return fmt.Errorf("QQBot连接失败: %s", errMsg)
	}

	logrus.Infof("[QQ] QQBot连接请求已发送 appId=%s", appID)
	return nil
}

func (m *Manager) Disconnect() {
	logrus.Info("[QQ] 断开QQBot连接")
	resp, err := m.httpClient.Post(m.sidecarURL+"/api/disconnect", "application/json", nil)
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

	logrus.Infof("[QQ] 群消息已发送 to=%s", groupID)
	return nil
}

func (m *Manager) refreshFromSidecar() {
	resp, err := m.httpClient.Get(m.sidecarURL + "/api/status")
	if err != nil {
		m.status = StatusDisconnected
		return
	}
	defer resp.Body.Close()
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			QQOnline     bool   `json:"qqOnline"`
			Status       string `json:"status"`
			AccountID    string `json:"accountId"`
			Error        string `json:"error"`
			MessageCount int64  `json:"messageCount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		m.status = StatusDisconnected
		return
	}
	switch result.Data.Status {
	case "online":
		m.status = StatusOnline
	case "connecting":
		m.status = StatusConnecting
	default:
		m.status = StatusDisconnected
	}
	m.accountID = result.Data.AccountID
	m.lastError = result.Data.Error
	m.messageCount = result.Data.MessageCount
}
