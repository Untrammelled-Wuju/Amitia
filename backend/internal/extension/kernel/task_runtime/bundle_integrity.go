package task_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func TaskBundleHash(ctx context.Context, root string) (string, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", errors.New("任务插件文件树目录无效")
	}
	entries := make(map[string]string)
	var total int64
	var nodes int
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		nodes++
		if nodes > 16384 || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("任务插件文件树超限或包含符号链接")
		}
		if entry.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 {
			return errors.New("任务插件包含无效或超限文件")
		}
		total += info.Size()
		if total > 256<<20 {
			return errors.New("任务插件文件树超过大小限制")
		}
		hash := sha256.New()
		buffer := make([]byte, 64<<10)
		var count int64
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			n, readErr := file.Read(buffer)
			count += int64(n)
			if count > info.Size() {
				return errors.New("任务插件文件在校验时发生变化")
			}
			hash.Write(buffer[:n])
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
		if count != info.Size() {
			return errors.New("任务插件文件在校验时发生变化")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries[filepath.ToSlash(relative)] = hex.EncodeToString(hash.Sum(nil))
		return nil
	})
	if err != nil {
		return "", err
	}
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		hash.Write([]byte(path))
		hash.Write([]byte{0})
		hash.Write([]byte(entries[path]))
		hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
