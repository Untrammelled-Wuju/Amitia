package notificationruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type PushProvider interface {
	Name() string
	Available() bool
	Send(context.Context, DeviceEndpoint, PushEnvelope) ProviderResult
}

type PushGateway struct {
	repo         *Repository
	fcm          *FCMProvider
	apns         *APNSProvider
	xiaomi       *XiaomiProvider
	huawei       *HuaweiProvider
	honor        *HonorProvider
	oppo         *OppoProvider
	vivo         *VivoProvider
	native       *NativeBridgeProvider
	preferNative bool
}

func NewPushGateway(repo *Repository, native *NativeBridgeProvider, preferNative bool) *PushGateway {
	return &PushGateway{
		repo:         repo,
		fcm:          NewFCMProviderFromEnvironment(),
		apns:         NewAPNSProviderFromEnvironment(),
		xiaomi:       NewXiaomiProviderFromEnvironment(),
		huawei:       NewHuaweiProviderFromEnvironment(),
		honor:        NewHonorProviderFromEnvironment(),
		oppo:         NewOppoProviderFromEnvironment(),
		vivo:         NewVivoProviderFromEnvironment(),
		native:       native,
		preferNative: preferNative,
	}
}

func (g *PushGateway) PreferNative() bool {
	return g != nil && g.preferNative
}

func (g *PushGateway) Capabilities() map[string]bool {
	nativeAvailable := false
	if g != nil && g.native != nil {
		nativeAvailable = g.native.Available(context.Background())
	}
	return map[string]bool{
		"fcm":          g != nil && g.fcm != nil && g.fcm.Available(),
		"apns":         g != nil && g.apns != nil && g.apns.Available(),
		"mipush":       g != nil && g.xiaomi != nil && g.xiaomi.Available(),
		"hms":          g != nil && g.huawei != nil && g.huawei.Available(),
		"honor":        g != nil && g.honor != nil && g.honor.Available(),
		"oppo":         g != nil && g.oppo != nil && g.oppo.Available(),
		"vivo":         g != nil && g.vivo != nil && g.vivo.Available(),
		"liveActivity": g != nil && g.apns != nil && g.apns.Available(),
		"native":       nativeAvailable,
	}
}

// NativeDataCapabilities describes the Cloud Core side of the Android data plane.
// Effective native-data delivery is the intersection of this map and the
// device-reported nativeDataProviders. OPPO/vivo intentionally remain false:
// their public server APIs do not expose a stable pass-through endpoint, so the
// Cloud Core must retain vendor-system notification delivery instead of silently
// switching to a payload the OS may never deliver to the app.
func (g *PushGateway) NativeDataCapabilities() map[string]bool {
	return map[string]bool{
		"mipush": g != nil && g.xiaomi != nil && g.xiaomi.Available(),
		"hms":    g != nil && g.huawei != nil && g.huawei.Available(),
		"honor":  g != nil && g.honor != nil && g.honor.Available(),
		"oppo":   false,
		"vivo":   false,
	}
}

func (g *PushGateway) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	if g == nil {
		return ProviderResult{ErrorCode: "gateway_unavailable", ErrorMessage: "push gateway unavailable"}
	}
	if g.preferNative && g.native != nil && g.native.Available(ctx) {
		result := g.native.Send(ctx, endpoint, envelope)
		if result.Accepted {
			return result
		}
	}
	var result ProviderResult
	switch strings.ToLower(endpoint.Platform) {
	case "android":
		result = g.sendAndroid(ctx, endpoint, envelope)
	case "ios":
		if g.apns == nil || !g.apns.Available() {
			return ProviderResult{Provider: "apns", ErrorCode: "provider_unavailable", ErrorMessage: "APNs is not configured"}
		}
		if strings.HasPrefix(envelope.Type, "call.") && strings.TrimSpace(endpoint.VoIPToken) != "" {
			result = g.apns.SendVoIP(ctx, endpoint, envelope)
			if result.Accepted {
				break
			}
			if result.InvalidToken && g.repo != nil {
				_ = g.repo.DisableToken(ctx, &endpoint, result.Provider)
				endpoint.VoIPToken = ""
			}
			if !strings.EqualFold(envelope.Type, "call.incoming") ||
				strings.TrimSpace(endpoint.APNSToken) == "" {
				break
			}
			// A stale VoIP token must not make an incoming call disappear.
			// Fall back to a visible APNs alert/deep-link. This does not replace
			// CallKit, but it preserves reachability until PushKit rotates a
			// valid token and the next registration reaches the Core.
		}
		if strings.TrimSpace(endpoint.APNSToken) == "" {
			return ProviderResult{Provider: "apns", ErrorCode: "provider_unavailable", ErrorMessage: "APNs device token is missing"}
		}
		result = g.apns.Send(ctx, endpoint, envelope)
	default:
		return ProviderResult{ErrorCode: "platform_unsupported", ErrorMessage: "unsupported push platform"}
	}
	if result.InvalidToken && g.repo != nil {
		_ = g.repo.DisableToken(ctx, &endpoint, result.Provider)
	}
	return result
}

func (g *PushGateway) sendAndroid(
	ctx context.Context,
	endpoint DeviceEndpoint,
	envelope PushEnvelope,
) ProviderResult {
	preferred := strings.ToLower(strings.TrimSpace(endpoint.PreferredProvider))
	order := make([]string, 0, 4)
	appendProvider := func(provider string) {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			return
		}
		for _, existing := range order {
			if existing == provider {
				return
			}
		}
		order = append(order, provider)
	}

	if strings.HasPrefix(envelope.Type, "call.") {
		// On domestic Android devices a valid manufacturer token is a stronger
		// reachability signal than GMS availability. Prefer the device-selected
		// vendor channel and retain FCM as fallback; generic/GMS devices still
		// resolve to FCM because their preferred provider is fcm.
		appendProvider(preferred)
		appendProvider("fcm")
		if endpoint.HMSToken != "" {
			appendProvider("hms")
		}
		if endpoint.MiPushToken != "" {
			appendProvider("mipush")
		}
		if endpoint.HonorToken != "" {
			appendProvider("honor")
		}
		if endpoint.OppoToken != "" {
			appendProvider("oppo")
		}
		if endpoint.VivoToken != "" {
			appendProvider("vivo")
		}
	} else {
		appendProvider(preferred)
		if endpoint.HMSToken != "" {
			appendProvider("hms")
		}
		if endpoint.MiPushToken != "" {
			appendProvider("mipush")
		}
		if endpoint.HonorToken != "" {
			appendProvider("honor")
		}
		if endpoint.OppoToken != "" {
			appendProvider("oppo")
		}
		if endpoint.VivoToken != "" {
			appendProvider("vivo")
		}
		appendProvider("fcm")
	}

	var last ProviderResult
	for _, provider := range order {
		var result ProviderResult
		switch provider {
		case "hms":
			if g.huawei == nil || !g.huawei.Available() || strings.TrimSpace(endpoint.HMSToken) == "" {
				continue
			}
			result = g.huawei.Send(ctx, endpoint, envelope)
		case "mipush":
			if g.xiaomi == nil || !g.xiaomi.Available() || strings.TrimSpace(endpoint.MiPushToken) == "" {
				continue
			}
			result = g.xiaomi.Send(ctx, endpoint, envelope)
		case "honor":
			if g.honor == nil || !g.honor.Available() || strings.TrimSpace(endpoint.HonorToken) == "" {
				continue
			}
			result = g.honor.Send(ctx, endpoint, envelope)
		case "oppo":
			if g.oppo == nil || !g.oppo.Available() || strings.TrimSpace(endpoint.OppoToken) == "" {
				continue
			}
			result = g.oppo.Send(ctx, endpoint, envelope)
		case "vivo":
			if g.vivo == nil || !g.vivo.Available() || strings.TrimSpace(endpoint.VivoToken) == "" {
				continue
			}
			result = g.vivo.Send(ctx, endpoint, envelope)
		case "fcm":
			if g.fcm == nil || !g.fcm.Available() || strings.TrimSpace(endpoint.FCMToken) == "" {
				continue
			}
			result = g.fcm.Send(ctx, endpoint, envelope)
		default:
			continue
		}
		last = result
		if result.Accepted {
			return result
		}
		if result.InvalidToken {
			if g.repo != nil {
				_ = g.repo.DisableToken(ctx, &endpoint, result.Provider)
			}
			continue
		}
		if result.ErrorCode == "provider_unavailable" || result.ErrorCode == "token_missing" {
			continue
		}
		// Transient and provider-specific errors must be retried on the same
		// channel by the delivery worker to avoid duplicate cross-channel pushes.
		return result
	}
	if last.Provider != "" {
		return last
	}
	return ProviderResult{
		Provider:     preferred,
		ErrorCode:    "provider_unavailable",
		ErrorMessage: "no configured Android push provider has a usable token",
	}
}

func (g *PushGateway) SendLiveActivity(ctx context.Context, endpoint DeviceEndpoint, token string, state ExecutionState, event string) ProviderResult {
	if g == nil || g.apns == nil || !g.apns.Available() {
		return ProviderResult{Provider: "apns", ErrorCode: "provider_unavailable", ErrorMessage: "APNs live activity is unavailable"}
	}
	return g.apns.SendLiveActivity(ctx, endpoint, token, state, event)
}

type fcmServiceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type FCMProvider struct {
	account     fcmServiceAccount
	client      *http.Client
	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewFCMProviderFromEnvironment() *FCMProvider {
	raw := strings.TrimSpace(os.Getenv("AMITIA_FCM_SERVICE_ACCOUNT_JSON"))
	if raw == "" {
		if path := strings.TrimSpace(os.Getenv("AMITIA_FCM_SERVICE_ACCOUNT_FILE")); path != "" {
			data, err := os.ReadFile(path)
			if err == nil {
				raw = string(data)
			}
		}
	}
	var account fcmServiceAccount
	_ = json.Unmarshal([]byte(raw), &account)
	if account.TokenURI == "" {
		account.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return &FCMProvider{account: account, client: &http.Client{Timeout: 15 * time.Second}}
}

func (p *FCMProvider) Name() string { return "fcm" }

func (p *FCMProvider) Available() bool {
	return p != nil && strings.TrimSpace(p.account.ProjectID) != "" && strings.TrimSpace(p.account.ClientEmail) != "" && strings.TrimSpace(p.account.PrivateKey) != ""
}

func (p *FCMProvider) accessTokenFor(ctx context.Context) (string, error) {
	if !p.Available() {
		return "", errors.New("FCM service account is not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.accessToken != "" && time.Until(p.tokenExpiry) > 2*time.Minute {
		return p.accessToken, nil
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(p.account.PrivateKey))
	if err != nil {
		return "", err
	}
	now := time.Now()
	claim := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   p.account.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   p.account.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(55 * time.Minute).Unix(),
	})
	assertion, err := claim.SignedString(key)
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.account.TokenURI, strings.NewReader(form.Encode()))
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
		return "", fmt.Errorf("FCM oauth failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", errors.New("FCM oauth response did not contain access token")
	}
	p.accessToken = payload.AccessToken
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 3600
	}
	p.tokenExpiry = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	return p.accessToken, nil
}

func (p *FCMProvider) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "fcm"}
	accessToken, err := p.accessTokenFor(ctx)
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	data := map[string]string{
		"notificationId": envelope.NotificationID,
		"type":           envelope.Type,
		"spaceId":        envelope.SpaceID,
		"conversationId": envelope.ConversationID,
		"characterId":    envelope.CharacterID,
		"messageId":      envelope.MessageID,
		"runId":          envelope.RunID,
		"revision":       strconv.FormatInt(envelope.Revision, 10),
		"title":          envelope.Title,
		"body":           envelope.Body,
		"deepLink":       envelope.DeepLink,
		"sound":          strconv.FormatBool(envelope.Sound),
	}
	for key, value := range envelope.Data {
		data[key] = value
	}
	ttl := envelope.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	priority := "NORMAL"
	if strings.EqualFold(envelope.Priority, "high") {
		priority = "HIGH"
	}
	body := map[string]any{
		"message": map[string]any{
			"token": endpoint.FCMToken,
			"data":  data,
			"android": map[string]any{
				"priority":     priority,
				"ttl":          fmt.Sprintf("%ds", int(ttl.Seconds())),
				"collapse_key": collapseKey(envelope),
			},
		},
	}
	encoded, _ := json.Marshal(body)
	target := "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(p.account.ProjectID) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(encoded))
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
		if resp.StatusCode == http.StatusNotFound || strings.Contains(result.ErrorMessage, "UNREGISTERED") {
			result.InvalidToken = true
		}
		return result
	}
	var response struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(responseBody, &response)
	result.Accepted = true
	result.ProviderMessageID = response.Name
	return result
}

func collapseKey(envelope PushEnvelope) string {
	switch {
	case strings.HasPrefix(envelope.Type, "run.") && strings.TrimSpace(envelope.RunID) != "":
		return "run:" + strings.TrimSpace(envelope.RunID)
	case (strings.HasPrefix(envelope.Type, "message.") ||
		strings.HasPrefix(envelope.Type, "reminder.") ||
		strings.HasPrefix(envelope.Type, "proactive.")) &&
		strings.TrimSpace(envelope.MessageID) != "":
		return "message:" + strings.TrimSpace(envelope.MessageID)
	case strings.HasPrefix(envelope.Type, "call.") &&
		strings.TrimSpace(envelope.Data["callId"]) != "":
		return "call:" + strings.TrimSpace(envelope.Data["callId"])
	default:
		return strings.TrimSpace(envelope.Type)
	}
}

type APNSProvider struct {
	teamID       string
	keyID        string
	bundleID     string
	privateKey   string
	production   bool
	client       *http.Client
	mu           sync.Mutex
	authToken    string
	authIssuedAt time.Time
}

func NewAPNSProviderFromEnvironment() *APNSProvider {
	privateKey := strings.TrimSpace(os.Getenv("AMITIA_APNS_PRIVATE_KEY"))
	if privateKey == "" {
		if path := strings.TrimSpace(os.Getenv("AMITIA_APNS_PRIVATE_KEY_FILE")); path != "" {
			data, err := os.ReadFile(path)
			if err == nil {
				privateKey = string(data)
			}
		}
	}
	return &APNSProvider{
		teamID:     strings.TrimSpace(os.Getenv("AMITIA_APNS_TEAM_ID")),
		keyID:      strings.TrimSpace(os.Getenv("AMITIA_APNS_KEY_ID")),
		bundleID:   strings.TrimSpace(os.Getenv("AMITIA_APNS_BUNDLE_ID")),
		privateKey: privateKey,
		production: strings.EqualFold(strings.TrimSpace(os.Getenv("AMITIA_APNS_ENV")), "production"),
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *APNSProvider) Name() string { return "apns" }

func (p *APNSProvider) Available() bool {
	return p != nil && p.teamID != "" && p.keyID != "" && p.bundleID != "" && p.privateKey != ""
}

func (p *APNSProvider) authorizationToken() (string, error) {
	if !p.Available() {
		return "", errors.New("APNs is not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authToken != "" && time.Since(p.authIssuedAt) < 45*time.Minute {
		return p.authToken, nil
	}
	key, err := jwt.ParseECPrivateKeyFromPEM([]byte(p.privateKey))
	if err != nil {
		return "", err
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": p.teamID,
		"iat": now.Unix(),
	})
	token.Header["kid"] = p.keyID
	value, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	p.authToken = value
	p.authIssuedAt = now
	return value, nil
}

func (p *APNSProvider) endpoint(token string) string {
	host := "https://api.sandbox.push.apple.com"
	if p.production {
		host = "https://api.push.apple.com"
	}
	return host + "/3/device/" + url.PathEscape(strings.TrimSpace(token))
}

func (p *APNSProvider) Send(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "apns"}
	auth, err := p.authorizationToken()
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	aps := map[string]any{
		"thread-id": envelope.ConversationID,
	}
	if strings.HasPrefix(envelope.Type, "message.") {
		aps["mutable-content"] = 1
	}
	if envelope.Title != "" || envelope.Body != "" {
		aps["alert"] = map[string]string{"title": envelope.Title, "body": envelope.Body}
	}
	if strings.HasPrefix(envelope.Type, "message.") {
		aps["category"] = "AMITIA_MESSAGE"
	} else if envelope.Type == "call.incoming" {
		aps["category"] = "AMITIA_CALL_FALLBACK"
	} else if strings.HasPrefix(envelope.Type, "reminder.") ||
		strings.HasPrefix(envelope.Type, "proactive.") {
		aps["category"] = "AMITIA_REMINDER"
	} else if strings.HasPrefix(envelope.Type, "run.") {
		aps["category"] = "AMITIA_EXECUTION"
	}
	if envelope.Type == "call.ended" {
		aps["content-available"] = 1
	}
	if envelope.Sound {
		aps["sound"] = "default"
	}
	if envelope.Badge > 0 {
		aps["badge"] = envelope.Badge
	}
	payload := map[string]any{
		"aps":            aps,
		"notificationId": envelope.NotificationID,
		"type":           envelope.Type,
		"spaceId":        envelope.SpaceID,
		"conversationId": envelope.ConversationID,
		"characterId":    envelope.CharacterID,
		"messageId":      envelope.MessageID,
		"runId":          envelope.RunID,
		"revision":       envelope.Revision,
		"deepLink":       envelope.DeepLink,
		"data":           envelope.Data,
	}
	encoded, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(endpoint.APNSToken), bytes.NewReader(encoded))
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Authorization", "bearer "+auth)
	req.Header.Set("apns-topic", p.bundleID)
	if envelope.Type == "call.ended" {
		req.Header.Set("apns-push-type", "background")
		req.Header.Set("apns-priority", "5")
	} else {
		req.Header.Set("apns-push-type", "alert")
		if strings.EqualFold(envelope.Priority, "high") {
			req.Header.Set("apns-priority", "10")
		} else {
			req.Header.Set("apns-priority", "5")
		}
	}
	if envelope.TTL > 0 {
		req.Header.Set("apns-expiration", strconv.FormatInt(time.Now().Add(envelope.TTL).Unix(), 10))
	}
	if key := collapseKey(envelope); key != "" {
		req.Header.Set("apns-collapse-id", truncateRunes(key, 64))
	}
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
		if resp.StatusCode == http.StatusGone || strings.Contains(result.ErrorMessage, "BadDeviceToken") || strings.Contains(result.ErrorMessage, "Unregistered") {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	result.ProviderMessageID = resp.Header.Get("apns-id")
	return result
}

func (p *APNSProvider) SendVoIP(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) ProviderResult {
	result := ProviderResult{Provider: "apns-voip"}
	if strings.TrimSpace(endpoint.VoIPToken) == "" {
		result.ErrorCode = "token_missing"
		result.ErrorMessage = "VoIP token is missing"
		return result
	}
	auth, err := p.authorizationToken()
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	aps := map[string]any{"content-available": 1}
	payload := map[string]any{
		"aps":            aps,
		"notificationId": envelope.NotificationID,
		"type":           envelope.Type,
		"spaceId":        envelope.SpaceID,
		"conversationId": envelope.ConversationID,
		"characterId":    envelope.CharacterID,
		"callId":         envelope.Data["callId"],
		"title":          envelope.Title,
		"body":           envelope.Body,
		"deepLink":       envelope.DeepLink,
		"data":           envelope.Data,
	}
	encoded, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(endpoint.VoIPToken), bytes.NewReader(encoded))
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Authorization", "bearer "+auth)
	req.Header.Set("apns-topic", p.bundleID+".voip")
	req.Header.Set("apns-push-type", "voip")
	req.Header.Set("apns-priority", "10")
	if envelope.TTL > 0 {
		req.Header.Set("apns-expiration", strconv.FormatInt(time.Now().Add(envelope.TTL).Unix(), 10))
	}
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
		if resp.StatusCode == http.StatusGone || strings.Contains(result.ErrorMessage, "BadDeviceToken") || strings.Contains(result.ErrorMessage, "Unregistered") {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	result.ProviderMessageID = resp.Header.Get("apns-id")
	return result
}

func liveActivityAPNSPayload(endpoint DeviceEndpoint, state ExecutionState, event string, now time.Time) map[string]any {
	businessAt := state.UpdatedAt
	if businessAt.IsZero() {
		businessAt = now
	}
	startedAt := state.StartedAt
	if startedAt.IsZero() {
		startedAt = businessAt
	}
	contentState := map[string]any{
		"revision":    state.Revision,
		"phase":       state.Phase,
		"title":       state.Title,
		"summary":     state.Summary,
		"currentStep": state.CurrentStep,
		"totalSteps":  state.TotalSteps,
		"progress":    state.Progress,
		"updatedAt":   businessAt.Unix(),
		"locale":      strings.TrimSpace(endpoint.Locale),
		"appearance":  strings.TrimSpace(endpoint.Appearance),
		"agentName":   strings.TrimSpace(state.AgentID),
	}
	if state.TotalTokens > 0 {
		contentState["totalTokens"] = state.TotalTokens
	}
	aps := map[string]any{
		"timestamp":     now.Unix(),
		"event":         event,
		"content-state": contentState,
	}
	if event == "end" {
		aps["relevance-score"] = 0
	} else {
		aps["relevance-score"] = float64(businessAt.Unix())
		// Heartbeats are emitted every two minutes. A five-minute stale window
		// survives transient APNs delay without allowing an abandoned card to
		// look current indefinitely.
		aps["stale-date"] = now.Add(5 * time.Minute).Unix()
	}
	if event == "start" {
		aps["alert"] = map[string]string{
			"title": fallback(state.Title, "Amitia"),
			"body":  executionNotificationBody(state, endpoint.Locale),
		}
		aps["attributes-type"] = "AmitiaRunAttributes"
		aps["attributes"] = map[string]any{
			"runId":          state.RunID,
			"conversationId": state.ConversationID,
			"characterId":    state.CharacterID,
			"agentName":      state.AgentID,
			"startedAt":      startedAt.Unix(),
		}
	}
	if event == "end" {
		if state.Phase == "interrupted" || state.Phase == "cancelled" {
			aps["dismissal-date"] = now.Unix()
		} else {
			aps["dismissal-date"] = now.Add(60 * time.Second).Unix()
		}
	}
	return map[string]any{"aps": aps}
}

func (p *APNSProvider) SendLiveActivity(ctx context.Context, endpoint DeviceEndpoint, token string, state ExecutionState, event string) ProviderResult {
	result := ProviderResult{Provider: "apns"}
	token = strings.TrimSpace(token)
	if token == "" {
		result.ErrorCode = "token_missing"
		result.ErrorMessage = "live activity token is missing"
		return result
	}
	auth, err := p.authorizationToken()
	if err != nil {
		result.ErrorCode = "auth_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	now := time.Now()
	payload := liveActivityAPNSPayload(endpoint, state, event, now)
	encoded, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(token), bytes.NewReader(encoded))
	if err != nil {
		result.ErrorCode = "request_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	req.Header.Set("Authorization", "bearer "+auth)
	req.Header.Set("apns-topic", p.bundleID+".push-type.liveactivity")
	req.Header.Set("apns-push-type", "liveactivity")
	req.Header.Set("apns-priority", "10")
	expiration := 5 * time.Minute
	if event == "end" {
		expiration = time.Minute
	}
	req.Header.Set("apns-expiration", strconv.FormatInt(now.Add(expiration).Unix(), 10))
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
		if resp.StatusCode == http.StatusGone || strings.Contains(result.ErrorMessage, "BadDeviceToken") || strings.Contains(result.ErrorMessage, "Unregistered") {
			result.InvalidToken = true
		}
		return result
	}
	result.Accepted = true
	result.ProviderMessageID = resp.Header.Get("apns-id")
	return result
}
