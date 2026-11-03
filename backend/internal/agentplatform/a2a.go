package agentplatform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RegisterA2ACardInput struct {
	AgentID     string         `json:"agentId"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	URL         string         `json:"url"`
	Version     string         `json:"version"`
	Skills      any            `json:"skills"`
	Security    map[string]any `json:"security"`
}

type A2ATaskInput struct {
	AgentID        string `json:"agentId"`
	RemoteAgentURL string `json:"remoteAgentUrl"`
	Input          any    `json:"input"`
}

type InboundA2ATaskInput struct {
	AgentID       string `json:"agentId"`
	Input         any    `json:"input"`
	RequestID     string `json:"requestId"`
	SenderAgentID string `json:"senderAgentId,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	Nonce         string `json:"nonce,omitempty"`
	Signature     string `json:"signature,omitempty"`
	RawBody       []byte `json:"-"`
}

type A2ATaskResult struct {
	Task   A2ATask `json:"task"`
	Remote any     `json:"remote,omitempty"`
}

func (s *Service) RegisterA2ACard(ctx context.Context, spaceID string, input RegisterA2ACardInput) (A2AAgentCard, error) {
	if strings.TrimSpace(input.AgentID) == "" || strings.TrimSpace(input.URL) == "" {
		return A2AAgentCard{}, errors.New("A2A Agent ID 和地址不能为空")
	}
	if err := validateA2AEndpoint(input.URL); err != nil {
		return A2AAgentCard{}, err
	}
	publicKey, err := s.a2aPublicKey()
	if err != nil {
		return A2AAgentCard{}, err
	}
	security := map[string]any{}
	for key, value := range input.Security {
		security[key] = value
	}
	if existing, _ := security["publicKey"].(string); strings.TrimSpace(existing) == "" {
		security["publicKey"] = publicKey
	}
	customKey, _ := security["publicKey"].(string)
	decodedKey, decodeErr := base64.RawStdEncoding.DecodeString(customKey)
	if decodeErr != nil {
		decodedKey, decodeErr = base64.StdEncoding.DecodeString(customKey)
	}
	if decodeErr != nil || len(decodedKey) != ed25519.PublicKeySize {
		return A2AAgentCard{}, errors.New("A2A Agent Card 公钥无效")
	}
	security["signatureAlgorithm"] = "ed25519"
	security["replayWindowSeconds"] = 300
	skillsJSON, _, err := hashJSON(input.Skills)
	if err != nil {
		return A2AAgentCard{}, err
	}
	securityJSON, _, err := hashJSON(security)
	if err != nil {
		return A2AAgentCard{}, err
	}
	now := time.Now().UTC()
	var card A2AAgentCard
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("space_id = ? AND agent_id = ?", spaceID, input.AgentID).First(&card).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			card = A2AAgentCard{
				ID: newID("a2acard"), SpaceID: spaceID, AgentID: input.AgentID, CreatedAt: now,
			}
		} else if err != nil {
			return fmt.Errorf("读取 A2A Agent Card: %w", err)
		}
		card.Name = firstNonEmpty(input.Name, input.AgentID)
		card.Description = input.Description
		card.URL = strings.TrimSpace(input.URL)
		card.Version = firstNonEmpty(input.Version, "1.0.0")
		card.SkillsJSON = skillsJSON
		card.SecurityJSON = securityJSON
		card.Status = "active"
		card.UpdatedAt = now
		return tx.Save(&card).Error
	})
	if err != nil {
		return A2AAgentCard{}, fmt.Errorf("注册 A2A Agent Card: %w", err)
	}
	return card, nil
}

func (s *Service) ListA2ACards(ctx context.Context, spaceID string) ([]A2AAgentCard, error) {
	cards := make([]A2AAgentCard, 0)
	if err := s.db.WithContext(ctx).Where("space_id = ? AND status = ?", spaceID, "active").Order("updated_at DESC").Find(&cards).Error; err != nil {
		return nil, fmt.Errorf("读取 A2A Agent Card: %w", err)
	}
	return cards, nil
}

func (s *Service) SubmitA2ATask(ctx context.Context, spaceID string, input A2ATaskInput) (A2ATaskResult, error) {
	endpoint := strings.TrimSpace(input.RemoteAgentURL)
	if endpoint == "" {
		return A2ATaskResult{}, errors.New("远程 Agent 地址不能为空")
	}
	if err := validateA2AEndpoint(endpoint); err != nil {
		return A2ATaskResult{}, err
	}
	if err := validateA2AResolvedEndpoint(ctx, endpoint); err != nil {
		return A2ATaskResult{}, err
	}
	var senderCard A2AAgentCard
	if err := s.db.WithContext(ctx).Where("space_id = ? AND agent_id = ? AND status = ?", spaceID, input.AgentID, "active").First(&senderCard).Error; err != nil {
		return A2ATaskResult{}, fmt.Errorf("发送 A2A 任务前必须注册 Agent Card: %w", err)
	}
	requestID := newID("a2a-request")
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		return A2ATaskResult{}, fmt.Errorf("生成 A2A 防重放随机数: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	payload := map[string]any{
		"id": requestID, "agentId": input.AgentID, "senderAgentId": input.AgentID,
		"input": input.Input, "createdAt": timestamp, "timestamp": timestamp, "nonce": nonce,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return A2ATaskResult{}, fmt.Errorf("编码 A2A 任务: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return A2ATaskResult{}, fmt.Errorf("创建 A2A 请求: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Amitia-A2A-Agent", input.AgentID)
	request.Header.Set("X-Amitia-A2A-Timestamp", timestamp)
	request.Header.Set("X-Amitia-A2A-Nonce", nonce)
	signature, err := s.signA2ABody(body)
	if err != nil {
		return A2ATaskResult{}, err
	}
	request.Header.Set("X-Amitia-A2A-Signature", signature)
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return A2ATaskResult{}, fmt.Errorf("调用远程 Agent: %w", err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if readErr != nil {
		return A2ATaskResult{}, fmt.Errorf("读取远程 Agent 响应: %w", readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return A2ATaskResult{}, fmt.Errorf("远程 Agent 返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var remote any
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &remote); err != nil {
			return A2ATaskResult{}, fmt.Errorf("解析远程 Agent 响应: %w", err)
		}
	}
	inputJSON, _, err := hashJSON(input.Input)
	if err != nil {
		return A2ATaskResult{}, err
	}
	outputJSON, _, err := hashJSON(remote)
	if err != nil {
		return A2ATaskResult{}, err
	}
	now := time.Now().UTC()
	task := A2ATask{
		ID: newID("a2atask"), SpaceID: spaceID, AgentID: input.AgentID,
		RemoteAgentURL: endpoint, Status: "completed", InputJSON: inputJSON,
		OutputJSON: outputJSON, CreatedAt: now, UpdatedAt: now, CompletedAt: &now,
	}
	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		return A2ATaskResult{}, fmt.Errorf("保存 A2A 任务: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: input.AgentID, Kind: "a2a.task", State: "completed", Title: "远程 Agent 任务已完成", Detail: endpoint}); err != nil {
		return A2ATaskResult{}, err
	}
	return A2ATaskResult{Task: task, Remote: remote}, nil
}

func (s *Service) ListA2ATasks(ctx context.Context, spaceID, agentID string, limit int) ([]A2ATask, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(agentID) != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	tasks := make([]A2ATask, 0)
	if err := query.Order("created_at DESC").Limit(trimLimit(limit, 100, 500)).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("读取 A2A 任务: %w", err)
	}
	return tasks, nil
}

func (s *Service) CancelA2ATask(ctx context.Context, spaceID, taskID, remoteCancelURL string) (A2ATask, error) {
	var task A2ATask
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", taskID, spaceID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return A2ATask{}, errors.New("A2A 任务不存在")
		}
		return A2ATask{}, fmt.Errorf("读取 A2A 任务: %w", err)
	}
	if task.Status == "completed" {
		return A2ATask{}, errors.New("已完成的 A2A 任务不能取消")
	}
	if strings.TrimSpace(remoteCancelURL) != "" {
		if err := validateA2AEndpoint(remoteCancelURL); err != nil {
			return A2ATask{}, err
		}
		if err := validateA2AResolvedEndpoint(ctx, remoteCancelURL); err != nil {
			return A2ATask{}, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodDelete, remoteCancelURL, nil)
		if err != nil {
			return A2ATask{}, fmt.Errorf("创建 A2A 取消请求: %w", err)
		}
		response, err := (&http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}).Do(request)
		if err != nil {
			return A2ATask{}, fmt.Errorf("取消远程 Agent 任务: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil {
			return A2ATask{}, fmt.Errorf("读取 A2A 取消响应: %w", readErr)
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return A2ATask{}, fmt.Errorf("远程 Agent 取消失败 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		}
	}
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&A2ATask{}).Where("id = ?", task.ID).Updates(map[string]any{"status": "canceled", "updated_at": now}).Error; err != nil {
		return A2ATask{}, fmt.Errorf("保存 A2A 取消状态: %w", err)
	}
	task.Status, task.UpdatedAt = "canceled", now
	return task, nil
}

func (s *Service) GetPublicA2ACard(ctx context.Context, spaceID, agentID string) (A2AAgentCard, error) {
	var card A2AAgentCard
	if err := s.db.WithContext(ctx).Where("space_id = ? AND agent_id = ? AND status = ?", spaceID, agentID, "active").First(&card).Error; err != nil {
		return A2AAgentCard{}, fmt.Errorf("读取公开 A2A Agent Card: %w", err)
	}
	return card, nil
}

func (s *Service) HandleInboundA2ATask(ctx context.Context, spaceID string, input InboundA2ATaskInput) (A2ATaskResult, error) {
	if strings.TrimSpace(input.AgentID) == "" {
		return A2ATaskResult{}, errors.New("A2A 目标 Agent 不能为空")
	}
	if s.a2aExecutor == nil {
		return A2ATaskResult{}, errors.New("A2A 执行器不可用")
	}
	if err := s.verifyInboundA2A(ctx, spaceID, input); err != nil {
		return A2ATaskResult{}, err
	}
	inputJSON, _, err := hashJSON(input.Input)
	if err != nil {
		return A2ATaskResult{}, err
	}
	now := time.Now().UTC()
	task := A2ATask{
		ID: newID("a2atask"), SpaceID: spaceID, AgentID: input.AgentID,
		Status: "running", InputJSON: inputJSON, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		return A2ATaskResult{}, fmt.Errorf("保存入站 A2A 任务: %w", err)
	}
	output, executeErr := s.a2aExecutor.ExecuteA2ATask(ctx, spaceID, input.AgentID, input.Input)
	if executeErr != nil {
		original := executeErr.Error()
		if err := s.db.WithContext(ctx).Model(&A2ATask{}).Where("id = ?", task.ID).Updates(map[string]any{
			"status": "failed", "original_error": original, "updated_at": time.Now().UTC(),
		}).Error; err != nil {
			return A2ATaskResult{}, fmt.Errorf("%w; 保存 A2A 失败状态: %v", executeErr, err)
		}
		return A2ATaskResult{}, fmt.Errorf("执行入站 A2A 任务: %w", executeErr)
	}
	outputJSON, _, err := hashJSON(output)
	if err != nil {
		return A2ATaskResult{}, err
	}
	completed := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&A2ATask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status": "completed", "output_json": outputJSON, "completed_at": completed, "updated_at": completed,
	}).Error; err != nil {
		return A2ATaskResult{}, fmt.Errorf("保存入站 A2A 结果: %w", err)
	}
	task.Status, task.OutputJSON, task.CompletedAt, task.UpdatedAt = "completed", outputJSON, &completed, completed
	if err := s.recordActivity(ctx, ActivityEvent{
		SpaceID: spaceID, AgentID: input.AgentID, ConversationID: "a2a-" + input.AgentID,
		Kind: "a2a.inbound", State: "completed", Title: "入站 A2A 任务已完成",
		Detail: input.RequestID,
	}); err != nil {
		return A2ATaskResult{}, err
	}
	return A2ATaskResult{Task: task, Remote: output}, nil
}

func (s *Service) verifyInboundA2A(ctx context.Context, spaceID string, input InboundA2ATaskInput) error {
	var target AgentProfile
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ? AND status = ? AND deleted_at IS NULL", input.AgentID, spaceID, "active").First(&target).Error; err != nil {
		return fmt.Errorf("A2A 目标 Agent 未发布或不可用: %w", err)
	}
	if strings.TrimSpace(input.SenderAgentID) == "" || strings.TrimSpace(input.Signature) == "" || strings.TrimSpace(input.Nonce) == "" || len(input.RawBody) == 0 {
		return errors.New("A2A 入站请求缺少发送 Agent 签名")
	}
	var signedBody struct {
		AgentID       string `json:"agentId"`
		SenderAgentID string `json:"senderAgentId"`
		RequestID     string `json:"requestId"`
		ID            string `json:"id"`
		Nonce         string `json:"nonce"`
		Timestamp     string `json:"timestamp"`
	}
	if err := json.Unmarshal(input.RawBody, &signedBody); err != nil {
		return fmt.Errorf("解析 A2A 签名正文: %w", err)
	}
	if signedBody.AgentID != input.AgentID || signedBody.SenderAgentID != input.SenderAgentID ||
		signedBody.Nonce != input.Nonce || signedBody.Timestamp != input.Timestamp ||
		(signedBody.RequestID != input.RequestID && signedBody.ID != input.RequestID) {
		return errors.New("A2A 签名正文与请求头或任务身份不一致")
	}
	sentAt, err := time.Parse(time.RFC3339Nano, input.Timestamp)
	if err != nil {
		return fmt.Errorf("解析 A2A 签名时间: %w", err)
	}
	now := time.Now().UTC()
	if sentAt.Before(now.Add(-5*time.Minute)) || sentAt.After(now.Add(time.Minute)) {
		return errors.New("A2A 请求签名时间超出允许窗口")
	}
	var card A2AAgentCard
	if err := s.db.WithContext(ctx).Where("space_id = ? AND agent_id = ? AND status = ?", spaceID, input.SenderAgentID, "active").First(&card).Error; err != nil {
		return fmt.Errorf("发送 A2A 任务的远端 Agent 未登记: %w", err)
	}
	var security map[string]any
	if err := json.Unmarshal([]byte(card.SecurityJSON), &security); err != nil {
		return fmt.Errorf("解析发送 Agent 公钥: %w", err)
	}
	publicKeyText, _ := security["publicKey"].(string)
	publicKey, err := base64.RawStdEncoding.DecodeString(publicKeyText)
	if err != nil {
		publicKey, err = base64.StdEncoding.DecodeString(publicKeyText)
	}
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return errors.New("发送 Agent 公钥无效")
	}
	signature, err := base64.RawURLEncoding.DecodeString(input.Signature)
	if err != nil {
		signature, err = base64.StdEncoding.DecodeString(input.Signature)
	}
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), input.RawBody, signature) {
		return errors.New("A2A 请求签名校验失败")
	}
	sum := sha256.Sum256([]byte(input.Signature))
	if err := s.db.WithContext(ctx).Where("expires_at <= ?", now).Delete(&A2AReplayRecord{}).Error; err != nil {
		return fmt.Errorf("清理过期 A2A 防重放记录: %w", err)
	}
	record := A2AReplayRecord{
		ID: newID("a2areplay"), SpaceID: spaceID, SenderAgentID: input.SenderAgentID,
		Nonce: input.Nonce, SignatureHash: hex.EncodeToString(sum[:]),
		ExpiresAt: sentAt.Add(10 * time.Minute), CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("拒绝重复或并发的 A2A 请求: %w", err)
	}
	return nil
}

func (s *Service) a2aPublicKey() (string, error) {
	key, err := s.a2aSigningKey()
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), nil
}

func (s *Service) signA2ABody(body []byte) (string, error) {
	key, err := s.a2aSigningKey()
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, body)), nil
}

func (s *Service) a2aSigningKey() (ed25519.PrivateKey, error) {
	if s == nil {
		return nil, errors.New("A2A 签名服务不可用")
	}
	s.a2aKeyMu.Lock()
	defer s.a2aKeyMu.Unlock()
	if len(s.a2aPrivateKey) == ed25519.PrivateKeySize {
		return s.a2aPrivateKey, nil
	}
	root := filepath.Join(s.dataDir, "agent-platform", "a2a")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("创建 A2A 密钥目录: %w", err)
	}
	keyPath := filepath.Join(root, "ed25519-private.key")
	body, err := os.ReadFile(keyPath)
	if err == nil {
		if len(body) != ed25519.PrivateKeySize {
			return nil, errors.New("A2A Ed25519 私钥长度无效")
		}
		s.a2aPrivateKey = ed25519.PrivateKey(append([]byte(nil), body...))
		return s.a2aPrivateKey, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取 A2A 签名私钥: %w", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成 A2A 签名私钥: %w", err)
	}
	temp, err := os.CreateTemp(root, ".a2a-key-*")
	if err != nil {
		return nil, fmt.Errorf("创建 A2A 私钥临时文件: %w", err)
	}
	tempName := temp.Name()
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return nil, fmt.Errorf("设置 A2A 私钥权限: %w", err)
	}
	if _, err := temp.Write(privateKey); err != nil {
		temp.Close()
		return nil, fmt.Errorf("写入 A2A 签名私钥: %w", err)
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("关闭 A2A 签名私钥: %w", err)
	}
	if err := os.Rename(tempName, keyPath); err != nil {
		return nil, fmt.Errorf("保存 A2A 签名私钥: %w", err)
	}
	s.a2aPrivateKey = privateKey
	return s.a2aPrivateKey, nil
}

func validateA2AEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("解析远程 Agent 地址: %w", err)
	}
	if parsed.Hostname() == "" {
		return errors.New("远程 Agent 地址缺少主机名")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1")) {
		return errors.New("远程 Agent 必须使用 HTTPS，本机回环地址除外")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && ip.IsPrivate() && !ip.IsLoopback() {
		return errors.New("远程 Agent 地址不能指向私有网络")
	}
	return nil
}

func validateA2AResolvedEndpoint(ctx context.Context, endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("解析远程 Agent 主机: %w", err)
	}
	for _, address := range addresses {
		if address.IP.IsPrivate() || address.IP.IsLoopback() || address.IP.IsLinkLocalUnicast() || address.IP.IsUnspecified() {
			return errors.New("远程 Agent DNS 指向私有或本机网络")
		}
	}
	return nil
}
