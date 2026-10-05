package secretstore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
)

var envelope = []byte("AMITIA-SECRET-V1\n")
var protect = portableProtect
var unprotect = portableUnprotect

func portableCipher() (cipher.AEAD, error) {
	key := os.Getenv("AMITIA_SECRET_KEY")
	if key == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("设备安全存储密钥无效")
	}
	block, err := aes.NewCipher(decoded)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func portableProtect(data []byte) ([]byte, error) {
	aead, err := portableCipher()
	if err != nil {
		return nil, err
	}
	if aead == nil {
		return append([]byte(nil), data...), nil
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	encoded := aead.Seal(nonce, nonce, data, envelope)
	return append(append([]byte(nil), envelope...), encoded...), nil
}

func portableUnprotect(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, envelope) {
		return append([]byte(nil), data...), nil
	}
	aead, err := portableCipher()
	if err != nil {
		return nil, err
	}
	if aead == nil {
		return nil, errors.New("缺少设备安全存储密钥，拒绝读取复制的设备凭证")
	}
	encoded := data[len(envelope):]
	if len(encoded) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("设备安全存储数据无效")
	}
	return aead.Open(nil, encoded[:aead.NonceSize()], encoded[aead.NonceSize():], envelope)
}

func Read(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoded, err := unprotect(data)
	if err != nil {
		return nil, errors.New("设备安全存储无法解密，请重新配对")
	}
	if !bytes.HasPrefix(data, envelope) {
		encoded, err := protect(decoded)
		if err != nil {
			return nil, err
		}
		if bytes.HasPrefix(encoded, envelope) {
			if err := writeEncoded(path, encoded); err != nil {
				return nil, err
			}
		}
	}
	return decoded, nil
}

func Write(path string, data []byte) error {
	encoded, err := protect(data)
	if err != nil {
		return err
	}
	return writeEncoded(path, encoded)
}

func writeEncoded(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".secret-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
