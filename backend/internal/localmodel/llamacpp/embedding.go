// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package llamacpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/localmodel/gguf"
	"github.com/u-ai/backend/internal/runtimehost"
)

type llamaCppEmbeddingBackend struct {
	config     LlamaCppEmbeddingConfig
	mu         sync.Mutex
	state      string
	loadedAt   string
	lastError  string
	manifest   *gguf.GGUFModelManifest
	runtime    *llamaEmbeddingRuntime
	host       runtimehost.RuntimeHost
}

type llamaEmbeddingRuntime struct {
	config     LlamaCppEmbeddingConfig
	mu         sync.Mutex
	baseURL    string
	port       int
	host       runtimehost.RuntimeHost
	supervisor runtimehost.ProcessSupervisor
	processID  runtimehost.ProcessID
}

func newLlamaEmbeddingRuntime(config LlamaCppEmbeddingConfig) *llamaEmbeddingRuntime {
	return &llamaEmbeddingRuntime{
		config:    config,
		port:      0,
		processID: runtimehost.ProcessID("llama-embedding-" + config.LocalModelID),
	}
}

func (r *llamaEmbeddingRuntime) attachHost(host runtimehost.RuntimeHost) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.host = host
	if host != nil {
		r.supervisor = host.Processes()
	}
}

func (r *llamaEmbeddingRuntime) validateModelArtifact() error {
	if r.config.ResourceURI == "" {
		return fmt.Errorf("embedding model resource URI is empty")
	}

	info, err := os.Stat(r.config.ResourceURI)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("embedding model not found: %w", err)
		}
		return fmt.Errorf("stat embedding model: %w", err)
	}

	if info.Size() < 1024 {
		return fmt.Errorf("embedding model too small: %d bytes", info.Size())
	}

	if err := gguf.ValidateGGUFResource(r.config.ResourceURI); err != nil {
		return fmt.Errorf("invalid GGUF: %w", err)
	}
	return nil
}

func (r *llamaEmbeddingRuntime) inspectModel() (*gguf.GGUFModelManifest, error) {
	inspector := gguf.NewInspector()
	manifest, err := inspector.Inspect(r.config.ResourceURI)
	if err != nil {
		return nil, fmt.Errorf("inspect GGUF: %w", err)
	}
	manifest.LocalModelID = r.config.LocalModelID
	return manifest, nil
}

func (r *llamaEmbeddingRuntime) startServer(ctx context.Context) error {
	if r.supervisor == nil {
		return fmt.Errorf("no runtime host available")
	}

	executable := r.findEmbeddingServerExecutable()
	if executable == "" {
		return fmt.Errorf("llama-server not found")
	}

	port := findAvailablePort()
	if port == 0 {
		return fmt.Errorf("no available port for embedding server")
	}
	r.port = port
	r.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	args := []string{
		"--model", r.config.ResourceURI,
		"--port", strconv.Itoa(port),
		"--host", "127.0.0.1",
		"--ctx-size", strconv.Itoa(r.config.ContextSize),
		"--threads", strconv.Itoa(r.config.Threads),
		"--embedding",
	}
	if r.config.GPULayers > 0 {
		args = append(args, "--n-gpu-layers", strconv.Itoa(r.config.GPULayers))
	}
	if r.config.BatchSize > 0 {
		args = append(args, "--batch-size", strconv.Itoa(r.config.BatchSize))
	}
	if r.config.MMap {
		args = append(args, "--mmap")
	}

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
		releasePort(port)
		r.port = 0
		return fmt.Errorf("register embedding server: %w", err)
	}

	if err := r.supervisor.Start(ctx, r.processID); err != nil {
		releasePort(port)
		r.port = 0
		r.supervisor.Unregister(r.processID)
		return fmt.Errorf("start embedding server: %w", err)
	}

	if err := r.supervisor.WaitReady(ctx, r.processID); err != nil {
		r.stopServer()
		return fmt.Errorf("embedding server not ready: %w", err)
	}
	return nil
}

func (r *llamaEmbeddingRuntime) stopServer() {
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

func (r *llamaEmbeddingRuntime) findEmbeddingServerExecutable() string {
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

func NewLlamaCppEmbeddingBackend(config LlamaCppEmbeddingConfig) (*llamaCppEmbeddingBackend, error) {
	b := &llamaCppEmbeddingBackend{
		config:   config,
		state:    "unloaded",
		runtime:  newLlamaEmbeddingRuntime(config),
	}
	return b, nil
}

func (b *llamaCppEmbeddingBackend) AttachHost(host runtimehost.RuntimeHost) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.host = host
	b.runtime.attachHost(host)
}

type EmbeddingResult struct {
	Vectors           [][]float32
	Dimension         int
	Normalized        bool
	Pooling           string
	ModelFingerprint  string
	Truncated         []bool
	TokenCounts       []int
}

type EmbeddingCapabilities struct {
	SupportsEmbedding  bool
	EmbeddingDimension int
	PoolingType        string
	SupportsTruncate   bool
}

func (b *llamaCppEmbeddingBackend) Capabilities(ctx context.Context) (EmbeddingCapabilities, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	caps := EmbeddingCapabilities{
		SupportsEmbedding:  true,
		EmbeddingDimension: 0,
		PoolingType:        b.config.Pooling,
		SupportsTruncate:   b.config.Truncate,
	}

	if b.manifest != nil {
		caps.EmbeddingDimension = b.manifest.EmbeddingLength
		caps.SupportsEmbedding = b.manifest.SupportsEmbedding
		if b.manifest.PoolingType != "" {
			caps.PoolingType = b.manifest.PoolingType
		}
	}

	return caps, nil
}

func (b *llamaCppEmbeddingBackend) Embed(ctx context.Context, inputs []string, purpose string) (*EmbeddingResult, error) {
	b.mu.Lock()
	if b.state != "ready" {
		b.mu.Unlock()
		return nil, fmt.Errorf("embedding model not loaded")
	}
	b.state = "embedding"
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.state = "ready"
		b.mu.Unlock()
	}()

	if len(inputs) == 0 {
		return &EmbeddingResult{
			Vectors:     [][]float32{},
			Dimension:   b.getEmbeddingDimension(),
			Normalized:  b.config.Normalize != "none",
			Pooling:     b.config.Pooling,
			Truncated:   []bool{},
			TokenCounts: []int{},
		}, nil
	}

	if b.runtime != nil && b.runtime.baseURL != "" {
		vectors, err := b.runtime.embeddingRequest(ctx, inputs)
		if err != nil {
			return nil, fmt.Errorf("embedding inference failed: %w", err)
		}

		dim := b.getEmbeddingDimension()
		if len(vectors) > 0 && len(vectors[0]) > 0 {
			dim = len(vectors[0])
		}

		if b.config.Normalize == "l2" {
			for i := range vectors {
				normalizeVectorL2(vectors[i])
			}
		}

		truncated := make([]bool, len(inputs))
		tokenCounts := make([]int, len(inputs))
		for i, text := range inputs {
			tokenCounts[i] = len([]rune(text))
			if b.config.Truncate && b.manifest != nil && b.manifest.ContextLength > 0 {
				if tokenCounts[i] > b.manifest.ContextLength {
					truncated[i] = true
				}
			}
		}

		return &EmbeddingResult{
			Vectors:          vectors,
			Dimension:        dim,
			Normalized:       b.config.Normalize == "l2",
			Pooling:          b.config.Pooling,
			ModelFingerprint: b.getFingerprint(),
			Truncated:        truncated,
			TokenCounts:      tokenCounts,
		}, nil
	}

	return nil, fmt.Errorf("no local embedding backend available")
}

func (b *llamaCppEmbeddingBackend) Load(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == "ready" || b.state == "loading" || b.state == "embedding" {
		return nil
	}

	b.state = "loading"
	b.lastError = ""

	if err := b.runtime.validateModelArtifact(); err != nil {
		b.lastError = err.Error()
		b.state = "failed"
		return fmt.Errorf("load embedding model failed: %w", err)
	}

	manifest, err := b.runtime.inspectModel()
	if err != nil {
		b.lastError = err.Error()
		b.state = "failed"
		return fmt.Errorf("inspect embedding model failed: %w", err)
	}

	if manifest.EmbeddingLength <= 0 {
		b.lastError = "model does not support embeddings"
		b.state = "failed"
		return fmt.Errorf("model does not support embeddings")
	}

	b.manifest = manifest

	if b.host == nil {
		b.lastError = "native bridge unavailable"
		b.state = "failed"
		return fmt.Errorf("load embedding model failed: runtime host unavailable")
	}

	if err := b.runtime.startServer(ctx); err != nil {
		b.lastError = err.Error()
		b.state = "failed"
		return fmt.Errorf("start embedding server failed: %w", err)
	}

	b.state = "ready"
	b.loadedAt = time.Now().Format("2006-01-02 15:04:05")
	return nil
}

func (b *llamaCppEmbeddingBackend) Unload(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.runtime.stopServer()
	b.manifest = nil
	b.state = "unloaded"
	b.loadedAt = ""
	return nil
}

func (b *llamaCppEmbeddingBackend) Health(ctx context.Context) (string, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state, b.lastError
}

func (b *llamaCppEmbeddingBackend) getEmbeddingDimension() int {
	if b.manifest != nil && b.manifest.EmbeddingLength > 0 {
		return b.manifest.EmbeddingLength
	}
	return 0
}

func (b *llamaCppEmbeddingBackend) getFingerprint() string {
	if b.manifest == nil {
		return ""
	}
	return gguf.ComputeEmbeddingFingerprint(b.manifest)
}

func (r *llamaEmbeddingRuntime) embeddingRequest(ctx context.Context, inputs []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"input": inputs,
	}
	if r.config.LocalModelID != "" {
		reqBody["model"] = r.config.LocalModelID
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

func normalizeVectorL2(vector []float32) {
	var sum float64
	for _, v := range vector {
		sum += float64(v) * float64(v)
	}
	norm := math.Sqrt(sum)
	if norm < 1e-9 {
		return
	}
	for i := range vector {
		vector[i] = float32(float64(vector[i]) / norm)
	}
}
