package secretstore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/u-ai/backend/internal/ioshostbridge"
	"os"
	"path/filepath"
	"strings"
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
	bridge, err := ioshostbridge.FromEnvironment()
	if err != nil {
		return nil, err
	}
	if bridge != nil {
		key, err := hostKey(path)
		if err != nil {
			return nil, err
		}
		var result struct {
			Found bool   `json:"found"`
			Data  string `json:"data"`
		}
		if err := bridge.Call("secret.get", map[string]string{"key": key}, &result); err != nil {
			return nil, err
		}
		if !result.Found {
			return nil, os.ErrNotExist
		}
		data, err := base64.StdEncoding.Strict().DecodeString(result.Data)
		if err != nil || len(data) > 1<<20 {
			return nil, errors.New("iOS 宿主安全存储数据无效")
		}
		return data, nil
	}
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
	bridge, err := ioshostbridge.FromEnvironment()
	if err != nil {
		return err
	}
	if bridge != nil {
		key, err := hostKey(path)
		if err != nil {
			return err
		}
		if len(data) > 1<<20 {
			return errors.New("iOS 宿主安全存储数据超过上限")
		}
		var result struct {
			OK bool `json:"ok"`
		}
		if err := bridge.Call("secret.set", map[string]string{"key": key, "data": base64.StdEncoding.EncodeToString(data)}, &result); err != nil {
			return err
		}
		if !result.OK {
			return errors.New("iOS 宿主安全存储未确认保存")
		}
		return nil
	}
	encoded, err := protect(data)
	if err != nil {
		return err
	}
	return writeEncoded(path, encoded)
}

func Delete(path string) error {
	bridge, err := ioshostbridge.FromEnvironment()
	if err != nil {
		return err
	}
	if bridge != nil {
		key, err := hostKey(path)
		if err != nil {
			return err
		}
		var result struct {
			OK bool `json:"ok"`
		}
		if err := bridge.Call("secret.delete", map[string]string{"key": key}, &result); err != nil {
			return err
		}
		if !result.OK {
			return errors.New("iOS 宿主安全存储未确认删除")
		}
		return nil
	}
	return os.Remove(path)
}

func hostKey(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("安全存储路径无效")
	}
	root := os.Getenv("AMITIA_DATA_DIR")
	if root == "" {
		root = os.Getenv("AMITIA_RUNTIME_ROOT")
	}
	if root == "" {
		return "", errors.New("缺少 iOS Runtime 数据目录")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", errors.New("iOS Runtime 数据目录无效")
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("安全存储路径超出 Runtime 数据目录")
	}
	digest := sha256.Sum256([]byte(filepath.ToSlash(relative)))
	return hex.EncodeToString(digest[:]), nil
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
