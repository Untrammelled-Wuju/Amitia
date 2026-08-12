package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestGeneratePKCE(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE failed: %v", err)
	}
	if verifier == "" {
		t.Fatal("verifier is empty")
	}
	if challenge == "" {
		t.Fatal("challenge is empty")
	}
	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	if challenge != expected {
		t.Fatalf("challenge mismatch: got %s, want %s", challenge, expected)
	}
}

func TestGeneratePKCE_Unique(t *testing.T) {
	verifiers := make(map[string]bool)
	challenges := make(map[string]bool)
	for i := 0; i < 100; i++ {
		v, c, err := GeneratePKCE()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if verifiers[v] {
			t.Fatalf("duplicate verifier at iteration %d", i)
		}
		if challenges[c] {
			t.Fatalf("duplicate challenge at iteration %d", i)
		}
		verifiers[v] = true
		challenges[c] = true
	}
}

func TestGenerateState(t *testing.T) {
	state, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState failed: %v", err)
	}
	if state == "" {
		t.Fatal("state is empty")
	}
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		t.Fatalf("state not valid base64: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(raw))
	}
}

func TestGenerateState_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := GenerateState()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[s] {
			t.Fatalf("duplicate state at iteration %d", i)
		}
		seen[s] = true
	}
}

func TestHashState(t *testing.T) {
	state := "test-state-value"
	hash1 := HashState(state)
	hash2 := HashState(state)
	if hash1 != hash2 {
		t.Fatal("HashState not deterministic")
	}
	if hash1 == state {
		t.Fatal("hash should not equal original state")
	}
}

func TestVerifyStateHash(t *testing.T) {
	state := "test-state-for-verification"
	hash := HashState(state)
	if !VerifyStateHash(state, hash) {
		t.Fatal("VerifyStateHash should return true for valid state")
	}
	if VerifyStateHash("wrong-state", hash) {
		t.Fatal("VerifyStateHash should return false for invalid state")
	}
	if VerifyStateHash(state, "wrong-hash") {
		t.Fatal("VerifyStateHash should return false for wrong hash")
	}
}

func TestNormalizeScopes(t *testing.T) {
	scopes := []string{"read", "write", "read", "", "  admin  ", "write"}
	result := NormalizeScopes(scopes)
	expected := []string{"admin", "read", "write"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d scopes, got %d", len(expected), len(result))
	}
	for i, s := range expected {
		if result[i] != s {
			t.Fatalf("index %d: expected %s, got %s", i, s, result[i])
		}
	}
}

func TestNormalizeScopes_Empty(t *testing.T) {
	result := NormalizeScopes([]string{})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestNormalizeScopes_AllBlank(t *testing.T) {
	result := NormalizeScopes([]string{"", "  ", " "})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestParseWWWAuthenticate_ResourceMetadata(t *testing.T) {
	header := `Bearer resource_metadata="https://api.example.com/.well-known/oauth-protected-resource", scope="read write", error="insufficient_scope", issuer="https://auth.example.com"`
	challenge := ParseWWWAuthenticate(header)
	if challenge.ResourceMetadata != "https://api.example.com/.well-known/oauth-protected-resource" {
		t.Fatalf("resource_metadata mismatch: %s", challenge.ResourceMetadata)
	}
	if challenge.Scope != "read write" {
		t.Fatalf("scope mismatch: %s", challenge.Scope)
	}
	if challenge.Error != "insufficient_scope" {
		t.Fatalf("error mismatch: %s", challenge.Error)
	}
	if challenge.Issuer != "https://auth.example.com" {
		t.Fatalf("issuer mismatch: %s", challenge.Issuer)
	}
}

func TestParseWWWAuthenticate_Empty(t *testing.T) {
	challenge := ParseWWWAuthenticate("")
	if challenge.ResourceMetadata != "" || challenge.Scope != "" {
		t.Fatal("expected empty challenge for empty header")
	}
}

func TestParseWWWAuthenticate_NoBearer(t *testing.T) {
	header := `Basic realm="test"`
	challenge := ParseWWWAuthenticate(header)
	if challenge.ResourceMetadata != "" {
		t.Fatal("expected empty challenge for non-bearer header")
	}
}

func TestParseWWWAuthenticate_ErrorDescription(t *testing.T) {
	header := `Bearer error="invalid_token", error_description="The access token has expired"`
	challenge := ParseWWWAuthenticate(header)
	if challenge.Error != "invalid_token" {
		t.Fatalf("error mismatch: %s", challenge.Error)
	}
	if challenge.ErrorDescription != "The access token has expired" {
		t.Fatalf("error_description mismatch: %s", challenge.ErrorDescription)
	}
}

func TestPKCE_S256_NotPlain(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		t.Fatal(err)
	}
	if verifier == challenge {
		t.Fatal("S256 challenge should not equal verifier")
	}
	if strings.Contains(challenge, "=") {
		t.Fatal("challenge should not contain padding")
	}
}
