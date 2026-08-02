package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
)

type archiveEntry struct {
	path string
	data []byte
}

func runPack(args []string, output *Output) int {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".", "扩展项目目录")
	outFile := fs.String("o", "", "输出文件路径（默认为 <扩展ID>-<版本>.amitiax）")
	fs.Parse(args)

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析路径失败: %v", err))
	}

	manifestPath := filepath.Join(absDir, "manifest.json")
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		output.fail(ExitConfig, fmt.Sprintf("读取 manifest.json 失败: %v", err))
	}

	var m manifest_v2.Manifest
	if err := json.Unmarshal(manifestRaw, &m); err != nil {
		output.fail(ExitConfig, fmt.Sprintf("解析 manifest.json 失败: %v", err))
	}

	m.Integrity.Algorithm = "sha256"
	m.Integrity.ContentTreeHash = ""
	m.Integrity.FileHashes = nil

	manifestData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("序列化 manifest 失败: %v", err))
	}

	var entries []archiveEntry
	entries = append(entries, archiveEntry{path: amitiax.ManifestFile, data: manifestData})

	collectDirs := []string{"modules", "resources", "assets", "migrations", "licenses", "docs"}
	for _, d := range collectDirs {
		dirPath := filepath.Join(absDir, d)
		if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
			continue
		}
		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(absDir, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if err := validateArchivePath(rel); err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entries = append(entries, archiveEntry{path: rel, data: data})
			return nil
		})
		if err != nil {
			output.fail(ExitFailure, fmt.Sprintf("收集文件失败: %v", err))
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	var fileEntries []amitiax.FileEntry
	filesMap := make(map[string]amitiax.FileEntry)
	for _, e := range entries {
		sum := sha256.Sum256(e.data)
		fe := amitiax.FileEntry{
			Path: e.path,
			Size: int64(len(e.data)),
			Hash: hex.EncodeToString(sum[:]),
		}
		fileEntries = append(fileEntries, fe)
		filesMap[e.path] = fe
	}

	treeHash := amitiax.ComputeTreeHash(fileEntries)

	integrityFiles := amitiax.IntegrityFilesDoc{
		Algorithm:   "sha256",
		Files:       filesMap,
		GeneratedAt: time.Time{},
	}
	integrityFilesData, err := json.MarshalIndent(integrityFiles, "", "  ")
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("序列化 integrity/files.json 失败: %v", err))
	}
	entries = append(entries, archiveEntry{path: amitiax.IntegrityFiles, data: integrityFilesData})

	integrityTree := amitiax.IntegrityTreeDoc{
		Algorithm:   "sha256",
		TreeHash:    treeHash,
		GeneratedAt: time.Time{},
	}
	integrityTreeData, err := json.MarshalIndent(integrityTree, "", "  ")
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("序列化 integrity/content-tree.json 失败: %v", err))
	}
	entries = append(entries, archiveEntry{path: amitiax.IntegrityTree, data: integrityTreeData})

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	if *outFile == "" {
		*outFile = fmt.Sprintf("%s-%s.amitiax", m.Extension.ID, m.Extension.Version)
	}
	outAbs, err := filepath.Abs(*outFile)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析输出路径失败: %v", err))
	}

	if err := createArchive(outAbs, entries); err != nil {
		output.fail(ExitFailure, fmt.Sprintf("创建归档失败: %v", err))
	}

	output.emit(Result{
		OK:      true,
		Message: fmt.Sprintf("打包成功: %s", outAbs),
		Data: map[string]any{
			"file":        outAbs,
			"treeHash":    treeHash,
			"fileCount":   len(fileEntries),
			"extensionId": m.Extension.ID,
			"version":     m.Extension.Version,
		},
	})
	return ExitSuccess
}

func validateArchivePath(p string) error {
	if strings.Contains(p, "..") {
		return fmt.Errorf("路径越界: %s", p)
	}
	if strings.Contains(p, "//") {
		return fmt.Errorf("无效路径（双斜杠）: %s", p)
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("无效路径（绝对路径）: %s", p)
	}
	return nil
}

func createArchive(outPath string, entries []archiveEntry) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, e := range entries {
		fh := &zip.FileHeader{
			Name:     e.path,
			Method:   zip.Deflate,
			Modified: time.Time{},
		}
		writer, err := w.CreateHeader(fh)
		if err != nil {
			return err
		}
		if _, err := writer.Write(e.data); err != nil {
			return err
		}
	}
	return w.Close()
}
