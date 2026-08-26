package amitiax

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createTestArchive(t *testing.T, path string) {
	t.Helper()
	manifestJSON := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/test",
			"name": {"default": "Test"},
			"description": {"default": "Test extension"},
			"version": "1.0.0"
		},
		"publisher": {
			"id": "com.example",
			"displayName": "Example"
		},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"runtime": {"type": "javascript", "entryPoint": "index.js"},
				"contributions": [
					{"id": "test-tool", "kind": "tool", "name": {"default": "Test Tool"}}
				]
			}
		],
		"integrity": {
			"algorithm": "sha256",
			"contentTreeHash": ""
		}
	}`
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleDir := filepath.Join(dir, "modules", "main")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "index.js"), []byte("// test"), 0o644); err != nil {
		t.Fatal(err)
	}
	integrityDir := filepath.Join(dir, "integrity")
	if err := os.MkdirAll(integrityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"manifest.json":         manifestJSON,
		"modules/main/index.js": "// test",
	}
	filesDoc := IntegrityFilesDoc{
		Algorithm:   "sha256",
		Files:       make(map[string]FileEntry),
		GeneratedAt: time.Now().UTC(),
	}
	for name, content := range files {
		sum := sha256.Sum256([]byte(content))
		entry := FileEntry{
			Path: name,
			Size: int64(len(content)),
			Hash: hex.EncodeToString(sum[:]),
		}
		filesDoc.Files[name] = entry
	}
	filesData, _ := json.Marshal(filesDoc)
	if err := os.WriteFile(filepath.Join(integrityDir, "files.json"), filesData, 0o644); err != nil {
		t.Fatal(err)
	}
	var fileList []FileEntry
	for _, f := range filesDoc.Files {
		fileList = append(fileList, f)
	}
	treeHash := ComputeTreeHash(fileList)
	treeDoc := IntegrityTreeDoc{
		Algorithm:   "sha256",
		TreeHash:    treeHash,
		GeneratedAt: time.Now().UTC(),
	}
	treeData, _ := json.Marshal(treeDoc)
	if err := os.WriteFile(filepath.Join(integrityDir, "content-tree.json"), treeData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := zipDirectory(dir, path); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyIntegrityExcludesV2SignatureMetadata(t *testing.T) {
	manifest := FileEntry{Path: ManifestFile, Size: 2, Hash: hashBytes([]byte("{}"))}
	signature := FileEntry{Path: V2SignatureFile, Size: 2, Hash: hashBytes([]byte("{}"))}
	pkg := &Package{
		Files: []FileEntry{manifest, signature},
		Integrity: IntegrityFilesDoc{Files: map[string]FileEntry{
			ManifestFile: manifest,
		}},
		Tree: IntegrityTreeDoc{TreeHash: ComputeTreeHash([]FileEntry{manifest})},
	}
	if err := VerifyIntegrity(pkg); err != nil {
		t.Fatalf("VerifyIntegrity must exclude V2 signature metadata: %v", err)
	}
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func zipDirectory(srcDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	w := zip.NewWriter(zipFile)
	defer w.Close()
	return filepath.Walk(srcDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		writer, err := w.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	})
}

func TestOpenArchive(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.amitiax")
	createTestArchive(t, archivePath)
	pkg, err := OpenArchive(archivePath)
	if err != nil {
		t.Fatalf("OpenArchive: %v", err)
	}
	if pkg.Manifest.Extension.ID != "com.example/test" {
		t.Errorf("unexpected id: %s", pkg.Manifest.Extension.ID)
	}
	if pkg.Layout.ManifestPath != "manifest.json" {
		t.Errorf("expected manifest path, got %s", pkg.Layout.ManifestPath)
	}
	if len(pkg.Layout.Modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(pkg.Layout.Modules))
	}
	if pkg.Tree.TreeHash == "" {
		t.Fatal("expected canonical content tree hash")
	}
	if pkg.Manifest.Integrity.ContentTreeHash != pkg.Tree.TreeHash {
		t.Fatalf("manifest tree binding = %q, want %q", pkg.Manifest.Integrity.ContentTreeHash, pkg.Tree.TreeHash)
	}
}

func TestVerifyIntegrity(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.amitiax")
	createTestArchive(t, archivePath)
	pkg, err := OpenArchive(archivePath)
	if err != nil {
		t.Fatalf("OpenArchive: %v", err)
	}
	if err := VerifyIntegrity(pkg); err != nil {
		t.Errorf("VerifyIntegrity: %v", err)
	}
}

func TestInstall(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.amitiax")
	createTestArchive(t, archivePath)
	installer := NewInstaller()
	targetDir := filepath.Join(tmp, "install")
	result := installer.Install(nil, InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
	})
	if result.Status != InstallSucceeded {
		t.Fatalf("expected succeeded, got %s: %v", result.Status, result.Errors)
	}
	if result.Definition.ID != "com.example/test" {
		t.Errorf("unexpected id: %s", result.Definition.ID)
	}
	if len(result.InstalledFiles) == 0 {
		t.Errorf("expected installed files")
	}
}

func TestInstallInvalidArchive(t *testing.T) {
	installer := NewInstaller()
	result := installer.Install(nil, InstallRequest{
		ArchivePath: "nonexistent.amitiax",
		TargetDir:   t.TempDir(),
	})
	if result.Status != InstallFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
}

func TestInstallRollbackOnFailure(t *testing.T) {
	installer := NewInstaller()
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.amitiax")
	createTestArchive(t, archivePath)
	targetDir := filepath.Join(tmp, "install")
	result := installer.Install(nil, InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
		ExtensionID: "com.example/wrong",
	})
	if result.Status != InstallFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
}

func TestOpenArchiveAllowsCanonicalArtifactsRoot(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "artifact-package.amitiax")
	createTestArchive(t, archivePath)

	// Rebuild the fixture with one artifact file and matching integrity docs so
	// this exercises the canonical package parser rather than a relaxed helper.
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	staging := t.TempDir()
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			reader.Close()
			t.Fatal(openErr)
		}
		data, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			reader.Close()
			t.Fatal(readErr)
		}
		dest := filepath.Join(staging, filepath.FromSlash(f.Name))
		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			reader.Close()
			t.Fatal(mkErr)
		}
		if writeErr := os.WriteFile(dest, data, 0o644); writeErr != nil {
			reader.Close()
			t.Fatal(writeErr)
		}
	}
	reader.Close()

	artifactPath := filepath.Join(staging, "artifacts", "companion.bin")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("companion"), 0o644); err != nil {
		t.Fatal(err)
	}

	filesDocPath := filepath.Join(staging, "integrity", "files.json")
	var filesDoc IntegrityFilesDoc
	filesData, err := os.ReadFile(filesDocPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(filesData, &filesDoc); err != nil {
		t.Fatal(err)
	}
	artifactData := []byte("companion")
	artifactEntry := FileEntry{
		Path: "artifacts/companion.bin",
		Size: int64(len(artifactData)),
		Hash: hashBytes(artifactData),
	}
	filesDoc.Files[artifactEntry.Path] = artifactEntry
	filesData, _ = json.Marshal(filesDoc)
	if err := os.WriteFile(filesDocPath, filesData, 0o644); err != nil {
		t.Fatal(err)
	}
	entries := make([]FileEntry, 0, len(filesDoc.Files))
	for _, entry := range filesDoc.Files {
		entries = append(entries, entry)
	}
	treeDoc := IntegrityTreeDoc{Algorithm: "sha256", TreeHash: ComputeTreeHash(entries), GeneratedAt: time.Now().UTC()}
	treeData, _ := json.Marshal(treeDoc)
	if err := os.WriteFile(filepath.Join(staging, "integrity", "content-tree.json"), treeData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := zipDirectory(staging, archivePath); err != nil {
		t.Fatal(err)
	}

	pkg, err := OpenArchive(archivePath)
	if err != nil {
		t.Fatalf("OpenArchive with artifacts root: %v", err)
	}
	if err := VerifyIntegrity(pkg); err != nil {
		t.Fatalf("VerifyIntegrity with artifacts root: %v", err)
	}
	if len(pkg.Layout.Artifacts) != 1 || pkg.Layout.Artifacts[0] != "artifacts/companion.bin" {
		t.Fatalf("artifacts layout = %v", pkg.Layout.Artifacts)
	}
}
