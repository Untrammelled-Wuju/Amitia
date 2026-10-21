package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memorySecrets struct {
	mu     sync.Mutex
	next   int
	values map[string][]byte
}

func (s *memorySecrets) Put(_ context.Context, namespace string, value []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	reference := fmt.Sprintf("mcp-secret://%s/%d", namespace, s.next)
	s.values[reference] = append([]byte(nil), value...)
	return reference, nil
}
func (s *memorySecrets) Get(_ context.Context, reference string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[reference]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), value...), nil
}
func (s *memorySecrets) Delete(_ context.Context, reference string) error {
	s.mu.Lock()
	delete(s.values, reference)
	s.mu.Unlock()
	return nil
}

type memorySessions struct {
	mu      sync.Mutex
	pending map[string]PendingSession
	tokens  map[string]string
}

func (s *memorySessions) CreateOAuthSession(_ context.Context, session PendingSession) error {
	s.mu.Lock()
	s.pending[session.ID] = session
	s.mu.Unlock()
	return nil
}
func (s *memorySessions) ConsumeOAuthSession(_ context.Context, id, stateHash string) (PendingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.pending[id]
	if !ok || session.StateHash != stateHash {
		return PendingSession{}, fmt.Errorf("invalid")
	}
	return session, nil
}
func (s *memorySessions) SaveOAuthTokenReference(_ context.Context, serverID, reference string, _ time.Time, _ []string) error {
	s.mu.Lock()
	s.tokens[serverID] = reference
	s.mu.Unlock()
	return nil
}
func (s *memorySessions) OAuthTokenReference(_ context.Context, serverID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.tokens[serverID]
	if !ok {
		return "", fmt.Errorf("missing")
	}
	return value, nil
}
func (s *memorySessions) DeleteOAuthTokenReference(_ context.Context, serverID string) error {
	s.mu.Lock()
	delete(s.tokens, serverID)
	s.mu.Unlock()
	return nil
}
func (s *memorySessions) DeleteOAuthState(_ context.Context, id string) error {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
	return nil
}

func oauthTestManager(t *testing.T) (*Manager, *memorySecrets, *memorySessions, *atomic.Int32, *atomic.Int32, string) {
	t.Helper()
	var tokenCalls atomic.Int32
	var revokeCalls atomic.Int32
	var baseURL string
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/.well-known/oauth-protected-resource":
			_ = json.NewEncoder(response).Encode(map[string]any{"resource": baseURL + "/mcp", "authorization_servers": []string{baseURL}, "scopes_supported": []string{"read", "write"}})
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(response).Encode(map[string]any{"issuer": baseURL, "authorization_endpoint": baseURL + "/authorize", "token_endpoint": baseURL + "/token", "registration_endpoint": baseURL + "/register", "revocation_endpoint": baseURL + "/revoke", "scopes_supported": []string{"read", "write"}, "code_challenge_methods_supported": []string{"S256"}})
		case "/register":
			_ = json.NewEncoder(response).Encode(map[string]any{"client_id": "dynamic-client", "client_secret": "dynamic-secret"})
		case "/token":
			tokenCalls.Add(1)
			_ = request.ParseForm()
			if request.Form.Get("grant_type") == "refresh_token" {
				_ = json.NewEncoder(response).Encode(map[string]any{"access_token": "refreshed", "token_type": "Bearer", "expires_in": 3600, "refresh_token": "refresh-2", "scope": "read write"})
				return
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"access_token": "initial", "token_type": "Bearer", "expires_in": 3600, "refresh_token": "refresh-1", "scope": "read write"})
		case "/revoke":
			revokeCalls.Add(1)
			response.WriteHeader(http.StatusOK)
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	baseURL = server.URL
	t.Cleanup(server.Close)
	secrets := &memorySecrets{values: map[string][]byte{}}
	sessions := &memorySessions{pending: map[string]PendingSession{}, tokens: map[string]string{}}
	return NewManager(server.Client(), secrets, sessions), secrets, sessions, &tokenCalls, &revokeCalls, baseURL
}

func TestPKCEAndStateAreRandomAndVerifiable(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil || len(verifier) < 43 || len(challenge) < 43 {
		t.Fatalf("invalid PKCE verifier=%q challenge=%q err=%v", verifier, challenge, err)
	}
	state1, err := GenerateState()
	if err != nil {
		t.Fatal(err)
	}
	state2, err := GenerateState()
	if err != nil || state1 == state2 || HashState(state1) == state1 {
		t.Fatalf("invalid states %q %q", state1, state2)
	}
}

func TestOAuthDiscoveryCallbackRefreshConcurrencyAndRevoke(t *testing.T) {
	manager, secrets, sessions, tokenCalls, revokeCalls, baseURL := oauthTestManager(t)
	begin, err := manager.Begin(context.Background(), BeginRequest{ServerID: "server-1", ResourceURL: baseURL + "/mcp", RedirectURI: "http://127.0.0.1:18899/api/mcp/oauth/callback", Scopes: []string{"write", "read"}})
	if err != nil {
		t.Fatal(err)
	}
	if begin.AuthorizationURL == "" || begin.State == "" || begin.SessionID == "" {
		t.Fatalf("invalid begin result: %#v", begin)
	}
	if _, err := manager.Callback(context.Background(), begin.SessionID, "wrong-state", "code", "", "", ""); err == nil {
		t.Fatal("expected state validation error")
	}
	begin, err = manager.Begin(context.Background(), BeginRequest{ServerID: "server-1", ResourceURL: baseURL + "/mcp", RedirectURI: "http://127.0.0.1:18899/api/mcp/oauth/callback", Scopes: []string{"read", "write"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Callback(context.Background(), begin.SessionID, begin.State, "code", "", "", ""); err != nil {
		t.Fatal(err)
	}
	access, err := manager.AccessToken(context.Background(), "server-1")
	if err != nil || access != "initial" {
		t.Fatalf("unexpected access=%q err=%v", access, err)
	}
	reference, _ := sessions.OAuthTokenReference(context.Background(), "server-1")
	raw, _ := secrets.Get(context.Background(), reference)
	var stored StoredToken
	_ = json.Unmarshal(raw, &stored)
	stored.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	expiredReference, _ := secrets.Put(context.Background(), "server-1-oauth-token", mustJSON(stored))
	_ = sessions.SaveOAuthTokenReference(context.Background(), "server-1", expiredReference, time.Now().Add(-time.Minute), stored.Scopes)
	var wait sync.WaitGroup
	errors := make(chan error, 8)
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, callErr := manager.AccessToken(context.Background(), "server-1")
			if callErr != nil || value != "refreshed" {
				errors <- fmt.Errorf("value=%s err=%v", value, callErr)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for callErr := range errors {
		t.Error(callErr)
	}
	if tokenCalls.Load() != 2 {
		t.Fatalf("expected one code and one refresh exchange, got %d", tokenCalls.Load())
	}
	if err := manager.Revoke(context.Background(), "server-1"); err != nil {
		t.Fatal(err)
	}
	if revokeCalls.Load() != 1 {
		t.Fatalf("expected revoke call, got %d", revokeCalls.Load())
	}
	if _, err := sessions.OAuthTokenReference(context.Background(), "server-1"); err == nil {
		t.Fatal("token reference was not deleted")
	}
}

func TestOAuthRejectsUnsupportedScope(t *testing.T) {
	manager, _, _, _, _, baseURL := oauthTestManager(t)
	if _, err := manager.Begin(context.Background(), BeginRequest{ServerID: "server", ResourceURL: baseURL + "/mcp", RedirectURI: "http://127.0.0.1:18899/callback", Scopes: []string{"admin"}}); err == nil {
		t.Fatal("expected scope rejection")
	}
}
