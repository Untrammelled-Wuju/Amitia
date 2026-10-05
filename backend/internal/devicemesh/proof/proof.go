package proof

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const IdentitySchema = `CREATE TABLE IF NOT EXISTS kernel_device_identity_keys (
	device_id TEXT PRIMARY KEY, public_key TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0)`

const NonceSchema = `CREATE TABLE IF NOT EXISTS kernel_device_identity_nonces (
	public_key TEXT NOT NULL, nonce TEXT NOT NULL, expires_at INTEGER NOT NULL,
	PRIMARY KEY(public_key,nonce))`

var ErrProof = errors.New("设备身份签名无效或已重放，拒绝配对")
var ErrIdentityCopy = errors.New("设备身份密钥已被其他身份使用或已撤销，请重新配对")

type ClaimBody struct {
	DeviceID   string `json:"deviceId"`
	RuntimeID  string `json:"runtimeId"`
	Platform   string `json:"platform"`
	Label      string `json:"label"`
	OfferToken string `json:"offerToken"`
	SetupCode  string `json:"setupCode"`
}

type Proof struct {
	PublicKey string `json:"publicKey"`
	Audience  string `json:"audience"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	BodyHash  string `json:"bodyHash"`
	IssuedAt  int64  `json:"issuedAt"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

func BodyHash(body ClaimBody) string {
	encoded, _ := json.Marshal(body)
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func (p Proof) SigningBytes() []byte {
	p.Signature = ""
	encoded, _ := json.Marshal(p)
	return encoded
}

func New(publicKey, core, nonce string, body ClaimBody, now time.Time) Proof {
	return Proof{PublicKey: publicKey, Audience: core, Method: "POST", Path: "/api/public/device-mesh/v1/pairing/claim", BodyHash: BodyHash(body), IssuedAt: now.Unix(), Nonce: nonce}
}

func Verify(p Proof, core string, body ClaimBody, now time.Time) error {
	if p.Audience != core || p.Method != "POST" || p.Path != "/api/public/device-mesh/v1/pairing/claim" || p.BodyHash != BodyHash(body) || len(p.Nonce) < 16 || len(p.Nonce) > 128 || p.IssuedAt < now.Add(-90*time.Second).Unix() || p.IssuedAt > now.Add(30*time.Second).Unix() || body.DeviceID == "" || body.RuntimeID == "" {
		return ErrProof
	}
	public, err := base64.RawURLEncoding.DecodeString(p.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize {
		return ErrProof
	}
	signature, err := base64.RawURLEncoding.DecodeString(p.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(public), p.SigningBytes(), signature) {
		return ErrProof
	}
	return nil
}
