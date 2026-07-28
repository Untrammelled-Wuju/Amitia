package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func runSign(args []string, output *Output) int {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	keyFile := fs.String("key", "", "ed25519 私钥文件路径（hex 编码）")
	keyID := fs.String("key-id", "", "密钥 ID（留空则从公钥自动计算 sha256 指纹）")
	publisher := fs.String("publisher", "", "发布者 ID")
	channel := fs.String("channel", "", "发布通道")
	fs.Parse(args)

	if fs.NArg() < 1 {
		output.fail(ExitConfig, "用法: amitiax sign <package.amitiax> --key <key-file> --publisher <publisher-id> [--key-id <key-id>] [--channel <channel>]")
	}
	archivePath := fs.Arg(0)

	if *keyFile == "" {
		output.fail(ExitConfig, "缺少 --key 参数")
	}
	if *publisher == "" {
		output.fail(ExitConfig, "缺少 --publisher 参数")
	}

	privKey, err := loadPrivateKey(*keyFile)
	if err != nil {
		output.fail(ExitConfig, err.Error())
	}

	resolvedKeyID := *keyID
	if resolvedKeyID == "" {
		pubKey := privKey.Public().(ed25519.PublicKey)
		resolvedKeyID = trust.ComputeKeyID(pubKey)
	}

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		output.fail(ExitFailure, fmt.Sprintf("打开包失败: %v", err))
	}

	manifestHash := computeManifestHashCLI(pkg)
	artifactHash := computeArtifactHashCLI(pkg)

	signer := trust.NewSigner(*publisher, resolvedKeyID, privKey)
	packageSigner := trust.NewPackageSigner(signer)

	doc, payload, err := packageSigner.SignPackage(trust.PackageSignatureInput{
		ExtensionID:       pkg.Manifest.Extension.ID,
		Version:           pkg.Manifest.Extension.Version,
		ManifestVersion:   pkg.Manifest.ManifestVersion,
		ManifestHash:      manifestHash,
		ContentTreeHash:   pkg.Tree.TreeHash,
		ArtifactHash:      artifactHash,
		Channel:           *channel,
	})
	if err != nil {
		output.fail(ExitSig, fmt.Sprintf("签名失败: %v", err))
	}

	sigData, err := trust.SerializeSignatureDocument(doc)
	if err != nil {
		output.fail(ExitSig, fmt.Sprintf("序列化签名失败: %v", err))
	}

	if err := amitiax.WriteV2SignatureToArchive(archivePath, sigData); err != nil {
		output.fail(ExitFailure, fmt.Sprintf("写入签名失败: %v", err))
	}

	output.emit(Result{
		OK:      true,
		Message: fmt.Sprintf("签名成功 (amitiax-signature-v1): %s", archivePath),
		Data: map[string]any{
			"format":         doc.Format,
			"algorithm":      doc.Algorithm,
			"keyId":          doc.KeyID,
			"publisherId":    doc.PublisherID,
			"payloadHash":    doc.PayloadHash,
			"createdAt":      doc.CreatedAt,
			"channel":        doc.Channel,
			"extensionId":    payload.ExtensionID,
			"version":        payload.Version,
			"manifestHash":   payload.ManifestHash,
			"contentTreeHash": payload.ContentTreeHash,
			"artifactHash":   payload.PackageHash,
		},
	})
	return ExitSuccess
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %v", err)
	}
	keyHex := strings.TrimSpace(string(keyData))
	if strings.HasPrefix(keyHex, "0x") || strings.HasPrefix(keyHex, "0X") {
		keyHex = keyHex[2:]
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败（需要 hex 编码）: %v", err)
	}
	if len(keyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("私钥长度错误: 期望 %d 字节, 得到 %d 字节", ed25519.PrivateKeySize, len(keyBytes))
	}
	return ed25519.PrivateKey(keyBytes), nil
}

func computeManifestHashCLI(pkg *amitiax.Package) string {
	if entry, ok := pkg.Integrity.Files[amitiax.ManifestFile]; ok {
		if strings.HasPrefix(entry.Hash, "sha256:") {
			return entry.Hash
		}
		return "sha256:" + entry.Hash
	}
	return ""
}

func computeArtifactHashCLI(pkg *amitiax.Package) string {
	entries := make([]trust.ArtifactEntry, 0, len(pkg.Integrity.Files))
	for path, entry := range pkg.Integrity.Files {
		hash := entry.Hash
		if !strings.HasPrefix(hash, "sha256:") {
			hash = "sha256:" + hash
		}
		entries = append(entries, trust.ArtifactEntry{
			Path:   path,
			Size:   entry.Size,
			SHA256: hash,
		})
	}
	return trust.ComputeCanonicalArtifactHash(entries)
}
