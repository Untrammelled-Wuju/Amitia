// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package llamacpp

import (
	"bufio"
	"bytes"
	"context"
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
	"github.com/u-ai/backend/internal/localmodel/gguf"
	"github.com/u-ai/backend/internal/runtimehost"
)

type llamaRuntime struct {
	config     LlamaCppProviderConfig
	mu         sync.Mutex
	state      string
	loadedAt   string
	lastError  string
	manifest   *gguf.GGUFModelManifest
	host       runtimehost.RuntimeHost
	processID  runtimehost.ProcessID
	baseURL    string
	port       int
	supervisor runtimehost.ProcessSupervisor
}

var (
	defaultLlamaPort = 18888
	llamaPortMutex   sync.Mutex
	allocatedPorts   = make(map[int]bool)
)

func findAvailablePort() int {
	llamaPortMutex.Lock()
	defer llamaPortMutex.Unlock()
	for p := defaultLlamaPort; p < 65535; p++ {
		if allocatedPorts[p] {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			continue
		}
		ln.Close()
		allocatedPorts[p] = true
		return p
	}
	return 0
}

func releasePort(p int) {
	llamaPortMutex.Lock()
	defer llamaPortMutex.Unlock()
	delete(allocatedPorts, p)
}

func newLlamaRuntime(config LlamaCppProviderConfig) *llamaRuntime {
	return &llamaRuntime{
		config:   config,
		state:    "unloaded",
		port:     0,
		processID: runtimehost.ProcessID("llama-cpp-" + config.LocalModelID),
	}
}

func (r *llamaRuntime) attachHost(host runtimehost.RuntimeHost) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.host = host
	if host != nil {
		r.supervisor = host.Processes()
	}
}

func (r *llamaRuntime) validateModelArtifact() error {
	if r.config.ResourceURI == "" {
		return fmt.Errorf("model resource URI is empty")
	}

	info, err := os.Stat(r.config.ResourceURI)
	if err != nil {
		if os.IsNotExist(err) {
			return localmodel.ErrModelPackageNotFound
		}
		return fmt.Errorf("stat model file: %w", err)
	}

	if info.Size() < 1024 {
		return localmodel.ErrModelPackageInvalid
	}

	f, err := os.Open(r.config.ResourceURI)
	if err != nil {
		return fmt.Errorf("open model file: %w", err)
	}
	defer f.Close()

	if err := gguf.ValidateGGUFResource(r.config.ResourceURI); err != nil {
		return localmodel.ErrModelPackageInvalid
	}

	f.Close()
	return nil
}

func (r *llamaRuntime) inspectModel() (*gguf.GGUFModelManifest, error) {
	inspector := gguf.NewInspector()
	manifest, err := inspector.Inspect(r.config.ResourceURI)
	if err != nil {
		return nil, fmt.Errorf("inspect GGUF: %w", err)
	}
	manifest.LocalModelID = r.config.LocalModelID
	return manifest, nil
}

func (r *llamaRuntime) startServer(ctx context.Context) error {
	if r.supervisor == nil {
		return localmodel.ErrNativeBridgeUnavailable
	}

	executable := r.findLlamaServerExecutable()
	if executable == "" {
		return fmt.Errorf("%w: llama-server not found", localmodel.ErrNativeBridgeUnavailable)
	}

	port := findAvailablePort()
	if port == 0 {
		return fmt.Errorf("no available port for llama-server")
	}
	r.port = port
	r.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	args := r.buildServerArgs(port)

	workDir := filepath.Dir(executable)

	spec := runtimehost.ProcessSpec{
		ID: r.processID,
		Executable: executable,
		Args: args,
		WorkingDir: workDir,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyMinimal,
		},
		HealthProbe: &runtimehost.HTTPHealthProbe{
			URL:     r.baseURL + "/health",
			Timeout: 2 * time.Second,
		},
		StartupTimeout: 60 * time.Second,
		StopGracePeriod: 10 * time.Second,
		RestartPolicy: runtimehost.RestartPolicy{
			Mode: runtimehost.RestartNever,
		},
	}

	if err := r.supervisor.Register(spec); err != nil {
		releasePort(port)
		r.port = 0
		return fmt.Errorf("register llama-server: %w", err)
	}

	if err := r.supervisor.Start(ctx, r.processID); err != nil {
		releasePort(port)
		r.port = 0
		r.supervisor.Unregister(r.processID)
		return fmt.Errorf("start llama-server: %w", err)
	}

	if err := r.supervisor.WaitReady(ctx, r.processID); err != nil {
		r.stopServer()
		return fmt.Errorf("llama-server not ready: %w", err)
	}

	return nil
}

func (r *llamaRuntime) stopServer() {
	if r.supervisor == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = r.supervisor.Stop(ctx, r.processID)
	_ = r.supervisor.Unregister(r.processID)
	if r.port > 0 {
		releasePort(r.port)
		r.port = 0
	}
}

func (r *llamaRuntime) findLlamaServerExecutable() string {
	candidates := []string{
		r.config.Backend,
		filepath.Join("resources", "llama", "llama-server.exe"),
		filepath.Join("resources", "llama", "llama-server"),
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
		localPath := filepath.Join(execDir, "resources", "llama", "llama-server.exe")
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			return localPath
		}
		localPath = filepath.Join(execDir, "resources", "llama", "llama-server")
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			return localPath
		}
	}
	return ""
}

func (r *llamaRuntime) buildServerArgs(port int) []string {
	args := []string{
		"--model", r.config.ResourceURI,
		"--port", strconv.Itoa(port),
		"--host", "127.0.0.1",
		"--ctx-size", strconv.Itoa(r.config.ContextSize),
		"--threads", strconv.Itoa(r.config.Threads),
		"--temp", fmt.Sprintf("%.2f", r.config.Temperature),
	}
	if r.config.TopP > 0 && r.config.TopP < 1 {
		args = append(args, "--top-p", fmt.Sprintf("%.2f", r.config.TopP))
	}
	if r.config.TopK > 0 {
		args = append(args, "--top-k", strconv.Itoa(r.config.TopK))
	}
	if r.config.MinP > 0 {
		args = append(args, "--min-p", fmt.Sprintf("%.2f", r.config.MinP))
	}
	if r.config.GPULayers > 0 {
		args = append(args, "--n-gpu-layers", strconv.Itoa(r.config.GPULayers))
	}
	if r.config.BatchSize > 0 {
		args = append(args, "--batch-size", strconv.Itoa(r.config.BatchSize))
	}
	if r.config.UBatchSize > 0 {
		args = append(args, "--ubatch-size", strconv.Itoa(r.config.UBatchSize))
	}
	if r.config.FlashAttention {
		args = append(args, "--flash-attn")
	}
	if r.config.MMap {
		args = append(args, "--mmap")
	}
	if r.config.MLock {
		args = append(args, "--mlock")
	}
	if r.config.MMProjURI != "" {
		args = append(args, "--mmproj", r.config.MMProjURI)
	}
	if r.config.KVCacheTypeK != "" {
		args = append(args, "--cache-type-k", r.config.KVCacheTypeK)
	}
	if r.config.KVCacheTypeV != "" {
		args = append(args, "--cache-type-v", r.config.KVCacheTypeV)
	}
	if r.config.Seed != nil {
		args = append(args, "--seed", strconv.FormatInt(*r.config.Seed, 10))
	}
	args = append(args, "--embedding")
	return args
}

func (r *llamaRuntime) chatRequest(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
	if sink == nil {
		return r.chatNonStream(ctx, request)
	}
	return r.chatStream(ctx, request, sink)
}

func (r *llamaRuntime) chatNonStream(ctx context.Context, request localmodel.LocalModelRequest) (localmodel.LocalModelResult, error) {
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
		return localmodel.LocalModelResult{}, fmt.Errorf("llama-server returned %d: %s", resp.StatusCode, truncateBytes(body, 512))
	}

	return parseChatCompletionResponse(body)
}

func (r *llamaRuntime) chatStream(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
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

	return parseChatCompletionStream(resp.Body, sink)
}

func (r *llamaRuntime) buildChatRequestBody(request localmodel.LocalModelRequest, stream bool) map[string]interface{} {
	reqMsgs := make([]map[string]interface{}, 0, len(request.Messages))
	for _, msg := range request.Messages {
		content := buildMessageContent(msg)
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
	} else if r.config.Temperature > 0 {
		reqBody["temperature"] = r.config.Temperature
	}

	if request.TopP > 0 && request.TopP < 1 {
		reqBody["top_p"] = request.TopP
	} else if r.config.TopP > 0 && r.config.TopP < 1 {
		reqBody["top_p"] = r.config.TopP
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

func buildMessageContent(msg localmodel.LocalModelMessage) interface{} {
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

func (r *llamaRuntime) embeddingRequest(ctx context.Context, inputs []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"input": inputs,
	}
	if r.manifest != nil && r.manifest.LocalModelID != "" {
		reqBody["model"] = r.manifest.LocalModelID
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.baseURL+"/v1/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding returned %d: %s", resp.StatusCode, truncateBytes(body, 512))
	}

	return parseEmbeddingResponse(body)
}

func (r *llamaRuntime) Load(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == "ready" || r.state == "loading" {
		return nil
	}

	r.state = "loading"
	r.lastError = ""

	if err := r.validateModelArtifact(); err != nil {
		r.lastError = err.Error()
		r.state = "failed"
		return fmt.Errorf("%w: %v", localmodel.ErrLoadFailed, err)
	}

	manifest, err := r.inspectModel()
	if err != nil {
		r.lastError = err.Error()
		r.state = "failed"
		return fmt.Errorf("%w: %v", localmodel.ErrLoadFailed, err)
	}
	r.manifest = manifest

	if r.supervisor != nil {
		if err := r.startServer(ctx); err != nil {
			r.lastError = err.Error()
			r.state = "failed"
			return fmt.Errorf("%w: %v", localmodel.ErrLoadFailed, err)
		}
	}

	r.loadedAt = time.Now().Format("2006-01-02 15:04:05")
	r.state = "ready"
	return nil
}

func (r *llamaRuntime) Unload(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.unlock()
}

func (r *llamaRuntime) unlock() error {
	r.stopServer()
	r.state = "unloaded"
	r.loadedAt = ""
	r.manifest = nil
	return nil
}

func (r *llamaRuntime) Health() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state, r.lastError
}

func parseChatCompletionResponse(body []byte) (localmodel.LocalModelResult, error) {
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
		return localmodel.LocalModelResult{}, fmt.Errorf("llama-server error: %s", result.Error.Message)
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

func parseChatCompletionStream(body io.Reader, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
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

func parseEmbeddingResponse(body []byte) ([][]float32, error) {
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("embedding error: %s", result.Error.Message)
	}

	vectors := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(vectors) {
			vectors[d.Index] = d.Embedding
		}
	}
	return vectors, nil
}

func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}
