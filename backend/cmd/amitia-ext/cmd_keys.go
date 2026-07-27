package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/blake2b"
)

func runKeys(args []string, output *Output) int {
	if len(args) < 1 {
		output.fail(ExitConfig, "用法: amitia-ext keys <create|export-public> [选项]")
	}
	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "create":
		return keysCreate(rest, output)
	case "export-public":
		return keysExportPublic(rest, output)
	default:
		output.fail(ExitConfig, fmt.Sprintf("未知子命令: keys %s（支持: create, export-public）", subcmd))
	}
	return ExitSuccess
}

func keysCreate(args []string, output *Output) int {
	fs := flag.NewFlagSet("keys create", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	outDir := fs.String("output", "", "输出目录（可选，将生成 private.key 和 public.key）")
	keyID := fs.String("key-id", "", "密钥 ID（可选，默认自动生成）")
	fs.Parse(args)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("生成密钥失败: %v", err))
	}

	pubHex := hex.EncodeToString(pub)
	privHex := hex.EncodeToString(priv)

	kid := *keyID
	if kid == "" {
		kid = computeKeyID(pub)
	}

	data := map[string]any{
		"publicKey": pubHex,
		"keyId":     kid,
		"algorithm": "ed25519",
	}

	if *outDir != "" {
		absDir, err := filepath.Abs(*outDir)
		if err != nil {
			output.fail(ExitInternal, fmt.Sprintf("解析路径失败: %v", err))
		}
		if err := os.MkdirAll(absDir, 0o700); err != nil {
			output.fail(ExitEnv, fmt.Sprintf("创建目录失败: %v", err))
		}
		privPath := filepath.Join(absDir, "private.key")
		pubPath := filepath.Join(absDir, "public.key")
		if err := os.WriteFile(privPath, []byte(privHex), 0o600); err != nil {
			output.fail(ExitEnv, fmt.Sprintf("写入私钥失败: %v", err))
		}
		if err := os.WriteFile(pubPath, []byte(pubHex), 0o644); err != nil {
			output.fail(ExitEnv, fmt.Sprintf("写入公钥失败: %v", err))
		}
		data["privateKeyPath"] = privPath
		data["publicKeyPath"] = pubPath
		output.emit(Result{
			OK:      true,
			Message: fmt.Sprintf("密钥对已生成: %s", absDir),
			Data:    data,
		})
	} else {
		data["privateKey"] = privHex
		output.emit(Result{
			OK:       true,
			Message:  "密钥对已生成（请妥善保管私钥，不要将私钥提交到版本控制）",
			Data:     data,
			Warnings: []string{"私钥已显示在输出中，请妥善保存"},
		})
	}
	return ExitSuccess
}

func keysExportPublic(args []string, output *Output) int {
	fs := flag.NewFlagSet("keys export-public", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	keyFile := fs.String("key", "", "私钥文件路径（hex 编码）")
	outFile := fs.String("o", "", "输出文件路径（可选，默认输出到标准输出）")
	fs.Parse(args)

	if *keyFile == "" {
		output.fail(ExitConfig, "缺少 --key 参数")
	}

	privKey, err := loadPrivateKey(*keyFile)
	if err != nil {
		output.fail(ExitConfig, err.Error())
	}

	pubKey := privKey.Public().(ed25519.PublicKey)
	pubHex := hex.EncodeToString(pubKey)
	kid := computeKeyID(pubKey)

	data := map[string]any{
		"publicKey": pubHex,
		"keyId":     kid,
		"algorithm": "ed25519",
	}

	if *outFile != "" {
		if err := os.WriteFile(*outFile, []byte(pubHex), 0o644); err != nil {
			output.fail(ExitEnv, fmt.Sprintf("写入公钥失败: %v", err))
		}
		data["path"] = *outFile
		output.emit(Result{
			OK:      true,
			Message: fmt.Sprintf("公钥已导出: %s", *outFile),
			Data:    data,
		})
	} else {
		output.emit(Result{
			OK:      true,
			Message: "公钥已导出",
			Data:    data,
		})
	}
	return ExitSuccess
}

func computeKeyID(pubKey ed25519.PublicKey) string {
	h, err := blake2b.New(8, nil)
	if err != nil {
		return hex.EncodeToString(pubKey[:8])
	}
	h.Write(pubKey)
	return hex.EncodeToString(h.Sum(nil))
}
