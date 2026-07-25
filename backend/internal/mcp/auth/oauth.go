// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/mcp/transport"
)

type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

type AuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	ScopesSupported                   []string `json:"scopes_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

type ClientRegistration struct {
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	TokenEndpoint  string `json:"token_endpoint,omitempty"`
	RevokeEndpoint string `json:"revoke_endpoint,omitempty"`
}

type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type StoredToken struct {
	AccessToken  string   `json:"accessToken"`
	TokenType    string   `json:"tokenType"`
	RefreshToken string   `json:"refreshToken"`
	Scopes       []string `json:"scopes"`
	ExpiresAt    string   `json:"expiresAt"`
	TokenURL     string   `json:"tokenUrl"`
	RevokeURL    string   `json:"revokeUrl"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret,omitempty"`
}

type PendingSession struct {
	ID                    string
	ServerID              string
	StateHash             string
	CodeVerifierReference string
	RedirectURI           string
	RequestedScopes       []string
	Status                string
	ExpiresAt             time.Time
}

type SessionStore interface {
	CreateOAuthSession(context.Context, PendingSession) error
	ConsumeOAuthSession(context.Context, string, string) (PendingSession, error)
	SaveOAuthTokenReference(context.Context, string, string, time.Time, []string) error
	OAuthTokenReference(context.Context, string) (string, error)
	DeleteOAuthTokenReference(context.Context, string) error
	DeleteOAuthState(context.Context, string) error
}

type Manager struct {
	client        *http.Client
	secrets       SecretStore
	sessions      SessionStore
	refreshMu     sync.Mutex
	refreshLocks  map[string]*sync.Mutex
	secureDefault bool
}

type BeginRequest struct {
	ServerID     string
	ResourceURL  string
	MetadataURL  string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type BeginResult struct {
	AuthorizationURL string `json:"authorizationUrl"`
	State            string `json:"-"`
	SessionID        string `json:"sessionId"`
}

func NewManager(client *http.Client, secrets SecretStore, sessions SessionStore) *Manager {
	secureDefault := false
	if client == nil {
		base := http.DefaultTransport.(*http.Transport).Clone()
		base.Proxy = nil
		client = &http.Client{Timeout: 15 * time.Second, Transport: base, CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: redirect limit")
			}
			if request.URL.Scheme != "https" || !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: redirect")
			}
			return nil
		}}
		secureDefault = true
	}
	return &Manager{client: client, secrets: secrets, sessions: sessions, refreshLocks: map[string]*sync.Mutex{}, secureDefault: secureDefault}
}

func GeneratePKCE() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func GenerateState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (m *Manager) Discover(ctx context.Context, resourceURL, metadataURL string) (ProtectedResourceMetadata, AuthorizationServerMetadata, error) {
	resource, err := url.Parse(resourceURL)
	if err != nil || resource.Scheme == "" || resource.Host == "" {
		return ProtectedResourceMetadata{}, AuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	if metadataURL == "" {
		metadataURL = resource.Scheme + "://" + resource.Host + "/.well-known/oauth-protected-resource"
	}
	var protected ProtectedResourceMetadata
	if err := m.getMetadata(ctx, metadataURL, resource.Hostname(), &protected); err != nil {
		return protected, AuthorizationServerMetadata{}, err
	}
	if protected.Resource != "" && !sameOrigin(protected.Resource, resourceURL) {
		return protected, AuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: resource mismatch")
	}
	if len(protected.AuthorizationServers) == 0 {
		return protected, AuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: authorization server missing")
	}
	issuer, err := url.Parse(protected.AuthorizationServers[0])
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
		return protected, AuthorizationServerMetadata{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: issuer")
	}
	metadataEndpoint := strings.TrimRight(issuer.String(), "/") + "/.well-known/oauth-authorization-server"
	var authorization AuthorizationServerMetadata
	if err := m.getMetadata(ctx, metadataEndpoint, issuer.Hostname(), &authorization); err != nil {
		return protected, authorization, err
	}
	if authorization.Issuer == "" || strings.TrimRight(authorization.Issuer, "/") != strings.TrimRight(issuer.String(), "/") {
		return protected, authorization, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: issuer mismatch")
	}
	if err := validateAuthorizationMetadata(authorization, issuer.Hostname()); err != nil {
		return protected, authorization, err
	}
	return protected, authorization, nil
}

func (m *Manager) Begin(ctx context.Context, request BeginRequest) (BeginResult, error) {
	protected, metadata, err := m.Discover(ctx, request.ResourceURL, request.MetadataURL)
	if err != nil {
		return BeginResult{}, err
	}
	requested := normalizedScopes(request.Scopes)
	if !scopeSubset(requested, append(protected.ScopesSupported, metadata.ScopesSupported...)) {
		return BeginResult{}, fmt.Errorf("MCP_AUTH_SCOPE_REQUIRED")
	}
	clientID := strings.TrimSpace(request.ClientID)
	clientSecret := strings.TrimSpace(request.ClientSecret)
	if clientID == "" {
		registration, err := m.registerClient(ctx, metadata, request.RedirectURI)
		if err != nil {
			return BeginResult{}, err
		}
		clientID, clientSecret = registration.ClientID, registration.ClientSecret
	}
	registration := ClientRegistration{ClientID: clientID, ClientSecret: clientSecret, TokenEndpoint: metadata.TokenEndpoint, RevokeEndpoint: metadata.RevocationEndpoint}
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return BeginResult{}, err
	}
	state, err := GenerateState()
	if err != nil {
		return BeginResult{}, err
	}
	verifierReference, err := m.secrets.Put(ctx, request.ServerID+"-pkce", []byte(verifier))
	if err != nil {
		return BeginResult{}, err
	}
	clientReference, err := m.secrets.Put(ctx, request.ServerID+"-oauth-client", mustJSON(registration))
	if err != nil {
		m.secrets.Delete(ctx, verifierReference)
		return BeginResult{}, err
	}
	sessionID := clientReference
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	pending := PendingSession{ID: sessionID, ServerID: request.ServerID, StateHash: HashState(state), CodeVerifierReference: verifierReference, RedirectURI: request.RedirectURI, RequestedScopes: requested, Status: "pending", ExpiresAt: expiresAt}
	if err := m.sessions.CreateOAuthSession(ctx, pending); err != nil {
		m.secrets.Delete(ctx, verifierReference)
		m.secrets.Delete(ctx, clientReference)
		return BeginResult{}, err
	}
	authorization, _ := url.Parse(metadata.AuthorizationEndpoint)
	query := authorization.Query()
	query.Set("response_type", "code")
	query.Set("client_id", clientID)
	query.Set("redirect_uri", request.RedirectURI)
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("resource", request.ResourceURL)
	if len(requested) > 0 {
		query.Set("scope", strings.Join(requested, " "))
	}
	authorization.RawQuery = query.Encode()
	return BeginResult{AuthorizationURL: authorization.String(), State: state, SessionID: sessionID}, nil
}

func (m *Manager) Callback(ctx context.Context, sessionID, state, code, redirectURI, tokenURL, revokeURL string) (string, error) {
	if state == "" || code == "" {
		return "", fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	session, err := m.sessions.ConsumeOAuthSession(ctx, sessionID, HashState(state))
	if err != nil || session.ExpiresAt.Before(time.Now().UTC()) || session.Status != "pending" {
		return "", fmt.Errorf("MCP_OAUTH_STATE_INVALID")
	}
	if redirectURI == "" {
		redirectURI = session.RedirectURI
	}
	if session.RedirectURI != redirectURI {
		return "", fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED: redirect URI")
	}
	verifier, err := m.secrets.Get(ctx, session.CodeVerifierReference)
	if err != nil {
		return "", fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	clientRaw, err := m.secrets.Get(ctx, session.ID)
	if err != nil {
		return "", fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	var registration ClientRegistration
	if json.Unmarshal(clientRaw, &registration) != nil {
		return "", fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	if tokenURL != "" && tokenURL != registration.TokenEndpoint {
		return "", fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED: token endpoint")
	}
	if revokeURL != "" && revokeURL != registration.RevokeEndpoint {
		return "", fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED: revoke endpoint")
	}
	tokenURL = registration.TokenEndpoint
	revokeURL = registration.RevokeEndpoint
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}, "client_id": {registration.ClientID}, "code_verifier": {string(verifier)}}
	if registration.ClientSecret != "" {
		form.Set("client_secret", registration.ClientSecret)
	}
	token, err := m.exchange(ctx, tokenURL, form)
	if err != nil {
		return "", err
	}
	stored := storedToken(token, tokenURL, revokeURL, registration)
	reference, err := m.secrets.Put(ctx, session.ServerID+"-oauth-token", mustJSON(stored))
	if err != nil {
		return "", err
	}
	if err := m.sessions.SaveOAuthTokenReference(ctx, session.ServerID, reference, parseTime(stored.ExpiresAt), stored.Scopes); err != nil {
		m.secrets.Delete(ctx, reference)
		return "", err
	}
	m.secrets.Delete(ctx, session.CodeVerifierReference)
	m.secrets.Delete(ctx, session.ID)
	m.sessions.DeleteOAuthState(ctx, sessionID)
	return reference, nil
}

func (m *Manager) AccessToken(ctx context.Context, serverID string) (string, error) {
	reference, err := m.sessions.OAuthTokenReference(ctx, serverID)
	if err != nil {
		return "", err
	}
	raw, err := m.secrets.Get(ctx, reference)
	if err != nil {
		return "", err
	}
	var token StoredToken
	if json.Unmarshal(raw, &token) != nil {
		return "", fmt.Errorf("MCP_AUTH_EXPIRED")
	}
	if token.ExpiresAt != "" && parseTime(token.ExpiresAt).Before(time.Now().UTC().Add(30*time.Second)) {
		token, err = m.Refresh(ctx, serverID)
		if err != nil {
			return "", err
		}
	}
	return token.AccessToken, nil
}

func (m *Manager) Refresh(ctx context.Context, serverID string) (StoredToken, error) {
	lock := m.refreshLock(serverID)
	lock.Lock()
	defer lock.Unlock()
	reference, err := m.sessions.OAuthTokenReference(ctx, serverID)
	if err != nil {
		return StoredToken{}, err
	}
	raw, err := m.secrets.Get(ctx, reference)
	if err != nil {
		return StoredToken{}, err
	}
	var current StoredToken
	if json.Unmarshal(raw, &current) != nil || current.RefreshToken == "" {
		return StoredToken{}, fmt.Errorf("MCP_TOKEN_REFRESH_FAILED")
	}
	if current.ExpiresAt != "" && parseTime(current.ExpiresAt).After(time.Now().UTC().Add(30*time.Second)) {
		return current, nil
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {current.RefreshToken}, "client_id": {current.ClientID}}
	if current.ClientSecret != "" {
		form.Set("client_secret", current.ClientSecret)
	}
	if len(current.Scopes) > 0 {
		form.Set("scope", strings.Join(current.Scopes, " "))
	}
	response, err := m.exchange(ctx, current.TokenURL, form)
	if err != nil {
		return StoredToken{}, fmt.Errorf("MCP_TOKEN_REFRESH_FAILED")
	}
	updated := storedToken(response, current.TokenURL, current.RevokeURL, ClientRegistration{ClientID: current.ClientID, ClientSecret: current.ClientSecret})
	if updated.RefreshToken == "" {
		updated.RefreshToken = current.RefreshToken
	}
	if len(updated.Scopes) == 0 {
		updated.Scopes = current.Scopes
	}
	newReference, err := m.secrets.Put(ctx, serverID+"-oauth-token", mustJSON(updated))
	if err != nil {
		return StoredToken{}, err
	}
	if err := m.sessions.SaveOAuthTokenReference(ctx, serverID, newReference, parseTime(updated.ExpiresAt), updated.Scopes); err != nil {
		m.secrets.Delete(ctx, newReference)
		return StoredToken{}, err
	}
	m.secrets.Delete(ctx, reference)
	return updated, nil
}

func (m *Manager) Revoke(ctx context.Context, serverID string) error {
	reference, err := m.sessions.OAuthTokenReference(ctx, serverID)
	if err != nil {
		return err
	}
	raw, err := m.secrets.Get(ctx, reference)
	if err != nil {
		return err
	}
	var token StoredToken
	if json.Unmarshal(raw, &token) == nil && token.RevokeURL != "" {
		value := token.RefreshToken
		if value == "" {
			value = token.AccessToken
		}
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, token.RevokeURL, strings.NewReader(url.Values{"token": {value}, "client_id": {token.ClientID}}.Encode()))
		if requestErr == nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if response, callErr := m.do(request); callErr == nil {
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
			}
		}
	}
	if err := m.secrets.Delete(ctx, reference); err != nil {
		return err
	}
	return m.sessions.DeleteOAuthTokenReference(ctx, serverID)
}

func (m *Manager) getMetadata(ctx context.Context, endpoint, expectedHost string, target any) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), expectedHost) {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	request.Header.Set("Accept", "application/json")
	response, err := m.do(request)
	if err != nil {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	return nil
}

func (m *Manager) registerClient(ctx context.Context, metadata AuthorizationServerMetadata, redirectURI string) (ClientRegistration, error) {
	if metadata.RegistrationEndpoint == "" {
		return ClientRegistration{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: client registration unavailable")
	}
	payload := mustJSON(map[string]any{"client_name": "Amitia", "redirect_uris": []string{redirectURI}, "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}, "token_endpoint_auth_method": "none"})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, metadata.RegistrationEndpoint, strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	response, err := m.do(request)
	if err != nil {
		return ClientRegistration{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ClientRegistration{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	var registration ClientRegistration
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&registration) != nil || registration.ClientID == "" {
		return ClientRegistration{}, fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	return registration, nil
}

func (m *Manager) exchange(ctx context.Context, endpoint string, form url.Values) (Token, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return Token{}, fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := m.do(request)
	if err != nil {
		return Token{}, fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Token{}, fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	var token Token
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token) != nil || token.AccessToken == "" {
		return Token{}, fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	}
	return token, nil
}

func (m *Manager) do(request *http.Request) (*http.Response, error) {
	if !m.secureDefault {
		return m.client.Do(request)
	}
	security, err := transport.ValidateEndpoint(request.Context(), request.URL.String(), transport.EndpointPolicy{AllowLoopback: true, MaxRedirects: 3})
	if err != nil {
		return nil, err
	}
	secureClient := transport.NewSecureHTTPClient(security, transport.EndpointPolicy{AllowLoopback: true, MaxRedirects: 3}, 15*time.Second)
	return secureClient.Do(request)
}

func (m *Manager) refreshLock(serverID string) *sync.Mutex {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	if m.refreshLocks[serverID] == nil {
		m.refreshLocks[serverID] = &sync.Mutex{}
	}
	return m.refreshLocks[serverID]
}

func validateAuthorizationMetadata(metadata AuthorizationServerMetadata, issuerHost string) error {
	if metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	}
	for _, endpoint := range []string{metadata.AuthorizationEndpoint, metadata.TokenEndpoint, metadata.RegistrationEndpoint, metadata.RevocationEndpoint} {
		if endpoint == "" {
			continue
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), issuerHost) {
			return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: endpoint")
		}
	}
	found := false
	for _, method := range metadata.CodeChallengeMethodsSupported {
		if method == "S256" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED: PKCE S256 required")
	}
	return nil
}

func normalizedScopes(scopes []string) []string {
	unique := map[string]bool{}
	for _, scope := range scopes {
		if value := strings.TrimSpace(scope); value != "" {
			unique[value] = true
		}
	}
	result := make([]string, 0, len(unique))
	for scope := range unique {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result
}

func scopeSubset(requested, supported []string) bool {
	if len(requested) == 0 || len(supported) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, scope := range supported {
		set[scope] = true
	}
	for _, scope := range requested {
		if !set[scope] {
			return false
		}
	}
	return true
}

func sameOrigin(first, second string) bool {
	a, errA := url.Parse(first)
	b, errB := url.Parse(second)
	return errA == nil && errB == nil && strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func storedToken(token Token, tokenURL, revokeURL string, registration ClientRegistration) StoredToken {
	expires := ""
	if token.ExpiresIn > 0 {
		expires = time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second).Format(time.RFC3339Nano)
	}
	return StoredToken{AccessToken: token.AccessToken, TokenType: token.TokenType, RefreshToken: token.RefreshToken, Scopes: normalizedScopes(strings.Fields(token.Scope)), ExpiresAt: expires, TokenURL: tokenURL, RevokeURL: revokeURL, ClientID: registration.ClientID, ClientSecret: registration.ClientSecret}
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
func mustJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }
