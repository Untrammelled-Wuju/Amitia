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

func runSign(args []string, output *Output) int {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	keyFile := fs.String("key", "", "ed25519 私钥文件路径（hex 编码）")
	keyID := fs.String("key-id", "", "密钥 ID")
	publisher := fs.String("publisher", "", "发布者 ID")
	fs.Parse(args)

	if fs.NArg() < 1 {
		output.fail(ExitConfig, "用法: amitia-ext sign <package.amitiax> --key <key-file> --key-id <key-id> --publisher <publisher-id>")
	}
	archivePath := fs.Arg(0)

	if *keyFile == "" {
		output.fail(ExitConfig, "缺少 --key 参数")
	}
	if *keyID == "" {
		output.fail(ExitConfig, "缺少 --key-id 参数")
	}
	if *publisher == "" {
		output.fail(ExitConfig, "缺少 --publisher 参数")
	}

	privKey, err := loadPrivateKey(*keyFile)
	if err != nil {
		output.fail(ExitConfig, err.Error())
	}

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		output.fail(ExitFailure, fmt.Sprintf("打开包失败: %v", err))
	}

	sig, err := amitiax.SignPackage(pkg, privKey, *keyID, *publisher)
	if err != nil {
		output.fail(ExitSig, fmt.Sprintf("签名失败: %v", err))
	}

	if err := amitiax.WriteSignatureToArchive(archivePath, sig); err != nil {
		output.fail(ExitFailure, fmt.Sprintf("写入签名失败: %v", err))
	}

	output.emit(Result{
		OK:      true,
		Message: fmt.Sprintf("签名成功: %s", archivePath),
		Data: map[string]any{
			"keyId":       sig.KeyID,
			"publisherId": sig.PublisherID,
			"algorithm":   sig.Algorithm,
			"signedAt":    sig.SignedAt,
			"treeHash":    pkg.Tree.TreeHash,
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
