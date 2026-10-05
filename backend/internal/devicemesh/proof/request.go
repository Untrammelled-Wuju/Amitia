package proof

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const RequestHeader = "X-Amitia-Device-Proof"

type RequestProof struct {
	PublicKey string `json:"publicKey"`
	Audience  string `json:"audience"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	BodyHash  string `json:"bodyHash"`
	TokenHash string `json:"tokenHash"`
	IssuedAt  int64  `json:"issuedAt"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

func HashBytes(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

func (p RequestProof) SigningBytes() []byte {
	p.Signature = ""
	encoded, _ := json.Marshal(p)
	return encoded
}

func NewRequest(public, audience, nonce, method, path, authorization string, body []byte, now time.Time) RequestProof {
	return RequestProof{PublicKey: public, Audience: audience, Method: method, Path: path, BodyHash: HashBytes(body), TokenHash: HashBytes([]byte(authorization)), IssuedAt: now.Unix(), Nonce: nonce}
}

func RequestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil || request.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, (64<<20)+1))
	_ = request.Body.Close()
	if err != nil || len(body) > 64<<20 {
		return nil, ErrProof
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func EncodeRequest(p RequestProof) string {
	encoded, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func VerifyRequest(ctx context.Context, db *sql.DB, device, core string, request *http.Request, now time.Time) error {
	var public string
	var revoked bool
	err := db.QueryRowContext(ctx, `SELECT public_key,revoked FROM kernel_device_identity_keys WHERE device_id=?`, device).Scan(&public, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrProof
	}
	if err != nil {
		return err
	}
	if revoked {
		return ErrIdentityCopy
	}
	header := request.Header.Get(RequestHeader)
	if len(header) == 0 || len(header) > 4096 {
		return ErrProof
	}
	encoded, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return ErrProof
	}
	var p RequestProof
	if err := json.Unmarshal(encoded, &p); err != nil {
		return ErrProof
	}
	body, err := RequestBody(request)
	if err != nil {
		return err
	}
	if p.PublicKey != public || p.Audience != core || p.Method != request.Method || p.Path != request.URL.RequestURI() || p.BodyHash != HashBytes(body) || p.TokenHash != HashBytes([]byte(strings.TrimSpace(request.Header.Get("Authorization")))) || len(p.Nonce) < 16 || len(p.Nonce) > 128 || p.IssuedAt < now.Add(-90*time.Second).Unix() || p.IssuedAt > now.Add(30*time.Second).Unix() {
		return ErrProof
	}
	key, err := base64.RawURLEncoding.DecodeString(public)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return ErrProof
	}
	signature, err := base64.RawURLEncoding.DecodeString(p.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(key), p.SigningBytes(), signature) {
		return ErrProof
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM kernel_device_identity_nonces WHERE expires_at<?`, now.Unix()); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_identity_nonces(public_key,nonce,expires_at) VALUES(?,?,?) ON CONFLICT DO NOTHING`, public, p.Nonce, now.Add(2*time.Minute).Unix())
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrProof
	}
	return tx.Commit()
}
