package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	amitialog "log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type HTTPProviderOptions struct {
	BaseURL           string
	Definition        Definition
	HealthPath        string
	SendPath          string
	ImagePath         string
	VoicePath         string
	VideoPath         string
	FilePath          string
	StatusPath        string
	ConnectPath       string
	DisconnectPath    string
	ConfigPath        string
	MessagesPath      string
	ActionPath        string
	LogsPath          string
	PeersPath         string
	DefaultHeaders    map[string]string
	UseFallbackImage  bool
	IdempotencyHeader bool
}

type HTTPProvider struct {
	definition        Definition
	baseURL           string
	healthPath        string
	sendPath          string
	imagePath         string
	voicePath         string
	videoPath         string
	filePath          string
	statusPath        string
	connectPath       string
	disconnectPath    string
	configPath        string
	messagesPath      string
	actionPath        string
	logsPath          string
	peersPath         string
	defaultHeaders    map[string]string
	useFallbackImage  bool
	idempotencyHeader bool
	client            *http.Client
	running           atomic.Bool
}

func NewHTTPProvider(options HTTPProviderOptions) *HTTPProvider {
	return &HTTPProvider{
		definition:        options.Definition,
		baseURL:           strings.TrimRight(options.BaseURL, "/"),
		healthPath:        defaultPath(options.HealthPath, "/api/health"),
		sendPath:          defaultPath(options.SendPath, "/api/send"),
		imagePath:         defaultPath(options.ImagePath, "/api/send-image"),
		voicePath:         defaultPath(options.VoicePath, "/api/send-voice"),
		videoPath:         defaultPath(options.VideoPath, "/api/send-video"),
		filePath:          defaultPath(options.FilePath, "/api/send-file"),
		statusPath:        defaultPath(options.StatusPath, "/api/status"),
		connectPath:       defaultPath(options.ConnectPath, "/api/connect"),
		disconnectPath:    defaultPath(options.DisconnectPath, "/api/disconnect"),
		configPath:        defaultPath(options.ConfigPath, "/api/config"),
		messagesPath:      defaultPath(options.MessagesPath, "/api/messages"),
		actionPath:        defaultPath(options.ActionPath, "/api/action"),
		logsPath:          defaultPath(options.LogsPath, "/api/logs"),
		peersPath:         defaultPath(options.PeersPath, "/api/peers"),
		defaultHeaders:    cloneHeaders(options.DefaultHeaders),
		useFallbackImage:  options.UseFallbackImage,
		idempotencyHeader: options.IdempotencyHeader,
		client:            &http.Client{Timeout: 30 * time.Second},
	}
}

func defaultPath(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func (p *HTTPProvider) Definition() Definition { return p.definition }

func (p *HTTPProvider) Start(context.Context) error {
	if p.baseURL == "" {
		return fmt.Errorf("channel %s: base url is empty", p.definition.ID)
	}
	p.running.Store(true)
	return nil
}

func (p *HTTPProvider) Stop(context.Context) error {
	p.running.Store(false)
	return nil
}

func (p *HTTPProvider) Connect(ctx context.Context, config map[string]any) error {
	_, err := p.ConnectResult(ctx, config)
	return err
}

func (p *HTTPProvider) ConnectResult(ctx context.Context, config map[string]any) (map[string]any, error) {
	var result map[string]any
	if err := p.post(ctx, p.connectPath, config, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func (p *HTTPProvider) Disconnect(ctx context.Context) error {
	return p.post(ctx, p.disconnectPath, map[string]any{}, nil)
}

func (p *HTTPProvider) Config(ctx context.Context) (map[string]any, error) {
	var result map[string]any
	if err := p.get(ctx, p.configPath, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *HTTPProvider) StatusData(ctx context.Context) (map[string]any, error) {
	var response map[string]any
	if err := p.get(ctx, p.statusPath, &response); err != nil {
		return nil, err
	}
	if data, ok := response["data"].(map[string]any); ok {
		return data, nil
	}
	return response, nil
}

func (p *HTTPProvider) Messages(ctx context.Context, request map[string]any) (map[string]any, error) {
	var response map[string]any
	if err := p.post(ctx, p.messagesPath, request, &response); err != nil {
		return nil, err
	}
	if data, ok := response["data"].(map[string]any); ok {
		return data, nil
	}
	return response, nil
}

func (p *HTTPProvider) Invoke(ctx context.Context, action string, input map[string]any) (map[string]any, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return nil, fmt.Errorf("channel %s: action is required", p.definition.ID)
	}
	if input == nil {
		input = map[string]any{}
	}
	var response map[string]any
	if err := p.post(ctx, p.actionPath, map[string]any{"action": action, "input": input}, &response); err != nil {
		return nil, err
	}
	if data, ok := response["data"].(map[string]any); ok {
		return data, nil
	}
	return response, nil
}

func (p *HTTPProvider) Logs(ctx context.Context, request map[string]any) (map[string]any, error) {
	var response map[string]any
	if err := p.post(ctx, p.logsPath, request, &response); err != nil {
		return nil, err
	}
	if data, ok := response["data"].(map[string]any); ok {
		return data, nil
	}
	return response, nil
}

func (p *HTTPProvider) Peers(ctx context.Context, request map[string]any) (map[string]any, error) {
	var response map[string]any
	if err := p.post(ctx, p.peersPath, request, &response); err != nil {
		return nil, err
	}
	if data, ok := response["data"].(map[string]any); ok {
		return data, nil
	}
	return response, nil
}

func (p *HTTPProvider) Status(ctx context.Context, accountID string) (AccountStatus, error) {
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := p.get(ctx, p.statusPath, &response); err != nil {
		return AccountStatus{}, err
	}
	data := response.Data
	status := AccountStatus{Channel: p.definition.ID, AccountID: stringValue(data["accountId"]), Status: stringValue(data["status"]), Running: boolValue(data["running"]), LastError: stringValue(data["lastError"])}
	if status.AccountID == "" {
		status.AccountID = accountID
	}
	status.Connected = status.Status == "connected" || boolValue(data["connected"])
	if status.Status == "online" {
		status.Connected = true
	} else if status.Status == "" {
		if status.Connected {
			status.Status = "connected"
		} else {
			status.Status = "disconnected"
		}
	}
	return status, nil
}

func (p *HTTPProvider) Send(ctx context.Context, request SendRequest) (SendResult, error) {
	body := map[string]any{"toUserId": request.PeerID, "text": request.Text, "contextToken": request.ContextToken, "deliveryKey": request.IdempotencyKey}
	if request.ConversationID != "" {
		body["conversationId"] = request.ConversationID
	}
	payload := map[string]any{}
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &payload); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	for source, target := range map[string]string{
		"content":          "content",
		"originalPath":     "assetUrl",
		"fallbackPath":     "fallbackUrl",
		"mimeType":         "mimeType",
		"altText":          "altText",
		"messageId":        "messageId",
		"replyToMessageId": "replyToMessageId",
	} {
		if value, ok := payload[source]; ok && value != nil {
			body[target] = value
		}
	}
	if strings.TrimSpace(request.Text) == "" {
		if content, ok := payload["content"].(string); ok {
			body["text"] = content
		}
	}
	path := p.sendPath
	switch request.ContentType {
	case "", "text":
	case "image":
		asset := payload["originalPath"]
		if p.useFallbackImage {
			asset = payload["fallbackPath"]
		}
		body["assetUrl"] = asset
		if asset == nil || strings.TrimSpace(fmt.Sprint(asset)) == "" {
			return SendResult{}, fmt.Errorf("channel %s: image asset is required", p.definition.ID)
		}
		path = p.imagePath
	case "voice":
		path = p.voicePath
	case "video":
		if strings.TrimSpace(fmt.Sprint(body["assetUrl"])) == "" {
			return SendResult{}, fmt.Errorf("channel %s: video asset is required", p.definition.ID)
		}
		path = p.videoPath
	case "file":
		if strings.TrimSpace(fmt.Sprint(body["assetUrl"])) == "" {
			return SendResult{}, fmt.Errorf("channel %s: file asset is required", p.definition.ID)
		}
		path = p.filePath
	default:
		return SendResult{}, fmt.Errorf("channel %s: unsupported content type %s", p.definition.ID, request.ContentType)
	}
	if err := p.postWithHeaders(ctx, path, body, nil, p.idempotencyHeaders(request)); err != nil {
		return SendResult{}, err
	}
	return SendResult{Accepted: true, DeliveredAt: time.Now().UTC()}, nil
}

func (p *HTTPProvider) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return err
	}
	p.applyHeaders(req, nil)
	return p.do(req, target)
}

func (p *HTTPProvider) post(ctx context.Context, path string, body any, target any) error {
	return p.postWithHeaders(ctx, path, body, target, nil)
}

func (p *HTTPProvider) idempotencyHeaders(request SendRequest) map[string]string {
	if !p.idempotencyHeader || strings.TrimSpace(request.IdempotencyKey) == "" {
		return nil
	}
	return map[string]string{"Idempotency-Key": request.IdempotencyKey}
}

func (p *HTTPProvider) postWithHeaders(ctx context.Context, path string, body any, target any, headers map[string]string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	p.applyHeaders(req, headers)
	return p.do(req, target)
}

func (p *HTTPProvider) applyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range p.defaultHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func (p *HTTPProvider) do(req *http.Request, target any) error {
	response, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("channel %s returned HTTP %d: %s", p.definition.ID, response.StatusCode, strings.TrimSpace(string(data)))
	}
	if target == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func stringValue(value any) string { text, _ := value.(string); return text }
func boolValue(value any) bool     { result, _ := value.(bool); return result }
