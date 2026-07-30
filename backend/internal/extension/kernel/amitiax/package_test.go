package amitiax

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
