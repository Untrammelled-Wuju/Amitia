package extension

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

const encryptedConfigPrefix = "enc:v1:"

type configCipher struct {
	aead cipher.AEAD
}

func newConfigCipher(db *gorm.DB) (*configCipher, error) {
	key, err := configEncryptionKey(db)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &configCipher{aead: aead}, nil
}

func configEncryptionKey(db *gorm.DB) ([]byte, error) {
	if configured := strings.TrimSpace(os.Getenv("AMITIA_EXTENSION_CONFIG_KEY")); configured != "" {
		sum := sha256.Sum256([]byte(configured))
		return sum[:], nil
	}
	var databases []struct {
		Seq  int    `gorm:"column:seq"`
		Name string `gorm:"column:name"`
		File string `gorm:"column:file"`
	}
	if db != nil && db.Dialector.Name() == "sqlite" && db.Raw("PRAGMA database_list").Scan(&databases).Error == nil {
		for _, database := range databases {
			if database.Name == "main" && strings.TrimSpace(database.File) != "" {
				return loadOrCreateConfigKey(database.File + ".extension-key")
			}
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func loadOrCreateConfigKey(path string) ([]byte, error) {
	if raw, err := os.ReadFile(path); err == nil {
		key, decodeErr := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if decodeErr != nil || len(key) != 32 {
			return nil, fmt.Errorf("invalid extension config key")
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	encoded := []byte(base64.RawStdEncoding.EncodeToString(key))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return loadOrCreateConfigKey(path)
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(encoded); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

func (c *configCipher) encrypt(plain []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, plain, nil)
	payload := append(nonce, sealed...)
	return encryptedConfigPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *configCipher) decrypt(stored string) ([]byte, bool, error) {
	if !strings.HasPrefix(stored, encryptedConfigPrefix) {
		return []byte(stored), false, nil
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, encryptedConfigPrefix))
	if err != nil || len(payload) < c.aead.NonceSize() {
		return nil, true, fmt.Errorf("invalid encrypted skill configuration")
	}
	plain, err := c.aead.Open(nil, payload[:c.aead.NonceSize()], payload[c.aead.NonceSize():], nil)
	if err != nil {
		return nil, true, fmt.Errorf("decrypt skill configuration: %w", err)
	}
	return plain, true, nil
}
