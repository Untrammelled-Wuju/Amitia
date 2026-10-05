package agent

import (
	"crypto"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io"
)

type identitySigner struct {
	store  *IdentityStore
	public ed25519.PublicKey
}

func (s *IdentityStore) CryptoSigner() (crypto.Signer, error) {
	identity, err := s.Load()
	if err != nil {
		return nil, err
	}
	public, err := base64.RawURLEncoding.DecodeString(identity.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize {
		return nil, errors.New("设备公钥无效")
	}
	return &identitySigner{store: s, public: ed25519.PublicKey(public)}, nil
}

func (s *identitySigner) Public() crypto.PublicKey { return s.public }

func (s *identitySigner) Sign(_ io.Reader, message []byte, options crypto.SignerOpts) ([]byte, error) {
	if options.HashFunc() != crypto.Hash(0) {
		return nil, errors.New("设备签名仅接受原始消息")
	}
	signature, err := s.store.Sign(message)
	if err != nil {
		return nil, err
	}
	return base64.RawURLEncoding.DecodeString(signature)
}
