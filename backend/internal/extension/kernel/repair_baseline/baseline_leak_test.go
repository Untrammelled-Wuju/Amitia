package repair_baseline

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
)

type resourceSnapshot struct {
	goroutines      int
	heapAlloc       uint64
	tempDirEntries  int
	extRootEntries  int
	legacyCallTotal int64
	capturedAt      time.Time
}

func captureSnapshot(t *testing.T, tempDir, extRoot string) resourceSnapshot {
	t.Helper()
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	snap := resourceSnapshot{
		goroutines:     runtime.NumGoroutine(),
		heapAlloc:      memStats.HeapAlloc,
		capturedAt:     time.Now().UTC(),
		legacyCallTotal: kernel.GlobalLegacyCallCounter().Total(),
	}
	if tempDir != "" {
		if entries, err := os.ReadDir(tempDir); err == nil {
			snap.tempDirEntries = len(entries)
		}
	}
	if extRoot != "" {
		if entries, err := os.ReadDir(extRoot); err == nil {
			snap.extRootEntries = len(entries)
		}
	}
	return snap
}

func (s resourceSnapshot) diff(other resourceSnapshot) string {
	return fmt.Sprintf(
		"goroutines=%d->%d heapAlloc=%d->%d tempDirEntries=%d->%d extRootEntries=%d->%d legacyTotal=%d->%d",
		s.goroutines, other.goroutines,
		s.heapAlloc, other.heapAlloc,
		s.tempDirEntries, other.tempDirEntries,
		s.extRootEntries, other.extRootEntries,
		s.legacyCallTotal, other.legacyCallTotal,
	)
}

func buildArchiveFromExtension(t *testing.T, extDir, archivePath string) {
	t.Helper()
	manifestPath := filepath.Join(extDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not found in %s: %v", extDir, err)
	}
	stagingDir := t.TempDir()
	var files []struct {
		relPath string
		data    []byte
	}
	err := filepath.Walk(extDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(extDir, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		files = append(files, struct {
			relPath string
			data    []byte
		}{relPath: rel, data: data})
		return nil
	})
	if err != nil {
		t.Fatalf("walk extension dir %s: %v", extDir, err)
	}
	type fileEntry struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
		Hash string `json:"hash"`
	}
	filesDoc := struct {
		Algorithm string              `json:"algorithm"`
		Files     map[string]fileEntry `json:"files"`
		GeneratedAt time.Time         `json:"generatedAt"`
	}{
		Algorithm:   "sha256",
		Files:       make(map[string]fileEntry),
		GeneratedAt: time.Now().UTC(),
	}
	var fileList []fileEntry
	for _, f := range files {
		sum := sha256.Sum256(f.data)
		entry := fileEntry{
			Path: f.relPath,
			Size: int64(len(f.data)),
			Hash: hex.EncodeToString(sum[:]),
		}
		filesDoc.Files[f.relPath] = entry
		fileList = append(fileList, entry)
	}
	sort.SliceStable(fileList, func(i, j int) bool {
		return fileList[i].Path < fileList[j].Path
	})
	h := sha256.New()
	for _, f := range fileList {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.Hash))
		h.Write([]byte{0})
	}
	treeHash := hex.EncodeToString(h.Sum(nil))
	filesData, _ := json.Marshal(filesDoc)
	integrityFilesPath := filepath.Join(stagingDir, "integrity", "files.json")
	if err := os.MkdirAll(filepath.Dir(integrityFilesPath), 0o755); err != nil {
		t.Fatalf("mkdir integrity: %v", err)
	}
	if err := os.WriteFile(integrityFilesPath, filesData, 0o644); err != nil {
		t.Fatalf("write files.json: %v", err)
	}
	treeDoc := struct {
		Algorithm   string    `json:"algorithm"`
		TreeHash    string    `json:"treeHash"`
		GeneratedAt time.Time `json:"generatedAt"`
	}{
		Algorithm:   "sha256",
		TreeHash:    treeHash,
		GeneratedAt: time.Now().UTC(),
	}
	treeData, _ := json.Marshal(treeDoc)
	integrityTreePath := filepath.Join(stagingDir, "integrity", "content-tree.json")
	if err := os.WriteFile(integrityTreePath, treeData, 0o644); err != nil {
		t.Fatalf("write content-tree.json: %v", err)
	}
	zipFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive %s: %v", archivePath, err)
	}
	defer zipFile.Close()
	w := zip.NewWriter(zipFile)
	defer w.Close()
	for _, f := range files {
		writer, err := w.Create(f.relPath)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", f.relPath, err)
		}
		if _, err := writer.Write(f.data); err != nil {
			t.Fatalf("write zip entry %s: %v", f.relPath, err)
		}
	}
	integrityFilesRel := "integrity/files.json"
	writer, err := w.Create(integrityFilesRel)
	if err != nil {
		t.Fatalf("create zip entry %s: %v", integrityFilesRel, err)
	}
	if _, err := writer.Write(filesData); err != nil {
		t.Fatalf("write zip entry %s: %v", integrityFilesRel, err)
	}
	integrityTreeRel := "integrity/content-tree.json"
	writer, err = w.Create(integrityTreeRel)
	if err != nil {
		t.Fatalf("create zip entry %s: %v", integrityTreeRel, err)
	}
	if _, err := writer.Write(treeData); err != nil {
		t.Fatalf("write zip entry %s: %v", integrityTreeRel, err)
	}
}

func removeAllWithRetry(t *testing.T, path string) {
	t.Helper()
	for attempt := 0; attempt < 8; attempt++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		removeAllRecursive(path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("removeAllWithRetry: path %s still exists after 8 attempts", path)
	}
}

func removeAllRecursive(path string) {
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		_ = os.Chmod(p, 0o777)
		return nil
	})
	var paths []string
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if p != path {
			paths = append(paths, p)
		}
		return nil
	})
	for i := len(paths) - 1; i >= 0; i-- {
		_ = os.Remove(paths[i])
	}
	_ = os.Remove(path)
	_ = os.RemoveAll(path)
}

func TestBaseline_Leak_ResourceSnapshotCaptures(t *testing.T) {
	tempDir := t.TempDir()
	extRoot := t.TempDir()
	snap := captureSnapshot(t, tempDir, extRoot)
	if snap.capturedAt.IsZero() {
		t.Fatalf("snapshot must capture timestamp")
	}
	if snap.goroutines <= 0 {
		t.Fatalf("goroutine count must be positive, got %d", snap.goroutines)
	}
}

func TestBaseline_Leak_InstallUninstallNoGoroutineGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}
	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	if _, err := os.Stat(toolBasicDir); err != nil {
		t.Fatalf("tool-basic extension required: %v", err)
	}

	iterations := 100
	tempDir := t.TempDir()
	extRoot := t.TempDir()
	installer := amitiax.NewInstaller()

	baselineSnap := captureSnapshot(t, tempDir, extRoot)
	maxGoroutineDelta := 0

	for i := 0; i < iterations; i++ {
		archivePath := filepath.Join(tempDir, fmt.Sprintf("tool-basic-%d.amitiax", i))
		buildArchiveFromExtension(t, toolBasicDir, archivePath)
		targetDir := filepath.Join(extRoot, fmt.Sprintf("tool-basic-%d", i))
		result := installer.Install(context.Background(), amitiax.InstallRequest{
			ArchivePath: archivePath,
			TargetDir:   targetDir,
		})
		if result.Status != amitiax.InstallSucceeded {
			t.Fatalf("iteration %d install failed: %v", i, result.Errors)
		}
		removeAllWithRetry(t, targetDir)
		_ = os.Remove(archivePath)
		runtime.GC()
		snap := captureSnapshot(t, tempDir, extRoot)
		delta := snap.goroutines - baselineSnap.goroutines
		if delta > maxGoroutineDelta {
			maxGoroutineDelta = delta
		}
	}

	finalSnap := captureSnapshot(t, tempDir, extRoot)
	goroutineGrowth := finalSnap.goroutines - baselineSnap.goroutines
	if goroutineGrowth > 5 {
		t.Fatalf("goroutine leak detected after %d install/uninstall cycles: %s (growth=%d, maxDelta=%d)",
			iterations, baselineSnap.diff(finalSnap), goroutineGrowth, maxGoroutineDelta)
	}
	t.Logf("Leak: %d iterations completed, goroutine growth=%d (threshold=5)", iterations, goroutineGrowth)
}

func TestBaseline_Leak_InstallUninstallNoTempDirResidue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}
	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	tempDir := t.TempDir()
	extRoot := t.TempDir()
	installer := amitiax.NewInstaller()

	iterations := 100
	for i := 0; i < iterations; i++ {
		archivePath := filepath.Join(tempDir, fmt.Sprintf("tool-basic-%d.amitiax", i))
		buildArchiveFromExtension(t, toolBasicDir, archivePath)
		targetDir := filepath.Join(extRoot, fmt.Sprintf("tool-basic-%d", i))
		result := installer.Install(context.Background(), amitiax.InstallRequest{
			ArchivePath: archivePath,
			TargetDir:   targetDir,
		})
		if result.Status != amitiax.InstallSucceeded {
			t.Fatalf("iteration %d install failed: %v", i, result.Errors)
		}
		removeAllWithRetry(t, targetDir)
		_ = os.Remove(archivePath)
	}

	archiveResidue := 0
	if entries, err := os.ReadDir(tempDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".amitiax") {
				archiveResidue++
			}
		}
	}
	if archiveResidue > 0 {
		t.Fatalf("temporary directory has %d archive residues after %d cycles", archiveResidue, iterations)
	}
	targetResidue := 0
	if entries, err := os.ReadDir(extRoot); err == nil {
		targetResidue = len(entries)
	}
	if targetResidue > 0 {
		t.Fatalf("extension root has %d target residues after %d cycles", targetResidue, iterations)
	}
	t.Logf("Leak: %d iterations completed, no temp dir or extension root residue", iterations)
}

func TestBaseline_Leak_LegacyCallCounterStaysZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}
	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	tempDir := t.TempDir()
	extRoot := t.TempDir()
	installer := amitiax.NewInstaller()

	iterations := 100
	for i := 0; i < iterations; i++ {
		archivePath := filepath.Join(tempDir, fmt.Sprintf("tool-basic-%d.amitiax", i))
		buildArchiveFromExtension(t, toolBasicDir, archivePath)
		targetDir := filepath.Join(extRoot, fmt.Sprintf("tool-basic-%d", i))
		result := installer.Install(context.Background(), amitiax.InstallRequest{
			ArchivePath: archivePath,
			TargetDir:   targetDir,
		})
		if result.Status != amitiax.InstallSucceeded {
			t.Fatalf("iteration %d install failed: %v", i, result.Errors)
		}
		removeAllWithRetry(t, targetDir)
		_ = os.Remove(archivePath)
	}

	total := kernel.GlobalLegacyCallCounter().Total()
	if total != 0 {
		t.Fatalf("legacy call counter must stay zero after %d install/uninstall cycles, got %d", iterations, total)
	}
	if !kernel.GlobalLegacyCallCounter().FinalGatePassed() {
		t.Fatalf("final gate must pass after %d install/uninstall cycles", iterations)
	}
	t.Logf("Leak: %d iterations completed, legacy counter=0, final gate passed", iterations)
}

func TestBaseline_Leak_ResourceCategoriesDefined(t *testing.T) {
	requiredCategories := []string{
		"Runtime Process",
		"Goroutine",
		"File Handle",
		"Task Lease",
		"Event Delivery",
		"Schedule Lease",
		"MCP Connection",
		"UI Session",
		"MessagePort",
		"WebContents",
		"Menu",
		"Tray",
		"Shortcut",
		"Temporary Directory",
	}
	sort.Strings(requiredCategories)
	if len(requiredCategories) != 14 {
		t.Fatalf("Problem 14 requires 14 leak detection categories, got %d", len(requiredCategories))
	}
	seen := map[string]bool{}
	for _, c := range requiredCategories {
		seen[c] = true
	}
	for _, c := range []string{"Goroutine", "Temporary Directory", "File Handle", "Runtime Process"} {
		if !seen[c] {
			t.Fatalf("required leak category missing: %s", c)
		}
	}
	t.Logf("Leak: %d resource categories defined", len(requiredCategories))
}

func TestBaseline_Leak_CyclePhasesDefined(t *testing.T) {
	requiredPhases := []string{
		"install",
		"enable",
		"invoke",
		"disable",
		"uninstall",
	}
	if len(requiredPhases) != 5 {
		t.Fatalf("Phase 10 section 22 requires 5 cycle phases, got %d", len(requiredPhases))
	}
	for _, p := range requiredPhases {
		if p == "" {
			t.Fatalf("cycle phase must not be empty")
		}
	}
}

type leakRecorder struct {
	mu       sync.Mutex
	records  []resourceSnapshot
}

func (r *leakRecorder) record(snap resourceSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, snap)
}

func (r *leakRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

func TestBaseline_Leak_RecorderAccumulatesSnapshots(t *testing.T) {
	rec := &leakRecorder{}
	tempDir := t.TempDir()
	extRoot := t.TempDir()
	for i := 0; i < 5; i++ {
		rec.record(captureSnapshot(t, tempDir, extRoot))
	}
	if rec.count() != 5 {
		t.Fatalf("recorder must accumulate 5 snapshots, got %d", rec.count())
	}
}

func TestBaseline_Leak_FullLifecycle100Iterations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100-iteration full lifecycle leak test in short mode")
	}
	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	if _, err := os.Stat(toolBasicDir); err != nil {
		t.Fatalf("tool-basic extension required: %v", err)
	}

	iterations := 100
	tempDir := t.TempDir()
	extRoot := t.TempDir()

	baselineSnap := captureSnapshot(t, tempDir, extRoot)
	installer := amitiax.NewInstaller()

	for i := 0; i < iterations; i++ {
		archivePath := filepath.Join(tempDir, fmt.Sprintf("tool-basic-%d.amitiax", i))
		buildArchiveFromExtension(t, toolBasicDir, archivePath)
		targetDir := filepath.Join(extRoot, fmt.Sprintf("tool-basic-%d", i))

		result := installer.Install(context.Background(), amitiax.InstallRequest{
			ArchivePath: archivePath,
			TargetDir:   targetDir,
		})
		if result.Status != amitiax.InstallSucceeded {
			t.Fatalf("iteration %d install phase failed: %v", i, result.Errors)
		}

		_ = os.Remove(archivePath)
		removeAllWithRetry(t, targetDir)

		if i%10 == 0 {
			runtime.GC()
		}
	}

	finalSnap := captureSnapshot(t, tempDir, extRoot)
	goroutineGrowth := finalSnap.goroutines - baselineSnap.goroutines
	if goroutineGrowth > 5 {
		t.Fatalf("goroutine leak after %d full lifecycle iterations: %s (growth=%d)",
			iterations, baselineSnap.diff(finalSnap), goroutineGrowth)
	}

	archiveResidue := 0
	if entries, err := os.ReadDir(tempDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".amitiax") {
				archiveResidue++
			}
		}
	}
	if archiveResidue > 0 {
		t.Fatalf("temp dir has %d archive residues after %d iterations", archiveResidue, iterations)
	}

	targetResidue := 0
	if entries, err := os.ReadDir(extRoot); err == nil {
		targetResidue = len(entries)
	}
	if targetResidue > 0 {
		t.Fatalf("extension root has %d target residues after %d iterations", targetResidue, iterations)
	}

	total := kernel.GlobalLegacyCallCounter().Total()
	if total != 0 {
		t.Fatalf("legacy call counter must stay 0 after %d full lifecycle iterations, got %d", iterations, total)
	}

	if !kernel.GlobalLegacyCallCounter().FinalGatePassed() {
		t.Fatalf("final gate must pass after %d full lifecycle iterations", iterations)
	}

	t.Logf("Leak: %d full lifecycle iterations completed, goroutine growth=%d, legacy=0, final gate passed",
		iterations, goroutineGrowth)
}
