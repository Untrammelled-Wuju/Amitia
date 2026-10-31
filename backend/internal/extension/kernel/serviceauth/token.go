package serviceauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	amitialog "log"
	"sync"
)

const ContractVersion = "1"

var (
	masterOnce sync.Once
	masterKey  []byte
	masterErr  error
)

func processMasterKey() ([]byte, error) {
	masterOnce.Do(func() {
		masterKey = make([]byte, 32)
		if _, err := rand.Read(masterKey); err != nil {
			masterErr = fmt.Errorf("service auth: generate process master key: %w", err)
			masterKey = nil
		}
	})
	if masterErr != nil {
		return nil, masterErr
	}
	return masterKey, nil
}

// Token returns a process-lifetime bearer token scoped to one extension module.
// The token is intentionally not persisted. Host-side callers and the trusted
// service independently derive the same value during one host process lifetime.
func Token(extensionID, moduleID string) (string, error) {
	key, err := processMasterKey()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write([]byte("amitia-service-auth-v1\x00")); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(extensionID)); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte{0}); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(moduleID)); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
