package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
)

type RevisionPublisher struct {
	DataDir string
}

func NewRevisionPublisher(dataDir string) *RevisionPublisher {
	return &RevisionPublisher{DataDir: dataDir}
}

type PublishRequest struct {
	WorkDir             *WorkDirectory
	GenerationTaskID    string
	RevisionID          string
	ProcessingTaskID    string
	ProcessingActionID  string
	ActionKey           string
	ProcessingVersion   int
	Manifest            *contracts.RevisionManifest
	Journal             *Journal
}

func (p *RevisionPublisher) Publish(req PublishRequest) (string, error) {
	if req.WorkDir == nil {
		return "", fmt.Errorf("artifact: publish: workDir is nil")
	}
	if req.GenerationTaskID == "" {
		return "", fmt.Errorf("artifact: publish: generationTaskID is empty")
	}
	if req.RevisionID == "" {
		return "", fmt.Errorf("artifact: publish: revisionID is empty")
	}
	if req.ProcessingVersion <= 0 {
		return "", fmt.Errorf("artifact: publish: processingVersion must be positive")
	}
	if req.Manifest == nil {
		return "", fmt.Errorf("artifact: publish: manifest is nil")
	}
	if req.Journal == nil {
		return "", fmt.Errorf("artifact: publish: journal is nil")
	}

	if !req.WorkDir.Exists() {
		return "", fmt.Errorf("artifact: publish: work directory does not exist: %s", req.WorkDir.RootPath)
	}

	if err := req.Journal.Load(); err != nil {
		return "", fmt.Errorf("artifact: publish: load journal: %w", err)
	}

	if !req.Journal.IsStageDone("validated") {
		return "", fmt.Errorf("artifact: publish: journal not at validated stage, last stage: %s",
			req.Journal.GetLastStage())
	}

	if req.Journal.IsStageDone("files_published") {
		return "", fmt.Errorf("artifact: publish: files already published for revision %s", req.RevisionID)
	}

	targetRoot := filepath.Join(
		p.DataDir,
		"desktop-pets",
		"generation-tasks",
		req.GenerationTaskID,
		"processed",
		"versions",
		fmt.Sprintf("%d", req.ProcessingVersion),
		"actions",
		req.ActionKey,
		"revisions",
		req.RevisionID,
	)

	targetDirs := []string{
		targetRoot,
		filepath.Join(targetRoot, "frames"),
		filepath.Join(targetRoot, "masks"),
		filepath.Join(targetRoot, "transforms"),
		filepath.Join(targetRoot, "measurements"),
	}
	for _, d := range targetDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return "", fmt.Errorf("artifact: publish: create target dir %s: %w", d, err)
		}
	}

	if err := copyWorkDirContents(req.WorkDir.RootPath, targetRoot); err != nil {
		return "", fmt.Errorf("artifact: publish: copy work dir: %w", err)
	}

	manifestPath := filepath.Join(targetRoot, "revision.json")
	manifestData, err := json.MarshalIndent(req.Manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("artifact: publish: marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return "", fmt.Errorf("artifact: publish: write manifest %s: %w", manifestPath, err)
	}

	if err := req.Journal.Record("files_published", "done",
		fmt.Sprintf("published to %s", targetRoot)); err != nil {
		return "", fmt.Errorf("artifact: publish: record journal: %w", err)
	}

	rootRelative := filepath.ToSlash(filepath.Join(
		"desktop-pets",
		"generation-tasks",
		req.GenerationTaskID,
		"processed",
		"versions",
		fmt.Sprintf("%d", req.ProcessingVersion),
		"actions",
		req.ActionKey,
		"revisions",
		req.RevisionID,
	))

	return rootRelative, nil
}

func copyWorkDirContents(srcRoot, dstRoot string) error {
	return filepath.Walk(srcRoot, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if srcPath == srcRoot {
			return nil
		}

		relPath, err := filepath.Rel(srcRoot, srcPath)
		if err != nil {
			return fmt.Errorf("compute relative path: %w", err)
		}
		dstPath := filepath.Join(dstRoot, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed in work directory: %s", srcPath)
		}

		srcHash, err := computeFileHash(srcPath)
		if err != nil {
			return fmt.Errorf("hash source %s: %w", srcPath, err)
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy %s -> %s: %w", srcPath, dstPath, err)
		}

		dstHash, err := computeFileHash(dstPath)
		if err != nil {
			return fmt.Errorf("hash destination %s: %w", dstPath, err)
		}

		if srcHash != dstHash {
			return fmt.Errorf("hash mismatch after copy: %s (%s) != %s (%s)",
				srcPath, srcHash, dstPath, dstHash)
		}

		return nil
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func isPathUnderBase(base, target string) bool {
	cleanBase := filepath.Clean(base)
	cleanTarget := filepath.Clean(target)
	if cleanTarget == cleanBase {
		return true
	}
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
