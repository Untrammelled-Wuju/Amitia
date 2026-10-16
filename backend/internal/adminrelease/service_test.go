// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"archive/zip"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/youmark/pkcs8"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("StrongPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword("StrongPassword123!", hash) {
		t.Fatal("password should verify")
	}
	if verifyPassword("wrong-password", hash) {
		t.Fatal("wrong password should not verify")
	}
}

func TestBuildDesktopLatestYAML(t *testing.T) {
	release := &UpdateRelease{
		Version:      "26.2.0-beta.2",
		ReleaseName:  "Amitia 26.2.0-beta.2",
		ReleaseNotes: "修复更新流程",
	}
	artifact := UpdateReleaseArtifact{
		OriginalName: "AmitiaSetup-26.2.0-beta.2-x64.exe",
		Size:         1024,
		SHA512Base64: "sha512-base64",
	}
	content, err := buildDesktopLatestYAML(release, artifact)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "version: 26.2.0-beta.2") {
		t.Fatalf("latest.yml missing version: %s", text)
	}
	if !strings.Contains(text, "url: AmitiaSetup-26.2.0-beta.2-x64.exe") {
		t.Fatalf("latest.yml missing artifact url: %s", text)
	}
	if !strings.Contains(text, "sha512: sha512-base64") {
		t.Fatalf("latest.yml missing sha512: %s", text)
	}
}

func TestBuildAndroidManifest(t *testing.T) {
	cfg := Config{
		PublicBaseURL:      "https://amitia.untrammelled.top/amitia",
		AndroidPackageName: "com.amitia.amitia_app",
	}
	release := &UpdateRelease{
		Channel:           ChannelBeta,
		Version:           "26.2.0-beta.2",
		VersionCode:       4,
		MinVersionCode:    3,
		Mandatory:         true,
		RolloutPercentage: 20,
		ABI:               "arm64-v8a",
		ReleaseNotes:      "修复手机端更新",
	}
	apk := UpdateReleaseArtifact{
		OriginalName: "amitia-26.2.0-beta.2-release-arm64-v8a.apk",
		Size:         2048,
		SHA256:       strings.Repeat("a", 64),
	}
	content, err := buildAndroidManifestBytes(cfg, release, apk)
	if err != nil {
		t.Fatal(err)
	}
	var manifest androidManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.VersionCode != 4 || !manifest.Mandatory || manifest.RolloutPercentage != 20 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	expectedURL := "https://amitia.untrammelled.top/amitia/android/releases/4/amitia-26.2.0-beta.2-release-arm64-v8a.apk"
	if manifest.APK.URL != expectedURL {
		t.Fatalf("apk url = %q, want %q", manifest.APK.URL, expectedURL)
	}
}

func TestSignAndroidManifest(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "manifest-key.pem")
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(keyPath, keyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	server := &Server{cfg: Config{AndroidManifestKeyPath: keyPath}}
	content := []byte(`{"schemaVersion":1}`)
	signature, err := server.signAndroidManifest(content)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if err := rsa.VerifyPKCS1v15(&privateKey.PublicKey, crypto.SHA256, digest[:], decoded); err != nil {
		t.Fatal(err)
	}
}

func TestParseEncryptedPKCS8PrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := pkcs8.MarshalPrivateKey(privateKey, []byte("verify-passphrase"), nil)
	if err != nil {
		t.Fatal(err)
	}
	content := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: encrypted})
	parsed, err := parseRSAPrivateKey(content, "verify-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.N.Cmp(privateKey.N) != 0 {
		t.Fatal("parsed private key does not match")
	}
}

func TestValidateAPKArchive(t *testing.T) {
	apkPath := filepath.Join(t.TempDir(), "app-release.apk")
	file, err := os.Create(apkPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	manifest, err := writer.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.Write([]byte("manifest")); err != nil {
		t.Fatal(err)
	}
	library, err := writer.Create("lib/arm64-v8a/libapp.so")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := library.Write([]byte("library")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validateAPKArchive(apkPath); err != nil {
		t.Fatal(err)
	}
}

func TestValidateArtifactName(t *testing.T) {
	if err := validateArtifactName(ProductDesktop, "installer", "AmitiaSetup-26.2.0-beta.2-x64.exe"); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactName(ProductDesktop, "blockmap", "AmitiaSetup-26.2.0-beta.2-x64.exe.blockmap"); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactName(ProductAndroid, "apk", "amitia-release-arm64-v8a.apk"); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactName(ProductAndroid, "apk", "amitia-debug.apk"); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactName(ProductDesktop, "installer", "other.exe"); err == nil {
		t.Fatal("invalid desktop artifact name should fail")
	}
}

func TestSelectAndroidChannelRelease(t *testing.T) {
	candidates := []UpdateRelease{
		{ID: 1, Channel: ChannelStable, VersionCode: 6},
		{ID: 2, Channel: ChannelBeta, VersionCode: 7},
		{ID: 3, Channel: ChannelAlpha, VersionCode: 8},
	}
	if got := selectAndroidChannelRelease(ChannelStable, candidates); got == nil || got.ID != 1 {
		t.Fatalf("stable 通道应只选 stable 记录: %+v", got)
	}
	if got := selectAndroidChannelRelease(ChannelBeta, candidates); got == nil || got.ID != 2 {
		t.Fatalf("beta 通道应选 stable/beta 中版本号最高记录: %+v", got)
	}
	if got := selectAndroidChannelRelease(ChannelAlpha, candidates); got == nil || got.ID != 3 {
		t.Fatalf("alpha 通道应选三条通道中版本号最高记录: %+v", got)
	}
	fallback := []UpdateRelease{
		{ID: 4, Channel: ChannelStable, VersionCode: 9},
		{ID: 5, Channel: ChannelBeta, VersionCode: 3},
	}
	if got := selectAndroidChannelRelease(ChannelBeta, fallback); got == nil || got.ID != 4 {
		t.Fatalf("beta 通道应累进到更高版本号的 stable 记录: %+v", got)
	}
	if got := selectAndroidChannelRelease(ChannelStable, fallback); got == nil || got.ID != 4 {
		t.Fatalf("stable 通道不应选 beta 记录: %+v", got)
	}
	tie := []UpdateRelease{
		{ID: 6, Channel: ChannelStable, VersionCode: 10},
		{ID: 7, Channel: ChannelStable, VersionCode: 10},
	}
	if got := selectAndroidChannelRelease(ChannelStable, tie); got == nil || got.ID != 7 {
		t.Fatalf("版本号相同时应选较新记录: %+v", got)
	}
	onlyBeta := []UpdateRelease{{ID: 8, Channel: ChannelBeta, VersionCode: 11}}
	if got := selectAndroidChannelRelease(ChannelStable, onlyBeta); got != nil {
		t.Fatalf("stable 通道不应选 beta 记录: %+v", got)
	}
	if got := selectAndroidChannelRelease(ChannelAlpha, onlyBeta); got == nil || got.ID != 8 {
		t.Fatalf("alpha 通道应选 beta 记录: %+v", got)
	}
}
