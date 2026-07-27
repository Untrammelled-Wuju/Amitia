package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
)

func runVerify(args []string, output *Output) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	pubkeyFile := fs.String("pubkey", "", "ed25519 公钥文件路径（hex 编码，可选）")
	fs.Parse(args)

	if fs.NArg() < 1 {
		output.fail(ExitConfig, "用法: amitia-ext verify <package.amitiax> [--pubkey <pubkey-file>]")
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
			"extensionId": pkg.Manifest.Extension.ID,
			"version":     pkg.Manifest.Extension.Version,
			"treeHash":    pkg.Tree.TreeHash,
			"fileCount":   len(pkg.Integrity.Files),
			"signed":      pkg.Signatures != nil,
		},
	}

	if *pubkeyFile != "" {
		pubKey, err := loadPublicKey(*pubkeyFile)
		if err != nil {
			output.fail(ExitConfig, err.Error())
		}

		if pkg.Signatures == nil {
			output.fail(ExitSig, "包未签名，无法验证签名")
		}

		if err := amitiax.VerifySignature(pkg, pubKey); err != nil {
			output.fail(ExitSig, fmt.Sprintf("签名验证失败: %v", err))
		}

		data := result.Data.(map[string]any)
		data["signatureVerified"] = true
		data["keyId"] = pkg.Signatures.KeyID
		data["publisherId"] = pkg.Signatures.PublisherID
		data["algorithm"] = pkg.Signatures.Algorithm
		result.Message = fmt.Sprintf("验证通过（完整性+签名）: %s", archivePath)
	} else if pkg.Signatures != nil {
		result.Warnings = append(result.Warnings, "包已签名但未提供公钥，跳过签名验证")
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
