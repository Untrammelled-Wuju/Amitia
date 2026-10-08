package task_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ResolveTaskEntry(ctx context.Context, bundleRoot string, definition *TaskDefinition) (string, error) {
	return resolveTaskEntry(ctx, bundleRoot, definition, false)
}

func PinTaskEntry(ctx context.Context, bundleRoot string, definition *TaskDefinition) error {
	if _, err := resolveTaskEntry(ctx, bundleRoot, definition, true); err != nil {
		return err
	}
	bundleHash, err := TaskBundleHash(ctx, bundleRoot)
	if err != nil {
		return err
	}
	if definition.BundleHash != "" && definition.BundleHash != bundleHash {
		return errors.New("任务插件文件树与安装声明不一致")
	}
	definition.BundleHash = bundleHash
	if definition.DefinitionHash == "" {
		fingerprint, err := taskDefinitionFingerprint(definition)
		if err != nil {
			return err
		}
		definition.DefinitionHash = fingerprint
	}
	return nil
}

func resolveTaskEntry(ctx context.Context, bundleRoot string, definition *TaskDefinition, pin bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if bundleRoot == "" || definition == nil || definition.Entry == "" || filepath.IsAbs(definition.Entry) || strings.ContainsRune(definition.Entry, '\x00') {
		return "", errors.New("任务入口参数无效")
	}
	root, err := filepath.EvalSymlinks(bundleRoot)
	if err != nil {
		return "", errors.New("已安装插件目录不可用")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	entry, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(definition.Entry)))
	if err != nil {
		return "", errors.New("已安装任务入口不可用")
	}
	relative, err := filepath.Rel(root, entry)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("任务入口超出已安装插件目录")
	}
	hash := strings.TrimPrefix(definition.EntryHash, "sha256:")
	if len(hash) != 64 && !(pin && definition.EntryHash == "") {
		return "", errors.New("任务入口缺少有效的源码完整性校验")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", errors.New("任务入口完整性校验无效")
	}
	file, err := os.Open(entry)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return "", errors.New("任务入口文件无效或超过限制")
	}
	digest := sha256.New()
	count, err := io.Copy(digest, io.LimitReader(file, 16<<20+1))
	if err != nil {
		return "", err
	}
	if count > 16<<20 {
		return "", errors.New("任务入口文件超过限制")
	}
	actualHash := hex.EncodeToString(digest.Sum(nil))
	if hash != "" && actualHash != strings.ToLower(hash) {
		return "", errors.New("任务入口源码已变化，请重新安装或确认插件版本")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if pin && definition.EntryHash == "" {
		definition.EntryHash = "sha256:" + actualHash
	}
	return entry, nil
}

func TaskBundleRoot(entry string, definition *TaskDefinition) (string, error) {
	if definition == nil || definition.BundleHash == "" {
		return "", nil
	}
	if !validTaskFingerprint(strings.TrimPrefix(definition.BundleHash, "sha256:")) {
		return "", errors.New("任务插件文件树摘要无效")
	}
	relative := filepath.Clean(filepath.FromSlash(definition.Entry))
	if filepath.IsAbs(relative) || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("任务插件入口路径无效")
	}
	root := entry
	for range strings.Split(relative, string(filepath.Separator)) {
		root = filepath.Dir(root)
	}
	actual, err := filepath.Abs(filepath.Join(root, relative))
	if err != nil || actual != entry {
		return "", errors.New("任务插件入口与固定文件树不一致")
	}
	return root, nil
}
