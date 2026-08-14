// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mnn

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/chat/localmodel"
	"github.com/u-ai/backend/internal/runtimehost"
)

type mnnRuntime struct {
	config     MNNProviderConfig
	mu         sync.Mutex
	state      string
	loadedAt   string
	lastError  string
	host       runtimehost.RuntimeHost
	processID  runtimehost.ProcessID
	baseURL    string
	port       int
	supervisor runtimehost.ProcessSupervisor
	modelPath  string
}

var (
	mnnDefaultPort = 19999
	mnnPortMutex   sync.Mutex
	mnnAllocatedPorts = make(map[int]bool)
)

func findMNNAvailablePort() int {
	mnnPortMutex.Lock()
	defer mnnPortMutex.Unlock()
	for p := mnnDefaultPort; p < 65535; p++ {
		if mnnAllocatedPorts[p] {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			continue
		}
		ln.Close()
		mnnAllocatedPorts[p] = true
		return p
	}
	return 0
}

func releaseMNNPort(p int) {
	mnnPortMutex.Lock()
	defer mnnPortMutex.Unlock()
	delete(mnnAllocatedPorts, p)
}

func newMNNRuntime(config MNNProviderConfig) *mnnRuntime {
	return &mnnRuntime{
		config:    config,
		state:     "unloaded",
		port:      0,
		processID: runtimehost.ProcessID("mnn-" + config.LocalModelID + "-" + configFingerprint(config)),
		modelPath: config.ModelResourceURI,
	}
}

func configFingerprint(config MNNProviderConfig) string {
	h := sha256.New()
	h.Write([]byte(config.ModelResourceURI))
	h.Write([]byte(config.Backend))
	h.Write([]byte(strconv.Itoa(config.ContextSize)))
	h.Write([]byte(strconv.Itoa(config.ThreadNum)))
	h.Write([]byte(config.Precision))
	h.Write([]byte(config.Memory))
	return hex.EncodeToString(h.Sum(nil))[:8]
}

func (r *mnnRuntime) attachHost(host runtimehost.RuntimeHost) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.host = host
	if host != nil {
		r.supervisor = host.Processes()
	}
}

func (r *mnnRuntime) validateModelArtifact() error {
	if r.config.ModelResourceURI == "" {
		return fmt.Errorf("MNN model resource URI is empty")
	}

	info, err := os.Stat(r.config.ModelResourceURI)
	if err != nil {
		if os.IsNotExist(err) {
			return localmodel.ErrModelPackageNotFound
		}
		return fmt.Errorf("stat MNN model file: %w", err)
	}

	if info.Size() < 1024 {
		return localmodel.ErrModelPackageInvalid
	}

	ext := filepath.Ext(r.config.ModelResourceURI)
	if ext != ".mnn" {
		return fmt.Errorf("invalid MNN model format: %s", ext)
	}

	return nil
}

func (r *mnnRuntime) startServer(ctx context.Context) error {
	if r.supervisor == nil {
		return localmodel.ErrNativeBridgeUnavailable
	}

	executable := r.findMNNServerExecutable()
	if executable == "" {
		return fmt.Errorf("%w: MNN server not found", localmodel.ErrNativeBridgeUnavailable)
	}

	port := findMNNAvailablePort()
	if port == 0 {
		return fmt.Errorf("no available port for MNN server")
	}
	r.port = port
	r.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	args := r.buildServerArgs(port)

	workDir := filepath.Dir(executable)

	spec := runtimehost.ProcessSpec{
		ID:          r.processID,
		Executable:  executable,
		Args:        args,
		WorkingDir:  workDir,
		Environment: runtimehost.EnvironmentSpec{Policy: runtimehost.EnvPolicyMinimal},
		HealthProbe: &runtimehost.HTTPHealthProbe{
			URL:     r.baseURL + "/health",
			Timeout: 2 * time.Second,
		},
		StartupTimeout:  60 * time.Second,
		StopGracePeriod: 10 * time.Second,
		RestartPolicy:   runtimehost.RestartPolicy{Mode: runtimehost.RestartNever},
	}

	if err := r.supervisor.Register(spec); err != nil {
		releaseMNNPort(port)
		r.port = 0
		return fmt.Errorf("register MNN server: %w", err)
	}

	if err := r.supervisor.Start(ctx, r.processID); err != nil {
		releaseMNNPort(port)
		r.port = 0
		r.supervisor.Unregister(r.processID)
		return fmt.Errorf("start MNN server: %w", err)
	}

	if err := r.supervisor.WaitReady(ctx, r.processID); err != nil {
		r.stopServer()
		return fmt.Errorf("MNN server not ready: %w", err)
	}

	return nil
}

func (r *mnnRuntime) stopServer() {
	if r.supervisor == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = r.supervisor.Stop(ctx, r.processID)
	_ = r.supervisor.Unregister(r.processID)
	if r.port > 0 {
		releaseMNNPort(r.port)
		r.port = 0
	}
}

func (r *mnnRuntime) findMNNServerExecutable() string {
	candidates := []string{
		filepath.Join("resources", "mnn", "mnn-server.exe"),
		filepath.Join("resources", "mnn", "mnn-server"),
		"mnn-server",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		absPath, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return absPath
		}
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		localPath := filepath.Join(execDir, "resources", "mnn", "mnn-server.exe")
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			return localPath
		}
		localPath = filepath.Join(execDir, "resources", "mnn", "mnn-server")
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			return localPath
		}
	}
	return ""
}

func (r *mnnRuntime) buildServerArgs(port int) []string {
	args := []string{
		"--model", r.config.ModelResourceURI,
		"--port", strconv.Itoa(port),
		"--host", "127.0.0.1",
		"--threads", strconv.Itoa(r.config.ThreadNum),
	}
	if r.config.Precision != "" {
		args = append(args, "--precision", r.config.Precision)
	}
	if r.config.Memory != "" {
		args = append(args, "--memory", r.config.Memory)
	}
	if r.config.UseMMap {
		args = append(args, "--mmap")
	}
	if r.config.KVCacheMMap {
		args = append(args, "--kv-cache-mmap")
	}
	if r.config.AttentionMode > 0 {
		args = append(args, "--attention-mode", strconv.Itoa(r.config.AttentionMode))
	}
	if r.config.ReuseKV {
		args = append(args, "--reuse-kv")
	}
	return args
}

func (r *mnnRuntime) chatRequest(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
	if sink == nil {
		return r.chatNonStream(ctx, request)
	}
	return r.chatStream(ctx, request, sink)
}

func (r *mnnRuntime) chatNonStream(ctx context.Context, request localmodel.LocalModelRequest) (localmodel.LocalModelResult, error) {
	reqBody := r.buildChatRequestBody(request, false)
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
	if err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return localmodel.LocalModelResult{}, fmt.Errorf("MNN server returned %d: %s", resp.StatusCode, truncateBytes(body, 512))
	}

	return parseMNNChatResponse(body)
}

func (r *mnnRuntime) chatStream(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
	reqBody := r.buildChatRequestBody(request, true)
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return parseMNNChatStream(resp.Body, sink)
}

func (r *mnnRuntime) buildChatRequestBody(request localmodel.LocalModelRequest, stream bool) map[string]interface{} {
	reqMsgs := make([]map[string]interface{}, 0, len(request.Messages))
	for _, msg := range request.Messages {
		content := mnnBuildMessageContent(msg)
		reqMsgs = append(reqMsgs, map[string]interface{}{
			"role":    msg.Role,
			"content": content,
		})
	}

	reqBody := map[string]interface{}{
		"messages": reqMsgs,
		"stream":   stream,
	}

	if request.MaxNewTokens > 0 {
		reqBody["max_tokens"] = request.MaxNewTokens
	}

	if request.Temperature > 0 {
		reqBody["temperature"] = request.Temperature
	} else if r.config.Sampler.Temperature > 0 {
		reqBody["temperature"] = r.config.Sampler.Temperature
	}

	if request.TopP > 0 && request.TopP < 1 {
		reqBody["top_p"] = request.TopP
	} else if r.config.Sampler.TopP > 0 && r.config.Sampler.TopP < 1 {
		reqBody["top_p"] = r.config.Sampler.TopP
	}

	if r.config.Sampler.TopK > 0 {
		reqBody["top_k"] = r.config.Sampler.TopK
	}
	if r.config.Sampler.MinP > 0 {
		reqBody["min_p"] = r.config.Sampler.MinP
	}

	if request.JSONOnly {
		reqBody["response_format"] = map[string]interface{}{"type": "json_object"}
	}

	if len(request.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(request.Tools))
		for _, t := range request.Tools {
			tools = append(tools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			})
		}
		reqBody["tools"] = tools
	}

	return reqBody
}

func mnnBuildMessageContent(msg localmodel.LocalModelMessage) interface{} {
	if len(msg.Parts) == 1 && msg.Parts[0].Type == "text" {
		return msg.Parts[0].Text
	}
	parts := make([]map[string]interface{}, 0, len(msg.Parts))
	for _, p := range msg.Parts {
		switch p.Type {
		case "text":
			parts = append(parts, map[string]interface{}{
				"type": "text",
				"text": p.Text,
			})
		case "image":
			parts = append(parts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": p.ResourceURI,
				},
			})
		}
	}
	return parts
}

func parseMNNChatResponse(body []byte) (localmodel.LocalModelResult, error) {
	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return localmodel.LocalModelResult{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.Error.Message != "" {
		return localmodel.LocalModelResult{}, fmt.Errorf("MNN server error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return localmodel.LocalModelResult{}, fmt.Errorf("no choices in response")
	}

	choice := result.Choices[0]
	res := localmodel.LocalModelResult{
		Text:         choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: localmodel.LocalModelUsage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		res.ToolCalls = append(res.ToolCalls, localmodel.LocalModelToolCall{
			Name:      tc.Function.Name,
			Arguments: string(tc.Function.Arguments),
		})
	}

	return res, nil
}

func parseMNNChatStream(body io.Reader, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
	result := localmodel.LocalModelResult{
		ToolCalls: []localmodel.LocalModelToolCall{},
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimSpace(line[6:])
		if bytes.Equal(data, []byte("[DONE]")) {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Function struct {
							Name      string          `json:"name"`
							Arguments json.RawMessage `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(data, &chunk); err != nil {
			continue
		}

		if chunk.Error.Message != "" {
			return result, fmt.Errorf("stream error: %s", chunk.Error.Message)
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				result.Text += choice.Delta.Content
				if err := sink.OnTextDelta(choice.Delta.Content); err != nil {
					return result, err
				}
			}
			for _, tc := range choice.Delta.ToolCalls {
				if tc.Function.Name != "" || len(tc.Function.Arguments) > 0 {
					toolCall := localmodel.LocalModelToolCall{
						Name:      tc.Function.Name,
						Arguments: string(tc.Function.Arguments),
					}
					result.ToolCalls = append(result.ToolCalls, toolCall)
					if err := sink.OnToolCallDelta(tc.Function.Name, tc.Function.Name, string(tc.Function.Arguments)); err != nil {
						return result, err
					}
				}
			}
			if choice.FinishReason != nil {
				result.FinishReason = *choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("stream scanner error: %w", err)
	}

	return result, nil
}

func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}
