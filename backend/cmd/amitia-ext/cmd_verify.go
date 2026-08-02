package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func runVerify(args []string, output *Output) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	pubkeyFile := fs.String("pubkey", "", "ed25519 公钥文件路径（hex 编码，用于 V2 签名验证）")
	publisherID := fs.String("publisher", "", "发布者 ID（用于 V2 签名验证）")
	keyID := fs.String("key-id", "", "密钥 ID（用于 V2 签名验证，留空则从公钥计算）")
	fs.Parse(args)

	if fs.NArg() < 1 {
		output.fail(ExitConfig, "用法: amitiax verify <package.amitiax> [--pubkey <pubkey-file> --publisher <publisher-id>]")
	}
	archivePath := fs.Arg(0)

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		output.fail(ExitFailure, fmt.Sprintf("打开包失败: %v", err))
	}

	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		output.fail(ExitFailure, fmt.Sprintf("完整性验证失败: %v", err))
	}

	result := Result{
		OK:      true,
		Message: fmt.Sprintf("完整性验证通过: %s", archivePath),
		Data: map[string]any{
			"extensionId":  pkg.Manifest.Extension.ID,
			"version":      pkg.Manifest.Extension.Version,
			"treeHash":     pkg.Tree.TreeHash,
			"fileCount":    len(pkg.Integrity.Files),
			"hasV2Sig":     len(pkg.V2Signature) > 0,
			"hasLegacySig": pkg.Signatures != nil,
		},
	}

	if len(pkg.V2Signature) > 0 && *pubkeyFile != "" {
		pubKey, err := loadPublicKey(*pubkeyFile)
		if err != nil {
			output.fail(ExitConfig, err.Error())
		}

		resolvedKeyID := *keyID
		if resolvedKeyID == "" {
			resolvedKeyID = trust.ComputeKeyID(pubKey)
		}
		resolvedPublisher := *publisherID
		if resolvedPublisher == "" {
			output.fail(ExitConfig, "V2 签名验证需要 --publisher 参数")
		}

		store := trust.NewPublisherStore()
		pubKeyEntry := trust.PublisherKey{
			KeyID:       resolvedKeyID,
			PublisherID: resolvedPublisher,
			PublicKey:   pubKey,
			Algorithm:   trust.AlgorithmEd25519,
			State:       trust.KeyStateActive,
		}
		_ = store.RegisterDevelopment(resolvedPublisher, pubKeyEntry)
		verifier := trust.NewSignatureVerifier(store)

		doc, err := trust.ParseSignatureDocument(pkg.V2Signature)
		if err != nil {
			output.fail(ExitSig, fmt.Sprintf("解析 V2 签名失败: %v", err))
		}

		manifestHash := computeManifestHashCLI(pkg)
		artifactHash := computeArtifactHashCLI(pkg)

		verResult := verifier.VerifyPackage(context.Background(), trust.PackageVerificationInput{
			Document:              doc,
			ActualExtensionID:     pkg.Manifest.Extension.ID,
			ActualVersion:         pkg.Manifest.Extension.Version,
			ActualManifestVersion: pkg.Manifest.ManifestVersion,
			ActualManifestHash:    manifestHash,
			ActualContentTreeHash: pkg.Tree.TreeHash,
			ActualArtifactHash:    artifactHash,
		})

		data := result.Data.(map[string]any)
		data["v2SignatureStatus"] = string(verResult.Status)
		data["v2KeyFingerprint"] = verResult.KeyFingerprint
		data["v2PayloadHash"] = doc.PayloadHash

		if !trust.IsSignatureValid(verResult) {
			output.fail(ExitSig, fmt.Sprintf("V2 签名验证失败: %s: %s", verResult.Status, verResult.Reason))
		}

		data["signatureVerified"] = true
		data["format"] = doc.Format
		data["algorithm"] = doc.Algorithm
		data["keyId"] = doc.KeyID
		data["publisherId"] = doc.PublisherID
		result.Message = fmt.Sprintf("验证通过（完整性+V2签名）: %s", archivePath)
	} else if len(pkg.V2Signature) > 0 {
		result.Warnings = append(result.Warnings, "包有 V2 签名但未提供公钥，跳过签名验证")
	} else if pkg.Signatures != nil && *pubkeyFile != "" {
		pubKey, err := loadPublicKey(*pubkeyFile)
		if err != nil {
			output.fail(ExitConfig, err.Error())
		}
		if err := amitiax.VerifySignature(pkg, pubKey); err != nil {
			output.fail(ExitSig, fmt.Sprintf("旧格式签名验证失败: %v", err))
		}
		data := result.Data.(map[string]any)
		data["signatureVerified"] = true
		data["signatureFormat"] = "legacy"
		data["keyId"] = pkg.Signatures.KeyID
		data["publisherId"] = pkg.Signatures.PublisherID
		data["algorithm"] = pkg.Signatures.Algorithm
		result.Message = fmt.Sprintf("验证通过（完整性+旧格式签名）: %s", archivePath)
		result.Warnings = append(result.Warnings, "检测到旧格式签名，建议使用 amitiax sign 重新签名为 amitiax-signature-v1 格式")
	} else if pkg.Signatures != nil {
		result.Warnings = append(result.Warnings, "包有旧格式签名但未提供公钥，跳过签名验证")
		result.Warnings = append(result.Warnings, "建议使用 amitiax sign 重新签名为 amitiax-signature-v1 格式")
	}

	output.emit(result)
	return ExitSuccess
}

func loadPublicKey(path string) (ed25519.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取公钥文件失败: %v", err)
	}
	keyHex := strings.TrimSpace(string(keyData))
	if strings.HasPrefix(keyHex, "0x") || strings.HasPrefix(keyHex, "0X") {
		keyHex = keyHex[2:]
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败（需要 hex 编码）: %v", err)
	}
	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("公钥长度错误: 期望 %d 字节, 得到 %d 字节", ed25519.PublicKeySize, len(keyBytes))
	}
	return ed25519.PublicKey(keyBytes), nil
}
