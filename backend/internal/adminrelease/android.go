// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/youmark/pkcs8"
)

type androidManifest struct {
	SchemaVersion           int                `json:"schemaVersion"`
	Product                 string             `json:"product"`
	PackageName             string             `json:"packageName"`
	Channel                 string             `json:"channel"`
	VersionCode             int64              `json:"versionCode"`
	VersionName             string             `json:"versionName"`
	MinSupportedVersionCode int64              `json:"minSupportedVersionCode"`
	Mandatory               bool               `json:"mandatory"`
	RolloutPercentage       int                `json:"rolloutPercentage"`
	PublishedAt             string             `json:"publishedAt"`
	APK                     androidManifestAPK `json:"apk"`
	ReleaseNotes            string             `json:"releaseNotes"`
}

type androidManifestAPK struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	ABI    string `json:"abi"`
}

func (s *Server) publishAndroidRelease(release *UpdateRelease) error {
	var artifacts []UpdateReleaseArtifact
	if err := s.db.Where("release_id = ?", release.ID).Find(&artifacts).Error; err != nil {
		return err
	}
	var apk *UpdateReleaseArtifact
	for index := range artifacts {
		if artifacts[index].Kind == "apk" {
			apk = &artifacts[index]
			break
		}
	}
	if apk == nil {
		return errors.New("缺少 APK")
	}
	manifestBytes, err := buildAndroidManifestBytes(s.cfg, release, *apk)
	if err != nil {
		return err
	}
	signature, err := s.signAndroidManifest(manifestBytes)
	if err != nil {
		return err
	}
	apkDir := filepath.Join(s.cfg.AndroidPublishDir, "releases", fmt.Sprintf("%d", release.VersionCode))
	if err := os.MkdirAll(apkDir, 0o755); err != nil {
		return err
	}
	apkTarget := filepath.Join(apkDir, apk.OriginalName)
	if err := copyFileAtomic(apk.StoredPath, apkTarget); err != nil {
		return err
	}
	signatureTarget := filepath.Join(s.cfg.AndroidPublishDir, release.Channel+".json.sig")
	if err := writeFileAtomic(signatureTarget, []byte(signature+"\n"), 0o644); err != nil {
		return err
	}
	manifestTarget := filepath.Join(s.cfg.AndroidPublishDir, release.Channel+".json")
	if err := writeFileAtomic(manifestTarget, manifestBytes, 0o644); err != nil {
		return err
	}
	release.PublishPath = apkTarget
	return nil
}

func buildAndroidManifestBytes(cfg Config, release *UpdateRelease, apk UpdateReleaseArtifact) ([]byte, error) {
	apkURL := fmt.Sprintf(
		"%s/android/releases/%d/%s",
		cfg.PublicBaseURL,
		release.VersionCode,
		apk.OriginalName,
	)
	manifest := androidManifest{
		SchemaVersion:           1,
		Product:                 ProductAndroid,
		PackageName:             cfg.AndroidPackageName,
		Channel:                 release.Channel,
		VersionCode:             release.VersionCode,
		VersionName:             release.Version,
		MinSupportedVersionCode: release.MinVersionCode,
		Mandatory:               release.Mandatory,
		RolloutPercentage:       release.RolloutPercentage,
		PublishedAt:             time.Now().UTC().Format(time.RFC3339),
		APK: androidManifestAPK{
			URL:    apkURL,
			Size:   apk.Size,
			SHA256: apk.SHA256,
			ABI:    release.ABI,
		},
		ReleaseNotes: release.ReleaseNotes,
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func (s *Server) signAndroidManifest(content []byte) (string, error) {
	if s.cfg.AndroidManifestKeyPath == "" {
		return "", errors.New("未配置手机端清单签名私钥")
	}
	keyBytes, err := os.ReadFile(s.cfg.AndroidManifestKeyPath)
	if err != nil {
		return "", fmt.Errorf("读取手机端清单签名私钥失败: %w", err)
	}
	privateKey, err := parseRSAPrivateKey(keyBytes, s.cfg.AndroidManifestKeyPassphrase)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func parseRSAPrivateKey(content []byte, passphrase string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, errors.New("签名私钥格式无效")
	}
	data := block.Bytes
	if block.Type == "ENCRYPTED PRIVATE KEY" {
		parsed, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, []byte(passphrase))
		if err != nil {
			return nil, errors.New("签名私钥口令无效")
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("签名私钥不是 RSA 密钥")
		}
		return key, nil
	}
	if x509.IsEncryptedPEMBlock(block) {
		decrypted, err := x509.DecryptPEMBlock(block, []byte(passphrase))
		if err != nil {
			return nil, errors.New("签名私钥口令无效")
		}
		data = decrypted
	}
	if key, err := x509.ParsePKCS1PrivateKey(data); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(data)
	if err != nil {
		return nil, errors.New("签名私钥格式无效")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("签名私钥不是 RSA 密钥")
	}
	return key, nil
}
