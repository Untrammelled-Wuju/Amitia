package proof_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func TestRequestProofRejectsCredentialCopyReplayAndModifiedRequest(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "requests.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if err := proof.VerifyRequest(t.Context(), db, "legacy-device", "core", httptest.NewRequest("GET", "https://core.test/api/messages", nil), time.Now()); !errors.Is(err, proof.ErrProof) {
		t.Fatal("未登记密钥的旧凭证绕过身份验证", err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey := base64.RawURLEncoding.EncodeToString(public)
	if _, err := db.Exec(`INSERT INTO kernel_device_identity_keys(device_id,public_key,created_at) VALUES('a',?,?)`, encodedKey, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	makeRequest := func() *http.Request {
		request := httptest.NewRequest("POST", "https://core.test/api/messages?q=1", strings.NewReader(`{"message":"hello"}`))
		request.Header.Set("Authorization", "AmitiaDevice secret")
		p := proof.NewRequest(encodedKey, "core", uuid.NewString(), request.Method, request.URL.RequestURI(), request.Header.Get("Authorization"), []byte(`{"message":"hello"}`), time.Now())
		p.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, p.SigningBytes()))
		request.Header.Set(proof.RequestHeader, proof.EncodeRequest(p))
		return request
	}
	request := makeRequest()
	if err := proof.VerifyRequest(t.Context(), db, "a", "core", request, time.Now()); err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(request.Body)
	if string(body) != `{"message":"hello"}` {
		t.Fatal("验证吞掉了业务请求正文")
	}
	request.Body = io.NopCloser(strings.NewReader(string(body)))
	if err := proof.VerifyRequest(t.Context(), db, "a", "core", request, time.Now()); !errors.Is(err, proof.ErrProof) {
		t.Fatal("重放请求未被拒绝", err)
	}
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) { r.Header.Del(proof.RequestHeader) },
		func(r *http.Request) { r.Header.Set("Authorization", "AmitiaDevice another-secret") },
		func(r *http.Request) { r.Method = "DELETE" },
		func(r *http.Request) { r.URL.RawQuery = "q=2" },
		func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(`{"message":"changed"}`)) },
	} {
		request := makeRequest()
		mutate(request)
		if err := proof.VerifyRequest(t.Context(), db, "a", "core", request, time.Now()); !errors.Is(err, proof.ErrProof) {
			t.Fatal("变更请求或裸凭证被接受", err)
		}
	}
	if err := proof.VerifyRequest(t.Context(), db, "a", "different-core", makeRequest(), time.Now()); !errors.Is(err, proof.ErrProof) {
		t.Fatal("请求跨越 Core", err)
	}
}
