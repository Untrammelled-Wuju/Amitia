package notificationruntime

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type XiaomiProvider struct {
	appSecret string
	packageID string
	client    *http.Client
}

func NewXiaomiProviderFromEnvironment() *XiaomiProvider {
	return &XiaomiProvider{
		appSecret: strings.TrimSpace(os.Getenv("AMITIA_XIAOMI_APP_SECRET")),
		packageID: androidPackageID(),
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *XiaomiProvider) Available() bool {
	return p != nil && p.appSecret != "" && p.packageID != ""
}

func (p *XiaomiProvider) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "mipush"}
	if !p.Available() || strings.TrimSpace(endpoint.MiPushToken) == "" {
		result.ErrorCode = "provider_unavailable"
		result.ErrorMessage = "Xiaomi Push is not configured or RegId is missing"
		return result
	}
	payload, _ := json.Marshal(vendorEnvelope(envelope))
	ttl := envelope.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	form := url.Values{}
	form.Set("registration_id", strings.TrimSpace(endpoint.MiPushToken))
	form.Set("restricted_package_name", p.packageID)
	form.Set("payload", string(payload))
	form.Set("time_to_live", strconv.FormatInt(ttl.Milliseconds(), 10))
	if endpoint.SupportsNativeData("mipush") {
		form.Set("pass_through", "1")
	} else {
		form.Set("pass_through", "0")
		form.Set("title", truncateRunes(envelope.Title, 48))
		form.Set("description", truncateRunes(envelope.Body, 120))
		form.Set("notify_id", strconv.Itoa(stableVendorNotifyID(envelope)))
		form.Set("extra.notify_foreground", "1")
		form.Set("extra.channel_id", androidChannelForEnvelope(envelope))
		if intentURI := androidIntentURI(envelope.DeepLink, p.packageID); intentURI != "" {
			form.Set("extra.notify_effect", "2")
			form.Set("extra.intent_uri", intentURI)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.xmpush.xiaomi.com/v3/message/regid",
		strings.NewReader(form.Encode()))
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Authorization", "key="+p.appSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		result.ErrorCode = "network_error"
		result.ErrorMessage = err.Error()
		return result
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = strings.TrimSpace(string(body))
		return result
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		result.ErrorCode = "invalid_response"
		result.ErrorMessage = err.Error()
		return result
	}
	code := anyInt(decoded["code"])
	if code != 0 || !strings.EqualFold(anyString(decoded["result"]), "ok") {
		result.ErrorCode = fmt.Sprintf("xiaomi_%d", code)
		result.ErrorMessage = strings.TrimSpace(anyString(decoded["description"]) + " " + anyString(decoded["reason"]))
		if strings.Contains(strings.ToLower(result.ErrorMessage), "regid") {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	if data, ok := decoded["data"].(map[string]any); ok {
		result.ProviderMessageID = anyString(data["id"])
	}
	return result
}

type HuaweiProvider struct {
	clientID     string
	clientSecret string
	projectID    string
	packageID    string
	client       *http.Client
	mu           sync.Mutex
	accessToken  string
	tokenExpiry  time.Time
}

func NewHuaweiProviderFromEnvironment() *HuaweiProvider {
	return &HuaweiProvider{
		clientID:     strings.TrimSpace(os.Getenv("AMITIA_HMS_CLIENT_ID")),
		clientSecret: strings.TrimSpace(os.Getenv("AMITIA_HMS_CLIENT_SECRET")),
		projectID:    strings.TrimSpace(os.Getenv("AMITIA_HMS_PROJECT_ID")),
		packageID:    androidPackageID(),
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *HuaweiProvider) Available() bool {
	return p != nil && p.clientID != "" && p.clientSecret != "" && p.projectID != ""
}

func (p *HuaweiProvider) accessTokenFor(ctx context.Context) (string, error) {
	if !p.Available() {
		return "", fmt.Errorf("HMS Push is not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.accessToken != "" && time.Until(p.tokenExpiry) > 2*time.Minute {
		return p.accessToken, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth-login.cloud.huawei.com/oauth2/v3/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HMS oauth failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	token := strings.TrimSpace(anyString(decoded["access_token"]))
	if token == "" {
		return "", fmt.Errorf("HMS oauth response did not contain access token")
	}
	expires := anyInt(decoded["expires_in"])
	if expires <= 0 {
		expires = 3600
	}
	p.accessToken = token
	p.tokenExpiry = time.Now().Add(time.Duration(expires) * time.Second)
	return token, nil
}

func (p *HuaweiProvider) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "hms"}
	token, err := p.accessTokenFor(ctx)
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	if strings.TrimSpace(endpoint.HMSToken) == "" {
		result.ErrorCode = "token_missing"
		result.ErrorMessage = "HMS push token is missing"
		return result
	}
	data, _ := json.Marshal(vendorEnvelope(envelope))
	ttl := envelope.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	androidPayload := map[string]any{
		"ttl":     fmt.Sprintf("%ds", int(ttl.Seconds())),
		"urgency": map[bool]string{true: "HIGH", false: "NORMAL"}[strings.EqualFold(envelope.Priority, "high")],
	}
	if !endpoint.SupportsNativeData("hms") {
		clickAction := map[string]any{"type": 3}
		if intentURI := androidIntentURI(envelope.DeepLink, p.packageID); intentURI != "" {
			clickAction = map[string]any{
				"type":   1,
				"intent": intentURI,
			}
		}
		androidPayload["notification"] = map[string]any{
			"title":        envelope.Title,
			"body":         envelope.Body,
			"channel_id":   androidChannelForEnvelope(envelope),
			"notify_id":    stableVendorNotifyID(envelope),
			"click_action": clickAction,
		}
	}
	message := map[string]any{
		"token":   []string{strings.TrimSpace(endpoint.HMSToken)},
		"data":    string(data),
		"android": androidPayload,
	}
	body, _ := json.Marshal(map[string]any{"validate_only": false, "message": message})
	target := "https://push-api.cloud.huawei.com/v2/" + url.PathEscape(p.projectID) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		result.ErrorCode = "network_error"
		result.ErrorMessage = err.Error()
		return result
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = strings.TrimSpace(string(responseBody))
		return result
	}
	var decoded map[string]any
	_ = json.Unmarshal(responseBody, &decoded)
	code := anyString(decoded["code"])
	if code != "" && code != "80000000" {
		result.ErrorCode = "hms_" + code
		result.ErrorMessage = anyString(decoded["msg"])
		if code == "80300007" || code == "80300010" {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	result.ProviderMessageID = anyString(decoded["requestId"])
	return result
}

type OppoProvider struct {
	appKey       string
	masterSecret string
	baseURL      string
	client       *http.Client
	mu           sync.Mutex
	authToken    string
	tokenExpiry  time.Time
}

func NewOppoProviderFromEnvironment() *OppoProvider {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("AMITIA_OPPO_PUSH_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://api.push.oppomobile.com"
	}
	return &OppoProvider{
		appKey:       strings.TrimSpace(os.Getenv("AMITIA_OPPO_APP_KEY")),
		masterSecret: strings.TrimSpace(os.Getenv("AMITIA_OPPO_MASTER_SECRET")),
		baseURL:      baseURL,
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *OppoProvider) Available() bool {
	return p != nil && p.appKey != "" && p.masterSecret != ""
}

func (p *OppoProvider) accessTokenFor(ctx context.Context) (string, error) {
	if !p.Available() {
		return "", fmt.Errorf("OPPO Push is not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authToken != "" && time.Until(p.tokenExpiry) > 10*time.Minute {
		return p.authToken, nil
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sum := sha256.Sum256([]byte(p.appKey + timestamp + p.masterSecret))
	sign := hex.EncodeToString(sum[:])
	form := url.Values{}
	form.Set("app_key", p.appKey)
	form.Set("sign", sign)
	form.Set("timestamp", timestamp)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/server/v1/auth",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OPPO auth failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	if anyInt(decoded["code"]) != 0 {
		return "", fmt.Errorf("OPPO auth failed: %s", anyString(decoded["message"]))
	}
	data, _ := decoded["data"].(map[string]any)
	token := strings.TrimSpace(anyString(data["auth_token"]))
	if token == "" {
		return "", fmt.Errorf("OPPO auth response did not contain auth_token")
	}
	p.authToken = token
	p.tokenExpiry = time.Now().Add(23 * time.Hour)
	return token, nil
}

func (p *OppoProvider) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "oppo"}
	registrationID := strings.TrimSpace(endpoint.OppoToken)
	if registrationID == "" {
		result.ErrorCode = "token_missing"
		result.ErrorMessage = "OPPO registration_id is missing"
		return result
	}
	authToken, err := p.accessTokenFor(ctx)
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	ttl := envelope.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if ttl > 72*time.Hour {
		ttl = 72 * time.Hour
	}
	notification := map[string]any{
		"app_message_id":    envelope.NotificationID,
		"notify_id":         stableVendorNotifyID(envelope),
		"channel_id":        androidChannelForEnvelope(envelope),
		"style":             1,
		"title":             truncateRunes(envelope.Title, 50),
		"content":           truncateRunes(envelope.Body, 50),
		"off_line":          true,
		"off_line_ttl":      int(ttl.Seconds()),
		"click_action_type": 0,
	}
	if strings.TrimSpace(envelope.DeepLink) != "" {
		notification["click_action_type"] = 5
		notification["click_action_url"] = envelope.DeepLink
	}
	message := map[string]any{
		"target_type":  2,
		"target_value": registrationID,
		"notification": notification,
	}
	messageJSON, _ := json.Marshal(message)
	form := url.Values{}
	form.Set("message", string(messageJSON))
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/server/v1/message/notification/unicast",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("auth_token", authToken)
	resp, err := p.client.Do(req)
	if err != nil {
		result.ErrorCode = "network_error"
		result.ErrorMessage = err.Error()
		return result
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = strings.TrimSpace(string(body))
		return result
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		result.ErrorCode = "invalid_response"
		result.ErrorMessage = err.Error()
		return result
	}
	code := anyInt(decoded["code"])
	if code != 0 {
		result.ErrorCode = fmt.Sprintf("oppo_%d", code)
		result.ErrorMessage = anyString(decoded["message"])
		if code == 10000 || strings.Contains(strings.ToLower(result.ErrorMessage), "registration") {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	if data, ok := decoded["data"].(map[string]any); ok {
		result.ProviderMessageID = anyString(data["messageId"])
		if result.ProviderMessageID == "" {
			result.ProviderMessageID = anyString(data["message_id"])
		}
	}
	return result
}

type VivoProvider struct {
	appID       string
	appKey      string
	appSecret   string
	baseURL     string
	client      *http.Client
	mu          sync.Mutex
	authToken   string
	tokenExpiry time.Time
}

func NewVivoProviderFromEnvironment() *VivoProvider {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("AMITIA_VIVO_PUSH_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://api-push.vivo.com.cn"
	}
	return &VivoProvider{
		appID:     strings.TrimSpace(os.Getenv("AMITIA_VIVO_APP_ID")),
		appKey:    strings.TrimSpace(os.Getenv("AMITIA_VIVO_APP_KEY")),
		appSecret: strings.TrimSpace(os.Getenv("AMITIA_VIVO_APP_SECRET")),
		baseURL:   baseURL,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *VivoProvider) Available() bool {
	return p != nil && p.appID != "" && p.appKey != "" && p.appSecret != ""
}

func (p *VivoProvider) accessTokenFor(ctx context.Context) (string, error) {
	if !p.Available() {
		return "", fmt.Errorf("vivo Push is not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authToken != "" && time.Until(p.tokenExpiry) > 2*time.Hour {
		return p.authToken, nil
	}
	timestamp := time.Now().UnixMilli()
	raw := p.appID + p.appKey + strconv.FormatInt(timestamp, 10) + p.appSecret
	sum := md5.Sum([]byte(strings.TrimSpace(raw)))
	body, _ := json.Marshal(map[string]any{
		"appId":     vivoAppIDValue(p.appID),
		"appKey":    p.appKey,
		"timestamp": timestamp,
		"sign":      hex.EncodeToString(sum[:]),
	})
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/message/auth",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("vivo auth failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var decoded map[string]any
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", err
	}
	if anyInt(decoded["result"]) != 0 {
		return "", fmt.Errorf("vivo auth failed: %s", anyString(decoded["desc"]))
	}
	token := strings.TrimSpace(anyString(decoded["authToken"]))
	if token == "" {
		return "", fmt.Errorf("vivo auth response did not contain authToken")
	}
	p.authToken = token
	p.tokenExpiry = time.Now().Add(22 * time.Hour)
	return token, nil
}

func (p *VivoProvider) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "vivo"}
	regID := strings.TrimSpace(endpoint.VivoToken)
	if regID == "" {
		result.ErrorCode = "token_missing"
		result.ErrorMessage = "vivo regId is missing"
		return result
	}
	authToken, err := p.accessTokenFor(ctx)
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	ttl := envelope.TTL
	if ttl < 60*time.Second {
		ttl = 60 * time.Second
	}
	if ttl > 24*time.Hour {
		ttl = 24 * time.Hour
	}
	payload := map[string]any{
		"regId":          regID,
		"notifyType":     1,
		"notifyId":       stableVendorNotifyID(envelope),
		"title":          truncateRunes(envelope.Title, 40),
		"content":        truncateRunes(envelope.Body, 100),
		"timeToLive":     int(ttl.Seconds()),
		"skipType":       1,
		"networkType":    -1,
		"classification": vivoClassification(envelope),
		"pushMode":       0,
		"requestId":      truncateRunes(envelope.NotificationID, 64),
	}
	if intentURI := androidIntentURI(envelope.DeepLink, androidPackageID()); intentURI != "" {
		payload["skipType"] = 4
		payload["skipContent"] = intentURI
	}
	custom := map[string]string{
		"type":           envelope.Type,
		"deepLink":       envelope.DeepLink,
		"conversationId": envelope.ConversationID,
		"messageId":      envelope.MessageID,
		"runId":          envelope.RunID,
	}
	if callID := strings.TrimSpace(envelope.Data["callId"]); callID != "" {
		custom["callId"] = callID
	}
	payload["clientCustomMap"] = custom
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/message/send",
		bytes.NewReader(body),
	)
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authToken", authToken)
	resp, err := p.client.Do(req)
	if err != nil {
		result.ErrorCode = "network_error"
		result.ErrorMessage = err.Error()
		return result
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = strings.TrimSpace(string(responseBody))
		return result
	}
	var decoded map[string]any
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		result.ErrorCode = "invalid_response"
		result.ErrorMessage = err.Error()
		return result
	}
	code := anyInt(decoded["result"])
	if code != 0 {
		result.ErrorCode = fmt.Sprintf("vivo_%d", code)
		result.ErrorMessage = anyString(decoded["desc"])
		if code == 10302 || strings.Contains(strings.ToLower(result.ErrorMessage), "regid") {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	result.ProviderMessageID = anyString(decoded["taskId"])
	return result
}

func vivoClassification(envelope PushEnvelope) int {
	if strings.HasPrefix(envelope.Type, "proactive.") {
		return 0
	}
	return 1
}

func vivoAppIDValue(raw string) any {
	if value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
		return value
	}
	return strings.TrimSpace(raw)
}

type HonorProvider struct {
	clientID     string
	clientSecret string
	appID        string
	packageID    string
	authBaseURL  string
	pushBaseURL  string
	client       *http.Client
	mu           sync.Mutex
	accessToken  string
	tokenExpiry  time.Time
}

func NewHonorProviderFromEnvironment() *HonorProvider {
	authBase := strings.TrimRight(strings.TrimSpace(os.Getenv("AMITIA_HONOR_AUTH_BASE_URL")), "/")
	if authBase == "" {
		authBase = "https://iam.developer.honor.com"
	}
	pushBase := strings.TrimRight(strings.TrimSpace(os.Getenv("AMITIA_HONOR_PUSH_BASE_URL")), "/")
	if pushBase == "" {
		pushBase = "https://push-api.cloud.hihonor.com"
	}
	return &HonorProvider{
		clientID:     strings.TrimSpace(os.Getenv("AMITIA_HONOR_CLIENT_ID")),
		clientSecret: strings.TrimSpace(os.Getenv("AMITIA_HONOR_CLIENT_SECRET")),
		appID:        strings.TrimSpace(os.Getenv("AMITIA_HONOR_APP_ID")),
		packageID:    androidPackageID(),
		authBaseURL:  authBase,
		pushBaseURL:  pushBase,
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *HonorProvider) Available() bool {
	return p != nil && p.clientID != "" && p.clientSecret != "" && p.appID != ""
}

func (p *HonorProvider) accessTokenFor(ctx context.Context) (string, error) {
	if !p.Available() {
		return "", fmt.Errorf("HONOR Push is not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.accessToken != "" && time.Until(p.tokenExpiry) > 2*time.Minute {
		return p.accessToken, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.authBaseURL+"/auth/realms/developer/protocol/openid-connect/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The China endpoint also exposes /auth/token. Retry once there when
		// the realm path is not enabled for this tenant.
		retryReq, retryErr := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			p.authBaseURL+"/auth/token",
			strings.NewReader(form.Encode()),
		)
		if retryErr != nil {
			return "", retryErr
		}
		retryReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		retryResp, retryErr := p.client.Do(retryReq)
		if retryErr != nil {
			return "", retryErr
		}
		defer retryResp.Body.Close()
		body, _ = io.ReadAll(io.LimitReader(retryResp.Body, 1<<20))
		if retryResp.StatusCode < 200 || retryResp.StatusCode >= 300 {
			return "", fmt.Errorf("HONOR oauth failed: status=%d body=%s", retryResp.StatusCode, strings.TrimSpace(string(body)))
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	token := strings.TrimSpace(anyString(decoded["access_token"]))
	if token == "" {
		return "", fmt.Errorf("HONOR oauth response did not contain access token")
	}
	expires := anyInt(decoded["expires_in"])
	if expires <= 0 {
		expires = 3600
	}
	p.accessToken = token
	p.tokenExpiry = time.Now().Add(time.Duration(expires) * time.Second)
	return token, nil
}

func (p *HonorProvider) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "honor"}
	if strings.TrimSpace(endpoint.HonorToken) == "" {
		result.ErrorCode = "token_missing"
		result.ErrorMessage = "HONOR push token is missing"
		return result
	}
	token, err := p.accessTokenFor(ctx)
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	data, _ := json.Marshal(vendorEnvelope(envelope))
	androidPayload := map[string]any{}
	if !endpoint.SupportsNativeData("honor") {
		clickAction := map[string]any{"type": 3}
		if intentURI := androidIntentURI(envelope.DeepLink, p.packageID); intentURI != "" {
			clickAction = map[string]any{
				"type":   1,
				"intent": intentURI,
			}
		}
		androidPayload["notification"] = map[string]any{
			"title":       envelope.Title,
			"body":        envelope.Body,
			"clickAction": clickAction,
		}
	}
	message := map[string]any{
		"data":    string(data),
		"android": androidPayload,
		"token":   []string{strings.TrimSpace(endpoint.HonorToken)},
	}
	body, _ := json.Marshal(message)
	target := p.pushBaseURL + "/api/v1/" + url.PathEscape(p.appID) + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	resp, err := p.client.Do(req)
	if err != nil {
		result.ErrorCode = "network_error"
		result.ErrorMessage = err.Error()
		return result
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.ErrorCode = fmt.Sprintf("http_%d", resp.StatusCode)
		result.ErrorMessage = strings.TrimSpace(string(responseBody))
		return result
	}
	var decoded map[string]any
	_ = json.Unmarshal(responseBody, &decoded)
	code := anyInt(decoded["code"])
	if code != 0 {
		result.ErrorCode = fmt.Sprintf("honor_%d", code)
		result.ErrorMessage = anyString(decoded["message"])
		if strings.Contains(strings.ToLower(result.ErrorMessage), "token") {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	result.ProviderMessageID = anyString(decoded["requestId"])
	if result.ProviderMessageID == "" {
		result.ProviderMessageID = anyString(decoded["data"])
	}
	return result
}

func vendorEnvelope(envelope PushEnvelope) map[string]any {
	data := map[string]any{
		"notificationId": envelope.NotificationID,
		"type":           envelope.Type,
		"spaceId":        envelope.SpaceID,
		"conversationId": envelope.ConversationID,
		"characterId":    envelope.CharacterID,
		"messageId":      envelope.MessageID,
		"runId":          envelope.RunID,
		"revision":       envelope.Revision,
		"title":          envelope.Title,
		"body":           envelope.Body,
		"deepLink":       envelope.DeepLink,
	}
	for key, value := range envelope.Data {
		data[key] = value
	}
	return data
}

func androidPackageID() string {
	value := strings.TrimSpace(os.Getenv("AMITIA_ANDROID_PACKAGE"))
	if value == "" {
		return "com.amitia.amitia_app"
	}
	return value
}

func androidIntentURI(rawDeepLink, packageID string) string {
	rawDeepLink = strings.TrimSpace(rawDeepLink)
	if rawDeepLink == "" {
		return ""
	}
	parsed, err := url.Parse(rawDeepLink)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if packageID = strings.TrimSpace(packageID); packageID == "" {
		packageID = androidPackageID()
	}
	var builder strings.Builder
	builder.WriteString("intent://")
	builder.WriteString(parsed.Host)
	builder.WriteString(parsed.EscapedPath())
	if parsed.RawQuery != "" {
		builder.WriteByte('?')
		builder.WriteString(parsed.RawQuery)
	}
	builder.WriteString("#Intent;scheme=")
	builder.WriteString(parsed.Scheme)
	builder.WriteString(";package=")
	builder.WriteString(packageID)
	builder.WriteString(";launchFlags=0x04000000;end")
	return builder.String()
}

func androidChannelForEnvelope(envelope PushEnvelope) string {
	switch {
	case strings.HasPrefix(envelope.Type, "message."):
		return "amitia_messages"
	case strings.HasPrefix(envelope.Type, "run."):
		return "amitia_tasks"
	case strings.HasPrefix(envelope.Type, "call."):
		return "amitia_calls"
	case strings.HasPrefix(envelope.Type, "reminder."),
		strings.HasPrefix(envelope.Type, "proactive."):
		return "amitia_reminders"
	default:
		return "amitia_system"
	}
}

func stableVendorNotifyID(envelope PushEnvelope) int {
	value := int64(17)
	source := strings.TrimSpace(envelope.RunID)
	if source == "" {
		source = strings.TrimSpace(envelope.ConversationID)
	}
	if source == "" {
		source = strings.TrimSpace(envelope.MessageID)
	}
	if source == "" {
		source = strings.TrimSpace(envelope.NotificationID)
	}
	for _, r := range source {
		value = (value*31 + int64(r)) & 0x7fffffff
	}
	return int(value)
}

func anyString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func anyInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		number, _ := strconv.Atoi(typed.String())
		return number
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}
